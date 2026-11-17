// Adapted from jplayer-mobile/internal/media/media_test.go at 49de3c2.

package media

import (
	"errors"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorsWrapCoreSentinels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		is   error
	}{
		{"aborted", ErrAborted, core.ErrClosed},
		{"decode", ErrDecode, core.ErrUnsupported},
		{"source", ErrSource, core.ErrInvalid},
		{"missing file", ErrMissingFile, core.ErrNotFound},
		{"decode is errors.ErrUnsupported", ErrDecode, errors.ErrUnsupported},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, tt.err, tt.is)
		})
	}
	assert.NotErrorIs(t, ErrNetwork, core.ErrInvalid)
}

func TestEnumsZeroIsUnknown(t *testing.T) {
	t.Parallel()

	assert.Equal(t, KindUnknown, Kind(0))
	assert.Equal(t, StateUnknown, PlayState(0))
	assert.Equal(t, EventUnknown, EventKind(0))
	assert.Equal(t, SessionEventUnknown, SessionEventKind(0))
	var e Event
	assert.Equal(t, EventUnknown, e.Kind)
}

// publish draws a frame of n into the surface and publishes it.
func publish(t testing.TB, s *Surface, w, h int, n byte) *scene.Image {
	t.Helper()
	img := s.Back(w, h, true)
	require.NotNil(t, img)
	for i := range img.Pix {
		img.Pix[i] = n
	}
	s.Publish()

	return img
}

func TestSurface(t *testing.T) {
	t.Parallel()

	t.Run("empty before the first frame", func(t *testing.T) {
		t.Parallel()
		var s Surface
		img, seq := s.Frame(0)
		assert.Nil(t, img)
		assert.Zero(t, seq)
		s.Publish() // no back: nothing happens
		img, seq = s.Frame(0)
		assert.Nil(t, img)
		assert.Zero(t, seq)
	})
	t.Run("bad size", func(t *testing.T) {
		t.Parallel()
		var s Surface
		assert.Nil(t, s.Back(0, 10, false))
		assert.Nil(t, s.Back(10, -1, false))
		s.Publish()
		img, _ := s.Frame(0)
		assert.Nil(t, img, "nothing to publish after a bad Back")
	})
	t.Run("publish numbers frames and Frame latches the newest", func(t *testing.T) {
		t.Parallel()
		var s Surface
		a := s.Back(4, 2, true)
		require.NotNil(t, a)
		assert.True(t, a.BGRA)
		assert.True(t, a.Opaque)
		assert.Len(t, a.Pix, 4*2*4)
		a.Pix[0] = 7
		s.Publish()
		front, seq := s.Frame(0)
		assert.Same(t, a, front)
		assert.EqualValues(t, 1, seq)
		assert.EqualValues(t, 1, front.Version)

		b := s.Back(4, 2, true)
		assert.NotSame(t, a, b, "the back is not the front")
		s.Publish()
		front, seq = s.Frame(0)
		assert.Same(t, b, front)
		assert.EqualValues(t, 2, seq)
		assert.EqualValues(t, 2, front.Version)
	})
	t.Run("the front is not changed until the next Frame", func(t *testing.T) {
		t.Parallel()
		var s Surface
		a := publish(t, &s, 2, 2, 1)
		front, _ := s.Frame(0)
		require.Same(t, a, front)
		b := publish(t, &s, 2, 2, 2)
		again, seq := s.Frame(0)
		assert.Same(t, b, again, "a second Frame takes the news")
		assert.EqualValues(t, 2, seq)
		again, seq = s.Frame(0)
		assert.Same(t, b, again, "and one more changes nothing")
		assert.EqualValues(t, 2, seq)
	})
	t.Run("a frame the UI never took is replaced", func(t *testing.T) {
		t.Parallel()
		var s Surface
		publish(t, &s, 2, 2, 1)
		publish(t, &s, 2, 2, 2)
		c := publish(t, &s, 2, 2, 3)
		front, seq := s.Frame(0)
		assert.Same(t, c, front)
		assert.EqualValues(t, 3, seq)
		assert.Equal(t, byte(3), front.Pix[0])
	})
	t.Run("the producer never gets the front back", func(t *testing.T) {
		t.Parallel()
		var s Surface
		for n := range 20 {
			publish(t, &s, 2, 2, byte(n))
			front, _ := s.Frame(0)
			for range 3 {
				back := s.Back(2, 2, true)
				assert.NotSame(t, front, back)
			}
			if n%3 == 0 {
				s.Publish() // the third frame of this round, taken at once
			}
		}
	})
	t.Run("three images, no more", func(t *testing.T) {
		t.Parallel()
		var s Surface
		seen := map[*scene.Image]bool{}
		for n := range 30 {
			seen[publish(t, &s, 2, 2, byte(n))] = true
			if n%2 == 0 {
				_, _ = s.Frame(0)
			}
		}
		assert.Len(t, seen, 3)
	})
	t.Run("resize only on change", func(t *testing.T) {
		t.Parallel()
		var s Surface
		a := s.Back(4, 2, false)
		assert.Same(t, a, s.Back(4, 2, false))
		assert.False(t, s.Back(4, 2, false).BGRA)
		c := s.Back(8, 6, false)
		assert.NotSame(t, a, c)
		assert.Equal(t, 8, c.Width)
		assert.Equal(t, 6, c.Height)
	})
	t.Run("a new size reaches the front through the three images", func(t *testing.T) {
		t.Parallel()
		var s Surface
		for range 4 {
			publish(t, &s, 4, 2, 1)
			_, _ = s.Frame(0)
		}
		for range 4 {
			publish(t, &s, 8, 6, 2)
			img, _ := s.Frame(0)
			require.NotNil(t, img)
			assert.Equal(t, 8, img.Width)
			assert.Len(t, img.Pix, 8*6*4)
		}
	})
	t.Run("versions keep growing", func(t *testing.T) {
		t.Parallel()
		var s Surface
		var last uint64
		for range 10 {
			publish(t, &s, 2, 2, 0)
			img, seq := s.Frame(0)
			assert.Greater(t, seq, last)
			assert.Equal(t, seq, img.Version)
			last = seq
		}
	})
}

