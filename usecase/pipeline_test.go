package usecase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTags(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		maxTags int
		want    []string
	}{
		{
			name:    "comma separated",
			text:    "sunset, ocean, calm",
			maxTags: 10,
			want:    []string{"sunset", "ocean", "calm"},
		},
		{
			name:    "lowercases",
			text:    "Sunset OCEAN Calm",
			maxTags: 10,
			want:    []string{"sunset", "ocean", "calm"},
		},
		{
			name:    "deduplicates case-insensitively",
			text:    "sunset, Sunset, SUNSET, ocean",
			maxTags: 10,
			want:    []string{"sunset", "ocean"},
		},
		{
			name:    "truncates at maxTags",
			text:    "one two three four five",
			maxTags: 3,
			want:    []string{"one", "two", "three"},
		},
		{
			name:    "drops tokens containing non-letters",
			text:    "sunset sunset2024 2024 !!! sky-line ocean",
			maxTags: 10,
			want:    []string{"sunset", "ocean"},
		},
		{
			name:    "drops single-character tokens",
			text:    "a an the ocean",
			maxTags: 10,
			want:    []string{"an", "the", "ocean"},
		},
		{
			name:    "splits on whitespace, commas and semicolons",
			text:    "alpha,beta;gamma\ndelta\tepsilon",
			maxTags: 10,
			want:    []string{"alpha", "beta", "gamma", "delta", "epsilon"},
		},
		{
			name:    "empty input yields nothing",
			text:    "",
			maxTags: 10,
			want:    nil,
		},
		{
			name:    "whitespace only yields nothing",
			text:    "   \n\t ",
			maxTags: 10,
			want:    nil,
		},
		{
			name:    "all tokens rejected yields nothing",
			text:    "123, 456, 7-8",
			maxTags: 10,
			want:    nil,
		},
		{
			// The cap is checked after appending, so maxTags <= 0 still
			// yields one tag. Run() never passes that: it clamps to 10.
			name:    "maxTags of zero still yields one tag",
			text:    "sunset ocean",
			maxTags: 0,
			want:    []string{"sunset"},
		},
		{
			name:    "deduplication happens before the cap",
			text:    "sunset sunset ocean calm",
			maxTags: 2,
			want:    []string{"sunset", "ocean"},
		},
		{
			// The minimum-length check counts bytes, not runes, so a
			// single multi-byte letter survives it.
			name:    "non-ASCII letters are kept",
			text:    "夕焼け, 海",
			maxTags: 10,
			want:    []string{"夕焼け", "海"},
		},
		{
			name:    "single-byte tokens are dropped",
			text:    "a b sunset",
			maxTags: 10,
			want:    []string{"sunset"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, parseTags(tt.text, tt.maxTags))
		})
	}
}

func TestWithDefaults(t *testing.T) {
	require.Equal(t, "given", withDefaultStr("given", "fallback"))
	require.Equal(t, "fallback", withDefaultStr("", "fallback"))
	require.Equal(t, 5, withDefaultInt(5, 10))
	require.Equal(t, 10, withDefaultInt(0, 10))
}

func TestIsAlphaOnly(t *testing.T) {
	require.True(t, isAlphaOnly("sunset"))
	require.True(t, isAlphaOnly("夕焼け"))
	require.False(t, isAlphaOnly("sunset2024"))
	require.False(t, isAlphaOnly("sky-line"))
	require.False(t, isAlphaOnly("!!!"))
	require.True(t, isAlphaOnly(""), "vacuously true for the empty string")
}
