//go:build !tvos

// Pattern from internal/ui/tv/screens/player_cover_test.go (K47 on the TV: the
// darkened cover and the ring until the first frame, the cross-fade, the picture
// lost), on the two phones.

package screens_test

import (
	"errors"
	"image/color"
	"strings"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// coverPath is the end of the course's cover address in the mock, set-35's (the lesson
// plays with it): no real picture is behind it, the Fake answers with a synthetic one.
const coverPath = "/covers/set-35.jpg"

// The places the tests read, in DIPs on both phones (393 and 412 wide, 852 and 892
// tall): the ring is 48 across with a line of 5 in the middle of the window, the
// wordmark is 24 above it.
const (
	coverLeftX, coverY = 60, 200 // inside the cover, off every control, left of the middle
	ringSide           = 48
	ringLine           = 5
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// twoToneCover is a 16 by 8 picture, red on its left half and green on its right: an
// aspect-fill of it into a phone's window crops the sides about the middle, so the red
// and the green meet at half the width.
func twoToneCover(tb testing.TB) *scene.Image {
	tb.Helper()
	img, err := scene.NewImage(16, 8)
	require.NoError(tb, err)
	for y := range 8 {
		for x := range 16 {
			o := y*img.Stride + x*4
			if x < 8 {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = 255, 0, 0, 255
			} else {
				img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3] = 0, 255, 0, 255
			}
		}
	}

	return img
}

// atVideoCover is atVideo with the course's cover answered by the Fake as img (nil: it
// is refused, as the mock's covers are). fast turns the motion off.
func atVideoCover(tb testing.TB, ph kittest.Phone, img *scene.Image, fast bool) (*shelltest.Rig, *shelltest.FakePlayback, *screens.Video, *shelltest.Fake) {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	r := shelltest.MountRig(tb, ph, false, f, shell.Options{})
	if fast {
		r.Tester.View().Context().Theme().ReduceMotion = true
	}
	r.Boot()
	r.Shell.Detail("set-35")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	// the detail's hero asked for the course's cover: that is the address the lesson has
	var url string
	for _, c := range f.CoverCalls() {
		if strings.HasSuffix(c, coverPath) {
			url = c
		}
	}
	require.NotEmpty(tb, url, "the course's cover address")
	f.SetCover(url, img)
	r.Shell.PlayVideo("set-35", "l_3503", 3)
	r.Settle()
	require.Equal(tb, env.RouteVideo, r.Route())
	pb := f.FakePlayback()
	require.NotNil(tb, pb)
	require.Equal(tb, url, pb.Lesson().CoverURL, "a lesson has the course's cover")
	v, ok := r.Shell.Nav().Screen().(*screens.Video)
	require.True(tb, ok)
	r.Tester.Elapse(kittest.Frame)

	return r, pb, v, f
}

// lit reports whether the window's box has a pixel of the brand's yellow, however
// dim: both of R and G well above B and above the dark of a darkened cover.
func lit(r *shelltest.Rig, box geom.Rect) bool {
	snap := r.Tester.Snapshot()
	for y := int(box.Y); y < int(box.Y+box.H); y++ {
		for x := int(box.X); x < int(box.X+box.W); x++ {
			if c := snap.RGBAAt(x, y); c.R > 90 && c.G > 70 && c.B < c.G/2 {
				return true
			}
		}
	}

	return false
}

// logoBox is the window's box the wordmark stands in: above the ring, 24 apart, 32 tall,
// well inside the width.
func logoBox(ph kittest.Phone) geom.Rect {
	mid := ph.Size.H / 2
	return geom.XYWH(ph.Size.W/2-70, mid-ringSide/2-24-32-2, 140, 36)
}

// nearPx asserts that got is want to within 1 per channel (the scrim's .5 of 255).
func nearPx(t testing.TB, want, got color.RGBA, msg string) {
	t.Helper()
	diff := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	assert.LessOrEqual(t, diff(want.R, got.R), 1, msg+" (R)")
	assert.LessOrEqual(t, diff(want.G, got.G), 1, msg+" (G)")
	assert.LessOrEqual(t, diff(want.B, got.B), 1, msg+" (B)")
	assert.Equal(t, uint8(255), got.A, msg+" (opaque)")
}

// ringPixels is the window's pixels of the ring's box, to see the arc move.
func ringPixels(r *shelltest.Rig, ph kittest.Phone) []color.RGBA {
	snap := r.Tester.Snapshot()
	cx, cy := int(ph.Size.W)/2, int(ph.Size.H)/2
	var px []color.RGBA
	for y := cy - ringSide/2; y < cy+ringSide/2; y++ {
		for x := cx - ringSide/2; x < cx+ringSide/2; x++ {
			px = append(px, snap.RGBAAt(x, y))
		}
	}

	return px
}

// ringTop is a pixel on the ring's arc with the motion off: the top of the circle.
func ringTop(ph kittest.Phone) geom.Point {
	return geom.Pt(ph.Size.W/2, ph.Size.H/2-ringSide/2+ringLine/2)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestVideoLoadingShowsTheDarkenedCover (K47): until the first frame the picture is the
// course's cover, filled and centred, under a black scrim at .5 (red and green become
// 128), with the ring in --pr over it and no wordmark, and no stage. The mirror is on
// and does not flip it.
func TestVideoLoadingShowsTheDarkenedCover(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v, f := atVideoCover(t, ph, twoToneCover(t), true)
		assert.Contains(t, f.CoverCalls(), pb.Lesson().CoverURL, "the cover is asked for")
		require.True(t, pb.Mirror(), "the prototype starts with the mirror on")
		assert.True(t, v.Covered())
		require.NotNil(t, v.Cover())

		w := ph.Size.W
		nearPx(t, color.RGBA{R: 128, A: 255}, pixelAt(r, geom.Pt(coverLeftX, coverY)), "the red half stays on the left, darkened: not mirrored")
		nearPx(t, color.RGBA{G: 128, A: 255}, pixelAt(r, geom.Pt(w-60, coverY)), "the green half on the right")
		top := pixelAt(r, ringTop(ph))
		assert.InDelta(t, 255, float64(top.R), 6, "the ring's arc is --pr")
		assert.InDelta(t, 208, float64(top.G), 6)
		assert.False(t, lit(r, logoBox(ph)), "a cover is there: no wordmark")
	})
}

