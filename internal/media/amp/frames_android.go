// Adapted from jplayer-mobile/internal/media/amp/frames_android.go at 49de3c2.

//go:build android && cgo

package amp

/*
#cgo CFLAGS: -O2
#cgo LDFLAGS: -lmediandk -landroid -ldl
#include <stdlib.h>
#include "frames_android.h"
*/
import "C"

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"
	"runtime/cgo"
	"sort"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The parts of an AImage's data space (android/data_space.h) that pick
// the matrix: the standard field and its BT.709 and BT.601 values.
const (
	standardMask      = 63 << 16
	standardBT709     = 1 << 16
	standardBT601Low  = 2 << 16 // BT601_625
	standardBT601High = 5 << 16 // BT601_525_UNADJUSTED
)

// The size a reader is made with: the decoder sets the size of its own
// buffers (a rotated video's frames come unrotated, M4.2a), so this is
// only the default of the window.
const (
	readerWidth  = 640
	readerHeight = 360
)

// convRing is how many conversion times the producer keeps for Stats.
const convRing = 256

// producer is what the readers' thread works with: it converts the frames
// of the engine's current reader into the Surface and wakes the UI. One
// per engine, reached from C through a cgo.Handle made once.
type producer struct {
	surf     *media.Surface
	dispatch func(func())
	notifyFn func() // made once: clears pending, Notify
	pending  atomic.Bool
	back     atomic.Bool // the app is in the background: no copies

	frames  atomic.Int64 // frames the readers handed over
	copied  atomic.Int64 // converted and published
	dropped atomic.Int64 // not converted: nobody watching, or background
	bad     atomic.Int64 // could not be read
	conv    [convRing]atomic.Int64
	convN   atomic.Int64
}

// reader is one AImageReader the player of a load draws into, and the
// Surface the player is handed. The opener makes it (newReader), the UI
// thread or the opener closes it (close, from hand), the releaser frees
// it (free); C's reader thread delivers its frames.
type reader struct {
	rd      *C.amp_reader
	surface jni.Object // global
}

// liveReaders counts the readers made and not deleted (Stats).
var liveReaders atomic.Int64

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newProducer returns the producer of surf, and the handle C reaches it by.
func newProducer(surf *media.Surface, dispatch func(func())) (*producer, cgo.Handle) {
	p := &producer{surf: surf, dispatch: dispatch}
	p.notifyFn = p.notify

	return p, cgo.NewHandle(p)
}

// newReader makes a reader whose frames go to the producer of h, and the
// Surface of its window, in env: the opener's thread, as it opens a load.
func newReader(env jni.Env, h cgo.Handle) (*reader, error) {
	var st C.int32_t
	rd := C.amp_reader_new(readerWidth, readerHeight, C.uintptr_t(h), &st)
	if rd == nil {
		return nil, fmt.Errorf("amp: AImageReader: media_status %d: %w", int(st), core.ErrUnsupported)
	}
	local := jni.Object(C.amp_reader_surface(C.uintptr_t(env.Native()), rd))
	if local == 0 {
		C.amp_reader_delete(rd)

		return nil, fmt.Errorf("amp: ANativeWindow_toSurface: %w", core.ErrInvalid)
	}
	r := &reader{rd: rd, surface: newGlobal(env, local)}
	env.DeleteLocalRef(local)
	liveReaders.Add(1)

	return r, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// close stops the frames: none reaches Go after it returns. The UI thread
// or the opener's (hand).
func (r *reader) close() {
	if r != nil && r.rd != nil {
		C.amp_reader_close(r.rd)
	}
}

// sync waits for the frame the reader's thread may be handing over: the
// producer's flags stored before it are seen by every frame after. The UI
// thread.
func (r *reader) sync() {
	if r != nil && r.rd != nil {
		C.amp_reader_sync(r.rd)
	}
}

// free releases the Surface and deletes the reader, after close and once
// the player writes no more (released), in env: the releaser's thread.
func (r *reader) free(env jni.Env) {
	if r == nil || r.rd == nil {
		return
	}
	C.amp_reader_close(r.rd)
	if r.surface != 0 {
		if err := env.CallVoid(r.surface, ids.surfaceRelease); err != nil {
			logf("amp: Surface.release: %v", err)
		}
		deleteGlobal(env, r.surface)
		r.surface = 0
	}
	C.amp_reader_delete(r.rd)
	r.rd = nil
	liveReaders.Add(-1)
}

// stats returns the reader's callbacks, failed acquires and the last
// failure's media_status_t.
func (r *reader) stats() (calls, fails, last int64) {
	if r == nil || r.rd == nil {
		return 0, 0, 0
	}
	var out [3]C.int64_t
	C.amp_reader_stats(r.rd, &out[0])

	return int64(out[0]), int64(out[1]), int64(out[2])
}

// convertTimes returns the median and the 95th percentile of the last
// conversions.
func (p *producer) convertTimes() (p50, p95 time.Duration) {
	n := min(int(p.convN.Load()), convRing)
	if n == 0 {
		return 0, 0
	}
	v := make([]int64, n)
	for i := range n {
		v[i] = p.conv[i].Load()
	}
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })

	return time.Duration(v[n/2]), time.Duration(v[(n*95)/100])
}

