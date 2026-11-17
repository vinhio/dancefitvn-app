//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestAudioBoxesMatchPrototype lays screen 08 out with the iPhone's real text engine
// and compares every element of M08 with the prototype's within 1 DIP: the top row,
// the cover, the title and the line under it, the chips, the times, the transport
// row and the last row, with their icons.
func TestAudioBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, _, _ := atAudio(t, kittest.Phones[0], true)
	checkAgainstProto(t, "M08", r.Dump("M08"), 1, nil)
}

// TestAudioOnTheWiderPhone checks the 412 wide phone against the iPhone's: every y
// of the column is the same (the parts hang from the top), the top row's button, the
// cover and the transport follow the middle or the edges of the wider window.
func TestAudioOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI, _, _ := atAudio(t, kittest.Phones[0], true)
	rA, _, _ := atAudio(t, kittest.Phones[1], true)
	a, b := rI.Dump("M08"), rA.Dump("M08")
	require.Len(t, b.Elements, len(a.Elements))
	right := map[string]bool{"toast": true, "svg.i#2": true, "cycleSpeed": true}
	middle := map[string]bool{"div.eb": true, "div.poster": true, "div.ttl": true, "s": true, "b": true, "togglePlay": true}
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		assert.InDelta(t, ea.Y, eb.Y, .6, ea.Key+": y")
		switch {
		case right[ea.Key]:
			// the right edge follows; the width is the phone's own text engine's
			assert.InDelta(t, ea.X+ea.W+19, eb.X+eb.W, .6, ea.Key+": the right edge follows the window's")
		case middle[ea.Key]:
			assert.InDelta(t, ea.X+9.5, eb.X, .6, ea.Key+": x follows the middle")
		}
	}
	d := element(t, b, "div.d")
	assert.InDelta(t, 372, d.W, .01, "the title spans the 20 DIP margins")
	assert.InDelta(t, 20, element(t, b, "div.d").X, .01)
	assert.InDelta(t, 372, element(t, b, "div.mono.muted").W, .01)
}

// TestAudioTexts checks the strings of the screen on both phones: the course, the
// line with the number, the chips, the times, the speed, the buttons of the last row.
func TestAudioTexts(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		for _, s := range []string{"Đang phát từ", "DanceFIT Set 35", "Bài 03 · File âm thanh của bài học",
			"Bụng", "Đã tải · 8,8 MB", "01:12", "03:40", "0.75x", "Hẹn giờ", "Đếm 8 nhịp", "10 bài"} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.False(t, r.Tester.Shows("Đang phát trực tuyến"), "the file is on the device")
		assert.Equal(t, 23, a.WaveLit(), "the bars 0..22 are heard at 72 of 220 s (i / 70 <= pos / dur)")

		pb.Tick(0)
		assert.Equal(t, 1, a.WaveLit(), "bar 0 is heard at 0")
		pb.Tick(220)
		assert.Equal(t, 70, a.WaveLit())
		pb.Tick(110)
		assert.Equal(t, 36, a.WaveLit(), "35 / 70 <= .5")
		assert.True(t, r.Tester.Shows("01:50"))
	})
}

// TestAudioStreamingChip checks a lesson whose file is not on the device: its chip
// says it streams.
func TestAudioStreamingChip(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atAudio(t, ph, false)
		r.Shell.PlayAudio("set-35", "l_3505", 5)
		r.Settle()
		assert.True(t, r.Tester.Shows("Bài 05 · File âm thanh của bài học"))
		assert.True(t, r.Tester.Shows("Đang phát trực tuyến"))
		assert.False(t, r.Tester.Shows("Đã tải · 8,4 MB"))
		assert.True(t, r.Tester.Shows("Trending"))
	})
}

