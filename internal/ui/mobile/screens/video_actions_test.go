//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// videoParts are the widgets of the screen a test reads the state of.
type videoParts struct {
	toggle *kit.DiscButton
	dl     *kit.SideButton
	scrub  *kit.Scrub
	view   *kit.VideoView
}

// partsOf finds them in the tree.
func partsOf(t testing.TB, r *shelltest.Rig) videoParts {
	t.Helper()
	var p videoParts
	var ok bool
	p.toggle, ok = named(r.Shell, "togglePlay", 1).(*kit.DiscButton)
	require.True(t, ok)
	p.dl, ok = named(r.Shell, "dlTap:set35-3", 1).(*kit.SideButton)
	require.True(t, ok)
	p.scrub, ok = named(r.Shell, "scrub", 1).(*kit.Scrub)
	require.True(t, ok)
	p.view, ok = named(r.Shell, kit.NameVideo, 1).(*kit.VideoView)
	require.True(t, ok)

	return p
}

// TestVideoTransport checks the transport row: the seek buttons move by 10 s and
// clamp to [0, length - 1] (index.html:607), the play button toggles and shows
// pause while it plays, on both phones.
func TestVideoTransport(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		assert.Equal(t, icon.Pause, p.toggle.Icon, "it plays")

		pb.ClearCalls()
		pb.Tick(5)
		tap(r, named(r.Shell, "seek:-10", 1))
		assert.Zero(t, pb.Pos(), "5 s less 10 is 0")
		pb.Tick(215)
		tap(r, named(r.Shell, "seek:10", 1))
		assert.Equal(t, 219.0, pb.Pos(), "215 s more 10 is the length less 1")
		pb.Tick(100)
		tap(r, named(r.Shell, "seek:10", 1))
		tap(r, named(r.Shell, "seek:-10", 1))
		tap(r, named(r.Shell, "seek:-10", 1))
		assert.Equal(t, 90.0, pb.Pos())
		assert.Equal(t, []string{"SeekBy:-10", "SeekBy:10", "SeekBy:10", "SeekBy:-10", "SeekBy:-10"}, pb.Calls())

		tap(r, p.toggle)
		assert.Equal(t, player.StatePaused, pb.State())
		r.Settle()
		assert.Equal(t, icon.Play, p.toggle.Icon, "paused: the play symbol")
		tap(r, p.toggle)
		assert.Equal(t, player.StatePlaying, pb.State())
		r.Settle()
		assert.Equal(t, icon.Pause, p.toggle.Icon)
		assert.Equal(t, 2, pb.Count("Toggle"))
	})
}

// TestVideoLiveParts checks what the tick of the player moves (liveM): the time,
// the length, the fill of the bar (share of the length), the lit beat of
// floor(pos x 2) mod 8 at 120 BPM (K16), changing only when the value changed.
func TestVideoLiveParts(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		beats, ok := named(r.Shell, "div.beats", 1).(*kit.Beats)
		require.True(t, ok)
		assert.True(t, r.Tester.Shows("03:40"), "the length of 220 s")
		assert.True(t, r.Tester.Shows("01:12"), "the resume position")
		assert.InDelta(t, 72.0/220, p.scrub.Pos(), 1e-6)
		assert.Equal(t, 0, beats.Lit(), "pos 72")
		for _, c := range []struct {
			pos  float64
			time string
			lit  int
		}{{0, "00:00", 0}, {.49, "00:00", 0}, {.5, "00:00", 1}, {3.99, "00:03", 7}, {4, "00:04", 0}, {75.25, "01:15", 6}, {219, "03:39", 6}} {
			pb.Tick(c.pos)
			r.Tester.Elapse(kittest.Frame)
			assert.True(t, r.Tester.Shows(c.time), "%v: %s", c.pos, c.time)
			assert.Equal(t, c.lit, beats.Lit(), "pos %v", c.pos)
			assert.Equal(t, c.lit, pb.BeatIndex(), "the bar follows the player's BeatIndex")
			assert.InDelta(t, c.pos/220, p.scrub.Pos(), 1e-6)
		}
	})
}

