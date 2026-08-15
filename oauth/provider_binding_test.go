package oauth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuiltInProviderBindingColumns(t *testing.T) {
	tests := []struct {
		provider BindingColumnProvider
		want     string
	}{
		{provider: &GitHubProvider{}, want: "github_id"},
		{provider: &DiscordProvider{}, want: "discord_id"},
		{provider: &OIDCProvider{}, want: "oidc_id"},
		{provider: &LinuxDOProvider{}, want: "linux_do_id"},
	}

	for _, test := range tests {
		require.Equal(t, test.want, test.provider.BindingColumn())
	}

	var generic any = &GenericOAuthProvider{}
	_, ok := generic.(BindingColumnProvider)
	require.False(t, ok, "generic OAuth bindings must remain in user_oauth_bindings")
}
