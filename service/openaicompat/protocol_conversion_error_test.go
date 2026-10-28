package openaicompat

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/require"
)

func TestClaudeProtocolFeaturesAreDistinctFromMalformedInput(t *testing.T) {
	for _, scenario := range []struct {
		body        string
		unsupported bool
	}{
		{`{"tools":[{"name":"a","defer_loading":false,"input_schema":{}}]}`, true},
		{`{"thinking":{"type":"adaptive","display":"future-display"}}`, true},
		{`{"context_management":{"edits":[]}}`, true},
		{`{"messages":[{"role":"assistant","content":[{"type":"thinking","signature":"signed","thinking":"preserve"}]}]}`, true},
		{`{"tools":[{"input_schema":{}}]}`, false},
		{`{"messages":[{"content":"missing role"}]}`, false},
		{`{"output_config":{"format":{"type":"json_schema"}}}`, false},
	} {
		var request dto.ClaudeRequest
		require.NoError(t, common.Unmarshal([]byte(scenario.body), &request))
		_, err := ClaudeRequestToResponsesRequest(&request)
		require.Error(t, err, scenario.body)
		require.Equal(t, scenario.unsupported, IsUnsupportedProtocolConversion(err), scenario.body)
	}
}
