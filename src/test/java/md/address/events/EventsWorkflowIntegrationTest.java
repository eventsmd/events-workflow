package md.address.events;

import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import md.address.events.ai.MessageParser;
import md.address.events.domain.*;
import md.address.events.geo.AddressApi;
import md.address.events.geo.AddressKladr;
import md.address.events.geo.KladrEntity;
import md.address.events.persistence.*;
import md.address.events.workflows.TelegramMessageProcess;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.DynamicPropertyRegistry;
import org.springframework.test.context.DynamicPropertySource;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.localstack.LocalStackContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.sqs.SqsClient;

import java.math.BigInteger;
import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.when;

@SpringBootTest
@Testcontainers
@Timeout(value = 120, unit = TimeUnit.SECONDS)
class EventsWorkflowIntegrationTest {

    static Network network = Network.newNetwork();

    @Container
    static PostgreSQLContainer<?> postgres = new PostgreSQLContainer<>("postgres:16-alpine")
            .withDatabaseName("events")
            .withUsername("test")
            .withPassword("test")
            .withNetwork(network)
            .withNetworkAliases("postgres");

    @Container
    static GenericContainer<?> temporal = new GenericContainer<>("temporalio/auto-setup:latest")
            .withExposedPorts(7233)
            .withNetwork(network)
            .withEnv("DB", "postgres12")
            .withEnv("DB_PORT", "5432")
            .withEnv("POSTGRES_USER", "test")
            .withEnv("POSTGRES_PWD", "test")
            .withEnv("POSTGRES_SEEDS", "postgres")
            .waitingFor(Wait.forListeningPort())
            .withStartupTimeout(Duration.ofMinutes(2))
            .dependsOn(postgres);

    @Container
    static LocalStackContainer localstack = new LocalStackContainer(
            DockerImageName.parse("localstack/localstack:3"))
            .withServices(LocalStackContainer.Service.SQS);

    static final String QUEUE_NAME = "test-notifications";
    static String queueUrl;

    @MockitoBean
    MessageParser messageParser;

    @MockitoBean
    AddressApi addressApi;

    @Autowired WorkflowClient workflowClient;
    @Autowired TelegramMessageRepository telegramMessageRepository;
    @Autowired TelegramMessageTranscribeRepository transcribeRepository;
    @Autowired AddressRepository addressRepository;
    @Autowired SubscriptionRepository subscriptionRepository;

    @DynamicPropertySource
    static void configureProperties(DynamicPropertyRegistry registry) {
        // PostgreSQL
        registry.add("spring.datasource.url", postgres::getJdbcUrl);
        registry.add("spring.datasource.username", postgres::getUsername);
        registry.add("spring.datasource.password", postgres::getPassword);
        registry.add("spring.flyway.url", postgres::getJdbcUrl);
        registry.add("spring.flyway.user", postgres::getUsername);
        registry.add("spring.flyway.password", postgres::getPassword);

        // Temporal
        registry.add("spring.temporal.connection.target", () ->
                temporal.getHost() + ":" + temporal.getMappedPort(7233));

        // AWS / SQS
        registry.add("spring.cloud.aws.credentials.access-key", () -> "test");
        registry.add("spring.cloud.aws.credentials.secret-key", () -> "test");
        registry.add("spring.cloud.aws.region.static", () -> localstack.getRegion());
        registry.add("spring.cloud.aws.sqs.endpoint", () ->
                localstack.getEndpointOverride(LocalStackContainer.Service.SQS).toString());
        registry.add("AWS_SQS_QUEUE_NAME", () -> QUEUE_NAME);

        // OpenAI (dummy — MessageParser is mocked)
        registry.add("spring.ai.openai.api-key", () -> "test-key");

        // GEO (dummy — AddressApi is mocked)
        registry.add("GEO_BASE_URL", () -> "http://localhost:9999");
    }

    @BeforeAll
    static void createSqsQueue() {
        try (var sqsClient = createSqsClient()) {
            queueUrl = sqsClient.createQueue(r -> r.queueName(QUEUE_NAME)).queueUrl();
        }
    }

