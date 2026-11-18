//go:build !tvos

package screens_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertSeeks checks that the player was asked for these seeks, in seconds, and nothing
// else (the positions of a finger on a bar are not whole seconds to the last bit).
func assertSeeks(t testing.TB, pb *shelltest.FakePlayback, want ...float64) {
	t.Helper()
	calls := pb.Calls()
	require.Len(t, calls, len(want), "%v", calls)
	for i, c := range calls {
		v, ok := strings.CutPrefix(c, "Seek:")
		require.Truef(t, ok, "%q is not a seek", c)
		got, err := strconv.ParseFloat(v, 64)
		require.NoError(t, err)
		assert.InDelta(t, want[i], got, .01, c)
	}
}

// assertCall checks that the player got this one call, with a number near v.
func assertCall(t testing.TB, pb *shelltest.FakePlayback, name string, v float64) {
	t.Helper()
	calls := pb.Calls()
	require.Len(t, calls, 1, "%v", calls)
	rest, ok := strings.CutPrefix(calls[0], name+":")
	require.Truef(t, ok, "%q is not %s", calls[0], name)
	got, err := strconv.ParseFloat(rest, 64)
	require.NoError(t, err)
	assert.InDelta(t, v, got, .01, calls[0])
}

// barAt returns the point of the position bar at share of its width, dy from its
// middle, in the window.
func barAt(sc kit.SeekBar, share, dy float32) geom.Point {
	f := sc.Node().WindowFrame()

	return geom.Pt(f.X+f.W*share, f.Y+f.H/2+dy)
}

// TestVideoScrubDragWhilePlaying checks K38 on both phones: a finger on the bar while the
// video plays takes it, the knob and the time follow it, the player is asked nothing and
// plays on, the controls do not hide, and the release asks for one seek, the video still
// playing; the controls hide 3 s after it.
func TestVideoScrubDragWhilePlaying(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		p := partsOf(t, r)
		r.Tester.Elapse(2 * time.Second)
		pb.ClearCalls()

		r.Tester.TouchDown(barAt(p.scrub, .25, 0))
		assert.True(t, p.scrub.Dragging())
		assert.InDelta(t, .25, p.scrub.Shown(), 1e-3, "the knob is under the finger at once")
		assert.True(t, r.Tester.Shows("00:55"), "the label tells the time under the finger: 55 of 220 s")
		assert.Empty(t, pb.Calls(), "the player is asked nothing while the finger is down")
		r.Tester.TouchMove(barAt(p.scrub, .5, 0))
		assert.True(t, r.Tester.Shows("01:50"))
		r.Tester.TouchMove(barAt(p.scrub, .5, 150)) // far below the bar: still the bar's
		assert.InDelta(t, .5, p.scrub.Shown(), 1e-3)

		// the controls do not hide under a finger, however long it stays
		r.Tester.Elapse(10 * time.Second)
		assert.True(t, v.Shown(), "no auto-hide while dragging")
		pb.Tick(80) // the player goes on playing
		assert.InDelta(t, .5, p.scrub.Shown(), 1e-3, "the tick does not move the knob under the finger")
		assert.True(t, r.Tester.Shows("01:50"), "nor the label")
		assert.Empty(t, pb.Calls())
		assert.True(t, pb.Playing())

		r.Tester.TouchUp(barAt(p.scrub, .5, 150))
		assertSeeks(t, pb, 110) // one seek, half of 220 s
		assert.False(t, p.scrub.Dragging())
		assert.True(t, pb.Playing(), "it plays on")
		assert.True(t, r.Tester.Shows("01:50"), "the label is the player's again")
		assert.InDelta(t, .5, p.scrub.Pos(), 1e-3)
		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown(), "2999 ms after the release")
		r.Tester.Elapse(time.Millisecond)
		assert.False(t, v.Shown(), "3000 ms after the release the controls hide")
	})
}

