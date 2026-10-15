package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestResponsesNamespaceToMessagesRoundTrip(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, lifted := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream_%t_additional_tools_%t", stream, lifted), func(t *testing.T) {
				db := setupEmptyStreamRecoveryDB(t)
				seedRecoveryPrincipal(t, db)
				old := operation_setting.ModelEndpointDefaults2JsonString()
				t.Cleanup(func() { require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(old)) })
				require.NoError(t, operation_setting.UpdateModelEndpointDefaultsByJsonString(fmt.Sprintf(`{"enabled":true,"normalize_text_endpoints":true,"entries":[{"match_type":"exact","pattern":"%s","channel_type":1,"default_endpoint":"anthropic"}]}`, failoverTestModel)))
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					if r.URL.Path != "/v1/messages" || !bytes.Contains(body, []byte(`"name":"team__lookup"`)) {
						t.Errorf("incorrect Messages conversion: path=%s body=%s", r.URL.Path, body)
						w.WriteHeader(400)
						return
					}
					fixture := httptest.NewRecorder()
					writeBridgeFixture(t, fixture, "anthropic", stream, 1)
					w.Header().Set("Content-Type", fixture.Header().Get("Content-Type"))
					_, _ = io.WriteString(w, strings.ReplaceAll(fixture.Body.String(), "lookup", "team__lookup"))
				}))
				t.Cleanup(server.Close)
				channel := recoveryIntegrationChannel(9300, "namespace-messages", server.URL, 100, failoverTestModel)
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, channel.AddAbilities(nil))
				model.InitChannelCache()
				payload := bridgeClientRequest("openai-response", stream, 1)
				tools := []any{map[string]any{"type": "namespace", "name": "team", "tools": payload["tools"]}}
				payload["tool_choice"] = map[string]any{"type": "function", "namespace": "team", "name": "lookup"}
				if lifted {
					delete(payload, "tools")
					payload["input"] = append(payload["input"].([]any), map[string]any{"type": "additional_tools", "tools": tools})
				} else {
					payload["tools"] = tools
				}
				body, err := common.Marshal(payload)
				require.NoError(t, err)
				router, _, _ := recoveryIntegrationRouter(fmt.Sprintf("namespace-roundtrip-%t-%t", stream, lifted))
				req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				router.ServeHTTP(w, req)
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				require.Equal(t, 1, calls)
				require.Contains(t, w.Body.String(), `"namespace":"team"`)
				require.Contains(t, w.Body.String(), `"name":"lookup"`)
				require.NotContains(t, w.Body.String(), `"name":"team__lookup"`)
				perfmetrics.WaitForPendingSamples()
			})
		}
	}
}
