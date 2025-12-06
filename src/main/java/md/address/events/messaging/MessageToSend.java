package md.address.events.messaging;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigInteger;

public record MessageToSend(
        @JsonProperty("telegram_id")
        BigInteger telegramId,
        String message,
        String topic
) {
}