// TestVideoScrubDragWhilePaused checks the same while paused: one seek at the release, the
// video stays paused.
func TestVideoScrubDragWhilePaused(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.Pause()
		r.Settle()
		p := partsOf(t, r)
		pb.ClearCalls()
		r.Tester.TouchDown(barAt(p.scrub, .1, 0))
		r.Tester.TouchMove(barAt(p.scrub, .9, 0))
		r.Tester.TouchUp(barAt(p.scrub, .9, 0))
		assertSeeks(t, pb, 198)
		assert.False(t, pb.Playing())
	})
}

// TestVideoScrubDragToTheEndStopsOneSecondShort checks the end of the bar: a finger on its
// right end, or past it, seeks to the length less 1 s, not to the end, so that nothing
// ends the lesson (and its auto next) before it is played; a tap at the very end is the same.
func TestVideoScrubDragToTheEndStopsOneSecondShort(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		pb.ClearCalls()
		r.Tester.TouchDown(barAt(p.scrub, .5, 0))
		r.Tester.TouchMove(geom.Pt(ph.Size.W+50, barAt(p.scrub, 1, 0).Y))
		assert.InDelta(t, 1, p.scrub.Shown(), 1e-4, "the knob is at the end")
		r.Tester.TouchUp(geom.Pt(ph.Size.W+50, barAt(p.scrub, 1, 0).Y))
		assertSeeks(t, pb, 219)
		assert.Equal(t, 219.0, pb.Pos())

		pb.ClearCalls()
		r.Tester.ClickAt(barAt(p.scrub, .9999, 0))
		assertSeeks(t, pb, 219)
		pb.ClearCalls()
		r.Tester.ClickAt(barAt(p.scrub, 0, 0))
		assertSeeks(t, pb, 0) // the start is the start
	})
}

// TestVideoScrubTouchTargetIs44Tall checks that the bar takes a finger up to 22 DIPs above
// or below its middle (44 in all) although it is drawn 18 tall, and not one farther; the
// bar's box in the layout is the prototype's 18.
func TestVideoScrubTouchTargetIs44Tall(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		assert.InDelta(t, 18, p.scrub.Node().WindowFrame().H, 1e-3)
		pb.ClearCalls()
		for _, dy := range []float32{-20, 20} {
			r.Tester.ClickAt(barAt(p.scrub, .3, dy))
		}
		assertSeeks(t, pb, 66, 66) // 20 DIPs above and below
		pb.ClearCalls()
		r.Tester.ClickAt(barAt(p.scrub, .3, 34))
		assert.Empty(t, pb.Calls(), "34 DIPs below is on nothing of the bar's")
	})
}

// TestVideoScrubDragFromTheHiddenControls checks that a drag that begins on the bar while
// the controls are hidden still works: the finger goes sideways past the slop, the controls
// come back, the knob follows, and the release seeks once; a tap there is still the tap
// on the picture (the controls show, nothing seeks), and a finger that goes up or down first
// is no drag.
func TestVideoScrubDragFromTheHiddenControls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		p := partsOf(t, r)
		hide := func() {
			restart(r, pb)
			r.Tester.Elapse(ms3s + 300*time.Millisecond)
			require.False(t, v.Shown())
			pb.ClearCalls()
		}

		hide()
		r.Tester.TouchDown(barAt(p.scrub, .2, 0))
		assert.False(t, p.scrub.Dragging(), "a press alone is the picture's")
		r.Tester.TouchMove(barAt(p.scrub, .2, 0).Add(geom.Pt(4, 0)))
		assert.False(t, p.scrub.Dragging(), "within the 8 DIP slop")
		r.Tester.TouchMove(barAt(p.scrub, .45, 0))
		assert.True(t, p.scrub.Dragging(), "past the slop, sideways: the drag")
		assert.True(t, v.Shown(), "the controls come back")
		assert.InDelta(t, .45, p.scrub.Shown(), 1e-3)
		r.Tester.Elapse(10 * time.Second)
		assert.True(t, v.Shown(), "and stay while the finger is down")
		r.Tester.TouchUp(barAt(p.scrub, .6, 0))
		assertSeeks(t, pb, 132) // one seek, at the release: .6 of 220 s
		assert.Equal(t, 0, pb.Count("Toggle"))

		hide()
		r.Tester.ClickAt(barAt(p.scrub, .3, 0))
		assert.True(t, v.Shown(), "a tap on the bar while hidden shows the controls, as before")
		assert.Empty(t, pb.Calls(), "and seeks nothing")

		hide()
		r.Tester.TouchDown(barAt(p.scrub, .3, 0))
		r.Tester.TouchMove(barAt(p.scrub, .3, -40)) // up first: no drag
		r.Tester.TouchMove(barAt(p.scrub, .7, -40))
		assert.False(t, p.scrub.Dragging(), "horizontal must win: a finger that went up first is no drag")
		r.Tester.TouchUp(barAt(p.scrub, .7, -40))
		assert.Empty(t, pb.Calls())
	})
}

