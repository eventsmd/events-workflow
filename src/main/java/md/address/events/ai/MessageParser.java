package md.address.events.ai;

import com.fasterxml.jackson.databind.ObjectMapper;
import md.address.events.domain.MessageTranscription;
import org.springframework.ai.chat.model.ChatModel;
import org.springframework.ai.chat.model.ChatResponse;
import org.springframework.ai.chat.prompt.Prompt;
import org.springframework.ai.openai.OpenAiChatOptions;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;

@Component
public class MessageParser {

    private final ChatModel chatModel;
    private final ObjectMapper objectMapper;

    public MessageParser(ChatModel chatModel, ObjectMapper objectMapper) {
        this.chatModel = chatModel;
        this.objectMapper = objectMapper;
    }

    public MessageTranscription parse(LocalDateTime time, String message) {

        var prompt = "Given a text message at %s describing a supply event, generate a JSON object according to the following schema: {\"type\":\"object\",\"properties\":{\"organization\":{\"type\":\"string\"},\"short_description\":{\"type\":\"string\"},\"event\":{\"type\":\"string\",\"enum\":[\"shutdown\",\"resume\", \"other\"]},\"event_start\":{\"type\":\"string\",\"format\":\"iso date-time without tz and seconds yyyy-MM-dd'T'HH:mm\"},\"event_stop\":{\"type\":\"string\",\"format\":\"iso date-time without tz and seconds yyyy-MM-dd'T'HH:mm\"},\"addresses\":{\"type\":\"array\",\"items\":{\"type\":\"object\",\"properties\":{\"city\":{\"type\":\"string\"},\"street_type\":{\"type\":\"string\",\"enum\":[\"ул.\",\"пер.\",\"пл.\"]},\"street\":{\"type\":\"string\"},\"house\":{\"type\":\"object\",\"properties\":{\"numbers\":{\"type\":\"array\",\"items\":{\"type\":\"string\"}},\"ranges\":{\"type\":\"array\",\"items\":{\"type\":\"array\",\"items\":{\"type\":\"string\"}}}}}}}}}} The JSON output must accurately reflect the details from the message. Response - ONLY JSON. Ignore named places, we need only addresses. Message starts with organization name usually. Be accurate with addresses! Message could have no address. In case you can't recognize what has happened - return just null.";

        ChatResponse response = chatModel.call(
                new Prompt(
                        prompt + """
                        
                        Message at %s:
                        %s""".formatted(time, message),
                        OpenAiChatOptions.builder()
                                .model("gpt-5-mini")
                                .temperature(1.0)
                                .build()
                ));
        var responseText = response.getResult().getOutput().getText().replaceAll("```(?:json)?", "");
        try {
            return objectMapper.readValue(responseText, MessageTranscription.class);
        } catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
