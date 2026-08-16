// Package messaging — sending notifications to SQS. JSON payload and message
// attributes (contentType, JavaType) are identical to Spring Cloud AWS SqsTemplate.
package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// javaType — value of the message attribute JavaType, which Spring Cloud AWS
// SqsTemplate sets by default (payload class name), so that
// a consumer resolving the payload type by this header does not break.
const javaType = "md.address.events.messaging.MessageToSend"

// defaultAWSRegion/defaultAWS*Key — defaults from Java version (application.yaml:
// cloud.aws.region.static/credentials), preserved for wire compatibility in
// local/test environments (LocalStack etc.) where these variables are not
// set.
const (
	defaultAWSRegion      = "us-east-1"
	defaultAWSAccessKeyID = "test"
	defaultAWSSecretKey   = "test"
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
	// Java (application.yaml) defaults AWS_REGION → us-east-1 and
	// AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY → test/test. The bare SDK
	// default config loader leaves these unset and fails/misbehaves against
	// LocalStack-style environments the spec's env table assumes.
	opts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithDefaultRegion(defaultAWSRegion),
	}
	if os.Getenv("AWS_ACCESS_KEY_ID") == "" && os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(defaultAWSAccessKeyID, defaultAWSSecretKey, "")))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &SQSSender{client: sqs.NewFromConfig(cfg), queueName: queueName}, nil
}

func (s *SQSSender) resolveQueueURL(ctx context.Context) (string, error) {
	// Java accepted a queue name OR a full URL/ARN (SqsTemplate resolves
	// either). Mirror that: a value that already looks like a URL is used
	// verbatim instead of being passed to GetQueueUrl as a name.
	if strings.HasPrefix(s.queueName, "http://") || strings.HasPrefix(s.queueName, "https://") {
		return s.queueName, nil
	}
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
			"JavaType": {DataType: aws.String("String"),
				StringValue: aws.String(javaType)},
		},
	})
	return err
}
