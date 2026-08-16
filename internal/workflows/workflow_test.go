package workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"events-workflow/internal/domain"
)

func assertAnError() error { return errors.New("nats down") }

func testMessage() domain.TelegramMessage {
	return domain.TelegramMessage{
		ID: 1, ChatID: 2, Text: "Отключение воды",
		Date: domain.LocalDateTime{Time: time.Date(2025, 12, 1, 22, 50, 0, 0, time.UTC)},
	}
}

func newEnv(t *testing.T) (*testsuite.TestWorkflowEnvironment, *Activities) {
	var s testsuite.WorkflowTestSuite
	env := s.NewTestWorkflowEnvironment()
	a := &Activities{}
	env.RegisterWorkflowWithOptions(TelegramMessageProcess,
		workflowRegisterOptions())
	env.RegisterActivity(a)
	return env, a
}

func TestWorkflow_FullFlow(t *testing.T) {
	env, a := newEnv(t)
	msg := testMessage()
	parsed := domain.ParsedMessage{
		OriginalMessage:      msg,
		MessageTranscription: &domain.MessageTranscription{Event: "shutdown"},
	}
	env.OnActivity(a.SaveRawMessage, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.ParseMessage, mock.Anything, mock.Anything).Return(parsed, nil).Once()
	env.OnActivity(a.SaveParsedMessage, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.NormalizeAddress, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.PublishEvent, mock.Anything, int64(1), int64(2)).Return(nil).Once()
	env.OnActivity(a.Notify, mock.Anything, int64(1), int64(2)).Return(nil).Once()

	env.ExecuteWorkflow(TelegramMessageProcess, msg)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestWorkflow_NullTranscription_Skips(t *testing.T) {
	env, a := newEnv(t)
	msg := testMessage()
	env.OnActivity(a.SaveRawMessage, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.ParseMessage, mock.Anything, mock.Anything).
		Return(domain.ParsedMessage{OriginalMessage: msg}, nil).Once()

	env.ExecuteWorkflow(TelegramMessageProcess, msg)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	// SaveParsedMessage/Notify/PublishEvent не были замоканы — вызов упал бы
	env.AssertExpectations(t)
}

func TestWorkerOptions_GracefulDrain(t *testing.T) {
	// Регрессия: WorkerStopTimeout=0 (дефолт) отменяет in-flight активити
	// мгновенно на SIGTERM ⇒ Temporal ретраит активность целиком, и
	// подписчик, уже получивший SQS-уведомление, получает его снова.
	opts := WorkerOptions()
	require.Equal(t, 5*time.Minute, opts.WorkerStopTimeout)
}

func TestWorkflow_PublishFailure_Ignored(t *testing.T) {
	env, a := newEnv(t)
	msg := testMessage()
	parsed := domain.ParsedMessage{
		OriginalMessage:      msg,
		MessageTranscription: &domain.MessageTranscription{Event: "shutdown"},
	}
	env.OnActivity(a.SaveRawMessage, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.ParseMessage, mock.Anything, mock.Anything).Return(parsed, nil).Once()
	env.OnActivity(a.SaveParsedMessage, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.NormalizeAddress, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnActivity(a.PublishEvent, mock.Anything, mock.Anything, mock.Anything).
		Return(assertAnError()).Times(3) // 3 попытки ретраев, затем ошибка игнорируется
	env.OnActivity(a.Notify, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	env.ExecuteWorkflow(TelegramMessageProcess, msg)
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError()) // publish best-effort — workflow зелёный
}
