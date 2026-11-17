package api

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// waitLog is an instant Wait that records the delays.
type waitLog struct {
	mu sync.Mutex
	ds []time.Duration
}

func (w *waitLog) wait(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.ds = append(w.ds, d)
	w.mu.Unlock()

	return ctx.Err()
}

func (w *waitLog) got() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]time.Duration(nil), w.ds...)
}

func newLive(t *testing.T, srv *httptest.Server, w *waitLog) *Live {
	t.Helper()

	l, err := NewLive(LiveOptions{Base: srv.URL + "/api/v1/", Client: srv.Client(), Wait: w.wait})
	require.NoError(t, err)

	return l
}

func TestNewLive(t *testing.T) {
	t.Parallel()

	for _, base := range []string{"", "ftp://x.test", "https://", "::bad", "dancefitvn.com/mobile/api"} {
		_, err := NewLive(LiveOptions{Base: base})
		assert.ErrorIs(t, err, core.ErrInvalid, base)
	}
	l, err := NewLive(LiveOptions{Base: "https://dancefitvn.com/mobile/api/v1"})
	require.NoError(t, err)
	assert.Equal(t, liveTimeout, l.client.Timeout)
	assert.NotNil(t, l.wait)
}

func TestLiveRoundTrip(t *testing.T) {
	t.Parallel()

	var got struct {
		method, uri, ctype, auth, ua, body string
	}
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got.method, got.uri, got.ctype, got.auth, got.body = r.Method, r.RequestURI, r.Header.Get("Content-Type"), r.Header.Get("Authorization"), string(b)
		w.Header().Set("ETag", `"e1"`)
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	l := newLive(t, srv, &waitLog{})
	resp, err := l.Do(context.Background(), &Request{
		RouteID: "me.update", Method: "PATCH", Path: "/courses/a%2Fb",
		Query:  map[string][]string{"limit": {"5"}, "cursor": {"x y"}},
		Header: http.Header{"Content-Type": {"application/json"}, "Authorization": {"Bearer t"}},
		Body:   []byte(`{"a":1}`),
	})
	require.NoError(t, err)

	assert.Equal(t, 200, resp.Status)
	assert.Equal(t, `"e1"`, resp.Header.Get("Etag"))
	assert.JSONEq(t, `{"ok":true}`, string(resp.Body))
	assert.Equal(t, "PATCH", got.method)
	assert.Equal(t, "/api/v1/courses/a%2Fb?cursor=x+y&limit=5", got.uri)
	assert.Equal(t, "application/json", got.ctype)
	assert.Equal(t, "Bearer t", got.auth)
	assert.JSONEq(t, `{"a":1}`, got.body)
}

func TestLiveStatuses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		status int
		body   string
		nilBd  bool
	}{
		{"204", 204, "", true},
		{"304", 304, "", true},
		{"401 json", 401, `{"error":{"code":"token_expired"}}`, false},
		{"500 html", 500, "<html>", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()

			resp, err := newLive(t, srv, &waitLog{}).Do(context.Background(), &Request{Method: "GET", Path: "/x"})
			require.NoError(t, err, "an HTTP error is a response")
			assert.Equal(t, tc.status, resp.Status)
			if tc.nilBd {
				assert.Nil(t, resp.Body)
			} else {
				assert.Equal(t, tc.body, string(resp.Body))
			}
		})
	}
}

