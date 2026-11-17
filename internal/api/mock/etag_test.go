package mock

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

func TestETag304(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	resp, err := tr.Do(context.Background(), get("/me/home"))
	require.NoError(t, err)
	etag := resp.Header.Get("ETag")
	assert.Regexp(t, `^"[0-9a-f]{16}"$`, etag)

	for _, inm := range []string{etag, "W/" + etag, `"x", ` + etag, "*"} {
		req := get("/me/home")
		req.Header.Set("If-None-Match", inm)
		r, err := tr.Do(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 304, r.Status, inm)
		assert.Nil(t, r.Body)
		assert.Equal(t, etag, r.Header.Get("ETag"))
	}

	req := get("/me/home")
	req.Header.Set("If-None-Match", `"0000000000000000"`)
	r, err := tr.Do(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.Status)
	assert.Equal(t, resp.Body, r.Body)

	// another body, another tag.
	require.NoError(t, tr.SetScenario("me.home", "new_user"))
	r, err = tr.Do(context.Background(), get("/me/home"))
	require.NoError(t, err)
	assert.NotEqual(t, etag, r.Header.Get("ETag"))
}

func TestETagOnlyOnGET200(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, req := range []*api.Request{
		{Method: http.MethodPatch, Path: "/me", Header: bearer()},
		{Method: http.MethodPost, Path: "/auth/forgot", Header: http.Header{}},
	} {
		req.Header.Set("If-None-Match", "*")
		r, err := tr.Do(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, 200, r.Status)
		assert.Empty(t, r.Header.Get("ETag"))
	}
	require.NoError(t, tr.SetScenario("me.home", "server_error"))
	req := get("/me/home")
	req.Header.Set("If-None-Match", "*")
	r, err := tr.Do(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 500, r.Status)
	assert.Empty(t, r.Header.Get("ETag"))
}

func TestNoETag(t *testing.T) {
	t.Parallel()
	tr := newTransport(t, func(o *Options) { o.NoETag = true })
	req := get("/me/home")
	req.Header.Set("If-None-Match", "*")
	r, err := tr.Do(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, 200, r.Status)
	assert.Empty(t, r.Header.Get("ETag"))
}
