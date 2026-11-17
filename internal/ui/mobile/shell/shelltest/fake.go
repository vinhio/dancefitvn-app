//go:build !tvos

package shelltest

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/api/mock"
	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DemoNow is the clock of a Fake until SetNow: Saturday 14 November 2026, 09:41
// at +07:00, the time of the prototype's screens and of the mock's server_time.
var DemoNow = time.Date(2026, time.November, 14, 9, 41, 0, 0, time.FixedZone("+07", 7*3600))

// Fake is env.Services over the mock API. Make one with New; every method is
// safe for concurrent use. The knobs (Set...) are for the test's goroutine.
type Fake struct {
	mu sync.Mutex

	fsys   fs.FS
	fails  map[string]failure
	tr     *mock.Transport
	client *api.Client
	mgr    *auth.Manager
	vault  *auth.MemoryVault

	online     bool
	watchers   []*watcher
	signedOut  []*watcher
	gates      map[string]chan struct{}
	restore    chan struct{}
	calls      map[string]int
	replaced   []string // replace_device_id of each auth.login ("" when it carried none)
	urls       []string
	last       string
	now        time.Time
	bio        sys.BiometryKind
	bioErr     error
	bioPrompts []sys.BioPrompt
	restoreErr error
	cfgEdit    func(*model.AppConfig)
	pb         env.Playback
	watched    map[string]bool // the lessons SetWatchedLocal named
	prefs      *FakePrefs
	dl         *FakeDownloads
	acct       *FakeAccount
	background bool     // SetForeground(false)
	fronts     []func() // WatchForeground

	// posted are the sign-outs found off the UI thread (a read's 401 after the
	// refresh, on the goroutine of ui.Go's work), waiting for RunPosted on the
	// UI thread, as the app's wire posts them (host.App.Enqueue); post is set by
	// the Rig (PostSignOuts). inline counts the SignOut calls under way, whose
	// callbacks run on their call.
	posted []func()
	post   bool
	inline int

	// avatars are the pictures Avatar answers, by URL (SetAvatarOf); avatarAny
	// the one for any other URL (SetAvatar).
	avatars   map[string]*scene.Image
	avatarAny *scene.Image
	// covers are the pictures Cover answers, by URL (SetCover); coverAsked the
	// addresses it was asked for.
	covers     map[string]*scene.Image
	coverAsked []string
}

// failure is the error answer Fail makes a route give.
type failure struct {
	status int
	file   string
}

// watcher is a registered callback with an id to stop it by.
type watcher struct {
	fn func(bool)
	so func(error)
}

var _ env.Services = (*Fake)(nil)

// transport is the Fake's api.Transport: it counts the calls, holds the ones a
// gate holds and fails them as a dead network does while the Fake is offline.
type transport struct {
	f     *Fake
	inner api.Transport
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// New returns a Fake over fsys, the mock API's root (MockFS). The session is
// empty (signed out), the network is up, the biometry is none, and a sign-in
// scenario is the mock's default.
func New(fsys fs.FS) (*Fake, error) {
	f := &Fake{
		fsys: fsys, fails: map[string]failure{},
		online: true, gates: map[string]chan struct{}{}, calls: map[string]int{},
		now: DemoNow, bio: sys.BiometryNone,
		prefs: NewFakePrefs(),
	}
	f.pb = NewFakePlayback(f.prefs)
	f.acct = newFakeAccount(f)
	tr, err := mock.New(fsys, mock.Options{Wait: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }})
	if err != nil {
		return nil, fmt.Errorf("shelltest: mock: %w", err)
	}
	f.tr = tr
	f.vault = auth.NewMemoryVault()
	f.mgr, err = auth.New(auth.Options{Vault: f.vault, OnSignedOut: f.fireSignedOut})
	if err != nil {
		return nil, fmt.Errorf("shelltest: auth: %w", err)
	}
	f.client, err = api.New(api.Options{
		Transport: &transport{f: f, inner: tr}, Version: "1.0.0", Platform: "ios", InstallID: "shelltest", Tokens: f.mgr,
		OnNetwork: func(bool) {},
	})
	if err != nil {
		return nil, fmt.Errorf("shelltest: api: %w", err)
	}
	f.mgr.Bind(f.client)
	if course, err := f.readCourse(fsys, demoCourse); err == nil {
		f.dl = newFakeDownloads(f, &course)
	} else {
		f.dl = newFakeDownloads(f, nil)
	}

	return f, nil
}

