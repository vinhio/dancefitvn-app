// Adapted from jplayer-mobile/internal/ui/np/video_test.go and video_alloc_test.go
// at 49de3c2 (the fit table, black before the first frame, the letterbox, the
// pull, one frame a paint, the allocation gates), over a VideoSource of the
// test's own: the kit does not import internal/media. The real Surface is
// driven in internal/ui/mobile/screens.

package kit_test

import (
	stdcolor "image/color"
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

var (
	vvRed   = color.RGB(255, 0, 0)
	vvBlue  = color.RGB(0, 0, 255)
	vvBlack = color.RGB(0, 0, 0)
)

// fakeSource is a VideoSource with one image the test fills and replaces.
type fakeSource struct {
	img     *scene.Image
	kept    *scene.Image
	rot     int
	frames  event.Topic[uint64]
	active  bool
	pulls   bool
	pulled  int
	watches int
	seq     uint64
}

var _ kit.VideoSource = (*fakeSource)(nil)

func (f *fakeSource) Frame(uint64) (*scene.Image, uint64) { return f.img, f.seq }
func (f *fakeSource) Rotation() int                       { return f.rot }
func (f *fakeSource) Kept() (*scene.Image, int)           { return f.kept, 0 }
func (f *fakeSource) Active() bool                        { return f.active }
func (f *fakeSource) Pulls() bool                         { return f.pulls }
func (f *fakeSource) Pull()                               { f.pulled++; f.publish(nil) }
func (f *fakeSource) Frames() *event.Topic[uint64]        { return &f.frames }

func (f *fakeSource) Watch() func() {
	f.watches++

	return func() { f.watches-- }
}

// publish makes the next frame: halves left red and right blue, w by h; with a
// pick, that picture's color.
func (f *fakeSource) publish(pick func(x, y int) color.Color) {
	if f.img == nil {
		f.img, _ = scene.NewImage(160, 90)
	}
	f.seq++
	f.img.Version = f.seq
	if pick == nil {
		return
	}
	for y := range f.img.Height {
		for x := range f.img.Width {
			c := pick(x, y)
			o := y*f.img.Stride + x*4
			f.img.Pix[o], f.img.Pix[o+1], f.img.Pix[o+2], f.img.Pix[o+3] = c.R, c.G, c.B, 255
		}
	}
}

// sizedBox gives its child a box of w by h.
type sizedBox struct {
	ui.Base
	w, h float32
}

// sized returns child in a box of w by h.
func sized(child ui.Widget, w, h float32) *sizedBox {
	b := &sizedBox{w: w, h: h}
	b.Add(child)

	return b
}

// Layout makes the child as large as the box.
func (b *sizedBox) Layout(c *ui.Context, _ ui.Constraints) geom.Size {
	for ch := range b.Children() {
		c.Layout(ch, ui.Tight(geom.Sz(b.w, b.h)))
		c.Place(ch, geom.Point{})
	}

	return geom.Sz(b.w, b.h)
}

// redBlue is red on the left half of the picture, blue on the right.
func redBlue(x, _ int) color.Color {
	if x < 80 {
		return vvRed
	}

	return vvBlue
}

// vvPixel is the window's pixel at p.
func vvPixel(tt *ui.Tester, p geom.Point) stdcolor.RGBA {
	img := tt.Snapshot()
	return img.RGBAAt(int(p.X), int(p.Y))
}

// vvWant checks the pixel at p is c.
func vvWant(t testing.TB, tt *ui.Tester, p geom.Point, c color.Color, msg string) {
	t.Helper()
	g := vvPixel(tt, p)
	d := func(x, y uint8) int { return max(int(x)-int(y), int(y)-int(x)) }
	assert.True(t, d(g.R, c.R) <= 3 && d(g.G, c.G) <= 3 && d(g.B, c.B) <= 3, "%s at %v: got %v, want %v", msg, p, g, c)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestVideoViewLayout checks that the view fills what it is given and is 16:9
// when it has no height to fill.
func TestVideoViewLayout(t *testing.T) {
	t.Parallel()

	v := kit.NewVideoView(nil, kit.FitContain)
	assert.Equal(t, kit.NameVideo, v.Name)
	assert.Equal(t, geom.Sz(393, 221), v.Layout(nil, ui.Tight(geom.Sz(393, 221))))
	got := v.Layout(nil, ui.Constraints{Max: geom.Sz(320, ui.Unbounded)})
	assert.InDelta(t, 320, got.W, .001)
	assert.InDelta(t, 180, got.H, .001)
}

// TestVideoViewNoFrame checks black before the first frame, and the stage under
// it showing through with Under.
func TestVideoViewNoFrame(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		v := kit.NewVideoView(&fakeSource{}, kit.FitContain)
		tt := kittest.Mount(t, ph, newStage(placeAt(v, 20, 20, 200)))
		vvWant(t, tt, geom.Pt(120, 40), vvBlack, "black box before the first frame")

		u := kit.NewVideoView(&fakeSource{}, kit.FitContain)
		u.Under = true
		ut := kittest.Mount(t, ph, newStage(placeAt(u, 20, 20, 200)))
		g := vvPixel(ut, geom.Pt(120, 40))
		assert.NotEqual(t, stdcolor.RGBA{R: 0, G: 0, B: 0, A: 255}, g, "Under paints nothing: --bg shows")
	})
}

