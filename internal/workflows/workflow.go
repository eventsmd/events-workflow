package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"events-workflow/internal/domain"
)

// WorkflowName — workflow type name; external service starts it by this name,
// must not be changed.
const WorkflowName = "TelegramMessageProcess"

func workflowRegisterOptions() workflow.RegisterOptions {
	return workflow.RegisterOptions{Name: WorkflowName}
}

// WorkerOptions — worker.Options for events-workflow. WorkerStopTimeout allows
// already-accepted (not in-flight-cancelled) activities to complete on SIGTERM —
// a port of WorkerFactory.shutdown() behavior from Java, which waited for
// accepted tasks to finish. Without this timeout (default 0) the activity context
// is cancelled immediately on each deploy: Temporal retries the entire activity,
// and a subscriber that already received the SQS message receives it again.
func WorkerOptions() worker.Options {
	return worker.Options{WorkerStopTimeout: 5 * time.Minute}
}

func Register(w worker.Worker, a *Activities) {
	w.RegisterWorkflowWithOptions(TelegramMessageProcess, workflowRegisterOptions())
	w.RegisterActivity(a)
}

// TelegramMessageProcess — port of TelegramMessageProcessWorkflow.processMessage.
// Activity options are identical to Java: 5 min start-to-close, retries 1s→300s ×5.0,
// maximum 3 attempts.
func TelegramMessageProcess(ctx workflow.Context, msg domain.TelegramMessage) error {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			MaximumInterval:    300 * time.Second,
			BackoffCoefficient: 5.0,
			MaximumAttempts:    3,
		},
	})
	var a *Activities

	if err := workflow.ExecuteActivity(ctx, a.SaveRawMessage, msg).Get(ctx, nil); err != nil {
		return err
	}
	var parsed domain.ParsedMessage
	if err := workflow.ExecuteActivity(ctx, a.ParseMessage, msg).Get(ctx, &parsed); err != nil {
		return err
	}
	if parsed.MessageTranscription == nil {
		return nil
	}
	if err := workflow.ExecuteActivity(ctx, a.SaveParsedMessage, parsed).Get(ctx, nil); err != nil {
		return err
	}
	if err := workflow.ExecuteActivity(ctx, a.NormalizeAddress, parsed).Get(ctx, nil); err != nil {
		return err
	}

	id := parsed.OriginalMessage.ID
	chatID := parsed.OriginalMessage.ChatID

	// Best-effort publishing to NATS: parallel to notify, error only
	// logged — like Async.procedure + soft-await in Java version.
	publishFuture := workflow.ExecuteActivity(ctx, a.PublishEvent, id, chatID)

	if err := workflow.ExecuteActivity(ctx, a.Notify, id, chatID).Get(ctx, nil); err != nil {
		return err
	}
	if err := publishFuture.Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("publishEvent failed (ignored)", "error", err)
	}
	return nil
}
