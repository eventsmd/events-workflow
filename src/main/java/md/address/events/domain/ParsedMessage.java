package md.address.events.domain;

public record ParsedMessage(
        TelegramMessage originalMessage,
        MessageTranscription messageTranscription
) {
}
