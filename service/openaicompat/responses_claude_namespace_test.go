package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestResponsesToClaudePreservesNamespaceTools(t *testing.T) {
	var req dto.OpenAIResponsesRequest
	require.NoError(t, common.UnmarshalJsonStr(`{"model":"test","tools":[{"type":"namespace","name":"team","tools":[{"type":"function","name":"send","parameters":{"type":"object"}}]}],"input":[{"type":"function_call","id":"fc_1","call_id":"call_1","namespace":"team","name":"send","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"ok"}]}`, &req))
	out, err := ResponsesRequestToClaudeRequest(&req)
	require.NoError(t, err)
	tools := out.GetTools()
	require.Len(t, tools, 1)
	encoded, err := common.Marshal(tools)
	require.NoError(t, err)
	require.Contains(t, string(encoded), `"name":"team__send"`)
	parts, err := out.Messages[0].ParseContent()
	require.NoError(t, err)
	require.Equal(t, "team__send", parts[0].Name)
	require.Equal(t, "call_1", parts[0].Id)
	require.Contains(t, string(req.Tools), `"type":"namespace"`, "do not mutate original request across channel attempts")
}
