//go:build tvos

// The app's side of the Apple TV screens' contract (tvenv.Services). The
// pattern is services_mobile.go (mobileWire, P3-P6): the screens see
// interfaces that import none of the app's packages, the parts are made by
// the first blocking call (Restore, off the UI thread), every API error
// goes through auth.Manager.Handle, and a sign-out cancels the requests of
// the session before it (scoped). What differs on the TV: X-Platform tvos,
// no password and no biometry (the session comes from a pairing, Adopt),
// no downloads, the small documents on the Mirror of NSUserDefaults and
// files (tvBackend, Q-P7-6), and auth.Manager with TV set, so that its
// Logout tells tv.logout.

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/safe"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
	tvenv "dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvPlatform is the TV's X-Platform and device platform.
const tvPlatform = "tvos"

// How long close waits for a LogoutTV under way: first for it to end by
// itself, then, cut short (logoutRun.hurry), for its local sign-out.
const (
	logoutCloseWait = 2 * time.Second
	logoutHurryWait = time.Second
)

// tvDeps is what the TV's services are built with: the system's parts,
// which a test replaces.
type tvDeps struct {
	// Mock is the mock API's folder; nil (a live build) talks to the live
	// API (apimode.go).
	Mock fs.FS
	// API decides where the requests go (apimode.go); nil is planAPI over
	// the build: the mock, the live API of a -tags live build, or a
	// developer's DFV_API and DFV_LIVE_ROUTES.
	API apiPlanFunc
	// Enqueue runs fn on the UI thread later (host.App.Enqueue).
	Enqueue func(fn func()) bool
	// Secrets opens the store of the session's tokens (sys.OpenSecrets, F1
	// files in Library/Caches on tvOS until G1).
	Secrets func() (auth.SecretStore, error)
	// InstallID reads or makes the install id (sys.InstallID).
	InstallID func() (string, error)
	// DataDir and CacheDir are sys.DataDir and sys.CacheDir.
	DataDir, CacheDir func() (string, error)
	// State opens the store of the TV's small documents (the preferences,
	// the progress queue, the device document) in dir: tvBackend, the
	// Mirror of NSUserDefaults and files. nil is a store.Dir.
	State func(dir string, logf func(format string, args ...any)) store.Backend
	// MockWait is the mock's delay (mock.Options.Wait); nil is a real timer.
	MockWait func(ctx context.Context, d time.Duration) error
	// Clock times the network probe; nil is clock.System on Enqueue.
	Clock clock.Clock
	// Logf logs a line; nil is log.Printf. It never gets tokens.
	Logf func(format string, args ...any)
	// NewMedia makes the engine; nil is the Apple TV's (avftv, the fake
	// engine off tvOS).
	NewMedia newMediaFunc
	// Awake keeps the screen saver away while a video plays
	// (sys.KeepAwake); nil keeps nothing on.
	Awake awakeSink
	// NowPlaying is the system's Now Playing; nil shows nothing.
	NowPlaying npSink
	// OnLifecycle adds a listener of the app's lifecycle, told on the UI
	// thread (host.App.OnLifecycle); nil hears none.
	OnLifecycle func(fn func(platform.LifecycleState)) (remove func())
	// Network starts watching the device's network (watchNetwork); nil
	// knows none.
	Network func(dispatch func(func())) netWatch
	// DeviceName is the TV's name sent with a pairing (sys.DeviceName);
	// nil is "Apple TV".
	DeviceName func() string
	// ShotWall is the real clock of a shot's rest (devShot); nil is
	// time.Now. A Tester, whose view runs far ahead of the wall clock,
	// passes its view's, so that TV02's countdown is not spent meanwhile.
	ShotWall func() time.Time
}

// tvKnobs are what a shot state sets before the shell starts
// (states_tv.go); the zero value is the app as it ships.
type tvKnobs struct {
	// now pins the clock of the screens; zero is the device's.
	now time.Time
	// offline makes every request fail as a dead network does.
	offline bool
	// prepare runs once the parts are made, before the session is read:
	// the state's way to it through the mock.
	prepare func(ctx context.Context, w *tvWire) error
	// media makes the engine in place of the system's (fakeMediaOf).
	media newMediaFunc
	// memOverlay keeps the mock's overlay in memory, empty at launch.
	memOverlay bool
	// mediaClock is the frozen clock of media's fake engine, which the
	// playback states move (tvDemoVideo); nil for the system's.
	mediaClock *clock.Fake
}

