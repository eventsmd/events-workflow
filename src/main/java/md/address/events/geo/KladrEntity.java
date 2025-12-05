package md.address.events.geo;

import com.fasterxml.jackson.annotation.JsonProperty;

public record KladrEntity(
        String kladr,
        String name,
        @JsonProperty("name_with_type")
        String nameWithType,
        String type,
        @JsonProperty("type_full")
        String typeFull
) {
}
