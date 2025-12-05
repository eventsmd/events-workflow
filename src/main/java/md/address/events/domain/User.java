package md.address.events.domain;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigInteger;

public record User(
        BigInteger id,
        @JsonProperty("name")
        String name
) {
}
