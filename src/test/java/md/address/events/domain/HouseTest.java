package md.address.events.domain;

import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.*;

class HouseTest {

    @Test
    void shouldStoreNumbers() {
        var house = new House(List.of("1", "2", "3А"), null);
        assertEquals(List.of("1", "2", "3А"), house.numbers());
        assertNull(house.ranges());
    }

    @Test
    void shouldStoreRanges() {
        var house = new House(null, List.of(List.of("1", "10"), List.of("20", "30")));
        assertNull(house.numbers());
        assertEquals(2, house.ranges().size());
        assertEquals(List.of("1", "10"), house.ranges().get(0));
    }

    @Test
    void shouldHandleBothNumbersAndRanges() {
        var house = new House(List.of("5"), List.of(List.of("1", "3")));
        assertEquals(1, house.numbers().size());
        assertEquals(1, house.ranges().size());
    }
}