// tvWire is tvenv.Services over the app's parts: the API client on the
// mock transport (or cmd/mockserve's), the session's Manager and vault,
// the cache, the preferences and the progress queue on the TV's state
// store, the playback (player_tv.go) and the system's services.
type tvWire struct {
	deps     tvDeps
	knobs    tvKnobs
	dispatch sys.Dispatch
	clk      clock.Clock
	online   *store.Online
	images   components.ImageCache
	// covers loads the covers of the courses (covers.go); disk is its folder.
	covers *coverLoader
	disk   *store.Images
	http   *http.Client
	// ui is the clock of the playback and of the pairing: the view's timers
	// once attached (uiClock). UI thread only.
	ui       *uiClock
	pb       *tvPlayback
	net      netWatch
	bound    bool   // ui is bound to the view (UI thread)
	stopLife func() // ends the lifecycle listener
	bg       sync.WaitGroup
	// life ends with the window: the pairings' requests.
	life    context.Context //nolint:containedctx // the window's lifetime
	endLife context.CancelFunc

	once sync.Once
	err  error
	// Made by setup, read only after it.
	cache   *store.Cache
	mgr     *auth.Manager
	mock    inProcMock
	overlay mockOverlay
	tr      *devTransport
	client  *api.Client
	state   store.Backend
	probe   *onlineProbe
	// built is set once build made the parts.
	built atomic.Bool
	// noMock is set when the state's prepare needed the mock in the
	// process and the API is a server (errNoMock): its runner stops.
	noMock     atomic.Bool
	sessGen    atomic.Uint64
	apiReached atomic.Bool
	installID  string
	prefStore  atomic.Pointer[store.PrefsStore]
	queue      atomic.Pointer[progress.Queue]
	appConfig  atomic.Pointer[model.AppConfig]
	tracker    atomic.Pointer[progress.Tracker]
	// pairStarts counts the codes asked for (POST /tv/pair/start).
	pairStarts atomic.Int64

	// logoutMu serializes LogoutTV and adopt: TV12 leaves for TV01 at once
	// and the logout (up to logoutBudget + logoutServerTimeout) runs on,
	// so a phone may confirm a new pairing meanwhile. Its session is
	// installed only once the logout cleared the one before (adopt cuts
	// the logout's waits short), and a logout never clears it.
	logoutMu sync.Mutex
	// adopts counts the sessions adopted, under logoutMu: a LogoutTV asked
	// before the last of them is stale and clears nothing.
	adopts atomic.Uint64

	mu       sync.Mutex
	logout   *logoutRun      // the LogoutTV under way; nil when none
	sess     context.Context //nolint:containedctx // the session's lifetime: canceled at sign-out
	endSess  context.CancelFunc
	watchers []*watch
	pairs    []*tvPairing
	view     *ui.View
	closed   bool
	watched  map[string]bool
}

// tvPairing is tvenv.Pairing over pairing.TV: the confirmed session is
// adopted (kept in the vault) before the screen hears OnSignedIn. UI
// thread.
type tvPairing struct {
	w       *tvWire
	h       tvenv.PairHooks
	tv      *pairing.TV
	stopped bool
}

// tvPairer is pairing.Pairer over the client, made first.
type tvPairer struct{ w *tvWire }

// logoutRun is a LogoutTV under way: hurry cuts its waits short (the last
// save, the flush, the server's answer), so that only its local sign-out
// is left; done is closed once it returned.
type logoutRun struct {
	hurry context.CancelFunc
	done  chan struct{}
}

