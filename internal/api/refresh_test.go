package api

import (
	"context"
	"errors"
	"net/url"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/model"
)

const expiredBody = `{"error":{"code":"token_expired","message":"x"}}`

// authTransport answers 401 token_expired to any token but good.
func authTransport(good string) *scriptTransport {
	return &scriptTransport{fn: func(_ int, r *Request) (*Response, error) {
		if r.Header.Get("Authorization") == "Bearer "+good {
			return &Response{Status: 200, Body: []byte(`{}`)}, nil
		}

		return &Response{Status: 401, Body: []byte(expiredBody)}, nil
	}}
}

func TestRefreshOnceThenRetry(t *testing.T) {
	t.Parallel()

	tr := authTransport("new")
	ts := &tokenSource{token: "old", next: "new"}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	require.NoError(t, ignore(c.Me(context.Background())))
	assert.Equal(t, 1, ts.count())
	reqs := tr.seen()
	require.Len(t, reqs, 2)
	assert.Equal(t, "Bearer old", reqs[0].Header.Get("Authorization"))
	assert.Equal(t, "Bearer new", reqs[1].Header.Get("Authorization"))
}

func TestNoRefreshOnLoginFailure(t *testing.T) {
	t.Parallel()

	body := string(mockFile(t, "errors/invalid_credentials.json"))
	tr := always(401, body)
	ts := &tokenSource{token: "old", next: "new"}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	err := ignore(c.Login(context.Background(), "a@example.test", "pw", modelDevice()))
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "invalid_credentials", e.Code)
	assert.NotErrorIs(t, err, ErrSignedOut)
	assert.Zero(t, ts.count())
	assert.Len(t, tr.seen(), 1)
}

func TestNoRefreshForOtherCodes(t *testing.T) {
	t.Parallel()

	for _, code := range []string{"refresh_invalid", "invalid_credentials", "not_found"} {
		tr := always(401, `{"error":{"code":"`+code+`","message":"x"}}`)
		ts := &tokenSource{token: "old", next: "new"}
		c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })
		err := ignore(c.Me(context.Background()))
		assert.ErrorIs(t, err, ErrUnauthorized, code)
		assert.Zero(t, ts.count(), code)
	}
}

func TestRefreshFailureSignsOut(t *testing.T) {
	t.Parallel()

	tr := authTransport("never")
	ts := &tokenSource{token: "old", err: ErrSignedOut}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	err := ignore(c.Me(context.Background()))
	assert.ErrorIs(t, err, ErrSignedOut)
	assert.Len(t, tr.seen(), 1, "no resend after a failed refresh")
}

func TestStill401AfterRefreshSignsOut(t *testing.T) {
	t.Parallel()

	tr := authTransport("never")
	ts := &tokenSource{token: "old", next: "new"}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	err := ignore(c.Me(context.Background()))
	assert.ErrorIs(t, err, ErrSignedOut)
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "token_expired", e.Code)
	assert.Len(t, tr.seen(), 2, "one resend only")
	assert.Equal(t, 1, ts.count())
}

func TestRefreshNetworkErrorKeepsSession(t *testing.T) {
	t.Parallel()

	tr := authTransport("never")
	ts := &tokenSource{token: "old", err: &url.Error{Op: "Post", URL: "https://x.test", Err: errors.New("down")}}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	err := ignore(c.Me(context.Background()))
	assert.ErrorIs(t, err, ErrOffline)
	assert.NotErrorIs(t, err, ErrSignedOut)
	assert.Equal(t, "old", ts.Token(), "the session is kept")
}

func TestRefreshServerErrorKeepsSession(t *testing.T) {
	t.Parallel()

	tr := authTransport("never")
	ts := &tokenSource{token: "old", err: &Error{Status: 500, Code: "server_error", sentinel: ErrServer}}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	err := ignore(c.Me(context.Background()))
	assert.ErrorIs(t, err, ErrServer)
	assert.NotErrorIs(t, err, ErrSignedOut)
}

func TestAnonymousClientDoesNotRefresh(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, authTransport("x"))
	err := ignore(c.Me(context.Background()))
	var e *Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, "token_expired", e.Code)
}

// TestRefreshSingleFlight: 10 calls fail with the same token while the
// refresh is held; there is exactly one Refresh and 10 successes.
func TestRefreshSingleFlight(t *testing.T) {
	t.Parallel()

	const n = 10
	gate := make(chan struct{})
	ts := &tokenSource{token: "old", next: "new", gate: gate}

	var mu sync.Mutex
	failed := 0
	allFailed := make(chan struct{})
	tr := &scriptTransport{fn: func(_ int, r *Request) (*Response, error) {
		if r.Header.Get("Authorization") == "Bearer new" {
			return &Response{Status: 200, Body: []byte(`{}`)}, nil
		}
		mu.Lock()
		failed++
		last := failed == n
		mu.Unlock()
		if last {
			close(allFailed)
		}

		return &Response{Status: 401, Body: []byte(expiredBody)}, nil
	}}
	c := newTestClient(t, tr, func(o *Options) { o.Tokens = ts })

	errs := make(chan error, n)
	for range n {
		go func() { errs <- ignore(c.Me(context.Background())) }()
	}
	<-allFailed
	close(gate)
	for range n {
		assert.NoError(t, <-errs)
	}
	assert.Equal(t, 1, ts.count())
}

// TestRefreshLateCallerUsesNewToken: a call that failed with the old token
// after the refresh finished takes the current token without refreshing.
func TestRefreshLateCallerUsesNewToken(t *testing.T) {
	t.Parallel()

	ts := &tokenSource{token: "new", next: "newer"}
	c := newTestClient(t, always(200, `{}`), func(o *Options) { o.Tokens = ts })
	tok, err := c.refresh(context.Background(), "old")
	require.NoError(t, err)
	assert.Equal(t, "new", tok)
	assert.Zero(t, ts.count())
}

func TestRefreshFollowerCanceled(t *testing.T) {
	t.Parallel()

	gate := make(chan struct{})
	ts := &tokenSource{token: "old", next: "new", gate: gate}
	c := newTestClient(t, always(200, `{}`), func(o *Options) { o.Tokens = ts })

	leaderDone := make(chan struct{})
	go func() {
		defer close(leaderDone)
		_, _ = c.refresh(context.Background(), "old")
	}()
	// Wait until the leader is inside Refresh.
	for ts.count() == 0 {
		runtimeGosched()
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.refresh(ctx, "old")
	assert.ErrorIs(t, err, context.Canceled)
	close(gate)
	<-leaderDone
}

func modelDevice() model.DeviceInfo { return model.DeviceInfo{} }

func runtimeGosched() { runtime.Gosched() }
