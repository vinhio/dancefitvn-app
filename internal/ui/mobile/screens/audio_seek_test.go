//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAudioWaveDrag checks K38 on both phones: a finger on the waveform takes it, the bars
// and the time follow it while the sound plays on, the player is asked nothing meanwhile,
// and the release asks for one seek; a tap still jumps; a finger past the end seeks to the
// length less 1 s.
func TestAudioWaveDrag(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		pb.Play()
		r.Settle()
		pb.ClearCalls()
		w, ok := named(r.Shell, "div.wave", 1).(*kit.Wave)
		require.True(t, ok)
		wf := w.Node().WindowFrame()
		at := func(share, dy float32) geom.Point { return geom.Pt(wf.X+wf.W*share, wf.Y+wf.H/2+dy) }
		require.Equal(t, geom.Sz(wf.W, 40), wf.Size(), "the waveform keeps its 40 DIPs")

		r.Tester.TouchDown(at(.5, 0))
		assert.True(t, w.Dragging())
		assert.Equal(t, 36, a.WaveLit(), "35 / 70 <= .5: the bars are lit to the finger")
		assert.True(t, r.Tester.Shows("01:50"), "the time of the finger: 110 of 220 s")
		r.Tester.TouchMove(at(.8, 90))
		assert.Equal(t, 57, a.WaveLit())
		assert.True(t, r.Tester.Shows("02:56"))
		pb.Tick(80) // the sound goes on
		assert.Equal(t, 57, a.WaveLit(), "the tick does not light other bars under the finger")
		assert.True(t, r.Tester.Shows("02:56"))
		assert.Empty(t, pb.Calls(), "the player is asked nothing before the release")
		r.Tester.TouchUp(at(.8, 90))
		assertSeeks(t, pb, 176)
		assert.True(t, pb.Playing())
		assert.False(t, w.Dragging())

		// the pad is 44 tall, the bar 40: 21 DIPs from its middle is on it, 30 is not
		pb.ClearCalls()
		r.Tester.ClickAt(at(.25, 21))
		assertSeeks(t, pb, 55)
		pb.ClearCalls()
		r.Tester.ClickAt(at(.25, 30))
		assert.Empty(t, pb.Calls())

		// the end of the bar: one second short of the length
		pb.ClearCalls()
		r.Tester.TouchDown(at(.5, 0))
		r.Tester.TouchMove(geom.Pt(ph.Size.W+30, at(1, 0).Y))
		r.Tester.TouchUp(geom.Pt(ph.Size.W+30, at(1, 0).Y))
		assertSeeks(t, pb, 219)
	})
}

// TestAudioWaveDragWhilePausedSeeksOnceAndStaysPaused checks the same with the sound paused.
func TestAudioWaveDragWhilePausedSeeksOnceAndStaysPaused(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, false)
		require.False(t, pb.Playing())
		w, _ := named(r.Shell, "div.wave", 1).(*kit.Wave)
		require.NotNil(t, w)
		wf := w.Node().WindowFrame()
		r.Tester.TouchDown(geom.Pt(wf.X+wf.W*.1, wf.Y+20))
		r.Tester.TouchMove(geom.Pt(wf.X+wf.W*.4, wf.Y+20))
		r.Tester.TouchUp(geom.Pt(wf.X+wf.W*.4, wf.Y+20))
		assertSeeks(t, pb, 88)
		assert.False(t, pb.Playing())
		r.Tester.Elapse(10 * time.Millisecond)
	})
}

// TestAudioWaveIncrementsOfAScreenReader checks that the waveform is a slider of "elapsed /
// total" whose increments and decrements seek by 10 s.
func TestAudioWaveIncrementsOfAScreenReader(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, false)
		n, ok := findNode(r.Tester.Semantics(), "Vị trí phát", platform.RoleSlider)
		require.True(t, ok)
		assert.Equal(t, "01:12 / 03:40", n.Value)
		assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActIncrement))
		assert.Equal(t, 82.0, pb.Pos())
		assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActDecrement))
		assert.Equal(t, 72.0, pb.Pos())
	})
}