// MockFS returns the mock API's folder of the repository, found from the test's
// working directory; the test fails when there is none.
func MockFS(tb testing.TB) fs.FS {
	tb.Helper()
	dir, err := os.Getwd()
	if err != nil {
		tb.Fatalf("shelltest: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.DirFS(filepath.Join(dir, "mock", "dancefitvn-mock-api"))
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			tb.Fatal("shelltest: no go.mod above the test")
		}
		dir = parent
	}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// SetScenario picks the mock's scenario of a route ("auth.login",
// "invalid_credentials").
func (f *Fake) SetScenario(routeID, name string) error { return f.tr.SetScenario(routeID, name) }

// Fail makes the route answer with status and the body of a file of the mock's
// errors folder ("errors/server_error.json"), as a server in trouble does, where
// the mock has no scenario for it; an empty file puts the route back.
func (f *Fake) Fail(routeID string, status int, file string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if file == "" {
		delete(f.fails, routeID)

		return
	}
	f.fails[routeID] = failure{status: status, file: file}
}

// SetOnline turns the network on or off: while it is off every request fails as
// a dead network does (api.ErrOffline), and the watchers are told on this call,
// which the test makes on the UI thread.
func (f *Fake) SetOnline(on bool) {
	f.mu.Lock()
	if f.online == on {
		f.mu.Unlock()

		return
	}
	f.online = on
	ws := append([]*watcher(nil), f.watchers...)
	f.mu.Unlock()
	for _, w := range ws {
		w.fn(on)
	}
}

// Gate holds every request of the route (its id of routes.json) until the
// returned release is called or the request's context ends: the loading state of
// a screen stays on show.
func (f *Fake) Gate(routeID string) (release func()) {
	ch := make(chan struct{})
	f.mu.Lock()
	f.gates[routeID] = ch
	f.mu.Unlock()
	var once sync.Once

	return func() {
		once.Do(func() {
			f.mu.Lock()
			if f.gates[routeID] == ch {
				delete(f.gates, routeID)
			}
			f.mu.Unlock()
			close(ch)
		})
	}
}

// HoldRestore makes Restore wait until the release is called: a slow read of the
// vault.
func (f *Fake) HoldRestore() (release func()) {
	ch := make(chan struct{})
	f.mu.Lock()
	f.restore = ch
	f.mu.Unlock()
	var once sync.Once

	return func() { once.Do(func() { close(ch) }) }
}

// SeedSession puts the mock's session (auth/login.json) in the vault, as a
// launch finds it after a sign-in.
func (f *Fake) SeedSession() error {
	s, err := f.client.Login(context.Background(), "seed@example.com", "x", model.DeviceInfo{})
	if err != nil {
		return fmt.Errorf("shelltest: seed: %w", err)
	}
	f.vault.Put(s)

	return nil
}

// SetBiometry says what BioReady answers and how BioLogin ends: nil is a
// recognised user (the session of auth/login.json is adopted), else the error.
func (f *Fake) SetBiometry(kind sys.BiometryKind, result error) {
	f.mu.Lock()
	f.bio, f.bioErr = kind, result
	f.mu.Unlock()
}

// SetNow sets the device's clock.
func (f *Fake) SetNow(t time.Time) {
	f.mu.Lock()
	f.now = t
	f.mu.Unlock()
}

// SetLastEmail sets the email LastEmail answers.
func (f *Fake) SetLastEmail(s string) {
	f.mu.Lock()
	f.last = s
	f.mu.Unlock()
}

// SignOut ends the session as the server refusing the token does: the vault is
// cleared and the OnSignedOut callbacks run on this call (the UI thread).
func (f *Fake) SignOut(reason error) {
	f.mu.Lock()
	f.inline++
	f.mu.Unlock()
	defer func() {
		f.mu.Lock()
		f.inline--
		f.mu.Unlock()
	}()
	f.mgr.SignOut(reason)
}

// PostSignOuts makes the OnSignedOut callbacks of a sign-out found off the UI
// thread (auth.Manager.Handle in a read's work) wait for RunPosted, as the app's
// wire posts them to the UI thread; the Rig asks for it. Without it they run on
// the goroutine that found out.
func (f *Fake) PostSignOuts() {
	f.mu.Lock()
	f.post = true
	f.mu.Unlock()
}

// RunPosted runs the callbacks PostSignOuts held, in order, and returns how many
// ran. Call it on the UI thread (the Rig's Settle does).
func (f *Fake) RunPosted() int {
	n := 0
	for {
		f.mu.Lock()
		fns := f.posted
		f.posted = nil
		f.mu.Unlock()
		if len(fns) == 0 {
			return n
		}
		for _, fn := range fns {
			fn()
			n++
		}
	}
}

// Calls returns how many requests of the route reached the transport.
func (f *Fake) Calls(routeID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls[routeID]
}

// LoginReplaced returns the replace_device_id of each auth.login the Fake sent,
// in order; "" for a sign-in that carried none (K31). Never the password.
func (f *Fake) LoginReplaced() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.replaced...)
}

