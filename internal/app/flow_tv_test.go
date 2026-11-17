//go:build tvos

package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvtest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTVLaunchSignedOutShowsWelcome: a TV with no session shows TV01 as
// the root, and nothing before the session is read (K12).
func TestTVLaunchSignedOutShowsWelcome(t *testing.T) {
	t.Parallel()
	r := newTVRig(t, nil, "")
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	_, ok := r.sh.Screen().(*screens.Welcome)
	assert.True(t, ok, "the welcome screen, not %T", r.sh.Screen())
	assert.Equal(t, 0, r.sh.Stack().Depth())
	_, signed := r.w.SessionUser()
	assert.False(t, signed)
	assert.Equal(t, appVersion, r.w.Version())
}

// TestTVPairingAdoptsBeforeSignedIn: the code the mock gives shows, and
// once the phone confirmed it the session is in the vault (Adopt) before
// OnSignedIn is told, with the account's email; the device document keeps
// it. The device sent is the TV's (tvos, its name).
func TestTVPairingAdoptsBeforeSignedIn(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r := newTVRig(t, secrets, "")
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	require.NoError(t, setScenario(r.w.mock, "tv.pair.poll", "confirmed"))

	var states []pairing.State
	var email string
	var vaultAtSignIn, sessionAtSignIn bool
	p := r.w.Pair(tvenv.PairHooks{
		OnState: func(st pairing.State) { states = append(states, st) },
		OnSignedIn: func(e string) {
			email = e
			_, sessionAtSignIn = r.w.mgr.Session()
			vaultAtSignIn = len(secrets.vals) > 0
		},
		OnError: func(err error) { t.Errorf("pairing: %v", err) },
	})
	r.drive("a code", func() bool { return len(states) > 0 })
	assert.Equal(t, "7K4-M2P", states[0].DisplayCode)
	assert.InDelta(t, 300, p.Remaining().Seconds(), 1)
	r.drive("signed in", func() bool { return email != "" })
	assert.Equal(t, "thaovy.hlv@gmail.com", email)
	assert.True(t, sessionAtSignIn, "the Manager holds the session when OnSignedIn runs")
	assert.True(t, vaultAtSignIn, "the vault holds the session when OnSignedIn runs")
	assert.Equal(t, "thaovy.hlv@gmail.com", r.w.lastEmail())
	u, ok := r.w.SessionUser()
	require.True(t, ok)
	assert.Equal(t, "u_412", u.ID)
	p.Stop()
	assert.Empty(t, r.w.pairList())
}

// TestTVPairingThroughTheScreen: TV01 → TV02 shows the mock's code; with
// the poll confirmed, TV04 becomes the root with the toast of the account.
func TestTVPairingThroughTheScreen(t *testing.T) {
	t.Parallel()
	r := newTVRig(t, nil, "")
	r.mount("TV-toast")
	r.drive("TV02", func() bool { return r.route() == tvenv.RouteCode })
	r.home()
	assert.Equal(t, 0, r.sh.Stack().Depth(), "TV04 is the root")
	assert.True(t, r.sh.ToastHost().Visible())
	assert.Equal(t, "Đã đăng nhập bằng thaovy.hlv@gmail.com", r.sh.ToastHost().Message())
	assert.Empty(t, r.w.pairList(), "no pairing left once signed in")
}

// TestTVRestoreSignedInShowsHome: the session a pairing kept is read at the
// next launch: TV04 as the root.
func TestTVRestoreSignedInShowsHome(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())

	second := newTVRig(t, secrets, dir)
	second.mount("")
	second.home()
	u, ok := second.w.SessionUser()
	require.True(t, ok)
	assert.Equal(t, "thaovy.hlv@gmail.com", u.Email)
	assert.Equal(t, "thaovy.hlv@gmail.com", second.w.lastEmail(), "the device document survives on the state store")
}

