package controller

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestShouldCompatRetryByError_InvalidEncryptedContent(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("invalid_encrypted_content"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	assert.True(t, shouldCompatRetryByError(err))
	assert.True(t, shouldCompatExcludeChannelForRetry(err))
	assert.False(t, shouldCompatDisableChannel(err))
}

func TestShouldCompatRetryByError_ResponsesFunctionCallArgumentsObject(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("Invalid type for 'input[8].arguments': expected an object, but got a string instead."),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	assert.True(t, shouldCompatRetryByError(err))
}

func TestCompatStreamRetryError_PreFirstChunkHandlerStop(t *testing.T) {
	status := relaycommon.NewStreamStatus()
	status.RecordError("invalid_encrypted_content")
	status.SetEndReason(relaycommon.StreamEndReasonHandlerStop, nil)
	info := &relaycommon.RelayInfo{
		IsStream:              true,
		ReceivedResponseCount: 0,
		StreamStatus:          status,
	}

	err := compatStreamRetryError(info)

	if assert.NotNil(t, err) {
		assert.Equal(t, http.StatusBadGateway, err.StatusCode)
	}
}

func TestCompatStreamRetryError_AfterChunkHandlerStopNoRetry(t *testing.T) {
	status := relaycommon.NewStreamStatus()
	status.RecordError("invalid_encrypted_content")
	status.SetEndReason(relaycommon.StreamEndReasonHandlerStop, nil)
	info := &relaycommon.RelayInfo{
		IsStream:              true,
		ReceivedResponseCount: 1,
		StreamStatus:          status,
	}

	assert.Nil(t, compatStreamRetryError(info))
}

func TestShouldExcludeChannelAfterRetryDecision_SingleKeyRetryable404(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	err := types.NewOpenAIError(
		errors.New("not found"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusNotFound,
	)
	channel := &model.Channel{Id: 42}

	assert.True(t, shouldExcludeChannelAfterRetryDecision(c, &service.RetryParam{}, channel, err, true))
}

func TestShouldExcludeChannelAfterRetryDecision_ExcludesMultiKeyChannelScopedError(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	err := types.NewOpenAIError(
		errors.New("upstream error"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusInternalServerError,
	)
	channel := &model.Channel{Id: 42}
	channel.ChannelInfo.IsMultiKey = true
	channel.Key = "sk-1\nsk-2"

	assert.True(t, shouldExcludeChannelAfterRetryDecision(c, &service.RetryParam{}, channel, err, true))
}

func TestShouldExcludeChannelAfterRetryDecision_KeepsMultiKeyKeyScopedErrorWithUntriedKey(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	err := types.NewOpenAIError(
		errors.New("rate limit exceeded"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)
	channel := &model.Channel{Id: 42, Key: "sk-1\nsk-2"}
	channel.ChannelInfo.IsMultiKey = true
	retryParam := &service.RetryParam{}
	service.RecordTriedMultiKeyIndex(retryParam, channel.Id, 0)

	assert.False(t, shouldExcludeChannelAfterRetryDecision(c, retryParam, channel, err, true))
}

func TestShouldExcludeChannelAfterRetryDecision_ExcludesMultiKeyKeyScopedErrorWhenExhausted(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, true)
	err := types.NewOpenAIError(
		errors.New("rate limit exceeded"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)
	channel := &model.Channel{Id: 42, Key: "sk-1\nsk-2"}
	channel.ChannelInfo.IsMultiKey = true
	retryParam := &service.RetryParam{}
	service.RecordTriedMultiKeyIndex(retryParam, channel.Id, 0)
	service.RecordTriedMultiKeyIndex(retryParam, channel.Id, 1)

	assert.True(t, shouldExcludeChannelAfterRetryDecision(c, retryParam, channel, err, true))
}

func TestShouldRecordRelayErrorLogRespectsNoRecordAndDedup(t *testing.T) {
	c, _ := gin.CreateTestContext(nil)
	oldErrorLogEnabled := constant.ErrorLogEnabled
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		constant.ErrorLogEnabled = oldErrorLogEnabled
	})

	err := types.NewErrorWithStatusCode(
		errors.New("token TPM limit exceeded"),
		types.ErrorCodeRateLimitExceeded,
		http.StatusTooManyRequests,
		types.ErrOptionWithNoRecordErrorLog(),
	)
	assert.False(t, shouldRecordRelayErrorLog(c, err, false))

	err = types.NewErrorWithStatusCode(errors.New("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	assert.True(t, shouldRecordRelayErrorLog(c, err, false))
	common.SetContextKey(c, contextKeyErrorLogged, true)
	assert.False(t, shouldRecordRelayErrorLog(c, err, false))
}
