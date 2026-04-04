package md.address.events.persistence;

import io.hypersistence.utils.hibernate.type.basic.PostgreSQLHStoreType;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.IdClass;
import jakarta.persistence.Table;
import md.address.events.domain.MessageReference;
import md.address.events.domain.TelegramMessage;
import md.address.events.domain.User;
import org.hibernate.annotations.Type;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.Map;
import java.util.UUID;

@Entity
@Table(name = "telegram_messages")
@IdClass(TelegramMessageId.class)
public class TelegramMessageEntity {

    @Id
    @Column(name = "id", nullable = false)
    private BigInteger id;

    @Id
    @Column(name = "chat_id", nullable = false)
    private BigInteger chatId;

    @Column(name = "text")
    private String text;

    @Column(name = "date")
    private LocalDateTime date;

    @Column(name = "service_name")
    private String serviceName;

    @Column(name = "from_id")
    private BigInteger fromId;

    @Column(name = "from_name")
    private String fromName;

    @Column(name = "reply_to_id")
    private BigInteger replyToId;

    @Column(name = "reply_to_chat_id")
    private BigInteger replyToChatId;

    @Column(name = "incident_id")
    private UUID incidentId;

    @Type(PostgreSQLHStoreType.class)
    @Column(name = "context", columnDefinition = "hstore")
    private Map<String, String> context;

    public static TelegramMessageEntity from(TelegramMessage message, UUID incidentId) {
        if (message == null) return null;

        TelegramMessageEntity e = new TelegramMessageEntity();
        e.setId(message.id());
        e.setChatId(message.chatId());
        e.setText(message.text());
        e.setDate(message.date());
        e.setServiceName(message.serviceName());
        e.setContext(message.context());

        User from = message.from();
        if (from != null) {
            e.setFromId(from.id());
            e.setFromName(from.name());
        }

        MessageReference reply = message.replyTo();
        if (reply != null) {
            e.setReplyToId(reply.id());
            e.setReplyToChatId(reply.chatId());
        }

        if (incidentId != null) {
            e.setIncidentId(incidentId);
        }

        return e;
    }

    public TelegramMessage toDomain() {
        User from = null;
        if (fromId != null || fromName != null) {
            from = new User(fromId, fromName);
        }

        MessageReference reply = null;
        if (replyToId != null || replyToChatId != null) {
            reply = new MessageReference(replyToId, replyToChatId);
        }

        var supplierName = this.serviceName;

        if (supplierName == null && context != null) supplierName = context.get("supplier");

        return new TelegramMessage(
                id,
                chatId,
                from,
                text,
                date,
                reply,
                supplierName,
                context
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

    public String getText() {
        return text;
    }

    public void setText(String text) {
        this.text = text;
    }

    public LocalDateTime getDate() {
        return date;
    }

    public void setDate(LocalDateTime date) {
        this.date = date;
    }

    public String getServiceName() {
        return serviceName;
    }

    public void setServiceName(String serviceName) {
        this.serviceName = serviceName;
    }

    public BigInteger getFromId() {
        return fromId;
    }

    public void setFromId(BigInteger fromId) {
        this.fromId = fromId;
    }

    public String getFromName() {
        return fromName;
    }

    public void setFromName(String fromName) {
        this.fromName = fromName;
    }

    public BigInteger getReplyToId() {
        return replyToId;
    }

    public void setReplyToId(BigInteger replyToId) {
        this.replyToId = replyToId;
    }

    public BigInteger getReplyToChatId() {
        return replyToChatId;
    }

    public void setReplyToChatId(BigInteger replyToChatId) {
        this.replyToChatId = replyToChatId;
    }

    public UUID getIncidentId() {
        return incidentId;
    }

    public void setIncidentId(UUID incidentId) {
        this.incidentId = incidentId;
    }

    public Map<String, String> getContext() {
        return context;
    }

    public void setContext(Map<String, String> context) {
        this.context = context;
    }
}