// TestTVPlayContinueFromHome: h1 plays the lesson of "Tiếp tục xem"
// (l_3503) from continue.position_sec (72) on the fake engine, on TV07
// with p1 focused; the keep-awake holds while the video plays.
func TestTVPlayContinueFromHome(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())
	r := newTVRig(t, secrets, dir)
	r.mount("")
	r.home()

	ctx := t.Context()
	h, err := r.w.Home(ctx)
	require.NoError(t, err)
	require.NotNil(t, h.Continue)
	assert.Equal(t, tvDemoLesson, h.Continue.Lesson.ID)
	assert.Equal(t, tvDemoAt, h.Continue.PositionSec)
	c, err := r.w.Course(ctx, h.Continue.Course.ID)
	require.NoError(t, err)
	list := tvenv.PlayerLessons(c)
	i := tvenv.LessonIndex(list, h.Continue.Lesson.ID)
	require.GreaterOrEqual(t, i, 0)

	pb := r.w.Playback()
	pb.PlayVideo(list[i], list) // what h1 does (P7.3)
	r.sh.Go(nav.Route{Screen: tvenv.RoutePlayer}, false)
	r.drive("loaded", func() bool {
		if pb.State() == player.StateLoading {
			r.media.Advance(time.Millisecond)
		}

		return pb.State() == player.StatePlaying
	})
	assert.InDelta(t, tvDemoAt, pb.Pos(), 0.5, "from continue.position_sec")
	assert.Equal(t, player.KindVideo, pb.Kind())
	assert.Equal(t, "p1", focusKeyOf(r))
	assert.True(t, r.awake.On(), "the screen saver is kept away while the video plays")

	// Background: the video pauses, the screen is let go (Q-P7-14).
	r.lifecycle(platform.LifecycleBackground)
	r.drive("paused", func() bool { return !pb.Playing() })
	assert.False(t, r.awake.On())
	r.lifecycle(platform.LifecycleActive)
	assert.False(t, pb.Playing(), "the video stays paused back in front")
}

// TestTVBackgroundKeepsMusicPlaying: a sound plays on in the background,
// with no keep-awake.
func TestTVBackgroundKeepsMusicPlaying(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	c, err := r.w.Course(t.Context(), tvDemoCourse)
	require.NoError(t, err)
	list := tvenv.PlayerLessons(c)
	i := tvenv.LessonIndex(list, tvDemoLesson)
	pb := r.w.Playback()
	pb.PlayAudio(list[i], list, tvDemoAt)
	r.sh.SwitchTab(2)
	r.drive("playing", func() bool {
		if pb.State() == player.StateLoading {
			r.media.Advance(time.Millisecond)
		}

		return pb.State() == player.StatePlaying
	})
	assert.InDelta(t, tvDemoAt, pb.Pos(), 0.5, "the audio of another lesson starts at the position given")
	assert.False(t, r.awake.On(), "music lets the screen saver come (Q-P7-11)")
	r.lifecycle(platform.LifecycleBackground)
	assert.True(t, pb.Playing(), "the sound plays on")

	// In the background the engine tells no time; back in front the
	// position is the engine's, 10 s of the rate on.
	rate := pb.Rate()
	r.media.Advance(10 * time.Second)
	r.q.drain()
	r.lifecycle(platform.LifecycleActive)
	assert.InDelta(t, tvDemoAt+10*rate, pb.Pos(), 0.5, "the position caught up with the engine")
	assert.True(t, pb.Playing())
}

// TestTVLogoutClearsSecrets: LogoutTV tells tv.logout, ends the session,
// empties the vault and overwrites the device document.
func TestTVLogoutClearsSecrets(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	r := newTVRig(t, secrets, "")
	ct := &countingTransport{}
	r.deps.API = func(bool) (apiPlan, error) {
		mt, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
		if err != nil {
			return apiPlan{}, err
		}
		t.Cleanup(func() { _ = mt.Close() })
		ct.inner = mt

		return apiPlan{mode: apiModeLive, live: ct, tags: true}, nil
	}
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	r.adoptMock(t.Context())
	require.NotEmpty(t, secrets.vals)
	assert.Equal(t, "thaovy.hlv@gmail.com", r.w.lastEmail())

	done := make(chan error, 1)
	go func() { done <- r.w.LogoutTV(context.Background()) }()
	var err error
	r.drive("logout", func() bool {
		select {
		case err = <-done:
			return true
		default:
			return false
		}
	})
	require.NoError(t, err)
	assert.Equal(t, 1, ct.count("tv.logout"), "POST /tv/logout told")
	assert.Zero(t, ct.count("auth.logout"))
	_, ok := r.w.SessionUser()
	assert.False(t, ok)
	assert.Empty(t, secrets.vals, "the vault is empty")
	assert.Empty(t, r.w.lastEmail(), "the device document is overwritten")
}

