// Adapted from jplayer-mobile/internal/media/surface.go at 49de3c2.

package media

import (
	"sync/atomic"

	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The fields of Surface.state: three slot numbers, each in two bits, and
// the flag that the ready slot holds a frame the UI has not taken.
//
// The zero state must be a valid one, so the ready and back numbers are kept
// XORed with their values at rest: front 0, ready 1, back 2.
const (
	slots      = 3
	readyShift = 2
	backShift  = 4
	slotMask   = 3
	freshBit   = 1 << 6
	readyFlip  = 1
	backFlip   = 2
)

// Surface is a video's frames, triple-buffered: a producer (the engine's
// thread, or the UI thread for an engine the UI pulls from) draws into the
// back image and publishes it; the UI thread latches the newest one as the
// front at the start of a paint. The producer never touches the front, nor
// the UI thread the back, so no frame is copied, nothing is locked, and
// nothing allocates once the three images have the video's size.
//
// One goroutine produces (Back, Publish, SetPull); the UI thread does the
// rest. Free and Reset need the producer to have stopped, or at least to
// publish nothing while they run: a frame published meanwhile is dropped.
//
// The latch is once per paint (Frame's paint number): the views of one
// surface in one paint (the mini player and Now Playing) draw the same
// image, even when the producer publishes between them. Were the second
// to latch again, the first one's image would go back to the producer as
// the next back while the scene of the paint still points to it.
type Surface struct {
	slots   [slots]*scene.Image
	gens    [slots]uint64 // the Reset count each slot's frame was drawn under
	state   atomic.Uint32 // front, ready and back slots, and the fresh flag
	seq     atomic.Uint64 // frames published
	gen     atomic.Uint64 // Reset count
	backGen uint64        // producer: gen when Back was last called
	backOut bool          // producer: Back was called and Publish was not
	watch   atomic.Int32  // VideoViews showing the surface
	active  atomic.Bool
	rot     atomic.Int32 // 0, 90, 180 or 270
	pull    atomic.Pointer[func()]
	onFree  atomic.Pointer[func()]
	held    atomic.Int32 // the slots holding an image
	frames  event.Topic[uint64]
	painted uint64 // UI thread: the paint number of the last latch

	// kept is the frame Keep copied, the app's own: Free and Reset leave it
	// alone, so the views draw it while the engine has no frame of the media
	// (M17, docs/plan/12-paused-frame.md). keptRot is the turn it needs and
	// keptOf the Version of the frame it is a copy of. UI thread.
	kept    *scene.Image
	keptRot int
	keptOf  uint64
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Frame returns the frame of the paint numbered paint, with its sequence
// number, which grows with every Publish. The first call of a paint latches
// the newest frame the producer published, if any; the other calls with the
// same number give that frame again, whatever was published since, so every
// widget of the paint draws the same image and the image a scene of the
// paint points to stays the front until the next paint. paint 0 is no
// paint: every call with it latches (a probe, a test). The image is nil
// before the first frame, and after Reset until a frame published after it
// (the number is then 0).
//
// Call it on the UI thread at the start of a paint, with a number that
// changes from one paint to the next (ui.View.FramesDrawn): the image of
// the paint before is then done with. It allocates nothing.
func (s *Surface) Frame(paint uint64) (img *scene.Image, seq uint64) {
	var st uint32
	if paint != 0 && paint == s.painted {
		st = s.state.Load()
	} else {
		s.painted = paint
		st = s.latch()
	}
	f := frontOf(st)
	img = s.slots[f]
	if img == nil || s.gens[f] != s.gen.Load() {
		return nil, 0
	}

	return img, img.Version
}

// Back returns the image the producer draws the next frame into, w by h
// pixels, in B, G, R, A order when bgra. The image is made again only when
// the size changes, on the producer's thread. A size that is not positive
// gives nil. Its pixels are those of an older frame, so draw all of them.
func (s *Surface) Back(w, h int, bgra bool) *scene.Image {
	b := backOf(s.state.Load())
	img := s.slots[b]
	if img == nil || img.Width != w || img.Height != h {
		var err error
		img, err = scene.NewImage(w, h)
		if err != nil {
			s.backOut = false

			return nil
		}
		if s.slots[b] == nil {
			s.held.Add(1)
		}
		s.slots[b] = img
	}
	img.BGRA = bgra
	img.Opaque = true
	s.backGen = s.gen.Load()
	s.backOut = true

	return img
}

// Publish makes the back image the newest frame and takes the slot of the
// frame not yet latched, if any, as the next back, so no frame is copied.
// The image's Version becomes the frame's sequence number, which tells
// renderers to upload it again. It allocates nothing. A Publish without a
// Back does nothing, and so does one of a back taken before a Reset.
func (s *Surface) Publish() {
	if !s.backOut {
		return
	}
	s.backOut = false
	if s.backGen != s.gen.Load() {
		return
	}
	n := s.seq.Load() + 1
	for {
		st := s.state.Load()
		b := backOf(st)
		img := s.slots[b]
		if img == nil {
			return
		}
		img.Version = n
		s.gens[b] = s.backGen
		// Back and ready change places; the fresh flag says the UI has not
		// seen the new ready frame.
		if s.state.CompareAndSwap(st, compose(frontOf(st), b, readyOf(st), true)) {
			break
		}
	}
	s.seq.Store(n)
}

// Reset forgets the picture: Frame gives nil until a frame published after
// the call. It is for a new media, whose first frame must not wait behind the
// last frame of the one before. The frame Keep kept stays (Unkeep).
func (s *Surface) Reset() { s.gen.Add(1) }

// Gen counts the Resets and Frees: it changes as the engine loads another
// media (Load resets, Stop frees), so a frame on screen drawn under an
// earlier count is not the media loaded now. Any thread.
func (s *Surface) Gen() uint64 { return s.gen.Load() }

// Free drops the three images, which a video of 1280 by 720 makes 11 MB, and
// resets; the next Back makes them again. The view keeps the scenes of its
// last frames, which point to the images drawn in them and which no paint
// replaces in the background: so that those images hold no memory either,
// each gives its pixels up and becomes one opaque black pixel, with a new
// Version, and a renderer drawing such a scene again draws black, as before
// a first frame. When it dropped any image it tells the views to paint
// again (Notify), which then draw the frame Keep kept, if any, and runs the
// func of SetOnFree, after. The kept frame stays. Call it on the UI thread
// (a scene's images are read there, by the renderer), with the producer
// stopped: when the media is audio, or stopped, or the app in the
// background, or the engine closed.
func (s *Surface) Free() {
	s.gen.Add(1)
	dropped := false
	for i, img := range s.slots {
		if img == nil {
			continue
		}
		dropped = true
		blacken(img)
		s.slots[i] = nil
	}
	s.held.Store(0)
	s.gens = [slots]uint64{}
	s.state.Store(0)
	s.backOut = false
	if !dropped {
		return
	}
	// The views paint again: the image their last scene drew is black now,
	// and they draw the kept frame, or what lies under them, instead.
	s.Notify()
	if fn := s.onFree.Load(); fn != nil {
		(*fn)()
	}
}

// Keep copies the frame on screen, the front the last paint latched, into
// an image of the surface's own that Free and Reset leave alone, and keeps
// it until Unkeep: the frame a paused, stalled, reloading or failed video
// shows while the engine has none to give (M17, docs/plan/12-paused-frame.md
// P01-P03). It tells whether a frame is kept now: with no frame on screen
// (none yet, or Reset or Free since) it copies nothing and the frame kept
// before, if any, stays. The copy is made once per frame (a Keep of the
// frame kept already copies nothing) into one image, made again only when
// the size changes, so it allocates nothing once that image has the
// video's size. It does not latch. UI thread.
func (s *Surface) Keep() bool {
	f := frontOf(s.state.Load())
	img := s.slots[f]
	if img == nil || s.gens[f] != s.gen.Load() {
		return s.kept != nil // no frame of the media loaded on screen
	}
	if len(img.Pix) < img.Stride*(img.Height-1)+4*img.Width {
		return s.kept != nil
	}
	k := s.kept
	same := k != nil && k.Width == img.Width && k.Height == img.Height
	if same && s.keptOf == img.Version && k.BGRA == img.BGRA {
		s.keptRot = s.Rotation()

		return true
	}
	if !same {
		nk, err := scene.NewImage(img.Width, img.Height)
		if err != nil {
			return s.kept != nil
		}
		if k != nil {
			nk.Version = k.Version
			blacken(k) // a scene still drawing it holds no memory
		}
		k = nk
	}
	row := 4 * img.Width
	if img.Stride == k.Stride {
		copy(k.Pix, img.Pix[:img.Stride*(img.Height-1)+row])
	} else {
		for y := range img.Height {
			copy(k.Pix[y*k.Stride:y*k.Stride+row], img.Pix[y*img.Stride:y*img.Stride+row])
		}
	}
	k.BGRA, k.Opaque = img.BGRA, true
	k.Version++
	s.kept, s.keptRot, s.keptOf = k, s.Rotation(), img.Version

	return true
}

// Kept returns the frame Keep copied and the turn it needs (Rotation's), nil
// when none is kept. It allocates nothing. UI thread.
func (s *Surface) Kept() (img *scene.Image, rot int) { return s.kept, s.keptRot }

// Unkeep lets the kept frame go: another media is current, or none, or the
// system is short of memory (M17 P03, P07). As Free does with its images,
// the image gives its pixels up and becomes one black pixel with a new
// Version, so that the scenes still drawing it, which no paint replaces
// while hidden, hold no memory. The views paint again. UI thread.
func (s *Surface) Unkeep() {
	if s.kept == nil {
		return
	}
	blacken(s.kept)
	s.kept, s.keptRot, s.keptOf = nil, 0, 0
	s.Notify()
}

// Holds tells that the surface holds images: a frame was drawn since the
// last Free. Any thread.
func (s *Surface) Holds() bool { return s.held.Load() > 0 }

// SetOnFree sets what Free runs once it dropped images, on the UI thread: the
// app gives the memory back to the system then. nil runs nothing. Call it on
// the UI thread.
func (s *Surface) SetOnFree(fn func()) {
	if fn == nil {
		s.onFree.Store(nil)

		return
	}
	s.onFree.Store(&fn)
}

// SetPull sets what Pull runs, for an engine that gets its frames when the UI
// asks (iOS), on the UI thread. Call it once, when the engine is made.
func (s *Surface) SetPull(fn func()) {
	if fn == nil {
		s.pull.Store(nil)

		return
	}
	s.pull.Store(&fn)
}

// Pull asks the engine for a frame, on the UI thread: VideoView.Paint calls
// it while Active. It does nothing for an engine without a pull.
func (s *Surface) Pull() {
	if fn := s.pull.Load(); fn != nil {
		(*fn)()
	}
}

// Pulls tells that the engine has a pull, so a view repaints every frame
// while Active to call it.
func (s *Surface) Pulls() bool { return s.pull.Load() != nil }

// SetActive tells that frames are coming: the media plays, or settles after a
// seek. A view that Pulls repaints each frame while it is.
func (s *Surface) SetActive(on bool) { s.active.Store(on) }

// Active tells that frames are coming.
func (s *Surface) Active() bool { return s.active.Load() }

// Watch tells that a view shows the surface, until the returned function is
// called, which may be called more than once. It allocates once per mount.
func (s *Surface) Watch() (unwatch func()) {
	s.watch.Add(1)
	var done atomic.Bool

	return func() {
		if done.CompareAndSwap(false, true) {
			s.watch.Add(-1)
		}
	}
}

// Watched tells that a view shows the surface, so that a producer off the UI
// thread has someone to copy frames for.
func (s *Surface) Watched() bool { return s.watch.Load() > 0 }

// SetRotation sets how far the frames turn clockwise to stand upright: deg is
// rounded to a multiple of 90 and taken modulo 360.
func (s *Surface) SetRotation(deg int) {
	q := (deg + 45) / 90
	if deg < 0 {
		q = -((-deg + 45) / 90)
	}
	s.rot.Store(int32(((q%4)+4)%4) * 90)
}

// Rotation returns the turn the frames need, 0, 90, 180 or 270 degrees.
func (s *Surface) Rotation() int { return int(s.rot.Load()) }

// Frames is the topic Notify publishes to.
func (s *Surface) Frames() *event.Topic[uint64] { return &s.frames }

// Notify publishes the sequence number of the newest frame to Frames, to
// tell the UI to paint. Call it on the UI thread: an engine whose producer
// is another thread hands it to Config.Dispatch, with a func made once.
func (s *Surface) Notify() { s.frames.Publish(s.seq.Load()) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// blacken makes img one opaque black pixel, whatever the order of its
// bytes, with a new Version: its pixels go to the garbage collector, and a
// renderer drawing a scene that points to it draws black.
func blacken(img *scene.Image) {
	img.Pix = []byte{0, 0, 0, 0xff}
	img.Width, img.Height, img.Stride = 1, 1, 4
	img.Version++
}

// latch makes the ready frame the front if the producer published one the
// UI has not taken, and returns the state after.
func (s *Surface) latch() uint32 {
	for {
		st := s.state.Load()
		if st&freshBit == 0 {
			return st
		}
		// Ready and front change places; the old front waits as the ready
		// slot, until the producer takes it as the back one.
		next := compose(readyOf(st), frontOf(st), backOf(st), false)
		if s.state.CompareAndSwap(st, next) {
			return next
		}
	}
}

// frontOf, readyOf and backOf read the slot numbers of a state.
func frontOf(st uint32) int { return int(st & slotMask) }

func readyOf(st uint32) int { return int((st>>readyShift)&slotMask ^ readyFlip) }

func backOf(st uint32) int { return int((st>>backShift)&slotMask ^ backFlip) }

// compose makes a state of the three slot numbers and the fresh flag.
func compose(front, ready, back int, fresh bool) uint32 {
	st := uint32(front) | uint32(ready^readyFlip)<<readyShift | uint32(back^backFlip)<<backShift
	if fresh {
		st |= freshBit
	}

	return st
}