func TestSurfaceLatchesOncePerPaint(t *testing.T) {
	t.Parallel()

	t.Run("the calls of one paint see one frame", func(t *testing.T) {
		t.Parallel()
		var s Surface
		a := publish(t, &s, 2, 2, 1)
		first, seq := s.Frame(1)
		require.Same(t, a, first)
		assert.EqualValues(t, 1, seq)
		publish(t, &s, 2, 2, 2)
		again, seq := s.Frame(1)
		assert.Same(t, a, again, "the second view of paint 1 draws the first one's image")
		assert.EqualValues(t, 1, seq)
		publish(t, &s, 2, 2, 3)
		for range 3 {
			back := s.Back(2, 2, true)
			assert.NotSame(t, a, back, "the image of the paint is never the producer's")
		}
		s.Publish()
		next, seq := s.Frame(2)
		assert.NotSame(t, a, next, "the next paint takes the news")
		assert.EqualValues(t, 4, seq)
		same, _ := s.Frame(2)
		assert.Same(t, next, same)
	})
	t.Run("paint 0 latches at every call", func(t *testing.T) {
		t.Parallel()
		var s Surface
		a := publish(t, &s, 2, 2, 1)
		got, _ := s.Frame(0)
		require.Same(t, a, got)
		b := publish(t, &s, 2, 2, 2)
		got, _ = s.Frame(0)
		assert.Same(t, b, got)
	})
	t.Run("a Reset within a paint shows black", func(t *testing.T) {
		t.Parallel()
		var s Surface
		publish(t, &s, 2, 2, 1)
		img, _ := s.Frame(7)
		require.NotNil(t, img)
		s.Reset()
		img, seq := s.Frame(7)
		assert.Nil(t, img, "the frame of the media before is not shown")
		assert.Zero(t, seq)
	})
	t.Run("the first frame comes within a paint that saw none", func(t *testing.T) {
		t.Parallel()
		var s Surface
		img, _ := s.Frame(1)
		require.Nil(t, img)
		publish(t, &s, 2, 2, 4)
		img, _ = s.Frame(1)
		assert.Nil(t, img, "not before the next paint")
		img, _ = s.Frame(2)
		require.NotNil(t, img)
		assert.Equal(t, byte(4), img.Pix[0])
	})
}