// dropper closes the first n connections before answering.
func dropper(n int32, status int) (*httptest.Server, *atomic.Int32) {
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= n {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()

			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{}`))
	}))

	return srv, &hits
}

func TestLiveRetriesGETOnly(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		method   string
		drops    int32
		wantHits int32
		wantWait []time.Duration
		wantErr  bool
	}{
		{"GET", 2, 3, []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}, false},
		{"GET", 1, 2, []time.Duration{500 * time.Millisecond}, false},
		{"GET", 3, 3, []time.Duration{500 * time.Millisecond, 1500 * time.Millisecond}, true},
		{"POST", 1, 1, nil, true},
		{"PUT", 1, 1, nil, true},
		{"PATCH", 1, 1, nil, true},
		{"DELETE", 1, 1, nil, true},
	} {
		t.Run(tc.method+"/"+string(rune('0'+tc.drops)), func(t *testing.T) {
			t.Parallel()

			srv, hits := dropper(tc.drops, 200)
			defer srv.Close()
			w := &waitLog{}

			resp, err := newLive(t, srv, w).Do(context.Background(), &Request{Method: tc.method, Path: "/x", Body: nil})
			if tc.wantErr {
				require.Error(t, err)
				assert.True(t, isNetwork(err), err)
				assert.False(t, errors.Is(err, ErrOffline), "Live returns the raw error; the Client maps it")
			} else {
				require.NoError(t, err)
				assert.Equal(t, 200, resp.Status)
			}
			assert.Equal(t, tc.wantHits, hits.Load())
			assert.Equal(t, tc.wantWait, w.got())
		})
	}
}

func TestLiveHTTPErrorsAreNotRetried(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(503)
	}))
	defer srv.Close()
	w := &waitLog{}
	resp, err := newLive(t, srv, w).Do(context.Background(), &Request{Method: "GET", Path: "/x"})
	require.NoError(t, err)
	assert.Equal(t, 503, resp.Status)
	assert.Equal(t, int32(1), hits.Load())
	assert.Empty(t, w.got())
}

func TestLiveClientOverNetworkFailure(t *testing.T) {
	t.Parallel()

	srv, _ := dropper(100, 200)
	defer srv.Close()
	c := newTestClient(t, newLive(t, srv, &waitLog{}))
	err := ignore(c.Config(context.Background()))
	assert.ErrorIs(t, err, ErrOffline)
	assert.ErrorIs(t, err, core.ErrClosed)
}

// TestLiveTimeout: the context deadline ends a call the server holds; the
// server waits on a channel, nothing sleeps.
func TestLiveTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	w := &waitLog{}
	_, err := newLive(t, srv, w).Do(ctx, &Request{Method: "GET", Path: "/x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Empty(t, w.got(), "no retry after the deadline")
}

func TestLiveClientTimeout(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	cl := srv.Client()
	cl.Timeout = 30 * time.Millisecond
	l, err := NewLive(LiveOptions{Base: srv.URL, Client: cl, Wait: (&waitLog{}).wait})
	require.NoError(t, err)
	_, err = l.Do(context.Background(), &Request{Method: "POST", Path: "/x"})
	require.Error(t, err)
	assert.True(t, isNetwork(err))
	assert.Equal(t, 30*time.Millisecond, cl.Timeout, "the caller's client is not changed")
}

func TestLiveCancel(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := newLive(t, srv, &waitLog{}).Do(ctx, &Request{Method: "GET", Path: "/x"})
		done <- err
	}()
	<-started
	cancel()
	err := <-done
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, isNetwork(err))
}

func TestLiveCancelDuringWait(t *testing.T) {
	t.Parallel()

	srv, hits := dropper(100, 200)
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	l, err := NewLive(LiveOptions{Base: srv.URL, Client: srv.Client(), Wait: func(ctx context.Context, d time.Duration) error {
		cancel()

		return waitTimer(ctx, time.Hour)
	}})
	require.NoError(t, err)
	_, err = l.Do(ctx, &Request{Method: "GET", Path: "/x"})
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(1), hits.Load())
}

func TestLiveBodyCap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		size int
		fail bool
	}{
		{"at cap", liveMaxBody, false},
		{"over cap", liveMaxBody + 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(strings.Repeat("a", tc.size)))
			}))
			defer srv.Close()
			resp, err := newLive(t, srv, &waitLog{}).Do(context.Background(), &Request{Method: "GET", Path: "/x"})
			if tc.fail {
				assert.ErrorIs(t, err, ErrServer)

				return
			}
			require.NoError(t, err)
			assert.Len(t, resp.Body, tc.size)
		})
	}
}

func TestLiveRedirects(t *testing.T) {
	t.Parallel()

	var otherHits atomic.Int32
	other := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherHits.Add(1)
	}))
	defer other.Close()

	srv := httptest.NewTLSServer(nil)
	defer srv.Close()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/same", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/api/v1/target", 302) })
	mux.HandleFunc("/api/v1/target", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"t":1}`)) })
	mux.HandleFunc("/api/v1/away", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL+"/steal", 302) })
	mux.HandleFunc("/api/v1/loop", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/api/v1/loop", 302) })
	srv.Config.Handler = mux

	l := newLive(t, srv, &waitLog{})
	ctx := context.Background()

	resp, err := l.Do(ctx, &Request{Method: "GET", Path: "/same", Header: http.Header{"Authorization": {"Bearer t"}}})
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)

	resp, err = l.Do(ctx, &Request{Method: "GET", Path: "/away", Header: http.Header{"Authorization": {"Bearer t"}}})
	require.NoError(t, err)
	assert.Equal(t, 302, resp.Status, "the 3xx answer itself, other host not contacted")
	assert.Zero(t, otherHits.Load())

	resp, err = l.Do(ctx, &Request{Method: "GET", Path: "/loop"})
	require.NoError(t, err)
	assert.Equal(t, 302, resp.Status)
}

func TestLiveRefusedConnection(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())

	w := &waitLog{}
	l, err := NewLive(LiveOptions{Base: "http://" + addr, Wait: w.wait})
	require.NoError(t, err)
	_, err = l.Do(context.Background(), &Request{Method: "GET", Path: "/x"})
	require.Error(t, err)
	assert.True(t, isNetwork(err))
	assert.Len(t, w.got(), 2)
}

// TestLiveEndToEnd: Client over Live over httptest with ETag and 304.
func TestLiveEndToEnd(t *testing.T) {
	t.Parallel()

	cfg := mockFile(t, "app/config.json")
	var conds []string
	var mu sync.Mutex
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		conds = append(conds, r.Header.Get("If-None-Match"))
		mu.Unlock()
		assert.Equal(t, "/api/v1/app/config", r.URL.Path)
		assert.Equal(t, "vi", r.Header.Get("Accept-Language"))
		assert.Equal(t, "ios", r.Header.Get("X-Platform"))
		w.Header().Set("ETag", `"cfg"`)
		if r.Header.Get("If-None-Match") == `"cfg"` {
			w.WriteHeader(304)

			return
		}
		_, _ = w.Write(cfg)
	}))
	defer srv.Close()

	c := newTestClient(t, newLive(t, srv, &waitLog{}), func(o *Options) { o.Cache = NewMemoryCache() })
	a, err := c.Config(context.Background())
	require.NoError(t, err)
	b, err := c.Config(context.Background())
	require.NoError(t, err)
	assert.Equal(t, a, b)
	assert.NotEmpty(t, a.MinVersion)
	assert.Equal(t, []string{"", `"cfg"`}, conds)
}

func TestDefaultBase(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "https://dancefitvn.com/mobile/api/v1", DefaultBase)
	l, err := NewLive(LiveOptions{Base: DefaultBase})
	require.NoError(t, err)
	assert.NotNil(t, l)
}
