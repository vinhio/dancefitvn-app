//go:build tvos

// Adapted from jplayer-tv/internal/ui/shell/walk_test.go at 781bee5.

package shell_test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// walkStep is one move of the random walk.
type walkStep int

const (
	stepLeft walkStep = iota
	stepUp
	stepRight
	stepDown
	stepSelect
	stepHold
	stepMenu
	stepPlayPause
	stepWait       // 2.6 s pass: a poll of the pairing, a toast must be gone
	stepNetwork    // the network goes away or comes back
	stepLessonEnds // the lesson of the player reaches its end
	stepKinds
)

// stepNames name the steps in a failure's history.
var stepNames = [stepKinds]string{"Left", "Up", "Right", "Down", "Select", "Hold", "Menu", "PlayPause", "Wait", "Network", "End"}

// stepWeights are the odds of each step: the four arrows 5, Select 10, a hold
// 2, Menu 4, Play/Pause 1, a wait 2, the network 2, the end of a lesson 2
// (07 section 2, the keys of the spec plus the three events the keys cannot
// make). Select weighs most: the screens below the tabs are reached by it.
var stepWeights = [stepKinds]int{5, 5, 5, 5, 10, 2, 4, 1, 2, 2, 2}

// Limits of the invariants (P7.6 step 5).
const (
	walkSeed = 20261114
	// maxDepth is the stack's depth for the routes the walk takes, apart from the
	// laps below: home, detail, player, TV11 over it, and a little more.
	maxDepth = 6
	// lapDepth is what a lap of detail, player, up-next, "Về khoá học" adds to the
	// stack: the prototype's tvGo pushes the up-next screen it leaves (`data-tgo=
	// "detail"` of `u2`, index.html:703, tvGo :730), so the stack the prototype
	// builds has no bound a walk could count on, and each lap adds these two.
	lapDepth = 2
	// seedWalks is the number of walks of the seeds test: seeds +1..+7, the
	// walks that start at TV01 staying on the pairing for the most part, the
	// seventh being the first to reach the settings (the walks are the same
	// each run: the reads of the screens are settled after each step).
	seedWalks      = 7
	toastGone      = 2600 * time.Millisecond
	pollEvery      = 3 // the pairing is confirmed at this poll
	walkConfirmWho = "thaovy.hlv@gmail.com"
	panelScreen    = "player+panel" // the 11th screen: TV08
)

// walkPauses are the pauses after a step; the race build leaves out the long
// one, every frame costing more there.
var walkPauses = func() []time.Duration {
	if kittest.Race {
		return []time.Duration{0, 50 * time.Millisecond, 200 * time.Millisecond}
	}

	return []time.Duration{0, 50 * time.Millisecond, 200 * time.Millisecond, 600 * time.Millisecond}
}()

// walkLen is the length of a walk of n steps: the race detector makes every
// frame cost many times more (a plain 600-step walk is 15 s, over 10 minutes
// under -race), so a race build keeps a smoke walk of a sixth; the plain build
// runs them in full and asserts the coverage.
func walkLen(n int) int {
	if kittest.Race {
		return max(n/6, 25)
	}

	return n
}

