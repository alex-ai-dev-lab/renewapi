package openai

import (
	"bytes"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func OaiResponsesCompactionHandler(c *gin.Context, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}

	var compactResp dto.OpenAIResponsesCompactionResponse
	if err := common.Unmarshal(responseBody, &compactResp); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if err := service.ValidateCompactionResponse(responseBody); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway)
	}
	service.CaptureCompactionResponseAffinity(c, responseBody)
	if oaiError := compactResp.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if c.GetBool("responses_legacy_bridge_sse") {
		c.Header("Content-Type", "text/event-stream")
		output := gjson.GetBytes(responseBody, "output").Array()
		for index, item := range output {
			base := []byte(`{"type":"response.output_item.added","output_index":0}`)
			base, _ = sjson.SetBytes(base, "output_index", index)
			added, _ := sjson.SetRawBytes(base, "item", []byte(item.Raw))
			_, _ = c.Writer.Write([]byte("event: response.output_item.added\ndata: "))
			_, _ = c.Writer.Write(added)
			_, _ = c.Writer.Write([]byte("\n\n"))
			done := bytes.Replace(added, []byte("response.output_item.added"), []byte("response.output_item.done"), 1)
			_, _ = c.Writer.Write([]byte("event: response.output_item.done\ndata: "))
			_, _ = c.Writer.Write(done)
			_, _ = c.Writer.Write([]byte("\n\n"))
		}
		completed := []byte(`{"type":"response.completed"}`)
		completed, _ = sjson.SetRawBytes(completed, "response", responseBody)
		_, _ = c.Writer.Write([]byte("event: response.completed\ndata: "))
		_, _ = c.Writer.Write(completed)
		_, _ = c.Writer.Write([]byte("\n\n"))
		c.Writer.Flush()
	} else {
		service.IOCopyBytesGracefully(c, resp, responseBody)
	}

	usage := dto.Usage{}
	if compactResp.Usage != nil {
		usage.PromptTokens = compactResp.Usage.InputTokens
		usage.CompletionTokens = compactResp.Usage.OutputTokens
		usage.TotalTokens = compactResp.Usage.TotalTokens
		if compactResp.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = compactResp.Usage.InputTokensDetails.CachedTokens
		}
	}

	return &usage, nil
}
