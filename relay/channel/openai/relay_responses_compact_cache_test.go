package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestResponsesCompactionPreservesCacheWriteTokens(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(
			`{"object":"response.compaction","output":[{"type":"compaction","encrypted_content":"ciphertext"}],"usage":{"input_tokens":100,"output_tokens":5,"total_tokens":105,"input_tokens_details":{"cached_tokens":20,"cache_write_tokens":30}}}`,
		)),
	}
	usage, apiErr := OaiResponsesCompactionHandler(ctx, resp)
	require.Nil(t, apiErr)
	require.Equal(t, 30, usage.PromptTokensDetails.CacheWriteTokens)
}
