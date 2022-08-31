package prepalert

import "context"

type HandleInfo struct {
	ReqID uint64
}

type contextKey string

var handleContextKey contextKey = "__handle_context"

func WithHandleInfo(ctx context.Context, info *HandleInfo) context.Context {
	return context.WithValue(ctx, handleContextKey, info)
}

func GetHandleInfo(ctx context.Context) (*HandleInfo, bool) {
	info, ok := ctx.Value(handleContextKey).(*HandleInfo)
	return info, ok
}
