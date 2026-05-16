// Package handler implements the AWS Lambda Function URL handler for the
// stats ingest endpoint. It authenticates the caller with a shared secret,
// validates the JSON body, and forwards the raw bytes onto SQS.
package handler

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/zap"
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
	Log      *zap.Logger
}

// Handle is the Lambda Function URL entrypoint (payload format v2.0).
func (h *Handler) Handle(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	base := h.Log
	if base == nil {
		base = zap.NewNop()
	}
	logger := base.With(zap.String("request_id", req.RequestContext.RequestID))

	provided := req.Headers["x-ingest-token"]
	if subtle.ConstantTimeCompare([]byte(provided), []byte(h.Secret)) != 1 {
		logger.Warn("auth rejected")
		return jsonResponse(401, `{"error":"unauthorized"}`), nil
	}

	if req.RequestContext.HTTP.Method != "POST" {
		logger.Warn("method rejected", zap.String("method", req.RequestContext.HTTP.Method))
		return jsonResponse(405, `{"error":"method not allowed"}`), nil
	}

	body := []byte(req.Body)
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			logger.Warn("base64 decode failed", zap.Error(err))
			return jsonResponse(400, `{"error":"invalid body encoding"}`), nil
		}
		body = decoded
	}
	if len(body) == 0 {
		logger.Warn("empty body")
		return jsonResponse(400, `{"error":"empty body"}`), nil
	}
	if !json.Valid(body) {
		logger.Warn("invalid json")
		return jsonResponse(400, `{"error":"invalid json"}`), nil
	}

	out, err := h.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(h.QueueURL),
		MessageBody: aws.String(string(body)),
	})
	if err != nil {
		logger.Error("sqs send failed", zap.Error(err))
		return jsonResponse(500, `{"error":"internal error"}`), nil
	}

	msgID := aws.ToString(out.MessageId)
	logger.Info("queued message",
		zap.String("message_id", msgID),
		zap.Int("bytes", len(body)),
	)
	return jsonResponse(202, fmt.Sprintf(`{"queued":true,"message_id":%q}`, msgID)), nil
}

func jsonResponse(status int, body string) events.LambdaFunctionURLResponse {
	return events.LambdaFunctionURLResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       body,
	}
}
