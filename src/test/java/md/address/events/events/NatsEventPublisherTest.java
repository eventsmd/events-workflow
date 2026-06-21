package md.address.events.events;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.time.Duration;

import static org.junit.jupiter.api.Assertions.*;

class NatsEventPublisherTest {

    @Test
    void buildsSanitizedSubject() {
        assertEquals("pmr.utility.event.water.shutdown",
                NatsEventPublisher.subjectFor("pmr.utility.event", "water", "shutdown"));
    }

    @Test
    void sanitizesIllegalTokenChars() {
        assertEquals("a_b", NatsEventPublisher.sanitizeToken("a b"));
        assertEquals("a_b", NatsEventPublisher.sanitizeToken("a.b"));
        assertEquals("a_b", NatsEventPublisher.sanitizeToken("a*b"));
        assertEquals("a_b", NatsEventPublisher.sanitizeToken("a>b"));
        assertEquals("a_b", NatsEventPublisher.sanitizeToken("a/b"));
        assertEquals("x", NatsEventPublisher.sanitizeToken(".x."));
        assertEquals("_", NatsEventPublisher.sanitizeToken(""));
        assertEquals("_", NatsEventPublisher.sanitizeToken(null));
    }

    @Test
    void publishIsNoOpAndDoesNotThrowWhenUrlBlank() {
        var publisher = new NatsEventPublisher(
                new ObjectMapper(), "", "", "", "", "UTILITY", "pmr.utility.event", Duration.ofHours(24));
        assertDoesNotThrow(() -> publisher.publish(new UtilityEvent(
                "i", "water", "shutdown", "o", "d", null, null,
                java.time.Instant.EPOCH, new EventSource(java.math.BigInteger.ONE, java.math.BigInteger.TWO),
                java.util.List.of())));
    }
}
