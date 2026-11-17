// Adapted from jplayer-mobile/internal/media/enginekit/imageformat.go at 49de3c2.

package enginekit

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The Android image formats the frame path of internal/media/amp reads.
// Its AImageReader asks for YUV_420_888, which is flexible: the decoder's
// gralloc picks the layout, and AImage_getFormat tells the one it picked.
// MediaTek decoders (Galaxy A17, c2.mtk.avc.decoder) hand YV12 out; every
// frame of them was once dropped as unreadable, so no video showed
// (2026-11-08).
const (
	// ImageYUV420888 is AIMAGE_FORMAT_YUV_420_888 (media/NdkImage.h) and
	// android.graphics.ImageFormat.YUV_420_888.
	ImageYUV420888 = 0x23
	// ImageYV12 is android.graphics.ImageFormat.YV12 (android.jar,
	// android-35: 842094169), which media/NdkImage.h does not name. Its
	// AImage planes come in the order Y, U (Cb), V (Cr), as those of
	// YUV_420_888 do, whatever order the buffer holds them in.
	ImageYV12 = 0x32315659
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// ReadableYUV420 tells whether an Android image of format, with planes
// planes of the given pixel and row strides, is a 4:2:0 picture the frame
// path converts: a luma plane then two chroma planes laid out alike (I420,
// YV12, NV12, NV21). It allocates nothing (it runs for every frame).
func ReadableYUV420(format, planes int32, pixStride, rowStride [3]int32) bool {
	switch format {
	case ImageYUV420888, ImageYV12:
	default:
		return false
	}

	return planes >= 3 && pixStride[1] > 0 && pixStride[1] == pixStride[2] && rowStride[1] == rowStride[2] &&
		rowStride[0] > 0 && rowStride[1] > 0
}