// TestTVRefreshRefusedShowsWelcomeWithToast: a 401 whose refresh the server
// refuses ends the session: TV01 as the root and the expired toast.
func TestTVRefreshRefusedShowsWelcomeWithToast(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	require.NoError(t, setScenario(r.w.mock, "me.get", "token_expired"))
	require.NoError(t, setScenario(r.w.mock, "auth.refresh", "refresh_invalid"))
	_, err := r.w.Me(t.Context())
	require.ErrorIs(t, err, api.ErrSignedOut)
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	assert.Equal(t, 0, r.sh.Stack().Depth())
	assert.True(t, r.sh.ToastHost().Visible())
	assert.Equal(t, locales.MustTexts()["session.expired"], r.sh.ToastHost().Message())
}

// TestTVOfflineAtLaunch: a TV signed in that starts with no network shows
// TV04 from the vault, offline, and a read fails as offline.
func TestTVOfflineAtLaunch(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())

	r := newTVRig(t, secrets, dir)
	r.deps.Network = func(func(func())) netWatch { return &fakeTVNet{kind: sys.NetworkNone} }
	r.mount("")
	r.home()
	assert.False(t, r.w.Online())
	_, err := r.w.Courses(t.Context())
	require.ErrorIs(t, err, api.ErrOffline)
}

// TestTVQualityCycleIsStoredAndLimitsTheEngine: CycleQualityTV goes Tự
// động, 1080p, 720p, Tự động, saved on the state store, and tells the engine
// its limit.
func TestTVQualityCycleIsStoredAndLimitsTheEngine(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	pic := &fakePicture{}
	r.w.pb.made(platformMedia{name: "fake-picture", eng: pictureEngine{pic: pic}})
	prefs := r.w.Prefs()
	assert.Equal(t, store.QualityAuto, prefs.Quality())
	prefs.CycleQualityTV()
	assert.Equal(t, store.QualityP1080, prefs.Quality())
	assert.Equal(t, avftv.Quality1080, pic.limits)
	prefs.SetAutoNext(false)
	r.drive("saved", func() bool {
		p := r.w.prefStore.Load().Get()

		return p.Quality == store.QualityP1080 && !p.AutoNext
	})
	raw, err := r.state.Load(store.PrefsKey)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "1080")
}

