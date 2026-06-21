package md.address.events;

import com.fasterxml.jackson.databind.ObjectMapper;
import io.temporal.api.workflowservice.v1.DescribeNamespaceRequest;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowOptions;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.serviceclient.WorkflowServiceStubsOptions;
import md.address.events.domain.TelegramMessage;
import md.address.events.domain.User;
import md.address.events.workflows.TelegramMessageProcess;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Tag;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.containers.Network;
import org.testcontainers.containers.PostgreSQLContainer;
import org.testcontainers.containers.localstack.LocalStackContainer;
import org.testcontainers.containers.wait.strategy.Wait;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;
import software.amazon.awssdk.auth.credentials.AwsBasicCredentials;
import software.amazon.awssdk.auth.credentials.StaticCredentialsProvider;
import software.amazon.awssdk.regions.Region;
import software.amazon.awssdk.services.sqs.SqsClient;
import software.amazon.awssdk.services.sqs.model.Message;

import java.io.OutputStream;
import java.math.BigInteger;
import java.net.HttpURLConnection;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.ResultSet;
import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeUnit;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Smoke test that runs the actual native image as a Docker container and
 * exercises the FULL business path end-to-end inside the native binary:
 * saveRawMessage -> parseMessage (OpenAI) -> saveParsedMessage ->
 * normalizeAddress (geo HTTP client + AddressPicker OpenAI) -> notify (SQS).
 *
 * <p>OpenAI and the geo API are stubbed with WireMock, so no real API keys are
 * needed and the whole reflection/serialization/proxy surface of the workflow
 * is actually invoked under native-image — which is exactly where missing
 * runtime hints would surface (ClassNotFoundException, "No serializer",
 * missing reflection/proxy registration, etc.).
 *
 * Requires: docker image 'events-workflow:native' built beforehand.
 * Run: mvn -Pnative -DskipTests spring-boot:build-image -Dspring-boot.build-image.imageName=events-workflow:native
 */
