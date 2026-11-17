//go:build tvos

package tvtest

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/ui/tv/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Services is a fake env.Services: it answers from the fields a test sets,
// records what the screens did, and fires the callbacks it was given when a
// test asks (SignOut, SetOnline). Nothing blocks. It is safe for the tests'
// goroutines (ui.Go work functions run on others).
type Services struct {
	// Signed is what Restore answers; User the signed-in user.
	Signed bool
	User   model.User
	// HomeData, CoursesData, CourseData, MeData and ConfigData are the answers
	// of the reads; the *Err fields fail them.
	HomeData    model.Home
	CoursesData model.CoursePage
	CourseData  model.Course
	MeData      model.Me
	ConfigData  model.AppConfig
	ReadErr     error
	// OnlineNow is what Online answers.
	OnlineNow bool
	// PB and PrefsV are what Playback and Prefs return; nil for none.
	PB     env.Playback
	PrefsV env.Prefs
	// Ver is Version; Clock is Now, the zero time reading as a fixed day.
	Ver   string
	Clock time.Time

	mu         sync.Mutex
	pairs      []*Pairing
	logouts    int
	signedOut  []func(error)
	onlineFns  map[int]func(bool)
	onlineSeq  int
	watchedIDs map[string]bool
	avatars    []string
	covers     map[string]*scene.Image
	coverAsked []string
	reads      map[string]int
	gate       chan struct{}
	logoutHold chan struct{}
	logoutDone chan struct{}
}

var _ env.Services = (*Services)(nil)

// Pairing is a fake env.Pairing: the hooks the screen gave, a count of Stops
// and the state a test pushed.
type Pairing struct {
	Hooks   env.PairHooks
	Stopped bool
	left    time.Duration
	state   pairing.State
	have    bool
}

var _ env.Pairing = (*Pairing)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewServices returns a signed-in, online fake with version "0.1.0".
func NewServices() *Services {
	return &Services{Signed: true, OnlineNow: true, Ver: "0.1.0"}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Restore answers Signed.
func (s *Services) Restore(context.Context) (bool, error) { return s.Signed, nil }

// SessionUser answers User when Signed.
func (s *Services) SessionUser() (model.User, bool) { return s.User, s.Signed }

// OnSignedOut keeps fn for SignOut.
func (s *Services) OnSignedOut(fn func(error)) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signedOut = append(s.signedOut, fn)

	return func() {}
}

// SignOut calls the OnSignedOut functions with reason, on the caller's thread.
func (s *Services) SignOut(reason error) {
	s.mu.Lock()
	fns := append([]func(error){}, s.signedOut...)
	s.mu.Unlock()
	for _, fn := range fns {
		fn(reason)
	}
}

// Pair records a new fake Pairing and returns it.
func (s *Services) Pair(h env.PairHooks) env.Pairing {
	p := &Pairing{Hooks: h}
	s.mu.Lock()
	s.pairs = append(s.pairs, p)
	s.mu.Unlock()

	return p
}

// Pairings returns the pairings the screens started, oldest first.
func (s *Services) Pairings() []*Pairing {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]*Pairing(nil), s.pairs...)
}

// LogoutTV counts the call, waits for HoldLogout's release when a test holds it
// (or for ctx), answers ReadErr and then signals LogoutDone.
func (s *Services) LogoutTV(ctx context.Context) error {
	s.mu.Lock()
	s.logouts++
	hold := s.logoutHold
	done := s.logoutDoneLocked()
	s.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	done <- struct{}{}

	return s.ReadErr
}

// HoldLogout keeps the answer of LogoutTV back, as a slow server does, until the
// returned release is called (once). The call still counts when it starts.
func (s *Services) HoldLogout() (release func()) {
	g := make(chan struct{})
	s.mu.Lock()
	s.logoutHold = g
	s.mu.Unlock()

	return func() { close(g) }
}

// LogoutDone receives once for each LogoutTV that has finished.
func (s *Services) LogoutDone() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.logoutDoneLocked()
}

// logoutDoneLocked returns the channel LogoutDone reads, made on first use; s.mu is held.
func (s *Services) logoutDoneLocked() chan struct{} {
	if s.logoutDone == nil {
		s.logoutDone = make(chan struct{}, 8)
	}

	return s.logoutDone
}

// Logouts returns how many times LogoutTV was called.
func (s *Services) Logouts() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.logouts
}

// Home answers HomeData.
func (s *Services) Home(ctx context.Context) (model.Home, error) {
	s.count(ctx, "Home")

	return s.HomeData, s.ReadErr
}

// Courses answers CoursesData.
func (s *Services) Courses(ctx context.Context) (model.CoursePage, error) {
	s.count(ctx, "Courses")

	return s.CoursesData, s.ReadErr
}

// Course answers CourseData.
func (s *Services) Course(ctx context.Context, _ string) (model.Course, error) {
	s.count(ctx, "Course")

	return s.CourseData, s.ReadErr
}

// Me answers MeData.
func (s *Services) Me(ctx context.Context) (model.Me, error) {
	s.count(ctx, "Me")

	return s.MeData, s.ReadErr
}

// Config answers ConfigData.
func (s *Services) Config(ctx context.Context) (model.AppConfig, error) {
	s.count(ctx, "Config")

	return s.ConfigData, s.ReadErr
}

