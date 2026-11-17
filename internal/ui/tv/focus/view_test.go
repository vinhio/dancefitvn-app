//go:build tvos

// Adapted from jplayer-tv/internal/ui/focus/view_test.go at 781bee5.

package focus_test

import (
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/ui/tv/focus"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tile is a 100x100 focus.Item, placed by a grid.
type tile struct {
	ui.Base
	key string
}

func (t *tile) FocusKey() string   { return t.key }
func (t *tile) FocusDefault() bool { return false }
func (t *tile) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Size{W: 100, H: 100})
}

var _ focus.Item = (*tile)(nil)

// grid places its children in rows of cols tiles, 120 apart.
type grid struct {
	ui.Base
	cols int
}

func (g *grid) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	i := 0
	for w := range g.Children() {
		c.Layout(w, ui.Loose(geom.Size{W: 100, H: 100}))
		c.Place(w, geom.Pt(float32(i%g.cols)*120, float32(i/g.cols)*120))
		i++
	}

	return cs.Max
}

// viewRig is a manager over a ViewEngine bound to a tester, over a 3x2 grid.
type viewRig struct {
	tt    *ui.Tester
	eng   *focus.ViewEngine
	m     *focus.Manager
	tiles map[string]*tile
	grid  *grid
}

func newViewRig(t *testing.T) *viewRig {
	t.Helper()
	r := &viewRig{eng: focus.NewViewEngine(nil), tiles: map[string]*tile{}, grid: &grid{cols: 3}}
	r.m = focus.NewManager(r.eng, r.eng)
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		tl := &tile{key: k}
		tl.Name, tl.Focusable = k, true
		r.tiles[k] = tl
		r.grid.Add(tl)
	}
	tt, err := ui.NewTester(focus.NewRoot(r.m, r.grid), 640, 360, ui.WithDirectionalFocus())
	require.NoError(t, err)
	r.tt = tt
	r.eng.Bind(tt.View())
	t.Cleanup(func() { r.m.Close(); tt.Close() })

	return r
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestViewEngineSatisfiesEngine(t *testing.T) {
	t.Parallel()

	var e focus.Engine = focus.NewViewEngine(nil)
	var c focus.Clock = focus.NewViewEngine(nil)
	require.NotNil(t, e)
	require.NotNil(t, c)

	// Without a view nothing happens and nothing panics.
	ve := focus.NewViewEngine(nil)
	assert.Nil(t, ve.Focused())
	ve.Focus(nil)
	assert.False(t, ve.Move(focus.DirRight))
	ve.SetScope(nil)
	ve.Reveal(nil)
	assert.Empty(t, ve.Candidates(nil, nil))
	assert.False(t, ve.AfterFunc(time.Second, func() {}).Pending())
}

func TestViewEngineMoveUsesGeometry(t *testing.T) {
	t.Parallel()

	r := newViewRig(t)
	r.m.Set(r.tiles["a"], focus.SetOptions{})
	tests := []struct {
		key  keys.Key
		want string
	}{
		{keys.KeyRight, "b"},
		{keys.KeyDown, "e"},
		{keys.KeyRight, "f"},
		{keys.KeyUp, "c"},
		{keys.KeyLeft, "b"},
		{keys.KeyUp, "b"}, // at the top edge: stays
	}
	for _, tt := range tests {
		r.tt.Key(tt.key, 0)
		it := r.m.Current()
		require.NotNil(t, it)
		assert.Equal(t, tt.want, it.FocusKey(), tt.key.String())
	}
	assert.True(t, r.eng.Move(focus.DirLeft), "Move reports a move")
	assert.False(t, r.eng.Move(focus.DirLeft), "and an edge")
	assert.False(t, r.eng.Move(focus.DirUnknown))
}

func TestViewEngineBindRegistersEarlyObservers(t *testing.T) {
	t.Parallel()

	eng := focus.NewViewEngine(nil)
	var seen []string
	remove := eng.OnFocusChange(func(old, now ui.Widget) {
		name := ""
		if now != nil {
			name = now.Node().Name
		}
		seen = append(seen, name)
	})
	cancelled := false
	eng.OnFocusChange(func(old, now ui.Widget) { cancelled = true })()

	tl := &tile{key: "x"}
	tl.Name, tl.Focusable = "x", true
	root := &ui.Base{}
	root.Add(tl)
	tt, err := ui.NewTester(root, 320, 200, ui.WithDirectionalFocus())
	require.NoError(t, err)
	defer tt.Close()
	eng.Bind(tt.View())

	eng.Focus(tl)
	assert.Equal(t, []string{"x"}, seen, "the function added before Bind hears the change")
	assert.False(t, cancelled, "one removed before Bind never does")
	remove()
	eng.Focus(nil)
	assert.Equal(t, []string{"x"}, seen, "removed after Bind: silent")
}

