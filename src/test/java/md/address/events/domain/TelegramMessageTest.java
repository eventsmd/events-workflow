package md.address.events.domain;

import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.*;

class TelegramMessageTest {

    @Test
    void shouldStoreAllFields() {
        var user = new User(BigInteger.ONE, "TestUser");
        var replyTo = new MessageReference(BigInteger.TWO, BigInteger.TEN);
        var context = Map.of("supplier", "water");
        var date = LocalDateTime.of(2025, 12, 1, 10, 0);

        var message = new TelegramMessage(
                BigInteger.valueOf(100),
                BigInteger.valueOf(200),
                user,
                "Test message",
                date,
                replyTo,
                "water",
                context
        );

        assertEquals(BigInteger.valueOf(100), message.id());
        assertEquals(BigInteger.valueOf(200), message.chatId());
        assertEquals("TestUser", message.from().name());
        assertEquals("Test message", message.text());
        assertEquals(date, message.date());
        assertEquals(BigInteger.TWO, message.replyTo().id());
        assertEquals("water", message.serviceName());
        assertEquals("water", message.context().get("supplier"));
    }

    @Test
    void shouldAllowNullReplyTo() {
        var message = new TelegramMessage(
                BigInteger.ONE, BigInteger.TWO,
                new User(BigInteger.ONE, "User"),
                "text", LocalDateTime.now(), null, null, null
        );
        assertNull(message.replyTo());
    }
}
