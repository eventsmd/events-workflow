package md.address.events.geo;

import java.util.regex.Pattern;

/**
 * Value object representing a KLADR code.
 * <p>
 * Format: {@code CCC-TT.RRR-TT.CCC-TT.DDD-TT.SSS}
 * <ul>
 *   <li>CCC — country (3 digits)</li>
 *   <li>TT.RRR — region (type.id)</li>
 *   <li>TT.CCC — city (type.id)</li>
 *   <li>TT.DDD — district (type.id)</li>
 *   <li>TT.SSS — street (type.id)</li>
 * </ul>
 * A zero block {@code 00.000} means that level is not specified.
 */
public record KladrCode(String raw) {

    private static final Pattern FORMAT = Pattern.compile(
            "\\d{3}(-\\d{2}\\.\\d{3}){4}"
    );

    private static final String ZERO_BLOCK = "00.000";

    public enum Level {
        COUNTRY, REGION, CITY, DISTRICT, STREET
    }

    public KladrCode {
        if (raw == null || !FORMAT.matcher(raw).matches()) {
            throw new IllegalArgumentException("Invalid KLADR code: " + raw);
        }
    }

    public static KladrCode parse(String raw) {
        return new KladrCode(raw);
    }

    /**
     * Determines the deepest non-zero level of this KLADR code.
     */
    public Level level() {
        String[] blocks = splitBlocks();
        // blocks: [country, region, city, district, street]
        if (!ZERO_BLOCK.equals(blocks[4])) return Level.STREET;
        if (!ZERO_BLOCK.equals(blocks[3])) return Level.DISTRICT;
        if (!ZERO_BLOCK.equals(blocks[2])) return Level.CITY;
        if (!ZERO_BLOCK.equals(blocks[1])) return Level.REGION;
        return Level.COUNTRY;
    }

    /**
     * Returns the code trimmed of trailing {@code -00.000} blocks.
     */
    public String prefix() {
        String[] blocks = splitBlocks();
        int last = 0;
        for (int i = 1; i < blocks.length; i++) {
            if (!ZERO_BLOCK.equals(blocks[i])) {
                last = i;
            }
        }
        var sb = new StringBuilder(blocks[0]);
        for (int i = 1; i <= last; i++) {
            sb.append('-').append(blocks[i]);
        }
        return sb.toString();
    }

    /**
     * Splits the raw code into 5 blocks: country, region, city, district, street.
     */
    private String[] splitBlocks() {
        // Format: CCC-TT.RRR-TT.CCC-TT.DDD-TT.SSS
        // Positions: 0-2  4-9  11-16  18-23  25-30
        return new String[] {
                raw.substring(0, 3),   // country
                raw.substring(4, 10),  // region  TT.RRR
                raw.substring(11, 17), // city    TT.CCC
                raw.substring(18, 24), // district TT.DDD
                raw.substring(25, 31), // street  TT.SSS
        };
    }
}
