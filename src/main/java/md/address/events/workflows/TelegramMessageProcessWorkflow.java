package md.address.events.workflows;

import io.temporal.activity.ActivityOptions;
import io.temporal.common.RetryOptions;
import io.temporal.spring.boot.WorkflowImpl;
import io.temporal.workflow.Workflow;
import md.address.events.domain.TelegramMessage;

import java.time.Duration;

@WorkflowImpl(taskQueues = "TelegramMessageQueue")
public class TelegramMessageProcessWorkflow implements TelegramMessageProcess {

    private final MessageProcess activity =
            Workflow.newActivityStub(
                    MessageProcess.class,
                    ActivityOptions.newBuilder()
                            .setStartToCloseTimeout(Duration.ofMinutes(5))
                            .setRetryOptions(RetryOptions.newBuilder()
                                    .setInitialInterval(Duration.ofSeconds(1))
                                    .setMaximumInterval(Duration.ofSeconds(300))
                                    .setBackoffCoefficient(5.0)
                                    .setMaximumAttempts(3)
                                    .build())
                            .build()
            );

    @Override
    public void processMessage(TelegramMessage message) {
        activity.saveRawMessage(message);
        var parsedMessage = activity.parseMessage(message);
        if (parsedMessage.messageTranscription() != null) {
            activity.saveParsedMessage(parsedMessage);
            activity.normalizeAddress(parsedMessage);
            activity.notify(parsedMessage.originalMessage().id(), parsedMessage.originalMessage().chatId());
        }
    }
}
