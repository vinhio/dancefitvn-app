//go:build tvos

package screens_test

import (
	"image"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/shell"
	"dancefitvn-app/internal/ui/tv/tvtest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// boxTol is the tolerance of a box against the prototype's layout.json: 2 px
// (docs/plan/P7 section 5, P7.2 done-when).
const boxTol = 2

// rig is a Shell mounted in a Tester at 1920x1080.
type rig struct {
	t   testing.TB
	sh  *shell.Shell
	tt  *ui.Tester
	svc env.Services
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// texts returns the app's strings.
func texts() map[string]string { return locales.MustTexts() }

// open mounts a Shell over svc and goes to route as a root, like the app does
// after the session is read. real mounts with Core Text and the app's fonts.
func open(t testing.TB, svc env.Services, route string, real bool) *rig {
	t.Helper()

	return openAt(t, svc, nav.Route{Screen: route}, real)
}

// openFast is open with the motion reduced: the focus and the glide land at once.
// A frame of a TV screen costs the CPU rasterizer 0.1 to 1.4 s (blurred shadows
// over 1080p), so the tests that do not pin a timing turn the animations off.
func openFast(t testing.TB, svc env.Services, route string, real bool) *rig {
	t.Helper()
	r := open(t, svc, route, real)
	r.tt.View().Context().Theme().ReduceMotion = true

	return r
}

// openAt is open for a route with its params (the course id of TV06).
func openAt(t testing.TB, svc env.Services, route nav.Route, real bool) *rig {
	t.Helper()
	sh := shell.New(svc, texts())
	sh.Go(route, true)
	var tt *ui.Tester
	if real {
		tt = tvtest.MountReal(t, sh)
	} else {
		tt = tvtest.Mount(t, sh)
	}
	tvtest.PinMotion(tt)
	tt.Elapse(tvtest.Frame)

	return &rig{t: t, sh: sh, tt: tt, svc: svc}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// key is the key of the focused item, "" when nothing is focused.
func (r *rig) key() string {
	r.t.Helper()
	if it := r.sh.Manager().Current(); it != nil {
		return it.FocusKey()
	}

	return ""
}

// press sends a key (press and release) and lets the frames run.
func (r *rig) press(k keys.Key) bool {
	r.t.Helper()
	used := r.tt.Key(k, 0)
	r.tt.Elapse(tvtest.Frame)

	return used
}

// layout dumps the tester's tree.
func (r *rig) layout() layoutdump.Layout {
	r.t.Helper()

	return layoutdump.Dump(r.tt.View().Context(), r.tt.View().Root(), "T", "tv", geom.Sz(tvtest.Width, tvtest.Height), nil)
}

// near asserts that got is within tol of want on every side.
func near(t *testing.T, want, got geom.Rect, tol float32, what string) {
	t.Helper()
	assert.InDelta(t, want.X, got.X, float64(tol), "%s x: want %v got %v", what, want, got)
	assert.InDelta(t, want.Y, got.Y, float64(tol), "%s y: want %v got %v", what, want, got)
	assert.InDelta(t, want.W, got.W, float64(tol), "%s w: want %v got %v", what, want, got)
	assert.InDelta(t, want.H, got.H, float64(tol), "%s h: want %v got %v", what, want, got)
}

// scaled returns r scaled by s about its centre.
func scaled(r geom.Rect, s float32) geom.Rect {
	c := r.Center()

	return geom.XYWH(c.X-r.W*s/2, c.Y-r.H*s/2, r.W*s, r.H*s)
}

// pixels returns the picture of the tester.
func (r *rig) pixels() *image.RGBA { return r.tt.Snapshot() }

// skipUnderRace skips a test that counts allocations.
func skipUnderRace(t *testing.T) {
	t.Helper()
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
}

// must fails the test unless err is nil.
func must(t testing.TB, err error) {
	t.Helper()
	require.NoError(t, err)
}

// settle runs the reads a screen started and the frames they ask for.
func (r *rig) settle() {
	r.t.Helper()
	r.tt.RunUntilIdle()
	r.tt.Elapse(tvtest.Frame)
	r.tt.RunUntilIdle()
}

// boxes asserts that every element of the prototype's layout of state id is in
// the screen's dump within boxTol, the focused one at its scaled box.
func (r *rig) boxes(t *testing.T, id string) {
	t.Helper()
	r.boxesLoose(t, id, nil)
}

// boxesLoose is boxes with a wider tolerance for the keys of loose.
func (r *rig) boxesLoose(t *testing.T, id string, loose map[string]float32) {
	t.Helper()
	r.boxesSkip(t, id, loose, nil)
}

// boxesSkip is boxesLoose leaving out the keys skip is true for (the parts that a
// K row changes, which the test checks on its own).
func (r *rig) boxesSkip(t *testing.T, id string, loose map[string]float32, skip func(key string) bool) {
	t.Helper()
	lay := r.layout()
	for _, key := range tvtest.ProtoKeys(t, id) {
		if skip != nil && skip(key) {
			continue
		}
		e, ok := lay.Find(key)
		if !assert.True(t, ok, "%s has no element %q", id, key) {
			continue
		}
		tol := float32(boxTol)
		if l, ok := loose[key]; ok {
			tol = l
		}
		near(t, tvtest.ProtoBox(t, id, key), geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H)), tol, id+" "+key)
	}
}

// welcomeGuard is more than the time the welcome screen takes no Select after it
// appears (screens.welcomeGuard, 450 ms).
const welcomeGuard = 500 * time.Millisecond

// armed lets the welcome screen's guard of the first Select run out.
func (r *rig) armed() *rig {
	r.t.Helper()
	r.tt.Elapse(welcomeGuard)

	return r
}
