//go:build !tvos

// Helpers of the flow tests (flow_mobile_test.go). The gate that holds a
// request until the test lets it go is adapted from
// jplayer-mobile/internal/ui/views/flow_test.go at 49de3c2
// (TestAddFilesToastsInOrder: a channel the fake waits on while frames run).

package app

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// scripted is one answer the script transport gives in place of the mock's.
type scripted struct {
	status int
	file   string // under the mock's folder, "errors/token_expired.json"
	body   string // when set, the body itself in place of the file's
}

// scriptTransport sits between the app's dev transport and the mock: it
// counts the requests of each route, answers the ones the test scripted
// (an HTTP error the mock has no scenario for), and can keep an answer back
// after the mock produced it, ignoring the request's context, as a
// response that was already read when the session ended.
type scriptTransport struct {
	inner api.Transport

	mu    sync.Mutex
	queue map[string][]scripted
	calls map[string]int
	late  map[string]*lateGate
	// expired are the access tokens the server no longer accepts.
	expired map[string]bool
	gates   map[string]chan struct{}
	// bodies are the bodies of the requests, by route, in the order they came.
	bodies map[string][][]byte
	// done counts the requests of each route that returned.
	done map[string]int
}

// lateGate holds one route's answers back.
type lateGate struct {
	honor   bool // the request's context ends the wait
	reached chan struct{}
	release chan struct{}
	once    sync.Once
}

// flow is a phone's shell over the real services, with the script
// transport in the way of the requests.
type flow struct {
	*rig
	ph kittest.Phone
	st *scriptTransport
}

