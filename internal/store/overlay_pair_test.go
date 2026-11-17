package store_test

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/store"
)

var pairT0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

// pairRig is a TV client and a phone client on one Overlay with the relay.
type pairRig struct {
	overlay *store.Overlay
	tv      *api.Client
	phone   *api.Client
	fake    *clock.Fake
}

func newPairRig(t *testing.T, b store.Backend, scenarios ...[2]string) *pairRig {
	t.Helper()
	r := newRig(t, b)
	for _, s := range scenarios {
		require.NoError(t, r.mock.SetScenario(s[0], s[1]))
	}
	fake := clock.NewFake(pairT0)
	confirmed, err := os.ReadFile(mockDir + "/tv/pair-poll.confirmed.json")
	require.NoError(t, err)
	require.NoError(t, r.overlay.EnablePairRelay(store.PairRelayOptions{Confirmed: confirmed, Now: fake.Now}))
	tv, err := api.New(api.Options{Transport: r.overlay, Version: "1.0.0", Platform: "tvos", InstallID: "i-tv"})
	require.NoError(t, err)

	return &pairRig{overlay: r.overlay, tv: tv, phone: r.client, fake: fake}
}

var tvDevice = model.DeviceInfo{Platform: "tvos", Name: "Apple TV", AppVersion: "1.0.0", InstallID: "i-tv"}

func TestPairRelayStartPollConfirm(t *testing.T) {
	t.Parallel()

	r := newPairRig(t, store.NewMemory())
	ctx := t.Context()
	start, err := r.tv.StartPairing(ctx, tvDevice)
	require.NoError(t, err)
	assert.Len(t, start.Code, pairing.CodeLen)
	assert.NotEqual(t, "7K4M2P", start.Code, "a random code, not the file's")
	assert.Equal(t, start.Code[:3]+"-"+start.Code[3:], start.DisplayCode)
	assert.Contains(t, start.QRURL, start.Code)
	assert.Equal(t, 300, start.ExpiresIn)
	assert.True(t, strings.HasPrefix(start.PollSecret, "ps_"))
	assert.Equal(t, 3, start.PollIntervalSec)
	_, err = pairing.NormalizeCode(start.Code)
	require.NoError(t, err)

	r.fake.Advance(30 * time.Second)
	p, err := r.tv.PollPairing(ctx, start.Code, start.PollSecret)
	require.NoError(t, err)
	assert.Equal(t, model.PairPending, p.Status)
	assert.Equal(t, 270, p.ExpiresIn)

	res, err := pairing.NewPhone(r.phone).Confirm(ctx, strings.ToLower(start.DisplayCode))
	require.NoError(t, err)
	assert.Equal(t, "d_201", res.Device.ID)

	p, err = r.tv.PollPairing(ctx, start.Code, start.PollSecret)
	require.NoError(t, err)
	assert.Equal(t, model.PairConfirmed, p.Status)
	s := p.Session()
	assert.NotEmpty(t, s.AccessToken)
	assert.NotEmpty(t, s.RefreshToken)
	assert.Equal(t, "u_412", s.User.ID)
	assert.Equal(t, "d_201", s.Device.ID)

	m, err := auth.New(auth.Options{Vault: auth.NewMemoryVault(), TV: true})
	require.NoError(t, err)
	require.NoError(t, m.Adopt(s))

	_, err = r.tv.PollPairing(ctx, start.Code, start.PollSecret)
	require.Error(t, err, "the session goes out once")
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusGone, ae.Status)
}

func TestPairRelayErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		act    func(t *testing.T, r *pairRig, s model.PairStart) error
		status int
		code   string
	}{
		{"expired poll", func(t *testing.T, r *pairRig, s model.PairStart) error {
			r.fake.Advance(300 * time.Second)
			_, err := r.tv.PollPairing(t.Context(), s.Code, s.PollSecret)

			return err
		}, 410, "pair_code_expired"},
		{"expired confirm", func(t *testing.T, r *pairRig, s model.PairStart) error {
			r.fake.Advance(301 * time.Second)
			_, err := pairing.NewPhone(r.phone).Confirm(t.Context(), s.Code)

			return err
		}, 404, "pair_code_invalid"},
		{"wrong code confirm", func(t *testing.T, r *pairRig, s model.PairStart) error {
			_, err := pairing.NewPhone(r.phone).Confirm(t.Context(), "ZZZZZZ")

			return err
		}, 404, "pair_code_invalid"},
		{"wrong code poll", func(t *testing.T, r *pairRig, s model.PairStart) error {
			_, err := r.tv.PollPairing(t.Context(), "ZZZZZZ", s.PollSecret)

			return err
		}, 410, "pair_code_expired"},
		{"wrong secret poll", func(t *testing.T, r *pairRig, s model.PairStart) error {
			_, err := r.tv.PollPairing(t.Context(), s.Code, "ps_other")

			return err
		}, 410, "pair_code_expired"},
		{"confirm twice", func(t *testing.T, r *pairRig, s model.PairStart) error {
			ph := pairing.NewPhone(r.phone)
			if _, err := ph.Confirm(t.Context(), s.Code); err != nil {
				return err
			}
			_, err := ph.Confirm(t.Context(), s.Code)

			return err
		}, 404, "pair_code_invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r := newPairRig(t, store.NewMemory())
			s, err := r.tv.StartPairing(t.Context(), tvDevice)
			require.NoError(t, err)
			err = tt.act(t, r, s)
			var ae *api.Error
			require.ErrorAs(t, err, &ae)
			assert.Equal(t, tt.status, ae.Status)
			assert.Equal(t, tt.code, ae.Code)
			for _, c := range ae.Message {
				assert.Less(t, c, rune(0x80), "the built-in bodies are ASCII")
			}
		})
	}
}

