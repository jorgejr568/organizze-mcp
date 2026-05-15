// Package handler implements the AWS Lambda Function URL handler for the
// stats ingest endpoint. It authenticates the caller with a shared secret,
// validates the JSON body, and forwards the raw bytes onto SQS.
package handler

import (
	"context"
	"crypto/subtle"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// SendMessageAPI is the subset of the SQS client used by the handler.
// Defined as an interface so tests can substitute a fake.
type SendMessageAPI interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Handler holds the runtime dependencies that survive across Lambda
// invocations (cold-start initialization).
type Handler struct {
	QueueURL string
	Secret   string
	SQS      SendMessageAPI
	Log      *log.Logger
}

// Handle is the Lambda Function URL entrypoint (payload format v2.0).
func (h *Handler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	logger := h.Log
	if logger == nil {
		logger = log.Default()
	}
	prefix := "[" + req.RequestContext.RequestID + "] "

	provided := req.Headers["x-ingest-token"]
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.Secret)) != 1 {
		logger.Print(prefix + "auth: rejected request (token mismatch or missing)")
		return jsonResponse(401, `{"error":"unauthorized"}`), nil
	}

	if req.RequestContext.HTTP.Method != "POST" {
		logger.Printf(prefix+"method %s rejected", req.RequestContext.HTTP.Method)
		return jsonResponse(405, `{"error":"method not allowed"}`), nil
	}

	return jsonResponse(500, `{"error":"not implemented"}`), nil
}

func jsonResponse(status int, body string) events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}
