package render

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadHTML_InlineMarkupIsUsedVerbatim(t *testing.T) {
	const markup = `<div class="v">24°C</div>`
	got, err := loadHTML(markup)
	require.NoError(t, err)
	require.Equal(t, markup, got)
}

// Prompts are expanded by the runner before the stage runs. Braces that
// survive into the LLM's output (JSON, CSS-in-JS, Vue/Angular markup) used to
// hit a second template parse here and fail the whole pipeline.
func TestLoadHTML_LeavesBracesAlone(t *testing.T) {
	const markup = `<script>const cfg = {{"a": 1}};</script><style>.x{color:red}</style>`
	got, err := loadHTML(markup)
	require.NoError(t, err)
	require.Equal(t, markup, got)
}

func TestLoadHTML_ReadsFileWhenContentHasNoTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "page.html")
	const contents = "<html><body>hello</body></html>"
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))

	got, err := loadHTML(path)
	require.NoError(t, err)
	require.Equal(t, contents, got)
}

func TestLoadHTML_MissingFile(t *testing.T) {
	_, err := loadHTML(filepath.Join(t.TempDir(), "absent.html"))
	require.ErrorContains(t, err, "load template")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestParseSize(t *testing.T) {
	w, h, err := parseSize("800x480")
	require.NoError(t, err)
	require.Equal(t, 800, w)
	require.Equal(t, 480, h)

	for _, bad := range []string{"", "800", "axb", "0x480", "800x0", "-10x10"} {
		_, _, err := parseSize(bad)
		require.Error(t, err, "size %q should be rejected", bad)
	}
}

func TestQuoteEscapesBackticksAndBackslashes(t *testing.T) {
	require.Equal(t, "`a\\`b`", quote("a`b"))
	require.Equal(t, "`a\\\\b`", quote(`a\b`))
}
