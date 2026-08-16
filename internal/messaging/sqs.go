// Package messaging — отправка уведомлений в SQS. JSON-пейлоад и
// message attribute contentType идентичны Spring Cloud AWS SqsTemplate.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

type MessageToSend struct {
	TelegramID int64  `json:"telegram_id"`
	Message    string `json:"message"`
	Topic      string `json:"topic"`
	MessageID  int64  `json:"message_id"`
	ChatID     int64  `json:"chat_id"`
}

type SQSSender struct {
	client    *sqs.Client
	queueName string

	mu       sync.Mutex
	queueURL string
}

func NewSQSSender(ctx context.Context, queueName string) (*SQSSender, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &SQSSender{client: sqs.NewFromConfig(cfg), queueName: queueName}, nil
}

func (s *SQSSender) resolveQueueURL(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.queueURL != "" {
		return s.queueURL, nil
	}
	out, err := s.client.GetQueueUrl(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(s.queueName)})
	if err != nil {
		return "", fmt.Errorf("resolve queue %q: %w", s.queueName, err)
	}
	s.queueURL = *out.QueueUrl
	return s.queueURL, nil
}

func (s *SQSSender) Send(ctx context.Context, m MessageToSend) error {
	queueURL, err := s.resolveQueueURL(ctx)
	if err != nil {
		return err
	}
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queueURL),
		MessageBody: aws.String(string(body)),
		MessageAttributes: map[string]types.MessageAttributeValue{
			"contentType": {DataType: aws.String("String"),
				StringValue: aws.String("application/json")},
		},
	})
	return err
}