// TestSurfaceConcurrentPaint has a producer goroutine publish frames, each
// written byte by byte, between the two views' calls of each paint (the
// mini player, then Now Playing): both draw the same image, and its bytes
// do not change until the next paint. With the race detector on it fails if
// the producer ever writes an image a view of the paint read.
func TestSurfaceConcurrentPaint(t *testing.T) {
	t.Parallel()

	const paints = 500
	var s Surface
	produce := func(n int) {
		img := s.Back(8, 8, true)
		if img == nil {
			return
		}
		for i := range img.Pix {
			img.Pix[i] = byte(n)
		}
		s.Publish()
	}
	ask := make(chan int)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for n := range ask {
			for k := range 3 { // more than the three images: any slot not the front is taken
				produce(n*3 + k)
			}
			done <- struct{}{}
		}
	}()
	defer func() {
		close(ask)
		wg.Wait()
	}()
	produce(0)
	for paint := uint64(1); paint <= paints; paint++ {
		mini, seq := s.Frame(paint)
		require.NotNil(t, mini)
		want := mini.Pix[0]
		ask <- int(paint)
		<-done
		big, again := s.Frame(paint)
		require.Same(t, mini, big, "paint %d: the two views draw one image", paint)
		assert.Equal(t, seq, again)
		for _, b := range big.Pix {
			if b != want {
				t.Fatalf("paint %d: the image changed under the paint: %d then %d", paint, want, b)
			}
		}
	}
}

func TestSurfaceReset(t *testing.T) {
	t.Parallel()

	t.Run("no picture until a frame after it", func(t *testing.T) {
		t.Parallel()
		var s Surface
		publish(t, &s, 2, 2, 1)
		img, _ := s.Frame(0)
		require.NotNil(t, img)

		s.Reset()
		img, seq := s.Frame(0)
		assert.Nil(t, img)
		assert.Zero(t, seq)

		publish(t, &s, 2, 2, 2)
		img, seq = s.Frame(0)
		require.NotNil(t, img)
		assert.EqualValues(t, 2, seq)
		assert.Equal(t, byte(2), img.Pix[0])
	})
	t.Run("a frame published before it is not shown", func(t *testing.T) {
		t.Parallel()
		var s Surface
		publish(t, &s, 2, 2, 1) // ready, not latched
		s.Reset()
		img, _ := s.Frame(0)
		assert.Nil(t, img)
	})
	t.Run("a back taken before it is dropped", func(t *testing.T) {
		t.Parallel()
		var s Surface
		img := s.Back(2, 2, true)
		require.NotNil(t, img)
		s.Reset()
		s.Publish()
		got, _ := s.Frame(0)
		assert.Nil(t, got, "the frame being drawn when the media changed")
		publish(t, &s, 2, 2, 5)
		got, _ = s.Frame(0)
		require.NotNil(t, got)
		assert.Equal(t, byte(5), got.Pix[0])
	})
	t.Run("twice", func(t *testing.T) {
		t.Parallel()
		var s Surface
		s.Reset()
		s.Reset()
		img, _ := s.Frame(0)
		assert.Nil(t, img)
	})
}

func TestSurfaceFree(t *testing.T) {
	t.Parallel()

	var s Surface
	for range 4 {
		publish(t, &s, 16, 9, 1)
		_, _ = s.Frame(0)
	}
	s.Free()
	img, seq := s.Frame(0)
	assert.Nil(t, img)
	assert.Zero(t, seq)
	s.Publish()
	img, _ = s.Frame(0)
	assert.Nil(t, img, "a Publish after Free, with no Back, does nothing")

	a := publish(t, &s, 16, 9, 7)
	front, seq := s.Frame(0)
	assert.Same(t, a, front, "usable again")
	assert.Greater(t, seq, uint64(4), "the numbers go on")
	s.Free()
	s.Free()
}