// TestVideoSideColumnHoldsTheAudioFileButtonAlone checks K38: the speed, the mirror and
// the A-B loop buttons are gone from the column (the options sheet has them) and
// the audio file's button stands where it did in the prototype (M06: 333, 534),
// on both phones; no element of those names is left in the tree.
func TestVideoSideColumnHoldsTheAudioFileButtonAlone(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		assert.True(t, p.dl.On, "the audio file is there")
		for _, name := range []string{"mirror", "loop", "speed"} {
			assert.Nil(t, named(r.Shell, name, 1), name+": no side button")
		}
		assert.Nil(t, named(r.Shell, optionsOpen, 2), "the only button of the options is the top one")
		f := p.dl.Node().WindowFrame()
		assert.InDelta(t, ph.Size.W-14-46, f.X, .5)
		assert.InDelta(t, 534, f.Y, .5, "the place of the prototype's last of four")
		assert.Equal(t, geom.Sz(46, 62), f.Size())
		for _, n := range r.Tester.Semantics().Nodes {
			assert.NotContains(t, []string{"Lật gương", "Lặp A–B"}, n.Label, "the removed buttons are not read")
		}
	})
}

// TestVideoOptionsButtonOpensTheSheet checks that the "more" button opens the options
// sheet over the screen, which stays.
func TestVideoOptionsButtonOpensTheSheet(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		require.False(t, r.Shell.Nav().Sheet().IsOpen())
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		assert.True(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, r.Tester.Shows("TUỲ CHỌN PHÁT") || r.Tester.Shows("Tuỳ chọn phát"))
		assert.Equal(t, "video", r.Route())
	})
}

// TestVideoOptionsSheetDrivesWhatTheScreenShows checks that the sheet's switches still
// do what the removed side buttons did: the mirror flips the picture, the loop shows
// the band of A to B on the bar (from the position, K38), the speed changes the
// player's, with the sheet closing the way it opened.
func TestVideoOptionsSheetDrivesWhatTheScreenShows(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		p := partsOf(t, r)
		assert.True(t, p.view.Mirror, "the picture is flipped")
		on, _, _ := p.scrub.Band()
		assert.False(t, on, "no band while the loop is off")
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)

		tap(r, named(r.Shell, "mirror", 1))
		assert.False(t, pb.Mirror())
		assert.False(t, p.view.Mirror)
		tap(r, named(r.Shell, "mirror", 1))
		assert.True(t, p.view.Mirror)

		tap(r, named(r.Shell, "loop", 1))
		assert.True(t, pb.Loop())
		assert.Equal(t, 72.0, pb.Pos(), "turning it on does not seek: A is the position")
		on, a, b := p.scrub.Band()
		assert.True(t, on)
		assert.InDelta(t, 72.0/220, a, 1e-6)
		assert.InDelta(t, 102.0/220, b, 1e-6, "B is 30 s on")
		assert.True(t, r.Tester.Shows("01:12 → 01:42 · lặp liên tục"))
		tap(r, named(r.Shell, "loop", 1))
		on, _, _ = p.scrub.Band()
		assert.False(t, on)

		tap(r, named(r.Shell, "speed:1", 1))
		assert.Equal(t, 1.0, pb.Rate())
	})
}

