package md.address.events.persistence;

import md.address.events.domain.MessageTranscription;
import md.address.events.domain.ParsedMessage;
import md.address.events.domain.TelegramMessage;
import md.address.events.domain.User;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class TelegramMessageTranscribeEntityTest {

    @Test
    void fromShouldMapAllFields() {
        var start = LocalDateTime.of(2025, 12, 1, 10, 0);
        var stop = LocalDateTime.of(2025, 12, 1, 18, 0);

        var message = new TelegramMessage(
                BigInteger.valueOf(100), BigInteger.valueOf(200),
                new User(BigInteger.ONE, "User"), "text",
                LocalDateTime.now(), null, null, null
        );
        var transcription = new MessageTranscription(
                "SA Apă-Canal", "Отключение воды", "shutdown",
                start, stop, List.of()
        );
        var parsed = new ParsedMessage(message, transcription);

        var entity = TelegramMessageTranscribeEntity.from(parsed);

        assertEquals(BigInteger.valueOf(100), entity.getId());
        assertEquals(BigInteger.valueOf(200), entity.getChatId());
        assertEquals("SA Apă-Canal", entity.getOrganization());
        assertEquals("Отключение воды", entity.getDescription());
        assertEquals("shutdown", entity.getEvent());
        assertEquals(start, entity.getEventStart());
        assertEquals(stop, entity.getEventStop());
    }

    @Test
    void shouldSupportDefaultConstructorAndSetters() {
        var entity = new TelegramMessageTranscribeEntity();
        entity.setId(BigInteger.ONE);
        entity.setChatId(BigInteger.TWO);
        entity.setOrganization("org");
        entity.setDescription("desc");
        entity.setEvent("resume");
        entity.setEventStart(LocalDateTime.of(2025, 1, 1, 0, 0));
        entity.setEventStop(LocalDateTime.of(2025, 1, 2, 0, 0));

        assertEquals(BigInteger.ONE, entity.getId());
        assertEquals(BigInteger.TWO, entity.getChatId());
        assertEquals("org", entity.getOrganization());
        assertEquals("desc", entity.getDescription());
        assertEquals("resume", entity.getEvent());
    }
}
