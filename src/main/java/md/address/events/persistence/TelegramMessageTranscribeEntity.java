package md.address.events.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.IdClass;
import jakarta.persistence.Table;
import md.address.events.domain.ParsedMessage;

import java.math.BigInteger;
import java.time.LocalDateTime;

@Entity
@Table(name = "telegram_message_transcribes")
@IdClass(TelegramMessageId.class)
public class TelegramMessageTranscribeEntity {

    @Id
    @Column(name = "id", nullable = false)
    private BigInteger id;
    @Id
    @Column(name = "chatId", nullable = false)
    private BigInteger chatId;
    private String organization;
    private String description;
    private String event;
    @Column(name = "event_start")
    private LocalDateTime eventStart;
    @Column(name = "event_stop")
    private LocalDateTime eventStop;

    public TelegramMessageTranscribeEntity(){}
    public TelegramMessageTranscribeEntity(
            BigInteger id,
            BigInteger chatId,
            String organization,
            String description,
            String event,
            LocalDateTime eventStart,
            LocalDateTime eventStop
    ) {
        this.id = id;
        this.chatId = chatId;
        this.organization = organization;
        this.description = description;
        this.event = event;
        this.eventStart = eventStart;
        this.eventStop = eventStop;
    }

    public static TelegramMessageTranscribeEntity from(ParsedMessage parsedMessage) {
        return new TelegramMessageTranscribeEntity(
                parsedMessage.originalMessage().id(),
                parsedMessage.originalMessage().chatId(),
                parsedMessage.messageTranscription().organization(),
                parsedMessage.messageTranscription().shortDescription(),
                parsedMessage.messageTranscription().event(),
                parsedMessage.messageTranscription().eventStart(),
                parsedMessage.messageTranscription().eventStop()
        );
    }

    public BigInteger getId() {
        return id;
    }

    public void setId(BigInteger id) {
        this.id = id;
    }

    public BigInteger getChatId() {
        return chatId;
    }

    public void setChatId(BigInteger chatId) {
        this.chatId = chatId;
    }

    public String getOrganization() {
        return organization;
    }

    public void setOrganization(String organization) {
        this.organization = organization;
    }

    public String getDescription() {
        return description;
    }

    public void setDescription(String description) {
        this.description = description;
    }

    public String getEvent() {
        return event;
    }

    public void setEvent(String event) {
        this.event = event;
    }

    public LocalDateTime getEventStart() {
        return eventStart;
    }

    public void setEventStart(LocalDateTime eventStart) {
        this.eventStart = eventStart;
    }

    public LocalDateTime getEventStop() {
        return eventStop;
    }

    public void setEventStop(LocalDateTime eventStop) {
        this.eventStop = eventStop;
    }
}
