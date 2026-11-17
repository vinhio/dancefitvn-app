//go:build !tvos

// The app's side of the phone screens' contract (env.Services). The
// pattern is jplayer-mobile/internal/app/actionport.go and viewport.go at
// 49de3c2: the screens see interfaces that import none of the app's
// packages, and the app adapts its parts to them here.

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
	"sync"
	"sync/atomic"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/components"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The files of the phone's data folder (sys.DataDir), beside wire.go's.
const (
	// stateFolder holds the small documents of the device (store.Dir).
	stateFolder = "state"
	// audioFolder holds the downloaded MP3s (download.Options.Dir).
	audioFolder = "audio"
)

// mobileDeps is what the phone's services are built with: the system's
// parts, which a test replaces.
type mobileDeps struct {
	// Mock is the mock API's folder; nil (a live build) talks to the live
	// API (apimode.go).
	Mock fs.FS
	// API decides where the requests go (apimode.go); nil is planAPI over
	// the build: the mock, the live API of a -tags live build, or a
	// developer's DFV_API and DFV_LIVE_ROUTES.
	API apiPlanFunc
	// Enqueue runs fn on the UI thread later (host.App.Enqueue).
	Enqueue func(fn func()) bool
	// Secrets opens the store of the session's tokens (sys.OpenSecrets).
	Secrets func() (auth.SecretStore, error)
	// InstallID reads or makes the install id (sys.InstallID).
	InstallID func() (string, error)
	// DataDir and CacheDir are sys.DataDir and sys.CacheDir.
	DataDir, CacheDir func() (string, error)
	// Bio is the device's biometric check (sys.NewBiometric).
	Bio sys.Biometrics
	// OpenURL hands a URL to the system (sys.OpenURL).
	OpenURL func(c platform.Canvas, d sys.Dispatch, raw string, done func(error)) error
	// Platform is "ios" or "android", sent as X-Platform.
	Platform string
	// MockWait is the mock's delay (mock.Options.Wait); nil is a real timer.
	MockWait func(ctx context.Context, d time.Duration) error
	// Clock times the network probe and the downloads; nil is
	// clock.System on Enqueue.
	Clock clock.Clock
	// Logf logs a line; nil is log.Printf. It never gets tokens.
	Logf func(format string, args ...any)
	// NewMedia makes the engine and the audio session; nil is the
	// system's (newPlatformMedia: avf, amp, the fake elsewhere).
	NewMedia newMediaFunc
	// MediaLate makes NewMedia's media after the first frame, as the
	// iPhone's are (a test of lateMedia's wiring).
	MediaLate bool
	// Awake keeps the screen on while a video plays (sys.KeepAwake);
	// nil keeps nothing on.
	Awake awakeSink
	// NowPlaying is the system's Now Playing (bridge/system/nowplaying):
	// the lock screen, the media notification, the remote commands; nil
	// shows nothing.
	NowPlaying npSink
	// OnLifecycle adds a listener of the app's lifecycle, told on the UI
	// thread (host.App.OnLifecycle); nil hears none.
	OnLifecycle func(fn func(platform.LifecycleState)) (remove func())
	// AudioDir makes the folder of the downloads (sys.PrepareAudioDir),
	// off the UI thread; nil is <data>/audio.
	AudioDir func() (string, error)
	// FreeSpace reads the free and the total space of the volume of a
	// folder (sys.FreeSpace), off the UI thread; nil knows none.
	FreeSpace func(path string) (free, total int64, err error)
	// Network starts watching the device's network, its changes told
	// through dispatch (watchNetwork); nil knows none.
	Network func(dispatch func(func())) netWatch
	// BackgroundTask asks the system for time to finish the downloads in
	// the background (sys.BeginBackgroundTask, F5a); nil asks none.
	BackgroundTask func(name string, expired func()) (end func(), err error)
	// PickImage shows the system's picker of one picture (sys.PickImage,
	// F2); nil has none (PickPhoto fails with core.ErrUnsupported).
	PickImage pickImageFunc
}

