package md.address.events.persistence;

import md.address.events.domain.Address;
import md.address.events.domain.House;
import md.address.events.domain.MessageReference;
import org.junit.jupiter.api.Test;

import java.math.BigInteger;
import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

class AddressEntityTest {

    @Test
    void fromShouldMapBasicFields() {
        var id = UUID.randomUUID();
        var address = new Address(id, "Кишинёв", "Пушкина", "ул.", null);
        var ref = new MessageReference(BigInteger.ONE, BigInteger.TWO);

        var entity = AddressEntity.from(address, ref);

        assertEquals(id, entity.getId());
        assertEquals("Кишинёв", entity.getCityOriginal());
        assertEquals("Пушкина", entity.getStreetOriginal());
        assertEquals("ул.", entity.getStreetTypeOriginal());
        assertEquals(BigInteger.ONE, entity.getMessageId());
        assertEquals(BigInteger.TWO, entity.getChatId());
        assertNull(entity.getHouseNumbers());
        assertNull(entity.getHouseRanges());
    }

    @Test
    void fromShouldMapHouseNumbers() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.",
                new House(List.of("1", "2", "3А"), null));
        var ref = new MessageReference(BigInteger.ONE, BigInteger.TWO);

        var entity = AddressEntity.from(address, ref);

        assertEquals("1,2,3А", entity.getHouseNumbers());
        assertNull(entity.getHouseRanges());
    }

    @Test
    void fromShouldMapHouseRanges() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.",
                new House(null, List.of(List.of("1", "10"), List.of("20", "30"))));
        var ref = new MessageReference(BigInteger.ONE, BigInteger.TWO);

        var entity = AddressEntity.from(address, ref);

        assertNull(entity.getHouseNumbers());
        assertEquals("1-10;20-30", entity.getHouseRanges());
    }

    @Test
    void fromShouldHandleEmptyHouseLists() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.",
                new House(List.of(), List.of()));
        var ref = new MessageReference(BigInteger.ONE, BigInteger.TWO);

        var entity = AddressEntity.from(address, ref);

        assertNull(entity.getHouseNumbers());
        assertNull(entity.getHouseRanges());
    }

    @Test
    void fromShouldHandleBothNumbersAndRanges() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.",
                new House(List.of("5", "7"), List.of(List.of("10", "20"))));
        var ref = new MessageReference(BigInteger.ONE, BigInteger.TWO);

        var entity = AddressEntity.from(address, ref);

        assertEquals("5,7", entity.getHouseNumbers());
        assertEquals("10-20", entity.getHouseRanges());
    }
}
