package queryrunner

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type QueryRunningContext struct {
	ReqID uint64

	sqsClient *sqs.Client
	queueURL  string
	message   *events.SQSMessage
}

func NewQueryRunningContext(sqsClient *sqs.Client, queueURL string, reqID uint64, message *events.SQSMessage) *QueryRunningContext {
	return &QueryRunningContext{
		ReqID:     reqID,
		message:   message,
		queueURL:  queueURL,
		sqsClient: sqsClient,
	}
}

func (hctx *QueryRunningContext) ChangeSQSMessageVisibilityTimeout(ctx context.Context, visibilityTimeout time.Duration) error {
	if hctx == nil {
		return nil
	}
	log.Printf("[debug][%d] change message visivirity message id=%s, timeout=%s", hctx.ReqID, hctx.message.MessageId, visibilityTimeout)
	_, err := hctx.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(hctx.queueURL),
		ReceiptHandle:     aws.String(hctx.message.ReceiptHandle),
		VisibilityTimeout: int32(visibilityTimeout.Seconds()),
	})
	if err != nil {
		return err
	}
	return nil
}

type contextKey string

var QueryRunningContextKey contextKey = "__handle_context"

func WithQueryRunningContext(ctx context.Context, info *QueryRunningContext) context.Context {
	return context.WithValue(ctx, QueryRunningContextKey, info)
}

func GetQueryRunningContext(ctx context.Context) (*QueryRunningContext, bool) {
	info, ok := ctx.Value(QueryRunningContextKey).(*QueryRunningContext)
	return info, ok
}