// TestVideoLoadingWithoutACoverShowsTheWordmark (K47): a cover that is refused (offline,
// the mock's), a lesson with no address, a cover not loaded yet and a load that failed
// all give plain black with the dimmed wordmark above the ring, never the old stage.
func TestVideoLoadingWithoutACoverShowsTheWordmark(t *testing.T) {
	t.Parallel()

	plain := color.RGBA{A: 255}
	check := func(t *testing.T, r *shelltest.Rig, ph kittest.Phone, msg string) {
		t.Helper()
		for _, at := range []geom.Point{geom.Pt(coverLeftX, coverY), geom.Pt(ph.Size.W-60, coverY), geom.Pt(60, 500), geom.Pt(ph.Size.W-50, 520)} {
			assert.Equal(t, plain, pixelAt(r, at), msg+": the box is plain black")
		}
		assert.True(t, lit(r, logoBox(ph)), msg+": the wordmark stands above the ring")
		top := pixelAt(r, ringTop(ph))
		assert.InDelta(t, 255, float64(top.R), 6, msg+": the ring")
	}

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("refused", func(t *testing.T) {
			t.Parallel()
			r, _, v, _ := atVideoCover(t, ph, nil, true)
			assert.Nil(t, v.Cover())
			check(t, r, ph, "refused")
		})

		t.Run("no url", func(t *testing.T) {
			t.Parallel()
			r, pb, v, f := atVideoCover(t, ph, twoToneCover(t), true)
			require.NotNil(t, v.Cover())
			l := pb.Lesson()
			l.ID, l.CoverURL = "l_none", ""
			before := len(f.CoverCalls())
			pb.PlayVideo(l, []player.Lesson{l})
			r.Settle()
			assert.Len(t, f.CoverCalls(), before, "nothing to ask for")
			assert.Nil(t, v.Cover(), "the picture of the lesson before is gone")
			check(t, r, ph, "no url")
		})

		t.Run("failed load", func(t *testing.T) {
			t.Parallel()
			r, pb, _, _ := atVideoCover(t, ph, nil, true)
			pb.Fail(errors.New("synthetic: stream refused"))
			r.Settle()
			check(t, r, ph, "failed")
		})
	})
}

