package openai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// OpenaiImageHandler parses image errors before committing the upstream body to the client.
func OpenaiImageHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	if err := common.Unmarshal(responseBody, &usageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	service.IOCopyBytesGracefully(c, resp, responseBody)
	normalizeOpenAIImageUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)
	return &usageResp.Usage, nil
}

func normalizeOpenAIImageUsage(usage *dto.Usage) {
	if usage == nil {
		return
	}
	if usage.InputTokens != 0 {
		usage.PromptTokens = usage.InputTokens
	}
	if usage.OutputTokens != 0 {
		usage.CompletionTokens = usage.OutputTokens
	}
	if usage.InputTokensDetails != nil {
		usage.PromptTokensDetails.CachedTokens = usage.InputTokensDetails.CachedTokens
		usage.PromptTokensDetails.CachedCreationTokens = usage.InputTokensDetails.CachedCreationTokens
		usage.PromptTokensDetails.CacheWriteTokens = usage.InputTokensDetails.CacheWriteTokens
		usage.PromptTokensDetails.ImageTokens = usage.InputTokensDetails.ImageTokens
		usage.PromptTokensDetails.TextTokens = usage.InputTokensDetails.TextTokens
		usage.PromptTokensDetails.AudioTokens = usage.InputTokensDetails.AudioTokens
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
}

func OpenaiImageStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid image stream response")
		return nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return OpenaiImageHandler(c, info, resp)
	}
	if !strings.Contains(contentType, "text/event-stream") {
		return openaiImageJSONAsStreamHandler(c, info, resp)
	}

	usage := &dto.Usage{}
	var lastStreamData []byte
	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {
		raw := common.StringToByteSlice(data)
		lastStreamData = append(lastStreamData[:0], raw...)
		if isOpenAIImageStreamErrorEvent(raw) {
			sr.Stop(fmt.Errorf("%s", extractOpenAIImageStreamErrorMessage(raw)))
		}

		var usageResp dto.SimpleResponse
		if err := common.Unmarshal(raw, &usageResp); err == nil {
			normalizeOpenAIImageUsage(&usageResp.Usage)
			if service.ValidUsage(&usageResp.Usage) {
				usage = &usageResp.Usage
			}
		}
		if err := writeOpenaiImageStreamChunk(c, raw); err != nil {
			sr.Stop(err)
		}
	})

	if info != nil && info.StreamStatus != nil && info.StreamStatus.EndReason == relaycommon.StreamEndReasonDone {
		_ = helper.StringData(c, "[DONE]")
	}
	applyUsagePostProcessing(info, usage, lastStreamData)
	return usage, nil
}

func writeOpenaiImageStreamChunk(c *gin.Context, data []byte) error {
	var payload struct {
		Type string `json:"type"`
	}
	_ = common.Unmarshal(data, &payload)
	if eventName := strings.TrimSpace(payload.Type); eventName != "" {
		return helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: eventName}, string(data))
	}
	return helper.StringData(c, string(data))
}

func isOpenAIImageStreamErrorEvent(data []byte) bool {
	if !json.Valid(data) {
		return false
	}
	var payload struct {
		Type  string          `json:"type"`
		Error json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return false
	}
	payloadType := strings.ToLower(strings.TrimSpace(payload.Type))
	return payloadType == "error" || payloadType == "upstream_error" || len(payload.Error) > 0
}

func extractOpenAIImageStreamErrorMessage(data []byte) string {
	const fallback = "upstream image stream returned error event"
	if len(data) == 0 || !json.Valid(data) {
		return fallback
	}
	var payload struct {
		Message string          `json:"message"`
		Error   json.RawMessage `json:"error"`
	}
	if err := common.Unmarshal(data, &payload); err != nil {
		return fallback
	}
	if message := strings.TrimSpace(payload.Message); message != "" {
		return message
	}
	if len(payload.Error) > 0 {
		var nested struct {
			Message string `json:"message"`
		}
		if err := common.Unmarshal(payload.Error, &nested); err == nil {
			if message := strings.TrimSpace(nested.Message); message != "" {
				return message
			}
		}
		if message := strings.TrimSpace(common.JsonRawMessageToString(payload.Error)); message != "" {
			return message
		}
	}
	return fallback
}

func openaiImageJSONAsStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var usageResp dto.SimpleResponse
	if err := common.Unmarshal(responseBody, &usageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := usageResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	var imageResp dto.ImageResponse
	if err := common.Unmarshal(responseBody, &imageResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	normalizeOpenAIImageUsage(&usageResp.Usage)
	applyUsagePostProcessing(info, &usageResp.Usage, responseBody)

	helper.SetEventStreamHeaders(c)
	c.Header("Content-Length", "")
	c.Status(http.StatusOK)
	created := imageResp.Created
	if created == 0 {
		created = time.Now().Unix()
	}
	if info != nil {
		info.SetFirstResponseTime()
	}
	for _, image := range imageResp.Data {
		payload := map[string]any{
			"type":       "image_generation.completed",
			"created_at": created,
		}
		if image.Url != "" {
			payload["url"] = image.Url
		}
		if image.B64Json != "" {
			payload["b64_json"] = image.B64Json
		}
		if image.RevisedPrompt != "" {
			payload["revised_prompt"] = image.RevisedPrompt
		}
		if service.ValidUsage(&usageResp.Usage) {
			payload["usage"] = usageResp.Usage
		}
		data, marshalErr := common.Marshal(payload)
		if marshalErr != nil {
			return &usageResp.Usage, types.NewOpenAIError(marshalErr, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		if writeErr := helper.ResponseChunkData(c, dto.ResponsesStreamResponse{Type: "image_generation.completed"}, string(data)); writeErr != nil {
			setImageStreamWriteError(info, writeErr)
			return &usageResp.Usage, nil
		}
	}
	if writeErr := helper.StringData(c, "[DONE]"); writeErr != nil {
		setImageStreamWriteError(info, writeErr)
		return &usageResp.Usage, nil
	}
	if info != nil {
		info.ReceivedResponseCount += len(imageResp.Data)
		if info.StreamStatus == nil {
			info.StreamStatus = relaycommon.NewStreamStatus()
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	}
	return &usageResp.Usage, nil
}

func setImageStreamWriteError(info *relaycommon.RelayInfo, err error) {
	if info == nil {
		return
	}
	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, err)
}
