//go:build tvos

// The Apple TV's shot states (docs/plan/07-testing.md §3.3, local/plans/
// P7.md P7.5b step 6), the pattern of states_mobile.go: each is reached
// through the real flow over the mock (D7), the pairing included (TV-toast
// and every signed-in state pair with the mock's scenario tv.pair.poll =
// confirmed, so the session is the one a phone hands over), then the
// screen of the state is routed to. TV05 to TV12 show whatever screen the
// shell has for their route (P7.3, P7.4 replace the stubs).

package app

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/shell"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvDemoNow is the clock of the TV states: Saturday 14 November 2026,
// 09:41 at +07:00, the prototype's and the mock's server_time.
var tvDemoNow = time.Date(2026, time.November, 14, 9, 41, 0, 0, time.FixedZone("+07", 7*3600))

// The demo lesson of TV07 to TV10 (Q-P4-12): lesson 3 of Set 35, 220 s
// long, at 01:12 (`continue.position_sec`), paused, speed .75, mirror and
// beat on.
const (
	tvDemoCourse  = "set-35"
	tvDemoLesson  = "l_3503"
	tvDemoSeconds = 220
	tvDemoAt      = 72
	tvDemoRate    = 0.75
)

// tvToastGone is how long TV04 waits after the pairing's toast: its
// 2400 ms and the fade (index.html:732).
const tvToastGone = 2600 * time.Millisecond

// tvStepPoll is how often the runner looks whether its next step may run.
const tvStepPoll = 50 * time.Millisecond

// tvHoldEvery is how often a held state puts its moving part back: well
// within the toast's 2400 ms and the countdown's second.
const tvHoldEvery = 400 * time.Millisecond

// tvShotState is a state of the TV: the knobs pinned before the shell
// starts, the mock walked to its start (off the UI thread, before the
// session is read), then the steps on the UI thread. Once they ran, the
// runner waits until every key of shows is on screen and, when focus is
// set, until it is the focused item's key: a screen whose data is still
// on its way (a ui.Go read that has not reached the transport yet looks
// idle to the shot) is not shot blank.
type tvShotState struct {
	knobs   func(k *tvKnobs)
	prepare func(ctx context.Context, w *tvWire) error
	steps   []tvStep
	// shows are focus keys or names of widgets the state's content has once
	// read: never a screen's stand-in key while it waits (screens.waitAnchor:
	// h1, cf0, d1, mu0, p1, u1), which is on screen before the data.
	shows []string
	// focus is the focus key the state ends on ("" leaves it as it is).
	focus string
}

// tvStep is one step: once until holds (nil: at once), act runs (nil:
// the step only waits).
type tvStep struct {
	what  string
	until func(r *tvStateRunner) bool
	act   func(r *tvStateRunner)
}