// wireKnobs are what a shot state sets before the shell starts
// (states_mobile.go); the zero value is the app as it ships.
type wireKnobs struct {
	// bio is the biometry BioReady reports, BiometryUnknown for the
	// device's.
	bio sys.BiometryKind
	// now pins the clock of the screens; zero is the device's.
	now time.Time
	// demo shows the prototype's downloads (`S.dl`) instead of the files
	// on the device.
	demo *demoDownloads
	// offline makes every request fail as a dead network does.
	offline bool
	// prepare runs once the parts are made, before the session is read:
	// the state's way to it through the mock.
	prepare func(ctx context.Context, w *mobileWire) error
	// media makes the engine in place of the system's: the fake of a
	// shot state (fakeMediaOf); nil is the system's.
	media newMediaFunc
	// dlKBps reads the downloads' bodies at that many KiB a second
	// (DFV_DL_KBPS); 0 at the network's speed.
	dlKBps int
	// badSHA makes the sha256 of every audio file wrong (DFV_BADSHA).
	badSHA bool
	// noPrivacy empties links.privacy and links.terms of the app config, so that
	// the account has the rows of the prototype's M15 and the shot compares with
	// it (K32, K34).
	noPrivacy bool
	// memOverlay keeps the mock's overlay in memory, empty at launch: a
	// shot state starts from the mock's data whatever a run before changed,
	// and leaves nothing behind for the app as it ships.
	memOverlay bool
}

// mobileWire is env.Services over the app's parts: the API client on the
// mock transport, the session's Manager and vault, the cache, the
// downloads and the system's services. The parts are made by the first
// blocking call (Restore or Config, which the boot makes off the UI
// thread), as the vault's first read can take a second on Android.
type mobileWire struct {
	deps     mobileDeps
	knobs    wireKnobs
	dispatch sys.Dispatch
	clk      clock.Clock
	online   *store.Online
	images   components.ImageCache
	// covers loads the covers of the courses (covers.go); disk is its folder.
	covers *coverLoader
	disk   *store.Images
	http   *http.Client
	// ui is the clock of the playback: the view's timers once attached
	// (uiClock), so that a Tester's Elapse drives the engine and the
	// saves. UI thread only.
	ui *uiClock
	// pb is the window's playback (player_mobile.go).
	pb *playback
	// dls are the downloads of the user as the screens see them
	// (downloads_mobile.go); net is the device's network.
	dls      *wireDownloads
	net      netWatch
	bound    bool   // ui is bound to the view (UI thread)
	stopLife func() // ends the lifecycle listener
	bg       sync.WaitGroup

	once sync.Once
	err  error
	// Made by setup, read only after it.
	cache *store.Cache
	bio   *auth.BioVault
	mgr   *auth.Manager
	mock  inProcMock
	// overlay keeps what the user changed of the mock's data (05 section
	// 4): devTransport -> overlayTags -> overlay -> plainMedia -> mock, the
	// hybrid's mock side (apimode.go); nil when the live API answers all.
	overlay mockOverlay
	tr      *devTransport
	client  *api.Client
	prefs   *store.Dir
	data    string
	probe   *onlineProbe
	ready   atomic.Bool
	// built is set once build made the parts: the network's changes act
	// on them from then.
	built atomic.Bool
	// readBack is set when the API answers a write with the mock's fixed
	// files (the mock in the process, or a developer's server): what the
	// write changed is read back (UpdateMe, UpdateCoach). Made by setup.
	readBack bool
	// noMock is set when the state's prepare needed the mock in the
	// process and the API is a server (errNoMock): its runner stops.
	noMock atomic.Bool
	// wifiOnly is "Chỉ tải qua Wi‑Fi", read by the downloads' rule on any
	// goroutine.
	wifiOnly atomic.Bool
	// wasAway is set by a Background and ends with the next Active: a return to the
	// front (UI thread).
	wasAway bool
	// sessGen counts the ends of sessions (clearCache): a save of the
	// position that began in one session and failed after its end is not
	// queued for the next (wireSender).
	sessGen atomic.Uint64
	// apiReached is whether the client's last request reached the server
	// (its OnNetwork); true at first.
	apiReached atomic.Bool
	// installID is sent as the progress' device_id.
	installID string
	// Set by setup, read by the playback (any goroutine).
	prefStore  atomic.Pointer[store.PrefsStore]
	prefsFresh atomic.Bool // no preferences stored at launch
	queue      atomic.Pointer[progress.Queue]
	appConfig  atomic.Pointer[model.AppConfig]
	// tracker is the playback's progress.Tracker once made (wireSaver), for
	// Logout to wait for its saves on another goroutine.
	tracker atomic.Pointer[progress.Tracker]

	mu       sync.Mutex
	sess     context.Context //nolint:containedctx // the session's lifetime: canceled at sign-out
	endSess  context.CancelFunc
	watchers []*watch
	last     string
	dl       *download.Manager
	dlUser   string
	audio    string // the folder of the downloads, set by build
	view     *ui.View
	closed   bool
	// watched are the lessons watched in this run (WatchedLocal).
	watched map[string]bool
	// loggingOut is closed when the sign-out under way ends; nil when none
	// is (Logout, waitLogout).
	loggingOut chan struct{}
}