// TestTVPictureReadyFollowsTheEngine: the first frame (EventVideoSize) makes
// PictureReady true with a ChangeState, and the next load forgets it;
// Place reaches the engine's plane, Mirror its layer.
func TestTVPictureReadyFollowsTheEngine(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	pb := r.w.pb
	pic := &fakePicture{}
	pb.Place(geom.Rect{X: 656, W: 608, H: 1080})
	pb.made(platformMedia{name: "fake-picture", eng: pictureEngine{pic: pic}})
	assert.Equal(t, geom.Rect{X: 656, W: 608, H: 1080}, pic.place, "a Place before the engine is applied once made")

	c, err := r.w.Course(t.Context(), tvDemoCourse)
	require.NoError(t, err)
	list := tvenv.PlayerLessons(c)
	pb.PlayVideo(list[2], list)
	var states int
	stop := pb.Changes().Subscribe(func(c player.Change) {
		if c.Kind == player.ChangeState {
			states++
		}
	})
	defer stop()
	assert.False(t, pb.PictureReady())
	pb.onEngine(media.Event{Kind: media.EventVideoSize, Width: 1080, Height: 1920})
	assert.True(t, pb.PictureReady())
	assert.Equal(t, 1, states)
	pb.onEngine(media.Event{Kind: media.EventVideoSize, Width: 1080, Height: 1920})
	assert.Equal(t, 1, states, "told once a load")
	r.drive("the lesson loaded", func() bool {
		if pb.State() == player.StateLoading {
			r.media.Advance(time.Millisecond)
		}

		return pb.State() == player.StatePlaying && pb.PictureReady()
	})
	eng, ok := pb.late.sys.eng.(*fake.Engine)
	require.True(t, ok, "the rig plays on the fake engine, not %T", pb.late.sys.eng)
	tok := eng.Token() + pb.late.offset
	require.Equal(t, tok, pb.picTok, "the fake engine's first frame of the load")
	pb.SetLoop(true) // seeks to A, where the engine buffers: the frame stays on the plane
	eng.Stall()
	assert.Equal(t, player.StateLoading, pb.State())
	assert.True(t, pb.PictureReady(), "a seek that buffers keeps the picture")
	eng.Unstall()
	pb.onEngine(media.Event{Kind: media.EventState, State: media.StateLoading, Token: tok + 1})
	assert.False(t, pb.PictureReady(), "another load's event forgets it")
	pb.onEngine(media.Event{Kind: media.EventVideoSize, Width: 1080, Height: 1920, Token: tok + 1})
	assert.True(t, pb.PictureReady(), "the new load's first frame")
	assert.True(t, pic.mirror, "the player's mirror (on at first) is the layer's")
	pb.SetMirror(false)
	assert.False(t, pic.mirror)
	pb.PlayVideo(list[3], list)
	assert.False(t, pb.PictureReady(), "another lesson waits for its first frame")
	pb.Leave()
	assert.True(t, pic.place.Empty(), "leaving hides the plane")
}

// TestTVTickForwardDoesNotAllocate: the 250 ms tick through the playback's
// topic allocates nothing.
func TestTVTickForwardDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	r := signedInTV(t)
	pb := r.w.pb
	n := 0
	stop := pb.Changes().Subscribe(func(player.Change) { n++ })
	defer stop()
	tick := player.Change{Kind: player.ChangeTime}
	allocs := testing.AllocsPerRun(200, func() { pb.onChange(tick) })
	assert.Zero(t, allocs)
	assert.Positive(t, n)
}

// BenchmarkTVTickForward measures the tick through the playback's topic.
func BenchmarkTVTickForward(b *testing.B) {
	pb := &tvPlayback{}
	cfg := media.Config{Dispatch: func(fn func()) { fn() }, Clock: clock.NewFake(tvDemoNow)}
	pb.player = player.New(player.Options{Engine: fake.New(cfg, fake.Options{}), Dispatch: cfg.Dispatch})
	stop := pb.changes.Subscribe(func(player.Change) {})
	defer stop()
	tick := player.Change{Kind: player.ChangeTime}
	b.ReportAllocs()
	for b.Loop() {
		pb.onChange(tick)
	}
}