// TestAudioChipShowsTheFilesBytes checks Q-P5-5: the chip of a downloaded lesson
// says the size of the file as it is on the device (the entry of Items), not the
// catalog's audio_size_bytes (8,8 MB for lesson 3), follows it when it changes,
// and falls back to the catalog's while the entry has no size.
func TestAudioChipShowsTheFilesBytes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("the file's size wins over the catalog's", func(t *testing.T) {
			r, _, _ := atAudio(t, ph, false)
			require.True(t, r.Tester.Shows("Đã tải · 8,8 MB"), "the catalog's, the file being 8 800 000 bytes")
			r.Fake.Dl().SetFileBytes("l_3503", 8_212_345)
			r.Tester.Elapse(kittest.Frame)
			assert.True(t, r.Tester.Shows("Đã tải · 8,2 MB"), "the bytes of the file")
			assert.False(t, r.Tester.Shows("Đã tải · 8,8 MB"))
		})
		t.Run("an entry with no size falls back to the catalog's", func(t *testing.T) {
			r, _, _ := atAudio(t, ph, false)
			r.Fake.Dl().SetFileBytes("l_3503", 8_212_345)
			r.Tester.Elapse(kittest.Frame)
			require.True(t, r.Tester.Shows("Đã tải · 8,2 MB"))
			r.Fake.Dl().SetFileBytes("l_3503", 0)
			r.Tester.Elapse(kittest.Frame)
			assert.True(t, r.Tester.Shows("Đã tải · 8,8 MB"))
		})
		t.Run("a file that arrives while the sound streams", func(t *testing.T) {
			r, _, _ := atAudio(t, ph, false)
			r.Shell.PlayAudio("set-35", "l_3505", 5)
			r.Settle()
			require.True(t, r.Tester.Shows("Đang phát trực tuyến"), "not on the device")
			r.Fake.Dl().SetDownload("l_3505", kit.DlIng, .5)
			r.Fake.Dl().FinishDownload("l_3505")
			r.Fake.Dl().SetFileBytes("l_3505", 8_111_111)
			r.Tester.Elapse(kittest.Frame)
			assert.True(t, r.Tester.Shows("Đã tải · 8,1 MB"))
			assert.False(t, r.Tester.Shows("Đang phát trực tuyến"))
			assert.False(t, r.Tester.Shows("Đã tải · 8,4 MB"), "not the catalog's")
		})
	})
}

// TestAudioPlayPauseAndSeek checks the play button, ±10 s and the bar on both
// phones: each asks the player and the screen follows what it answers.
func TestAudioPlayPauseAndSeek(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		require.False(t, pb.Playing())
		require.NoError(t, r.Tester.Click("Phát"))
		assert.Equal(t, []string{"Toggle"}, pb.Calls())
		assert.True(t, pb.Playing())
		require.NoError(t, r.Tester.Click("Tạm dừng"), "the button is the pause one now")
		assert.False(t, pb.Playing())
		pb.ClearCalls()

		require.NoError(t, r.Tester.Click("Lùi 10 giây"))
		assert.Equal(t, []string{"SeekBy:-10"}, pb.Calls())
		assert.InDelta(t, 62, pb.Pos(), .01)
		assert.True(t, r.Tester.Shows("01:02"))
		require.NoError(t, r.Tester.Click("Tới 10 giây"))
		require.NoError(t, r.Tester.Click("Tới 10 giây"))
		assert.InDelta(t, 82, pb.Pos(), .01)
		assert.True(t, r.Tester.Shows("01:22"))
		assert.Equal(t, 27, a.WaveLit(), "bars 0..26 are heard at 82 s: 26 <= 26.09")

		// a press on the waveform seeks where it fell: the middle is half
		pb.ClearCalls()
		e := element(t, r.Dump("w"), "div.mono.muted") // the times row has the bar's width
		w := float32(e.W)
		r.Tester.ClickAt(geom.Pt(float32(e.X)+w/2, float32(e.Y)-30))
		r.Settle()
		assert.Equal(t, []string{"Seek:110"}, pb.Calls())
		assert.Equal(t, 36, a.WaveLit())
		assert.True(t, r.Tester.Shows("01:50"))
	})
}

// TestAudioCycleSpeed checks the button at the end of the row: .75, 1, 1.25, .75,
// and a speed outside the cycle goes to .75 (03 section 3).
func TestAudioCycleSpeed(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atAudio(t, ph, false)
		for _, want := range []string{"1x", "1.25x", "0.75x", "1x"} {
			tapKey(t, r, "cycleSpeed")
			assert.True(t, r.Tester.Shows(want), want)
		}
		assert.Equal(t, 4, pb.Count("CycleRate"))
		pb.SetRate(1.5)
		r.Settle()
		assert.True(t, r.Tester.Shows("1.5x"))
		tapKey(t, r, "cycleSpeed")
		assert.True(t, r.Tester.Shows("0.75x"))
	})
}

