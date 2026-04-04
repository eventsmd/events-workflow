package md.address.events.persistence;

import org.junit.jupiter.api.Test;

import java.math.BigInteger;

import static org.junit.jupiter.api.Assertions.*;

class TelegramMessageIdTest {

    @Test
    void equalsShouldReturnTrueForSameValues() {
        var id1 = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        var id2 = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        assertEquals(id1, id2);
    }

    @Test
    void equalsShouldReturnFalseForDifferentValues() {
        var id1 = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        var id2 = new TelegramMessageId(BigInteger.ONE, BigInteger.TEN);
        assertNotEquals(id1, id2);
    }

    @Test
    void hashCodeShouldBeConsistent() {
        var id1 = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        var id2 = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        assertEquals(id1.hashCode(), id2.hashCode());
    }

    @Test
    void equalsShouldHandleSameReference() {
        var id = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        assertEquals(id, id);
    }

    @Test
    void equalsShouldHandleNull() {
        var id = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        assertNotEquals(null, id);
    }

    @Test
    void equalsShouldHandleDifferentType() {
        var id = new TelegramMessageId(BigInteger.ONE, BigInteger.TWO);
        assertNotEquals("not an id", id);
    }

    @Test
    void defaultConstructorShouldCreateEmptyId() {
        var id = new TelegramMessageId();
        assertNull(id.getId());
        assertNull(id.getChatId());
    }

    @Test
    void settersShouldWork() {
        var id = new TelegramMessageId();
        id.setId(BigInteger.ONE);
        id.setChatId(BigInteger.TWO);
        assertEquals(BigInteger.ONE, id.getId());
        assertEquals(BigInteger.TWO, id.getChatId());
    }
}
