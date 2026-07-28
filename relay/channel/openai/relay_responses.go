package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relay/antipoison"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type responsesPendingStreamData struct {
	response dto.ResponsesStreamResponse
	data     string
}

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if info != nil && info.ResponsesRequestKind == dto.ResponsesCompactionTrigger {
		if err := service.ValidateCompactionResponse(responseBody); err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		service.CaptureCompactionResponseAffinity(c, responseBody)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}
	cfg := antipoison.ResponseGuardConfig(info)
	antipoison.RecordStreamMode(c, cfg)
	if names, args := antipoison.ResponsesToolCallsFromResponse(&responsesResponse); len(names) > 0 {
		if err := antipoison.ValidateToolCallsAgainstPolicy(info, names, args); err != nil {
			antipoison.RecordToolPolicyFailure(c, err)
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(responseBody))
			logger.LogError(c, "anti-poison responses tool-call policy validation failed: "+err.Error())
			return nil, types.NewError(antipoison.FixedClientError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultPass)
	}
	if info.AntiPoisonResponseProofNonce != "" && antipoison.ResponseProofEnabled(info) {
		if err := antipoison.ValidateAndStripResponsesResponseProof(&responsesResponse, cfg, info.AntiPoisonResponseProofNonce); err != nil {
			antipoison.RecordProofFailure(c, err)
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(responseBody))
			logger.LogError(c, "anti-poison responses proof validation failed: "+err.Error())
			return nil, types.NewError(antipoison.ResponseProofFailureError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonProofResult, antipoison.ResultPass)
		responseBody, err = common.Marshal(responsesResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
	}
	if info.AntiPoisonGuardPrefix != "" {
		if err := antipoison.ValidateAndStripResponsesResponse(&responsesResponse, cfg, info.AntiPoisonGuardPrefix); err != nil {
			antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultFail)
			antipoison.RecordRisk(c, antipoison.RiskHard, "tool_call_guard", "block")
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(responseBody))
			logger.LogError(c, "anti-poison responses validation failed: "+err.Error())
			return nil, types.NewError(antipoison.FixedClientError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultPass)
		responseBody, err = common.Marshal(responsesResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
	}
	if info.AntiPoisonAnswerEnvelopeNonce != "" {
		if err := antipoison.ValidateAndStripResponsesAnswerEnvelope(&responsesResponse, info.AntiPoisonAnswerEnvelopeNonce, cfg); err != nil {
			antipoison.RecordEnvelopeFailure(c, err)
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(responseBody))
			logger.LogError(c, "anti-poison responses answer envelope validation failed: "+err.Error())
			return nil, types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonEnvelopeResult, antipoison.ResultPass)
		responseBody, err = common.Marshal(responsesResponse)
		if err != nil {
			return nil, types.NewOpenAIError(err, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
		}
	}
	if result := antipoison.ScanResponsesOpaquePayloadResult(&responsesResponse, cfg, ""); antipoison.OpaqueScanError(result) != nil {
		antipoison.RecordOpaqueResult(c, result)
		common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(responseBody))
		logger.LogError(c, "anti-poison responses opaque payload validation failed")
		return nil, types.NewError(antipoison.OpaqueScanError(result), types.ErrorCodeAntiPoisonValidationFailed)
	} else {
		antipoison.RecordOpaqueResult(c, result)
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.CacheWriteTokens = responsesResponse.Usage.InputTokensDetails.CacheWriteTokens
		}
	}
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamToResponseHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if info != nil && info.ResponsesRequestKind != dto.ResponsesNormal {
		responseBody, usage, err := aggregateCompactionResponsesStreamRaw(resp)
		if err != nil {
			return nil, err
		}
		c.Header("Content-Type", "application/json; charset=utf-8")
		service.CaptureCompactionResponseAffinity(c, responseBody)
		service.IOCopyBytesGracefully(c, resp, responseBody)
		return usage, nil
	}
	finalResp, usage, err := aggregateResponsesStreamToResponse(c, info, resp)
	if err != nil {
		return nil, err
	}
	responseBody, marshalErr := common.Marshal(finalResp)
	if marshalErr != nil {
		return nil, types.NewOpenAIError(marshalErr, types.ErrorCodeJsonMarshalFailed, http.StatusInternalServerError)
	}
	c.Header("Content-Type", "application/json; charset=utf-8")
	service.IOCopyBytesGracefully(c, resp, responseBody)
	return usage, nil
}

