package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
)

const testMock = "../../mock/dancefitvn-mock-api"

type tokenSource struct{}

func (tokenSource) Token() string                                   { return "tok" }
func (tokenSource) Refresh(context.Context, string) (string, error) { return "", api.ErrSignedOut }

// rig is a mockserve behind httptest and two Live clients, a TV and a phone.
type rig struct {
	srv   *httptest.Server
	tv    *api.Client
	phone *api.Client
	base  string
}

func newRig(t *testing.T, overlay string) *rig {
	t.Helper()
	s, err := newServer(options{Mock: testMock, Overlay: overlay,
		wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}, func(string, ...any) {})
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.mock.Close() })
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	s.public.Store(srv.URL)
	live, err := api.NewLive(api.LiveOptions{Base: srv.URL + apiPrefix, Client: &http.Client{Transport: &http.Transport{DisableKeepAlives: true}}})
	require.NoError(t, err)
	tv, err := api.New(api.Options{Transport: live, Version: "1.0.0", Platform: "tvos", InstallID: "i-tv"})
	require.NoError(t, err)
	phone, err := api.New(api.Options{Transport: live, Version: "1.0.0", Platform: "ios", InstallID: "i-ph", Tokens: tokenSource{}})
	require.NoError(t, err)

	return &rig{srv: srv, tv: tv, phone: phone, base: srv.URL}
}

func TestPairOverHTTP(t *testing.T) {
	t.Parallel()

	r := newRig(t, "")
	ctx := t.Context()
	s, err := r.tv.StartPairing(ctx, model.DeviceInfo{Platform: "tvos", Name: "Apple TV", AppVersion: "1.0.0", InstallID: "i-tv"})
	require.NoError(t, err)
	p, err := r.tv.PollPairing(ctx, s.Code, s.PollSecret)
	require.NoError(t, err)
	assert.Equal(t, model.PairPending, p.Status)

	_, err = pairing.NewPhone(r.phone).Confirm(ctx, s.DisplayCode)
	require.NoError(t, err)
	p, err = r.tv.PollPairing(ctx, s.Code, s.PollSecret)
	require.NoError(t, err)
	assert.Equal(t, model.PairConfirmed, p.Status)
	assert.Equal(t, "u_412", p.Session().User.ID)

	_, err = pairing.NewPhone(r.phone).Confirm(ctx, "ZZZZZZ")
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusNotFound, ae.Status)
	assert.Equal(t, "pair_code_invalid", ae.Code)
	want, err := os.ReadFile(filepath.Join(testMock, "errors", "pair_code_invalid.json"))
	require.NoError(t, err)
	assert.Contains(t, string(want), ae.Message, "the message comes from the mock's error file")
}

func TestMediaServedAndRewritten(t *testing.T) {
	t.Parallel()

	r := newRig(t, "")
	home, err := r.phone.Home(t.Context())
	require.NoError(t, err)
	require.NotEmpty(t, home.MyCourses)
	cover := home.MyCourses[0].CoverURL
	require.True(t, strings.HasPrefix(cover, r.base+"/media/"), cover)
	resp, err := http.Get(cover) //nolint:noctx // test against httptest
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	n, _ := io.Copy(io.Discard, resp.Body)
	assert.Positive(t, n)
}

func TestOverlayFilePersists(t *testing.T) {
	t.Parallel()

	f := filepath.Join(t.TempDir(), "overlay.json")
	r := newRig(t, f)
	_, err := r.phone.UpdateMe(t.Context(), model.MeUpdateRequest{FirstName: "Vy", LastName: "Nguyễn", Email: "thaovy.hlv@gmail.com", Phone: "0905123457"})
	require.NoError(t, err)
	info, err := os.Stat(f)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())

	r2 := newRig(t, f)
	me, err := r2.phone.Me(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "Vy", me.User.FirstName)
}

// syncBuffer is a bytes.Buffer safe for a writer and a reader.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.String()
}

func TestListenLoopback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		addr string
		ok   bool
	}{
		{"127.0.0.1:0", true},
		{"localhost:0", true},
		{"[::1]:0", true},
		{"0.0.0.0:0", false},
		{":0", false},
		{"192.168.1.5:8787", false},
		{"example.com:80", false},
		{"nonsense", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			t.Parallel()

			ln, err := listenLoopback(tt.addr)
			if !tt.ok {
				require.Error(t, err)

				return
			}
			if err != nil && tt.addr == "[::1]:0" {
				t.Skip("no IPv6 loopback")
			}
			require.NoError(t, err)
			require.NoError(t, ln.Close())
		})
	}
}

func TestRunFlags(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		code int
		out  string
	}{
		{"help", []string{"-h"}, exitOK, "-addr"},
		{"bad flag", []string{"-nope"}, exitUsage, ""},
		{"stray arg", []string{"x"}, exitUsage, "usage"},
		{"public bind", []string{"-addr", "0.0.0.0:0"}, exitUsage, "loopback"},
		{"missing mock", []string{"-addr", "127.0.0.1:0", "-mock", "/nonexistent"}, exitUsage, "load mock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errb bytes.Buffer
			assert.Equal(t, tt.code, run(t.Context(), tt.args, &out, &errb))
			assert.Contains(t, out.String()+errb.String(), tt.out)
		})
	}
}

func TestRunServesUntilCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	var out, errb syncBuffer
	done := make(chan int, 1)
	go func() { done <- run(ctx, []string{"-addr", "127.0.0.1:0", "-mock", testMock}, &out, &errb) }()
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "API at") }, 5*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case code := <-done:
		assert.Equal(t, exitOK, code)
	case <-time.After(5 * time.Second):
		t.Fatal("run did not stop")
	}
}