@Testcontainers
@Timeout(value = 240, unit = TimeUnit.SECONDS)
@Tag("native-smoke")
class NativeImageSmokeTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    /** Street-level KLADR returned by the geo stub and matched by the seeded subscription. */
    private static final String STREET_KLADR = "510-01.000-01.000-00.000-12.345";

    static final Network network = Network.newNetwork();

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
            .withNetworkAliases("temporal")
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
            .withServices(LocalStackContainer.Service.SQS)
            .withNetwork(network)
            .withNetworkAliases("localstack");

    @Container
    static GenericContainer<?> wiremock = new GenericContainer<>(
            DockerImageName.parse("wiremock/wiremock:latest"))
            .withExposedPorts(8080)
            .withNetwork(network)
            .withNetworkAliases("wiremock")
            // Force HTTP/1.1: the app's HTTP client otherwise negotiates HTTP/2 cleartext with
            // WireMock and the OpenAI call dies with "Received RST_STREAM: Stream cancelled".
            .withCommand("--disable-http2-plain")
            .waitingFor(Wait.forHttp("/__admin/mappings").forStatusCode(200))
            .withStartupTimeout(Duration.ofMinutes(1));

    /** Image under test; override with -Dsmoke.image=events-workflow:jvm to isolate native-specific issues. */
    static final String APP_IMAGE = System.getProperty("smoke.image", "events-workflow:native");

    static GenericContainer<?> app;
    static WorkflowClient workflowClient;
    static String queueUrl;

    @BeforeAll
    static void startApp() throws Exception {
        // Create SQS queue
        try (var sqsClient = createSqsClient()) {
            queueUrl = sqsClient.createQueue(r -> r.queueName("test-notifications")).queueUrl();
        }

        // Stub OpenAI (parser + address picker) and the geo /parse endpoint.
        registerStubs();

        // Temporal auto-setup only reports a listening port well before the default namespace is
        // registered. The native app calls WorkerFactory.start() (describeNamespace) on
        // ApplicationReadyEvent and aborts startup if the namespace is not ready yet, so wait for
        // Temporal to be genuinely ready before launching the app.
        waitForTemporalReady();

        // Start the native image container. OpenAI and geo are redirected to WireMock.
        app = new GenericContainer<>(DockerImageName.parse(APP_IMAGE))
                .withNetwork(network)
                .withExposedPorts(8080)
                .withEnv("DB_URL", "jdbc:postgresql://postgres:5432/events")
                .withEnv("DB_USERNAME", "test")
                .withEnv("DB_PASSWORD", "test")
                .withEnv("TEMPORAL_URL", "temporal:7233")
                .withEnv("OPENAI_API_KEY", "test-key")
                .withEnv("SPRING_AI_OPENAI_BASE_URL", "http://wiremock:8080")
                .withEnv("GEO_BASE_URL", "http://wiremock:8080")
                .withEnv("AWS_ACCESS_KEY_ID", "test")
                .withEnv("AWS_SECRET_ACCESS_KEY", "test")
                .withEnv("AWS_REGION", localstack.getRegion())
                .withEnv("AWS_SQS_QUEUE_NAME", "test-notifications")
                .withEnv("SPRING_CLOUD_AWS_SQS_ENDPOINT", "http://localstack:4566")
                .withEnv("SPRING_CLOUD_AWS_ENDPOINT", "http://localstack:4566")
                .waitingFor(Wait.forHttp("/actuator/health")
                        .forPort(8080)
                        .forStatusCode(200)
                        .withStartupTimeout(Duration.ofSeconds(60)))
                .withLogConsumer(frame -> System.err.print("[NATIVE] " + frame.getUtf8String()))
                .dependsOn(postgres, temporal, localstack, wiremock);
        app.start();

        // Seed a subscription whose KLADR matches the prefix produced by the stubbed street KLADR.
        try (Connection conn = DriverManager.getConnection(
                postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {
            conn.createStatement().execute(
                    "INSERT INTO subscriptions (id, created_at, subscribe_to_kladr, tg_id, subscribe_to_fulltext) " +
                            "VALUES ('" + UUID.randomUUID() + "', NOW(), '" + STREET_KLADR + "', '12345', 'Кишинёв, ул. Пушкина') " +
                            "ON CONFLICT DO NOTHING"
            );
        }

        // Connect Temporal client to the exposed Temporal port
        var stubs = WorkflowServiceStubs.newServiceStubs(
                WorkflowServiceStubsOptions.newBuilder()
                        .setTarget(temporal.getHost() + ":" + temporal.getMappedPort(7233))
                        .build());
        workflowClient = WorkflowClient.newInstance(stubs);
    }

    @AfterAll
    static void stopApp() {
        if (app != null && app.isRunning()) {
            app.stop();
        }
    }

    @Test
    @DisplayName("Native image starts and health endpoint responds")
    void healthCheckShouldPass() throws Exception {
        var url = "http://" + app.getHost() + ":" + app.getMappedPort(8080) + "/actuator/health";
        var conn = (HttpURLConnection) URI.create(url).toURL().openConnection();
        conn.setRequestMethod("GET");
        assertEquals(200, conn.getResponseCode());
    }

    @Test
    @DisplayName("Full workflow runs natively: parse -> save -> normalize (geo + picker) -> notify (SQS)")
    void shouldProcessFullWorkflow() throws Exception {
        var message = new TelegramMessage(
                BigInteger.valueOf(5001), BigInteger.valueOf(6001),
                new User(BigInteger.valueOf(777), "UtilityBot"),
                "SA Apă-Canal: отключение водоснабжения по адресу г. Кишинёв, ул. Пушкина, д. 1, 2. "
                        + "Время: 01.12.2025 10:00 - 18:00",
                LocalDateTime.of(2025, 12, 1, 9, 0),
                null, "water", Map.of("supplier", "water")
        );

        var options = WorkflowOptions.newBuilder()
                .setTaskQueue("TelegramMessageQueue")
                .setWorkflowId("native-test-" + UUID.randomUUID())
                .setWorkflowExecutionTimeout(Duration.ofSeconds(60))
                .build();
        var workflow = workflowClient.newWorkflowStub(TelegramMessageProcess.class, options);

        // Synchronous: blocks until the entire workflow completes. With the stubs in place
        // every activity must succeed inside the native image; any missing native-image hint
        // surfaces here as an activity failure with the underlying reflection/serialization error.
        workflow.processMessage(message);

        // 1) Raw message persisted (saveRawMessage)
        // 2) Transcription persisted (saveParsedMessage)
        // 3) Address enriched with the street KLADR (normalizeAddress -> geo + AddressPicker)
        try (Connection conn = DriverManager.getConnection(
                postgres.getJdbcUrl(), postgres.getUsername(), postgres.getPassword())) {

            ResultSet raw = conn.createStatement().executeQuery(
                    "SELECT service_name, text FROM telegram_messages WHERE id = 5001 AND chat_id = 6001");
            assertTrue(raw.next(), "Raw message should be saved by native image");
            assertEquals("water", raw.getString("service_name"));
            assertNotNull(raw.getString("text"));

            ResultSet transcribe = conn.createStatement().executeQuery(
                    "SELECT event FROM telegram_message_transcribes WHERE id = 5001 AND chat_id = 6001");
            assertTrue(transcribe.next(), "Transcription should be saved (OpenAI parse path ran natively)");
            assertEquals("shutdown", transcribe.getString("event"));

            ResultSet address = conn.createStatement().executeQuery(
                    "SELECT street_kladr, street_name FROM incident_address " +
                            "WHERE message_id = 5001 AND chat_id = 6001");
            assertTrue(address.next(), "Address should be saved");
            assertEquals(STREET_KLADR, address.getString("street_kladr"),
                    "Address should be enriched via geo client + AddressPicker inside native image");
        }

        // 4) Notification sent to SQS (notify path: KLADR prefix match + MessageToSend serialization)
        try (var sqsClient = createSqsClient()) {
            var received = sqsClient.receiveMessage(r -> r.queueUrl(queueUrl)
                    .maxNumberOfMessages(10)
                    .waitTimeSeconds(10)).messages();
            assertFalse(received.isEmpty(), "A notification should be sent to SQS by the native image");
            var body = received.stream().map(Message::body).reduce("", (a, b) -> a + b);
            assertTrue(body.contains("12345"),
                    "SQS payload should contain the subscriber telegram id (MessageToSend serialized natively)");
        }
    }

    @Test
    @DisplayName("Native image startup completed successfully")
    void startupShouldSucceed() {
        var logs = app.getLogs();
        assertTrue(logs.contains("Started"), "Application should have started successfully");
    }

    /** Polls Temporal until the default namespace is registered (auto-setup finished bootstrapping). */
    private static void waitForTemporalReady() throws InterruptedException {
        var target = temporal.getHost() + ":" + temporal.getMappedPort(7233);
        RuntimeException last = null;
        for (int attempt = 0; attempt < 45; attempt++) {
            WorkflowServiceStubs stubs = null;
            try {
                stubs = WorkflowServiceStubs.newServiceStubs(
                        WorkflowServiceStubsOptions.newBuilder()
                                .setTarget(target)
                                .setRpcTimeout(Duration.ofSeconds(3))
                                .build());
                stubs.blockingStub().describeNamespace(
                        DescribeNamespaceRequest.newBuilder().setNamespace("default").build());
                return;
            } catch (Exception e) {
                last = new IllegalStateException("Temporal not ready yet: " + e.getMessage(), e);
                Thread.sleep(2000);
            } finally {
                if (stubs != null) {
                    stubs.shutdownNow();
                }
            }
        }
        throw last;
    }

    // --- WireMock stubbing ---------------------------------------------------

    private static void registerStubs() throws Exception {
        // MessageParser -> JSON transcription
        String transcription = MAPPER.writeValueAsString(Map.of(
                "organization", "SA Apă-Canal",
                "short_description", "Отключение водоснабжения",
                "event", "shutdown",
                "event_start", "2025-12-01T10:00",
                "event_stop", "2025-12-01T18:00",
                "addresses", List.of(Map.of(
                        "city", "Кишинёв",
                        "street", "Пушкина",
                        "street_type", "ул.",
                        "house", Map.of("numbers", List.of("1", "2"), "ranges", List.of())
                ))
        ));
        registerChatCompletionStub("generate a JSON object", transcription);

        // AddressPicker -> index of the chosen variant
        registerChatCompletionStub("Select the address variant", "0");

        // geo /parse -> two variants (forces the AddressPicker path); index 0 carries the street KLADR
        KladrEntityJson street = new KladrEntityJson(STREET_KLADR, "Пушкина", "ул. Пушкина", "ул", "улица");
        KladrEntityJson city = new KladrEntityJson("510-01.000-01.000-00.000-00.000", "Кишинёв", "мун. Кишинёв", "мун", "муниципий");
        KladrEntityJson region = new KladrEntityJson("510-01.000-00.000-00.000-00.000", "Кишинёв", "мун. Кишинёв", "мун", "муниципий");

        List<Map<String, Object>> variants = List.of(
                addressKladr("Кишинёв, ул. Пушкина", STREET_KLADR, region, city, street),
                addressKladr("Кишинёв, ул. Пушкинская",
                        "510-01.000-01.000-00.000-99.999", region, city,
                        new KladrEntityJson("510-01.000-01.000-00.000-99.999", "Пушкинская", "ул. Пушкинская", "ул", "улица"))
        );
        registerGeoStub(MAPPER.writeValueAsString(variants));
    }

    private static Map<String, Object> addressKladr(
            String fullAddress, String kladr,
            KladrEntityJson region, KladrEntityJson city, KladrEntityJson street) {
        return Map.of(
                "country", "MD",
                "full_address", fullAddress,
                "house", "",
                "kladr", kladr,
                "region", region.toMap(),
                "city", city.toMap(),
                "street", street.toMap()
        );
    }

    private record KladrEntityJson(String kladr, String name, String nameWithType, String type, String typeFull) {
        Map<String, Object> toMap() {
            return Map.of(
                    "kladr", kladr,
                    "name", name,
                    "name_with_type", nameWithType,
                    "type", type,
                    "type_full", typeFull
            );
        }
    }

    private static void registerChatCompletionStub(String promptMarker, String content) throws Exception {
        String envelope = MAPPER.writeValueAsString(Map.of(
                "id", "chatcmpl-test",
                "object", "chat.completion",
                "created", 1700000000,
                "model", "gpt-5-mini",
                "choices", List.of(Map.of(
                        "index", 0,
                        "message", Map.of("role", "assistant", "content", content),
                        "finish_reason", "stop"
                )),
                "usage", Map.of("prompt_tokens", 1, "completion_tokens", 1, "total_tokens", 2)
        ));
        String mapping = MAPPER.writeValueAsString(Map.of(
                "request", Map.of(
                        "method", "POST",
                        "urlPath", "/v1/chat/completions",
                        "bodyPatterns", List.of(Map.of("contains", promptMarker))
                ),
                "response", Map.of(
                        "status", 200,
                        "headers", Map.of("Content-Type", "application/json"),
                        "body", envelope
                )
        ));
        postMapping(mapping);
    }

    private static void registerGeoStub(String body) throws Exception {
        String mapping = MAPPER.writeValueAsString(Map.of(
                "request", Map.of("method", "GET", "urlPath", "/parse"),
                "response", Map.of(
                        "status", 200,
                        "headers", Map.of("Content-Type", "application/json"),
                        "body", body
                )
        ));
        postMapping(mapping);
    }

    private static void postMapping(String mapping) throws Exception {
        var url = "http://" + wiremock.getHost() + ":" + wiremock.getMappedPort(8080) + "/__admin/mappings";
        var conn = (HttpURLConnection) URI.create(url).toURL().openConnection();
        conn.setRequestMethod("POST");
        conn.setDoOutput(true);
        conn.setRequestProperty("Content-Type", "application/json");
        try (OutputStream os = conn.getOutputStream()) {
            os.write(mapping.getBytes(StandardCharsets.UTF_8));
        }
        int code = conn.getResponseCode();
        if (code != 200 && code != 201) {
            throw new IllegalStateException("WireMock mapping registration failed: HTTP " + code);
        }
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
