package md.address.events.domain;

import org.junit.jupiter.api.Test;

import java.util.List;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;

class AddressTest {

    @Test
    void shouldGenerateIdWhenNull() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.", null);
        assertNotNull(address.id());
    }

    @Test
    void shouldPreserveIdWhenProvided() {
        var id = UUID.randomUUID();
        var address = new Address(id, "Кишинёв", "Пушкина", "ул.", null);
        assertEquals(id, address.id());
    }

    @Test
    void shouldPreserveAllFields() {
        var house = new House(List.of("1", "2"), List.of(List.of("3", "5")));
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.", house);

        assertEquals("Кишинёв", address.city());
        assertEquals("Пушкина", address.street());
        assertEquals("ул.", address.streetType());
        assertNotNull(address.house());
        assertEquals(List.of("1", "2"), address.house().numbers());
    }

    @Test
    void shouldHandleNullHouse() {
        var address = new Address(null, "Кишинёв", "Пушкина", "ул.", null);
        assertNull(address.house());
    }
}
