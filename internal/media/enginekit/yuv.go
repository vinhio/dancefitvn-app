// Adapted from jplayer-mobile/internal/media/enginekit/yuv.go at 49de3c2.

package enginekit

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The fixed-point (16 bits) coefficients of the YUV to RGB conversion, for
// the narrow range (Y 16 to 235, U and V 16 to 240) of video.
const (
	yScale = 76309 // 255/219

	rv601, gu601, gv601, bu601 = 104597, 25675, 53279, 132201
	rv709, gu709, gv709, bu709 = 117489, 13975, 34925, 138438

	// HDEdge is the height from which video is taken to be BT.709 rather
	// than BT.601.
	HDEdge = 720
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// YUV420ToRGBA converts a w by h 4:2:0 picture to opaque R, G, B, A bytes in
// dst, rows dstStride bytes apart. y has the luma, rows yStride apart; u and
// v the chroma at half the size, rows uvStride apart and uvPixelStride bytes
// between two samples: 1 for planes (I420), 2 for the interleaved NV12 and
// NV21 (u and v are then the same buffer, one byte apart). The range is the
// narrow one and the matrix BT.709 when bt709, else BT.601 (HD takes 709: see
// HDEdge).
//
// It is the reference the C version of the Android engine is checked
// against, and a fallback. It allocates nothing. Buffers too short for the
// picture leave dst as it is.
func YUV420ToRGBA(dst []byte, dstStride, w, h int, y []byte, yStride int, u, v []byte, uvStride, uvPixelStride int, bt709 bool) {
	if w <= 0 || h <= 0 || uvPixelStride <= 0 || dstStride < 4*w || yStride < w {
		return
	}
	cw, ch := (w+1)/2, (h+1)/2
	if len(dst) < dstStride*(h-1)+4*w || len(y) < yStride*(h-1)+w ||
		len(u) < uvStride*(ch-1)+uvPixelStride*(cw-1)+1 || len(v) < uvStride*(ch-1)+uvPixelStride*(cw-1)+1 {
		return
	}
	rv, gu, gv, bu := rv601, gu601, gv601, bu601
	if bt709 {
		rv, gu, gv, bu = rv709, gu709, gv709, bu709
	}
	for row := range h {
		yr := y[row*yStride : row*yStride+w]
		out := dst[row*dstStride : row*dstStride+4*w]
		cr := (row >> 1) * uvStride
		for x := range w {
			ci := cr + (x>>1)*uvPixelStride
			yy := yScale * (int(yr[x]) - 16)
			cb, cv := int(u[ci])-128, int(v[ci])-128
			o := out[4*x : 4*x+4]
			o[0] = clamp8((yy + rv*cv + 1<<15) >> 16)
			o[1] = clamp8((yy - gu*cb - gv*cv + 1<<15) >> 16)
			o[2] = clamp8((yy + bu*cb + 1<<15) >> 16)
			o[3] = 0xff
		}
	}
}

// BT709For tells whether a picture h pixels high is converted with BT.709:
// from HDEdge up.
func BT709For(h int) bool { return h >= HDEdge }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// clamp8 limits n to 0 to 255.
func clamp8(n int) byte {
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}

	return byte(n)
}