// flowOpts are what a flow starts with.
type flowOpts struct {
	secrets *memSecrets // the phone's vault; nil is empty
	dir     string      // the phone's folders; "" is new ones
	bio     sys.BiometryKind
	options shell.Options
	// script, when set, runs on the script transport before the shell is
	// mounted: the boot's requests start with the first frame, so what must
	// shape them (a failure to answer, a request to hold) is set here, not
	// after newFlow returns.
	script func(st *scriptTransport)
	// edit, when set, changes the services' dependencies before they are made.
	edit func(d *mobileDeps)
	// knobs are the services' developer knobs (a download's speed, a wrong
	// sha256); its prepare runs after the script transport is in place.
	knobs wireKnobs
	// wire, when set, runs on the services as they are built, before any
	// request: what must be in place before the first frame (a mock scenario).
	wire func(w *mobileWire)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newScriptTransport returns a script transport with nothing scripted.
func newScriptTransport() *scriptTransport {
	return &scriptTransport{
		queue: map[string][]scripted{}, calls: map[string]int{},
		late: map[string]*lateGate{}, expired: map[string]bool{}, gates: map[string]chan struct{}{},
		bodies: map[string][][]byte{}, done: map[string]int{},
	}
}

// newFlow mounts the shell on the phone over the real services; the first
// blocking call of the boot makes them, and the script transport is put in
// the way before any request.
func newFlow(t *testing.T, ph kittest.Phone, o flowOpts) *flow {
	t.Helper()
	r := newRig(t, o.secrets, o.dir)
	r.bio.SetKind(o.bio)
	if o.edit != nil {
		o.edit(&r.deps)
	}
	f := &flow{rig: r, ph: ph, st: newScriptTransport()}
	if o.script != nil {
		o.script(f.st)
	}
	knobs := o.knobs
	userPrepare := knobs.prepare
	knobs.prepare = func(ctx context.Context, w *mobileWire) error {
		f.st.inner = w.tr.inner
		w.tr.inner = f.st
		if o.wire != nil {
			o.wire(w)
		}
		if userPrepare != nil {
			return userPrepare(ctx, w)
		}

		return nil
	}
	w := newMobileWire(r.deps, knobs)
	r.w = w
	r.sh = shell.New(w, locales.MustTexts(), o.options)
	r.tt = kittest.Mount(t, ph, r.sh)
	kittest.PinMotion(r.tt)
	w.attach(r.tt.View())
	t.Cleanup(w.close)

	return f
}

// seedSession signs a user in on a phone's vault and folders in a launch
// of its own, as a user who signed in some days ago.
func seedSession(t *testing.T, secrets *memSecrets, dir string, bio sys.BiometryKind) {
	t.Helper()
	r := newRig(t, secrets, dir)
	r.bio.SetKind(bio)
	w := r.wire(wireKnobs{})
	_, err := w.Restore(t.Context())
	require.NoError(t, err)
	require.NoError(t, w.Login(t.Context(), "coach@example.invalid", "secret1"))
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do counts the request, answers what was scripted, or asks the mock;
// a late route keeps the mock's answer back until released.
func (s *scriptTransport) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	defer func() {
		s.mu.Lock()
		s.done[req.RouteID]++
		s.mu.Unlock()
	}()
	s.mu.Lock()
	s.calls[req.RouteID]++
	if req.Body != nil {
		s.bodies[req.RouteID] = append(s.bodies[req.RouteID], append([]byte(nil), req.Body...))
	}
	hold := s.gates[req.RouteID]
	tok := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
	if tok != "" && s.expired[tok] {
		s.mu.Unlock()

		return s.answer(scripted{status: http.StatusUnauthorized, file: "errors/token_expired.json"})
	}
	s.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	s.mu.Lock()
	var next *scripted
	if q := s.queue[req.RouteID]; len(q) > 0 {
		next = &q[0]
		s.queue[req.RouteID] = q[1:]
	}
	gate := s.late[req.RouteID]
	s.mu.Unlock()
	if next != nil {
		return s.answer(*next)
	}
	resp, err := s.inner.Do(ctx, req)
	if gate != nil && err == nil {
		gate.once.Do(func() { close(gate.reached) })
		if gate.honor {
			select {
			case <-gate.release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		} else {
			<-gate.release // not the context: the answer is already here
		}
	}

	return resp, err
}

// answer is the scripted response.
func (s *scriptTransport) answer(sc scripted) (*api.Response, error) {
	if sc.body != "" {
		return &api.Response{Status: sc.status, Header: http.Header{}, Body: []byte(sc.body)}, nil
	}
	body, err := os.ReadFile(filepath.Join(mockDir, sc.file))
	if err != nil {
		return nil, err
	}

	return &api.Response{Status: sc.status, Header: http.Header{}, Body: body}, nil
}

// expire makes every request that carries the access token answer 401
// token_expired, as a server does after the token's 15 minutes.
func (s *scriptTransport) expire(token string) {
	s.mu.Lock()
	s.expired[token] = true
	s.mu.Unlock()
}

// holdRequest keeps the requests of the route back, before the mock, until
// release (or until their context ends).
func (s *scriptTransport) holdRequest(route string) (release func()) {
	ch := make(chan struct{})
	s.mu.Lock()
	s.gates[route] = ch
	s.mu.Unlock()
	var once sync.Once

	return func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.gates, route)
			s.mu.Unlock()
			close(ch)
		})
	}
}

// failNext makes the next n requests of the route answer status with the
// body of the mock's file.
func (s *scriptTransport) failNext(route string, n, status int, file string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for range n {
		s.queue[route] = append(s.queue[route], scripted{status: status, file: file})
	}
}

// answerNext makes the next request of the route answer status with body: an
// answer the mock has no scenario for (a 200 with email_change_pending).
func (s *scriptTransport) answerNext(route string, status int, body string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue[route] = append(s.queue[route], scripted{status: status, body: body})
}

// calledN is how many requests of the route reached the transport.
func (s *scriptTransport) calledN(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.calls[route]
}

// doneN is how many requests of the route returned.
func (s *scriptTransport) doneN(route string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.done[route]
}

// bodiesOf returns the bodies of the requests of the route so far.
func (s *scriptTransport) bodiesOf(route string) [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([][]byte(nil), s.bodies[route]...)
}

// holdAnswer keeps back the answers of the route after the mock made them:
// reached closes when the first is held, release lets them go; honor tells
// whether the request's context ends the wait.
func (s *scriptTransport) holdAnswer(route string, honor bool) (reached <-chan struct{}, release func()) {
	g := &lateGate{honor: honor, reached: make(chan struct{}), release: make(chan struct{})}
	s.mu.Lock()
	s.late[route] = g
	s.mu.Unlock()
	var once sync.Once

	return g.reached, func() {
		once.Do(func() {
			s.mu.Lock()
			delete(s.late, route)
			s.mu.Unlock()
			close(g.release)
		})
	}
}

