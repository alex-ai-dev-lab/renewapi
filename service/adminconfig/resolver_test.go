package adminconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveUsesLastPresentLayer(t *testing.T) {
	value := Resolve("base_url", false,
		Layer{Source: SourceGlobal, Present: true, Value: "global"},
		Layer{Source: SourceGroup, SourceID: "test", Present: false},
		Layer{Source: SourceChannel, SourceID: "93", Present: true, Value: "channel"},
		Layer{Source: SourceRequest, Present: false},
	)

	require.Equal(t, SourceChannel, value.Source)
	require.Equal(t, "93", value.SourceID)
	require.Equal(t, "channel", value.Value)
	require.Len(t, value.Chain, 4)
}

func TestResolveMasksSensitiveValuesAndChain(t *testing.T) {
	value := Resolve("key", true,
		Layer{Source: SourceChannel, SourceID: "93", Present: true, Value: "secret"},
	)

	require.True(t, value.Masked)
	require.Nil(t, value.Value)
	require.Nil(t, value.Chain[0].Value)
	require.True(t, value.Chain[0].Present)
}

func TestResolvePreservesExplicitEmptyOverride(t *testing.T) {
	value := Resolve("header_override", false,
		Layer{Source: SourceGlobal, Present: true, Value: "global"},
		Layer{Source: SourceChannel, SourceID: "93", Present: true, Value: ""},
	)

	require.Equal(t, SourceChannel, value.Source)
	require.Equal(t, "", value.Value)
}
