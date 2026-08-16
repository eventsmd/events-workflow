package workflows

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"events-workflow/internal/domain"
)

// WorkflowName — имя типа workflow; внешний сервис стартует его по этому
// имени, менять нельзя.
const WorkflowName = "TelegramMessageProcess"

func workflowRegisterOptions() workflow.RegisterOptions {
	return workflow.RegisterOptions{Name: WorkflowName}
}

// WorkerOptions — worker.Options для events-workflow. WorkerStopTimeout даёт
// уже принятым (не in-flight-cancelled) активити доработать при SIGTERM —
// порт поведения WorkerFactory.shutdown() из Java, который дожидался
// завершения принятых задач. Без этого таймаута (по умолчанию 0) контекст
// активности отменяется мгновенно на каждом деплое: Temporal ретраит всю
// активность заново, а подписчик, уже получивший SQS-сообщение, получает
// его повторно.
func WorkerOptions() worker.Options {
	return worker.Options{WorkerStopTimeout: 5 * time.Minute}
}

func Register(w worker.Worker, a *Activities) {
	w.RegisterWorkflowWithOptions(TelegramMessageProcess, workflowRegisterOptions())
	w.RegisterActivity(a)
}

// TelegramMessageProcess — порт TelegramMessageProcessWorkflow.processMessage.
// Опции активити идентичны Java: 5 мин start-to-close, ретраи 1s→300s ×5.0,
// максимум 3 попытки.
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

	// Best-effort публикация в NATS: параллельно с notify, ошибка только
	// логируется — как Async.procedure + soft-await в Java-версии.
	publishFuture := workflow.ExecuteActivity(ctx, a.PublishEvent, id, chatID)

	if err := workflow.ExecuteActivity(ctx, a.Notify, id, chatID).Get(ctx, nil); err != nil {
		return err
	}
	if err := publishFuture.Get(ctx, nil); err != nil {
		workflow.GetLogger(ctx).Warn("publishEvent failed (ignored)", "error", err)
	}
	return nil
}