func aggregateCompactionResponsesStreamRaw(resp *http.Response) ([]byte, *dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)
	const maxItems = 64
	const maxItemBytes = 8 * 1024 * 1024
	const maxTotalBytes = 16 * 1024 * 1024
	items := make(map[int][]byte)
	totalBytes := 0
	var completed []byte
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), helper.DefaultMaxScannerBufferSize)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		data := bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		if len(data) == 0 || bytes.Equal(data, []byte("[DONE]")) {
			continue
		}
		if !gjson.ValidBytes(data) {
			return nil, nil, types.NewOpenAIError(fmt.Errorf("invalid responses SSE JSON"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
		typ := gjson.GetBytes(data, "type").String()
		switch typ {
		case dto.ResponsesOutputTypeItemAdded, dto.ResponsesOutputTypeItemDone:
			indexResult := gjson.GetBytes(data, "output_index")
			item := gjson.GetBytes(data, "item")
			if !indexResult.Exists() || !item.IsObject() {
				continue
			}
			index := int(indexResult.Int())
			if index < 0 || index >= maxItems || len(item.Raw) > maxItemBytes {
				return nil, nil, types.NewOpenAIError(fmt.Errorf("responses raw output ledger limit exceeded"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
			}
			if old, exists := items[index]; exists {
				totalBytes -= len(old)
			}
			items[index] = append([]byte(nil), item.Raw...)
			totalBytes += len(item.Raw)
			if totalBytes > maxTotalBytes {
				return nil, nil, types.NewOpenAIError(fmt.Errorf("responses raw output ledger total limit exceeded"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
			}
		case "response.completed":
			response := gjson.GetBytes(data, "response")
			if response.IsObject() {
				completed = append([]byte(nil), response.Raw...)
			}
		case "response.failed", "response.error":
			return nil, nil, types.NewOpenAIError(fmt.Errorf("responses stream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if len(completed) == 0 {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("responses stream missing response.completed"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if len(gjson.GetBytes(completed, "output").Array()) == 0 && len(items) > 0 {
		var output bytes.Buffer
		output.WriteByte('[')
		for i := 0; i < maxItems; i++ {
			item, ok := items[i]
			if !ok {
				continue
			}
			if output.Len() > 1 {
				output.WriteByte(',')
			}
			output.Write(item)
		}
		output.WriteByte(']')
		var err error
		completed, err = sjson.SetRawBytes(completed, "output", output.Bytes())
		if err != nil {
			return nil, nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
		}
	}
	if err := service.ValidateCompactionResponse(completed); err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	usage := &dto.Usage{
		PromptTokens:     int(gjson.GetBytes(completed, "usage.input_tokens").Int()),
		CompletionTokens: int(gjson.GetBytes(completed, "usage.output_tokens").Int()),
		TotalTokens:      int(gjson.GetBytes(completed, "usage.total_tokens").Int()),
	}
	usage.PromptTokensDetails.CachedTokens = int(gjson.GetBytes(completed, "usage.input_tokens_details.cached_tokens").Int())
	return completed, usage, nil
}

func aggregateResponsesStreamToResponse(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.OpenAIResponsesResponse, *dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, nil, types.NewOpenAIError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	defer service.CloseResponseBodyGracefully(resp)

	var (
		finalResp       *dto.OpenAIResponsesResponse
		outputText      strings.Builder
		functionCallMap = make(map[string]dto.ResponsesOutput)
		functionCallSeq []string
	)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, helper.InitialScannerBufferSize), helper.DefaultMaxScannerBufferSize)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		if strings.HasPrefix(data, "event:") {
			continue
		}
		if strings.HasPrefix(data, "data:") {
			data = strings.TrimSpace(strings.TrimPrefix(data, "data:"))
		}
		if data == "" || data == "[DONE]" {
			continue
		}

		var streamResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResp); err != nil {
			return nil, nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
		}
		switch streamResp.Type {
		case "response.output_text.delta":
			outputText.WriteString(streamResp.Delta)
		case dto.ResponsesOutputTypeItemAdded, dto.ResponsesOutputTypeItemDone:
			if streamResp.Item == nil || streamResp.Item.Type != "function_call" {
				break
			}
			callID := strings.TrimSpace(streamResp.Item.CallId)
			if callID == "" {
				callID = strings.TrimSpace(streamResp.Item.ID)
			}
			if callID == "" {
				break
			}
			if _, exists := functionCallMap[callID]; !exists {
				functionCallSeq = append(functionCallSeq, callID)
			}
			functionCallMap[callID] = *streamResp.Item
		case "response.function_call_arguments.delta":
			callID := strings.TrimSpace(streamResp.ItemID)
			if callID == "" {
				break
			}
			item := functionCallMap[callID]
			item.Type = "function_call"
			item.CallId = callID
			args := dto.ResponsesArgumentsString(item.Arguments) + streamResp.Delta
			if argsData, marshalErr := common.Marshal(args); marshalErr == nil {
				item.Arguments = argsData
			}
			if _, exists := functionCallMap[callID]; !exists {
				functionCallSeq = append(functionCallSeq, callID)
			}
			functionCallMap[callID] = item
		case "response.completed":
			finalResp = streamResp.Response
		case "response.error", "response.failed":
			if streamResp.Response != nil {
				if oaiErr := streamResp.Response.GetOpenAIError(); oaiErr != nil && oaiErr.Type != "" {
					return nil, nil, types.WithOpenAIError(*oaiErr, http.StatusInternalServerError)
				}
			}
			return nil, nil, types.NewOpenAIError(fmt.Errorf("responses stream error: %s", streamResp.Type), types.ErrorCodeBadResponse, http.StatusInternalServerError)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	if finalResp == nil {
		finalResp = buildFallbackResponsesResponse(c, info, outputText.String(), functionCallSeq, functionCallMap)
	} else if len(finalResp.Output) == 0 && (outputText.Len() > 0 || len(functionCallMap) > 0) {
		finalResp.Output = buildFallbackResponsesResponse(c, info, outputText.String(), functionCallSeq, functionCallMap).Output
	}
	if info != nil {
		cfg := antipoison.ResponseGuardConfig(info)
		antipoison.RecordStreamMode(c, cfg)
		if names, args := antipoison.ResponsesToolCallsFromResponse(finalResp); len(names) > 0 {
			if err := antipoison.ValidateToolCallsAgainstPolicy(info, names, args); err != nil {
				antipoison.RecordToolPolicyFailure(c, err)
				if b, marshalErr := common.Marshal(finalResp); marshalErr == nil {
					common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(b))
				}
				logger.LogError(c, "anti-poison aggregated responses tool-call policy validation failed: "+err.Error())
				return nil, nil, types.NewError(antipoison.FixedClientError(), types.ErrorCodeAntiPoisonValidationFailed)
			}
			antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultPass)
		}
	}
	if info != nil && info.AntiPoisonGuardPrefix != "" {
		cfg := antipoison.ResponseGuardConfig(info)
		if err := antipoison.ValidateAndStripResponsesResponse(finalResp, cfg, info.AntiPoisonGuardPrefix); err != nil {
			antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultFail)
			antipoison.RecordRisk(c, antipoison.RiskHard, "tool_call_guard", "block")
			if b, marshalErr := common.Marshal(finalResp); marshalErr == nil {
				common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(b))
			}
			logger.LogError(c, "anti-poison aggregated responses validation failed: "+err.Error())
			return nil, nil, types.NewError(antipoison.FixedClientError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonToolGuardResult, antipoison.ResultPass)
	}
	if info != nil && info.AntiPoisonResponseProofNonce != "" && antipoison.ResponseProofEnabled(info) {
		cfg := antipoison.ResponseGuardConfig(info)
		if err := antipoison.ValidateAndStripResponsesResponseProof(finalResp, cfg, info.AntiPoisonResponseProofNonce); err != nil {
			antipoison.RecordProofFailure(c, err)
			if b, marshalErr := common.Marshal(finalResp); marshalErr == nil {
				common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(b))
			}
			logger.LogError(c, "anti-poison aggregated responses proof validation failed: "+err.Error())
			return nil, nil, types.NewError(antipoison.ResponseProofFailureError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordResult(c, constant.ContextKeyAntiPoisonProofResult, antipoison.ResultPass)
	}
	if info != nil {
		cfg := antipoison.ResponseGuardConfig(info)
		if info.AntiPoisonAnswerEnvelopeNonce != "" {
			if err := antipoison.ValidateAndStripResponsesAnswerEnvelope(finalResp, info.AntiPoisonAnswerEnvelopeNonce, cfg); err != nil {
				antipoison.RecordEnvelopeFailure(c, err)
				if b, marshalErr := common.Marshal(finalResp); marshalErr == nil {
					common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(b))
				}
				logger.LogError(c, "anti-poison aggregated responses answer envelope validation failed: "+err.Error())
				return nil, nil, types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
			}
			antipoison.RecordResult(c, constant.ContextKeyAntiPoisonEnvelopeResult, antipoison.ResultPass)
		}
		if result := antipoison.ScanResponsesOpaquePayloadResult(finalResp, cfg, ""); antipoison.OpaqueScanError(result) != nil {
			antipoison.RecordOpaqueResult(c, result)
			if b, marshalErr := common.Marshal(finalResp); marshalErr == nil {
				common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, string(b))
			}
			logger.LogError(c, "anti-poison aggregated responses opaque payload validation failed")
			return nil, nil, types.NewError(antipoison.OpaqueScanError(result), types.ErrorCodeAntiPoisonValidationFailed)
		} else {
			antipoison.RecordOpaqueResult(c, result)
		}
	}
	usage := responsesUsageFromResponse(info, finalResp, outputText.String())
	if finalResp.Usage == nil {
		finalResp.Usage = usage
	}
	return finalResp, usage, nil
}

func buildFallbackResponsesResponse(c *gin.Context, info *relaycommon.RelayInfo, text string, functionCallSeq []string, functionCallMap map[string]dto.ResponsesOutput) *dto.OpenAIResponsesResponse {
	model := ""
	if info != nil {
		model = info.UpstreamModelName
		if strings.TrimSpace(info.OriginModelName) != "" {
			model = info.OriginModelName
		}
	}
	resp := &dto.OpenAIResponsesResponse{ID: helper.GetResponseID(c), Object: "response", CreatedAt: int(common.GetTimestamp()), Model: model}
	if text != "" {
		resp.Output = []dto.ResponsesOutput{{Type: "message", Role: "assistant", Status: "completed", Content: []dto.ResponsesOutputContent{{Type: "output_text", Text: text}}}}
		return resp
	}
	if len(functionCallMap) > 0 {
		resp.Output = make([]dto.ResponsesOutput, 0, len(functionCallSeq))
		seen := make(map[string]bool, len(functionCallSeq))
		for _, callID := range functionCallSeq {
			if seen[callID] {
				continue
			}
			seen[callID] = true
			resp.Output = append(resp.Output, functionCallMap[callID])
		}
	}
	return resp
}

func responsesUsageFromResponse(info *relaycommon.RelayInfo, resp *dto.OpenAIResponsesResponse, fallbackText string) *dto.Usage {
	usage := &dto.Usage{}
	if resp != nil && resp.Usage != nil {
		usage.PromptTokens = resp.Usage.InputTokens
		usage.CompletionTokens = resp.Usage.OutputTokens
		usage.TotalTokens = resp.Usage.TotalTokens
		if resp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = resp.Usage.InputTokensDetails.CachedTokens
			usage.PromptTokensDetails.CacheWriteTokens = resp.Usage.InputTokensDetails.CacheWriteTokens
		}
	}
	if usage.CompletionTokens == 0 && fallbackText != "" && info != nil {
		usage.CompletionTokens = service.CountTextToken(fallbackText, info.UpstreamModelName)
	}
	if usage.PromptTokens == 0 && info != nil {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return usage
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	var antiPoisonMarkers []string
	var antiPoisonTools []string
	var streamErr *types.NewAPIError
	antiPoisonToolSeen := map[string]bool{}
	var responseProof *antipoison.ProofStreamValidator
	preflightBuffer := antipoison.NewStreamPreflightBuffer(antipoison.ResponseGuardConfig(info))
	var pendingResponsesData []responsesPendingStreamData
	compactionItemSeen := false
	compactionCompleted := false
	if info.AntiPoisonResponseProofNonce != "" && antipoison.ResponseProofEnabled(info) {
		responseProof = antipoison.NewProofStreamValidator(info.AntiPoisonResponseProofNonce, antipoison.ResponseGuardConfig(info))
	}

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if info != nil && info.ResponsesRequestKind == dto.ResponsesCompactionTrigger {
			raw := []byte(data)
			switch streamResponse.Type {
			case dto.ResponsesOutputTypeItemDone:
				item := gjson.GetBytes(raw, "item")
				typ := item.Get("type").String()
				encrypted := item.Get("encrypted_content")
				if typ == "compaction" || typ == "context_compaction" || typ == "compaction_summary" {
					if encrypted.Type != gjson.String || encrypted.String() == "" || len(encrypted.String()) > common.GetEnvOrDefault("RESPONSES_COMPACTION_MAX_ENCRYPTED_BYTES", 8*1024*1024) {
						sr.Stop(fmt.Errorf("invalid compaction_summary.encrypted_content"))
						return
					}
					compactionItemSeen = true
					service.CaptureCompactionItemAffinity(c, []byte(item.Raw))
				}
			case "response.completed":
				response := gjson.GetBytes(raw, "response")
				if response.Get("object").String() != "response.compaction" || !response.Get("usage").IsObject() {
					sr.Stop(fmt.Errorf("invalid response.compaction completed event"))
					return
				}
				compactionCompleted = true
			}
		}
		if info.AntiPoisonGuardPrefix != "" {
			switch streamResponse.Type {
			case "response.output_text.delta":
				cfg := antipoison.ResponseGuardConfig(info)
				cleaned, markers := antipoison.StripResponsesTextDelta(streamResponse.Delta, cfg)
				if len(markers) > 0 {
					streamResponse.Delta = cleaned
					antiPoisonMarkers = append(antiPoisonMarkers, markers...)
					if b, marshalErr := common.Marshal(streamResponse); marshalErr == nil {
						data = string(b)
					} else {
						logger.LogError(c, "failed to marshal stripped responses stream chunk: "+marshalErr.Error())
						sr.Error(marshalErr)
					}
				}
			case dto.ResponsesOutputTypeItemAdded, dto.ResponsesOutputTypeItemDone:
				if streamResponse.Item != nil && streamResponse.Item.Type == "function_call" {
					if strings.TrimSpace(streamResponse.Item.Name) != "" {
						name := strings.TrimSpace(streamResponse.Item.Name)
						key := strings.TrimSpace(streamResponse.Item.CallId)
						if key == "" {
							key = strings.TrimSpace(streamResponse.Item.ID)
						}
						if key == "" {
							key = name
						}
						if !antiPoisonToolSeen[key] {
							antiPoisonToolSeen[key] = true
							antiPoisonTools = append(antiPoisonTools, name)
						}
					}
				}
			case "response.completed":
				if streamResponse.Response != nil {
					for _, out := range streamResponse.Response.Output {
						if out.Type == "function_call" && strings.TrimSpace(out.Name) != "" {
							name := strings.TrimSpace(out.Name)
							key := strings.TrimSpace(out.CallId)
							if key == "" {
								key = strings.TrimSpace(out.ID)
							}
							if key == "" {
								key = name
							}
							if !antiPoisonToolSeen[key] {
								antiPoisonToolSeen[key] = true
								antiPoisonTools = append(antiPoisonTools, name)
							}
						}
					}
				}
			}
		}
		if responseProof != nil && !responseProof.Verified() {
			cleanData, hold, proofErr := stripResponsesStreamResponseProof(data, &streamResponse, responseProof)
			if proofErr != nil {
				logger.LogError(c, "anti-poison responses proof stream validation failed: "+proofErr.Error())
				sr.Stop(proofErr)
				return
			}
			if hold {
				if cleanData != "" {
					pendingResponsesData = append(pendingResponsesData, responsesPendingStreamData{response: streamResponse, data: cleanData})
				}
				return
			} else {
				data = cleanData
				for _, pending := range pendingResponsesData {
					if err := sendResponsesStreamDataWithPreflight(c, info, preflightBuffer, pending.response, pending.data); err != nil {
						streamErr = err
						sr.Stop(err)
						return
					}
				}
				pendingResponsesData = nil
				if data != "" {
					if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
						logger.LogError(c, "failed to unmarshal proof-cleaned responses stream response: "+err.Error())
						sr.Stop(err)
						return
					}
					if err := sendResponsesStreamDataWithPreflight(c, info, preflightBuffer, streamResponse, data); err != nil {
						streamErr = err
						sr.Stop(err)
						return
					}
				}
			}
		} else {
			if err := sendResponsesStreamDataWithPreflight(c, info, preflightBuffer, streamResponse, data); err != nil {
				streamErr = err
				sr.Stop(err)
				return
			}
		}
		switch streamResponse.Type {
		case "response.completed":
			if streamResponse.Response != nil {
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
						usage.PromptTokensDetails.CacheWriteTokens = streamResponse.Response.Usage.InputTokensDetails.CacheWriteTokens
					}
				}
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		case dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
				switch streamResponse.Item.Type {
				case dto.BuildInCallWebSearchCall:
					if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
						if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
							webSearchTool.CallCount++
						}
					}
				}
			}
		}
	})
	if streamErr != nil {
		return nil, streamErr
	}
	if info != nil && info.ResponsesRequestKind == dto.ResponsesCompactionTrigger && (!compactionItemSeen || !compactionCompleted) {
		return nil, types.NewOpenAIError(fmt.Errorf("incomplete responses compaction stream"), types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	if responseProof != nil {
		if err := responseProof.Finalize(); err != nil {
			logger.LogError(c, "anti-poison responses proof stream validation failed: "+err.Error())
			return nil, types.NewError(antipoison.ResponseProofFailureError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
	}
	if info.AntiPoisonGuardPrefix != "" {
		cfg := antipoison.ResponseGuardConfig(info)
		if err := antipoison.ValidateOpenAIStreamFinal(antiPoisonMarkers, antiPoisonTools, cfg, info.AntiPoisonGuardPrefix); err != nil {
			logger.LogError(c, "anti-poison responses stream validation failed: "+err.Error())
			return nil, types.NewError(antipoison.FixedClientError(), types.ErrorCodeAntiPoisonValidationFailed)
		}
	}
	if preflightBuffer != nil {
		chunks, result, err := preflightBuffer.Finalize()
		if err != nil {
			antipoison.RecordOpaqueResult(c, result)
			common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, preflightBuffer.RawData())
			return nil, types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
		}
		antipoison.RecordOpaqueResult(c, result)
		for _, chunk := range chunks {
			var streamResponse dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(chunk, &streamResponse); err != nil {
				return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
			}
			sendResponsesStreamData(c, streamResponse, chunk)
		}
	}

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return usage, nil
}

func sendResponsesStreamDataWithPreflight(c *gin.Context, info *relaycommon.RelayInfo, buffer *antipoison.StreamPreflightBuffer, streamResponse dto.ResponsesStreamResponse, data string) *types.NewAPIError {
	if buffer == nil {
		sendResponsesStreamData(c, streamResponse, data)
		return nil
	}
	chunks, result, err := buffer.Add(data, responsesStreamVisibleText(streamResponse))
	if err != nil {
		antipoison.RecordOpaqueResult(c, result)
		common.SetContextKey(c, constant.ContextKeyAntiPoisonEvidenceResponse, buffer.RawData())
		return types.NewError(err, types.ErrorCodeAntiPoisonValidationFailed)
	}
	if len(chunks) > 0 {
		antipoison.RecordOpaqueResult(c, result)
	}
	for _, chunk := range chunks {
		var chunkResp dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(chunk, &chunkResp); err != nil {
			return types.NewError(err, types.ErrorCodeBadResponseBody)
		}
		sendResponsesStreamData(c, chunkResp, chunk)
	}
	_ = info
	return nil
}

func responsesStreamVisibleText(streamResponse dto.ResponsesStreamResponse) string {
	if streamResponse.Type != "response.output_text.delta" {
		return ""
	}
	return streamResponse.Delta
}

func stripResponsesStreamResponseProof(data string, streamResponse *dto.ResponsesStreamResponse, proof *antipoison.ProofStreamValidator) (string, bool, error) {
	if proof == nil || proof.Verified() || data == "" || streamResponse == nil {
		return data, false, nil
	}
	if streamResponse.Type != "response.output_text.delta" {
		return data, false, nil
	}
	cleaned, hold, err := proof.ProcessText(streamResponse.Delta)
	if err != nil {
		return "", true, err
	}
	if hold {
		return "", true, nil
	}
	streamResponse.Delta = cleaned
	b, err := common.Marshal(streamResponse)
	if err != nil {
		return data, false, err
	}
	return string(b), false, nil
}
