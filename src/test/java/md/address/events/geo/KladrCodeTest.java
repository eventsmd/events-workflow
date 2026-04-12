package md.address.events.geo;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.CsvSource;
import org.junit.jupiter.params.provider.ValueSource;

import static org.junit.jupiter.api.Assertions.*;

class KladrCodeTest {

    @Test
    void rawReturnsOriginalCode() {
        var code = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        assertEquals("001-03.004-05.048-00.000-71.152", code.raw());
    }

    // --- Level detection ---

    @Test
    void streetLevelWhenAllBlocksNonZero() {
        var code = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        assertEquals(KladrCode.Level.STREET, code.level());
    }

    @Test
    void districtLevelWhenStreetIsZeroAndDistrictNonZero() {
        var code = KladrCode.parse("001-03.004-05.048-02.007-00.000");
        assertEquals(KladrCode.Level.DISTRICT, code.level());
    }

    @Test
    void cityLevelWhenDistrictAndStreetAreZero() {
        var code = KladrCode.parse("001-03.004-05.048-00.000-00.000");
        assertEquals(KladrCode.Level.CITY, code.level());
    }

    @Test
    void regionLevelWhenOnlyCountryAndRegionNonZero() {
        var code = KladrCode.parse("001-05.001-00.000-00.000-00.000");
        assertEquals(KladrCode.Level.REGION, code.level());
    }

    @Test
    void countryLevelWhenAllBlocksAfterCountryAreZero() {
        var code = KladrCode.parse("001-00.000-00.000-00.000-00.000");
        assertEquals(KladrCode.Level.COUNTRY, code.level());
    }

    // --- Prefix extraction (parameterized) ---

    @ParameterizedTest
    @CsvSource({
        "001-03.004-05.048-00.000-71.152, 001-03.004-05.048-00.000-71.152",
        "001-03.004-05.048-02.007-00.000, 001-03.004-05.048-02.007",
        "001-03.004-05.048-00.000-00.000, 001-03.004-05.048",
        "001-05.001-00.000-00.000-00.000, 001-05.001",
        "001-00.000-00.000-00.000-00.000, 001",
    })
    void prefixTrimsTrailingZeroBlocks(String raw, String expectedPrefix) {
        var code = KladrCode.parse(raw);
        assertEquals(expectedPrefix, code.prefix());
    }

    // --- Validation: valid codes ---

    @ParameterizedTest
    @ValueSource(strings = {
        "001-03.004-05.048-00.000-71.152",
        "001-00.000-00.000-00.000-00.000",
        "999-99.999-99.999-99.999-99.999",
        "000-00.000-00.000-00.000-00.000",
    })
    void validCodesAreParsedSuccessfully(String raw) {
        assertDoesNotThrow(() -> KladrCode.parse(raw));
    }

    // --- Validation: invalid codes ---

    @ParameterizedTest
    @ValueSource(strings = {
        "",
        "abc",
        "001-03.004-05.048-00.000",
        "001-03.004-05.048-00.000-71.1521",
        "001_03.004-05.048-00.000-71.152",
        "01-03.004-05.048-00.000-71.152",
        "0011-03.004-05.048-00.000-71.152",
        "001-3.004-05.048-00.000-71.152",
        "001-03.04-05.048-00.000-71.152",
    })
    void invalidCodesThrowIllegalArgument(String raw) {
        assertThrows(IllegalArgumentException.class, () -> KladrCode.parse(raw));
    }

    @Test
    void nullCodeThrowsException() {
        assertThrows(IllegalArgumentException.class, () -> KladrCode.parse(null));
    }

    // --- Equality ---

    @Test
    void equalCodesAreEqual() {
        var a = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        var b = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        assertEquals(a, b);
        assertEquals(a.hashCode(), b.hashCode());
    }

    @Test
    void differentCodesAreNotEqual() {
        var a = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        var b = KladrCode.parse("001-03.004-05.048-00.000-00.000");
        assertNotEquals(a, b);
    }

    @Test
    void toStringContainsRawCode() {
        var code = KladrCode.parse("001-03.004-05.048-00.000-71.152");
        assertTrue(code.toString().contains("001-03.004-05.048-00.000-71.152"));
    }
}
