//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holdSplash is the state M01: the splash on show, its bar at the end.
func holdSplash(t *testing.T, ph kittest.Phone, real bool) (*screens.Splash, func() map[string]float64) {
	t.Helper()
	r := newRig(t, ph, real, shell.Options{HoldSplash: true})
	r.Settle()
	r.Tester.Elapse(2 * time.Second)
	sp, ok := r.Shell.Nav().Screen().(*screens.Splash)
	require.True(t, ok)
	boxes := func() map[string]float64 {
		lay := r.Dump("M01")
		out := map[string]float64{}
		for _, e := range lay.Elements {
			out[e.Key+".x"], out[e.Key+".y"], out[e.Key+".w"], out[e.Key+".h"] = e.X, e.Y, e.W, e.H
		}

		return out
	}

	return sp, boxes
}

// TestSplashBoxesMatchPrototype lays the splash out with the phone's real text
// engine and compares it with M01: on the iPhone every element within 1 DIP; on
// the Android phone (wider and taller) the brand and the motto stay centred, the
// bar 120 wide and 120 above the bottom, and the texts' sizes are the iPhone's.
func TestSplashBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		sp, boxes := holdSplash(t, ph, true)
		b := boxes()
		assert.InDelta(t, 1, sp.Progress(), 0, "held: the bar is at the end (F-11)")
		if ph.Size.W == 393 {
			r := newRig(t, ph, true, shell.Options{HoldSplash: true})
			r.Settle()
			r.Tester.Elapse(2 * time.Second)
			checkAgainstProto(t, "M01", r.Dump("M01"), 1, nil)

			return
		}
		w, h := float64(ph.Size.W), float64(ph.Size.H)
		assert.InDelta(t, w/2, b["div.d.acc.x"]+b["div.d.acc.w"]/2, 1, "the brand is centred")
		assert.InDelta(t, w/2, b["div.eb.x"]+b["div.eb.w"]/2, 1, "the motto is centred")
		assert.InDelta(t, 72, b["div.d.acc.h"], 0.5)
		assert.InDelta(t, 120, b["div.prog.w"], 0)
		assert.InDelta(t, h-120, b["div.prog.y"]+b["div.prog.h"], 0.01, "the bar stands 120 over the bottom")
		assert.InDelta(t, (w-120)/2, b["div.prog.x"], 0.01)
	})
}

// TestSplashBarFillsIn1200ms checks the animation `ld 1.2s linear forwards`: the
// bar starts at 45 %, is at 72.5 % half way and at 100 % after 1200 ms, linear.
func TestSplashBarFillsIn1200ms(t *testing.T) {
	if kittest.Race {
		t.Skip("motion is reduced while the view mounts under the race detector: the bar jumps to its end")
	}
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		sp := screens.NewSplash(env.New(f, locales.MustTexts()), false)
		tt := kittest.Mount(t, ph, sp)
		kittest.PinMotion(tt)
		start := tt.Now()
		assert.InDelta(t, .45, sp.Progress(), .03, "starts at 45 %%")
		tt.Elapse(600*time.Millisecond - (tt.Now() - start))
		assert.InDelta(t, .725, sp.Progress(), .04, "half way, linear")
		tt.Elapse(700 * time.Millisecond)
		assert.InDelta(t, 1, sp.Progress(), 0, "the end")
		tt.Elapse(time.Second)
		assert.InDelta(t, 1, sp.Progress(), 0, "it stays (`forwards`)")
	})
}

// TestSplashBarFrameDoesNotAllocate checks that the frames of the bar filling in
// allocate nothing, the glow and the dancer included.
func TestSplashBarFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		sp := screens.NewSplash(env.New(f, locales.MustTexts()), false)
		tt := kittest.Mount(t, ph, sp)
		kittest.PinMotion(tt)
		for range 3 {
			tt.Elapse(kittest.Frame)
		}
		before := sp.Progress()
		assert.Zero(t, testing.AllocsPerRun(30, func() { tt.Elapse(kittest.Frame) }), ph.Name)
		assert.Greater(t, sp.Progress(), before, "the bar moved while the frames were counted")
	}
}

// TestSplashShowsTheTextsOfThePrototype checks the strings of MS.splash.
func TestSplashShowsTheTextsOfThePrototype(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shell.Options{HoldSplash: true})
		r.Settle()
		for _, s := range []string{"DANCEFIT", "VIỆT NAM", "Dễ · Vui · Khoẻ", "Ứng dụng học tập dành cho huấn luyện viên"} {
			assert.True(t, r.Tester.Shows(s), s)
		}
	})
}