// walkStats is what a walk did.
type walkStats struct {
	steps    int
	byKind   [stepKinds]int
	moved    int            // arrows that changed the focus
	menuOut  int            // Menu presses left to the system
	screens  map[string]int // steps spent on each screen (the panel apart)
	keysSeen map[string]bool
	toasts   int // announcements of the toast
	deepest  int
	laps     int // visits of the up-next screen's "Về khoá học" that pushed it
	confirms int // pairings confirmed
	polls    int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestRandomWalkFromHome presses 600 random remote keys from TV04, signed in
// (07 section 2, UI TV): after every one exactly one widget is focused and it
// is an item of the current scope, a panel open means the focus is inside it,
// Menu is unused only at a root screen with no panel, a toast is gone after
// 2.6 s, nothing leaks, the stack stays within 6 and nothing panics.
func TestRandomWalkFromHome(t *testing.T) {
	t.Parallel()

	st := randomWalk(t, walkSeed, walkLen(600), env.RouteHome)
	t.Logf("seed %d from TV04: %d steps %v, %d arrows moved the focus, %d Menu left to the system, deepest stack %d, %d toasts, %d pairings confirmed, screens %v",
		walkSeed, st.steps, st.byKind, st.moved, st.menuOut, st.deepest, st.toasts, st.confirms, st.screens)
	assert.Equal(t, walkLen(600), st.steps)
	if kittest.Race {
		return
	}
	assert.Greater(t, st.moved, 100, "the walk moves around")
	assert.Greater(t, len(st.keysSeen), 15, "and visits many items")
	assert.GreaterOrEqual(t, len(st.screens), 3, "and several screens")
}

// TestRandomWalkFromWelcome walks from TV01 signed out; the pairing of TV02
// is confirmed at its third poll (a step of 3 s on the screen), which takes
// the walk to TV04 with the toast.
func TestRandomWalkFromWelcome(t *testing.T) {
	t.Parallel()

	st := randomWalk(t, walkSeed, walkLen(600), env.RouteWelcome)
	t.Logf("seed %d from TV01: %d steps %v, %d arrows moved the focus, %d Menu left to the system, deepest stack %d, %d toasts, %d pairings confirmed after %d polls, screens %v",
		walkSeed, st.steps, st.byKind, st.moved, st.menuOut, st.deepest, st.toasts, st.confirms, st.polls, st.screens)
	assert.Equal(t, walkLen(600), st.steps)
	if kittest.Race {
		return
	}
	assert.Greater(t, st.moved, 50)
	assert.GreaterOrEqual(t, len(st.screens), 3)
	assert.Equal(t, 1, st.confirms, "the pairing is confirmed at its third poll")
}

// TestRandomWalkSeedsVisitEveryScreen runs seeds +1..+4 at 150 steps, from TV04
// and TV01 in turn, and checks that together they visit all 11 screens: TV01,
// TV02, TV04, TV05, TV06, TV07, TV08 (the panel), TV09, TV10, TV11 (by the
// network step) and TV12, the invariants holding on each step.
func TestRandomWalkSeedsVisitEveryScreen(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	visited := map[string]bool{}
	t.Run("walks", func(t *testing.T) {
		for i := range seedWalks {
			seed := uint64(walkSeed + 1 + i)
			start := []string{env.RouteHome, env.RouteWelcome}[i%2]
			t.Run(fmt.Sprintf("seed %d from %s", seed, start), func(t *testing.T) {
				t.Parallel()
				st := randomWalk(t, seed, walkLen(150), start)
				mu.Lock()
				for s := range st.screens {
					visited[s] = true
				}
				mu.Unlock()
				t.Logf("screens %v", st.screens)
			})
		}
	})
	if kittest.Race {
		return // the coverage needs the full walks
	}
	for _, want := range []string{
		env.RouteWelcome, env.RouteCode, env.RouteHome, env.RouteCourses, env.RouteDetail, env.RoutePlayer, panelScreen,
		env.RouteMusic, env.RouteUpNext, env.RouteError, env.RouteSettings,
	} {
		assert.True(t, visited[want], "the seeds never showed %s (visited %v)", want, visited)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// randomWalk runs n random steps from seed, starting on route, and checks the
// invariants after each; it stops at the first failure with the history.
func randomWalk(t *testing.T, seed uint64, n int, route string) walkStats {
	t.Helper()
	r := open(t, route, false)
	r.tt.View().Context().Theme().ReduceMotion = true // the frames of the animations cost the rasterizer
	rng := rand.New(rand.NewPCG(seed, 2))
	st := walkStats{screens: map[string]int{}, keysSeen: map[string]bool{}}
	var history []string
	total := 0
	for _, w := range stepWeights {
		total += w
	}
	pick := func() walkStep {
		x := rng.IntN(total)
		for k, w := range stepWeights {
			if x < w {
				return walkStep(k)
			}
			x -= w
		}

		return stepLeft
	}
	fail := func(step int, msg string) {
		t.Helper()
		start := max(0, len(history)-40)
		require.FailNow(t, msg, "seed %d from %s, step %d, last steps: %s", seed, route, step, strings.Join(history[start:], " "))
	}
	defer func() {
		if p := recover(); p != nil {
			fail(st.steps, fmt.Sprintf("panic: %v", p))
		}
	}()

	polls := 0 // polls of TV02 since the last confirmation
	for i := range n {
		kind := pick()
		before := r.key()
		depth, panel, screen := r.sh.Stack().Depth(), r.sh.Stack().Panel(), r.sh.Stack().Screen()
		announced := len(r.tt.Announcements())
		st.steps++
		st.byKind[kind]++
		switch kind {
		case stepLeft:
			r.tt.Key(keys.KeyLeft, 0)
		case stepUp:
			r.tt.Key(keys.KeyUp, 0)
		case stepRight:
			r.tt.Key(keys.KeyRight, 0)
		case stepDown:
			r.tt.Key(keys.KeyDown, 0)
		case stepSelect:
			r.tt.Key(keys.KeyEnter, 0)
		case stepHold:
			r.tt.KeyDown(keys.KeyEnter, 0)
			r.tt.Elapse(focus.HoldDuration + 50*time.Millisecond)
			r.tt.KeyUp(keys.KeyEnter, 0)
		case stepMenu:
			used := r.tt.Key(keys.KeyEscape, 0)
			if !used {
				st.menuOut++
				if depth != 0 || panel {
					fail(i, fmt.Sprintf("Menu was left to the system with stack depth %d and panel %v on %s", depth, panel, screen))
				}
			}
		case stepPlayPause:
			r.tt.Key(keys.KeyMediaPlayPause, 0)
		case stepWait:
			if ps := r.svc.Pairings(); screen == env.RouteCode && len(ps) > 0 && !ps[len(ps)-1].Stopped {
				p := ps[len(ps)-1]
				polls++
				st.polls++
				p.Advance(3 * time.Second)
				if polls == pollEvery {
					p.SignIn(walkConfirmWho)
					st.confirms++
					polls = 0
				}
			}
			r.tt.Elapse(toastGone)
			if len(r.tt.Announcements()) == announced && r.sh.ToastHost().Visible() {
				fail(i, "a toast is still on after 2.6 s")
			}
			r.tt.Elapse(400 * time.Millisecond)
		case stepNetwork:
			r.svc.SetOnline(!r.svc.OnlineNow)
			r.tt.Elapse(tvtest.Frame) // an event from outside is followed by a frame, as in the app
		case stepLessonEnds:
			if pb, ok := r.svc.PB.(*tvtest.Playback); ok && screen == env.RoutePlayer {
				pb.End()
				r.tt.Elapse(tvtest.Frame)
			}
		default:
		}
		d := walkPauses[rng.IntN(len(walkPauses))]
		r.tt.Elapse(d)
		// Elapse does not wait for the reads of the screens (ui.Go answers when its
		// goroutine is done), so under load a walk would meet screens with and
		// without their data by chance: settle them, the walk is the same each run.
		r.tt.RunUntilIdle()
		after := r.key()
		if kind <= stepDown && after != before && after != "" {
			st.moved++
		}
		history = append(history, fmt.Sprintf("%s(%s>%s)+%dms[%s d%d online=%v]", stepNames[kind], before, after, d.Milliseconds(), r.sh.Stack().Screen(), r.sh.Stack().Depth(), r.svc.OnlineNow))

		if msg := checkWalk(r); msg != "" {
			fail(i, msg+" after "+history[len(history)-1])
		}
		if leaks := r.tt.Leaks(); len(leaks) > 0 {
			fail(i, fmt.Sprintf("leaks: %v", leaks))
		}
		cur := r.sh.Stack().Screen()
		if r.sh.Stack().Panel() {
			st.screens[panelScreen]++
		}
		st.screens[cur]++
		st.keysSeen[after] = true
		got := r.sh.Stack().Depth()
		st.deepest = max(st.deepest, got)
		if got > depth+1 {
			fail(i, fmt.Sprintf("a step pushed %d routes (depth %d to %d)", got-depth, depth, got))
		}
		if screen == env.RouteUpNext && cur == env.RouteDetail && got == depth+1 {
			st.laps++
		}
		if limit := maxDepth + lapDepth*st.laps; got > limit {
			fail(i, fmt.Sprintf("stack depth %d (limit %d with %d laps)", got, limit, st.laps))
		}
	}
	st.toasts = len(r.tt.Announcements())

	return st
}

// checkWalk returns what is wrong with the focus, "" when it is as it should
// be: one widget drawn focused, an Item in the tree under the scope (the panel
// while it is open, else the shell), not under a hidden or disabled widget,
// with a frame inside the stage.
func checkWalk(r *rig) string {
	if n := countFocused(r.tt.View().Root()); n != 1 {
		return fmt.Sprintf("%d widgets are drawn focused", n)
	}
	it := r.sh.Manager().Current()
	if it == nil {
		return "no item has the focus"
	}
	scope := r.sh.Manager().Scope()
	if r.sh.Stack().Panel() && scope == nil {
		return "the panel is open and the focus scope is not confined to it"
	}
	if scope == nil {
		scope = r.sh
	}
	if !under(it, scope) {
		return fmt.Sprintf("%s has the focus outside the scope %T", it.FocusKey(), scope)
	}
	for p := ui.Widget(it); p != nil; p = p.Node().Parent() {
		if p.Node().Hidden || p.Node().Disabled {
			return fmt.Sprintf("%s is under a hidden or disabled %T", it.FocusKey(), p)
		}
	}
	f := it.Node().WindowFrame()
	if f.W <= 0 || f.H <= 0 {
		return fmt.Sprintf("%s has an empty frame %v", it.FocusKey(), f)
	}
	if f.Intersect(geom.XYWH(0, 0, tvtest.Width, tvtest.Height)).Empty() {
		return fmt.Sprintf("%s is outside the stage at %v", it.FocusKey(), f)
	}

	return ""
}

// under tells whether w is m or below it.
func under(w, m ui.Widget) bool {
	for p := w; p != nil; p = p.Node().Parent() {
		if p == m {
			return true
		}
	}

	return false
}

// countFocused counts the widgets of the tree that draw themselves focused.
func countFocused(w ui.Widget) int {
	n := 0
	if f, ok := w.(interface{ IsFocused() bool }); ok && f.IsFocused() {
		n++
	}
	for c := range w.Node().Children() {
		n += countFocused(c)
	}

	return n
}

var _ = tvkit.ToastTime