// OpenedURLs returns what OpenURL was asked to open, in order.
func (f *Fake) OpenedURLs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.urls...)
}

// BioPrompts returns the prompts BioLogin showed.
func (f *Fake) BioPrompts() []sys.BioPrompt {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]sys.BioPrompt(nil), f.bioPrompts...)
}

// Signed tells whether the vault holds a session.
func (f *Fake) Signed() bool { return f.vault.Has() }

// Restore reads the vault, after the hold of HoldRestore.
func (f *Fake) Restore(ctx context.Context) (bool, error) {
	f.mu.Lock()
	hold := f.restore
	f.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return false, fmt.Errorf("shelltest: restore: %w", ctx.Err())
		}
	}
	_, ok := f.mgr.Restore()

	return ok, nil
}

// SetConfigEdit changes the app config every Config call returns, after the mock
// answered: a feature switched off, a support line changed. nil puts it back.
func (f *Fake) SetConfigEdit(edit func(*model.AppConfig)) {
	f.mu.Lock()
	f.cfgEdit = edit
	f.mu.Unlock()
}

// Config is GET /app/config.
func (f *Fake) Config(ctx context.Context) (model.AppConfig, error) {
	c, err := f.client.Config(ctx)
	f.mgr.Handle(err)
	f.mu.Lock()
	edit := f.cfgEdit
	f.mu.Unlock()
	if err == nil && edit != nil {
		edit(&c)
	}

	return c, err
}

// Login is auth.login; the email is remembered. A Logout that began is waited
// for first, as the app's Login does.
func (f *Fake) Login(ctx context.Context, email, password string) error {
	return f.LoginReplacing(ctx, email, password, "")
}

// LoginReplacing is Login with the device of a device_limit answer to replace
// (K31); "" is Login.
func (f *Fake) LoginReplacing(ctx context.Context, email, password, replaceID string) error {
	if err := f.acct.waitLogout(ctx); err != nil {
		return err
	}
	if _, err := f.mgr.LoginReplacing(ctx, email, password, replaceID); err != nil {
		return err
	}
	f.SetLastEmail(email)

	return nil
}

// BioReady is the biometry set with SetBiometry.
func (f *Fake) BioReady() sys.BiometryKind {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.bio
}

// BioLogin ends as SetBiometry said.
func (f *Fake) BioLogin(ctx context.Context, p sys.BioPrompt) error {
	f.mu.Lock()
	f.bioPrompts = append(f.bioPrompts, p)
	err := f.bioErr
	f.mu.Unlock()
	if err != nil {
		return err
	}
	s, lerr := f.client.Login(ctx, "bio@example.com", "x", model.DeviceInfo{})
	if lerr != nil {
		return lerr
	}

	return f.mgr.Adopt(s)
}

// Home is GET /me/home.
func (f *Fake) Home(ctx context.Context) (model.Home, error) {
	h, err := f.client.Home(ctx)
	f.mgr.Handle(err)

	return h, err
}

// Courses is GET /me/courses, the first page (the mock answers whole).
func (f *Fake) Courses(ctx context.Context) (model.CoursePage, error) {
	p, err := f.client.MyCourses(ctx, "", 50)
	f.mgr.Handle(err)

	return p, err
}

// Course is GET /courses/{id}.
func (f *Fake) Course(ctx context.Context, id string) (model.Course, error) {
	c, err := f.client.Course(ctx, id)
	f.mgr.Handle(err)

	return c, err
}

// Online is the network set with SetOnline.
func (f *Fake) Online() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.online
}

// WatchOnline registers fn.
func (f *Fake) WatchOnline(fn func(online bool)) (stop func()) {
	w := &watcher{fn: fn}
	f.mu.Lock()
	f.watchers = append(f.watchers, w)
	f.mu.Unlock()

	return func() { f.drop(&f.watchers, w) }
}

// OnSignedOut registers fn.
func (f *Fake) OnSignedOut(fn func(reason error)) (stop func()) {
	w := &watcher{so: fn}
	f.mu.Lock()
	f.signedOut = append(f.signedOut, w)
	f.mu.Unlock()

	return func() { f.drop(&f.signedOut, w) }
}

// OpenURL records raw.
func (f *Fake) OpenURL(raw string) {
	f.mu.Lock()
	f.urls = append(f.urls, raw)
	f.mu.Unlock()
}

// LastEmail is the email of the last sign-in.
func (f *Fake) LastEmail() string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.last
}

// SetAvatar makes Avatar answer img for every URL that has no picture of its
// own (SetAvatarOf); nil takes it away. Until then Avatar has no picture (the
// mock's media are not served here).
func (f *Fake) SetAvatar(img *scene.Image) {
	f.mu.Lock()
	f.avatarAny = img
	f.mu.Unlock()
}