var (
	_ env.Services     = (*mobileWire)(nil)
	_ env.Foregrounder = (*mobileWire)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newMobileWire returns the services over deps, with the knobs of a shot
// state (the zero knobs as the app ships). Nothing is opened yet but the
// playback (player_mobile.go): the player and its engine, made late on
// the iPhone. UI thread.
func newMobileWire(deps mobileDeps, knobs wireKnobs) *mobileWire {
	if deps.Logf == nil {
		deps.Logf = log.Printf
	}
	w := &mobileWire{deps: deps, knobs: knobs, online: store.NewOnline()}
	w.dispatch = func(fn func()) { deps.Enqueue(fn) }
	w.clk = deps.Clock
	if w.clk == nil {
		w.clk = clock.System(w.dispatch)
	}
	tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the stdlib default
	tr.ResponseHeaderTimeout = 15 * time.Second
	w.http = &http.Client{Transport: tr, Timeout: 30 * time.Second}
	w.sess, w.endSess = context.WithCancel(context.Background())
	wall := time.Now
	if deps.Clock != nil {
		wall = deps.Clock.Now
	}
	w.ui = newUIClock(wall)
	w.wifiOnly.Store(store.DefaultPrefs(true).WiFiOnly)
	w.apiReached.Store(true)
	w.dls = newWireDownloads(w)
	w.pb = newPlayback(w)
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
func (w *mobileWire) Restore(ctx context.Context) (bool, error) {
	if err := w.setup(ctx); err != nil {
		return false, err
	}
	s, ok := w.mgr.Restore()
	if ok {
		w.signedIn(s, "")
	}
	w.mgr.HasBio() // read the slot now, off the UI thread, for BioReady
	w.ready.Store(true)

	return ok, nil
}

// Config is GET /app/config.
func (w *mobileWire) Config(ctx context.Context) (model.AppConfig, error) {
	if err := w.setup(ctx); err != nil {
		return model.AppConfig{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	c, err := w.client.Config(ctx)
	w.mgr.Handle(err)
	if err == nil {
		if w.knobs.noPrivacy {
			c.Links.Privacy, c.Links.Terms = "", ""
		}
		w.configured(c)
	}

	return c, err
}

// Login signs in with the password; the email is remembered. A sign-out
// under way is waited for first.
func (w *mobileWire) Login(ctx context.Context, email, password string) error {
	return w.LoginReplacing(ctx, email, password, "")
}

// LoginReplacing is Login that asks the server to revoke the device replaceID
// of a device_limit answer first (K31); "" is Login.
func (w *mobileWire) LoginReplacing(ctx context.Context, email, password, replaceID string) error {
	if err := w.setup(ctx); err != nil {
		return err
	}
	if err := w.waitLogout(ctx); err != nil {
		return err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	s, err := w.mgr.LoginReplacing(ctx, email, password, replaceID)
	if err != nil {
		w.mgr.Handle(err)

		return err
	}
	w.signedIn(s, email)

	return nil
}

// BioReady is the biometry the sign-in offers (K13): one the device has
// enrolled, and a copy of this app's session kept for it.
func (w *mobileWire) BioReady() sys.BiometryKind {
	if !w.ready.Load() {
		return sys.BiometryNone
	}
	k := w.knobs.bio
	if k == sys.BiometryUnknown {
		k = w.deps.Bio.Kind()
	}
	if !k.Usable() || !w.mgr.HasBio() {
		return sys.BiometryNone
	}

	return k
}

// bioUsable tells whether the device can sign in with its biometry (the
// BioVault keeps its copy then): the kind a shot state forced, or the
// device's.
func (w *mobileWire) bioUsable() bool {
	if w.knobs.bio != sys.BiometryUnknown {
		return w.knobs.bio.Usable()
	}

	return w.deps.Bio.Available()
}

// BioLogin shows the system's sheet, then signs in with the kept copy, a
// sign-out under way waited for first.
func (w *mobileWire) BioLogin(ctx context.Context, p sys.BioPrompt) error {
	if err := w.setup(ctx); err != nil {
		return err
	}
	if err := w.waitLogout(ctx); err != nil {
		return err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	s, err := w.mgr.BioLogin(ctx, func(ctx context.Context) error { return w.deps.Bio.Authenticate(ctx, p) })
	if err != nil {
		return err
	}
	w.signedIn(s, s.User.Email)

	return nil
}

// Home is GET /me/home.
func (w *mobileWire) Home(ctx context.Context) (model.Home, error) {
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
func (w *mobileWire) Courses(ctx context.Context) (model.CoursePage, error) {
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
func (w *mobileWire) Course(ctx context.Context, id string) (model.Course, error) {
	if err := w.setup(ctx); err != nil {
		return model.Course{}, err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	c, err := w.client.Course(ctx, id)
	w.mgr.Handle(err)

	return c, err
}

// Downloads is the audio files of the user on the device, and the
// downloads under way (or the prototype's demo in a shot state).
func (w *mobileWire) Downloads() env.DownloadManager {
	if w.knobs.demo != nil {
		return w.knobs.demo
	}

	return w.dls
}

// Foreground is env.Foregrounder (Q-P5-4): false while the app is in the
// background (host.App.OnLifecycle), true otherwise, before any lifecycle
// too. UI thread; cheap.
func (w *mobileWire) Foreground() bool { return w.dls == nil || !w.dls.away }

// Online tells whether the app thinks it is online.
func (w *mobileWire) Online() bool { return w.online.Online() }

// WatchOnline calls fn on the UI thread at each change of Online.
func (w *mobileWire) WatchOnline(fn func(online bool)) (stop func()) {
	return w.addWatch(&watch{online: fn})
}

// WatchForeground calls fn on the UI thread when the app comes back to the front
// after the background (env.ForegroundWatcher).
func (w *mobileWire) WatchForeground(fn func()) (stop func()) {
	return w.addWatch(&watch{front: fn})
}

// OnSignedOut calls fn on the UI thread when the server ends the session.
func (w *mobileWire) OnSignedOut(fn func(reason error)) (stop func()) {
	return w.addWatch(&watch{out: fn})
}

// OpenURL hands raw to the system's browser (F7); a failure is logged.
// UI thread only.
func (w *mobileWire) OpenURL(raw string) {
	w.mu.Lock()
	v := w.view
	w.mu.Unlock()
	if v == nil || w.deps.OpenURL == nil {
		w.deps.Logf("dancefitvn: open URL: no window: %v", core.ErrUnsupported)

		return
	}
	err := w.deps.OpenURL(v.Canvas(), w.dispatch, raw, func(err error) {
		if err != nil {
			w.deps.Logf("dancefitvn: open URL: %v", err)
		}
	})
	if err != nil {
		w.deps.Logf("dancefitvn: open URL: %v", err)
	}
}

// LastEmail is the email of the last sign-in on this device.
func (w *mobileWire) LastEmail() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.last
}

// Avatar loads the picture at url, once per URL (components.ImageCache).
func (w *mobileWire) Avatar(ctx context.Context, url string) (*scene.Image, error) {
	if err := w.setup(ctx); err != nil {
		return nil, err
	}

	return w.images.Loader(url, func(ctx context.Context) (*scene.Image, error) { return w.fetchImage(ctx, url) })(ctx)
}

// Cover loads the cover of a course at url (K14): the real picture in the
// live mode, from the disk cache or the network and reduced to the size it
// is drawn at; core.ErrUnsupported for the mock's, which keeps the vector
// poster. Blocks.
func (w *mobileWire) Cover(ctx context.Context, url string) (*scene.Image, error) {
	if err := w.setup(ctx); err != nil {
		return nil, err
	}

	return w.covers.Load(ctx, url)
}

// Now is the device's clock, or the one a shot state pinned.
func (w *mobileWire) Now() time.Time {
	if !w.knobs.now.IsZero() {
		return w.knobs.now
	}

	return time.Now()
}

// attach gives the services the view, whose canvas OpenURL acts for and
// whose timers the playback's clock runs on (bound once). UI thread.
func (w *mobileWire) attach(v *ui.View) {
	w.mu.Lock()
	w.view = v
	w.mu.Unlock()
	if !w.bound && v != nil {
		w.bound = true
		w.ui.Bind(v)
	}
}

// tap presses and releases a finger at the middle of wd, through the
// view's events as a touch on the screen arrives: what a developer's state
// does where a script cannot touch the simulator. UI thread only.
func (w *mobileWire) tap(wd ui.Widget) {
	w.mu.Lock()
	v := w.view
	w.mu.Unlock()
	if v == nil {
		return
	}
	c := wd.Node().WindowFrame().Center()
	for _, k := range [...]platform.EventKind{platform.EventPointerDown, platform.EventPointerUp} {
		v.HandleEvent(platform.Event{Kind: k, X: c.X, Y: c.Y, Button: platform.ButtonPrimary, Count: 1, Pointer: platform.PointerTouch})
	}
}

// close ends the session's requests and closes the parts, at the window's
// close: the playback first (its last position is told to the saves),
// then the goroutines of the flushes and the saves of the preferences are
// waited for. A setup under way is waited for; one not started never
// will. UI thread.
func (w *mobileWire) close() {
	if w.stopLife != nil {
		w.stopLife()
		w.stopLife = nil
	}
	w.pb.close()
	if w.net != nil {
		w.net.Close()
	}
	w.dls.away = false
	w.dls.background() // a background task held is given back
	w.once.Do(func() { w.err = errWireClosed })
	w.mu.Lock()
	w.closed = true
	w.endSess()
	dl := w.dl
	w.dl, w.view, w.watchers = nil, nil, nil
	w.mu.Unlock()
	w.bg.Wait()
	if dl != nil {
		dl.Close()
	}
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
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// setup makes the parts once: the secrets, the install id, the cache,
// the vault with its biometric copy, the Manager, the mock transport with
// its overlay and the client, then runs the state's prepare. Off the UI
// thread.
func (w *mobileWire) setup(ctx context.Context) error {
	w.once.Do(func() { w.err = w.build(ctx) })

	return w.err
}

// build is setup's work.
func (w *mobileWire) build(ctx context.Context) error {
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
	w.data, w.installID = data, id
	w.prefs = store.NewDir(filepath.Join(data, stateFolder))
	w.last = w.readLastEmail()
	w.cache = store.OpenCache(filepath.Join(cacheDir, apiFolder), 0)
	w.disk = store.OpenImages(filepath.Join(cacheDir, coversFolder), 0)
	w.bio = auth.NewBioVault(auth.NewSecretVault(secrets, id), auth.NewSecretVault(secrets, id+".bio"), w.bioUsable)
	w.mgr, err = auth.New(auth.Options{
		Vault:       w.bio,
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
	w.readBack = parts.overlay != nil || plan.tags
	d.Logf("dancefitvn: API: %s", plan)
	w.tr.setOffline(w.knobs.offline)
	w.prepareAudioDir(data)
	w.client, err = api.New(api.Options{
		Transport: w.tr, Version: appVersion, Platform: d.Platform, InstallID: id,
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
	if w.netKind() == sys.NetworkNone { // offline from the start (03 section 2)
		w.tr.setNetDown(true)
		w.online.Set(false)
	}

	return nil
}

// clientNetwork is the client's OnNetwork: what its requests learnt of the
// network, kept for the probe, and the app's online state.
func (w *mobileWire) clientNetwork(online bool) {
	w.apiReached.Store(online)
	w.online.Set(online)
}

// probeServer is the probe of the network (GET /app/config): it succeeds
// only when the server answered. An answer from the cache (the client
// serves one when the network fails) is a failure, and so is nothing at
// all. On success the app is online: the client tells only its own
// changes, and a network the device lost (netChanged) is not one it saw
// go.
func (w *mobileWire) probeServer(ctx context.Context) error {
	if _, err := w.client.Config(ctx); err != nil {
		return err
	}
	if !w.apiReached.Load() {
		return errProbeCached
	}
	w.online.Set(true)

	return nil
}

// scoped returns ctx ended also by the end of the session: a sign-out
// cancels the requests of the user before, so that none lands in the
// cache of the next (P1.11).
func (w *mobileWire) scoped(ctx context.Context) (context.Context, context.CancelFunc) {
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
// then the cache emptied.
func (w *mobileWire) clearCache() {
	w.mu.Lock()
	w.endSess()
	if !w.closed {
		w.sess, w.endSess = context.WithCancel(context.Background())
	}
	w.mu.Unlock()
	w.sessGen.Add(1)
	w.cache.Clear()
	w.forgetWatched()
	w.closeDownloads()
	// The lesson playing is left first (its last position saved, or
	// queued offline), then the queue emptied: no position of this user is
	// queued after it, to be sent with the next user's token.
	w.dispatch(func() {
		w.pb.signedOut()
		w.clearQueue()
	})
}

// signedIn takes a session that began (or was read at launch): the user's
// downloads, and the email to remember when the user typed one.
func (w *mobileWire) signedIn(s model.Session, email string) {
	if email != "" {
		w.rememberEmail(email)
	} else if w.LastEmail() == "" && s.User.Email != "" {
		w.rememberEmail(s.User.Email)
	}
	w.useDownloads(s.User.ID)
}

// signedOut is the Manager's OnSignedOut, on the goroutine that found
// out: the watchers are told on the UI thread.
func (w *mobileWire) signedOut(reason error) {
	for _, wt := range w.watchList() {
		if wt.out != nil {
			fn := wt.out
			w.dispatch(func() { fn(reason) })
		}
	}
}

// networkChanged is Online's subscriber: the probe starts or stops, the
// downloads resume, the positions saved offline are sent, and the
// watchers are told on the UI thread.
func (w *mobileWire) networkChanged(on bool) {
	if on {
		w.probe.stop()
		if m := w.downloads(); m != nil {
			m.ResumeAll()
		}
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
func (w *mobileWire) addWatch(wt *watch) func() {
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
func (w *mobileWire) watchList() []*watch {
	w.mu.Lock()
	defer w.mu.Unlock()

	return slices.Clone(w.watchers)
}

// readLastEmail reads the remembered email; "" when there is none.
func (w *mobileWire) readLastEmail() string {
	raw, err := w.prefs.Load(deviceKey)
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

// rememberEmail keeps email as the last one, on disk too.
func (w *mobileWire) rememberEmail(email string) {
	w.mu.Lock()
	same := w.last == email
	w.last = email
	w.mu.Unlock()
	if same {
		return
	}
	raw, err := json.Marshal(deviceDoc{LastEmail: email})
	if err == nil {
		err = w.prefs.Save(deviceKey, raw)
	}
	if err != nil {
		w.deps.Logf("dancefitvn: device document: %v", err)
	}
}

// fetchImage reads and decodes the picture at url.
func (w *mobileWire) fetchImage(ctx context.Context, url string) (*scene.Image, error) {
	data, err := w.fetchBytes(ctx, url, maxImageBytes)
	if err != nil {
		return nil, err
	}
	img, err := components.DecodeImage(data)
	if err != nil {
		return nil, fmt.Errorf("app: image: %w", err)
	}

	return img, nil
}

// fetchBytes reads limit bytes at most of the file at url; the mock's
// media server is started first, its URLs pointing at it. Blocks.
func (w *mobileWire) fetchBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	if w.mock != nil {
		if err := w.mock.Media().Ensure(); err != nil {
			return nil, fmt.Errorf("app: image: %w", err)
		}
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
