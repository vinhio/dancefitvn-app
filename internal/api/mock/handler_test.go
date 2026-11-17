package mock

import (
	"context"

	"dancefitvn-app/internal/api"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandlerServesAPI(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	srv := httptest.NewServer(tr.Handler())
	t.Cleanup(srv.Close)
	c := noKeepAliveClient()

	do := func(method, path, auth string, hdr map[string]string) (*http.Response, []byte) {
		req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, strings.NewReader(`{"x":1}`))
		require.NoError(t, err)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		resp, err := c.Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		return resp, b
	}

	resp, body := do("GET", "/api/v1/app/config", "", nil)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	assert.True(t, jsontext.Value(body).IsValid())

	resp, _ = do("GET", "/api/v1/me", "", nil)
	assert.Equal(t, 401, resp.StatusCode)
	resp, _ = do("GET", "/api/v1/me", "Bearer t", nil)
	assert.Equal(t, 200, resp.StatusCode)
	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)

	resp, body = do("GET", "/api/v1/me", "Bearer t", map[string]string{"If-None-Match": etag})
	assert.Equal(t, 304, resp.StatusCode)
	assert.Empty(t, body)

	resp, body = do("POST", "/api/v1/tv/logout", "Bearer t", nil)
	assert.Equal(t, 204, resp.StatusCode)
	assert.Empty(t, body)

	resp, _ = do("GET", "/api/v1/courses/set-99", "Bearer t", nil)
	assert.Equal(t, 404, resp.StatusCode)
	resp, _ = do("GET", "/api/v1/courses/a%2Fb", "Bearer t", nil)
	assert.Equal(t, 404, resp.StatusCode)

	// media URLs in handler bodies point at the media server.
	resp, body = do("GET", "/api/v1/lessons/l_3503/stream", "Bearer t", nil)
	require.Equal(t, 200, resp.StatusCode)
	var s struct {
		HLS string `json:"hls_url"`
	}
	require.NoError(t, json.Unmarshal(body, &s))
	r2, _ := fetch(t, "GET", s.HLS, nil)
	assert.Equal(t, 200, r2.StatusCode)

	resp, _ = do("HEAD", "/api/v1/me", "Bearer t", nil)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHandlerRequestTooLarge(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/v1/auth/login", strings.NewReader(strings.Repeat("a", maxRequestBody+1)))
	tr.Handler().ServeHTTP(rec, req)
	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}

// viaSpy records the request of the route it is asked and answers 204.
type viaSpy struct{ got []string }

func (s *viaSpy) Do(_ context.Context, req *api.Request) (*api.Response, error) {
	s.got = append(s.got, req.RouteID)

	return &api.Response{Status: http.StatusNoContent, Header: http.Header{}}, nil
}

func TestHandlerOverCarriesRouteID(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	spy := &viaSpy{}
	for _, p := range []string{"/api/v1/me", "/api/v1/tv/pair/start", "/api/v1/nothing"} {
		m := http.MethodGet
		if strings.Contains(p, "pair") {
			m = http.MethodPost
		}
		rec := httptest.NewRecorder()
		tr.HandlerOver(spy).ServeHTTP(rec, httptest.NewRequest(m, p, nil))
		assert.Equal(t, http.StatusNoContent, rec.Code)
	}
	assert.Equal(t, []string{"me.get", "tv.pair.start", ""}, spy.got)
	assert.Equal(t, "tv.pair.poll", tr.RouteID(http.MethodPost, "/tv/pair/poll"))
}