// TestSurfaceFreeReleasesPixels frees a surface whose images a scene still
// points to (the view keeps the scenes of its last frames, and none comes
// in the background): each gives its pixels up, as one black pixel with a
// new Version, and the func of SetOnFree runs once a Free that dropped
// images, never for one that had none to drop.
func TestSurfaceFreeReleasesPixels(t *testing.T) {
	t.Parallel()

	var s Surface
	freed := 0
	s.SetOnFree(func() { freed++ })
	assert.False(t, s.Holds())
	s.Free()
	assert.Zero(t, freed, "nothing to drop")

	var shown []*scene.Image
	for i := range 3 {
		publish(t, &s, 16, 9, byte(i+1))
		img, _ := s.Frame(0)
		shown = append(shown, img)
	}
	require.True(t, s.Holds())
	vers := make([]uint64, len(shown))
	for i, img := range shown {
		vers[i] = img.Version
	}
	s.Free()
	assert.Equal(t, 1, freed)
	assert.False(t, s.Holds())
	for i, img := range shown {
		assert.Equal(t, [4]int{1, 1, 4, 4}, [4]int{img.Width, img.Height, img.Stride, len(img.Pix)}, "image %d", i)
		assert.Equal(t, []byte{0, 0, 0, 0xff}, img.Pix, "opaque black, image %d", i)
		assert.Greater(t, img.Version, vers[i], "a new Version damages it, image %d", i)
	}
	s.Free()
	assert.Equal(t, 1, freed, "a second Free drops nothing")

	img := publish(t, &s, 16, 9, 9)
	assert.Len(t, img.Pix, 16*9*4, "the next Back makes the images again")
	assert.True(t, s.Holds())
	s.SetOnFree(nil)
	s.Free()
	assert.Equal(t, 1, freed, "no func")
}

func TestSurfaceFlags(t *testing.T) {
	t.Parallel()

	t.Run("active", func(t *testing.T) {
		t.Parallel()
		var s Surface
		assert.False(t, s.Active())
		s.SetActive(true)
		assert.True(t, s.Active())
		s.SetActive(false)
		assert.False(t, s.Active())
	})
	t.Run("watch counts views and unwatch is idempotent", func(t *testing.T) {
		t.Parallel()
		var s Surface
		assert.False(t, s.Watched())
		u1 := s.Watch()
		u2 := s.Watch()
		assert.True(t, s.Watched())
		u1()
		u1()
		assert.True(t, s.Watched(), "the second view is still there")
		u2()
		assert.False(t, s.Watched())
	})
	t.Run("pull", func(t *testing.T) {
		t.Parallel()
		var s Surface
		assert.False(t, s.Pulls())
		s.Pull() // none: nothing happens
		n := 0
		s.SetPull(func() { n++ })
		assert.True(t, s.Pulls())
		s.Pull()
		s.Pull()
		assert.Equal(t, 2, n)
		s.SetPull(nil)
		assert.False(t, s.Pulls())
		s.Pull()
		assert.Equal(t, 2, n)
	})
	t.Run("rotation", func(t *testing.T) {
		t.Parallel()
		tests := []struct{ in, want int }{
			{0, 0},
			{90, 90},
			{180, 180},
			{270, 270},
			{360, 0},
			{450, 90},
			{-90, 270},
			{-180, 180},
			{44, 0},
			{46, 90},
			{-44, 0},
			{-46, 270},
			{720, 0},
		}
		for _, tt := range tests {
			var s Surface
			s.SetRotation(tt.in)
			assert.Equal(t, tt.want, s.Rotation(), "%d", tt.in)
		}
	})
	t.Run("notify tells the newest sequence", func(t *testing.T) {
		t.Parallel()
		var s Surface
		var got []uint64
		off := s.Frames().Subscribe(func(n uint64) { got = append(got, n) })
		s.Notify()
		publish(t, &s, 2, 2, 1)
		publish(t, &s, 2, 2, 1)
		s.Notify()
		off()
		s.Notify()
		assert.Equal(t, []uint64{0, 2}, got)
	})
}

func TestSurfaceFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	var s Surface
	for range 3 {
		publish(t, &s, 64, 36, 1)
	}
	paint := uint64(0)
	allocs := testing.AllocsPerRun(100, func() {
		_, _ = s.Frame(0)
		paint++
		_, _ = s.Frame(paint)
		_, _ = s.Frame(paint)
	})
	assert.Zero(t, allocs)
}

// BenchmarkSurfaceFrame is a paint of two views of one surface, with a
// frame published before each.
func BenchmarkSurfaceFrame(b *testing.B) {
	var s Surface
	for range 3 {
		publish(b, &s, 64, 36, 1)
	}
	b.ReportAllocs()
	paint := uint64(0)
	for b.Loop() {
		s.Back(64, 36, true)
		s.Publish()
		paint++
		_, _ = s.Frame(paint)
		_, _ = s.Frame(paint)
	}
}

func TestSurfacePublishDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	var s Surface
	for range 3 {
		publish(t, &s, 64, 36, 1)
	}
	allocs := testing.AllocsPerRun(100, func() {
		s.Back(64, 36, true)
		s.Publish()
		_, _ = s.Frame(0)
	})
	assert.Zero(t, allocs)
}

func TestSurfaceNotifyDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	var s Surface
	n := 0
	s.Frames().Subscribe(func(uint64) { n++ })
	s.Notify()
	s.SetPull(func() {})
	allocs := testing.AllocsPerRun(100, func() {
		s.Notify()
		s.Pull()
		s.SetActive(true)
		_ = s.Active()
		_ = s.Watched()
		_ = s.Rotation()
	})
	assert.Zero(t, allocs)
	assert.Positive(t, n)
}

// TestSurfaceConcurrent has one goroutine produce frames, each written byte
// by byte, while the UI one reads every byte of what Frame gives: with the
// race detector on it fails if the UI ever reads an image the producer writes,
// and without it a torn frame (bytes of two numbers) fails the check.
func TestSurfaceConcurrent(t *testing.T) {
	t.Parallel()

	const frames = 3000
	var s Surface
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(done)
		for n := range frames {
			w := 8 + n%3*2 // the size changes now and then
			img := s.Back(w, 8, true)
			if img == nil {
				return
			}
			v := byte(n)
			for i := range img.Pix {
				img.Pix[i] = v
			}
			s.Publish()
		}
	}()
	last := uint64(0)
	check := func() {
		img, seq := s.Frame(0)
		if img == nil {
			return
		}
		assert.GreaterOrEqual(t, seq, last, "the sequence never goes back")
		last = seq
		first := img.Pix[0]
		for _, b := range img.Pix {
			if b != first {
				t.Fatalf("torn frame %d: %d and %d", seq, first, b)
			}
		}
		assert.Equal(t, byte(seq-1), first, "the pixels are those of the frame the number says")
	}
	for {
		select {
		case <-done:
			wg.Wait()
			check()
			_, seq := s.Frame(0)
			assert.EqualValues(t, frames, seq)

			return
		default:
			check()
		}
	}
}

// TestSurfaceConcurrentReset adds the UI thread's Reset and the producer's
// Back/Publish pair crossing it.
func TestSurfaceConcurrentReset(t *testing.T) {
	t.Parallel()

	var s Surface
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			if img := s.Back(8, 8, true); img != nil {
				for i := range img.Pix {
					img.Pix[i] = 9
				}
			}
			s.Publish()
		}
	}()
	for range 2000 {
		s.Reset()
		if img, _ := s.Frame(0); img != nil {
			for _, b := range img.Pix {
				if b != 9 {
					t.Errorf("pixel %d", b)

					break
				}
			}
		}
	}
	close(stop)
	wg.Wait()
}

func BenchmarkSurfacePublishFrame(b *testing.B) {
	var s Surface
	for range 3 {
		s.Back(1280, 720, true)
		s.Publish()
	}
	b.ReportAllocs()
	for b.Loop() {
		s.Back(1280, 720, true)
		s.Publish()
		_, _ = s.Frame(0)
	}
}