// ConvertYUV420 converts a w by h 4:2:0 picture into dst as the engine
// converts a frame, with the NEON loop and the scaling to dw by dh (no
// larger): the device tests check it against enginekit.YUV420ToRGBA, and
// time it. u and v are the chroma, rows uvStride apart, uvPixelStride
// bytes between samples (NV12 and NV21: the same buffer, a byte apart).
func ConvertYUV420(dst []byte, dstStride, dw, dh, w, h int, y []byte, yStride int, u, v []byte, uvStride, uvPixelStride int, bt709 bool) (time.Duration, error) {
	cw, ch := (w+1)/2, (h+1)/2
	if w <= 0 || h <= 0 || dw <= 0 || dh <= 0 || dw > w || dh > h || uvPixelStride <= 0 ||
		dstStride < 4*dw || len(dst) < dstStride*(dh-1)+4*dw || len(y) < yStride*(h-1)+w ||
		len(u) < uvStride*(ch-1)+uvPixelStride*(cw-1)+1 || len(v) < uvStride*(ch-1)+uvPixelStride*(cw-1)+1 {
		return 0, fmt.Errorf("amp: convert %dx%d to %dx%d: %w", w, h, dw, dh, core.ErrInvalid)
	}
	ns := C.amp_convert_planes((*C.uint8_t)(unsafe.Pointer(&y[0])), C.int32_t(yStride),
		(*C.uint8_t)(unsafe.Pointer(&u[0])), (*C.uint8_t)(unsafe.Pointer(&v[0])), C.int32_t(uvStride),
		C.int32_t(uvPixelStride), C.int32_t(w), C.int32_t(h), (*C.uint8_t)(unsafe.Pointer(&dst[0])),
		C.int32_t(dstStride), C.int32_t(dw), C.int32_t(dh), cbool(bt709))
	if ns < 0 {
		return 0, fmt.Errorf("amp: convert %dx%d: %w", w, h, core.ErrInvalid)
	}

	return time.Duration(ns), nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// ampOnFrame runs on a reader's thread for each image it acquired, while C
// holds it: unless nobody watches, it converts the frame into the back
// image (scaled to fit 1280), publishes it and has the UI told, once per
// UI turn. It allocates nothing once the three images have the frame's
// size.
//
//export ampOnFrame
func ampOnFrame(ctx C.uintptr_t, f *C.amp_frame) {
	p, ok := cgo.Handle(ctx).Value().(*producer)
	if !ok {
		return
	}
	p.frames.Add(1)
	if p.back.Load() || !p.surf.Watched() {
		p.dropped.Add(1)

		return
	}
	w, h := int(f.crop[2]-f.crop[0]), int(f.crop[3]-f.crop[1])
	dw, dh := enginekit.Fit(w, h, enginekit.MaxEdge)
	pix := [3]int32{int32(f.pix_stride[0]), int32(f.pix_stride[1]), int32(f.pix_stride[2])}
	row := [3]int32{int32(f.row_stride[0]), int32(f.row_stride[1]), int32(f.row_stride[2])}
	if w <= 0 || h <= 0 || dw > w || dh > h || !enginekit.ReadableYUV420(int32(f.format), int32(f.planes), pix, row) {
		p.bad.Add(1)

		return
	}
	img := p.surf.Back(dw, dh, false)
	if img == nil {
		p.bad.Add(1)

		return
	}
	ns := C.amp_convert(f, (*C.uint8_t)(unsafe.Pointer(&img.Pix[0])), C.int32_t(img.Stride),
		C.int32_t(dw), C.int32_t(dh), cbool(bt709Of(int32(f.data_space), h)))
	if ns < 0 {
		p.bad.Add(1)

		return
	}
	p.conv[(p.convN.Add(1)-1)%convRing].Store(int64(ns))
	p.surf.Publish()
	p.copied.Add(1)
	if !p.pending.Swap(true) {
		p.dispatch(p.notifyFn)
	}
}

// notify tells the UI that frames came. UI thread.
func (p *producer) notify() {
	p.pending.Store(false)
	p.surf.Notify()
}

// bt709Of tells whether a frame of data space ds, h pixels high, is
// BT.709: the standard of its data space when the system tells (API 34),
// else from 720 lines up. The range is always the limited one: the
// emulator's decoder reports a full range it does not use (M4.2a).
func bt709Of(ds int32, h int) bool {
	if ds > 0 {
		switch s := ds & standardMask; {
		case s == standardBT709:
			return true
		case s >= standardBT601Low && s <= standardBT601High:
			return false
		}
	}

	return enginekit.BT709For(h)
}

// cbool is a C int of b.
func cbool(b bool) C.int32_t {
	if b {
		return 1
	}

	return 0
}