// TestVideoViewContainAndMirror checks that a 160 by 90 picture in a square box
// is letterboxed, and that Mirror flips it left to right about the box (pixel).
func TestVideoViewContainAndMirror(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		src := &fakeSource{}
		v := kit.NewVideoView(src, kit.FitContain)
		tt := kittest.Mount(t, ph, newStage(placeAt(sized(v, 300, 300), 10, 10, 0)))
		src.publish(redBlue)
		src.frames.Publish(src.seq)
		tt.Frame()
		f := frameOf(v)
		mid := f.Y + f.H/2
		vvWant(t, tt, geom.Pt(f.X+50, mid), vvRed, "left half")
		vvWant(t, tt, geom.Pt(f.X+250, mid), vvBlue, "right half")
		vvWant(t, tt, geom.Pt(f.X+50, f.Y+3), vvBlack, "the bar above")

		v.SetMirror(true)
		tt.Frame()
		vvWant(t, tt, geom.Pt(f.X+50, mid), vvBlue, "mirrored: left half is the right one")
		vvWant(t, tt, geom.Pt(f.X+250, mid), vvRed, "mirrored: right half is the left one")
		v.SetMirror(false)
		tt.Frame()
		vvWant(t, tt, geom.Pt(f.X+50, mid), vvRed, "unmirrored again")
	})
}

// TestVideoViewRotationAndMirror checks a turn of 90 degrees with the flip.
func TestVideoViewRotationAndMirror(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		src := &fakeSource{rot: 90}
		v := kit.NewVideoView(src, kit.FitContain)
		tt := kittest.Mount(t, ph, newStage(placeAt(v, 0, 0, 200)))
		src.publish(redBlue)
		src.frames.Publish(src.seq)
		tt.Frame()
		f := frameOf(v)
		cx := f.X + f.W/2
		// 90: the left edge turns to the top; mirroring changes nothing on the
		// vertical axis of a picture that is symmetric about it.
		vvWant(t, tt, geom.Pt(cx, f.Y+15), vvRed, "90: top")
		vvWant(t, tt, geom.Pt(cx, f.Y+f.H-15), vvBlue, "90: bottom")
		v.SetMirror(true)
		tt.Frame()
		vvWant(t, tt, geom.Pt(cx, f.Y+15), vvRed, "90 mirrored: top")
	})
}

