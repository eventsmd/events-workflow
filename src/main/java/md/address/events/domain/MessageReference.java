package md.address.events.domain;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigInteger;

public record MessageReference(
        BigInteger id,
        @JsonProperty("chat_id")
        BigInteger chatId
) {
}