// tvStateRunner runs the steps of a state on the UI thread.
type tvStateRunner struct {
	id    string
	st    tvShotState
	b     *tvBoot
	c     *ui.Context
	next  int
	done  bool
	since time.Duration // when the step waiting began, on the view's clock
	tick  func()
	onEnd func()
	// course is the demo course, read by readCourse.
	course   model.Course
	haveCrs  bool
	crsError error
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// tvShotStates are the states by id: the prototype's (TV01 to TV12,
// TV-toast) and the developer's (TV01-confirm, TV02-expired, TV04-offline,
// TV-PAIR). The table is filled at init in every build but the shipped
// live one, which links none of them (devParts).
var tvShotStates = map[string]tvShotState{}

func init() { //nolint:gochecknoinits // the table of states, left out of the shipped live build
	if !devParts {
		return
	}
	maps.Copy(tvShotStates, baseTVShotStates())
}

// baseTVShotStates returns the states of the shots and of the flow tests.
func baseTVShotStates() map[string]tvShotState {
	return map[string]tvShotState{
		"TV01": {knobs: tvDemo, prepare: tvSignedOut(), steps: []tvStep{tvRouteIs(tvenv.RouteWelcome)}, shows: []string{"w1", "w2"}},
		// TV01-confirm is TV01 with the mock's pairing confirmed at its first
		// poll: the remote goes on to TV02 and TV04 by itself (the memory run of
		// local/plans/P7.md P7.5c, scripts/tvos-memgate.sh).
		"TV01-confirm": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: []tvStep{tvRouteIs(tvenv.RouteWelcome)},
			shows: []string{"w1", "w2"}, focus: "w1"},
		"TV02": {knobs: tvDemo, prepare: tvSignedOut(), steps: append(tvCodeSteps(), tvHoldPolls()), shows: tvCodeShows},
		"TV02-expired": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "expired"),
			steps: append(tvCodeSteps(), tvNewCode(), tvHoldPolls()), shows: tvCodeShows},
		// TV-toast holds the toast up (tvHoldToast): the shot rests 1.5 s after
		// the home's data, longer than the 2.4 s of the toast leave on the
		// simulator once the screenshot is taken.
		"TV-toast": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"),
			steps: append(tvPairedSteps(false), tvHoldToast()), shows: tvHomeShows},
		"TV04": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvPairedSteps(true), shows: tvHomeShows},
		"TV04-offline": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"),
			steps: append(tvPairedSteps(true), tvGoOffline()), shows: tvHomeShows},
		// TV05 is the prototype's capture: `S.tv.scr='courses'` with no focus
		// set, so the default rule puts it on the first filter (renderTV,
		// index.html:723), not on nav1 as Select on the nav item does.
		"TV05": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			tvGo(nav.Route{Screen: tvenv.RouteCourses}, true), tvRouteIs(tvenv.RouteCourses)),
			shows: []string{"cf1", "cg0", "nav1"}, focus: "cf0"},
		"TV06": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			tvGo(nav.Route{Screen: tvenv.RouteDetail, Params: tvDemoCourse}, false), tvRouteIs(tvenv.RouteDetail)),
			shows: []string{"d2", "dl0"}},
		"TV07": {knobs: tvDemoVideo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(tvVideoSteps()...),
			shows: []string{"p2", "pn0"}},
		"TV08": {knobs: tvDemoVideo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			append(tvVideoSteps(), tvPress("p2"), tvPanelOpen())...), shows: []string{"p2", "o0", "oc"}},
		// TV09 and TV12 are the prototype's captures as TV05 is: the route set
		// with no focus, so the default rule puts it on `mu0` and, on the
		// settings, on the first `.f`, `nav0` (index.html:723-725), not on the
		// nav item Select leaves it on.
		"TV09": {knobs: tvDemoVideo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(tvAudioSteps()...),
			shows: []string{"mu1", "mu2"}, focus: "mu0"},
		// TV10 is the up-next screen with its countdown held at 5, as captured
		// (tvHoldUpNext).
		"TV10": {knobs: tvDemoVideo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			append(tvVideoSteps(), tvResetUpNext(), tvReplace(nav.Route{Screen: tvenv.RouteUpNext}), tvRouteIs(tvenv.RouteUpNext),
				tvHoldUpNext())...),
			shows: []string{"u2"}, focus: "u1"},
		"TV11": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			tvGo(nav.Route{Screen: tvenv.RouteError}, false), tvRouteIs(tvenv.RouteError)), shows: []string{"e1", "e2"}},
		"TV12": {knobs: tvDemo, prepare: tvSignedOut("tv.pair.poll", "confirmed"), steps: tvAfterHome(
			tvGo(nav.Route{Screen: tvenv.RouteSettings}, true), tvRouteIs(tvenv.RouteSettings)),
			shows: []string{"s0", "s5"}, focus: "nav0"},
		// TV-PAIR is the TV half of the pairing with a phone over cmd/mockserve
		// (DFV_API, local/plans/P7.md section 8.2): TV02, then TV04 once a phone
		// confirmed the code shown. Without a phone it waits on TV02.
		"TV-PAIR": {knobs: tvDemo, prepare: tvSignedOut(), steps: append(tvCodeSteps(), tvRouteIs(tvenv.RouteHome), tvToastShown()),
			shows: tvHomeShows},
	}
}

// tvCodeShows and tvHomeShows are what TV02 and TV04 show once read.
var (
	tvCodeShows = []string{"c2", "tv.code.value"}
	tvHomeShows = []string{"hc0", "h2"}
)

