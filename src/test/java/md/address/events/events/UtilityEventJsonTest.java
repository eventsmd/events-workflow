package md.address.events.events;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.time.Instant;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;

class UtilityEventJsonTest {

    private final ObjectMapper mapper = new ObjectMapper()
            .registerModule(new JavaTimeModule())
            .disable(com.fasterxml.jackson.databind.SerializationFeature.WRITE_DATES_AS_TIMESTAMPS)
            .setSerializationInclusion(com.fasterxml.jackson.annotation.JsonInclude.Include.NON_NULL);

    @Test
    void serializesCamelCaseWithFormattedDates() throws Exception {
        var event = new UtilityEvent(
                "inc-1", "water", "shutdown", "SA Apă-Canal", "Отключение воды",
                LocalDateTime.of(2026, 6, 21, 10, 0),
                LocalDateTime.of(2026, 6, 21, 18, 0),
                Instant.parse("2026-06-21T07:00:00Z"),
                new EventSource(BigInteger.valueOf(123), BigInteger.valueOf(-100123)),
                List.of(new EventAddress(
                        new KladrRef("Кишинёв", "001", "г"),
                        new KladrRef("Кишинёв", "001-01", "г"),
                        new KladrRef("Пушкина", "001-01.001", "ул"),
                        List.of("1", "3", "5-9")))
        );

        var json = mapper.writeValueAsString(event);

        assertTrue(json.contains("\"incidentId\":\"inc-1\""), json);
        assertTrue(json.contains("\"supplier\":\"water\""), json);
        assertTrue(json.contains("\"eventStart\":\"2026-06-21T10:00\""), json);
        assertTrue(json.contains("\"publishedAt\":\"2026-06-21T07:00:00Z\""), json);
        assertTrue(json.contains("\"houses\":[\"1\",\"3\",\"5-9\"]"), json);
        assertTrue(json.contains("\"messageId\":123"), json);
    }

    @Test
    void omitsNullEventStop() throws Exception {
        var event = new UtilityEvent(
                "inc-2", "electricity", "resume", "org", "desc",
                LocalDateTime.of(2026, 6, 21, 10, 0), null,
                Instant.parse("2026-06-21T07:00:00Z"),
                new EventSource(BigInteger.ONE, BigInteger.TWO), List.of());
        var json = mapper.writeValueAsString(event);
        assertTrue(!json.contains("eventStop"), json);
    }
}
