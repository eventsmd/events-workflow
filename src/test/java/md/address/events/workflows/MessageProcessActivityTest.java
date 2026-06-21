package md.address.events.workflows;

import md.address.events.ai.MessageParser;
import md.address.events.domain.*;
import md.address.events.events.NatsEventPublisher;
import md.address.events.geo.AddressAdapter;
import md.address.events.messaging.MessageSender;
import md.address.events.persistence.*;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.math.BigInteger;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.*;

import md.address.events.messaging.MessageToSend;
import org.mockito.ArgumentCaptor;

@ExtendWith(MockitoExtension.class)
class MessageProcessActivityTest {

    @Mock private TelegramMessageRepository telegramMessageRepository;
    @Mock private TelegramMessageTranscribeRepository transcribeRepository;
    @Mock private AddressRepository addressRepository;
    @Mock private MessageParser messageParser;
    @Mock private AddressAdapter addressAdapter;
    @Mock private SubscriptionRepository subscriptionRepository;
    @Mock private MessageSender messageSender;
    @Mock private NatsEventPublisher natsEventPublisher;

    private MessageProcessActivity activity;

    @BeforeEach
    void setUp() {
        activity = new MessageProcessActivity(
                telegramMessageRepository,
                transcribeRepository,
                addressRepository,
                messageParser,
                addressAdapter,
                subscriptionRepository,
                messageSender,
                natsEventPublisher
        );
    }

    @Test
    void saveRawMessageShouldGenerateNewIncidentIdForNonReply() {
        var message = createTestMessage(null);

        activity.saveRawMessage(message);

        verify(telegramMessageRepository).saveAndFlush(any(TelegramMessageEntity.class));
        verify(telegramMessageRepository, never()).findById(any());
    }

    @Test
    void saveRawMessageShouldLookupIncidentIdForReply() {
        var replyTo = new MessageReference(BigInteger.TWO, BigInteger.TEN);
        var message = createTestMessage(replyTo);
        var existingEntity = new TelegramMessageEntity();
        existingEntity.setIncidentId(UUID.randomUUID());

        when(telegramMessageRepository.findById(new TelegramMessageId(BigInteger.TWO, BigInteger.TEN)))
                .thenReturn(Optional.of(existingEntity));

        activity.saveRawMessage(message);

        verify(telegramMessageRepository).findById(new TelegramMessageId(BigInteger.TWO, BigInteger.TEN));
        verify(telegramMessageRepository).saveAndFlush(any(TelegramMessageEntity.class));
    }

    @Test
    void saveRawMessageShouldGenerateNewIncidentIdWhenReplyNotFound() {
        var replyTo = new MessageReference(BigInteger.TWO, BigInteger.TEN);
        var message = createTestMessage(replyTo);

        when(telegramMessageRepository.findById(any())).thenReturn(Optional.empty());

        activity.saveRawMessage(message);

        verify(telegramMessageRepository).saveAndFlush(any(TelegramMessageEntity.class));
    }

    @Test
    void parseMessageShouldDelegateToMessageParser() {
        var message = createTestMessage(null);
        var transcription = new MessageTranscription(
                "org", "desc", "shutdown",
                LocalDateTime.now(), null, null
        );

        when(messageParser.parse(message.date(), message.text())).thenReturn(transcription);

        var result = activity.parseMessage(message);

        assertEquals(message, result.originalMessage());
        assertEquals(transcription, result.messageTranscription());
    }

    @Test
    void saveParsedMessageShouldSaveTranscribeAndAddresses() {
        var addressId = UUID.randomUUID();
        var address = new Address(addressId, "Кишинёв", "Пушкина", "ул.", null);
        var transcription = new MessageTranscription(
                "org", "desc", "shutdown",
                LocalDateTime.now(), null, List.of(address)
        );
        var message = createTestMessage(null);
        var parsed = new ParsedMessage(message, transcription);

        activity.saveParsedMessage(parsed);

        verify(transcribeRepository).save(any(TelegramMessageTranscribeEntity.class));
        verify(addressRepository).saveAllAndFlush(anyList());
    }

    @Test
    void saveParsedMessageShouldSkipAddressSaveWhenNull() {
        var transcription = new MessageTranscription(
                "org", "desc", "shutdown",
                LocalDateTime.now(), null, null
        );
        var parsed = new ParsedMessage(createTestMessage(null), transcription);

        activity.saveParsedMessage(parsed);

        verify(transcribeRepository).save(any());
        verify(addressRepository, never()).saveAllAndFlush(any());
    }

    @Test
    void normalizeAddressShouldEnrichEachAddress() {
        var addressId = UUID.randomUUID();
        var address = new Address(addressId, "Кишинёв", "Пушкина", "ул.", null);
        var transcription = new MessageTranscription(
                "org", "desc", "shutdown", null, null, List.of(address)
        );
        var message = createTestMessage(null);
        var parsed = new ParsedMessage(message, transcription);

        var addressEntity = new AddressEntity();
        addressEntity.setCityOriginal("Кишинёв");
        addressEntity.setStreetOriginal("Пушкина");
        when(addressRepository.findById(addressId)).thenReturn(Optional.of(addressEntity));

        activity.normalizeAddress(parsed);

        verify(addressAdapter).enrich(parsed, addressEntity);
        verify(addressRepository).saveAndFlush(addressEntity);
    }

