package messaging

import (
	"context"
	"testing"
)

// TestNewSQSSender_DefaultsRegionAndCredentials — parity with Java's
// application.yaml (cloud.aws.region.static: ${AWS_REGION:us-east-1},
// cloud.aws.credentials.access-key/secret-key: ${...:test}). Runs without
// network access: WithCredentialsProvider/WithDefaultRegion short-circuit
// the SDK's default provider chain.
func TestNewSQSSender_DefaultsRegionAndCredentials(t *testing.T) {
	for _, k := range []string{
		"AWS_REGION", "AWS_DEFAULT_REGION",
		"AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY",
		"AWS_PROFILE", "AWS_CONFIG_FILE", "AWS_SHARED_CREDENTIALS_FILE",
	} {
		t.Setenv(k, "")
	}
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	sender, err := NewSQSSender(context.Background(), "events-notifications")
	if err != nil {
		t.Fatal(err)
	}
	opts := sender.client.Options()
	if opts.Region != defaultAWSRegion {
		t.Fatalf("region = %q, want %q", opts.Region, defaultAWSRegion)
	}
	creds, err := opts.Credentials.Retrieve(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if creds.AccessKeyID != defaultAWSAccessKeyID || creds.SecretAccessKey != defaultAWSSecretKey {
		t.Fatalf("creds = %+v, want %s/%s", creds, defaultAWSAccessKeyID, defaultAWSSecretKey)
	}
}

// TestSQSSender_ResolveQueue_PassesThroughFullURL — Java accepted a
// queue name OR a URL/ARN; a value that already looks like a URL must be
// used verbatim instead of round-tripping through GetQueueUrl (which would
// treat it as an (invalid) queue name). A non-FIFO destination needs no
// further lookups, so this runs without a client.
func TestSQSSender_ResolveQueue_PassesThroughFullURL(t *testing.T) {
	for _, want := range []string{
		"https://sqs.us-east-1.amazonaws.com/123456789012/events-notifications",
		"http://localhost:4566/000000000000/events-notifications",
	} {
		s := &SQSSender{queueName: want}
		got, err := s.resolveQueue(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got.url != want {
			t.Fatalf("got %q, want %q", got.url, want)
		}
		if got.fifo {
			t.Fatalf("%q must not be treated as a FIFO queue", want)
		}
	}
}
