package md.address.events.persistence;

import java.io.Serializable;
import java.math.BigInteger;
import java.util.Objects;

public class TelegramMessageId implements Serializable {
    private BigInteger id;
    private BigInteger chatId;

    public TelegramMessageId() {
    }

    public TelegramMessageId(BigInteger id, BigInteger chatId) {
        this.id = id;
        this.chatId = chatId;
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        TelegramMessageId that = (TelegramMessageId) o;
        return Objects.equals(id, that.id) && Objects.equals(chatId, that.chatId);
    }

    @Override
    public int hashCode() {
        return Objects.hash(id, chatId);
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
}
