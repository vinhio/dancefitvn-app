package api

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestETag304UsesCache(t *testing.T) {
	t.Parallel()

	home := string(mockFile(t, "me/home.json"))
	tr := &scriptTransport{fn: func(n int, r *Request) (*Response, error) {
		if r.Header.Get("If-None-Match") == `"abc"` {
			return &Response{Status: 304, Header: http.Header{"Etag": {`"abc"`}}}, nil
		}

		return &Response{Status: 200, Header: http.Header{"Etag": {`"abc"`}}, Body: []byte(home)}, nil
	}}
	cache := NewMemoryCache()
	c := newTestClient(t, tr, func(o *Options) { o.Cache = cache })
	ctx := context.Background()

	first, err := c.Home(ctx)
	require.NoError(t, err)
	second, err := c.Home(ctx)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.NotEmpty(t, first.GreetingName)

	reqs := tr.seen()
	require.Len(t, reqs, 2)
	assert.Empty(t, reqs[0].Header.Get("If-None-Match"))
	assert.Equal(t, `"abc"`, reqs[1].Header.Get("If-None-Match"))

	etag, body, ok := cache.Get("GET /me/home?")
	assert.True(t, ok)
	assert.Equal(t, `"abc"`, etag)
	assert.Equal(t, home, string(body))
}

func TestETagKeyIncludesSortedQuery(t *testing.T) {
	t.Parallel()

	tr := always(200, `{}`, "ETag", `"e"`)
	cache := NewMemoryCache()
	c := newTestClient(t, tr, func(o *Options) { o.Cache = cache })
	require.NoError(t, ignore(c.MyCourses(context.Background(), "c1", 20)))

	_, _, ok := cache.Get("GET /me/courses?cursor=c1&limit=20")
	assert.True(t, ok)
	assert.Equal(t, "GET /x?a=1&b=2", cacheKey("GET", "/x", url.Values{"b": {"2"}, "a": {"1"}}))
}

func TestNoETagNoCache(t *testing.T) {
	t.Parallel()

	cache := NewMemoryCache()
	c := newTestClient(t, always(200, `{}`), func(o *Options) { o.Cache = cache })
	require.NoError(t, ignore(c.Me(context.Background())))
	_, _, ok := cache.Get("GET /me?")
	assert.False(t, ok)
}

func TestETag304WithoutCacheIsAnError(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, always(304, ""), func(o *Options) { o.Cache = NewMemoryCache() })
	err := ignore(c.Me(context.Background()))
	assert.ErrorIs(t, err, ErrServer)
}

type changes struct {
	mu  sync.Mutex
	log []bool
}

func (c *changes) on(v bool) {
	c.mu.Lock()
	c.log = append(c.log, v)
	c.mu.Unlock()
}

func (c *changes) get() []bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]bool(nil), c.log...)
}

func TestOfflineServesCache(t *testing.T) {
	t.Parallel()

	cfg := string(mockFile(t, "app/config.json"))
	down := false
	var mu sync.Mutex
	tr := &scriptTransport{fn: func(int, *Request) (*Response, error) {
		mu.Lock()
		defer mu.Unlock()
		if down {
			return nil, &url.Error{Op: "Get", URL: "https://x.test", Err: errors.New("offline")}
		}

		return &Response{Status: 200, Header: http.Header{"Etag": {`"v1"`}}, Body: []byte(cfg)}, nil
	}}
	var ch changes
	c := newTestClient(t, tr, func(o *Options) { o.Cache = NewMemoryCache(); o.OnNetwork = ch.on })
	ctx := context.Background()

	online, err := c.Config(ctx)
	require.NoError(t, err)
	assert.Empty(t, ch.get(), "online at start is not a change")

	mu.Lock()
	down = true
	mu.Unlock()
	for range 3 {
		got, err := c.Config(ctx)
		require.NoError(t, err)
		assert.Equal(t, online, got)
	}
	assert.Equal(t, []bool{false}, ch.get(), "one notification for three failures")

	// Not cached: offline error, no extra notification.
	err = ignore(c.Me(ctx))
	assert.ErrorIs(t, err, ErrOffline)
	assert.Equal(t, []bool{false}, ch.get())

	mu.Lock()
	down = false
	mu.Unlock()
	_, err = c.Config(ctx)
	require.NoError(t, err)
	assert.Equal(t, []bool{false, true}, ch.get())
}

func TestOfflineWithoutCacheIsOfflineError(t *testing.T) {
	t.Parallel()

	tr := &scriptTransport{fn: func(int, *Request) (*Response, error) { return nil, &net_err{} }}
	c := newTestClient(t, tr, func(o *Options) { o.Cache = NewMemoryCache() })
	assert.ErrorIs(t, ignore(c.Config(context.Background())), ErrOffline)
}

type net_err struct{}

func (*net_err) Error() string   { return "net down" }
func (*net_err) Timeout() bool   { return false }
func (*net_err) Temporary() bool { return false }

// TestNotCachedRoutes: stream and audio carry signed URLs and never touch
// the cache, nor send If-None-Match; writes neither.
func TestNotCachedRoutes(t *testing.T) {
	t.Parallel()

	for _, route := range Routes() {
		if route.Cacheable {
			continue
		}
		t.Run(route.ID, func(t *testing.T) {
			t.Parallel()

			tr := always(200, `{}`, "ETag", `"e"`)
			cache := NewMemoryCache()
			c := newTestClient(t, tr, func(o *Options) { o.Cache = cache })
			require.NoError(t, invokers[route.ID](context.Background(), c))
			assert.Empty(t, cache.m)
			assert.Empty(t, tr.seen()[0].Header.Get("If-None-Match"))
		})
	}

	// And offline: no stale stream URL is served.
	tr := &scriptTransport{fn: func(int, *Request) (*Response, error) { return nil, &net_err{} }}
	c := newTestClient(t, tr, func(o *Options) { o.Cache = NewMemoryCache() })
	assert.ErrorIs(t, ignore(c.Stream(context.Background(), "l_1")), ErrOffline)
}

func TestCacheableRoutesAreTheGETs(t *testing.T) {
	t.Parallel()

	for _, r := range Routes() {
		want := r.Method == "GET" && r.ID != "lessons.stream" && r.ID != "lessons.audio"
		assert.Equal(t, want, r.Cacheable, r.ID)
	}
}

func TestMemoryCacheCopies(t *testing.T) {
	t.Parallel()

	c := NewMemoryCache()
	b := []byte("abc")
	require.NoError(t, c.Put("k", "e", b))
	b[0] = 'X'
	_, got, ok := c.Get("k")
	require.True(t, ok)
	assert.Equal(t, "abc", string(got))
	got[0] = 'Y'
	_, again, _ := c.Get("k")
	assert.Equal(t, "abc", string(again))
	_, _, ok = c.Get("none")
	assert.False(t, ok)
}
