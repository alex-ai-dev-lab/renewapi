package helper

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newResponsesValidationContext(path, body string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	return ctx
}

func TestGetAndValidateResponsesRequestReusesRoutingContext(t *testing.T) {
	ctx := newResponsesValidationContext("/v1/responses", `{"model":`)
	cached := &dto.OpenAIResponsesRequest{Model: "gpt-5.5", Input: json.RawMessage(`"hello"`)}
	common.SetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest, cached)

	request, err := GetAndValidateResponsesRequest(ctx)
	require.NoError(t, err)
	require.Same(t, cached, request)
}

func TestGetAndValidateResponsesRequestKeepsValidationOnCachedDTO(t *testing.T) {
	ctx := newResponsesValidationContext("/v1/responses", `{}`)
	cached := &dto.OpenAIResponsesRequest{Model: "gpt-5.5"}
	common.SetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest, cached)

	request, err := GetAndValidateResponsesRequest(ctx)
	require.Nil(t, request)
	require.EqualError(t, err, "input is required")
}

func TestGetAndValidateResponsesRequestFallsBackForWrongContextType(t *testing.T) {
	ctx := newResponsesValidationContext("/v1/responses", `{"model":"gpt-5.5","input":"from-body"}`)
	common.SetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest, &dto.OpenAIResponsesCompactionRequest{Model: "wrong-type"})

	request, err := GetAndValidateResponsesRequest(ctx)
	require.NoError(t, err)
	require.JSONEq(t, `"from-body"`, string(request.Input))
}

func TestGetAndValidateResponsesCompactionRequestReusesRoutingContext(t *testing.T) {
	ctx := newResponsesValidationContext("/v1/responses/compact", `{"model":`)
	cached := &dto.OpenAIResponsesCompactionRequest{Model: "gpt-5.5"}
	common.SetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest, cached)

	request, err := GetAndValidateResponsesCompactionRequest(ctx)
	require.NoError(t, err)
	require.Same(t, cached, request)
}

func TestGetAndValidateResponsesCompactionRequestKeepsValidationOnCachedDTO(t *testing.T) {
	ctx := newResponsesValidationContext("/v1/responses/compact", `{}`)
	cached := &dto.OpenAIResponsesCompactionRequest{}
	common.SetContextKey(ctx, constant.ContextKeyResponsesRoutingRequest, cached)

	request, err := GetAndValidateResponsesCompactionRequest(ctx)
	require.Nil(t, request)
	require.EqualError(t, err, "model is required")
}