// TestAudioMoreDeletesTheFile checks the "…" of screen 08 (K38): while the lesson's sound is
// on the device it opens sheet 14 for that lesson, as the Downloads list's does; "Xoá khỏi máy"
// deletes the file, the sound that played from it is loaded again to come from the stream
// (Reload) and plays on, and "…" goes (the lesson is streamed now); "Huỷ" deletes nothing.
func TestAudioMoreDeletesTheFile(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, false)
		pb.Play()
		r.Settle()
		more := named(r.Shell, "toast", 1)
		require.NotNil(t, more, "the lesson's sound is on the device: the button is there")
		assert.Equal(t, geom.Sz(40, 40), more.Node().WindowFrame().Size())

		tap(r, more)
		r.Tester.Elapse(400 * time.Millisecond)
		require.True(t, r.Shell.Nav().Sheet().IsOpen(), "sheet 14 opens")
		assert.True(t, r.Tester.Shows("Xoá file âm thanh?"))
		assert.True(t, r.Tester.Shows("Xoá khỏi máy"))
		assert.Empty(t, r.Fake.Dl().Deleted())

		// "Huỷ": nothing is deleted
		require.NoError(t, r.Tester.Click("Huỷ"))
		r.Tester.Elapse(400 * time.Millisecond)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Empty(t, r.Fake.Dl().Deleted())
		assert.Zero(t, pb.Count("Reload"))

		// confirm
		pb.ClearCalls()
		tap(r, named(r.Shell, "toast", 1))
		r.Tester.Elapse(400 * time.Millisecond)
		require.True(t, r.Shell.Nav().Sheet().IsOpen())
		tapKey(t, r, "delConfirm")
		r.Tester.Elapse(400 * time.Millisecond)
		assert.Equal(t, []string{"l_3503"}, r.Fake.Dl().Deleted(), "the lesson's file")
		assert.Equal(t, 1, pb.Count("Reload"), "the sound is loaded again: it came from the file")
		assert.True(t, pb.Playing(), "and plays on")
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, r.Tester.Shows("Đã xoá file âm thanh"))
		assert.Equal(t, "audio", r.Route(), "the screen stays")
		assert.Nil(t, named(r.Shell, "toast", 1), "the lesson is streamed now: no button")
		assert.True(t, r.Tester.Shows("Đang phát trực tuyến"), "the chip says so")
	})
}

// TestAudioMoreFollowsTheDownloadState checks that the "…" shows only while the lesson's
// file is on the device, as the chip does, and follows the downloads live; the timer
// button keeps its name and the top row its places while it is hidden.
func TestAudioMoreFollowsTheDownloadState(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atAudio(t, ph, false)
		back, _ := r.Dump("on").Find("back")
		moreOn, ok := r.Dump("on").Find("toast")
		require.True(t, ok)
		timer, ok := r.Dump("on").Find("toast#2")
		require.True(t, ok, "the timer is the second toast, as in the capture")

		r.Fake.Dl().Delete("l_3503")
		r.Settle()
		assert.Nil(t, named(r.Shell, "toast", 1), "the file went: the button goes")
		off := r.Dump("off")
		_, ok = off.Find("toast")
		assert.False(t, ok)
		timerOff, ok := off.Find("toast#2")
		require.True(t, ok, "the timer keeps its key")
		assert.Equal(t, timer, timerOff, "and its place")
		backOff, _ := off.Find("back")
		assert.Equal(t, back, backOff, "the top row does not move")
		assert.True(t, r.Tester.Shows("Đang phát trực tuyến"))
		_ = moreOn

		r.Fake.Dl().SetDownload("l_3503", kit.DlDone, 1)
		r.Settle()
		assert.NotNil(t, named(r.Shell, "toast", 1), "a finished download brings it back")
		r.Fake.Dl().SetDownload("l_3503", kit.DlIng, .4)
		r.Settle()
		assert.Nil(t, named(r.Shell, "toast", 1), "a download under way is not a file yet")
		// a screen reader does not find a hidden button
		_, found := findNode(r.Tester.Semantics(), "Tuỳ chọn file", platform.RoleButton)
		assert.False(t, found)
	})
}

// TestAudioRepeatButtonCyclesThreeModes checks the leftmost button (K38): each press goes off,
// repeat this lesson, the A-B loop from the position, off, with a toast of what it
// became, the badge on the button ("1", "A–B"), and what a screen reader hears; the position
// is never sought.
func TestAudioRepeatButtonCyclesThreeModes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		require.Equal(t, "", a.RepeatBadge())
		n, ok := findNode(r.Tester.Semantics(), "Lặp lại", platform.RoleButton)
		require.True(t, ok)
		assert.Equal(t, "tắt", n.Value)
		assert.Equal(t, platform.ToggledOff, n.Toggle)

		tapKey(t, r, "loop")
		assert.Equal(t, []string{"CycleRepeat"}, pb.Calls())
		assert.Equal(t, "1", a.RepeatBadge())
		assert.True(t, r.Tester.Shows("Lặp lại bài này"))
		n, _ = findNode(r.Tester.Semantics(), "Lặp lại", platform.RoleButton)
		assert.Equal(t, "lặp bài", n.Value)
		assert.Equal(t, platform.ToggledOn, n.Toggle)
		assert.False(t, pb.Loop(), "Repeat One has no A-B band")

		tapKey(t, r, "loop")
		assert.Equal(t, "A–B", a.RepeatBadge())
		assert.True(t, r.Tester.Shows("Lặp đoạn 01:12 → 01:42"), "A is the position, B 30 s on")
		n, _ = findNode(r.Tester.Semantics(), "Lặp lại", platform.RoleButton)
		assert.Equal(t, "lặp đoạn 01:12 → 01:42", n.Value)
		assert.True(t, pb.Loop())
		assert.Equal(t, 72.0, pb.Pos(), "nothing was sought")

		tapKey(t, r, "loop")
		assert.Equal(t, "", a.RepeatBadge())
		assert.True(t, r.Tester.Shows("Đã tắt lặp lại"))
		assert.False(t, pb.Loop())
		assert.Zero(t, pb.Count("Seek"))
		r.Tester.Elapse(2300 * time.Millisecond)
		r.Settle()
		assert.False(t, r.Tester.Shows("Đã tắt lặp lại"), "the toast goes after 2200 ms")
	})
}
