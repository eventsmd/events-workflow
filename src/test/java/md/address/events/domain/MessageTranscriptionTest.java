package md.address.events.domain;

import org.junit.jupiter.api.Test;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class MessageTranscriptionTest {

    @Test
    void shouldStoreAllFields() {
        var start = LocalDateTime.of(2025, 12, 1, 10, 0);
        var stop = LocalDateTime.of(2025, 12, 1, 18, 0);
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.", null);

        var transcription = new MessageTranscription(
                "SA Apă-Canal",
                "Отключение водоснабжения",
                "shutdown",
                start,
                stop,
                List.of(address)
        );

        assertEquals("SA Apă-Canal", transcription.organization());
        assertEquals("Отключение водоснабжения", transcription.shortDescription());
        assertEquals("shutdown", transcription.event());
        assertEquals(start, transcription.eventStart());
        assertEquals(stop, transcription.eventStop());
        assertEquals(1, transcription.addresses().size());
    }

    @Test
    void shouldAllowNullAddresses() {
        var transcription = new MessageTranscription(
                "org", "desc", "other", null, null, null
        );
        assertNull(transcription.addresses());
    }
}