// newTVStateRunner returns the runner of state id over the boot; onEnd runs
// once the last step ran and the state shows what it should (shows, focus).
func newTVStateRunner(id string, st tvShotState, b *tvBoot, onEnd func()) *tvStateRunner {
	st.steps = slices.Clip(st.steps)
	if len(st.shows) > 0 {
		st.steps = append(st.steps, tvShows(st.shows))
	}
	if st.focus != "" {
		st.steps = append(st.steps, tvFocusOn(st.focus))
	}
	r := &tvStateRunner{id: id, st: st, b: b, onEnd: onEnd}
	r.tick = r.step

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// lookupTVState returns the state of id; core.ErrNotFound names the ones
// there are.
func lookupTVState(id string) (tvShotState, error) {
	st, ok := tvShotStates[id]
	if !ok {
		return tvShotState{}, fmt.Errorf("app: state %q: one of %s: %w", id, strings.Join(tvStateIDs(), ", "), core.ErrNotFound)
	}

	return st, nil
}

// tvStateIDs returns the ids of the states, sorted.
func tvStateIDs() []string {
	ids := make([]string, 0, len(tvShotStates))
	for id := range tvShotStates {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	return ids
}

// tvKnobsOf returns the knobs of st.
func tvKnobsOf(st tvShotState) tvKnobs {
	var k tvKnobs
	if st.knobs != nil {
		st.knobs(&k)
	}
	k.prepare = st.prepare

	return k
}

// start runs the steps from now, on the UI thread of c.
func (r *tvStateRunner) start(c *ui.Context) {
	r.c = c
	r.since = c.Now()
	r.step()
}

// Done tells whether every step ran.
func (r *tvStateRunner) Done() bool { return r.done }

// Waiting names the step the runner waits for, "" once done.
func (r *tvStateRunner) Waiting() string {
	if r.done {
		return ""
	}

	return r.st.steps[r.next].what
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// step runs the steps whose condition holds, then looks again later.
func (r *tvStateRunner) step() {
	if r.w().noMock.Load() { // a scenario of the mock over a live API (apimode.go)
		r.w().deps.Logf("dfv: state %s stopped: it needs the mock in the process, not a live API", r.id)

		return
	}
	for r.next < len(r.st.steps) {
		s := r.st.steps[r.next]
		if s.until != nil && !s.until(r) {
			r.c.AfterFunc(tvStepPoll, r.tick)

			return
		}
		if s.act != nil {
			s.act(r)
		}
		r.next++
		r.since = r.c.Now()
	}
	if !r.done {
		r.done = true
		if r.onEnd != nil {
			r.onEnd()
		}
	}
}

// sh is the shell of the state.
func (r *tvStateRunner) sh() *shell.Shell { return r.b.sh }

// w is the services of the state.
func (r *tvStateRunner) w() *tvWire { return r.b.w }

// tvDemo pins the clock of the screens.
func tvDemo(k *tvKnobs) { k.now = tvDemoNow }

// tvDemoVideo is tvDemo with the fake engine of the playback states: the
// media are 220 s long and keep time with a frozen clock the steps move.
func tvDemoVideo(k *tvKnobs) {
	tvDemo(k)
	if devEnv(engineEnv) == "real" {
		return
	}
	clk := clock.NewFake(tvDemoNow)
	k.mediaClock = clk
	k.media = func(cfg media.Config) (platformMedia, error) {
		cfg.Clock = clk

		return fakeMediaOf(fake.Options{VideoDuration: tvDemoSeconds, AudioDuration: tvDemoSeconds})(cfg)
	}
}

// tvSignedOut starts from a TV signed out (a session a run before left is
// forgotten), the scenario of a route set when given (route, name).
func tvSignedOut(scenario ...string) func(ctx context.Context, w *tvWire) error {
	return func(ctx context.Context, w *tvWire) error {
		if err := w.mgr.Logout(ctx); err != nil {
			return fmt.Errorf("sign out: %w", err)
		}
		if len(scenario) == 2 {
			return setScenario(w.mock, scenario[0], scenario[1])
		}

		return nil
	}
}

// tvRouteIs waits for the route showing to be route.
func tvRouteIs(route string) tvStep {
	return tvStep{what: "route " + route, until: func(r *tvStateRunner) bool {
		return r.b.restored && r.sh().Stack().Screen() == route && r.sh().Screen() != nil
	}}
}

// tvGo routes to rt (tvGo).
func tvGo(rt nav.Route, root bool) tvStep {
	return tvStep{what: "go " + rt.Screen, act: func(r *tvStateRunner) { r.sh().Go(rt, root) }}
}

// tvReplace puts rt in the place of the route showing.
func tvReplace(rt nav.Route) tvStep {
	return tvStep{what: "replace " + rt.Screen, act: func(r *tvStateRunner) { r.sh().Replace(rt) }}
}

// tvHold runs fn now and every tvHoldEvery after, for as long as the state
// is up: a developer's state that keeps a moving part where the prototype
// captured it.
func tvHold(what string, fn func(r *tvStateRunner)) tvStep {
	return tvStep{what: what, act: func(r *tvStateRunner) {
		var again func()
		again = func() {
			fn(r)
			r.c.AfterFunc(tvHoldEvery, again)
		}
		again()
	}}
}

// tvHoldToast shows the toast's message again while it is up, which starts
// its 2400 ms over without the fade (K17), so that the toast is still there
// when the shot is taken.
func tvHoldToast() tvStep {
	return tvHold("hold the toast", func(r *tvStateRunner) {
		if t := r.sh().ToastHost(); t.Visible() {
			r.sh().Toast(t.Message())
		}
	})
}

// tvResetUpNext starts the up-next count over at 5, as a video that ends
// does (`cdT=5`).
func tvResetUpNext() tvStep {
	return tvStep{what: "reset the count", act: func(r *tvStateRunner) { r.sh().Env().ResetUpNext() }}
}

// tvHoldUpNext holds the up-next countdown at 5 (TV10's capture): the count
// starts over before its first second is up.
func tvHoldUpNext() tvStep {
	return tvHold("hold the count", func(r *tvStateRunner) {
		if u, ok := r.sh().Screen().(interface{ Restart() }); ok {
			u.Restart()
		}
	})
}

// tvShows waits until every key of keys is on screen (tvFindKey).
func tvShows(keys []string) tvStep {
	return tvStep{what: "shows " + strings.Join(keys, ","), until: func(r *tvStateRunner) bool {
		for _, k := range keys {
			if tvFindKey(r.sh(), k) == nil {
				return false
			}
		}

		return true
	}}
}

// tvFocusOn waits until the focused item's key is key.
func tvFocusOn(key string) tvStep {
	return tvStep{what: "focus " + key, until: func(r *tvStateRunner) bool { return tvFocusKey(r.sh()) == key }}
}

// tvFocusKey is the key of the item the shell's focus is on, "" with none.
func tvFocusKey(sh *shell.Shell) string {
	if it := sh.Manager().Current(); it != nil {
		return it.FocusKey()
	}

	return ""
}

// tvCodeSteps go from TV01 to TV02 and wait for its code.
func tvCodeSteps() []tvStep {
	return []tvStep{
		tvRouteIs(tvenv.RouteWelcome),
		tvGo(nav.Route{Screen: tvenv.RouteCode}, false),
		tvRouteIs(tvenv.RouteCode),
		{what: "code shown", until: func(r *tvStateRunner) bool { return r.w().codeShown() }},
	}
}

// tvNewCode waits for a second code: the first expired (410) and the
// pairing asked again.
func tvNewCode() tvStep {
	return tvStep{what: "new code", until: func(r *tvStateRunner) bool {
		return r.w().pairStarts.Load() >= 2 && r.w().codeShown()
	}}
}

// tvHoldPolls holds the polls of the code shown, so that the state rests
// for its shot (a poll every 3 s is a request that starts the rest again).
func tvHoldPolls() tvStep {
	return tvStep{what: "hold the polls", act: func(r *tvStateRunner) { r.w().tr.hold("tv.pair.poll") }}
}

// tvPairedSteps pair with the mock's confirmed scenario: TV02, then TV04
// with the toast of the account (TV-toast), and, when gone, after it.
func tvPairedSteps(gone bool) []tvStep {
	steps := append(tvCodeSteps(), tvRouteIs(tvenv.RouteHome), tvToastShown())
	if gone {
		steps = append(steps, tvStep{what: "toast gone", until: func(r *tvStateRunner) bool {
			return !r.sh().ToastHost().Visible() && r.c.Now()-r.since >= tvToastGone
		}})
	}

	return steps
}

// tvAfterHome is TV04 (paired, the toast gone) then steps.
func tvAfterHome(steps ...tvStep) []tvStep {
	return append(tvPairedSteps(true), steps...)
}

// tvToastShown waits for the toast.
func tvToastShown() tvStep {
	return tvStep{what: "toast", until: func(r *tvStateRunner) bool { return r.sh().ToastHost().Visible() }}
}

// tvGoOffline makes every request fail as a dead network does, and the app
// offline.
func tvGoOffline() tvStep {
	return tvStep{what: "offline", act: func(r *tvStateRunner) {
		w := r.w()
		w.tr.setOffline(true)
		w.online.Set(false)
	}}
}

// tvReadCourse reads the demo course off the UI thread (the screens of
// P7.3 do when its card is chosen), then waits for it.
func tvReadCourse() []tvStep {
	return []tvStep{
		{what: "read the course", act: func(r *tvStateRunner) {
			w := r.w()
			w.background(func() {
				c, err := w.Course(context.Background(), tvDemoCourse)
				w.dispatch(func() { r.course, r.crsError, r.haveCrs = c, err, true })
			})
		}},
		{what: "course read", until: func(r *tvStateRunner) bool {
			if r.haveCrs && r.crsError != nil {
				r.w().deps.Logf("dfv: state %s: course: %v", r.id, r.crsError)
			}

			return r.haveCrs
		}},
	}
}

// tvPlay starts the demo lesson with play, on the course read.
func tvPlay(what string, play func(r *tvStateRunner, l player.Lesson, list []player.Lesson)) tvStep {
	return tvStep{what: what, act: func(r *tvStateRunner) {
		list := tvenv.PlayerLessons(r.course)
		i := tvenv.LessonIndex(list, tvDemoLesson)
		if i < 0 {
			r.w().deps.Logf("dfv: state %s: no lesson %s in %s", r.id, tvDemoLesson, tvDemoCourse)

			return
		}
		play(r, list[i], list)
	}}
}

// tvLoaded waits for the lesson loaded, moving the frozen clock of the fake
// engine by 1 ms while the load is under way (the address is asked for off
// the UI thread).
func tvLoaded() tvStep {
	return tvStep{what: "lesson loaded", until: func(r *tvStateRunner) bool {
		pb := r.w().pb
		switch pb.State() {
		case player.StateLoading:
			if c := r.w().knobs.mediaClock; c != nil {
				c.Advance(time.Millisecond)
			}

			return false
		case player.StatePlaying, player.StatePaused:
			return true
		default:
			return false
		}
	}}
}

// tvDemoOptions puts the demo lesson at 01:12, paused, .75×, mirror and
// beat on (the prototype's TV07).
func tvDemoOptions() tvStep {
	return tvStep{what: "demo options", act: func(r *tvStateRunner) {
		pb := r.w().pb
		if at := r.demoAt(); pb.Pos() != at {
			pb.player.Seek(at)
		}
		pb.SetRate(tvDemoRate)
		if pb.Kind() == player.KindVideo {
			pb.SetMirror(true)
		}
		pb.SetBeat(true)
		pb.Pause()
	}}
}

// tvVideoSteps read the course and play lesson 3 as a video on TV07.
func tvVideoSteps() []tvStep {
	return append(tvReadCourse(),
		tvPlay("play video 3", func(r *tvStateRunner, l player.Lesson, list []player.Lesson) {
			r.w().pb.PlayVideo(l, list)
			r.sh().Go(nav.Route{Screen: tvenv.RoutePlayer}, false)
		}),
		tvRouteIs(tvenv.RoutePlayer), tvLoaded(), tvDemoOptions())
}

// tvAudioSteps read the course and play lesson 3 as audio at 01:12 on TV09
// (the music route set as the root, the default focus on `mu0`).
func tvAudioSteps() []tvStep {
	return append(tvReadCourse(),
		tvPlay("play audio 3", func(r *tvStateRunner, l player.Lesson, list []player.Lesson) {
			r.w().pb.PlayAudio(l, list, r.demoAt())
			r.sh().Go(nav.Route{Screen: tvenv.RouteMusic}, true)
		}),
		tvRouteIs(tvenv.RouteMusic), tvLoaded(), tvDemoOptions())
}

// tvPress activates the control with focus key (or name) key, as Select
// on it does.
func tvPress(key string) tvStep {
	return tvStep{what: "press " + key,
		until: func(r *tvStateRunner) bool { return tvFindKey(r.sh(), key) != nil },
		act: func(r *tvStateRunner) {
			if a, ok := tvFindKey(r.sh(), key).(interface{ Activate(long bool) bool }); ok {
				a.Activate(false)
			}
		}}
}

// tvPanelOpen waits for the player's panel.
func tvPanelOpen() tvStep {
	return tvStep{what: "panel open", until: func(r *tvStateRunner) bool { return r.sh().Stack().Panel() }}
}

// tvFindKey returns the first visible widget under root whose focus key
// or name is key, nil when there is none.
func tvFindKey(root ui.Widget, key string) ui.Widget {
	b := root.Node()
	if b.Hidden {
		return nil
	}
	if k, ok := root.(interface{ FocusKey() string }); (ok && k.FocusKey() == key) || b.Name == key {
		return root
	}
	for ch := range b.Children() {
		if w := tvFindKey(ch, key); w != nil {
			return w
		}
	}

	return nil
}

// demoAt is where the demo lesson starts: 01:12 on the fake engine, the
// start on the system's (DFV_ENGINE=real: the mock's media last 12 s and
// 30 s).
func (r *tvStateRunner) demoAt() float64 {
	if r.w().knobs.media == nil && r.w().deps.NewMedia == nil {
		return 0
	}

	return tvDemoAt
}