func TestPairRelayKeepsMockScenarios(t *testing.T) {
	t.Parallel()

	r := newPairRig(t, store.NewMemory(),
		[2]string{"tv.pair.start", "rate_limited"}, [2]string{"tv.pair.confirm", "device_limit"})
	_, err := r.tv.StartPairing(t.Context(), tvDevice)
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 429, ae.Status)

	require.NoError(t, r.overlay.EnablePairRelay(store.PairRelayOptions{Confirmed: []byte(`{"status":"confirmed"}`)}))
	_, err = pairing.NewPhone(r.phone).Confirm(t.Context(), "7K4M2P")
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 409, ae.Status, "the mock refuses first")
}

func TestPairRelayRejectsBadTemplate(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	require.Error(t, r.overlay.EnablePairRelay(store.PairRelayOptions{Confirmed: []byte(`{"status":"pending"}`)}))
	require.Error(t, r.overlay.EnablePairRelay(store.PairRelayOptions{}))
}

func TestPairRelayRestartKeepsNothingPending(t *testing.T) {
	t.Parallel()

	b := store.NewMemory()
	r := newPairRig(t, b)
	s, err := r.tv.StartPairing(t.Context(), tvDevice)
	require.NoError(t, err)

	r2 := newPairRig(t, b)
	_, err = pairing.NewPhone(r2.phone).Confirm(t.Context(), s.Code)
	var ae *api.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, 404, ae.Status)
}

func TestPairRelayOffByDefault(t *testing.T) {
	t.Parallel()

	r := newRig(t, store.NewMemory())
	tv, err := api.New(api.Options{Transport: r.overlay, Version: "1.0.0", Platform: "tvos"})
	require.NoError(t, err)
	s, err := tv.StartPairing(t.Context(), tvDevice)
	require.NoError(t, err)
	assert.Equal(t, "7K4M2P", s.Code, "the mock's file")
}

func TestPairRelayLogoutRemovesTV(t *testing.T) {
	t.Parallel()

	r := newPairRig(t, store.NewMemory())
	s, err := r.tv.StartPairing(t.Context(), tvDevice)
	require.NoError(t, err)
	_, err = pairing.NewPhone(r.phone).Confirm(t.Context(), s.Code)
	require.NoError(t, err)
	me, err := r.phone.Me(t.Context())
	require.NoError(t, err)
	before := me.Devices.Active

	_, err = r.overlay.Do(t.Context(), &api.Request{RouteID: "tv.logout", Method: http.MethodPost, Path: "/tv/logout",
		Header: http.Header{"Authorization": {"Bearer x"}}})
	require.NoError(t, err)
	me, err = r.phone.Me(t.Context())
	require.NoError(t, err)
	assert.Equal(t, before-1, me.Devices.Active)
}

// TestPairRelayWithPairingTV runs pairing.TV and pairing.Phone on one
// overlay: the TV polls every 3 s of the fake clock, the phone confirms
// between two polls.
func TestPairRelayWithPairingTV(t *testing.T) {
	t.Parallel()

	r := newPairRig(t, store.NewMemory())
	ui := make(chan func(), 64)
	var (
		mu        sync.Mutex
		states    []pairing.State
		confirmed []model.Session
	)
	tv := pairing.NewTV(pairing.TVOptions{
		Client: r.tv, Clock: r.fake, Dispatch: func(fn func()) { ui <- fn }, Device: tvDevice,
		OnState:     func(s pairing.State) { mu.Lock(); states = append(states, s); mu.Unlock() },
		OnConfirmed: func(s model.Session) { mu.Lock(); confirmed = append(confirmed, s); mu.Unlock() },
	})
	t.Cleanup(func() { tv.Stop(); tv.Wait() })
	pump := func() { (<-ui)() }

	tv.Start(context.Background())
	pump()
	require.Len(t, states, 1)
	r.fake.Advance(3 * time.Second)
	pump() // pending
	require.Empty(t, confirmed)

	_, err := pairing.NewPhone(r.phone).Confirm(t.Context(), states[0].DisplayCode)
	require.NoError(t, err)
	r.fake.Advance(3 * time.Second)
	pump()
	require.Len(t, confirmed, 1)
	assert.Equal(t, "u_412", confirmed[0].User.ID)
	assert.NotEmpty(t, confirmed[0].AccessToken)
}
