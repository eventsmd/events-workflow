package md.address.events.events;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import io.nats.client.Connection;
import io.nats.client.ErrorListener;
import io.nats.client.JetStream;
import io.nats.client.Nats;
import io.nats.client.Options;
import io.nats.client.PullSubscribeOptions;
import io.nats.client.JetStreamSubscription;
import io.nats.client.Message;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;
import org.testcontainers.containers.GenericContainer;
import org.testcontainers.junit.jupiter.Container;
import org.testcontainers.junit.jupiter.Testcontainers;
import org.testcontainers.utility.DockerImageName;

import java.math.BigInteger;
import java.time.Duration;
import java.time.Instant;
import java.time.LocalDateTime;
import java.util.List;
import java.util.logging.Level;
import java.util.logging.Logger;

import static org.junit.jupiter.api.Assertions.*;

@Testcontainers
class NatsEventPublisherIntegrationTest {

    @Container
    static final GenericContainer<?> NATS = new GenericContainer<>(DockerImageName.parse("nats:2.10-alpine"))
            .withCommand("-js")
            .withExposedPorts(4222);

    // Strong reference kept so the JUL logger is not garbage-collected (LogManager holds it weakly);
    // otherwise the OFF level we set below would be lost before jnats logs at teardown.
    private static final Logger JNATS_LOGGER =
            Logger.getLogger("io.nats.client.impl.ErrorListenerLoggerImpl");

    @BeforeAll
    static void silenceJnatsTeardownNoise() {
        // jnats logs SEVERE EOF-on-close noise via JUL from its default error listener when the
        // publisher's internal connection (which we must not modify) sees a peer close on teardown.
        // Suppress it test-scoped only; it does not affect the publish/read assertions below.
        JNATS_LOGGER.setLevel(Level.OFF);
    }

    private String url() {
        return "nats://" + NATS.getHost() + ":" + NATS.getMappedPort(4222);
    }

    @Test
    void publishesEventReadableFromStream() throws Exception {
        var mapper = new ObjectMapper().registerModule(new JavaTimeModule());
        var publisher = new NatsEventPublisher(
                mapper, url(), "", "", "", "UTILITY", "pmr.utility.event", Duration.ofHours(24));

        var event = new UtilityEvent(
                "inc-int-1", "water", "shutdown", "SA Apă-Canal", "Отключение",
                LocalDateTime.of(2026, 6, 21, 10, 0), null,
                Instant.parse("2026-06-21T07:00:00Z"),
                new EventSource(BigInteger.valueOf(123), BigInteger.valueOf(200)),
                List.of(new EventAddress(null, null,
                        new KladrRef("Пушкина", "001-01.001", "ул"), List.of("1", "2"))));

        publisher.publish(event);

        Options options = new Options.Builder()
                .server(url())
                .errorListener(new ErrorListener() {
                })
                .build();
        try (Connection nc = Nats.connect(options)) {
            JetStream js = nc.jetStream();
            JetStreamSubscription sub = js.subscribe("pmr.utility.event.water.shutdown",
                    PullSubscribeOptions.builder().stream("UTILITY").build());
            List<Message> msgs = sub.fetch(1, Duration.ofSeconds(5));
            assertEquals(1, msgs.size(), "expected one message on the subject");
            String body = new String(msgs.get(0).getData());
            assertTrue(body.contains("\"incidentId\":\"inc-int-1\""), body);
            assertTrue(body.contains("\"supplier\":\"water\""), body);
            msgs.get(0).ack();
        }
    }
}