// TestVideoScrubDragPastTheEdgeStripIsNotABack checks that a drag that begins on the bar
// does not start the swipe back of the edge strip when the bar's left end is near it, and
// that the sheet's drag does not take it: the press is the bar's whatever follows.
func TestVideoScrubDragPastTheEdgeStripIsNotABack(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		pb.ClearCalls()
		f := p.scrub.Node().WindowFrame()
		r.Tester.TouchDown(geom.Pt(f.X+2, f.Y+f.H/2))
		r.Tester.TouchMove(geom.Pt(f.X+200, f.Y+f.H/2+12))
		r.Tester.TouchMove(geom.Pt(f.X+300, f.Y+f.H/2+60))
		r.Tester.TouchUp(geom.Pt(f.X+300, f.Y+f.H/2+60))
		assert.Equal(t, 1, pb.Count("Seek"), "the drag was the bar's")
		assert.Equal(t, "video", r.Route(), "and no Back")
	})
}

// TestVideoABMarkerDragMovesBAndNotThePlayhead checks the B handle of the loop's band: a
// press on it takes it over the knob, the drag moves the band's end, the release asks the
// player to move B once, and the video is not sought or paused; B never goes before A + 30 s.
func TestVideoABMarkerDragMovesBAndNotThePlayhead(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.SetLoop(true) // A = 72 s, B = 102 s (K38)
		r.Settle()
		p := partsOf(t, r)
		on, _, b := p.scrub.Band()
		require.True(t, on)
		require.InDelta(t, 102.0/220, b, 1e-6)
		pb.ClearCalls()

		r.Tester.TouchDown(barAt(p.scrub, b, 0))
		assert.True(t, p.scrub.BDragging())
		assert.False(t, p.scrub.Dragging(), "not the playhead")
		r.Tester.TouchMove(barAt(p.scrub, .7, 0))
		r.Tester.TouchUp(barAt(p.scrub, .7, 0))
		assertCall(t, pb, "SetB", 154) // B moved to .7 of 220 s, nothing else
		_, _, b = p.scrub.Band()
		assert.InDelta(t, 154.0/220, b, 1e-6)
		assert.Equal(t, 72.0, pb.Pos(), "the video was not sought")
		assert.True(t, pb.Playing())

		// before A + 30 s: kept
		pb.ClearCalls()
		r.Tester.TouchDown(barAt(p.scrub, b, 0))
		r.Tester.TouchMove(barAt(p.scrub, .1, 0))
		r.Tester.TouchUp(barAt(p.scrub, .1, 0))
		_, _, b = p.scrub.Band()
		assert.InDelta(t, 102.0/220, b, 1e-6, "A + 30 s at the least")

		// while the loop is off there is no handle: a press there is the playhead's
		pb.SetLoop(false)
		r.Settle()
		pb.ClearCalls()
		r.Tester.ClickAt(barAt(p.scrub, 102.0/220, 0))
		assertSeeks(t, pb, 102)
	})
}

