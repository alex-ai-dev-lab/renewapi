package channelconfig

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildChangeSetUsesSemanticJSONAndClassifiesOnce(t *testing.T) {
	beforeMapping := `{"glm-5.2":["upstream-a","upstream-b"]}`
	afterMapping := `{ "glm-5.2": [ "upstream-a", "upstream-b" ] }`
	before := &model.Channel{Name: "same", ModelMapping: &beforeMapping}
	after := &model.Channel{Name: "same", ModelMapping: &afterMapping}

	changes := BuildChangeSet(before, after, map[string]bool{
		"name": true, "model_mapping": true,
	}, false, nil, nil)
	require.False(t, changes.Changed())

	changedMapping := `{"glm-5.2":["upstream-b"]}`
	after.ModelMapping = &changedMapping
	changes = BuildChangeSet(before, after, map[string]bool{"model_mapping": true}, false, nil, nil)
	require.Equal(t, []string{"model_mapping"}, changes.ChangedFields)
	require.True(t, changes.AbilityChanged)
	require.True(t, changes.RoutingChanged)
	require.False(t, changes.TransportChanged)
}

func TestBuildChangeSetTreatsEndpointOrderAsIrrelevant(t *testing.T) {
	channel := &model.Channel{}
	left := []*model.ModelEndpoint{{Model: "a", BaseURL: "https://a.example"}, {Model: "b"}}
	right := []*model.ModelEndpoint{{Model: "b"}, {Model: "a", BaseURL: "https://a.example"}}
	changes := BuildChangeSet(channel, channel, map[string]bool{}, false, left, &right)
	require.False(t, changes.Changed())
}

func TestBuildChangeSetClassifiesCombinedChannelSetting(t *testing.T) {
	beforeSetting := `{"proxy":"","responses_compaction_capability":"unknown"}`
	afterSetting := `{ "responses_compaction_capability": "native_v2", "proxy": "" }`
	before := &model.Channel{Setting: &beforeSetting}
	after := &model.Channel{Setting: &afterSetting}

	changes := BuildChangeSet(before, after, map[string]bool{"setting": true}, false, nil, nil)
	require.Equal(t, []string{"setting"}, changes.ChangedFields)
	require.True(t, changes.TransportChanged)
	require.True(t, changes.RoutingChanged)
	require.True(t, changes.ProtocolChanged)

	afterSetting = `{ "proxy": "", "responses_compaction_capability": "unknown" }`
	changes = BuildChangeSet(before, after, map[string]bool{"setting": true}, false, nil, nil)
	require.False(t, changes.Changed())
}
