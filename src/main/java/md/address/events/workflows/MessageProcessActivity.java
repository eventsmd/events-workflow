package md.address.events.workflows;

import io.temporal.spring.boot.ActivityImpl;
import md.address.events.ai.MessageParser;
import md.address.events.domain.MessageReference;
import md.address.events.domain.ParsedMessage;
import md.address.events.domain.TelegramMessage;
import md.address.events.events.EventAddress;
import md.address.events.events.EventSource;
import md.address.events.events.KladrRef;
import md.address.events.events.NatsEventPublisher;
import md.address.events.events.UtilityEvent;
import md.address.events.geo.AddressAdapter;
import md.address.events.geo.KladrCode;
import md.address.events.messaging.MessageSender;
import md.address.events.messaging.MessageToSend;
import md.address.events.persistence.AddressEntity;
import md.address.events.persistence.AddressRepository;
import md.address.events.persistence.SubscriptionRepository;
import md.address.events.persistence.TelegramMessageEntity;
import md.address.events.persistence.TelegramMessageId;
import md.address.events.persistence.TelegramMessageRepository;
import md.address.events.persistence.TelegramMessageTranscribeEntity;
import md.address.events.persistence.TelegramMessageTranscribeRepository;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Component;

import java.math.BigInteger;
import java.time.Instant;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.List;
import java.util.UUID;

@Component
@ActivityImpl(taskQueues = "TelegramMessageQueue")
public class MessageProcessActivity implements MessageProcess {

    private final MessageSender messageSender;
    private static final Logger log = LoggerFactory.getLogger(MessageProcessActivity.class);
    private static final DateTimeFormatter DATETIME_FORMAT = DateTimeFormatter.ofPattern("dd-MM-yyyy HH:mm");

    private final TelegramMessageRepository telegramMessageRepository;
    private final TelegramMessageTranscribeRepository telegramMessageTranscribeRepository;
    private final AddressRepository addressRepository;
    private final MessageParser messageParser;
    private final AddressAdapter addressAdapter;
    private final SubscriptionRepository subscriptionRepository;
    private final NatsEventPublisher natsEventPublisher;

    public MessageProcessActivity(
            TelegramMessageRepository telegramMessageRepository,
            TelegramMessageTranscribeRepository telegramMessageTranscribeRepository,
            AddressRepository addressRepository,
            MessageParser messageParser,
            AddressAdapter addressAdapter,
            SubscriptionRepository subscriptionRepository,
            MessageSender messageSender,
            NatsEventPublisher natsEventPublisher) {
        this.telegramMessageRepository = telegramMessageRepository;
        this.telegramMessageTranscribeRepository = telegramMessageTranscribeRepository;
        this.addressRepository = addressRepository;
        this.messageParser = messageParser;
        this.addressAdapter = addressAdapter;
        this.subscriptionRepository = subscriptionRepository;
        this.messageSender = messageSender;
        this.natsEventPublisher = natsEventPublisher;
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

        addressRepository.findByMessageIdAndChatId(id, chatId).forEach(address ->
                telegramMessageTranscribeRepository.findById(new TelegramMessageId(id, chatId))
                    .ifPresent(messageTranscribe ->
                        telegramMessageRepository.findById(new TelegramMessageId(id, chatId)).ifPresent(
                                message -> {
                            var kladrRaw = address.getStreetKladr() != null ? address.getStreetKladr()
                                    : address.getCityKladr() != null ? address.getCityKladr()
                                    : address.getRegionKladr();
                            if (kladrRaw != null) {
                                var prefix = KladrCode.parse(kladrRaw).prefix();
                                subscriptionRepository.findBySubscribeToKladrStartingWith(prefix).forEach(
                                        subscription -> {

                                    var supplier = message.getContext().get("supplier");
                                    var serviceEmoji = switch (supplier) {
                                        case "water" -> "💧";
                                        case "electricity" -> "⚡️";
                                        case null, default -> "";
                                    };

                                    var serviceName = switch (supplier) {
                                        case "water" -> "водоснабжения";
                                        case "electricity" -> "электроснабжения";
                                        case null, default -> "";
                                    };

                                    var eventDescription = switch (messageTranscribe.getEvent()) {
                                        case "shutdown" -> "Отключение";
                                        case "resume" -> "Возобновление";
                                        case null, default -> "";
                                    };

                                    var houses = address.formatHouses();
                                    var addressText = houses.isEmpty()
                                            ? subscription.getSubscribeToFulltext()
                                            : "%s, %s".formatted(subscription.getSubscribeToFulltext(), houses);

                                    var messageText = "%s %s услуги %s по адресу «%s» с %s%n%n%s".formatted(
                                            serviceEmoji,
                                            eventDescription,
                                            serviceName,
                                            addressText,
                                            DATETIME_FORMAT.format(messageTranscribe.getEventStart()),
                                            messageTranscribe.getDescription()
                                    );

                                    messageSender.sendMessage(new MessageToSend(
                                            new BigInteger(subscription.getTgId()),
                                            messageText,
                                            supplier,
                                            id,
                                            chatId
                                    ));

                                    log.info("Notify client {} about address {}",
                                            subscription.getTgId(),
                                            subscription.getSubscribeToFulltext());
                                });
                            }
                    })));
    }

    @Override
    public void publishEvent(BigInteger id, BigInteger chatId) {
        var messageOpt = telegramMessageRepository.findById(new TelegramMessageId(id, chatId));
        var transcribeOpt = telegramMessageTranscribeRepository.findById(new TelegramMessageId(id, chatId));
        if (messageOpt.isEmpty() || transcribeOpt.isEmpty()) {
            log.info("Skip publishEvent for {}:{} — message or transcript missing", id, chatId);
            return;
        }
        var message = messageOpt.get();
        var transcribe = transcribeOpt.get();
        var supplier = message.getContext() != null ? message.getContext().get("supplier") : null;

        var addresses = addressRepository.findByMessageIdAndChatId(id, chatId).stream()
                .map(this::toEventAddress)
                .toList();

        var event = new UtilityEvent(
                message.getIncidentId() != null ? message.getIncidentId().toString() : null,
                supplier,
                transcribe.getEvent(),
                transcribe.getOrganization(),
                transcribe.getDescription(),
                transcribe.getEventStart(),
                transcribe.getEventStop(),
                Instant.now(),
                new EventSource(id, chatId),
                addresses
        );
        natsEventPublisher.publish(event);
    }

    private EventAddress toEventAddress(AddressEntity a) {
        return new EventAddress(
                kladrRef(a.getRegionName(), a.getRegionKladr(), a.getRegionType()),
                kladrRef(a.getCityName(), a.getCityKladr(), a.getCityType()),
                kladrRef(a.getStreetName(), a.getStreetKladr(), a.getStreetType()),
                houses(a)
        );
    }

    private KladrRef kladrRef(String name, String kladr, String type) {
        if (name == null && kladr == null && type == null) return null;
        return new KladrRef(name, kladr, type);
    }

    private List<String> houses(AddressEntity a) {
        var parts = new ArrayList<String>();
        if (a.getHouseNumbers() != null && !a.getHouseNumbers().isBlank()) {
            parts.addAll(List.of(a.getHouseNumbers().split(",")));
        }
        if (a.getHouseRanges() != null && !a.getHouseRanges().isBlank()) {
            parts.addAll(List.of(a.getHouseRanges().split(";")));
        }
        return parts;
    }
}
