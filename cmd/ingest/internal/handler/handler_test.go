package handler

import (
	"context"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
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
		Log:      log.New(io.Discard, "", 0),
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

// Sentinel test to confirm the fake satisfies the interface.
var _ SendMessageAPI = (*fakeSQS)(nil)
