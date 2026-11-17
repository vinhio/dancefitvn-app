package pairing_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
)

func offlineResp() (*api.Response, error, bool) {
	return nil, fmt.Errorf("%w: %w", api.ErrOffline, offlineErr), true
}

func TestStartShowsCode(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.pump()
	require.Len(t, r.states, 1)
	require.Equal(t, "7K4M2P", r.states[0].Code)
	require.Equal(t, "7K4-M2P", r.states[0].DisplayCode)
	require.Equal(t, "https://dancefitvn.com/tv?c=7K4M2P", r.states[0].QRURL)
	require.Equal(t, t0.Add(300*time.Second), r.states[0].ExpiresAt, "from expires_in on the injected clock, not the server's time")
	st, ok := r.tv.Current()
	require.True(t, ok)
	require.Equal(t, r.states[0], st)
	require.Equal(t, 1, r.fake.Pending(), "the first poll is armed")
}

func TestPollCadence3s(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.pump()

	r.fake.Advance(2999 * time.Millisecond)
	require.Zero(t, r.spy.count("tv.pair.poll"))
	r.fake.Advance(time.Millisecond)
	for i, at := range []time.Duration{3, 6, 9} {
		require.Equal(t, t0.Add(at*time.Second), r.fake.Now(), "poll %d is at %d s", i+1, at)
		r.pump()
		require.Equal(t, i+1, r.spy.count("tv.pair.poll"))
		r.fake.Advance(3 * time.Second)
	}
	r.pump() // the poll of 12 s
	require.Equal(t, 4, r.spy.count("tv.pair.poll"))
	require.Empty(t, r.confirmed)
	require.Len(t, r.states, 1, "pending keeps the same code")
}

func TestConfirmedStopsPolling(t *testing.T) {
	t.Parallel()
	r := newRig(t, [2]string{"tv.pair.poll", "confirmed"})
	r.tv.Start(context.Background())
	r.pump()
	r.fake.Advance(3 * time.Second)
	r.pump()

	require.Len(t, r.confirmed, 1)
	s := r.confirmed[0]
	require.NotEmpty(t, s.AccessToken)
	require.NotEmpty(t, s.RefreshToken)
	require.NotEmpty(t, s.User.Email)
	require.NotEmpty(t, s.Device.ID)
	require.Zero(t, r.fake.Pending(), "timer cleared")
	r.fake.Advance(time.Minute)
	require.Equal(t, 1, r.spy.count("tv.pair.poll"))
	require.Zero(t, r.tv.Remaining())
	_, ok := r.tv.Current()
	require.False(t, ok)
}

func TestPendingThenConfirmed(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.pump()
	for range 2 {
		r.fake.Advance(3 * time.Second)
		r.pump()
	}
	require.Empty(t, r.confirmed)
	require.NoError(t, r.mock.SetScenario("tv.pair.poll", "confirmed"))
	r.fake.Advance(3 * time.Second)
	r.pump()
	require.Len(t, r.confirmed, 1)
	require.Equal(t, 3, r.spy.count("tv.pair.poll"))
}

func TestExpiredRestarts(t *testing.T) {
	t.Parallel()
	r := newRig(t, [2]string{"tv.pair.poll", "expired"})
	r.tv.Start(context.Background())
	r.pump()
	r.fake.Advance(3 * time.Second)
	r.pump() // 410 pair_code_expired: start again at once
	r.pump() // the answer of the new start
	require.Equal(t, 2, r.spy.count("tv.pair.start"), "a start was sent right away")
	require.Len(t, r.states, 2, "new code shown")
	require.Empty(t, r.errs, "an expired code is not an error to show")
	require.Equal(t, t0.Add(3*time.Second+300*time.Second), r.states[1].ExpiresAt)
	require.Equal(t, 1, r.fake.Pending(), "polling continues with the new code")
}

func TestLocalExpiryRestarts(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.pump()
	require.Equal(t, 300*time.Second, r.tv.Remaining())
	for range 99 { // polls at 3 … 297 s
		r.fake.Advance(3 * time.Second)
		r.pump()
	}
	require.Equal(t, 99, r.spy.count("tv.pair.poll"))
	require.Equal(t, 3*time.Second, r.tv.Remaining())
	require.Equal(t, 1, r.spy.count("tv.pair.start"))
	r.fake.Advance(3 * time.Second) // 300 s: the code is gone
	require.Zero(t, r.tv.Remaining())
	r.pump()
	require.Equal(t, 99, r.spy.count("tv.pair.poll"), "no poll for an expired code")
	require.Equal(t, 2, r.spy.count("tv.pair.start"))
	require.Len(t, r.states, 2)
	require.Equal(t, 300*time.Second, r.tv.Remaining())
}

