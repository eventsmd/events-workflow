package messaging

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/testcontainers/testcontainers-go"
	tclocalstack "github.com/testcontainers/testcontainers-go/modules/localstack"
)

func TestSQSSender_Send(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	ls, err := tclocalstack.Run(ctx, "localstack/localstack:3")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(ls) })
	port, err := ls.MappedPort(ctx, "4566/tcp")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://localhost:" + port.Port()

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", endpoint)

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client := sqs.NewFromConfig(cfg)
	created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String("events-notifications")})
	if err != nil {
		t.Fatal(err)
	}

	sender, err := NewSQSSender(ctx, "events-notifications")
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(ctx, MessageToSend{
		TelegramID: 777, Message: "💧 Отключение", Topic: "water",
		MessageID: 1, ChatID: 2,
	}); err != nil {
		t.Fatal(err)
	}

	recv, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl: created.QueueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 5,
		MessageAttributeNames: []string{"All"},
	})
	if err != nil || len(recv.Messages) != 1 {
		t.Fatalf("recv: %v %v", recv, err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(*recv.Messages[0].Body), &got); err != nil {
		t.Fatal(err)
	}
	if got["telegram_id"] != float64(777) || got["message"] != "💧 Отключение" ||
		got["topic"] != "water" || got["message_id"] != float64(1) || got["chat_id"] != float64(2) {
		t.Fatalf("%v", got)
	}
	if attr, ok := recv.Messages[0].MessageAttributes["contentType"]; !ok ||
		*attr.StringValue != "application/json" {
		t.Fatalf("contentType attribute missing: %v", recv.Messages[0].MessageAttributes)
	}
	if attr, ok := recv.Messages[0].MessageAttributes["JavaType"]; !ok ||
		*attr.StringValue != "md.address.events.messaging.MessageToSend" {
		t.Fatalf("JavaType attribute missing/wrong: %v", recv.Messages[0].MessageAttributes)
	}
}

// TestSQSSender_Send_QueueURL — Java accepted a queue name OR a full
// URL/ARN via SqsTemplate; the Go sender must too.
func TestSQSSender_Send_QueueURL(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	ls, err := tclocalstack.Run(ctx, "localstack/localstack:3")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(ls) })
	port, err := ls.MappedPort(ctx, "4566/tcp")
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "http://localhost:" + port.Port()

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", endpoint)

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client := sqs.NewFromConfig(cfg)
	created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String("events-notifications-by-url")})
	if err != nil {
		t.Fatal(err)
	}

	sender, err := NewSQSSender(ctx, *created.QueueUrl)
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(ctx, MessageToSend{TelegramID: 1, Message: "x", ChatID: 2, MessageID: 3}); err != nil {
		t.Fatal(err)
	}

	recv, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl: created.QueueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 5,
	})
	if err != nil || len(recv.Messages) != 1 {
		t.Fatalf("recv: %v %v", recv, err)
	}
}

// startLocalStackSQS boots LocalStack and returns a client configured against it.
func startLocalStackSQS(ctx context.Context, t *testing.T) *sqs.Client {
	t.Helper()
	ls, err := tclocalstack.Run(ctx, "localstack/localstack:3")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testcontainers.TerminateContainer(ls) })
	port, err := ls.MappedPort(ctx, "4566/tcp")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_ENDPOINT_URL", "http://localhost:"+port.Port())

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return sqs.NewFromConfig(cfg)
}

// TestSQSSender_Send_FIFO — production runs against a FIFO queue, which
// rejects SendMessage without MessageGroupId ("The request must contain the
// parameter MessageGroupId"). Java never set one explicitly either: Spring
// Cloud AWS's SqsTemplate detected the .fifo suffix (FifoUtils.isFifo) and
// filled in a random UUID group id, plus a random deduplication id unless
// the queue has ContentBasedDeduplication enabled.
func TestSQSSender_Send_FIFO(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	client := startLocalStackSQS(ctx, t)

	const queueName = "events-notifications.fifo"
	created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName:  aws.String(queueName),
		Attributes: map[string]string{"FifoQueue": "true"},
	})
	if err != nil {
		t.Fatal(err)
	}

	sender, err := NewSQSSender(ctx, queueName)
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.Send(ctx, MessageToSend{
		TelegramID: 777, Message: "💧 Отключение", Topic: "water",
		MessageID: 1, ChatID: 2,
	}); err != nil {
		t.Fatalf("send to a FIFO queue: %v", err)
	}

	recv, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl: created.QueueUrl, MaxNumberOfMessages: 1, WaitTimeSeconds: 5,
		MessageSystemAttributeNames: []types.MessageSystemAttributeName{
			types.MessageSystemAttributeNameMessageGroupId,
			types.MessageSystemAttributeNameMessageDeduplicationId,
		},
	})
	if err != nil || len(recv.Messages) != 1 {
		t.Fatalf("recv: %v %v", recv, err)
	}
	attrs := recv.Messages[0].Attributes
	if attrs[string(types.MessageSystemAttributeNameMessageGroupId)] == "" {
		t.Fatalf("MessageGroupId not set: %v", attrs)
	}
	// ContentBasedDeduplication is off on this queue, so Java supplied an
	// explicit deduplication id; without it SQS rejects the send outright.
	if attrs[string(types.MessageSystemAttributeNameMessageDeduplicationId)] == "" {
		t.Fatalf("MessageDeduplicationId not set: %v", attrs)
	}
}

// TestSQSSender_Send_FIFOContentBasedDeduplication — when the queue computes
// deduplication ids from the body itself, SqsTemplate sent none (its default
// TemplateContentBasedDeduplication.AUTO probes the queue attribute), leaving
// SQS to collapse identical notifications inside the dedup window. Sending a
// random id instead would silently turn that off and let duplicate
// notifications through.
func TestSQSSender_Send_FIFOContentBasedDeduplication(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	ctx := context.Background()
	client := startLocalStackSQS(ctx, t)

	const queueName = "events-dedup.fifo"
	created, err := client.CreateQueue(ctx, &sqs.CreateQueueInput{
		QueueName: aws.String(queueName),
		Attributes: map[string]string{
			"FifoQueue":                 "true",
			"ContentBasedDeduplication": "true",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	sender, err := NewSQSSender(ctx, queueName)
	if err != nil {
		t.Fatal(err)
	}
	msg := MessageToSend{TelegramID: 42, Message: "same", Topic: "water", MessageID: 7, ChatID: 8}
	for i := 0; i < 2; i++ {
		if err := sender.Send(ctx, msg); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	var total int
	for {
		recv, err := client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl: created.QueueUrl, MaxNumberOfMessages: 10, WaitTimeSeconds: 2,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(recv.Messages) == 0 {
			break
		}
		total += len(recv.Messages)
		for _, m := range recv.Messages {
			client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl: created.QueueUrl, ReceiptHandle: m.ReceiptHandle})
		}
	}
	if total != 1 {
		t.Fatalf("got %d messages, want 1 — identical bodies must dedup, so no explicit id may be sent", total)
	}
}
