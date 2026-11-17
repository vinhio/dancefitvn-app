//go:build tvos

package app

import (
	"strings"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/pairing"
	tvenv "dancefitvn-app/internal/ui/tv/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTVLiveBuildStarts: the TV's services of the shipped live build (no
// mock: Mock nil, api.DefaultBase) make no mock and no overlay, and pair
// with a phone over HTTP: the code the TV shows is the one the phone
// confirms on the same server, the TV's next poll gets the session. It
// runs in every build, -tags live included.
func TestTVLiveBuildStarts(t *testing.T) {
	t.Parallel()

	srv := newLiveServer(t)
	r := newTVRigOf(t, nil, "")
	r.deps.Mock = nil
	r.deps.API = srv.shippedPlan(t)
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	assert.Nil(t, r.w.mock, "no mock in the process")
	assert.Nil(t, r.w.overlay, "no overlay")
	_, bare := r.w.tr.inner.(*api.Live)
	assert.True(t, bare, "the server is asked bare, not %T", r.w.tr.inner)
	assert.Contains(t, r.log.all(), "dancefitvn: API: live")

	var code, email string
	p := r.w.Pair(tvenv.PairHooks{
		OnState:    func(st pairing.State) { code = st.DisplayCode },
		OnSignedIn: func(e string) { email = e },
	})
	defer p.Stop()
	r.drive("a code", func() bool { return code != "" })

	live, err := api.NewLive(api.LiveOptions{Base: srv.url})
	require.NoError(t, err)
	phoneAuth, err := auth.New(auth.Options{Vault: auth.NewSecretVault(&memSecrets{vals: map[string][]byte{}}, "phone")})
	require.NoError(t, err)
	phone, err := api.New(api.Options{Transport: live, Version: appVersion, Platform: "ios", InstallID: "phone", Tokens: phoneAuth})
	require.NoError(t, err)
	phoneAuth.Bind(phone)
	_, err = phoneAuth.Login(t.Context(), "seed@example.invalid", "x")
	require.NoError(t, err)
	_, err = pairing.NewPhone(phone).Confirm(t.Context(), code)
	require.NoError(t, err)
	r.drive("signed in", func() bool { return email != "" })
	assert.Equal(t, "thaovy.hlv@gmail.com", email)
	for _, l := range r.log.all() {
		assert.NotContains(t, l, strings.TrimSuffix(srv.url, "/api/v1"), "no URL logged")
	}
	require.ErrorIs(t, setScenario(r.w.mock, "tv.pair.poll", "confirmed"), errNoMock)
}

// TestTVShippedLiveBuildHasNoStates: the shipped live build fills no table
// of states (devParts); every other build has them.
func TestTVShippedLiveBuildHasNoStates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, devParts, len(tvShotStates) > 0)
	_, err := lookupTVState("TV04")
	assert.Equal(t, devParts, err == nil)
}
