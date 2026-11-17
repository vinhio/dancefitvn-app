// Adapted from jplayer-mobile/internal/media/avf/shim_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

/*
#cgo CFLAGS: -x objective-c -fno-objc-arc
#cgo LDFLAGS: -framework AVFoundation -framework CoreVideo -framework CoreMedia -framework QuartzCore -framework Foundation

#import <AVFoundation/AVFoundation.h>
#import <CoreVideo/CoreVideo.h>
#import <QuartzCore/QuartzCore.h>
#include <stdint.h>
#include <string.h>

// jp_avf_pull is the whole per-frame path (04 §2.2, approved at M0 Gate 1):
// the item time for the host's time now (CACurrentMediaTime,
// AVPlayerItemOutput.h:62), whether the output has a new pixel buffer for
// it (:217; skipped when force is set, to take the frame shown whatever it
// is), the buffer (:234, retained), its BGRA rows copied into dst (stride
// bytes apart, w by h pixels, every pixel written: what the buffer lacks is
// opaque black), and the buffer released. It answers 1 when it copied a
// frame, 0 when there was none. It allocates nothing.
static int jp_avf_pull(uintptr_t output, unsigned char *dst, long stride, long w, long h, int force) {
	AVPlayerItemVideoOutput *o = (AVPlayerItemVideoOutput *)output;
	CMTime t = [o itemTimeForHostTime:CACurrentMediaTime()];
	if (!force && ![o hasNewPixelBufferForItemTime:t]) {
		return 0;
	}
	CVPixelBufferRef pb = [o copyPixelBufferForItemTime:t itemTimeForDisplay:NULL];
	if (pb == NULL) {
		return 0;
	}
	if (CVPixelBufferGetPixelFormatType(pb) != kCVPixelFormatType_32BGRA ||
		CVPixelBufferLockBaseAddress(pb, kCVPixelBufferLock_ReadOnly) != kCVReturnSuccess) {
		CVPixelBufferRelease(pb);
		return 0;
	}
	const unsigned char *src = CVPixelBufferGetBaseAddress(pb);
	size_t from = CVPixelBufferGetBytesPerRow(pb);
	long pw = (long)CVPixelBufferGetWidth(pb), ph = (long)CVPixelBufferGetHeight(pb);
	if (src == NULL) {
		pw = 0;
		ph = 0;
	}
	if (pw > w) pw = w;
	if (ph > h) ph = h;
	for (long y = 0; y < h; y++) {
		unsigned char *row = dst + y * stride;
		long x = 0;
		if (y < ph) {
			memcpy(row, src + (size_t)y * from, (size_t)pw * 4);
			x = pw;
		}
		for (; x < w; x++) {
			row[4 * x] = 0;
			row[4 * x + 1] = 0;
			row[4 * x + 2] = 0;
			row[4 * x + 3] = 255;
		}
	}
	CVPixelBufferUnlockBaseAddress(pb, kCVPixelBufferLock_ReadOnly);
	CVPixelBufferRelease(pb);
	return 1;
}

// jp_avf_has_new tells whether the output has a new pixel buffer for the
// item time of the host's time now, without taking it (AVPlayerItemOutput.h:62,
// :217): 1 if so. It allocates nothing.
static int jp_avf_has_new(uintptr_t output) {
	AVPlayerItemVideoOutput *o = (AVPlayerItemVideoOutput *)output;
	return [o hasNewPixelBufferForItemTime:[o itemTimeForHostTime:CACurrentMediaTime()]] ? 1 : 0;
}
*/
import "C"

import (
	"unsafe"

	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/gdeskdev/desk/foundation/scene"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// pullInto copies the output's frame into img when there is a new one, or,
// with force, whatever frame the output shows now. It tells whether it did.
// One crossing of cgo, no allocation.
func pullInto(output objc.ID, img *scene.Image, force bool) bool {
	f := C.int(0)
	if force {
		f = 1
	}

	return C.jp_avf_pull(C.uintptr_t(output), (*C.uchar)(unsafe.Pointer(&img.Pix[0])), //nolint:gosec // G103: Go memory without Go pointers, read by C during the call
		C.long(img.Stride), C.long(img.Width), C.long(img.Height), f) == 1
}

// hasNew tells whether output has a new frame now, leaving it there. One
// crossing of cgo, no allocation.
func hasNew(output objc.ID) bool { return C.jp_avf_has_new(C.uintptr_t(output)) == 1 }