// route is the id of the route showing.
func (f *flow) route() string { return f.sh.Nav().Current().Screen }

// until drives frames and queued work until the route is want.
func (f *flow) until(want string) {
	f.t.Helper()
	f.drive("the route "+want, func() bool { return f.route() == want })
}

// ready waits until the services are made and the session is read.
func (f *flow) ready() {
	f.t.Helper()
	f.drive("the services", func() bool { return f.w.ready.Load() })
}

// settleFor drives frames for the given virtual time, the queued work
// running between them.
func (f *flow) settleFor(frames int) {
	f.t.Helper()
	for range frames {
		f.q.drain()
		f.tt.Elapse(kittest.Frame)
	}
}

// login returns the login screen showing.
func (f *flow) login() *screens.Login {
	f.t.Helper()
	l, ok := f.sh.Nav().Screen().(*screens.Login)
	require.True(f.t, ok, "the screen is the login, not %T", f.sh.Nav().Screen())

	return l
}

// home returns the Home showing.
func (f *flow) home() *screens.Home {
	f.t.Helper()
	h, ok := f.sh.Nav().Screen().(*screens.Home)
	require.True(f.t, ok, "the screen is Home, not %T", f.sh.Nav().Screen())

	return h
}

// courses returns the Courses showing.
func (f *flow) courses() *screens.Courses {
	f.t.Helper()
	c, ok := f.sh.Nav().Screen().(*screens.Courses)
	require.True(f.t, ok, "the screen is Courses, not %T", f.sh.Nav().Screen())

	return c
}

// detail returns the Detail showing.
func (f *flow) detail() *screens.Detail {
	f.t.Helper()
	d, ok := f.sh.Nav().Screen().(*screens.Detail)
	require.True(f.t, ok, "the screen is the detail, not %T", f.sh.Nav().Screen())

	return d
}

// signIn types the email and the password and presses Enter, as a user
// does, then waits for the Home.
func (f *flow) signIn(email, password string) {
	f.t.Helper()
	l := f.login()
	f.tt.View().Focus(l.Email().Field())
	l.Email().SetText(email)
	f.tt.View().Focus(l.Password().Field())
	require.True(f.t, f.tt.Type(password))
	require.True(f.t, f.tt.Key(keys.KeyEnter, 0))
}

// homeLoaded waits until Home shows its content.
func (f *flow) homeLoaded() {
	f.t.Helper()
	f.until("home")
	f.drive("Home's content", func() bool {
		h, ok := f.sh.Nav().Screen().(*screens.Home)

		return ok && h.Content() != nil
	})
}

// coursesLoaded waits until Courses shows its list.
func (f *flow) coursesLoaded() {
	f.t.Helper()
	f.until("courses")
	f.drive("the courses", func() bool {
		c, ok := f.sh.Nav().Screen().(*screens.Courses)

		return ok && c.Content() != nil && c.Content().Rows() > 0
	})
}

// tab taps the tab at column col (0 home, 1 courses, 2 downloads, 3 account)
// of the tab bar, by place: the screens under the bar have words of their
// own with the names of the tabs.
func (f *flow) tab(col int) {
	f.t.Helper()
	bar := f.sh.Nav().TabBar().Node().WindowFrame()
	f.tt.ClickAt(geom.Pt(bar.X+(float32(col)+.5)*bar.W/4, bar.Y+bar.H*.4))
	f.settleFor(2)
}

// back presses Android's Back and reports whether the app took it.
func (f *flow) back() bool {
	f.t.Helper()
	took := f.tt.Key(keys.KeyBack, 0)
	f.settleFor(2)

	return took
}

// shows asserts that the text is on the screen within a few frames.
func (f *flow) shows(text string) {
	f.t.Helper()
	f.drive("the text "+text, func() bool { return f.tt.Shows(text) })
}

// gone asserts that the text is not on the screen.
func (f *flow) gone(text string) {
	f.t.Helper()
	assert.False(f.t, f.tt.Shows(text), "%q is still on the screen", text)
}
