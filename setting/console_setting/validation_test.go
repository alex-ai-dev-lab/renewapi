package console_setting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExceedsMaxCharactersUsesUTF16Units(t *testing.T) {
	require.False(t, exceedsMaxCharacters(strings.Repeat("a", 500), 500))
	require.False(t, exceedsMaxCharacters(strings.Repeat("界", 500), 500))
	require.True(t, exceedsMaxCharacters(strings.Repeat("😀", 251), 500))
}

func TestAnnouncementExtraKeepsExistingLimitWithUTF16Counting(t *testing.T) {
	prefix := `[{"content":"ok","publishDate":"2026-08-14T00:00:00Z","extra":"`
	require.NoError(t, validateAnnouncements(prefix+strings.Repeat("a", 200)+`"}]`))
	require.ErrorContains(t, validateAnnouncements(prefix+strings.Repeat("😀", 101)+`"}]`), "200字符")
}