// Hold keeps the answers of the reads back, as a slow network does, until the
// returned release is called (once). The reads still count when they start.
func (s *Services) Hold() (release func()) {
	g := make(chan struct{})
	s.mu.Lock()
	s.gate = g
	s.mu.Unlock()

	return func() { close(g) }
}

// Reads returns how many times the read named after the method (Home, Courses,
// Course, Me, Config) was asked for.
func (s *Services) Reads(name string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reads[name]
}

// count notes a read.
func (s *Services) count(ctx context.Context, name string) {
	s.mu.Lock()
	if s.reads == nil {
		s.reads = map[string]int{}
	}
	s.reads[name]++
	gate := s.gate
	s.mu.Unlock()
	if gate != nil { // a test holds the answers back
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
}

// Avatar records the address and answers no picture.
func (s *Services) Avatar(_ context.Context, url string) (*scene.Image, error) {
	s.mu.Lock()
	s.avatars = append(s.avatars, url)
	s.mu.Unlock()

	return nil, s.ReadErr
}

// SetCover makes Cover answer img for url; nil takes it away. Until then Cover
// answers core.ErrUnsupported, as the app does for the mock's covers (K14).
func (s *Services) SetCover(url string, img *scene.Image) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if img == nil {
		delete(s.covers, url)

		return
	}
	if s.covers == nil {
		s.covers = map[string]*scene.Image{}
	}
	s.covers[url] = img
}

// Cover answers the picture SetCover set for url; none (core.ErrUnsupported)
// otherwise.
func (s *Services) Cover(_ context.Context, url string) (*scene.Image, error) {
	s.mu.Lock()
	s.coverAsked = append(s.coverAsked, url)
	img := s.covers[url]
	s.mu.Unlock()
	if img == nil {
		return nil, fmt.Errorf("tvtest: no cover for %q: %w", url, core.ErrUnsupported)
	}

	return img, nil
}

// CoverCalls returns the addresses Cover was asked for, oldest first.
func (s *Services) CoverCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.coverAsked...)
}

// AvatarCalls returns the addresses Avatar was asked for, oldest first.
func (s *Services) AvatarCalls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]string(nil), s.avatars...)
}

// WatchedLocal tells whether MarkWatched was called for the lesson.
func (s *Services) WatchedLocal(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.watchedIDs[id]
}

// MarkWatched makes WatchedLocal true for the lesson.
func (s *Services) MarkWatched(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.watchedIDs == nil {
		s.watchedIDs = map[string]bool{}
	}
	s.watchedIDs[id] = true
}

// Online answers OnlineNow.
func (s *Services) Online() bool { return s.OnlineNow }

// WatchOnline keeps fn for SetOnline until the returned stop is called, as the
// app's wire does: a screen that was left must not hear the network again.
func (s *Services) WatchOnline(fn func(bool)) func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.onlineFns == nil {
		s.onlineFns = map[int]func(bool){}
	}
	s.onlineSeq++
	id := s.onlineSeq
	s.onlineFns[id] = fn

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.onlineFns, id)
	}
}

// SetOnline changes Online and calls the WatchOnline functions.
func (s *Services) SetOnline(on bool) {
	s.mu.Lock()
	s.OnlineNow = on
	ids := make([]int, 0, len(s.onlineFns))
	for id := range s.onlineFns {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	fns := make([]func(bool), 0, len(ids))
	for _, id := range ids {
		fns = append(fns, s.onlineFns[id])
	}
	s.mu.Unlock()
	for _, fn := range fns {
		fn(on)
	}
}

// Playback answers PB.
func (s *Services) Playback() env.Playback { return s.PB }

// Prefs answers PrefsV.
func (s *Services) Prefs() env.Prefs { return s.PrefsV }

// Version answers Ver.
func (s *Services) Version() string { return s.Ver }

// Now answers Clock, 2026-11-15 12:00 UTC when unset.
func (s *Services) Now() time.Time {
	if s.Clock.IsZero() {
		return time.Date(2026, time.November, 15, 12, 0, 0, 0, time.UTC)
	}

	return s.Clock
}

// Stop records the stop.
func (p *Pairing) Stop() { p.Stopped = true }

// Remaining answers the time set by Push.
func (p *Pairing) Remaining() time.Duration { return p.left }

// Current answers the state set by Push.
func (p *Pairing) Current() (pairing.State, bool) { return p.state, p.have }

// Push shows a new code: the state becomes current, remaining is left, and the
// OnState hook is called.
func (p *Pairing) Push(st pairing.State, remaining time.Duration) {
	p.state, p.have, p.left = st, true, remaining
	if p.Hooks.OnState != nil {
		p.Hooks.OnState(st)
	}
}

// Advance lets d of the code's time pass: Remaining is d less, never below 0. A
// test moves it with the view's clock (Tester.Elapse) to have the countdown run.
func (p *Pairing) Advance(d time.Duration) { p.left = max(p.left-d, 0) }

// SignIn is the code being confirmed on the phone.
func (p *Pairing) SignIn(email string) {
	if p.Hooks.OnSignedIn != nil {
		p.Hooks.OnSignedIn(email)
	}
}

// Fail is the pairing giving up with err.
func (p *Pairing) Fail(err error) {
	if p.Hooks.OnError != nil {
		p.Hooks.OnError(err)
	}
}
