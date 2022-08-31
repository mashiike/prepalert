package prepalert

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type HandleContext struct {
	ReqID uint64

	sqsClient         *sqs.Client
	queueURL          string
	message           *events.SQSMessage
	visibilityTimeout time.Duration
}

func (app *App) NewHandleContext(reqID uint64, message *events.SQSMessage) *HandleContext {
	return &HandleContext{
		ReqID:             reqID,
		message:           message,
		queueURL:          app.queueUrl,
		sqsClient:         app.sqsClient,
		visibilityTimeout: 30 * time.Second,
	}
}

func (hctx *HandleContext) ExtendTimeout(ctx context.Context, d time.Duration) error {
	hctx.visibilityTimeout += d
	log.Printf("[debug][%d] change message visivirity message id=%s, timeout=%s", hctx.ReqID, hctx.message.MessageId, hctx.visibilityTimeout)
	_, err := hctx.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(hctx.queueURL),
		ReceiptHandle:     aws.String(hctx.message.ReceiptHandle),
		VisibilityTimeout: int32(d.Seconds()),
	})
	if err != nil {
		return err
	}
	return nil
}

type contextKey string

var handleContextKey contextKey = "__handle_context"

func WithHandleContext(ctx context.Context, info *HandleContext) context.Context {
	return context.WithValue(ctx, handleContextKey, info)
}

func GetHandleContext(ctx context.Context) (*HandleContext, bool) {
	info, ok := ctx.Value(handleContextKey).(*HandleContext)
	return info, ok
}
