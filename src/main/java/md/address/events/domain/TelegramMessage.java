package md.address.events.domain;

import com.fasterxml.jackson.annotation.JsonProperty;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.Map;

public record TelegramMessage(
        BigInteger id,
        @JsonProperty("chat_id")
        BigInteger chatId,
        User from,
        String text,
        LocalDateTime date,
        @JsonProperty("reply_to")
        MessageReference replyTo,
        @JsonProperty("service_name")
        String serviceName,
        Map<String, String> context
) {
}
