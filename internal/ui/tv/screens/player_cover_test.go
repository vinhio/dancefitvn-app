//go:build tvos

package screens_test

import (
	"image/color"
	"testing"

	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The synthetic covers of the K47 tests (no real picture): an address that is not a
// URL of anyone's.
const (
	coverURL  = "https://cdn.example.test/covers/set-35.jpg"
	coverURL2 = "https://cdn.example.test/covers/set-36.jpg"
)

// The places of the hole (656, 0, 608 by 1080) the tests read: inside it, off the
// controls, left and right of its middle (960); the ring's top and its left, 40 from
// the middle (88 across, a line of 8 about the radius 40); and just off the ring.
const (
	holeLeftX, holeRightX, holeY = 800, 1100, 100
	ringTopX, ringTopY           = 960, 500
	ringLeftX, ringLeftY         = 920, 540
	offRingX, offRingY           = 880, 540
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// twoToneCover is a 16 by 8 picture, red on its left half and green on its right: an
// aspect-fill of it into 608 by 1080 crops the sides about the middle, so the red
// and green meet at x 960.
func twoToneCover(t testing.TB) *scene.Image {
	t.Helper()
	img, err := scene.NewImage(16, 8)
	require.NoError(t, err)
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

// coverRig opens TV07 with the first frame not yet on the plane, the lesson's course
// cover at coverURL (answered with img, nil for a cover that is refused); fast turns
// the motion off.
func coverRig(t testing.TB, img *scene.Image, fast bool) (*rig, *tvtest.Services) {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	pb, ok := svc.PB.(*tvtest.Playback)
	require.True(t, ok)
	pb.LessonV.CoverURL = coverURL
	svc.SetCover(coverURL, img)
	var r *rig
	if fast {
		r = openFast(t, svc, env.RouteHome, false)
	} else {
		r = open(t, svc, env.RouteHome, false)
	}
	r.settle()
	r.sh.Go(nav.Route{Screen: env.RoutePlayer}, false)
	r.settle()

	return r, svc
}

// wordmarkShown reports whether the box above the ring (the wordmark's 56 tall line, 40
// above the ring's top at 496) has a pixel of the brand's yellow, however dim.
func (r *rig) wordmarkShown() bool {
	snap := r.pixels()
	for y := 395; y < 465; y++ {
		for x := 800; x < 1120; x++ {
			if c := snap.RGBAAt(x, y); c.R > 90 && c.G > 70 && c.B < c.G/2 {
				return true
			}
		}
	}

	return false
}

// px returns the pixel of the last frame.
func (r *rig) px(x, y int) color.RGBA { return r.pixels().RGBAAt(x, y) }

// pixNear asserts that got is within d of want in each channel.
func pixNear(t testing.TB, want, got color.RGBA, d int, msg string) {
	t.Helper()
	diff := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	assert.LessOrEqual(t, diff(want.R, got.R), d, msg+" (R)")
	assert.LessOrEqual(t, diff(want.G, got.G), d, msg+" (G)")
	assert.LessOrEqual(t, diff(want.B, got.B), d, msg+" (B)")
	assert.Equal(t, uint8(255), got.A, msg+" (opaque)")
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPlayerLoadingShowsTheDarkenedCover (K47): until the first frame the hole is the
// course's cover, filled and centred, under a black scrim at .5 (red and green become
// 128), with the ring in --pr on it, and no stage.
func TestPlayerLoadingShowsTheDarkenedCover(t *testing.T) {
	t.Parallel()
	r, svc := coverRig(t, twoToneCover(t), true)
	assert.Contains(t, svc.CoverCalls(), coverURL, "the cover is asked for")
	require.NotNil(t, r.player().Plane().Cover())
	assert.True(t, r.player().Plane().Covered())

	pixNear(t, color.RGBA{R: 128, A: 255}, r.px(holeLeftX, holeY), 3, "left of the middle: the red half, darkened")
	pixNear(t, color.RGBA{G: 128, A: 255}, r.px(holeRightX, holeY), 3, "right of the middle: the green half, darkened")
	// The ring is --pr (#FFD028) at the top, where it stands with the motion off.
	pixNear(t, color.RGBA{R: 255, G: 208, B: 40, A: 255}, r.px(ringTopX, ringTopY), 6, "the ring's arc")
	assert.Greater(t, r.px(ringLeftX, ringLeftY).G, r.px(offRingX, offRingY).G, "the ring's track is lighter than the cover")
	assert.False(t, r.wordmarkShown(), "a cover is there: no wordmark")
}

// TestPlayerLoadingWithoutACoverIsBlackWithTheRing (K47): no cover URL, a cover that is
// refused, and a cover not loaded yet all give #0B0B0C with the ring and the dimmed
// wordmark above it (the owner's addition), never the old stage (a gradient, the yellow glow, the stripes).
func TestPlayerLoadingWithoutACoverIsBlackWithTheRing(t *testing.T) {
	t.Parallel()
	t.Run("refused", func(t *testing.T) {
		t.Parallel()
		r, _ := coverRig(t, nil, true)
		assert.Nil(t, r.player().Plane().Cover())
		want := color.RGBA{R: 0x0b, G: 0x0b, B: 0x0c, A: 255}
		for _, at := range [][2]int{{holeLeftX, holeY}, {holeRightX, holeY}, {700, 1040}, {1200, 600}, {960, 200}} {
			pixNear(t, want, r.px(at[0], at[1]), 0, "the hole is plain")
		}
		pixNear(t, color.RGBA{R: 255, G: 208, B: 40, A: 255}, r.px(ringTopX, ringTopY), 6, "the ring's arc")
		assert.True(t, r.wordmarkShown(), "the dimmed DanceFITVN wordmark stands above the ring")
	})

	t.Run("no url", func(t *testing.T) {
		t.Parallel()
		svc := tvtest.NewPlayerServices(t)
		pb, ok := svc.PB.(*tvtest.Playback)
		require.True(t, ok)
		pb.LessonV.CoverURL = ""
		r := inPlayer(t, svc, false)
		assert.NotContains(t, svc.CoverCalls(), "", "nothing to ask for")
		assert.Nil(t, r.player().Plane().Cover())
		pixNear(t, color.RGBA{R: 0x0b, G: 0x0b, B: 0x0c, A: 255}, r.px(holeLeftX, holeY), 0, "plain")
		pixNear(t, color.RGBA{R: 255, G: 208, B: 40, A: 255}, r.px(ringTopX, ringTopY), 6, "the ring's arc")
		assert.True(t, r.wordmarkShown(), "the dimmed DanceFITVN wordmark stands above the ring")
	})
}

// TestPlayerCoverFollowsTheLesson (K47): another lesson with another cover drops the
// picture of the first until its own is there.
func TestPlayerCoverFollowsTheLesson(t *testing.T) {
	t.Parallel()
	r, svc := coverRig(t, twoToneCover(t), true)
	require.NotNil(t, r.player().Plane().Cover())

	pb := r.playback()
	pb.LessonV.CoverURL = coverURL2 // the services know no picture of it
	pb.Publish(player.ChangeLesson)
	r.settle()
	assert.Contains(t, svc.CoverCalls(), coverURL2)
	assert.Nil(t, r.player().Plane().Cover(), "the first lesson's picture is gone")
	pixNear(t, color.RGBA{R: 0x0b, G: 0x0b, B: 0x0c, A: 255}, r.px(holeLeftX, holeY), 0, "plain until the cover is there")
}

// TestPlayerFirstFrameRevealsTheVideo (K47): the picture arriving cross-fades the
// cover and the ring out over 250 ms, leaving the hole transparent; it going away
// brings them back at once.
func TestPlayerFirstFrameRevealsTheVideo(t *testing.T) {
	t.Parallel()
	r, _ := coverRig(t, twoToneCover(t), false)
	pl := r.player().Plane()
	require.True(t, pl.Covered())

	pb := r.playback()
	pb.Picture = true
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvkit.PlaneFade / 4)
	require.True(t, pl.Fading(), "the cover is fading out")
	mid := r.px(holeLeftX, holeY)
	assert.NotEqual(t, uint8(255), mid.A, "part way, the video shows through")
	r.tt.Elapse(tvkit.PlaneFade / 4)
	mid2 := r.px(holeLeftX, holeY)
	assert.Less(t, mid2.A, mid.A, "and more of it later")

	r.tt.Elapse(tvkit.PlaneFade)
	assert.False(t, pl.Fading())
	assert.False(t, pl.Covered())
	for _, at := range [][2]int{{holeLeftX, holeY}, {holeRightX, holeY}, {ringTopX, ringTopY}, {960, 540}} {
		assert.Zero(t, r.px(at[0], at[1]).A, "nothing is painted over the picture")
	}

	pb.Picture = false
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.True(t, pl.Covered(), "a picture that went away is covered")
	pixNear(t, color.RGBA{R: 128, A: 255}, r.px(holeLeftX, holeY), 3, "the cover again, at once")
}

// TestPlayerFirstFrameUnderReducedMotionIsAtOnce (K47): with the system's Reduce Motion
// the cover is gone in the frame the picture comes.
func TestPlayerFirstFrameUnderReducedMotionIsAtOnce(t *testing.T) {
	t.Parallel()
	r, _ := coverRig(t, twoToneCover(t), true)
	pb := r.playback()
	pb.Picture = true
	pb.Publish(player.ChangeState)
	r.tt.Elapse(tvtest.Frame)
	assert.False(t, r.player().Plane().Fading())
	assert.False(t, r.player().Plane().Covered())
	assert.Zero(t, r.px(holeLeftX, holeY).A)
}

// TestPlayerLoadingRingDoesNotAllocate (K47): a frame of the turning ring, over the
// cover, allocates nothing.
func TestPlayerLoadingRingDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r, _ := coverRig(t, twoToneCover(t), false)
	require.True(t, r.player().Plane().Covered())
	step := func() { r.tt.Elapse(tvtest.Frame) }
	for range 3 {
		step()
	}
	before := r.px(ringTopX, ringTopY)
	for range 20 {
		step()
	}
	assert.NotEqual(t, before, r.px(ringTopX, ringTopY), "the arc turns")
	assert.Zero(t, testing.AllocsPerRun(30, step), "a frame of the loading ring")
}
