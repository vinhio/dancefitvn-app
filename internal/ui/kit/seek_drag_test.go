package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seekLog records what a position bar tells.
type seekLog struct {
	drags []float32
	seeks []float32
	ends  []bool
	bSets []float32
}

// wireScrub connects a Scrub to a log.
func wireScrub(s *kit.Scrub) *seekLog {
	l := &seekLog{}
	s.OnDrag = func(v float32) { l.drags = append(l.drags, v) }
	s.OnSeek = func(v float32) { l.seeks = append(l.seeks, v) }
	s.OnDragEnd = func(moved bool) { l.ends = append(l.ends, moved) }
	s.OnBSet = func(v float32) { l.bSets = append(l.bSets, v) }

	return l
}

// TestScrubDragFollowsTheFingerAndSeeksOnce checks K38: a press takes the bar at once (the
// knob and the fill go under the finger, OnDrag says where), every move follows it, and
// the release asks for the position once; the player's SetPos does not move the bar
// meanwhile, and does again afterwards.
func TestScrubDragFollowsTheFingerAndSeeksOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		log := wireScrub(s)
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)
		at := func(share float32) geom.Point { return geom.Pt(f.X+f.W*share, f.Y+f.H/2) }
		s.SetPos(.1)

		tt.TouchDown(at(.25))
		require.True(t, s.Dragging())
		assert.InDelta(t, .25, s.Shown(), 1e-4, "the knob is under the finger at once")
		tt.TouchMove(at(.4))
		tt.TouchMove(at(.6))
		tt.TouchMove(at(.6)) // the same place: nothing new to say
		assert.InDelta(t, .6, s.Shown(), 1e-4)
		assert.InDelta(t, .1, s.Pos(), 1e-6, "the player's position is kept apart")
		s.SetPos(.12) // a tick of the player meanwhile
		assert.InDelta(t, .6, s.Shown(), 1e-4, "it does not move the bar under the finger")
		assert.Empty(t, log.seeks, "no seek before the release")
		assert.Equal(t, []float32{.25, .4, .6}, roundShares(log.drags))

		tt.TouchUp(at(.6))
		assert.False(t, s.Dragging())
		require.Len(t, log.seeks, 1, "one seek, at the release")
		assert.InDelta(t, .6, log.seeks[0], 1e-3)
		assert.Equal(t, []bool{true}, log.ends, "the finger went somewhere")
		s.SetPos(.61)
		assert.InDelta(t, .61, s.Shown(), 1e-4, "the player moves it again")
	})
}

// TestScrubTapIsADragThatDidNotMove checks that a tap still jumps: one seek at the release
// at the tapped place, and the drag says it did not move.
func TestScrubTapIsADragThatDidNotMove(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		log := wireScrub(s)
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)

		tt.ClickAt(geom.Pt(f.X+f.W*.75, f.Y+f.H/2))
		require.Len(t, log.seeks, 1)
		assert.InDelta(t, .75, log.seeks[0], 1e-3)
		assert.Equal(t, []bool{false}, log.ends)
		tt.ClickAt(geom.Pt(f.X, f.Y+f.H/2))
		assert.InDelta(t, 0, log.seeks[1], 1e-3)
		tt.ClickAt(geom.Pt(f.X+f.W-.2, f.Y+f.H/2))
		assert.InDelta(t, 1, log.seeks[2], 1e-2)
	})
}

// TestSeekPadIsA44TallTouchTarget checks that the pad grows the target to 44 DIPs around
// the bar without changing the bar's own 18, so a press 13 above or below the bar's
// edge is on it, and one 23 away is not.
func TestSeekPadIsA44TallTouchTarget(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		log := wireScrub(s)
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)
		pf := frameOf(pad)
		assert.Equal(t, geom.Sz(353, 18), f.Size(), "the bar keeps the prototype's size")
		assert.InDelta(t, 44, pf.H, 1e-3)
		assert.InDelta(t, f.X, pf.X, 1e-3)
		assert.InDelta(t, f.W, pf.W, 1e-3)
		assert.InDelta(t, f.Y+f.H/2, pf.Y+pf.H/2, 1e-3, "centered on the bar")

		tt.ClickAt(geom.Pt(f.X+100, f.Y-12))
		tt.ClickAt(geom.Pt(f.X+100, f.Y+f.H+12))
		assert.Len(t, log.seeks, 2, "12 DIPs outside the bar still hit")
		tt.ClickAt(geom.Pt(f.X+100, f.Y-24))
		assert.Len(t, log.seeks, 2, "24 DIPs outside does not")
	})
}

