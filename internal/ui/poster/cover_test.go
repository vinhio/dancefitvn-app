package poster_test

import (
	"context"
	"errors"
	"image"
	"sync"
	"sync/atomic"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/poster"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// green is a w x h picture of one color, the stand-in of a cover.
func green(t testing.TB) *scene.Image {
	t.Helper()
	img, err := scene.NewImage(40, 20)
	require.NoError(t, err)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 0, 255, 0, 255
	}

	return img
}

// fakeLoader answers from a table, counts its calls, and can hold a load
// until its context ends (block).
type fakeLoader struct {
	mu       sync.Mutex
	pics     map[string]*scene.Image
	calls    []string
	block    bool
	started  chan struct{}
	canceled atomic.Bool
}

func (f *fakeLoader) load(ctx context.Context, url string) (*scene.Image, error) {
	f.mu.Lock()
	f.calls = append(f.calls, url)
	img, block := f.pics[url], f.block
	f.mu.Unlock()
	if block {
		f.started <- struct{}{}
		<-ctx.Done()
		f.canceled.Store(true)

		return nil, ctx.Err()
	}
	if img == nil {
		return nil, errors.New("no cover")
	}

	return img, nil
}

func (f *fakeLoader) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]string(nil), f.calls...)
}

// TestPosterCoverLoadedIsDrawn mounts a poster with a cover address: the vector
// drawing shows until the picture is loaded, then the picture covers the box.
func TestPosterCoverLoadedIsDrawn(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := &fakeLoader{pics: map[string]*scene.Image{"https://x.test/a.jpg": green(t)}}
		p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0}, 100, 100)
		p.SetCover(f.load, "https://x.test/a.jpg")
		root := &stage{}
		root.put(p, 10, 10)
		tt := kittest.Mount(t, ph, root)
		tt.RunUntilIdle()

		assert.Equal(t, []string{"https://x.test/a.jpg"}, f.asked())
		snap := tt.Snapshot()
		for _, pt := range []image.Point{{11, 11}, {109, 109}, {60, 60}} {
			c := snap.RGBAAt(pt.X, pt.Y)
			assert.Equal(t, uint8(255), c.G, "%v", pt)
			assert.Zero(t, c.R, "%v", pt)
		}
	})
}

// TestPosterCoverFailureKeepsTheDrawing leaves the vector poster when the load
// fails (offline, the mock's cover that the app refuses) and when there is no
// address.
func TestPosterCoverFailureKeepsTheDrawing(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := &fakeLoader{}
		failing := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0, NoTitle: true}, 100, 100)
		failing.SetCover(f.load, "https://x.test/missing.jpg")
		none := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0, NoTitle: true}, 100, 100)
		none.SetCover(f.load, "")
		bare := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0, NoTitle: true}, 100, 100)
		root := &stage{}
		root.put(failing, 10, 10)
		root.put(none, 200, 10)
		root.put(bare, 10, 200)
		tt := kittest.Mount(t, ph, root)
		tt.RunUntilIdle()

		assert.Equal(t, []string{"https://x.test/missing.jpg"}, f.asked(), "no address, no call")
		snap := tt.Snapshot()
		want := snap.RGBAAt(60, 250) // the poster nobody asked a cover for
		assert.Equal(t, want, snap.RGBAAt(60, 60), "the failed load keeps the vector poster")
		assert.Equal(t, want, snap.RGBAAt(250, 60))
	})
}

// TestPosterCoverLoadEndsWithThePoster cancels the load of a cover when the
// poster leaves the view.
func TestPosterCoverLoadEndsWithThePoster(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := &fakeLoader{block: true, started: make(chan struct{}, 1)}
		p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 0}, 100, 100)
		root := &stage{}
		root.put(p, 10, 10)
		tt := kittest.Mount(t, ph, root)
		p.SetCover(f.load, "https://x.test/slow.jpg") // in the view: the load runs off the UI thread
		<-f.started
		assert.False(t, f.canceled.Load(), "the load is under way")

		root.drop(p)
		tt.Elapse(kittest.Frame)
		tt.RunUntilIdle()
		assert.True(t, f.canceled.Load(), "the poster left: its load was canceled")
	})
}

