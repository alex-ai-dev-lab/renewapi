package common

import (
	"context"
	"net/http"
)

// ResponsesWSTransport 只替换传输层，鉴权、路由、提交门及计费仍走普通 Responses 请求。
type ResponsesWSTransport interface {
	RoundTrip(*http.Request, *RelayInfo, *ResponsesWSExecution) (*http.Response, error)
}

type ResponsesWSExecution struct {
	StreamID, EventID, PreviousResponseID string
	Transport                             ResponsesWSTransport
	// 由请求 worker 写入并在 Relay 返回后读取，terminal 不能早于结算结果。
	SettlementError error
}

type responsesWSContextKey struct{}

func WithResponsesWSExecution(ctx context.Context, execution *ResponsesWSExecution) context.Context {
	return context.WithValue(ctx, responsesWSContextKey{}, execution)
}

func GetResponsesWSExecution(ctx context.Context) *ResponsesWSExecution {
	execution, _ := ctx.Value(responsesWSContextKey{}).(*ResponsesWSExecution)
	return execution
}
