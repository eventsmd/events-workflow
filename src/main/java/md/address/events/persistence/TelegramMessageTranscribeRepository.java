package md.address.events.persistence;

import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

@Repository
public interface TelegramMessageTranscribeRepository extends
        JpaRepository<TelegramMessageTranscribeEntity, TelegramMessageId> {
}
