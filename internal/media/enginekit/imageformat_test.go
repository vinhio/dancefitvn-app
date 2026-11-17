// Adapted from jplayer-mobile/internal/media/enginekit/imageformat_test.go at 49de3c2.

package enginekit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReadableYUV420 checks which Android frame layouts the frame path
// converts: YUV_420_888 in its planar and semi-planar forms, and YV12,
// which the MediaTek decoder of a Galaxy A17 hands a YUV_420_888 reader
// (every frame of it was dropped before: video without a picture,
// 2026-11-08); not a private or RGB image, nor chroma planes laid out
// apart.
func TestReadableYUV420(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format int32
		planes int32
		pix    [3]int32
		row    [3]int32
		want   bool
	}{
		{"I420", ImageYUV420888, 3, [3]int32{1, 1, 1}, [3]int32{640, 320, 320}, true},
		{"NV12 or NV21", ImageYUV420888, 3, [3]int32{1, 2, 2}, [3]int32{1280, 1280, 1280}, true},
		{"YV12 of c2.mtk.avc.decoder", ImageYV12, 3, [3]int32{1, 1, 1}, [3]int32{640, 320, 320}, true},
		{"private", 0x22, 3, [3]int32{1, 1, 1}, [3]int32{640, 320, 320}, false},
		{"RGBA", 0x1, 1, [3]int32{4, 0, 0}, [3]int32{2560, 0, 0}, false},
		{"two planes", ImageYUV420888, 2, [3]int32{1, 2, 0}, [3]int32{640, 640, 0}, false},
		{"chroma pixel strides differ", ImageYUV420888, 3, [3]int32{1, 1, 2}, [3]int32{640, 320, 320}, false},
		{"chroma row strides differ", ImageYV12, 3, [3]int32{1, 1, 1}, [3]int32{640, 320, 336}, false},
		{"no strides", ImageYV12, 3, [3]int32{}, [3]int32{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, ReadableYUV420(tt.format, tt.planes, tt.pix, tt.row))
		})
	}
}

// TestReadableYUV420DoesNotAllocate checks the check of each frame
// allocates nothing.
func TestReadableYUV420DoesNotAllocate(t *testing.T) {
	pix, row := [3]int32{1, 1, 1}, [3]int32{640, 320, 320}
	allocs := testing.AllocsPerRun(100, func() { _ = ReadableYUV420(ImageYV12, 3, pix, row) })
	assert.Zero(t, allocs)
}
