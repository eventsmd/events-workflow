package md.address.events.workflows;

import io.temporal.spring.boot.ActivityImpl;
import md.address.events.ai.MessageParser;
import md.address.events.domain.MessageReference;
import md.address.events.domain.ParsedMessage;
import md.address.events.domain.TelegramMessage;
import md.address.events.geo.AddressAdapter;
import md.address.events.persistence.AddressEntity;
import md.address.events.persistence.AddressRepository;
import md.address.events.persistence.SubscribtionRepository;
import md.address.events.persistence.TelegramMessageEntity;
import md.address.events.persistence.TelegramMessageId;
import md.address.events.persistence.TelegramMessageRepository;
import md.address.events.persistence.TelegramMessageTranscribeEntity;
import md.address.events.persistence.TelegramMessageTranscribeRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

import java.math.BigInteger;
import java.util.UUID;

@Component
@ActivityImpl(taskQueues = "TelegramMessageQueue")
public class MessageProcessActivity implements MessageProcess {

    Logger log = LoggerFactory.getLogger(MessageProcessActivity.class);

    private final TelegramMessageRepository telegramMessageRepository;
    private final TelegramMessageTranscribeRepository telegramMessageTranscribeRepository;
    private final AddressRepository addressRepository;
    private final MessageParser messageParser;
    private final AddressAdapter addressAdapter;
    private final SubscribtionRepository subscribtionRepository;

    public MessageProcessActivity(TelegramMessageRepository telegramMessageRepository, TelegramMessageTranscribeRepository telegramMessageTranscribeRepository, AddressRepository addressRepository, MessageParser messageParser, AddressAdapter addressAdapter, SubscribtionRepository subscribtionRepository) {
        this.telegramMessageRepository = telegramMessageRepository;
        this.telegramMessageTranscribeRepository = telegramMessageTranscribeRepository;
        this.addressRepository = addressRepository;
        this.messageParser = messageParser;
        this.addressAdapter = addressAdapter;
        this.subscribtionRepository = subscribtionRepository;
    }

    @Override
    public void saveRawMessage(TelegramMessage message) {
        UUID incidentId = UUID.randomUUID();
        if (message.replyTo() != null) {
            incidentId = telegramMessageRepository.findById(new TelegramMessageId(
                    message.replyTo().id(),
                    message.replyTo().chatId()
            )).map(TelegramMessageEntity::getIncidentId)
                    .orElse(UUID.randomUUID());
        }
        telegramMessageRepository.saveAndFlush(TelegramMessageEntity.from(message, incidentId));
    }

    @Override
    public ParsedMessage parseMessage(TelegramMessage message) {
        return new ParsedMessage(
                message,
                messageParser.parse(message.date(), message.text())
        );
    }

    @Override
    public void normalizeAddress(ParsedMessage parsedMessage) {
        if(parsedMessage.messageTranscription().addresses() != null) {
            parsedMessage.messageTranscription().addresses()
                    .forEach(address -> addressRepository.findById(address.id())
                            .ifPresent(addressEntity -> {
                                try {
                                    addressAdapter.enrich(parsedMessage, addressEntity);
                                    addressRepository.saveAndFlush(addressEntity);
                                } catch (Exception e) {
                                    var errorText = "Error on processing address: %s %s".formatted(
                                            addressEntity.getCityName(),
                                            addressEntity.getStreetName());
                                    log.error(errorText, e);
                                }
                            }));
        }
    }

    @Override
    public void saveParsedMessage(ParsedMessage parsedMessage) {
        telegramMessageTranscribeRepository.save(TelegramMessageTranscribeEntity.from(parsedMessage));
        if(parsedMessage.messageTranscription().addresses() != null &&
                !parsedMessage.messageTranscription().addresses().isEmpty()) {
            addressRepository.saveAllAndFlush(parsedMessage.messageTranscription().addresses().stream()
                    .map(m -> AddressEntity.from(
                            m,
                            new MessageReference(
                                    parsedMessage.originalMessage().id(),
                                    parsedMessage.originalMessage().chatId())))
                    .toList());
        }
    }

    @Override
    public void notify(BigInteger id, BigInteger chatId) {

        addressRepository.findByMessageIdAndChatId(id, chatId).forEach(address -> {
            if(address.getStreetKladr() != null) {
                subscribtionRepository.findBySubscribeToKladr(address.getStreetKladr()).forEach(subscription -> {
                    log.info("Notify client {} about address {}", subscription.getTgId(), subscription.getSubscribeToFulltext());
                });
            }
        });
    }
}
