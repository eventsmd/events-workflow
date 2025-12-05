package md.address.events.workflows;

import io.temporal.workflow.WorkflowInterface;
import io.temporal.workflow.WorkflowMethod;
import md.address.events.domain.TelegramMessage;

@WorkflowInterface
public interface TelegramMessageProcess {

    @WorkflowMethod(name = "TelegramMessageProcess")
    void processMessage(TelegramMessage message);
}