// TestPosterCoverChangesWithTheAddress drops the old picture at a new address
// and asks once per address.
func TestPosterCoverChangesWithTheAddress(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := &fakeLoader{pics: map[string]*scene.Image{"https://x.test/a.jpg": green(t)}}
		p := poster.New(poster.Spec{ID: "set-35", Radius: 0, NoTitle: true}, 100, 100)
		p.SetCover(f.load, "https://x.test/a.jpg")
		root := &stage{}
		root.put(p, 10, 10)
		tt := kittest.Mount(t, ph, root)
		tt.RunUntilIdle()
		require.Equal(t, uint8(255), tt.Snapshot().RGBAAt(60, 60).G)

		p.SetCover(f.load, "https://x.test/a.jpg") // the same address: nothing new
		tt.RunUntilIdle()
		assert.Len(t, f.asked(), 1)

		p.SetCover(f.load, "https://x.test/b.jpg") // another cover that cannot be had
		tt.RunUntilIdle()
		tt.Elapse(kittest.Frame)
		assert.Len(t, f.asked(), 2)
		assert.NotEqual(t, uint8(255), tt.Snapshot().RGBAAt(60, 60).G, "the old picture is not the new course's")
	})
}

// TestPosterCoverPaintDoesNotAllocate draws a poster with a cover on a steady
// frame.
func TestPosterCoverPaintDoesNotAllocate(t *testing.T) {
	// Not parallel: AllocsPerRun.
	for _, ph := range kittest.Phones {
		t.Run(ph.Name, func(t *testing.T) {
			f := &fakeLoader{pics: map[string]*scene.Image{"https://x.test/a.jpg": green(t)}}
			p := poster.New(poster.Spec{ID: "set-35", Title: "SET 35", Radius: 20}, 156, 100)
			p.SetCover(f.load, "https://x.test/a.jpg")
			root := &stage{}
			root.put(p, 10, 10)
			tt := kittest.Mount(t, ph, root)
			tt.RunUntilIdle()
			for range 3 {
				tt.Elapse(kittest.Frame)
			}
			allocs := testing.AllocsPerRun(20, func() {
				root.Repaint()
				tt.Elapse(kittest.Frame)
			})
			assert.Zero(t, allocs, "steady frame")
		})
	}
}

// TestPosterTitleStaysInTheBox lays out a title far wider than a 156 x 100
// poster: the block is bounded to the room right of its 8 % left edge (the
// name wraps, as `div.ttl` does), and the paint is clipped to the poster.
func TestPosterTitleStaysInTheBox(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := poster.New(poster.Spec{ID: "set-35", Title: "MASTER TRAINER EXTENDED COURSE OF SOMETHING", Radius: 16, TitleSize: 20}, 156, 100)
		root := &stage{}
		root.put(p, 10, 10)
		tt := kittest.Mount(t, ph, root)

		_, r, ok := tt.Find("div.ttl")
		require.True(t, ok)
		pf := p.Frame()
		assert.GreaterOrEqual(t, r.X, pf.X-.01, "left")
		assert.LessOrEqual(t, r.X+r.W, pf.X+pf.W+.01, "the title block does not pass the poster's right edge")
		_, b, ok := tt.Find("b")
		require.True(t, ok)
		assert.Greater(t, b.H, float32(20), "a long name wraps to more than one line")

		snap := tt.Snapshot()
		for _, pt := range []image.Point{{10 + 156, 60}, {10 + 160, 60}, {200, 60}} {
			c := snap.RGBAAt(pt.X, pt.Y)
			assert.Equal(t, snap.RGBAAt(300, 300), c, "outside the poster is the background: %v", pt)
		}
	})
}