// TestVideoViewWatchFollowsMountAndShown checks the interest in the frames.
func TestVideoViewWatchFollowsMountAndShown(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		src := &fakeSource{}
		v := kit.NewVideoView(src, kit.FitContain)
		assert.Equal(t, 0, src.watches, "not mounted")
		tt := kittest.Mount(t, ph, newStage(placeAt(v, 0, 0, 100)))
		assert.Equal(t, 1, src.watches, "mounted")
		v.SetShown(false)
		assert.Equal(t, 0, src.watches, "hidden")
		v.SetShown(true)
		assert.Equal(t, 1, src.watches)
		other := &fakeSource{}
		v.SetSource(other)
		assert.Equal(t, 0, src.watches)
		assert.Equal(t, 1, other.watches)
		assert.Same(t, other, v.Source())
		v.SetSource(nil)
		assert.Equal(t, 0, other.watches)
		tt.Frame()
	})
}

// TestVideoViewPulls checks that an active, pulled source is asked for a frame
// at every paint.
func TestVideoViewPulls(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		src := &fakeSource{active: true, pulls: true}
		v := kit.NewVideoView(src, kit.FitContain)
		tt := kittest.Mount(t, ph, newStage(placeAt(v, 0, 0, 100)))
		before := src.pulled
		tt.Elapse(3 * kittest.Frame)
		assert.GreaterOrEqual(t, src.pulled, before+2, "a pull a paint, each asking for the next")
		src.active = false
		n := src.pulled
		v.Repaint()
		tt.Frame()
		assert.Equal(t, n, src.pulled, "no pull when the media does not play")
	})
}

// TestVideoViewKeptFrame checks the frame the source kept shows when it has no
// frame of its own.
func TestVideoViewKeptFrame(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		kept, err := scene.NewImage(160, 90)
		require.NoError(t, err)
		for i := 0; i < len(kept.Pix); i += 4 {
			kept.Pix[i], kept.Pix[i+3] = 255, 255
		}
		kept.Version = 1
		src := &fakeSource{kept: kept}
		v := kit.NewVideoView(src, kit.FitCover)
		tt := kittest.Mount(t, ph, newStage(placeAt(v, 0, 0, 100)))
		f := frameOf(v)
		vvWant(t, tt, geom.Pt(f.X+f.W/2, f.Y+f.H/2), vvRed, "the kept frame")
	})
}

// TestVideoViewFrameDoesNotAllocate is the gate of a video frame: a paint with no
// new frame, a paint of a new frame and a paint that pulls, plain and mirrored,
// allocate nothing once the images are made.
func TestVideoViewFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		for _, mirror := range []bool{false, true} {
			src := &fakeSource{}
			v := kit.NewVideoView(src, kit.FitContain)
			v.Mirror = mirror
			tt := kittest.Mount(t, ph, newStage(placeAt(sized(v, 300, 300), 10, 10, 0)))
			src.publish(redBlue)
			for range 6 {
				src.publish(nil)
				src.frames.Publish(src.seq)
				tt.Frame()
			}
			steady := func() {
				v.Repaint()
				tt.Frame()
			}
			steady()
			assert.Zero(t, testing.AllocsPerRun(100, steady), "%s mirror=%v: a paint with no new frame", ph.Name, mirror)
			fresh := func() {
				src.publish(nil)
				src.frames.Publish(src.seq)
				tt.Frame()
			}
			assert.Zero(t, testing.AllocsPerRun(100, fresh), "%s mirror=%v: a paint of a new frame", ph.Name, mirror)
			src.active, src.pulls = true, true
			assert.Zero(t, testing.AllocsPerRun(100, steady), "%s mirror=%v: a paint that pulls", ph.Name, mirror)
		}
	}
}

// BenchmarkVideoViewNewFrame is a paint of a new frame, plain and mirrored.
func BenchmarkVideoViewNewFrame(b *testing.B) {
	for _, mirror := range []bool{false, true} {
		name := "plain"
		if mirror {
			name = "mirror"
		}
		b.Run(name, func(b *testing.B) {
			src := &fakeSource{}
			v := kit.NewVideoView(src, kit.FitContain)
			v.Mirror = mirror
			tt := kittest.Mount(b, kittest.Phones[0], newStage(placeAt(sized(v, 300, 300), 10, 10, 0)))
			src.publish(redBlue)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				src.publish(nil)
				src.frames.Publish(src.seq)
				tt.Frame()
			}
		})
	}
}