// TestTVPairingOverMockserve: with DFV_API (a transport to cmd/mockserve:
// one overlay with the pair relay) the TV's code is the one a phone
// confirms, and the TV's next poll gets the session.
func TestTVPairingOverMockserve(t *testing.T) {
	t.Parallel()
	mt, err := mock.New(os.DirFS(mockDir), mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	require.NoError(t, err)
	t.Cleanup(func() { _ = mt.Close() })
	ov := store.NewOverlay(mt, store.NewMemory())
	confirmed, err := os.ReadFile(filepath.Join(mockDir, "tv", "pair-poll.confirmed.json"))
	require.NoError(t, err)
	require.NoError(t, ov.EnablePairRelay(store.PairRelayOptions{Confirmed: confirmed}))

	r := newTVRigOf(t, nil, "")
	r.deps.API = liveTo(ov)
	r.deps.Mock = nil
	r.mount("")
	r.drive("TV01", func() bool { return r.route() == tvenv.RouteWelcome })
	assert.Nil(t, r.w.mock, "no mock in the process")
	var code string
	var email string
	p := r.w.Pair(tvenv.PairHooks{
		OnState:    func(st pairing.State) { code = st.DisplayCode },
		OnSignedIn: func(e string) { email = e },
	})
	defer p.Stop()
	r.drive("a code", func() bool { return code != "" })
	assert.NotEqual(t, "7K4-M2P", code, "the relay's code")

	phoneAuth, err := auth.New(auth.Options{Vault: auth.NewSecretVault(&memSecrets{vals: map[string][]byte{}}, "phone")})
	require.NoError(t, err)
	phone, err := api.New(api.Options{Transport: ov, Version: appVersion, Platform: "ios", InstallID: "phone", Tokens: phoneAuth})
	require.NoError(t, err)
	phoneAuth.Bind(phone)
	_, err = phoneAuth.Login(t.Context(), "seed@example.invalid", "x")
	require.NoError(t, err)
	_, err = pairing.NewPhone(phone).Confirm(t.Context(), code)
	require.NoError(t, err)
	r.drive("signed in", func() bool { return email != "" })
	assert.Equal(t, "thaovy.hlv@gmail.com", email)
}

// focusKeyOf is the focus key (or name) of the focused widget, "" for none.
func focusKeyOf(r *tvRig) string {
	f := r.tt.View().Focused()
	if f == nil {
		return ""
	}
	if k, ok := f.(interface{ FocusKey() string }); ok && k.FocusKey() != "" {
		return k.FocusKey()
	}

	return f.Node().Name
}

// signedInTV is a rig on TV04 with the mock's TV session.
func signedInTV(t *testing.T) *tvRig {
	t.Helper()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())
	r := newTVRig(t, secrets, dir)
	r.mount("")
	r.home()

	return r
}

// fakeTVNet is a netWatch of a fixed kind.
type fakeTVNet struct{ kind sys.NetworkKind }

func (n *fakeTVNet) Kind() sys.NetworkKind          { return n.kind }
func (n *fakeTVNet) OnChange(func(sys.NetworkKind)) {}
func (n *fakeTVNet) Close()                         {}

// fakePicture records what the playback tells the engine's plane.
type fakePicture struct {
	place  geom.Rect
	mirror bool
	limits avftv.Quality
}

func (f *fakePicture) Place(r geom.Rect)         { f.place = r }
func (f *fakePicture) SetMirror(on bool)         { f.mirror = on }
func (f *fakePicture) SetLimits(q avftv.Quality) { f.limits = q }

// pictureEngine is a media.Engine (the fake's) with the plane of
// fakePicture, as avftv's.
type pictureEngine struct {
	media.Engine
	pic *fakePicture
}

func (e pictureEngine) Place(r geom.Rect)         { e.pic.Place(r) }
func (e pictureEngine) SetMirror(on bool)         { e.pic.SetMirror(on) }
func (e pictureEngine) SetLimits(q avftv.Quality) { e.pic.SetLimits(q) }

// countingTransport counts the requests by route on the way to inner.
type countingTransport struct {
	inner  api.Transport
	mu     sync.Mutex
	counts map[string]int
}

// Do counts req and hands it to inner.
func (c *countingTransport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	c.mu.Lock()
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	c.counts[req.RouteID]++
	c.mu.Unlock()

	return c.inner.Do(ctx, req)
}

// count is how many requests of route went through.
func (c *countingTransport) count(route string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.counts[route]
}

