package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminErrorRedactionKeepsHostAndRemovesCredentials(t *testing.T) {
	got := RedactErrorCredentials(`https://private.example/v1?api_key=secret Authorization: Bearer top-secret sk-test-credential failed`, "top-secret")
	require.Contains(t, got, "private.example")
	require.NotContains(t, got, "secret")
	require.NotContains(t, got, "sk-test-credential")
}

func TestErrorRedactionExcludesURLCredentialsAndEncodedQuery(t *testing.T) {
	for _, message := range []string{
		`Get "https://user:password@upstream.example/video?k%65y=FAKE%2BSECRET&key=SECOND": timeout`,
		`https://upstream.example/video?key=FAKE_SECRET#credential`,
		`key=FAKE_SECRET api-key=SECOND authorization: Bearer THIRD`,
	} {
		got := RedactErrorCredentials(message)
		for _, secret := range []string{"password", "FAKE", "SECOND", "THIRD", "credential"} {
			require.NotContains(t, got, secret)
		}
	}
	require.Equal(t, "https://upstream.example/video", SafeLogURL("https://user:pass@upstream.example/video?x=1#secret"))
}
