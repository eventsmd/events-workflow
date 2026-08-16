package messaging

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
}
