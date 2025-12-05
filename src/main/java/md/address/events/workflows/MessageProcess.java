package md.address.events.workflows;

import io.temporal.activity.ActivityInterface;
import md.address.events.domain.ParsedMessage;
import md.address.events.domain.TelegramMessage;

import java.math.BigInteger;

@ActivityInterface
public interface MessageProcess {

    void saveRawMessage(TelegramMessage message);
    ParsedMessage parseMessage(TelegramMessage message);
    void normalizeAddress(ParsedMessage parsedMessage);
    void saveParsedMessage(ParsedMessage parsedMessage);
    void notify(BigInteger id, BigInteger chatId);
}
