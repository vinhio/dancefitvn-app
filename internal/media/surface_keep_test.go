// Adapted from jplayer-mobile/internal/media/surface_keep_test.go at 49de3c2.

package media

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The kept frame of M17 (docs/plan/12-paused-frame.md P01-P03, P07): a copy
// of the frame on screen, the app's own, that Free and Reset leave alone.

// fill publishes a w by h frame whose every byte is b.
func fill(t testing.TB, s *Surface, w, h int, b byte) {
	t.Helper()
	img := s.Back(w, h, true)
	require.NotNil(t, img)
	for i := range img.Pix {
		img.Pix[i] = b
	}
	s.Publish()
}

func TestSurfaceKeepWithoutAFrameKeepsNothing(t *testing.T) {
	t.Parallel()
	var s Surface
	assert.False(t, s.Keep(), "no frame yet")
	img, _ := s.Kept()
	assert.Nil(t, img)

	fill(t, &s, 4, 2, 7)
	assert.False(t, s.Keep(), "published but never latched: nothing on screen")
}

func TestSurfaceKeepOutlivesFreeAndReset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		drop func(s *Surface)
	}{
		{"free (background, stop)", (*Surface).Free},
		{"reset (a load of the same media)", (*Surface).Reset},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var s Surface
			s.SetRotation(90)
			fill(t, &s, 4, 2, 9)
			front, _ := s.Frame(1)
			require.NotNil(t, front)
			require.True(t, s.Keep())

			tt.drop(&s)
			img, _ := s.Frame(2)
			assert.Nil(t, img, "the engine has no frame")
			kept, rot := s.Kept()
			require.NotNil(t, kept)
			assert.NotSame(t, front, kept, "a copy, not one of the three images")
			assert.Equal(t, 4, kept.Width)
			assert.Equal(t, 2, kept.Height)
			assert.True(t, kept.BGRA)
			assert.True(t, kept.Opaque)
			assert.Equal(t, 90, rot)
			for _, b := range kept.Pix {
				require.Equal(t, byte(9), b, "the pixels of the frame on screen")
			}
			assert.True(t, s.Keep(), "a Keep with nothing on screen leaves the frame kept")

			s.Unkeep()
			kept, _ = s.Kept()
			assert.Nil(t, kept, "another media: let go")
		})
	}
}

func TestSurfaceKeepCopiesTheFrameOnScreen(t *testing.T) {
	t.Parallel()
	var s Surface
	fill(t, &s, 2, 2, 1)
	_, _ = s.Frame(1)
	fill(t, &s, 2, 2, 2) // published, not yet latched: not on screen
	require.True(t, s.Keep())
	kept, _ := s.Kept()
	assert.Equal(t, byte(1), kept.Pix[0], "the front the last paint latched")
	v := kept.Version

	require.True(t, s.Keep())
	assert.Equal(t, v, kept.Version, "the same frame is not copied again")

	_, _ = s.Frame(2)
	require.True(t, s.Keep())
	same, _ := s.Kept()
	assert.Same(t, kept, same, "one image, reused")
	assert.Equal(t, byte(2), same.Pix[0])
	assert.Greater(t, same.Version, v, "a new copy is a new version for the renderers")

	v = same.Version
	fill(t, &s, 3, 2, 3)
	_, _ = s.Frame(3)
	require.True(t, s.Keep())
	bigger, _ := s.Kept()
	assert.Equal(t, 3, bigger.Width, "made again for another size")
	assert.Greater(t, bigger.Version, v)
	wantBlack(t, same, v, "the image of the other size is let go")
}

func TestSurfaceUnkeepLetsThePixelsGo(t *testing.T) {
	t.Parallel()
	var s Surface
	fill(t, &s, 4, 2, 5)
	_, _ = s.Frame(1)
	require.True(t, s.Keep())
	kept, _ := s.Kept()
	v := kept.Version
	s.Unkeep()
	wantBlack(t, kept, v, "a hidden scene still drawing it holds no memory")
}

func TestSurfaceGenCountsResetsAndFrees(t *testing.T) {
	t.Parallel()
	var s Surface
	g := s.Gen()
	fill(t, &s, 2, 2, 1)
	assert.Equal(t, g, s.Gen(), "a frame is no new media")
	s.Reset()
	assert.Equal(t, g+1, s.Gen(), "Load")
	s.Free()
	assert.Equal(t, g+2, s.Gen(), "Stop, the background")
}

// wantBlack checks that img gave its pixels up: one opaque black pixel,
// with a Version past v.
func wantBlack(t *testing.T, img *scene.Image, v uint64, msg string) {
	t.Helper()
	assert.Equal(t, 1, img.Width, msg)
	assert.Equal(t, 1, img.Height, msg)
	assert.Equal(t, []byte{0, 0, 0, 0xff}, img.Pix, msg)
	assert.Greater(t, img.Version, v, msg)
}

func TestSurfaceFreeTellsTheViews(t *testing.T) {
	t.Parallel()
	var s Surface
	told := 0
	unsub := s.Frames().Subscribe(func(uint64) { told++ })
	defer unsub()

	s.Free()
	assert.Zero(t, told, "nothing dropped: nothing to paint again")
	fill(t, &s, 2, 2, 1)
	s.Free()
	assert.Equal(t, 1, told, "the images went: the views paint again")

	_, _ = s.Frame(0)
	s.Unkeep()
	assert.Equal(t, 1, told, "nothing kept")
	fill(t, &s, 2, 2, 1)
	_, _ = s.Frame(0)
	require.True(t, s.Keep())
	s.Unkeep()
	assert.Equal(t, 2, told, "the kept frame went")
}

func TestSurfaceKeepDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	var s Surface
	var n uint64
	step := func() {
		img := s.Back(64, 36, true)
		img.Pix[0]++
		s.Publish()
		n++
		_, _ = s.Frame(n)
		s.Keep()
	}
	for range 4 {
		step()
	}
	assert.Zero(t, testing.AllocsPerRun(100, step), "a new frame kept, once the image has its size")
	assert.Zero(t, testing.AllocsPerRun(100, func() { s.Keep() }), "the same frame kept again")
	assert.Zero(t, testing.AllocsPerRun(100, func() { _, _ = s.Kept() }))
}

// BenchmarkSurfaceKeep is the one copy a pause makes of a 1280 by 720
// frame, the largest the engines give (enginekit.MaxEdge).
func BenchmarkSurfaceKeep(b *testing.B) {
	var s Surface
	var n uint64
	for b.Loop() {
		b.StopTimer()
		img := s.Back(1280, 720, true)
		img.Pix[0]++
		s.Publish()
		n++
		_, _ = s.Frame(n)
		b.StartTimer()
		s.Keep()
	}
}