func TestViewEngineScopeAndCandidates(t *testing.T) {
	t.Parallel()

	r := newViewRig(t)
	all := r.eng.Candidates(nil, nil)
	assert.Len(t, all, 6, "every tile, in tree order")
	assert.Equal(t, "a", all[0].Node().Name)
	assert.Equal(t, "f", all[5].Node().Name)

	inner := &ui.Base{}
	inner.Name = "inner"
	r.grid.Remove(r.tiles["e"])
	r.grid.Remove(r.tiles["f"])
	inner.Add(r.tiles["e"], r.tiles["f"])
	r.grid.Add(inner)
	r.tt.Frame()

	r.eng.SetScope(inner)
	scoped := r.eng.Candidates(nil, nil)
	require.Len(t, scoped, 2)
	assert.Equal(t, "e", scoped[0].Node().Name)
	assert.Len(t, r.eng.Candidates(nil, r.grid), 6, "within overrides the scope")
	r.eng.SetScope(nil)
	assert.Len(t, r.eng.Candidates(nil, nil), 6)
}

func TestViewEngineFocusRefusalLeavesFocus(t *testing.T) {
	t.Parallel()

	r := newViewRig(t)
	r.m.Set(r.tiles["a"], focus.SetOptions{})
	r.tiles["b"].Disabled = true

	r.eng.Focus(r.tiles["b"])

	assert.Equal(t, "a", r.m.Current().FocusKey(), "a disabled widget refuses the focus")
	r.eng.Focus(nil)
	assert.Nil(t, r.m.Current())
}

func TestViewEngineAfterFuncRunsOnTheViewsClock(t *testing.T) {
	t.Parallel()

	r := newViewRig(t)
	fired := false
	tm := r.eng.AfterFunc(100*time.Millisecond, func() { fired = true })
	assert.True(t, tm.Pending())
	r.tt.Elapse(99 * time.Millisecond)
	assert.False(t, fired)
	r.tt.Elapse(2 * time.Millisecond)
	assert.True(t, fired)
}

func TestViewEngineMoveDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	r := newViewRig(t)
	r.m.Set(r.tiles["a"], focus.SetOptions{})
	right := true
	move := func() {
		d := focus.DirRight
		if !right {
			d = focus.DirLeft
		}
		right = !right
		r.m.Move(d)
	}
	for range 4 {
		move()
	}

	assert.InDelta(t, 0, testing.AllocsPerRun(100, move), 0, "a focus move through the manager and the view")
}

func TestViewEngineScopeSetBeforeBindIsKept(t *testing.T) {
	t.Parallel()

	eng := focus.NewViewEngine(nil)
	inner := &grid{cols: 3}
	outer := &grid{cols: 3}
	for _, k := range []string{"a", "b", "c"} {
		tl := &tile{key: k}
		tl.Name, tl.Focusable = k, true
		inner.Add(tl)
	}
	for _, k := range []string{"d", "e"} {
		tl := &tile{key: k}
		tl.Name, tl.Focusable = k, true
		outer.Add(tl)
	}
	outer.Add(inner)
	eng.SetScope(inner) // before there is a view: the scope waits

	m := focus.NewManager(eng, eng)
	tt, err := ui.NewTester(focus.NewRoot(m, outer), 640, 360, ui.WithDirectionalFocus())
	require.NoError(t, err)
	t.Cleanup(func() { m.Close(); tt.Close() })
	eng.Bind(tt.View())

	var got []string
	for _, w := range eng.Candidates(nil, nil) {
		got = append(got, w.Node().Name)
	}
	assert.ElementsMatch(t, []string{"a", "b", "c"}, got, "the scope set before Bind confines the candidates")

	eng.SetScope(nil)
	got = got[:0]
	for _, w := range eng.Candidates(nil, nil) {
		got = append(got, w.Node().Name)
	}
	assert.ElementsMatch(t, []string{"a", "b", "c", "d", "e"}, got, "nil lifts the scope")
}
