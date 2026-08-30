package claude

import (
	"fmt"
	"net/http"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"
)

func claudeStreamCompletionError(info *relaycommon.RelayInfo, claudeInfo *ClaudeResponseInfo) *types.NewAPIError {
	if claudeInfo != nil && claudeInfo.Done {
		return nil
	}
	options := make([]types.NewAPIErrorOptions, 0, 1)
	if info != nil && info.ClientResponseCommitted() {
		options = append(options, types.ErrOptionWithSkipRetry())
	}
	return types.NewOpenAIError(
		fmt.Errorf("incomplete claude stream: missing message_delta"),
		types.ErrorCodeBadResponseBody,
		http.StatusBadGateway,
		options...,
	)
}
