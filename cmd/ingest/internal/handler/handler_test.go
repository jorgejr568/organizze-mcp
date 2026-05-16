package handler

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
)

type fakeSQS struct {
	out *sqs.SendMessageOutput
	err error
	in  *sqs.SendMessageInput
}

func (f *fakeSQS) SendMessage(_ context.Context, params *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.in = params
	return f.out, f.err
}

func newHandler(t *testing.T, sender SendMessageAPI) *Handler {
	t.Helper()
	return &Handler{
		QueueURL: "https://sqs.us-east-1.amazonaws.com/000/test",
		Secret:   "super-secret",
		SQS:      sender,
		Log:      zap.NewNop(),
	}
}

func req(method, body string, headers map[string]string) events.LambdaFunctionURLRequest {
	return events.LambdaFunctionURLRequest{
		RequestContext: events.LambdaFunctionURLRequestContext{
			RequestID: "req-test-id",
			HTTP: events.LambdaFunctionURLRequestContextHTTPDescription{
				Method: method,
			},
		},
		Headers: headers,
		Body:    body,
	}
}

func TestHandle_MissingToken_Returns401(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status: got %d want 401", resp.StatusCode)
	}
	if strings.Contains(resp.Body, "super-secret") {
		t.Fatalf("response body leaked the secret: %q", resp.Body)
	}
}

func TestHandle_WrongToken_Returns401(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{
		"x-ingest-token": "nope",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status: got %d want 401", resp.StatusCode)
	}
}

func TestHandle_NonPostMethod_Returns405(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("GET", "", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 405 {
		t.Fatalf("status: got %d want 405", resp.StatusCode)
	}
}

func TestHandle_EmptyBody_Returns400(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", "", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestHandle_InvalidJSON_Returns400(t *testing.T) {
	h := newHandler(t, &fakeSQS{})
	resp, err := h.Handle(context.Background(), req("POST", "not-json{", map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 400 {
		t.Fatalf("status: got %d want 400", resp.StatusCode)
	}
}

func TestHandle_HappyPath_Returns202_AndForwardsBody(t *testing.T) {
	fake := &fakeSQS{
		out: &sqs.SendMessageOutput{MessageId: ptr("msg-abc")},
	}
	h := newHandler(t, fake)
	body := `{"stat":"page_view","count":3}`
	resp, err := h.Handle(context.Background(), req("POST", body, map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 202 {
		t.Fatalf("status: got %d want 202", resp.StatusCode)
	}
	if !strings.Contains(resp.Body, `"queued":true`) {
		t.Fatalf("body missing queued:true: %q", resp.Body)
	}
	if !strings.Contains(resp.Body, `"message_id":"msg-abc"`) {
		t.Fatalf("body missing message_id: %q", resp.Body)
	}

	if fake.in == nil {
		t.Fatalf("SendMessage was not called")
	}
	if got := aws.ToString(fake.in.QueueUrl); got != "https://sqs.us-east-1.amazonaws.com/000/test" {
		t.Fatalf("queue url: got %q", got)
	}
	if got := aws.ToString(fake.in.MessageBody); got != body {
		t.Fatalf("message body: got %q want %q", got, body)
	}
}

func TestHandle_SQSError_Returns500_AndDoesNotLeakError(t *testing.T) {
	fake := &fakeSQS{
		err: errors.New("aws: ThrottlingException: rate exceeded for queue arn:aws:...:secret-queue"),
	}
	h := newHandler(t, fake)
	resp, err := h.Handle(context.Background(), req("POST", `{"ok":true}`, map[string]string{
		"x-ingest-token": "super-secret",
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("status: got %d want 500", resp.StatusCode)
	}
	if strings.Contains(resp.Body, "ThrottlingException") || strings.Contains(resp.Body, "secret-queue") {
		t.Fatalf("response body leaked aws error: %q", resp.Body)
	}
}

func ptr[T any](v T) *T { return &v }

// Sentinel test to confirm the fake satisfies the interface.
var _ SendMessageAPI = (*fakeSQS)(nil)