// TestScrubDragPastTheEndsClampsAndCancelDoesNotSeek checks that a finger beyond the
// bar's ends is at 0 or 1, and that a touch the system cancels gives the drag up: no
// seek, the bar back at the player's position.
func TestScrubDragPastTheEndsClampsAndCancelDoesNotSeek(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		log := wireScrub(s)
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)
		s.SetPos(.3)

		tt.TouchDown(geom.Pt(f.X+f.W*.5, f.Y+9))
		tt.TouchMove(geom.Pt(f.X+f.W+80, f.Y+300)) // far past the end and far below: still the bar's
		assert.InDelta(t, 1, s.Shown(), 1e-4)
		tt.TouchMove(geom.Pt(f.X-80, f.Y-300))
		assert.InDelta(t, 0, s.Shown(), 1e-4)
		tt.TouchUp(geom.Pt(f.X-80, f.Y-300))
		require.Len(t, log.seeks, 1)
		assert.InDelta(t, 0, log.seeks[0], 1e-4)

		log.seeks = nil
		s.SetPos(.3)
		tt.TouchDown(geom.Pt(f.X+f.W*.5, f.Y+9))
		tt.TouchMove(geom.Pt(f.X+f.W*.9, f.Y+9))
		require.InDelta(t, .9, s.Shown(), 1e-3)
		pad.End(false) // what a PointerCancel does
		assert.False(t, s.Dragging())
		assert.Empty(t, log.seeks, "a cancelled touch asks for nothing")
		assert.InDelta(t, .3, s.Shown(), 1e-4, "the bar is where the player says again")
		assert.Equal(t, []bool{true, true}, log.ends, "each drag ended once")
	})
}

// TestScrubKnobGrowsWhileDragged checks the knob is larger under a finger (1.25 times:
// 14 to 17.5) and back at rest, in pixels, once the 200 ms have gone by.
func TestScrubKnobGrowsWhileDragged(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)
		s.SetPos(.5)
		tt.Frame()
		x := f.X + f.W/2
		// the knob at rest is 14 high from 2 to 16: the row at y+1 is clear of it, at y+17 too
		assert.NotEqual(t, uint8(255), vvPixel(tt, geom.Pt(x, f.Y+1.2)).R, "at rest: nothing at the very top")
		tt.TouchDown(geom.Pt(x, f.Y+9))
		tt.Elapse(300 * time.Millisecond)
		tt.Frame()
		assert.Equal(t, uint8(255), vvPixel(tt, geom.Pt(x, f.Y+1.2)).R, "dragged: the knob reaches the top of the bar")
		tt.TouchUp(geom.Pt(x, f.Y+9))
		tt.Elapse(300 * time.Millisecond)
		tt.Frame()
		assert.NotEqual(t, uint8(255), vvPixel(tt, geom.Pt(x, f.Y+1.2)).R, "at rest again")
	})
}

// roundShares rounds shares to 3 places, to compare them.
func roundShares(in []float32) []float32 {
	out := make([]float32, len(in))
	for i, v := range in {
		out[i] = float32(int(v*1000+.5)) / 1000
	}

	return out
}

// mountPadStage mounts the bar and its pad on a stage.
func mountPadStage(t testing.TB, ph kittest.Phone, bar, pad ui.Widget) *ui.Tester {
	t.Helper()
	root := &padStage{bar: bar, pad: pad.(*kit.SeekPad)}
	root.Add(bar, pad)

	return kittest.Mount(t, ph, root)
}

// padStage lays a bar out at (20, 700), 353 wide, and its pad over it.
type padStage struct {
	ui.Base
	bar ui.Widget
	pad *kit.SeekPad
}

// Layout places the bar as a screen does, then the pad.
func (p *padStage) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(p.bar, ui.Constraints{Min: geom.Sz(353, 0), Max: geom.Sz(353, ui.Unbounded)})
	c.Place(p.bar, geom.Pt(20, 700))
	p.pad.Place(c)

	return cs.Max
}

// TestWaveDragLightsTheBarsUnderTheFingerAndSeeksOnce checks the same for the waveform:
// the bars light to the finger while it drags, the tick of the player does not
// change them, and the release asks once.
func TestWaveDragLightsTheBarsUnderTheFingerAndSeeksOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		w := kit.NewWave("")
		var drags, seeks []float32
		var ends []bool
		w.OnDrag = func(v float32) { drags = append(drags, v) }
		w.OnSeek = func(v float32) { seeks = append(seeks, v) }
		w.OnDragEnd = func(moved bool) { ends = append(ends, moved) }
		pad := kit.NewSeekPad(w)
		tt := mountPadStage(t, ph, w, pad)
		f := frameOf(w)
		w.SetPos(.1)
		require.Equal(t, 8, w.Lit(), "7 / 70 <= .1")

		tt.TouchDown(geom.Pt(f.X+f.W*.5, f.Y+f.H/2))
		assert.Equal(t, 36, w.Lit(), "35 / 70 <= .5: the bars are lit to the finger")
		tt.TouchMove(geom.Pt(f.X+f.W*.8, f.Y+f.H/2))
		assert.Equal(t, 57, w.Lit())
		w.SetPos(.11)
		assert.Equal(t, 57, w.Lit(), "a tick of the player does not change them")
		assert.Empty(t, seeks)
		tt.Frame()
		tt.TouchUp(geom.Pt(f.X+f.W*.8, f.Y+f.H/2))
		require.Len(t, seeks, 1)
		assert.InDelta(t, .8, seeks[0], 1e-3)
		assert.Equal(t, []bool{true}, ends)
		assert.Len(t, drags, 2)
		w.SetPos(.8)
		assert.Equal(t, 57, w.Lit())
		tt.ClickAt(geom.Pt(f.X+f.W*.2, f.Y+f.H/2))
		assert.Len(t, seeks, 2, "a tap still jumps")
		assert.InDelta(t, .2, seeks[1], 1e-3)
		assert.Equal(t, []bool{true, false}, ends)

		// the pad is 44 tall around the bar's 40
		pf := frameOf(pad)
		assert.InDelta(t, 44, pf.H, 1e-3)
		assert.Equal(t, geom.Sz(353, 40), f.Size())
	})
}

