package md.address.events.persistence;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;

import java.time.LocalDateTime;
import java.util.UUID;

@Entity
@Table(name = "subscriptions")
public class Subscription {
    @Id
    @Column(name = "id", nullable = false)
    private UUID id;

    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt;

    @Column(name = "subscribe_to_kladr", nullable = false, length = Integer.MAX_VALUE)
    private String subscribeToKladr;

    @Column(name = "tg_id", nullable = false, length = Integer.MAX_VALUE)
    private String tgId;

    @Column(name = "subscribe_to_fulltext", nullable = false, length = Integer.MAX_VALUE)
    private String subscribeToFulltext;

    public UUID getId() {
        return id;
    }

    public void setId(UUID id) {
        this.id = id;
    }

    public LocalDateTime getCreatedAt() {
        return createdAt;
    }

    public void setCreatedAt(LocalDateTime createdAt) {
        this.createdAt = createdAt;
    }

    public String getSubscribeToKladr() {
        return subscribeToKladr;
    }

    public void setSubscribeToKladr(String subscribeToKladr) {
        this.subscribeToKladr = subscribeToKladr;
    }

    public String getTgId() {
        return tgId;
    }

    public void setTgId(String tgId) {
        this.tgId = tgId;
    }

    public String getSubscribeToFulltext() {
        return subscribeToFulltext;
    }

    public void setSubscribeToFulltext(String subscribeToFulltext) {
        this.subscribeToFulltext = subscribeToFulltext;
    }
}