// TestVideoOptionsSheetMovesB checks the two buttons under "Lặp đoạn A–B": absent while the
// loop is off, they move B by 30 s, kept between A + 30 s and the end, and the row's text
// shows the ends; "Xong" is where it was.
func TestVideoOptionsSheetMovesB(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		assert.Nil(t, named(r.Shell, "loopB:+30", 1), "no buttons while the loop is off")
		panelOff := panelOf(r)

		tap(r, named(r.Shell, "loop", 1)) // A = 72, B = 102
		plus, minus := named(r.Shell, "loopB:+30", 1), named(r.Shell, "loopB:-30", 1)
		require.NotNil(t, plus)
		require.NotNil(t, minus)
		assert.True(t, r.Tester.Shows("−30s"))
		assert.True(t, r.Tester.Shows("+30s"))
		assert.GreaterOrEqual(t, plus.Node().WindowFrame().H, float32(44), "a finger's size")
		assert.True(t, r.Tester.Shows("01:12 → 01:42 · lặp liên tục"))
		assert.InDelta(t, 60, panelOff.Y-panelOf(r).Y, .5, "the sheet is 60 taller, resting on the bottom: the row of buttons is 44 and 8 around")

		pb.ClearCalls()
		tap(r, plus)
		assertCall(t, pb, "SetB", 132)
		assert.True(t, r.Tester.Shows("01:12 → 02:12 · lặp liên tục"))
		tap(r, plus)
		tap(r, plus)
		tap(r, plus)
		assert.Equal(t, 220.0, pb.B(), "kept at the end")
		assert.True(t, r.Tester.Shows("01:12 → 03:40 · lặp liên tục"))
		for range 8 {
			tap(r, minus)
		}
		assert.Equal(t, 102.0, pb.B(), "kept at A + 30 s")
		assert.Equal(t, 72.0, pb.Pos(), "nothing seeked")

		// the speech of the buttons
		sem := r.Tester.Semantics()
		for _, l := range []string{"Lùi điểm B 30 giây", "Tới điểm B 30 giây"} {
			n, ok := findNode(sem, l, platform.RoleButton)
			require.Truef(t, ok, "%q", l)
			assert.True(t, n.Actions.Has(platform.ActPress))
		}
		tap(r, named(r.Shell, "loop", 1)) // off: gone again
		assert.Nil(t, named(r.Shell, "loopB:+30", 1))
	})
}

// TestVideoSeekIncrementsOfAScreenReader checks the adjustable actions of the bar: an
// increment and a decrement ask for +10 s and -10 s (a seek by hand, which the player keeps
// one second short of the end), and the value is "elapsed / total".
func TestVideoSeekIncrementsOfAScreenReader(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.ClearCalls()
		n, ok := findNode(r.Tester.Semantics(), "Vị trí phát", platform.RoleSlider)
		require.True(t, ok)
		assert.Equal(t, "01:12 / 03:40", n.Value)
		assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActIncrement))
		assert.Equal(t, 82.0, pb.Pos())
		assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActDecrement))
		assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActDecrement))
		assert.Equal(t, 62.0, pb.Pos())
		n, _ = findNode(r.Tester.Semantics(), "Vị trí phát", platform.RoleSlider)
		assert.Equal(t, "01:02 / 03:40", n.Value)
	})
}

// TestVideoStateStillPlaysAfterADragSeek checks the Fake's play state is untouched by a
// drag in every state of the player (a load too): a drag of a loading video seeks once.
func TestVideoStateStillPlaysAfterADragSeek(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		pb.SetState(player.StateLoading)
		r.Settle()
		p := partsOf(t, r)
		pb.ClearCalls()
		r.Tester.TouchDown(barAt(p.scrub, .2, 0))
		r.Tester.TouchMove(barAt(p.scrub, .3, 0))
		r.Tester.TouchUp(barAt(p.scrub, .3, 0))
		assertSeeks(t, pb, 66)
		assert.Equal(t, player.StateLoading, pb.State())
	})
}

// panelOf returns the box of the options sheet's panel.
func panelOf(r *shelltest.Rig) layoutdump.Element {
	e, _ := r.Dump("panel").Find("div.sheet")

	return e
}
