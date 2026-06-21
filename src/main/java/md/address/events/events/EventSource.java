package md.address.events.events;

import com.fasterxml.jackson.annotation.JsonInclude;

import java.math.BigInteger;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record EventSource(BigInteger messageId, BigInteger chatId) {
}
