//go:build tvos

package screens_test

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"

	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/nav"
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

// vecFile is internal/ui/tv/focus/testdata/screen_vectors.json: the
// prototype's moveFocus run over the rectangles of its TV layouts (P7.6).
type vecFile struct {
	Screens []struct {
		Screen string `json:"screen"`
		Rects  []struct {
			FID string  `json:"fid"`
			X   float64 `json:"x"`
			Y   float64 `json:"y"`
			W   float64 `json:"w"`
			H   float64 `json:"h"`
		} `json:"rects"`
		Cases []struct {
			From string `json:"from"`
			Dir  string `json:"dir"`
			To   string `json:"to"`
		} `json:"cases"`
	} `json:"screens"`
}

// vecScreen opens one prototype state on the real screens.
type vecScreen struct {
	id   string
	open func(t testing.TB) *rig
}

// vecDiff is one move where the app's target is not the prototype's.
type vecDiff struct {
	screen, from, dir, proto, app string
	wrongOrder                    bool
}

// vecKeys maps the vector directions to keys.
var vecKeys = map[string]keys.Key{
	"ArrowLeft": keys.KeyLeft, "ArrowRight": keys.KeyRight, "ArrowUp": keys.KeyUp, "ArrowDown": keys.KeyDown,
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// vecScreens are the states of the prototype's TV layouts that have a
// vector, in the capture's data (P7.6 step 3).
func vecScreens() []vecScreen {
	fast := func(svc func(testing.TB) *tvtest.Services, route nav.Route) func(testing.TB) *rig {
		return func(t testing.TB) *rig {
			t.Helper()
			r := openAtFast(t, svc(t), route)
			r.settle()

			return r
		}
	}
	signed := tvtest.NewSignedServices
	plain := func(testing.TB) *tvtest.Services { return tvtest.NewServices() }

	return []vecScreen{
		{"TV01", fast(plain, nav.Route{Screen: env.RouteWelcome})},
		{"TV02", fast(plain, nav.Route{Screen: env.RouteCode})},
		{"TV04", fast(signed, nav.Route{Screen: env.RouteHome})},
		{"TV05", fast(signed, nav.Route{Screen: env.RouteCourses})},
		{"TV06", fast(signed, nav.Route{Screen: env.RouteDetail, Params: "set-35"})},
		{"TV07", fast(playerState, nav.Route{Screen: env.RoutePlayer})},
		{"TV08", func(t testing.TB) *rig {
			t.Helper()
			r := fast(playerState, nav.Route{Screen: env.RoutePlayer})(t)
			r.focusKey("p2")
			require.True(t, r.press(keys.KeyEnter))
			r.settle()
			require.True(t, r.sh.Stack().Panel(), "TV08: the panel is open")

			return r
		}},
		{"TV09", fast(audioState, nav.Route{Screen: env.RouteMusic})},
		{"TV10", func(t testing.TB) *rig { t.Helper(); return inUpNext(t, upNextState(t), false) }},
		{"TV11", fast(offlineServices, nav.Route{Screen: env.RouteError})},
		{"TV12", fast(playerState, nav.Route{Screen: env.RouteSettings})},
	}
}

// openAtFast is openAt with the motion reduced and no Core Text shaping,
// but the real layout fonts: MountReal, as the boxes tests do.
func openAtFast(t testing.TB, svc env.Services, route nav.Route) *rig {
	t.Helper()
	r := openAt(t, svc, route, true)
	r.tt.View().Context().Theme().ReduceMotion = true

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestScreensFollowVectors runs every item x 4 directions of the prototype's
// TV screens through the real screens, with Tester.Key, and compares the
// target with the one the prototype's moveFocus reached (01 K9, 03 sec 7.2).
// A difference is listed, not a failure: DFV_FOCUS_DIFF_APP=<path> writes
// the list for local/focus-diff.md. A key that leaves nothing focused, or the
// focus on a widget that is no item, fails.
func TestScreensFollowVectors(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("TV05's grid takes minutes to lay out with the real fonts")
	}

	raw, err := os.ReadFile("../focus/testdata/screen_vectors.json")
	require.NoError(t, err)
	var vf vecFile
	require.NoError(t, json.Unmarshal(raw, &vf))
	byID := map[string]int{}
	for i, s := range vf.Screens {
		byID[s.Screen] = i
	}

	var mu sync.Mutex
	var diffs []vecDiff
	counts := map[string][2]int{}
	t.Run("screens", func(t *testing.T) {
		for _, sc := range vecScreens() {
			t.Run(sc.id, func(t *testing.T) {
				t.Parallel()
				i, ok := byID[sc.id]
				require.True(t, ok, "no vectors for %s", sc.id)
				v := vf.Screens[i]
				boxes := map[string]vecBox{}
				for _, r := range v.Rects {
					boxes[r.FID] = vecBox{r.X, r.Y, r.W, r.H}
				}
				var mine []vecDiff
				moves := 0
				// A move may scroll the screen: the rig is rebuilt when a start item is
				// not where the first build had it.
				fids := make([]string, len(v.Rects))
				for i, b := range v.Rects {
					fids[i] = b.FID
				}
				var r *rig
				var pristine map[string]geom.Rect
				for _, from := range v.Rects {
					if r == nil || !r.pristine(pristine) {
						r = sc.open(t)
						require.NotEmpty(t, r.key(), "%s: nothing focused after opening", sc.id)
						if pristine == nil {
							pristine = r.frames(fids)
						}
					}
					for _, c := range v.Cases {
						if c.From != from.FID {
							continue
						}
						moves++
						if !r.refocus(from.FID) {
							mine = append(mine, vecDiff{sc.id, from.FID, c.Dir, c.To, "(item missing)", true})

							continue
						}
						require.Equal(t, from.FID, r.key())
						r.press(vecKeys[c.Dir])
						got := r.key()
						require.NotEmpty(t, got, "%s %s %s: the focus was lost", sc.id, from.FID, c.Dir)
						it := r.item(got)
						require.NotNil(t, it, "%s %s %s: focus on %q is no item", sc.id, from.FID, c.Dir, got)
						assert.Empty(t, r.tt.Leaks())
						if got != c.To {
							mine = append(mine, vecDiff{sc.id, from.FID, c.Dir, c.To, got, wrongOrder(boxes, from.FID, c.To, got, c.Dir)})
						}
					}
				}
				mu.Lock()
				diffs = append(diffs, mine...)
				counts[sc.id] = [2]int{len(mine), moves}
				mu.Unlock()
				t.Logf("%s: %d/%d moves differ", sc.id, len(mine), moves)
			})
		}
	})

	total, differ, wrong := 0, 0, 0
	for _, c := range counts {
		differ += c[0]
		total += c[1]
	}
	for _, d := range diffs {
		if d.wrongOrder {
			wrong++
		}
	}
	t.Logf("app: %d/%d moves differ from the prototype, %d of them wrong order", differ, total, wrong)
	writeAppDiff(t, diffs, counts)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// item returns the focus item with the key, nil when the tree has none.
func (r *rig) item(key string) focus.Item {
	r.t.Helper()

	return findItem(r.tt.View().Root(), key)
}

// findItem walks the tree for the item with the key.
func findItem(w ui.Widget, key string) focus.Item {
	if it, ok := w.(focus.Item); ok && it.FocusKey() == key {
		return it
	}
	for c := range w.Node().Children() {
		if it := findItem(c, key); it != nil {
			return it
		}
	}

	return nil
}

// frames returns the window frames of the items with the keys; an item the
// tree lacks has no entry.
func (r *rig) frames(keys []string) map[string]geom.Rect {
	r.t.Helper()
	m := make(map[string]geom.Rect, len(keys))
	for _, k := range keys {
		if it := r.item(k); it != nil {
			m[k] = it.Node().WindowFrame()
		}
	}

	return m
}

// pristine tells that every item of want is where it was on the first build.
func (r *rig) pristine(want map[string]geom.Rect) bool {
	r.t.Helper()
	for k, f := range want {
		if it := r.item(k); it == nil || it.Node().WindowFrame() != f {
			return false
		}
	}

	return true
}

// focusKey puts the focus on the item with the key.
func (r *rig) focusKey(key string) {
	r.t.Helper()
	require.True(r.t, r.refocus(key), "no item %q", key)
}

// refocus focuses the item without scrolling; false when the tree has none.
func (r *rig) refocus(key string) bool {
	r.t.Helper()
	it := r.item(key)
	if it == nil {
		return false
	}
	r.sh.Manager().Set(it, focus.SetOptions{NoScroll: true})

	return r.key() == key
}

// vecBox is a rectangle of the prototype's layout.
type vecBox struct{ x, y, w, h float64 }

// wrongOrder tells that the app's target is outside the group of the
// prototype's: for Left/Right the row of the prototype's target, for Up/Down
// its column (overlap on the other axis); or the app stays where the
// prototype moves, or moves where the prototype stays; or lands on an item
// the prototype's layout does not have.
func wrongOrder(boxes map[string]vecBox, from, proto, app, dir string) bool {
	a, ok := boxes[app]
	if !ok {
		return true
	}
	if (app == from) != (proto == from) {
		return true
	}
	if app == from {
		return false
	}
	p := boxes[proto]
	if dir == "ArrowLeft" || dir == "ArrowRight" {
		return a.y+a.h <= p.y || a.y >= p.y+p.h
	}

	return a.x+a.w <= p.x || a.x >= p.x+p.w
}

// writeAppDiff appends the diffs to the file named by DFV_FOCUS_DIFF_APP.
func writeAppDiff(t *testing.T, diffs []vecDiff, counts map[string][2]int) {
	t.Helper()
	path := os.Getenv("DFV_FOCUS_DIFF_APP")
	if path == "" {
		return
	}
	sort.Slice(diffs, func(i, j int) bool {
		a, b := diffs[i], diffs[j]
		if a.screen != b.screen {
			return a.screen < b.screen
		}
		if a.from != b.from {
			return a.from < b.from
		}

		return a.dir < b.dir
	})
	var sb strings.Builder
	ids := make([]string, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		fmt.Fprintf(&sb, "count\t%s\t%d\t%d\n", id, counts[id][0], counts[id][1])
	}
	for _, d := range diffs {
		fmt.Fprintf(&sb, "diff\t%s\t%s\t%s\t%s\t%s\t%v\n", d.screen, d.from, d.dir, d.proto, d.app, d.wrongOrder)
	}
	require.NoError(t, os.WriteFile(path, []byte(sb.String()), 0o644))
}
