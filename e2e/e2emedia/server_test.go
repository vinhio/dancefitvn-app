package e2emedia

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestServerServesAndGates(t *testing.T) {
	t.Parallel()

	s := startTest(t)
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Minute)
	tests := []struct {
		name   string
		url    string
		status int
		ctype  string
		body   string
	}{
		{"mock playlist", s.URL("hls/l_3503/480p.m3u8"), http.StatusOK, "application/vnd.apple.mpegurl", "#EXTM3U"},
		{"mock mp3", s.URL("audio/_sample.mp3"), http.StatusOK, "audio/mpeg", "ID3"},
		{"signed, valid", s.Signed("hls/l_3503/480p.m3u8", future), http.StatusOK, "application/vnd.apple.mpegurl", "#EXTM3U"},
		{"signed, expired", s.Signed("hls/l_3503/480p.m3u8", past), http.StatusForbidden, "", "forbidden"},
		{"long playlist", s.Long("l_3503", 2, future), http.StatusOK, "application/vnd.apple.mpegurl", "#EXT-X-DISCONTINUITY"},
		{"long playlist, expired", s.Long("l_3503", 2, past), http.StatusForbidden, "", "forbidden"},
		{"outside the gate", strings.Replace(s.Signed("x", future), "/g/", "/", 1), http.StatusNotFound, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, ctype, body := get(t, tt.url)
			assert.Equal(t, tt.status, status)
			if tt.ctype != "" {
				assert.Equal(t, tt.ctype, ctype)
			}
			assert.Contains(t, body, tt.body)
		})
	}
}

func TestLongPlaylist(t *testing.T) {
	t.Parallel()

	got := longPlaylist("l_3503", 2, "99")
	assert.Equal(t, 6, strings.Count(got, "#EXTINF:4.000000,"))
	assert.Equal(t, 1, strings.Count(got, "#EXT-X-DISCONTINUITY"))
	assert.Contains(t, got, "/g/hls/l_3503/480p_002.ts?exp=99&n=5\n")
	assert.True(t, strings.HasSuffix(got, "#EXT-X-ENDLIST\n"))
}

func TestRevoke(t *testing.T) {
	t.Parallel()

	s := startTest(t)
	link := s.Signed("hls/l_3503/480p_000.ts", time.Now().Add(time.Hour))
	status, _, _ := get(t, link)
	require.Equal(t, http.StatusOK, status)
	s.Revoke(true)
	status, _, _ = get(t, link)
	assert.Equal(t, http.StatusForbidden, status)
	s.Revoke(false)
	status, _, _ = get(t, link)
	assert.Equal(t, http.StatusOK, status)
	served, refused := s.Counts()
	assert.Equal(t, int64(2), served)
	assert.Equal(t, int64(1), refused)
}

func TestStartWithoutFolder(t *testing.T) {
	t.Parallel()

	_, err := Start(filepath.Join(t.TempDir(), "none"), nil)
	require.Error(t, err)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// startTest serves a folder of stand-in files, closed after the test.
func startTest(t *testing.T) *Server {
	t.Helper()

	dir := t.TempDir()
	files := map[string]string{
		"hls/l_3503/480p.m3u8":     "#EXTM3U\n",
		"hls/l_3503/480p_000.ts":   "ts",
		"audio/_sample.mp3":        "ID3",
		"hls/l_3503/master.m3u8":   "#EXTM3U\n",
		"hls/l_3503/480p_001.ts":   "ts",
		"hls/l_3503/480p_002.ts":   "ts",
		"audio/l_3503.unused.json": "{}",
	}
	for name, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte(body), 0o600))
	}
	s, err := Start(dir, t.Logf)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, s.Close()) })

	return s
}

// get fetches url and returns its status, Content-Type and body.
func get(t *testing.T, url string) (status int, ctype, body string) {
	t.Helper()

	resp, err := http.Get(url) //nolint:gosec,noctx // a loopback link of the test's own server
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, resp.Header.Get("Content-Type"), string(b)
}