func TestStopCancelsInFlight(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	entered := make(chan struct{})
	r.spy.hook = func(ctx context.Context, _ int, req *api.Request) (*api.Response, error, bool) {
		if req.RouteID != "tv.pair.poll" {
			return nil, nil, false
		}
		close(entered)
		<-ctx.Done()

		return nil, ctx.Err(), true
	}
	r.tv.Start(context.Background())
	r.pump()
	r.fake.Advance(3 * time.Second)
	<-entered
	r.tv.Stop() // leaving TV02
	r.tv.Wait()
	require.Zero(t, r.fake.Pending())
	r.pump() // the answer of the poll comes back and is dropped
	require.Empty(t, r.errs)
	require.Empty(t, r.confirmed)
	require.Zero(t, r.tv.Remaining())
	r.fake.Advance(time.Hour)
	require.Equal(t, 1, r.spy.count("tv.pair.poll"))
	require.Equal(t, 1, r.spy.count("tv.pair.start"))
}

func TestStopBeforeStartAnswer(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.tv.Stop()
	r.tv.Wait()
	r.pump()
	require.Empty(t, r.states, "a stale code is never shown")
	require.Zero(t, r.fake.Pending())
}

func TestStartTwiceReplaces(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tv.Start(context.Background())
	r.pump()
	r.tv.Start(context.Background())
	r.pump()
	require.Len(t, r.states, 2)
	require.Equal(t, 1, r.fake.Pending(), "one timer, not two")
}

func TestRateLimitedWaits(t *testing.T) {
	t.Parallel()
	r := newRig(t, [2]string{"tv.pair.start", "rate_limited"})
	r.tv.Start(context.Background())
	r.pump()
	require.Len(t, r.errs, 1)
	require.ErrorIs(t, r.errs[0], api.ErrRateLimited)
	require.Empty(t, r.states)
	require.NoError(t, r.mock.SetScenario("tv.pair.start", "default"))

	r.fake.Advance(59 * time.Second) // retry_after is 60
	require.Equal(t, 1, r.spy.count("tv.pair.start"))
	r.fake.Advance(time.Second)
	r.pump()
	require.Equal(t, 2, r.spy.count("tv.pair.start"))
	require.Len(t, r.states, 1)
	require.Equal(t, t0.Add(60*time.Second+300*time.Second), r.states[0].ExpiresAt)
}

func TestNetworkErrorsRetryNextTick(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		route string
	}{
		{"start", "tv.pair.start"},
		{"poll", "tv.pair.poll"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t)
			r.spy.hook = func(_ context.Context, n int, req *api.Request) (*api.Response, error, bool) {
				if req.RouteID == tc.route && n == 1 {
					return offlineResp()
				}

				return nil, nil, false
			}
			r.tv.Start(context.Background())
			r.pump()
			if tc.route == "tv.pair.poll" {
				require.Len(t, r.states, 1)
				r.fake.Advance(3 * time.Second)
				r.pump()
			}
			require.Len(t, r.errs, 1)
			require.ErrorIs(t, r.errs[0], api.ErrOffline)
			r.fake.Advance(2999 * time.Millisecond)
			require.Equal(t, 1, r.spy.count(tc.route))
			r.fake.Advance(time.Millisecond)
			r.pump()
			require.Equal(t, 2, r.spy.count(tc.route), "retried 3 s later")
			require.Len(t, r.states, 1, "same code after a poll error; first code after a start error")
		})
	}
}

func TestRemaining(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	require.Zero(t, r.tv.Remaining(), "no code yet")
	r.tv.Start(context.Background())
	require.Zero(t, r.tv.Remaining(), "still asking")
	r.pump()
	tests := []struct {
		at   time.Duration
		want time.Duration
	}{
		{0, 300 * time.Second},
		{1 * time.Second, 299 * time.Second},
		{90 * time.Second, 210 * time.Second},
	}
	for _, tc := range tests {
		r.fake.Advance(tc.at - (r.fake.Now().Sub(t0)))
		// Advancing runs due polls; answer them.
		for len(r.ui) > 0 {
			r.pump()
		}
		require.Equal(t, tc.want, r.tv.Remaining(), "at %v", tc.at)
	}
}

func TestPollStatusUnknownKeepsPolling(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.spy.hook = func(_ context.Context, _ int, req *api.Request) (*api.Response, error, bool) {
		if req.RouteID == "tv.pair.poll" {
			return &api.Response{Status: 200, Body: []byte(`{"status":"weird"}`)}, nil, true
		}

		return nil, nil, false
	}
	r.tv.Start(context.Background())
	r.pump()
	r.fake.Advance(3 * time.Second)
	r.pump()
	require.Empty(t, r.confirmed)
	require.Equal(t, 1, r.fake.Pending())
}