// TestScrubBMarkerMovesBAndNotThePlayhead checks the B handle of the A-B band (K38): a
// press within 22 DIPs of B takes it, even where the playhead's knob is; a drag follows
// the finger and never goes before the least B; the release asks for B once; nothing
// seeks and the playhead stays. A press away from B is the playhead's.
func TestScrubBMarkerMovesBAndNotThePlayhead(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		s := kit.NewScrub("")
		log := wireScrub(s)
		pad := kit.NewSeekPad(s)
		tt := mountPadStage(t, ph, s, pad)
		f := frameOf(s)
		at := func(share float32) geom.Point { return geom.Pt(f.X+f.W*share, f.Y+9) }
		s.SetPos(.5) // the knob stands on B: B wins
		s.SetBand(true, .2, .5)
		s.SetBMin(.35)

		tt.TouchDown(at(.5))
		assert.True(t, s.BDragging())
		assert.False(t, s.Dragging(), "not the playhead")
		tt.TouchMove(at(.7))
		tt.Frame()
		_, _, b := s.Band()
		assert.InDelta(t, .5, b, 1e-6, "the band is the screen's until the release")
		tt.TouchMove(at(.1)) // before the least B
		tt.TouchUp(at(.1))
		assert.False(t, s.BDragging())
		require.Len(t, log.bSets, 1)
		assert.InDelta(t, .35, log.bSets[0], 1e-4, "kept at A + 30 s")
		assert.Empty(t, log.seeks, "dragging B does not seek")
		assert.Empty(t, log.drags, "and does not say a playhead drag")
		assert.InDelta(t, .5, s.Pos(), 1e-6)

		// a press away from B, beyond the 22 DIPs, is the playhead's
		s.SetBand(true, .2, .5)
		tt.ClickAt(at(.9))
		assert.Empty(t, log.bSets[1:])
		require.Len(t, log.seeks, 1)
		assert.InDelta(t, .9, log.seeks[0], 1e-3)

		// 15 DIPs from B is still B (the target is 44 wide)
		tt.TouchDown(geom.Pt(f.X+f.W*.5+15, f.Y+9))
		assert.True(t, s.BDragging())
		tt.TouchUp(geom.Pt(f.X+f.W*.5+15, f.Y+9))

		// no band, no handle
		s.SetBand(false, 0, 0)
		tt.ClickAt(at(.5))
		assert.Len(t, log.seeks, 2)
	})
}

// TestSeekDragDoesNotAllocate checks that the frames of a drag allocate nothing: the
// moves of the finger, the repaints and the knob's growth.
func TestSeekDragDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	s := kit.NewScrub("")
	w := kit.NewWave("")
	sp, wp := kit.NewSeekPad(s), kit.NewSeekPad(w)
	s.OnDrag = func(float32) {}
	w.OnDrag = func(float32) {}
	tt := mountPadStage(t, kittest.Phones[0], s, sp)
	f := frameOf(s)
	s.SetBand(true, .2, .5)
	tt.TouchDown(geom.Pt(f.X+f.W*.25, f.Y+9))
	x := float32(.25)
	step := func() {
		x += .004
		if x > .95 {
			x = .25
		}
		tt.TouchMove(geom.Pt(f.X+f.W*x, f.Y+9))
		s.SetPos(x / 2) // the player's tick too
		tt.Frame()
	}
	for range 20 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step), "the playhead's drag")
	tt.TouchUp(geom.Pt(f.X+f.W*x, f.Y+9))

	// the waveform, driven through its pad
	wt := mountPadStage(t, kittest.Phones[0], w, wp)
	wf := frameOf(w)
	wt.TouchDown(geom.Pt(wf.X+wf.W*.25, wf.Y+20))
	y := float32(.25)
	wstep := func() {
		y += .004
		if y > .95 {
			y = .25
		}
		wt.TouchMove(geom.Pt(wf.X+wf.W*y, wf.Y+20))
		w.SetPos(y / 2)
		wt.Frame()
	}
	for range 20 {
		wstep()
	}
	assert.Zero(t, testing.AllocsPerRun(100, wstep), "the waveform's drag")
}
