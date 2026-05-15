// Package handler implements the AWS Lambda Function URL handler for the
// stats ingest endpoint. It authenticates the caller with a shared secret,
// validates the JSON body, and forwards the raw bytes onto SQS.
package handler

import (
	"context"
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
// Filled in across Tasks 4–7.
func (h *Handler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	return events.LambdaFunctionURLResponse{StatusCode: 500}, nil
}
