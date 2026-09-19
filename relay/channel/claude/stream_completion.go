package claude

import (
	"fmt"
	"net/http"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

func markClaudeStreamCommitted(c *gin.Context, info *relaycommon.RelayInfo) {
	if c == nil || c.Writer == nil || info == nil || info.StreamStatus == nil || !c.Writer.Written() {
		return
	}
	info.StreamStatus.MarkClientCommitted()
}

func claudeStreamCompletionError(c *gin.Context, info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) *types.NewAPIError {
	if claudeInfo != nil && claudeInfo.Done {
		return nil
	}
	markClaudeStreamCommitted(c, info)
	committed := info != nil && info.ClientResponseCommitted()
	if !committed && c != nil && c.Writer != nil {
		committed = c.Writer.Written()
	}
	options := make([]types.NewAPIErrorOptions, 0, 1)
	if committed {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.NewOpenAIError(
		fmt.Errorf("incomplete claude stream: missing message_delta"),
		types.ErrorCodeBadResponseBody,
		http.StatusBadGateway,
		options...,
	)
}
