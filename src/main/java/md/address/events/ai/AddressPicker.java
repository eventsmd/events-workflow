package md.address.events.ai;

import md.address.events.domain.ParsedMessage;
import md.address.events.geo.AddressKladr;
import md.address.events.persistence.AddressEntity;
import org.springframework.ai.chat.model.ChatModel;
import org.springframework.ai.chat.model.ChatResponse;
import org.springframework.ai.chat.prompt.Prompt;
import org.springframework.ai.openai.OpenAiChatOptions;
import org.springframework.stereotype.Component;

import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;

@Component
public class AddressPicker {

    private final ChatModel chatModel;

    public AddressPicker(ChatModel chatModel) {
        this.chatModel = chatModel;
    }

    public AddressKladr pickAddress(
            ParsedMessage parsedMessage,
            AddressEntity entity,
            List<AddressKladr> addressKladrList
    ) {

        AtomicInteger counter = new AtomicInteger(0);

        var addresses = addressKladrList.stream()
                .map(k ->
                    "%d. %s%n".formatted(counter.getAndIncrement(), k.fullAddress())
                ).collect(StringBuilder::new, StringBuilder::append, StringBuilder::append).toString();

        var prompt = "Select the address variant that best matches the original address based on semantic similarity and context of request. The original address is: %n'%s'. The variants are: %s . Respond only with the number of the most similar variant, in int64 format!%nIt was fetched from the message: (%s) %s"
                .formatted(
                        "%s %s".formatted(entity.getCityOriginal(), entity.getStreetOriginal()),
                        addresses,
                        parsedMessage.originalMessage().from().name(),
                        parsedMessage.originalMessage().text()
                );

        ChatResponse response = chatModel.call(
                new Prompt(
                        prompt,
                        OpenAiChatOptions.builder()
                            .model("gpt-5-mini")
                            .temperature(1.0)
                            .build())
        );

        var index = Integer.parseInt(response.getResult().getOutput().getText().trim());
        return addressKladrList.get(index);
    }
}