    @BeforeEach
    void setupMocks() {
        var eventStart = LocalDateTime.of(2025, 12, 1, 10, 0);
        var eventStop = LocalDateTime.of(2025, 12, 1, 18, 0);
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.",
                new House(List.of("1", "2"), null));

        var transcription = new MessageTranscription(
                "SA Apă-Canal", "Отключение водоснабжения",
                "shutdown", eventStart, eventStop, List.of(address)
        );
        when(messageParser.parse(any(), anyString())).thenReturn(transcription);

        var region = new KladrEntity("5100000000000", "Молдова", "Молдова", null, null);
        var city = new KladrEntity("5100000100000", "Кишинёв", "г Кишинёв", "г", "город");
        var street = new KladrEntity("5100000100078", "Пушкина", "ул Пушкина", "ул", "улица");
        var kladr = new AddressKladr("MD", "Кишинёв, ул Пушкина", null,
                "5100000100078", region, city, street);
        when(addressApi.find(anyString())).thenReturn(List.of(kladr));
    }

    @Test
    void shouldProcessFullWorkflowAndNotifySubscriber() {
        // Seed subscription for the street
        var subscription = new Subscription();
        subscription.setId(UUID.randomUUID());
        subscription.setCreatedAt(LocalDateTime.now());
        subscription.setTgId("12345");
        subscription.setSubscribeToKladr("5100000100078");
        subscription.setSubscribeToFulltext("Кишинёв, ул. Пушкина");
        subscriptionRepository.saveAndFlush(subscription);

        // Prepare test message
        var message = new TelegramMessage(
                BigInteger.valueOf(1001), BigInteger.valueOf(2001),
                new User(BigInteger.valueOf(777), "UtilityBot"),
                "SA Apă-Canal сообщает об отключении водоснабжения по адресу: "
                        + "г. Кишинёв, ул. Пушкина, д. 1, 2. Время: 01.12.2025 10:00 - 18:00",
                LocalDateTime.of(2025, 12, 1, 9, 0),
                null, "water", Map.of("supplier", "water")
        );

        // Execute workflow via Temporal
        var options = WorkflowOptions.newBuilder()
                .setTaskQueue("TelegramMessageQueue")
                .setWorkflowId("test-full-" + UUID.randomUUID())
                .build();
        var workflow = workflowClient.newWorkflowStub(TelegramMessageProcess.class, options);
        workflow.processMessage(message);

        // 1. Verify raw message persisted
        var savedMessage = telegramMessageRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(1001), BigInteger.valueOf(2001)));
        assertTrue(savedMessage.isPresent(), "Raw message should be saved");
        assertEquals("water", savedMessage.get().getServiceName());
        assertNotNull(savedMessage.get().getIncidentId());

        // 2. Verify transcription persisted
        var savedTranscription = transcribeRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(1001), BigInteger.valueOf(2001)));
        assertTrue(savedTranscription.isPresent(), "Transcription should be saved");
        assertEquals("SA Apă-Canal", savedTranscription.get().getOrganization());
        assertEquals("shutdown", savedTranscription.get().getEvent());
        assertEquals(LocalDateTime.of(2025, 12, 1, 10, 0), savedTranscription.get().getEventStart());

        // 3. Verify addresses saved and enriched via KLADR
        var addresses = addressRepository.findByMessageIdAndChatId(
                BigInteger.valueOf(1001), BigInteger.valueOf(2001));
        assertFalse(addresses.isEmpty(), "Addresses should be saved");
        var addr = addresses.getFirst();
        assertEquals("Кишинёв", addr.getCityName());
        assertEquals("Пушкина", addr.getStreetName());
        assertEquals("5100000100078", addr.getStreetKladr());
        assertEquals("ул", addr.getStreetType());

        // 4. Verify SQS notification sent
        try (var sqsClient = createSqsClient()) {
            var response = sqsClient.receiveMessage(r -> r
                    .queueUrl(queueUrl)
                    .maxNumberOfMessages(1)
                    .waitTimeSeconds(10));
            assertFalse(response.messages().isEmpty(), "SQS notification should be sent");
            var body = response.messages().getFirst().body();
            assertTrue(body.contains("12345") || body.contains("telegram_id"),
                    "Notification should contain subscriber telegram_id");
        }
    }

    @Test
    void shouldSaveRawMessageButSkipProcessingWhenTranscriptionNull() {
        when(messageParser.parse(any(), anyString())).thenReturn(null);

        var message = new TelegramMessage(
                BigInteger.valueOf(2001), BigInteger.valueOf(3001),
                new User(BigInteger.ONE, "Bot"),
                "Some unrecognizable text", LocalDateTime.now(),
                null, null, Map.of()
        );

        var options = WorkflowOptions.newBuilder()
                .setTaskQueue("TelegramMessageQueue")
                .setWorkflowId("test-null-" + UUID.randomUUID())
                .build();
        var workflow = workflowClient.newWorkflowStub(TelegramMessageProcess.class, options);
        workflow.processMessage(message);

        // Raw message should still be saved
        var saved = telegramMessageRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(2001), BigInteger.valueOf(3001)));
        assertTrue(saved.isPresent(), "Raw message should always be saved");

        // No transcription should exist
        var transcription = transcribeRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(2001), BigInteger.valueOf(3001)));
        assertFalse(transcription.isPresent(), "No transcription when AI returns null");

        // No addresses should exist
        var addresses = addressRepository.findByMessageIdAndChatId(
                BigInteger.valueOf(2001), BigInteger.valueOf(3001));
        assertTrue(addresses.isEmpty(), "No addresses when AI returns null");
    }

    @Test
    void shouldLinkReplyToSameIncident() {
        // Save original message
        var originalMessage = new TelegramMessage(
                BigInteger.valueOf(3001), BigInteger.valueOf(4001),
                new User(BigInteger.ONE, "Bot"),
                "Original message", LocalDateTime.now(),
                null, "water", Map.of("supplier", "water")
        );
        var options1 = WorkflowOptions.newBuilder()
                .setTaskQueue("TelegramMessageQueue")
                .setWorkflowId("test-original-" + UUID.randomUUID())
                .build();
        workflowClient.newWorkflowStub(TelegramMessageProcess.class, options1)
                .processMessage(originalMessage);

        // Get incident ID of the original message
        var original = telegramMessageRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(3001), BigInteger.valueOf(4001)));
        assertTrue(original.isPresent());
        var originalIncidentId = original.get().getIncidentId();

        // Send reply
        var replyMessage = new TelegramMessage(
                BigInteger.valueOf(3002), BigInteger.valueOf(4001),
                new User(BigInteger.ONE, "Bot"),
                "Reply: restored", LocalDateTime.now(),
                new MessageReference(BigInteger.valueOf(3001), BigInteger.valueOf(4001)),
                "water", Map.of("supplier", "water")
        );
        var options2 = WorkflowOptions.newBuilder()
                .setTaskQueue("TelegramMessageQueue")
                .setWorkflowId("test-reply-" + UUID.randomUUID())
                .build();
        workflowClient.newWorkflowStub(TelegramMessageProcess.class, options2)
                .processMessage(replyMessage);

        // Reply should share the same incident ID
        var reply = telegramMessageRepository.findById(
                new TelegramMessageId(BigInteger.valueOf(3002), BigInteger.valueOf(4001)));
        assertTrue(reply.isPresent());
        assertEquals(originalIncidentId, reply.get().getIncidentId(),
                "Reply should be linked to the same incident as the original message");
    }

    private static SqsClient createSqsClient() {
        return SqsClient.builder()
                .endpointOverride(localstack.getEndpointOverride(LocalStackContainer.Service.SQS))
                .region(Region.of(localstack.getRegion()))
                .credentialsProvider(StaticCredentialsProvider.create(
                        AwsBasicCredentials.create("test", "test")))
                .build();
    }
}