// TestVideoCoverFollowsTheLesson (K47): another lesson with another cover drops the
// first one's picture until its own is there.
func TestVideoCoverFollowsTheLesson(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v, f := atVideoCover(t, ph, twoToneCover(t), true)
		require.NotNil(t, v.Cover())
		l := pb.Lesson()
		l.ID, l.CoverURL = "l_next", "mock-media://covers/other.jpg" // the Fake knows no picture of it
		pb.PlayVideo(l, []player.Lesson{l})
		r.Settle()
		assert.Contains(t, f.CoverCalls(), l.CoverURL)
		assert.Nil(t, v.Cover())
		assert.Equal(t, color.RGBA{A: 255}, pixelAt(r, geom.Pt(coverLeftX, coverY)), "plain until its own is there")
	})
}

// TestVideoFirstFrameFadesTheLookOut (K47): the first frame cross-fades the cover and the
// ring out over 250 ms, leaving the picture; the picture going away (a reload) brings
// them back at once.
func TestVideoFirstFrameFadesTheLookOut(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v, _ := atVideoCover(t, ph, twoToneCover(t), false)
		kittest.PinMotion(r.Tester)
		require.True(t, v.Covered())
		red := pixelAt(r, geom.Pt(coverLeftX, coverY))
		require.InDelta(t, 128, float64(red.R), 1)

		publishHalves(t, pb.Video())
		r.Tester.Elapse(kit.CoverFade / 4)
		require.True(t, v.Fading(), "the look is fading out")
		mid := pixelAt(r, geom.Pt(coverLeftX, coverY)).R
		assert.Greater(t, mid, uint8(0), "part way, the cover is still there")
		assert.Less(t, mid, uint8(127), "and fainter")
		r.Tester.Elapse(kit.CoverFade / 4)
		assert.Less(t, pixelAt(r, geom.Pt(coverLeftX, coverY)).R, mid, "and fainter later")

		r.Tester.Elapse(kit.CoverFade)
		assert.False(t, v.Fading())
		assert.False(t, v.Covered())
		mirrored := pixelAt(r, geom.Pt(60, ph.Size.H/2))
		assert.Equal(t, color.RGBA{B: 255, A: 255}, mirrored, "the picture, mirrored, with no cover on it")
		assert.Equal(t, color.RGBA{A: 255}, pixelAt(r, geom.Pt(coverLeftX, coverY)), "the bar above the picture is black")

		// the picture goes away: the cover and the ring are back in the next frame
		pb.Video().Reset()
		pb.SetState(player.StateLoading)
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, v.Covered())
		nearPx(t, color.RGBA{R: 128, A: 255}, pixelAt(r, geom.Pt(coverLeftX, coverY)), "the cover again, at once")
		nearPx(t, color.RGBA{G: 128, A: 255}, pixelAt(r, geom.Pt(ph.Size.W-60, coverY)), "not mirrored")
	})
}

// TestVideoFirstFrameUnderReducedMotionIsAtOnce (K47): with Reduce Motion the look is
// gone in the frame the picture comes.
func TestVideoFirstFrameUnderReducedMotionIsAtOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v, _ := atVideoCover(t, ph, twoToneCover(t), true)
		publishHalves(t, pb.Video())
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, v.Fading())
		assert.False(t, v.Covered())
		assert.Equal(t, color.RGBA{A: 255}, pixelAt(r, geom.Pt(coverLeftX, coverY)))
	})
}

// TestVideoLoadingRingDoesNotAllocate (K47): a frame of the turning ring, over the
// cover and over the wordmark, allocates nothing.
func TestVideoLoadingRingDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, cover := range []bool{true, false} {
			var img *scene.Image
			name := ph.Name + " no cover"
			if cover {
				img, name = twoToneCover(t), ph.Name+" cover"
			}
			r, _, v, _ := atVideoCover(t, ph, img, false)
			kittest.PinMotion(r.Tester)
			require.True(t, v.Covered())
			step := func() { r.Tester.Elapse(kittest.Frame) }
			for range 20 {
				step()
			}
			before := ringPixels(r, ph)
			for range 20 {
				step()
			}
			assert.NotEqual(t, before, ringPixels(r, ph), name+": the arc turns")
			assert.Zero(t, testing.AllocsPerRun(50, step), name+": a frame of the loading ring")
		}
	}
}
