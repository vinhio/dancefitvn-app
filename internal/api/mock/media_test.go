package mock

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

// serve starts the media server of a fresh transport and returns its URL.
func serve(t *testing.T) (*Transport, string) {
	t.Helper()
	tr := newTransport(t)
	require.NoError(t, tr.MediaServer().Ensure())

	return tr, tr.MediaServer().BaseURL()
}

// fetch does a request with a client that keeps no connection.
func fetch(t *testing.T, method, url string, hdr map[string]string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, nil)
	require.NoError(t, err)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	resp, err := noKeepAliveClient().Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, body
}

func TestMediaRewrite(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	assert.Empty(t, tr.MediaServer().BaseURL(), "the server starts lazily")
	n := 0
	for _, e := range expected {
		var route Route
		for _, r := range tr.Routes() {
			if r.ID == e.route {
				route = r
			}
		}
		require.NoError(t, tr.SetScenario(e.route, e.scenario))
		resp, err := tr.Do(context.Background(), &api.Request{Method: route.Method, Path: concretePath(route.Path), Header: bearer()})
		require.NoError(t, err)
		assert.NotContains(t, string(resp.Body), "mock-media://", "%s/%s", e.route, e.scenario)
		if strings.Contains(string(resp.Body), "http://127.0.0.1:") {
			n++
			assert.Contains(t, string(resp.Body), tr.MediaServer().BaseURL()+"/")
		}
	}
	assert.Positive(t, n)
}

func TestMediaServerRange(t *testing.T) {
	t.Parallel()
	_, base := serve(t)
	want, err := os.ReadFile(mockDir + "/media/audio/l_3503.mp3")
	require.NoError(t, err)

	resp, body := fetch(t, "GET", base+"/audio/l_3503.mp3?exp=1&sig=x", map[string]string{"Range": "bytes=0-1"})
	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, want[:2], body)
	assert.Equal(t, "bytes 0-1/"+itoa(len(want)), resp.Header.Get("Content-Range"))
	assert.Equal(t, "audio/mpeg", resp.Header.Get("Content-Type"))

	resp, body = fetch(t, "GET", base+"/audio/l_3503.mp3", map[string]string{"Range": "bytes=-10"})
	assert.Equal(t, http.StatusPartialContent, resp.StatusCode)
	assert.Equal(t, want[len(want)-10:], body)

	resp, body = fetch(t, "GET", base+"/audio/l_3503.mp3", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"))
	assert.Equal(t, want, body)

	resp, body = fetch(t, "HEAD", base+"/audio/l_3503.mp3", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Empty(t, body)
	assert.Equal(t, itoa(len(want)), resp.Header.Get("Content-Length"))
	assert.Equal(t, "audio/mpeg", resp.Header.Get("Content-Type"))

	resp, _ = fetch(t, "GET", base+"/audio/l_3503.mp3", map[string]string{"Range": "bytes=" + itoa(len(want)+5) + "-"})
	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, resp.StatusCode)
	assert.Equal(t, "bytes */"+itoa(len(want)), resp.Header.Get("Content-Range"))

	for _, m := range []string{"POST", "PUT", "DELETE", "PATCH"} {
		resp, _ = fetch(t, m, base+"/audio/l_3503.mp3", nil)
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, m)
		assert.Equal(t, "GET, HEAD", resp.Header.Get("Allow"))
	}
}

func TestMediaContentTypes(t *testing.T) {
	t.Parallel()
	_, base := serve(t)
	for _, c := range []struct{ path, ct string }{
		{"/hls/l_3503/master.m3u8", "application/vnd.apple.mpegurl"},
		{"/hls/l_3503/480p.m3u8", "application/vnd.apple.mpegurl"},
		{"/hls/l_3503/480p_000.ts", "video/mp2t"},
		{"/audio/l_3503.mp3", "audio/mpeg"},
		{"/covers/set-35.jpg", "image/jpeg"},
		{"/avatars/u_412.jpg", "image/jpeg"},
		{"/badges/hdv_stepx.png", "image/png"},
	} {
		resp, body := fetch(t, "GET", base+c.path, nil)
		assert.Equal(t, 200, resp.StatusCode, c.path)
		assert.Equal(t, c.ct, resp.Header.Get("Content-Type"), c.path)
		assert.NotEmpty(t, body, c.path)
	}
	resp, body := fetch(t, "GET", base+"/hls/l_3503/master.m3u8", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, strings.HasPrefix(string(body), "#EXTM3U"))
}

func TestMediaNotFoundAndTraversal(t *testing.T) {
	t.Parallel()
	_, base := serve(t)
	for _, p := range []string{"/", "/nope.jpg", "/covers/", "/covers", "/hls", "/../routes.json", "/%2e%2e/routes.json", "/covers/../../routes.json", "/hls/l_3503/nope.ts", "/audio/x.wav", "/audio/a/b.mp3"} {
		resp, _ := fetch(t, "GET", base+p, nil)
		assert.Equal(t, 404, resp.StatusCode, p)
	}
}

