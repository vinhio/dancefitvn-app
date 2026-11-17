//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/np/alloc_spec_test.go at 49de3c2
// (the steady loop that warms a rig up, then measures one tick and its paint)
// and from video_frame_test.go's BenchmarkVideoScreenFrame in this package.

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// BenchmarkAudioScreenTick is a tick of screen 08 playing (the bar, the
// times, the speed) and its paint, on both phones (07 section 2: the position
// update).
func BenchmarkAudioScreenTick(b *testing.B) {
	for _, ph := range kittest.Phones {
		b.Run(ph.Name, func(b *testing.B) {
			r, pb, _ := atAudio(b, ph, false)
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
			for range 200 {
				step()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				step()
			}
		})
	}
}

// BenchmarkDoneCountdownTick is one second of the end-of-lesson countdown of
// screen 10 and the paint of its text, on both phones.
func BenchmarkDoneCountdownTick(b *testing.B) {
	for _, ph := range kittest.Phones {
		b.Run(ph.Name, func(b *testing.B) {
			r, _, d := atDone(b, ph, false)
			r.Fake.FakePrefs().SetAutoNext(false) // 0 asks, nothing plays: the screen stays
			step := func() {
				r.Tester.Elapse(second)
				if d.Seconds() == 0 {
					d.Restart()
				}
			}
			for range 3 * doneSeconds {
				step()
			}
			require.Equal(b, env.RouteDone, r.Route())
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				step()
			}
		})
	}
}