// TestAudioLoopBeatAndToasts checks the loop button (on: jumps to A, no loop),
// the beat button, and the two toasts with their exact texts, gone after 2200 ms.
func TestAudioLoopBeatAndToasts(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		pb.SetAB(40, 68)
		tapKey(t, r, "loop")
		assert.Equal(t, []string{"SetLoop:true"}, pb.Calls())
		assert.InDelta(t, 40, pb.Pos(), .01, "loop on seeks A")
		assert.True(t, r.Tester.Shows("00:40"))
		assert.Equal(t, 13, a.WaveLit(), "40 / 220 x 70 = 12.7")
		pb.Tick(100)
		assert.InDelta(t, 100, pb.Pos(), .01, "a sound does not loop: the position goes on past B")
		tapKey(t, r, "loop")
		assert.True(t, pb.Count("SetLoop") == 2 && !pb.Loop())

		require.True(t, pb.Beat())
		tapKey(t, r, "beat")
		assert.False(t, pb.Beat())
		tapKey(t, r, "beat")
		assert.True(t, pb.Beat())

		tapKey(t, r, "toast")
		assert.True(t, r.Tester.Shows("Chia sẻ / Hẹn giờ / Xoá file"))
		r.Tester.Elapse(2100 * time.Millisecond)
		assert.True(t, r.Tester.Shows("Chia sẻ / Hẹn giờ / Xoá file"), "still there at 2100 ms")
		r.Tester.Elapse(300 * time.Millisecond)
		r.Settle()
		assert.False(t, r.Tester.Shows("Chia sẻ / Hẹn giờ / Xoá file"), "gone after 2200 ms")

		tapKey(t, r, "toast#2")
		assert.True(t, r.Tester.Shows("Hẹn giờ dừng: 15 / 30 / 45 phút"))
		r.Tester.Elapse(2300 * time.Millisecond)
		r.Settle()
		assert.False(t, r.Tester.Shows("Hẹn giờ dừng: 15 / 30 / 45 phút"))
	})
}

// TestAudioFollowsTheLesson checks that a lesson the player changes (the
// lock screen's skip, a lesson started from another screen) changes the texts, the
// cover's title and the count.
func TestAudioFollowsTheLesson(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, a := atAudio(t, ph, false)
		pb.Next()
		r.Settle()
		assert.True(t, r.Tester.Shows("Em Yêu Người Ta"))
		assert.True(t, r.Tester.Shows("Bài 04 · File âm thanh của bài học"))
		assert.True(t, r.Tester.Shows("04:10"))
		assert.True(t, r.Tester.Shows("00:00"))
		assert.Equal(t, 1, a.WaveLit())
	})
}

// TestAudioBackKeepsPlaying checks that Back, from the arrow and from the list
// button, shows the screen under and never tells the player Leave: the sound
// plays on (03 section 1), and that the screen no longer follows the player once
// it is gone.
func TestAudioBackKeepsPlaying(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, key := range []string{"back", "back#2"} {
			r, pb, _ := atAudio(t, ph, false)
			require.NoError(t, r.Tester.Click("Phát"))
			require.True(t, pb.Playing())
			pb.ClearCalls()
			tapKey(t, r, key)
			assert.Equal(t, "detail", r.Route(), key)
			assert.Zero(t, pb.Count("Leave"), key+": a sound goes on playing")
			assert.True(t, pb.Playing(), key)
			assert.Equal(t, player.StatePlaying, pb.State(), key)
			assert.Equal(t, player.KindAudio, pb.Kind())
			pb.Tick(100) // no screen listens now
			r.Tester.Elapse(time.Second)
		}
	})
}

// TestAudioWithoutAPlayerIsTheStub checks that services with no player give the stub
// with a back button.
func TestAudioWithoutAPlayerIsTheStub(t *testing.T) {
	t.Parallel()

	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	e := env.New(noPlayer{f}, locales.MustTexts())
	e.Act = &recorder{}
	_, ok := screens.NewAudio(e).(*screens.Stub)
	assert.True(t, ok)
}

// TestAudioScreenTickDoesNotAllocate is the gate of a tick of screen 08 while it plays
// (the 250 ms of the player: the bar, the times, the speed), and of a paint with
// nothing new, on both phones.
func TestAudioScreenTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, pb, _ := atAudio(t, ph, false)
		pb.Play()
		pos := 10.0
		step := func() {
			pos += .25
			if pos > 40 {
				pos = 10
			}
			pb.Tick(pos)
			r.Tester.Frame()
		}
		for range 200 { // every second's text is shaped once
			step()
		}
		assert.Zero(t, testing.AllocsPerRun(100, step), ph.Name+": a tick and a paint")
		steady := func() { r.Tester.Repaint() }
		steady()
		assert.Zero(t, testing.AllocsPerRun(100, steady), ph.Name+": a paint with nothing new")
	}
}