// TestVideoOptionsSheetMutatesThePlayer checks the sheet's controls: a speed of the
// segmented control, the three switches (mirror, A-B loop, the beats), the quality
// that cycles the preference, and "Xong".
func TestVideoOptionsSheetMutatesThePlayer(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		prefs := r.Fake.FakePrefs()
		sh := r.Shell.Nav().Sheet()
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		require.True(t, sh.IsOpen())
		seg, ok := named(r.Shell, "div.segc", 1).(*kit.Segmented)
		require.True(t, ok)
		assert.Equal(t, 1, seg.Selected, "0.75x")
		sw := func(row string) *kit.Switch {
			r, ok := named(r.Shell, row, 1).(*kit.Row)
			require.True(t, ok, row)
			s, ok := r.Trailing.(*kit.Switch)
			require.True(t, ok)

			return s
		}
		assert.True(t, sw("mirror").On)
		assert.False(t, sw("loop").On)
		assert.True(t, sw("beat").On)

		pb.ClearCalls()
		tap(r, named(r.Shell, "speed:1.25", 1))
		assert.Equal(t, []string{"SetRate:1.25"}, pb.Calls())
		assert.Equal(t, 3, seg.Selected)
		assert.Equal(t, 1.25, pb.Rate())
		tap(r, named(r.Shell, "speed:0.5", 1))
		assert.Equal(t, 0.5, pb.Rate())
		assert.Equal(t, 0, seg.Selected)

		tap(r, named(r.Shell, "mirror", 1))
		assert.False(t, pb.Mirror())
		assert.False(t, sw("mirror").On)
		tap(r, named(r.Shell, "loop", 1))
		assert.True(t, pb.Loop())
		assert.True(t, sw("loop").On)
		assert.Equal(t, 72.0, pb.Pos(), "turning the loop on does not seek: A is the position (K38)")
		assert.True(t, r.Tester.Shows("01:12 → 01:42 · lặp liên tục"), "the loop's ends")
		tap(r, named(r.Shell, "beat", 1))
		assert.False(t, pb.Beat())
		assert.False(t, sw("beat").On)
		assert.Nil(t, named(r.Shell, "div.beats", 1), "the bar is gone")
		tap(r, named(r.Shell, "beat", 1))
		assert.True(t, pb.Beat())
		assert.NotNil(t, named(r.Shell, "div.beats", 1), "and back")

		// the screen under follows
		p := partsOf(t, r)
		assert.False(t, p.view.Mirror)
		on, _, _ := p.scrub.Band()
		assert.True(t, on, "the loop's band is on the bar")

		assert.True(t, r.Tester.Shows("Tự động ›"))
		for i, want := range []string{"1080p ›", "720p ›", "480p ›", "Tự động ›"} {
			tap(r, named(r.Shell, "quality", 1))
			assert.True(t, r.Tester.Shows(want), want)
			assert.Equal(t, i+1, prefs.Cycles())
		}
		assert.Equal(t, store.QualityAuto, prefs.Quality())

		require.NoError(t, r.Tester.Click("Xong"))
		assert.False(t, sh.IsOpen())
		assert.Equal(t, "video", r.Route())
	})
}

// TestVideoOptionsSheetOnAndroidHasNoQualityRow checks K21: where the quality cannot
// be chosen the row is left out and the button of "Xong" follows the switches.
func TestVideoOptionsSheetOnAndroidHasNoQualityRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, v := atVideo(t, ph, true)
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		with := r.Dump("with")
		panelWith, ok := with.Find("div.sheet")
		require.True(t, ok)
		_, ok = with.Find("quality")
		assert.True(t, ok, "the row of the quality")
		require.NoError(t, r.Tester.Click("Xong"))

		v.SetQualityRow(false)
		tap(r, named(r.Shell, optionsOpen, 1))
		r.Tester.Elapse(300 * time.Millisecond)
		without := r.Dump("without")
		_, ok = without.Find("quality")
		assert.False(t, ok, "no quality row")
		panel, ok := without.Find("div.sheet")
		require.True(t, ok)
		assert.InDelta(t, 52, panel.Y-panelWith.Y, .5, "the sheet is one row lower")
		_, ok = without.Find("beat")
		assert.True(t, ok)
	})
}

// TestVideoDownloadButton checks the audio file's button: a tap is `dlTap` of the
// lesson (P5 owns what it does), named by the lesson's key.
func TestVideoDownloadButton(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		d, ok := named(r.Shell, "dlTap:set35-3", 1).(*kit.SideButton)
		require.True(t, ok)
		assert.Equal(t, geom.Sz(46, 62), d.Node().WindowFrame().Size())
		tap(r, d)
		assert.Equal(t, 1, d.Presses())
	})
}
