package md.address.events.persistence;

import md.address.events.domain.MessageReference;
import md.address.events.domain.TelegramMessage;
import md.address.events.domain.User;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.Map;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

class TelegramMessageEntityTest {

    @Test
    void fromShouldMapAllFields() {
        var user = new User(BigInteger.ONE, "TestUser");
        var replyTo = new MessageReference(BigInteger.TWO, BigInteger.TEN);
        var context = Map.of("supplier", "water");
        var date = LocalDateTime.of(2025, 12, 1, 10, 0);
        var incidentId = UUID.randomUUID();

        var message = new TelegramMessage(
                BigInteger.valueOf(100), BigInteger.valueOf(200),
                user, "text", date, replyTo, "water", context
        );

        var entity = TelegramMessageEntity.from(message, incidentId);

        assertEquals(BigInteger.valueOf(100), entity.getId());
        assertEquals(BigInteger.valueOf(200), entity.getChatId());
        assertEquals("text", entity.getText());
        assertEquals(date, entity.getDate());
        assertEquals("water", entity.getServiceName());
        assertEquals(BigInteger.ONE, entity.getFromId());
        assertEquals("TestUser", entity.getFromName());
        assertEquals(BigInteger.TWO, entity.getReplyToId());
        assertEquals(BigInteger.TEN, entity.getReplyToChatId());
        assertEquals(incidentId, entity.getIncidentId());
        assertEquals(context, entity.getContext());
    }

    @Test
    void fromShouldHandleNullMessage() {
        assertNull(TelegramMessageEntity.from(null, UUID.randomUUID()));
    }

    @Test
    void fromShouldHandleNullFromAndReplyTo() {
        var message = new TelegramMessage(
                BigInteger.ONE, BigInteger.TWO,
                null, "text", LocalDateTime.now(), null, null, null
        );

        var entity = TelegramMessageEntity.from(message, UUID.randomUUID());

        assertNull(entity.getFromId());
        assertNull(entity.getFromName());
        assertNull(entity.getReplyToId());
        assertNull(entity.getReplyToChatId());
    }

    @Test
    void toDomainShouldMapAllFields() {
        var incidentId = UUID.randomUUID();
        var date = LocalDateTime.of(2025, 12, 1, 10, 0);
        var context = Map.of("supplier", "water");

        var entity = new TelegramMessageEntity();
        entity.setId(BigInteger.valueOf(100));
        entity.setChatId(BigInteger.valueOf(200));
        entity.setText("text");
        entity.setDate(date);
        entity.setServiceName("water");
        entity.setFromId(BigInteger.ONE);
        entity.setFromName("TestUser");
        entity.setReplyToId(BigInteger.TWO);
        entity.setReplyToChatId(BigInteger.TEN);
        entity.setIncidentId(incidentId);
        entity.setContext(context);

        var domain = entity.toDomain();

        assertEquals(BigInteger.valueOf(100), domain.id());
        assertEquals(BigInteger.valueOf(200), domain.chatId());
        assertEquals("text", domain.text());
        assertEquals(date, domain.date());
        assertEquals("water", domain.serviceName());
        assertEquals("TestUser", domain.from().name());
        assertEquals(BigInteger.TWO, domain.replyTo().id());
    }

    @Test
    void toDomainShouldFallbackToContextSupplierWhenServiceNameNull() {
        var entity = new TelegramMessageEntity();
        entity.setId(BigInteger.ONE);
        entity.setChatId(BigInteger.TWO);
        entity.setText("text");
        entity.setDate(LocalDateTime.now());
        entity.setServiceName(null);
        entity.setContext(Map.of("supplier", "electricity"));

        var domain = entity.toDomain();
        assertEquals("electricity", domain.serviceName());
    }

    @Test
    void toDomainShouldHandleNullContextWhenServiceNameNull() {
        var entity = new TelegramMessageEntity();
        entity.setId(BigInteger.ONE);
        entity.setChatId(BigInteger.TWO);
        entity.setText("text");
        entity.setDate(LocalDateTime.now());
        entity.setServiceName(null);
        entity.setContext(null);

        var domain = entity.toDomain();
        assertNull(domain.serviceName());
    }

    @Test
    void toDomainShouldHandleNullFromFields() {
        var entity = new TelegramMessageEntity();
        entity.setId(BigInteger.ONE);
        entity.setChatId(BigInteger.TWO);
        entity.setText("text");
        entity.setDate(LocalDateTime.now());
        entity.setContext(Map.of());

        var domain = entity.toDomain();
        assertNull(domain.from());
        assertNull(domain.replyTo());
    }
}
