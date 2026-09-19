package claude

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func claudePauseTurnError(info *relaycommon.RelayInfo, stopReason string) *types.NewAPIError {
	if info == nil || info.RelayFormat == types.RelayFormatClaude || !strings.EqualFold(strings.TrimSpace(stopReason), "pause_turn") {
		return nil
	}
	return types.NewOpenAIError(
		fmt.Errorf("claude response paused before completion; continuation requires replaying the paused assistant response"),
		types.ErrorCodeBadResponseBody,
		http.StatusBadGateway,
		types.ErrOptionWithSkipRetry(),
	)
}

func claudeResponseStopReason(response *dto.ClaudeResponse) string {
	if response == nil {
		return ""
	}
	if response.Delta != nil && response.Delta.StopReason != nil {
		return *response.Delta.StopReason
	}
	return response.StopReason
}

func claudePauseTurnStreamError(data string, info *relaycommon.RelayInfo) *types.NewAPIError {
	if info == nil || info.RelayFormat == types.RelayFormatClaude || data == "" {
		return nil
	}
	var response dto.ClaudeResponse
	if err := common.UnmarshalJsonStr(data, &response); err != nil {
		return nil
	}
	return claudePauseTurnError(info, claudeResponseStopReason(&response))
}

func claudeHandlerWithPauseTurnGuard(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	if info == nil || info.RelayFormat == types.RelayFormatClaude || resp == nil || resp.Body == nil {
		return ClaudeHandler(c, resp, info)
	}

	originalBody := resp.Body
	body, err := io.ReadAll(originalBody)
	_ = originalBody.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	var response dto.ClaudeResponse
	if err := common.Unmarshal(body, &response); err == nil {
		if pauseErr := claudePauseTurnError(info, claudeResponseStopReason(&response)); pauseErr != nil {
			return nil, pauseErr
		}
	}
	return ClaudeHandler(c, resp, info)
}