// TestTVCourseAndAvatarForTheScreens: what the P7.3 screens ask of the
// wire: Course as a prefetch from TV04 (several at once) and for each
// detail, and Avatar with the URL of an answer (this run's media server)
// or a mock-media:// one, both the same picture. A mock-media:// URL with
// no mock in the process is refused, not fetched.
func TestTVCourseAndAvatarForTheScreens(t *testing.T) {
	t.Parallel()
	secrets := &memSecrets{vals: map[string][]byte{}}
	dir := t.TempDir()
	first := newTVRig(t, secrets, dir)
	first.mount("")
	first.drive("TV01", func() bool { return first.route() == tvenv.RouteWelcome })
	first.adoptMock(t.Context())
	r := newTVRig(t, secrets, dir)
	r.mount("")
	r.home()
	ctx := t.Context()

	var wg sync.WaitGroup
	errs := make([]error, 4)
	ids := make([]string, 4)
	for i := range errs {
		wg.Go(func() {
			c, err := r.w.Course(ctx, tvDemoCourse)
			errs[i], ids[i] = err, c.ID
		})
	}
	wg.Wait()
	for i := range errs {
		require.NoError(t, errs[i])
		assert.Equal(t, tvDemoCourse, ids[i])
	}

	m, err := r.w.Me(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, m.User.AvatarURL)
	assert.NotContains(t, m.User.AvatarURL, mockMediaScheme, "the answers carry this run's address")
	byHTTP, err := r.w.Avatar(ctx, m.User.AvatarURL)
	require.NoError(t, err)
	byMock, err := r.w.Avatar(ctx, mockMediaScheme+"avatars/u_412.jpg")
	require.NoError(t, err)
	assert.Equal(t, [2]int{byHTTP.Width, byHTTP.Height}, [2]int{byMock.Width, byMock.Height})
	assert.Positive(t, byMock.Width)

	_, err = r.w.Avatar(ctx, mockMediaScheme+"avatars/none.jpg")
	require.ErrorIs(t, err, core.ErrNotFound)

	live := &tvWire{}
	_, err = live.mediaURL(mockMediaScheme + "avatars/u_412.jpg")
	require.ErrorIs(t, err, core.ErrInvalid)
	got, err := live.mediaURL("http://127.0.0.1:1/a.jpg")
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:1/a.jpg", got)
}

// TestTVDevPrefsSetsThroughTheSettingsCalls: DFV_PREFS's set reaches the
// stored preferences through CycleQualityTV and SetAutoNext; an empty set
// changes nothing.
func TestTVDevPrefsSetsThroughTheSettingsCalls(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	r.w.pb.applyDevPrefs(devPrefSet{})
	assert.Equal(t, store.QualityAuto, r.w.Prefs().Quality())
	assert.True(t, r.w.Prefs().AutoNext())

	r.w.pb.applyDevPrefs(devPrefSet{quality: store.QualityP720, setAutoNext: true})
	assert.Equal(t, store.QualityP720, r.w.Prefs().Quality())
	assert.False(t, r.w.Prefs().AutoNext())
	r.drive("saved", func() bool {
		p := r.w.prefStore.Load().Get()

		return p.Quality == store.QualityP720 && !p.AutoNext
	})

	r.w.pb.applyDevPrefs(devPrefSet{quality: store.QualityAuto})
	assert.Equal(t, store.QualityAuto, r.w.Prefs().Quality())
}

// TestTVResumeDrawsTheWholeFrame checks the return from the screen saver of a TV idle on
// a menu (nothing playing, nothing changing): the app, active again, asks for a frame
// that draws every pixel, as a driver does for a window made again. Without it the
// uikit driver's visibility change draws only what changed meanwhile, which is nothing,
// and what the system dropped of the picture stays black.
func TestTVResumeDrawsTheWholeFrame(t *testing.T) {
	t.Parallel()
	r := signedInTV(t)
	for range 8 { // steady: the home idle, no frame asked
		r.tt.Elapse(time.Second)
	}
	v := r.tt.View()
	drawn := v.FramesDrawn()
	r.tt.Elapse(time.Second)
	require.Equal(t, drawn, v.FramesDrawn(), "an idle menu draws nothing")

	r.lifecycle(platform.LifecycleInactive) // the screen saver comes
	r.lifecycle(platform.LifecycleBackground)
	r.tt.Elapse(time.Second)
	assert.Equal(t, drawn, v.FramesDrawn(), "and draws nothing while it is away")

	r.lifecycle(platform.LifecycleInactive) // and goes
	r.lifecycle(platform.LifecycleActive)
	r.tt.Elapse(tvtest.Frame)
	assert.Greater(t, v.FramesDrawn(), drawn, "back in front the screen is drawn again")
	assert.Equal(t, "home", r.route(), "and it is where it was")
}