// SetAvatarOf makes Avatar answer img for url (a badge's, the user's); nil takes
// it away.
func (f *Fake) SetAvatarOf(url string, img *scene.Image) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if img == nil {
		delete(f.avatars, url)

		return
	}
	if f.avatars == nil {
		f.avatars = map[string]*scene.Image{}
	}
	f.avatars[url] = img
}

// Avatar answers the picture SetAvatarOf set for url, else SetAvatar's; none
// (core.ErrNotFound) otherwise.
func (f *Fake) Avatar(_ context.Context, url string) (*scene.Image, error) {
	f.mu.Lock()
	img, ok := f.avatars[url]
	if !ok {
		img = f.avatarAny
	}
	f.mu.Unlock()
	if img == nil {
		return nil, fmt.Errorf("shelltest: no avatar for %q: %w", url, core.ErrNotFound)
	}

	return img, nil
}

// SetCover makes Cover answer img for url; nil takes it away. Until then Cover
// answers core.ErrUnsupported, as the app does for the mock's covers (K14).
func (f *Fake) SetCover(url string, img *scene.Image) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if img == nil {
		delete(f.covers, url)

		return
	}
	if f.covers == nil {
		f.covers = map[string]*scene.Image{}
	}
	f.covers[url] = img
}

// Cover answers the picture SetCover set for url; none (core.ErrUnsupported)
// otherwise.
func (f *Fake) Cover(_ context.Context, url string) (*scene.Image, error) {
	f.mu.Lock()
	f.coverAsked = append(f.coverAsked, url)
	img := f.covers[url]
	f.mu.Unlock()
	if img == nil {
		return nil, fmt.Errorf("shelltest: no cover for %q: %w", url, core.ErrUnsupported)
	}

	return img, nil
}

// CoverCalls returns the addresses Cover was asked for, oldest first.
func (f *Fake) CoverCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.coverAsked...)
}

// Now is the clock set with SetNow.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.now
}

// Do counts the request, then holds it, fails it or lets the mock answer.
func (t *transport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	t.f.mu.Lock()
	t.f.calls[req.RouteID]++
	if req.RouteID == "auth.login" {
		var lr model.LoginRequest
		_ = json.Unmarshal(req.Body, &lr)
		t.f.replaced = append(t.f.replaced, lr.ReplaceDeviceID)
	}
	gate := t.f.gates[req.RouteID]
	online := t.f.online
	fail, failed := t.f.fails[req.RouteID]
	t.f.mu.Unlock()
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
			return nil, fmt.Errorf("shelltest: gate %s: %w", req.RouteID, ctx.Err())
		}
	}
	if !online {
		return nil, &url.Error{Op: "Get", URL: req.Path, Err: &net.OpError{Op: "dial", Err: errors.New("network is unreachable")}}
	}

	if failed {
		body, err := fs.ReadFile(t.f.fsys, fail.file)
		if err != nil {
			return nil, fmt.Errorf("shelltest: read %s: %w", fail.file, err)
		}

		return &api.Response{Status: fail.status, Header: http.Header{}, Body: body}, nil
	}

	return t.inner.Do(ctx, req)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// fireSignedOut tells the OnSignedOut callbacks: at once on the call of
// SignOut or without PostSignOuts, else later, by RunPosted on the UI thread.
func (f *Fake) fireSignedOut(reason error) {
	f.mu.Lock()
	if f.post && f.inline == 0 {
		f.posted = append(f.posted, func() { f.tellSignedOut(reason) })
		f.mu.Unlock()

		return
	}
	f.mu.Unlock()
	f.tellSignedOut(reason)
}

// tellSignedOut calls the OnSignedOut callbacks.
func (f *Fake) tellSignedOut(reason error) {
	f.mu.Lock()
	ws := append([]*watcher(nil), f.signedOut...)
	f.mu.Unlock()
	for _, w := range ws {
		w.so(reason)
	}
}

// drop removes w from the list.
func (f *Fake) drop(list *[]*watcher, w *watcher) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, x := range *list {
		if x == w {
			*list = append((*list)[:i:i], (*list)[i+1:]...)

			return
		}
	}
}

// readCourse reads a course of the mock folder for the demo downloads.
func (f *Fake) readCourse(fsys fs.FS, id string) (model.Course, error) {
	var c model.Course
	raw, err := fs.ReadFile(fsys, "courses/"+id+".json")
	if err != nil {
		return c, fmt.Errorf("shelltest: read course: %w", err)
	}
	if err := jsonUnmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("shelltest: decode course: %w", err)
	}

	return c, nil
}
