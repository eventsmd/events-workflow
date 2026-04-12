package md.address.events;

import io.hypersistence.utils.hibernate.type.basic.PostgreSQLHStoreType;
import io.temporal.client.ActivityCompletionClient;
import io.temporal.client.BatchRequest;
import io.temporal.client.WorkflowClient;
import io.temporal.client.WorkflowStub;
import io.temporal.client.WorkflowUpdateHandle;
import io.temporal.client.schedules.ScheduleClient;
import io.temporal.client.schedules.ScheduleHandle;
import io.temporal.serviceclient.WorkflowServiceStubs;
import io.temporal.spring.boot.ActivityImpl;
import io.temporal.spring.boot.WorkflowImpl;
import md.address.events.domain.Address;
import md.address.events.domain.House;
import md.address.events.domain.MessageReference;
import md.address.events.domain.MessageTranscription;
import md.address.events.domain.ParsedMessage;
import md.address.events.domain.TelegramMessage;
import md.address.events.domain.User;
import md.address.events.geo.AddressKladr;
import md.address.events.geo.KladrCode;
import md.address.events.geo.KladrEntity;
import md.address.events.messaging.MessageToSend;
import md.address.events.persistence.AddressEntity;
import md.address.events.persistence.Subscription;
import md.address.events.persistence.TelegramMessageEntity;
import md.address.events.persistence.TelegramMessageId;
import md.address.events.persistence.TelegramMessageTranscribeEntity;
import md.address.events.workflows.MessageProcess;
import md.address.events.workflows.MessageProcessActivity;
import md.address.events.workflows.TelegramMessageProcess;
import md.address.events.workflows.TelegramMessageProcessWorkflow;
import org.springframework.aot.hint.MemberCategory;
import org.springframework.aot.hint.RuntimeHints;
import org.springframework.aot.hint.RuntimeHintsRegistrar;

import java.util.UUID;
import java.util.stream.Stream;

public class NativeImageHints implements RuntimeHintsRegistrar {

    @Override
    public void registerHints(RuntimeHints hints, ClassLoader classLoader) {
        // Temporal workflow/activity dynamic proxies
        hints.proxies().registerJdkProxy(TelegramMessageProcess.class);
        hints.proxies().registerJdkProxy(MessageProcess.class);
        try {
            hints.proxies().registerJdkProxy(
                    MessageProcess.class,
                    Class.forName("io.temporal.internal.sync.AsyncInternal$AsyncMarker"));
        } catch (ClassNotFoundException ignored) {
        }

        // Temporal SDK internal proxies
        hints.proxies().registerJdkProxy(WorkflowServiceStubs.class);
        hints.proxies().registerJdkProxy(WorkflowClient.class);
        hints.proxies().registerJdkProxy(ScheduleClient.class);
        hints.proxies().registerJdkProxy(ActivityCompletionClient.class);
        hints.proxies().registerJdkProxy(WorkflowStub.class);
        hints.proxies().registerJdkProxy(ScheduleHandle.class);
        hints.proxies().registerJdkProxy(WorkflowUpdateHandle.class);
        hints.proxies().registerJdkProxy(BatchRequest.class);

        // Temporal implementation classes
        Stream.of(TelegramMessageProcessWorkflow.class, MessageProcessActivity.class)
                .forEach(c -> hints.reflection().registerType(c, MemberCategory.values()));

        // Temporal Spring Boot annotations
        hints.reflection().registerType(WorkflowImpl.class, MemberCategory.values());
        hints.reflection().registerType(ActivityImpl.class, MemberCategory.values());

        // Domain records (Temporal/Jackson serialization)
        Stream.of(
                TelegramMessage.class, User.class, MessageReference.class,
                Address.class, House.class, MessageTranscription.class,
                ParsedMessage.class, MessageToSend.class,
                AddressKladr.class, KladrEntity.class, KladrCode.class
        ).forEach(c -> hints.reflection().registerType(c, MemberCategory.values()));

        hints.reflection().registerType(KladrCode.Level.class, MemberCategory.values());

        // JPA entities
        Stream.of(
                TelegramMessageEntity.class, TelegramMessageTranscribeEntity.class,
                AddressEntity.class, Subscription.class, TelegramMessageId.class
        ).forEach(c -> hints.reflection().registerType(c, MemberCategory.values()));

        // Hibernate array types for multi-id loaders
        hints.reflection().registerType(TelegramMessageId[].class, MemberCategory.values());
        hints.reflection().registerType(UUID[].class, MemberCategory.values());

        // Hypersistence Utils (PostgreSQL hstore)
        hints.reflection().registerType(PostgreSQLHStoreType.class, MemberCategory.values());

        // Flyway migration resources
        hints.resources().registerPattern("db/migration/*");
    }
}