func TestMediaFallback(t *testing.T) {
	t.Parallel()
	tr, base := serve(t)
	for _, c := range []struct{ got, want string }{
		{"/hls/l_2401/master.m3u8", "hls/_sample/master.m3u8"},
		{"/hls/l_2401/480p.m3u8", "hls/_sample/480p.m3u8"},
		{"/hls/l_2401/480p_001.ts", "hls/_sample/480p_001.ts"},
		{"/audio/l_2401.mp3", "audio/_sample.mp3"},
	} {
		want, err := os.ReadFile(mockDir + "/media/" + c.want)
		require.NoError(t, err)
		resp, body := fetch(t, "GET", base+c.got, nil)
		assert.Equal(t, 200, resp.StatusCode, c.got)
		assert.Equal(t, want, body, c.got)
	}
	// a lesson that has its own files keeps them.
	own, err := os.ReadFile(mockDir + "/media/audio/l_3503.mp3")
	require.NoError(t, err)
	sample, err := os.ReadFile(mockDir + "/media/audio/_sample.mp3")
	require.NoError(t, err)
	assert.NotEqual(t, own, sample)

	name, ok := tr.MediaServer().Resolve("audio/l_3503.mp3")
	assert.True(t, ok)
	assert.Equal(t, "audio/l_3503.mp3", name)
	_, ok = tr.MediaServer().Resolve("covers/none.jpg")
	assert.False(t, ok)
}

func TestMediaStreamEndToEnd(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	resp, err := tr.Do(context.Background(), get("/lessons/l_2401/stream"))
	require.NoError(t, err)
	var s struct {
		HLS string `json:"hls_url"`
	}
	require.NoError(t, json.Unmarshal(resp.Body, &s))
	assert.True(t, strings.HasPrefix(s.HLS, "http://127.0.0.1:"), s.HLS)
	r, body := fetch(t, "GET", s.HLS, nil)
	assert.Equal(t, 200, r.StatusCode)
	assert.Equal(t, "application/vnd.apple.mpegurl", r.Header.Get("Content-Type"))
	assert.True(t, strings.HasPrefix(string(body), "#EXTM3U"))
}

func TestAudioDigestMatchesServedFile(t *testing.T) {
	t.Parallel()
	for _, lesson := range []string{"l_3503", "l_2401"} { // own file, _sample
		tr := newTransport(t)
		resp, err := tr.Do(context.Background(), get("/lessons/"+lesson+"/audio"))
		require.NoError(t, err)
		var a struct {
			URL    string `json:"url"`
			Size   int64  `json:"size_bytes"`
			SHA256 string `json:"sha256"`
			File   string `json:"filename"`
		}
		require.NoError(t, json.Unmarshal(resp.Body, &a))
		r, body := fetch(t, "GET", a.URL, nil)
		assert.Equal(t, 200, r.StatusCode)
		sum := sha256.Sum256(body)
		assert.Equal(t, hex.EncodeToString(sum[:]), a.SHA256, lesson)
		assert.Equal(t, int64(len(body)), a.Size, lesson)
		assert.NotEmpty(t, a.File)

		// computed once and the same again.
		again, err := tr.Do(context.Background(), get("/lessons/"+lesson+"/audio"))
		require.NoError(t, err)
		assert.JSONEq(t, string(resp.Body), string(again.Body))
	}
}

func TestKeepAudioDigest(t *testing.T) {
	t.Parallel()
	tr := newTransport(t, func(o *Options) { o.KeepAudioDigest = true })
	resp, err := tr.Do(context.Background(), get("/lessons/l_3503/audio"))
	require.NoError(t, err)
	var a struct {
		Size   int64  `json:"size_bytes"`
		SHA256 string `json:"sha256"`
	}
	require.NoError(t, json.Unmarshal(resp.Body, &a))
	assert.Equal(t, int64(8800000), a.Size)
	assert.Equal(t, "3f1c9a7e0b5d4c2e8a6f1b9d7c5e3a1f0e2d4c6b8a9f7e5d3c1b2a4f6e8d0c9b", a.SHA256)
}

func TestAudioDigestSkipsErrors(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	require.NoError(t, tr.SetScenario("lessons.audio", "not_enrolled"))
	resp, err := tr.Do(context.Background(), get("/lessons/l_3503/audio"))
	require.NoError(t, err)
	assert.Equal(t, 403, resp.Status)
	assert.NotContains(t, string(resp.Body), "size_bytes")
	assert.Empty(t, tr.MediaServer().BaseURL(), "an error body needs no media server")
}

func TestPatchAudioLeavesStrangeBodies(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, in := range []string{`not json`, `{"a":1}`, `{"url":5}`, `{"url":"https://x/y.mp3"}`} {
		out, err := tr.patchAudio([]byte(in))
		require.NoError(t, err)
		assert.Equal(t, in, string(out))
	}
	_, err := tr.patchAudio([]byte(`{"url":"mock-media://covers/none.jpg"}`))
	assert.Error(t, err)
}
