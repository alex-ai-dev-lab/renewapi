package dto

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageRequestStreamJSONPreservesPresence(t *testing.T) {
	var enabled ImageRequest
	require.NoError(t, enabled.UnmarshalJSON([]byte(`{"model":"gpt-image-1","prompt":"cat","stream":true}`)))
	require.NotNil(t, enabled.Stream)
	require.True(t, *enabled.Stream)
	require.True(t, enabled.IsStream(nil))

	var disabled ImageRequest
	require.NoError(t, disabled.UnmarshalJSON([]byte(`{"model":"gpt-image-1","prompt":"cat","stream":false}`)))
	require.NotNil(t, disabled.Stream)
	require.False(t, *disabled.Stream)
	require.False(t, disabled.IsStream(nil))

	var omitted ImageRequest
	require.NoError(t, omitted.UnmarshalJSON([]byte(`{"model":"gpt-image-1","prompt":"cat"}`)))
	require.Nil(t, omitted.Stream)
}
