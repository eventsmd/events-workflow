package md.address.events.persistence;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

class AddressEntityHouseDisplayTest {

    @Test
    void shouldFormatNumbersOnly() {
        var entity = new AddressEntity();
        entity.setHouseNumbers("1,2,3");
        assertEquals("д. 1, 2, 3", entity.formatHouses());
    }

    @Test
    void shouldFormatRangesOnly() {
        var entity = new AddressEntity();
        entity.setHouseRanges("1-10;20-30");
        assertEquals("д. 1-10, 20-30", entity.formatHouses());
    }

    @Test
    void shouldFormatNumbersAndRanges() {
        var entity = new AddressEntity();
        entity.setHouseNumbers("1,2,3");
        entity.setHouseRanges("5-10;20-30");
        assertEquals("д. 1, 2, 3, 5-10, 20-30", entity.formatHouses());
    }

    @Test
    void shouldReturnEmptyStringWhenNoHouses() {
        var entity = new AddressEntity();
        assertEquals("", entity.formatHouses());
    }

    @Test
    void shouldHandleNullNumbers() {
        var entity = new AddressEntity();
        entity.setHouseNumbers(null);
        entity.setHouseRanges("1-10");
        assertEquals("д. 1-10", entity.formatHouses());
    }

    @Test
    void shouldHandleNullRanges() {
        var entity = new AddressEntity();
        entity.setHouseNumbers("5,7");
        entity.setHouseRanges(null);
        assertEquals("д. 5, 7", entity.formatHouses());
    }

    @Test
    void shouldHandleSingleNumber() {
        var entity = new AddressEntity();
        entity.setHouseNumbers("42");
        assertEquals("д. 42", entity.formatHouses());
    }

    @Test
    void shouldHandleSingleRange() {
        var entity = new AddressEntity();
        entity.setHouseRanges("1-100");
        assertEquals("д. 1-100", entity.formatHouses());
    }

    @Test
    void shouldHandleEmptyStrings() {
        var entity = new AddressEntity();
        entity.setHouseNumbers("");
        entity.setHouseRanges("");
        assertEquals("", entity.formatHouses());
    }
}