var (
	_ tvenv.Services = (*tvWire)(nil)
	_ tvenv.Pairing  = (*tvPairing)(nil)
	_ pairing.Pairer = tvPairer{}
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVWire returns the services over deps, with the knobs of a shot state
// (the zero knobs as the app ships). Nothing is opened yet but the
// playback, whose engine is made once the view is attached. UI thread.
func newTVWire(deps tvDeps, knobs tvKnobs) *tvWire {
	if deps.Logf == nil {
		deps.Logf = log.Printf
	}
	w := &tvWire{deps: deps, knobs: knobs, online: store.NewOnline()}
	w.dispatch = func(fn func()) { deps.Enqueue(fn) }
	w.clk = deps.Clock
	if w.clk == nil {
		w.clk = clock.System(w.dispatch)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the stdlib default
	tr.ResponseHeaderTimeout = 15 * time.Second
	w.http = &http.Client{Transport: tr, Timeout: 30 * time.Second}
	w.sess, w.endSess = context.WithCancel(context.Background())
	w.life, w.endLife = context.WithCancel(context.Background())
	wall := time.Now
	if deps.Clock != nil {
		wall = deps.Clock.Now
	}
	w.ui = newUIClock(wall)
	w.apiReached.Store(true)
	w.pb = newTVPlayback(w)
	if deps.OnLifecycle != nil {
		w.stopLife = deps.OnLifecycle(w.lifecycle)
	}
	if deps.Network != nil {
		w.net = deps.Network(w.dispatch)
		w.net.OnChange(w.netChanged)
	}

	return w
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Restore reads the session kept in the vault, after making the parts.
func (w *tvWire) Restore(ctx context.Context) (bool, error) {
	if err := w.setup(ctx); err != nil {
		return false, err
	}
	s, ok := w.mgr.Restore()
	if ok {
		w.rememberEmail(s.User.Email)
	}

	return ok, nil
}

// SessionUser is the signed-in user; false before Restore and signed out.
func (w *tvWire) SessionUser() (model.User, bool) {
	if !w.built.Load() {
		return model.User{}, false
	}
	s, ok := w.mgr.Session()
	u := s.User

	return model.User{ID: u.ID, FirstName: u.FirstName, LastName: u.LastName, FullName: u.FullName, Email: u.Email, AvatarURL: u.AvatarURL}, ok
}

// OnSignedOut calls fn on the UI thread when the server ends the session.
func (w *tvWire) OnSignedOut(fn func(reason error)) (stop func()) {
	return w.addWatch(&watch{out: fn})
}

// Pair starts the pairing of this TV: a code, then a poll every
// poll_interval_sec on the view's clock; the confirmed session is adopted
// off the UI thread, then h.OnSignedIn runs. UI thread.
func (w *tvWire) Pair(h tvenv.PairHooks) tvenv.Pairing {
	p := &tvPairing{w: w, h: h}
	p.tv = pairing.NewTV(pairing.TVOptions{
		Client:   tvPairer{w: w},
		Clock:    w.ui,
		Dispatch: w.dispatch,
		// The app version and the install id are the client's own (X-*).
		Device:      model.DeviceInfo{Platform: tvPlatform, Name: w.deviceName()},
		OnState:     p.onState,
		OnConfirmed: p.onConfirmed,
		OnError:     p.onError,
	})
	w.mu.Lock()
	closed := w.closed
	if !closed {
		w.pairs = append(w.pairs, p)
	}
	w.mu.Unlock()
	if closed {
		p.stopped = true

		return p
	}
	p.tv.Start(w.life)

	return p
}

// LogoutTV is the sign-out of TV12 (`s5`): within logoutBudget the save of
// the position under way is waited for and the queue sent with the token
// still valid; then POST /tv/logout (best effort, logoutServerTimeout) and
// the local sign-out (the vault, the cache, the queue, the device
// document). Blocks; TV12 runs it on a goroutine of its own and is on TV01
// already. A session a pairing adopts meanwhile waits for it and is not
// cleared by it (logoutMu); one adopted before it began makes it a no-op.
func (w *tvWire) LogoutTV(ctx context.Context) error {
	return w.logoutAsked(ctx, w.adopts.Load())
}

// logoutAsked is LogoutTV asked when gen sessions had been adopted: when
// another was adopted before it holds logoutMu, the user signed in again
// and it does nothing.
func (w *tvWire) logoutAsked(ctx context.Context, gen uint64) error {
	began := time.Now()
	if err := w.setup(ctx); err != nil {
		return err
	}
	w.logoutMu.Lock()
	defer w.logoutMu.Unlock()
	if w.adopts.Load() != gen {
		w.deps.Logf("dancefitvn: logout: skipped, a session was adopted after it was asked")

		return nil
	}
	ctx, run := w.beginLogout(ctx)
	defer w.endLogout(run)
	budget, cancel := context.WithTimeout(ctx, logoutBudget)
	defer cancel()
	if t := w.tracker.Load(); t != nil {
		if err := t.Wait(budget); err != nil {
			w.deps.Logf("dancefitvn: logout: the last save of the position: %v", err)
		}
	}
	w.flushBeforeLogout(budget)
	sctx, scancel := context.WithTimeout(ctx, logoutServerTimeout)
	err := w.mgr.Logout(sctx)
	scancel()
	w.clearQueue()
	w.forgetEmail()
	w.deps.Logf("dancefitvn: logout: the TV signed out in %v", time.Since(began).Round(time.Millisecond))
	if err != nil {
		return fmt.Errorf("app: logout: %w", err)
	}

	return nil
}

// Home is GET /me/home.
func (w *tvWire) Home(ctx context.Context) (model.Home, error) {
	if err := w.setup(ctx); err != nil {
		return model.Home{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	h, err := w.client.Home(ctx)
	w.mgr.Handle(err)

	return h, err
}

// Courses is GET /me/courses, every page merged: it stops on the last page,
// and on a cursor seen before (the mock's paged scenario repeats one).
func (w *tvWire) Courses(ctx context.Context) (model.CoursePage, error) {
	if err := w.setup(ctx); err != nil {
		return model.CoursePage{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	var all model.CoursePage
	var seen []string
	cursor := ""
	for {
		p, err := w.client.MyCourses(ctx, cursor, coursesPage)
		if err != nil {
			w.mgr.Handle(err)

			return model.CoursePage{}, err
		}
		items := all.Items
		all = p
		all.Items = append(items, p.Items...)
		next := p.Page.NextCursor
		if next == nil || *next == "" || slices.Contains(seen, *next) {
			all.Page.NextCursor = nil

			return all, nil
		}
		seen = append(seen, *next)
		cursor = *next
	}
}

// Course is GET /courses/{id}.
func (w *tvWire) Course(ctx context.Context, id string) (model.Course, error) {
	if err := w.setup(ctx); err != nil {
		return model.Course{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	c, err := w.client.Course(ctx, id)
	w.mgr.Handle(err)

	return c, err
}

// Me is GET /me.
func (w *tvWire) Me(ctx context.Context) (model.Me, error) {
	if err := w.setup(ctx); err != nil {
		return model.Me{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	m, err := w.client.Me(ctx)
	w.mgr.Handle(err)

	return m, err
}

// Config is GET /app/config; its intervals are kept for the progress
// tracker.
func (w *tvWire) Config(ctx context.Context) (model.AppConfig, error) {
	if err := w.setup(ctx); err != nil {
		return model.AppConfig{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	c, err := w.client.Config(ctx)
	w.mgr.Handle(err)
	if err == nil {
		w.appConfig.Store(&c)
	}

	return c, err
}

// Avatar loads the picture at url, once per URL (components.ImageCache).
func (w *tvWire) Avatar(ctx context.Context, url string) (*scene.Image, error) {
	if err := w.setup(ctx); err != nil {
		return nil, err
	}

	return w.images.Loader(url, func(ctx context.Context) (*scene.Image, error) {
		return safe.Value("app.avatar", func() (*scene.Image, error) { return w.fetchImage(ctx, url) })
	})(ctx)
}

// WatchedLocal tells whether lessonID was watched in this run (the tracker
// saw 90 % of it, or the server answered a save of it as watched). UI
// thread.
func (w *tvWire) WatchedLocal(lessonID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.watched[lessonID]
}

// Online tells whether the app thinks it is online.
func (w *tvWire) Online() bool { return w.online.Online() }

// WatchOnline calls fn on the UI thread at each change of Online.
func (w *tvWire) WatchOnline(fn func(online bool)) (stop func()) {
	return w.addWatch(&watch{online: fn})
}

// Playback is the one player of the window. UI thread.
func (w *tvWire) Playback() tvenv.Playback { return w.pb }

// Prefs are the preferences of the user. UI thread.
func (w *tvWire) Prefs() tvenv.Prefs { return w.pb.prefs }

// Version is the app's version (TV12's footer, K2).
func (w *tvWire) Version() string { return appVersion }

// Cover loads the cover of a course at url (K14): the real picture in the
// live mode, from the disk cache or the network and reduced to the size it
// is drawn at; core.ErrUnsupported for the mock's, which keeps the vector
// poster. Blocks.
func (w *tvWire) Cover(ctx context.Context, url string) (*scene.Image, error) {
	if err := w.setup(ctx); err != nil {
		return nil, err
	}

	return w.covers.Load(ctx, url)
}

// Now is the device's clock, or the one a shot state pinned.
func (w *tvWire) Now() time.Time {
	if !w.knobs.now.IsZero() {
		return w.knobs.now
	}

	return time.Now()
}

// attach gives the services the view: its timers become the clock of the
// playback and of the pairing (bound once), its canvas takes the video's
// layer, and the engine is made at the next turn. UI thread.
func (w *tvWire) attach(v *ui.View) {
	w.mu.Lock()
	w.view = v
	w.mu.Unlock()
	if !w.bound && v != nil {
		w.bound = true
		w.ui.Bind(v)
		w.dispatch(w.pb.attached)
	}
}

// close ends the pairings and the session's requests and closes the parts,
// at the window's close: the playback first (its last position is told to
// the saves), then the goroutines are waited for and the state store
// synced. UI thread.
func (w *tvWire) close() {
	if w.stopLife != nil {
		w.stopLife()
		w.stopLife = nil
	}
	w.waitLogout(logoutCloseWait, logoutHurryWait)
	for _, p := range w.pairList() {
		p.Stop()
	}
	w.pb.close()
	if w.net != nil {
		w.net.Close()
	}
	w.once.Do(func() { w.err = errWireClosed })
	w.mu.Lock()
	w.closed = true
	w.endSess()
	w.endLife()
	w.view, w.watchers, w.pairs = nil, nil, nil
	w.mu.Unlock()
	w.bg.Wait()
	if w.probe != nil {
		w.probe.close()
	}
	if w.mock != nil {
		if err := w.mock.Close(); err != nil {
			w.deps.Logf("dancefitvn: close mock: %v", err)
		}
	}
	if w.cache != nil {
		w.cache.Close()
	}
	w.disk.Close()
	w.syncState()
}

// Stop ends the polls; no hook is called after it, an adoption under way
// included. UI thread.
func (p *tvPairing) Stop() {
	if p.stopped {
		return
	}
	p.stopped = true
	p.tv.Stop()
	p.w.dropPairing(p)
}

// Remaining is how long the code shown has left.
func (p *tvPairing) Remaining() time.Duration { return p.tv.Remaining() }

// Current is the code shown, false before the first.
func (p *tvPairing) Current() (pairing.State, bool) { return p.tv.Current() }

// StartPairing is POST /tv/pair/start, the parts made first.
func (c tvPairer) StartPairing(ctx context.Context, dev model.DeviceInfo) (model.PairStart, error) {
	if err := c.w.setup(ctx); err != nil {
		return model.PairStart{}, err
	}
	c.w.pairStarts.Add(1)
	if devBuild {
		c.w.deps.Logf("dfv: pair start: platform %s, device name %q", dev.Platform, dev.Name)
	}

	return c.w.client.StartPairing(ctx, dev)
}

// PollPairing is POST /tv/pair/poll.
func (c tvPairer) PollPairing(ctx context.Context, code, pollSecret string) (model.PairPoll, error) {
	if err := c.w.setup(ctx); err != nil {
		return model.PairPoll{}, err
	}

	return c.w.client.PollPairing(ctx, code, pollSecret)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setup makes the parts once. Off the UI thread.
func (w *tvWire) setup(ctx context.Context) error {
	w.once.Do(func() { w.err = w.build(ctx) })

	return w.err
}

// build is setup's work: the secrets, the install id, the state store, the
// cache, the vault, the Manager, the transport (cmd/mockserve's, or the
// mock with its overlay in the process) and the client, then the state's
// prepare.
func (w *tvWire) build(ctx context.Context) error {
	d := w.deps
	plan, err := buildPlan(d.API, !liveBuild && d.Mock != nil)
	if err != nil {
		d.Logf("dancefitvn: %v", err)
	}
	if plan.mode == apiModeUnknown {
		return fmt.Errorf("app: services: %w", err)
	}
	data, err := d.DataDir()
	if err != nil {
		return fmt.Errorf("app: services: %w", err)
	}
	cacheDir, err := d.CacheDir()
	if err != nil {
		return fmt.Errorf("app: services: %w", err)
	}
	secrets, err := d.Secrets()
	if err != nil {
		return fmt.Errorf("app: services: secrets: %w", err)
	}
	id, err := d.InstallID()
	if err != nil {
		return fmt.Errorf("app: services: install id: %w", err)
	}
	w.installID = id
	stateDir := filepath.Join(data, tvStateFolder)
	if d.State != nil {
		w.state = d.State(stateDir, d.Logf)
	} else {
		w.state = store.NewDir(stateDir)
	}
	w.cache = store.OpenCache(filepath.Join(cacheDir, apiFolder), 0)
	w.disk = store.OpenImages(filepath.Join(cacheDir, coversFolder), 0)
	w.mgr, err = auth.New(auth.Options{
		Vault:       auth.NewSecretVault(secrets, id),
		TV:          true,
		ClearCache:  w.clearCache,
		OnSignedOut: w.signedOut,
		OnVaultError: func(op string, err error) {
			d.Logf("dancefitvn: vault %s: %v", op, err)
		},
	})
	if err != nil {
		return fmt.Errorf("app: services: %w", err)
	}
	var ob store.Backend = store.NewDir(data) // <data>/mock-overlay.json (Q-P1-13)
	if w.knobs.memOverlay {
		ob = store.NewMemory()
	}
	parts, err := wireAPI(apiWireIn{plan: plan, mockFS: d.Mock, wait: d.MockWait, logf: d.Logf, overlay: ob})
	if err != nil {
		return err
	}
	w.mock, w.overlay, w.tr = parts.mock, parts.overlay, parts.tr
	w.covers = newCoverLoader(&w.images, w.disk, w.fetchBytes, func(u string) bool { return isMockMedia(w.mock, u) })
	d.Logf("dancefitvn: API: %s", plan)
	w.tr.setOffline(w.knobs.offline)
	w.client, err = api.New(api.Options{
		Transport: w.tr, Version: appVersion, Platform: tvPlatform, InstallID: id,
		Tokens: w.mgr, Cache: w.cache, OnNetwork: w.clientNetwork,
	})
	if err != nil {
		return fmt.Errorf("app: services: %w", err)
	}
	w.mgr.Bind(w.client)
	w.probe = newOnlineProbe(w.clk, w.probeServer)
	w.online.Subscribe(w.networkChanged)
	w.openPrefs()
	if w.knobs.prepare != nil {
		if err := w.knobs.prepare(ctx, w); err != nil {
			d.Logf("dancefitvn: state: %v", err)
			w.noMock.Store(errors.Is(err, errNoMock))
		}
	}
	w.built.Store(true)
	if w.netKind() == sys.NetworkNone {
		w.tr.setNetDown(true)
		w.online.Set(false)
	}

	return nil
}

// openPrefs opens the preferences and the progress queue on the state
// store, off the UI thread (setup). A queue that cannot be read is logged
// and leaves the saves without one.
func (w *tvWire) openPrefs() {
	ps := store.OpenPrefs(w.state, store.DefaultPrefs(true).WiFiOnly)
	if err := ps.LoadErr(); err != nil {
		w.deps.Logf("dancefitvn: prefs: %v", err)
	}
	w.prefStore.Store(ps)
	w.dispatch(w.pb.prefsOpened)
	q, err := progress.NewQueue(w.state, tvSender{w: w})
	if err != nil {
		w.deps.Logf("dancefitvn: progress queue: %v", err)

		return
	}
	w.queue.Store(q)
}

// lifecycle follows the app's standing: the playback's (player_tv.go),
// and in the background the state store is synced.
func (w *tvWire) lifecycle(state platform.LifecycleState) {
	w.deps.Logf("dancefitvn: app %v", state)
	w.pb.lifecycle(state)
	if state == platform.LifecycleBackground {
		w.syncState()
	}
}

// syncState writes the state store's documents through (NSUserDefaults'
// synchronize, store.Mirror.Sync); a failure is logged.
func (w *tvWire) syncState() {
	sy, ok := w.state.(store.Syncer)
	if !ok || !w.built.Load() {
		return
	}
	if err := sy.Sync(); err != nil {
		w.deps.Logf("dancefitvn: store: sync: %v", err)
	}
}

// netKind is the kind of the device's network now.
func (w *tvWire) netKind() sys.NetworkKind {
	if w.net == nil {
		return sys.NetworkUnknown
	}

	return w.net.Kind()
}

// netChanged follows a change of the kind of network, on the UI thread: none
// is offline at once; a network back while offline is probed at once. While
// the API is the mock, the kind also cuts its transport.
func (w *tvWire) netChanged(k sys.NetworkKind) {
	if !w.built.Load() || k == sys.NetworkUnknown {
		return
	}
	w.deps.Logf("dancefitvn: network %s: online %t", k, w.online.Online())
	w.tr.setNetDown(k == sys.NetworkNone)
	switch {
	case k == sys.NetworkNone:
		w.online.Set(false)
	case !w.online.Online():
		w.probe.now()
	default:
	}
}

// clientNetwork is the client's OnNetwork.
func (w *tvWire) clientNetwork(online bool) {
	w.apiReached.Store(online)
	w.online.Set(online)
}

// probeServer is the probe of the network (GET /app/config): it succeeds
// only when the server answered.
func (w *tvWire) probeServer(ctx context.Context) error {
	if _, err := w.client.Config(ctx); err != nil {
		return err
	}
	if !w.apiReached.Load() {
		return errProbeCached
	}
	w.online.Set(true)

	return nil
}

// scoped returns ctx ended also by the end of the session.
func (w *tvWire) scoped(ctx context.Context) (context.Context, context.CancelFunc) {
	w.mu.Lock()
	sess := w.sess
	w.mu.Unlock()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(sess, cancel)

	return ctx, func() {
		stop()
		cancel()
	}
}

// clearCache is the Manager's ClearCache, called at every sign-out and when
// another user signs in: the requests of the session before are canceled,
// the cache emptied, the lesson playing left and the queue emptied.
func (w *tvWire) clearCache() {
	w.mu.Lock()
	w.endSess()
	if !w.closed {
		w.sess, w.endSess = context.WithCancel(context.Background())
	}
	w.mu.Unlock()
	w.sessGen.Add(1)
	w.cache.Clear()
	w.forgetWatched()
	w.dispatch(func() {
		w.pb.signedOut()
		w.clearQueue()
	})
}

// signedOut is the Manager's OnSignedOut: the watchers are told on the UI
// thread, and the device document is overwritten (the Mirror has no
// Remove).
func (w *tvWire) signedOut(reason error) {
	w.forgetEmail()
	for _, wt := range w.watchList() {
		if wt.out != nil {
			fn := wt.out
			w.dispatch(func() { fn(reason) })
		}
	}
}

// networkChanged is Online's subscriber: the probe starts or stops, the
// positions saved offline are sent, and the watchers are told.
func (w *tvWire) networkChanged(on bool) {
	if on {
		w.probe.stop()
		w.flushProgress()
	} else {
		w.probe.start()
	}
	for _, wt := range w.watchList() {
		if wt.online != nil {
			fn := wt.online
			w.dispatch(func() { fn(on) })
		}
	}
}

// addWatch registers wt and returns its stop.
func (w *tvWire) addWatch(wt *watch) func() {
	w.mu.Lock()
	w.watchers = append(w.watchers, wt)
	w.mu.Unlock()

	return func() {
		w.mu.Lock()
		w.watchers = slices.DeleteFunc(w.watchers, func(x *watch) bool { return x == wt })
		w.mu.Unlock()
	}
}

// watchList returns the watchers now.
func (w *tvWire) watchList() []*watch {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.watchers)
}

// pairList returns the pairings under way.
func (w *tvWire) pairList() []*tvPairing {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.pairs)
}

// codeShown tells whether a pairing under way shows a code. UI thread.
func (w *tvWire) codeShown() bool {
	for _, p := range w.pairList() {
		if _, ok := p.Current(); ok {
			return true
		}
	}

	return false
}

// dropPairing forgets p.
func (w *tvWire) dropPairing(p *tvPairing) {
	w.mu.Lock()
	w.pairs = slices.DeleteFunc(w.pairs, func(x *tvPairing) bool { return x == p })
	w.mu.Unlock()
}

// deviceName is the TV's name for the pairing.
func (w *tvWire) deviceName() string {
	if w.deps.DeviceName == nil {
		return fallbackTVName
	}

	return w.deps.DeviceName()
}

// fallbackTVName is the TV's name without a system to ask (Q-P7-13).
const fallbackTVName = "Apple TV"

// adopt keeps the session a phone confirmed: in the vault (Adopt, which
// may block on the first write) and the email in the device document.
// Off the UI thread.
//
// A LogoutTV under way is cut short and waited for first (logoutMu): it
// clears the session before, then this one is installed.
func (w *tvWire) adopt(s model.Session) error {
	if err := w.setup(w.life); err != nil {
		return err
	}
	w.hurryLogout()
	w.logoutMu.Lock()
	defer w.logoutMu.Unlock()
	if err := w.mgr.Adopt(s); err != nil {
		return fmt.Errorf("app: pairing: adopt: %w", err)
	}
	w.adopts.Add(1)
	w.rememberEmail(s.User.Email)

	return nil
}

// beginLogout makes the run of a LogoutTV holding logoutMu: its context,
// which hurry cancels, and its done channel.
func (w *tvWire) beginLogout(ctx context.Context) (context.Context, *logoutRun) {
	ctx, cancel := context.WithCancel(ctx)
	run := &logoutRun{hurry: cancel, done: make(chan struct{})}
	w.mu.Lock()
	w.logout = run
	w.mu.Unlock()

	return ctx, run
}

// endLogout ends the run: no hurry left to give, done closed.
func (w *tvWire) endLogout(run *logoutRun) {
	run.hurry()
	w.mu.Lock()
	if w.logout == run {
		w.logout = nil
	}
	w.mu.Unlock()
	close(run.done)
}

// hurryLogout cuts a LogoutTV under way short: what it waits for (the
// last save, the flush, POST /tv/logout) gives up, its local sign-out is
// left. Any goroutine.
func (w *tvWire) hurryLogout() {
	w.mu.Lock()
	run := w.logout
	w.mu.Unlock()
	if run != nil {
		w.deps.Logf("dancefitvn: logout: cut short by a new session")
		run.hurry()
	}
}

// waitLogout waits for a LogoutTV under way at most patient, then cuts it
// short and waits at most hurried more, for its local sign-out; it returns
// at once without one. For close: the user who left TV12 expects to be
// signed out at the next launch.
func (w *tvWire) waitLogout(patient, hurried time.Duration) {
	w.mu.Lock()
	run := w.logout
	w.mu.Unlock()
	if run == nil {
		return
	}
	t := time.NewTimer(patient)
	defer t.Stop()
	select {
	case <-run.done:
		return
	case <-t.C:
	}
	run.hurry()
	t.Reset(hurried)
	select {
	case <-run.done:
	case <-t.C:
		w.deps.Logf("dancefitvn: logout: still under way at close")
	}
}

// rememberEmail keeps email as the device document's.
func (w *tvWire) rememberEmail(email string) {
	if email == "" || w.state == nil {
		return
	}
	w.saveDevice(deviceDoc{LastEmail: email})
}

// forgetEmail overwrites the device document at a sign-out.
func (w *tvWire) forgetEmail() {
	if w.state == nil {
		return
	}
	w.saveDevice(deviceDoc{})
}

// saveDevice writes the device document; a failure is logged.
func (w *tvWire) saveDevice(doc deviceDoc) {
	raw, err := json.Marshal(doc)
	if err == nil {
		err = w.state.Save(deviceKey, raw)
	}
	if err != nil {
		w.deps.Logf("dancefitvn: device document: %v", err)
	}
}

// lastEmail reads the device document's email; "" when there is none.
func (w *tvWire) lastEmail() string {
	if w.state == nil {
		return ""
	}
	raw, err := w.state.Load(deviceKey)
	if err != nil {
		if !errors.Is(err, core.ErrNotFound) {
			w.deps.Logf("dancefitvn: device document: %v", err)
		}

		return ""
	}
	var doc deviceDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	return doc.LastEmail
}

// fetchImage reads and decodes the picture at url.
func (w *tvWire) fetchImage(ctx context.Context, url string) (*scene.Image, error) {
	data, err := w.fetchBytes(ctx, url, maxImageBytes)
	if err != nil {
		return nil, err
	}
	img, err := decodePicture(data)
	if err != nil {
		return nil, fmt.Errorf("app: image: %w", err)
	}

	return img, nil
}

// fetchBytes reads limit bytes at most of the file at url; the mock's media
// server is started first when the mock is in the process. Blocks.
func (w *tvWire) fetchBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	if w.mock != nil {
		if err := w.mock.Media().Ensure(); err != nil {
			return nil, fmt.Errorf("app: image: %w", err)
		}
	}
	url, err := w.mediaURL(url)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("app: image: %w: %w", core.ErrInvalid, err)
	}
	resp, err := w.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("app: image: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("app: image: status %d: %w", resp.StatusCode, core.ErrNotFound)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, fmt.Errorf("app: image: %w", err)
	}

	return data, nil
}

// mediaURL returns url with a mock-media:// scheme (the mock's files write
// it; a screen may hold one the overlay kept) put on the media server of
// this run; other URLs as they are. Without the mock in the process
// (DFV_API, a live server) there is no server to point it at:
// core.ErrInvalid. The shipped live build, which has no mock, takes every
// URL as it is.
func (w *tvWire) mediaURL(url string) (string, error) {
	if !devParts {
		return url, nil
	}
	rest, ok := strings.CutPrefix(url, mockMediaScheme)
	if !ok {
		return url, nil
	}
	if w.mock == nil {
		return "", fmt.Errorf("app: image: %s without the mock: %w", mockMediaScheme, core.ErrInvalid)
	}

	return w.mock.Media().BaseURL() + "/" + rest, nil
}

// background runs fn on a goroutine that close waits for; none once
// closed.
func (w *tvWire) background(fn func()) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()

		return
	}
	w.bg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.bg.Done()
		safe.Run("app.background", fn)
	}()
}

// markWatched remembers lessonID as watched in this run. UI thread.
func (w *tvWire) markWatched(lessonID string) {
	if lessonID == "" {
		return
	}
	w.mu.Lock()
	if w.watched == nil {
		w.watched = map[string]bool{}
	}
	w.watched[lessonID] = true
	w.mu.Unlock()
}

// forgetWatched drops what this run watched, at the end of a session.
func (w *tvWire) forgetWatched() {
	w.mu.Lock()
	clear(w.watched)
	w.mu.Unlock()
}

// flushProgress sends the positions saved offline, on a goroutine, when
// there are some and a user is signed in. Any goroutine.
func (w *tvWire) flushProgress() {
	q := w.queue.Load()
	if q == nil || q.Len() == 0 || !w.built.Load() {
		return
	}
	if _, ok := w.mgr.Session(); !ok {
		return
	}
	w.background(func() {
		ctx, done := w.scoped(context.Background())
		defer done()
		ctx, cancel := context.WithTimeout(ctx, flushTimeout)
		defer cancel()
		res, err := q.Flush(ctx)
		switch {
		case err != nil && !errors.Is(err, context.Canceled):
			w.deps.Logf("dancefitvn: progress flush: %d sent, %d left: %v", res.Sent, q.Len(), err)
		case res.Sent > 0:
			w.deps.Logf("dancefitvn: progress flush: %d sent, %d accepted, %d rejected", res.Sent, res.Accepted, len(res.Rejected))
		default:
		}
	})
}

// flushBeforeLogout sends the queue once, within ctx, with the session's
// token still valid. Blocks.
func (w *tvWire) flushBeforeLogout(ctx context.Context) {
	q := w.queue.Load()
	if q == nil || q.Len() == 0 {
		return
	}
	if _, ok := w.mgr.Session(); !ok {
		return
	}
	fctx, done := w.scoped(ctx)
	defer done()
	res, err := q.Flush(fctx)
	if err != nil {
		w.deps.Logf("dancefitvn: progress flush at logout: %d sent, %d dropped: %v", res.Sent, q.Len(), err)
	}
}

// clearQueue empties the offline progress queue: the positions of one user
// are never sent with the token of the next.
func (w *tvWire) clearQueue() {
	q := w.queue.Load()
	if q == nil {
		return
	}
	if err := q.Clear(); err != nil {
		w.deps.Logf("dancefitvn: progress queue clear: %v", err)
	}
}

// onState hands a new code to the screen.
func (p *tvPairing) onState(st pairing.State) {
	if p.stopped || p.h.OnState == nil {
		return
	}
	p.h.OnState(st)
}

// onError hands a failure the pairing retries by itself to the screen.
func (p *tvPairing) onError(err error) {
	if p.stopped || p.h.OnError == nil {
		return
	}
	p.h.OnError(err)
}

// onConfirmed is the code confirmed on a phone (the pairing has stopped):
// the session is adopted off the UI thread, then the screen hears
// OnSignedIn with the account's email, or OnError and a new code when the
// vault refused it.
func (p *tvPairing) onConfirmed(s model.Session) {
	if p.stopped {
		return
	}
	w := p.w
	w.background(func() {
		err := w.adopt(s)
		w.dispatch(func() { p.adopted(s.User.Email, err) })
	})
}

// adopted follows the adoption, on the UI thread.
func (p *tvPairing) adopted(email string, err error) {
	if p.stopped {
		p.w.deps.Logf("dancefitvn: pairing: confirmed after the code screen was left (adopted: %t)", err == nil)

		return
	}
	if err != nil {
		p.w.deps.Logf("dancefitvn: %v", err)
		if p.h.OnError != nil {
			p.h.OnError(err)
		}
		p.tv.Start(p.w.life)

		return
	}
	p.w.dropPairing(p)
	if p.h.OnSignedIn != nil {
		p.h.OnSignedIn(email)
	}
}