    @Test
    void normalizeAddressShouldSkipWhenAddressesNull() {
        var transcription = new MessageTranscription("org", "desc", "shutdown", null, null, null);
        var parsed = new ParsedMessage(createTestMessage(null), transcription);

        activity.normalizeAddress(parsed);

        verify(addressAdapter, never()).enrich(any(), any());
    }

    @Test
    void normalizeAddressShouldLogErrorWhenEnrichFails() {
        var addressId = UUID.randomUUID();
        var address = new Address(addressId, "Кишинёв", "Пушкина", "ул.", null);
        var transcription = new MessageTranscription("org", "desc", "shutdown", null, null, List.of(address));
        var parsed = new ParsedMessage(createTestMessage(null), transcription);

        var addressEntity = new AddressEntity();
        addressEntity.setCityName("Кишинёв");
        addressEntity.setStreetName("Пушкина");
        when(addressRepository.findById(addressId)).thenReturn(Optional.of(addressEntity));
        doThrow(new RuntimeException("API error")).when(addressAdapter).enrich(any(), any());

        // Should not throw, error is caught and logged
        assertDoesNotThrow(() -> activity.normalizeAddress(parsed));
    }

    @Test
    void notifyShouldSendMessageToSubscribers() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);

        var addressEntity = new AddressEntity();
        addressEntity.setStreetKladr("001-03.004-05.048-07.012-09.456");
        when(addressRepository.findByMessageIdAndChatId(id, chatId))
                .thenReturn(List.of(addressEntity));

        var transcribe = new TelegramMessageTranscribeEntity(
                id, chatId, "SA Apă-Canal", "Отключение воды", "shutdown",
                LocalDateTime.of(2025, 12, 1, 10, 0),
                LocalDateTime.of(2025, 12, 1, 18, 0)
        );
        when(transcribeRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(transcribe));

        var messageEntity = new TelegramMessageEntity();
        messageEntity.setContext(Map.of("supplier", "water"));
        when(telegramMessageRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(messageEntity));

        var subscription = new Subscription();
        subscription.setTgId("12345");
        subscription.setSubscribeToFulltext("Кишинёв, ул. Пушкина");
        subscription.setSubscribeToKladr("001-03.004-05.048-07.012-09.456");
        when(subscriptionRepository.findBySubscribeToKladrStartingWith("001-03.004-05.048-07.012-09.456"))
                .thenReturn(List.of(subscription));

        activity.notify(id, chatId);

        var captor = ArgumentCaptor.forClass(MessageToSend.class);
        verify(messageSender).sendMessage(captor.capture());

        var sent = captor.getValue();
        assertEquals(new BigInteger("12345"), sent.telegramId());
        assertEquals("water", sent.topic());
        assertEquals(id, sent.messageId(), "message_id should be passed in the SQS message");
        assertEquals(chatId, sent.chatId(), "chat_id should be passed in the SQS message");
        assertTrue(sent.message().contains("Отключение"));
        assertTrue(sent.message().contains("водоснабжения"));
    }

    @Test
    void notifyShouldSkipWhenNoAddresses() {
        when(addressRepository.findByMessageIdAndChatId(any(), any()))
                .thenReturn(List.of());

        activity.notify(BigInteger.ONE, BigInteger.TWO);

        verify(messageSender, never()).sendMessage(any());
    }

    @Test
    void notifyShouldSkipWhenAllKladrFieldsNull() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);

        var addressEntity = new AddressEntity();
        addressEntity.setStreetKladr(null);
        addressEntity.setCityKladr(null);
        addressEntity.setRegionKladr(null);
        when(addressRepository.findByMessageIdAndChatId(id, chatId))
                .thenReturn(List.of(addressEntity));

        var transcribe = new TelegramMessageTranscribeEntity(
                id, chatId, "org", "desc", "shutdown",
                LocalDateTime.now(), null
        );
        when(transcribeRepository.findById(any())).thenReturn(Optional.of(transcribe));

        var messageEntity = new TelegramMessageEntity();
        messageEntity.setContext(Map.of("supplier", "water"));
        when(telegramMessageRepository.findById(any())).thenReturn(Optional.of(messageEntity));

        activity.notify(id, chatId);

        verify(subscriptionRepository, never()).findBySubscribeToKladrStartingWith(any());
        verify(messageSender, never()).sendMessage(any());
    }

    @Test
    void notifyShouldMatchSubscribersByCityKladrWhenStreetKladrNull() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);

        var addressEntity = new AddressEntity();
        addressEntity.setStreetKladr(null);
        addressEntity.setCityKladr("001-03.004-05.048-00.000-00.000");
        when(addressRepository.findByMessageIdAndChatId(id, chatId))
                .thenReturn(List.of(addressEntity));

        var transcribe = new TelegramMessageTranscribeEntity(
                id, chatId, "SA Apă-Canal", "Отключение воды", "shutdown",
                LocalDateTime.of(2025, 12, 1, 10, 0),
                LocalDateTime.of(2025, 12, 1, 18, 0)
        );
        when(transcribeRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(transcribe));

        var messageEntity = new TelegramMessageEntity();
        messageEntity.setContext(Map.of("supplier", "water"));
        when(telegramMessageRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(messageEntity));

        var subscription = new Subscription();
        subscription.setTgId("12345");
        subscription.setSubscribeToFulltext("Кишинёв");
        subscription.setSubscribeToKladr("001-03.004-05.048-00.000-00.000");
        when(subscriptionRepository.findBySubscribeToKladrStartingWith("001-03.004-05.048"))
                .thenReturn(List.of(subscription));

        activity.notify(id, chatId);

        verify(messageSender).sendMessage(any(MessageToSend.class));
    }

    @Test
    void notifyShouldFallbackToRegionKladrWhenCityAndStreetNull() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);

        var addressEntity = new AddressEntity();
        addressEntity.setStreetKladr(null);
        addressEntity.setCityKladr(null);
        addressEntity.setRegionKladr("001-05.001-00.000-00.000-00.000");
        when(addressRepository.findByMessageIdAndChatId(id, chatId))
                .thenReturn(List.of(addressEntity));

        var transcribe = new TelegramMessageTranscribeEntity(
                id, chatId, "SA Apă-Canal", "Отключение воды", "shutdown",
                LocalDateTime.of(2025, 12, 1, 10, 0),
                LocalDateTime.of(2025, 12, 1, 18, 0)
        );
        when(transcribeRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(transcribe));

        var messageEntity = new TelegramMessageEntity();
        messageEntity.setContext(Map.of("supplier", "water"));
        when(telegramMessageRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(messageEntity));

        var subscription = new Subscription();
        subscription.setTgId("12345");
        subscription.setSubscribeToFulltext("Район");
        subscription.setSubscribeToKladr("001-05.001-00.000-00.000-00.000");
        when(subscriptionRepository.findBySubscribeToKladrStartingWith("001-05.001"))
                .thenReturn(List.of(subscription));

        activity.notify(id, chatId);

        verify(messageSender).sendMessage(any(MessageToSend.class));
    }

    @Test
    void publishEventShouldPublishMappedUtilityEvent() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);

        var incidentId = UUID.randomUUID();
        var messageEntity = new TelegramMessageEntity();
        messageEntity.setIncidentId(incidentId);
        messageEntity.setContext(Map.of("supplier", "water"));
        when(telegramMessageRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(messageEntity));

        var transcribe = new TelegramMessageTranscribeEntity(
                id, chatId, "SA Apă-Canal", "Отключение воды", "shutdown",
                LocalDateTime.of(2026, 6, 21, 10, 0), LocalDateTime.of(2026, 6, 21, 18, 0));
        when(transcribeRepository.findById(new TelegramMessageId(id, chatId)))
                .thenReturn(Optional.of(transcribe));

        var addressEntity = new AddressEntity();
        addressEntity.setStreetName("Пушкина");
        addressEntity.setStreetKladr("001-01.001");
        addressEntity.setStreetType("ул");
        addressEntity.setHouseNumbers("1,3");
        addressEntity.setHouseRanges("5-9");
        when(addressRepository.findByMessageIdAndChatId(id, chatId))
                .thenReturn(List.of(addressEntity));

        activity.publishEvent(id, chatId);

        var captor = org.mockito.ArgumentCaptor.forClass(md.address.events.events.UtilityEvent.class);
        verify(natsEventPublisher).publish(captor.capture());
        var published = captor.getValue();
        assertEquals(incidentId.toString(), published.incidentId());
        assertEquals("water", published.supplier());
        assertEquals("shutdown", published.event());
        assertEquals(id, published.source().messageId());
        assertEquals(1, published.addresses().size());
        assertEquals(List.of("1", "3", "5-9"), published.addresses().get(0).houses());
        assertEquals("001-01.001", published.addresses().get(0).street().kladr());
    }

    @Test
    void publishEventShouldSkipWhenTranscribeMissing() {
        var id = BigInteger.valueOf(100);
        var chatId = BigInteger.valueOf(200);
        when(telegramMessageRepository.findById(any())).thenReturn(Optional.of(new TelegramMessageEntity()));
        when(transcribeRepository.findById(any())).thenReturn(Optional.empty());

        activity.publishEvent(id, chatId);

        verify(natsEventPublisher, never()).publish(any());
    }

    @Test
    void publishEventShouldSkipWhenMessageMissing() {
        when(telegramMessageRepository.findById(any())).thenReturn(Optional.empty());

        activity.publishEvent(BigInteger.valueOf(100), BigInteger.valueOf(200));

        verify(natsEventPublisher, never()).publish(any());
    }

    private TelegramMessage createTestMessage(MessageReference replyTo) {
        return new TelegramMessage(
                BigInteger.valueOf(100), BigInteger.valueOf(200),
                new User(BigInteger.ONE, "TestUser"),
                "Test message text", LocalDateTime.of(2025, 12, 1, 10, 0),
                replyTo, "water", Map.of("supplier", "water")
        );
    }
}
