package app

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noise returns a w × h opaque picture of random pixels, which compresses
// badly: a few megapixels weigh megabytes.
func noise(w, h int) *image.NRGBA {
	r := rand.New(rand.NewPCG(uint64(w), uint64(h)))
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = uint8(r.UintN(256))
	}
	for i := 3; i < len(img.Pix); i += 4 {
		img.Pix[i] = 0xFF
	}

	return img
}

// encJPEG writes img as a JPEG of quality q.
func encJPEG(t testing.TB, img image.Image, q int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}))

	return buf.Bytes()
}

// encPNG writes img as a PNG.
func encPNG(t testing.TB, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))

	return buf.Bytes()
}

// pngHeader returns the signature and the IHDR chunk of a w × h RGBA PNG
// and nothing more: DecodeConfig reads it, Decode fails.
func pngHeader(w, h uint32) []byte {
	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], w)
	binary.BigEndian.PutUint32(ihdr[4:], h)
	ihdr[8], ihdr[9] = 8, 6 // 8 bits, RGBA
	b := []byte("\x89PNG\r\n\x1a\n")
	b = binary.BigEndian.AppendUint32(b, uint32(len(ihdr)))
	chunk := append([]byte("IHDR"), ihdr[:]...)
	b = append(b, chunk...)

	return binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(chunk))
}

func TestFitArtwork(t *testing.T) {
	t.Parallel()
	smallJPEG := encJPEG(t, noise(300, 200), 90)
	smallPNG := encPNG(t, noise(64, 48))
	bigJPEG := encJPEG(t, noise(2400, 1600), 98)
	tests := []struct {
		name   string
		in     []byte
		big    bool // the input weighs over artMaxBytes
		same   bool // handed over as it is
		w, h   int  // the size made
		errIs  error
		format string
	}{
		{name: "small JPEG passes", in: smallJPEG, same: true, w: 300, h: 200, format: "jpeg"},
		{name: "small PNG passes", in: smallPNG, same: true, w: 64, h: 48, format: "png"},
		{name: "heavy landscape JPEG", in: bigJPEG, big: true, w: 1024, h: 683, format: "jpeg"},
		{name: "heavy portrait JPEG", in: encJPEG(t, noise(1600, 2400), 98), big: true, w: 683, h: 1024, format: "jpeg"},
		{name: "heavy PNG", in: encPNG(t, noise(1500, 1000)), big: true, w: 1024, h: 683, format: "jpeg"},
		{name: "light PNG too wide", in: encPNG(t, image.NewGray(image.Rect(0, 0, 3000, 100))), w: 1024, h: 34, format: "jpeg"},
		{name: "empty", in: nil, errIs: core.ErrInvalid},
		{name: "JPEG magic then garbage", in: append([]byte{0xff, 0xd8, 0xff}, bytes.Repeat([]byte{0x42}, 64)...), errIs: core.ErrInvalid},
		{name: "not a picture", in: []byte("<html>not found</html>"), errIs: core.ErrInvalid},
		{name: "heavy JPEG cut short", in: bigJPEG[:len(bigJPEG)/2], errIs: core.ErrInvalid},
		{name: "too many pixels", in: pngHeader(7000, 7000), errIs: core.ErrInvalid},
		{name: "too wide to decode", in: pngHeader(artMaxSide+1, 1), errIs: core.ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.big {
				require.Greater(t, len(tt.in), artMaxBytes, "the input must weigh over the limit")
			}
			out, err := fitArtwork(tt.in)
			if tt.errIs != nil {
				require.ErrorIs(t, err, tt.errIs)
				assert.Nil(t, out)

				return
			}
			require.NoError(t, err)
			assert.LessOrEqual(t, len(out), artMaxBytes)
			assert.True(t, isPNGOrJPEG(out))
			if tt.same {
				assert.Equal(t, tt.in, out)
			}
			cfg, format, err := image.DecodeConfig(bytes.NewReader(out))
			require.NoError(t, err)
			assert.Equal(t, tt.format, format)
			assert.Equal(t, [2]int{tt.w, tt.h}, [2]int{cfg.Width, cfg.Height})
			assert.LessOrEqual(t, max(cfg.Width, cfg.Height), artPassSide)
		})
	}
}

func TestFitSide(t *testing.T) {
	t.Parallel()
	tests := []struct {
		w, h, side, ww, wh int
	}{
		{w: 100, h: 50, side: 1024, ww: 100, wh: 50},
		{w: 1024, h: 1024, side: 1024, ww: 1024, wh: 1024},
		{w: 4096, h: 2048, side: 1024, ww: 1024, wh: 512},
		{w: 2048, h: 4096, side: 1024, ww: 512, wh: 1024},
		{w: 3000, h: 2000, side: 1024, ww: 1024, wh: 683},
		{w: 8192, h: 1, side: 1024, ww: 1024, wh: 1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%dx%d", tt.w, tt.h), func(t *testing.T) {
			t.Parallel()
			w, h := fitSide(tt.w, tt.h, tt.side)
			assert.Equal(t, [2]int{tt.ww, tt.wh}, [2]int{w, h})
		})
	}
}

func TestBoxReduce(t *testing.T) {
	t.Parallel()
	red := color.NRGBA{R: 0xFF, A: 0xFF}
	blue := color.NRGBA{B: 0xFF, A: 0xFF}
	halves := image.NewNRGBA(image.Rect(10, 20, 14, 24)) // not at the origin
	for y := 20; y < 24; y++ {
		for x := 10; x < 14; x++ {
			if x < 12 {
				halves.Set(x, y, red)
			} else {
				halves.Set(x, y, blue)
			}
		}
	}
	checker := image.NewGray(image.Rect(0, 0, 2, 2))
	checker.SetGray(0, 0, color.Gray{Y: 0xFF})
	checker.SetGray(1, 1, color.Gray{Y: 0xFF})
	clearPx := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	clearPx.Set(0, 0, color.NRGBA{R: 0xFF, A: 0xFF})
	tests := []struct {
		name string
		src  image.Image
		w, h int
		want []color.RGBA
	}{
		{name: "halves kept apart", src: halves, w: 2, h: 2, want: []color.RGBA{
			{R: 0xFF, A: 0xFF}, {B: 0xFF, A: 0xFF}, {R: 0xFF, A: 0xFF}, {B: 0xFF, A: 0xFF},
		}},
		{name: "halves to one column", src: halves, w: 1, h: 1, want: []color.RGBA{{R: 0x80, B: 0x80, A: 0xFF}}},
		{name: "checker averaged", src: checker, w: 1, h: 1, want: []color.RGBA{{R: 0x80, G: 0x80, B: 0x80, A: 0xFF}}},
		{name: "transparent averaged premultiplied", src: clearPx, w: 1, h: 1, want: []color.RGBA{{R: 0x80, A: 0x80}}},
		{name: "same size copied", src: checker, w: 2, h: 2, want: []color.RGBA{
			{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, {A: 0xFF}, {A: 0xFF}, {R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dst := image.NewRGBA(image.Rect(0, 0, tt.w, tt.h))
			boxReduce(dst, tt.src)
			got := make([]color.RGBA, 0, tt.w*tt.h)
			for y := range tt.h {
				for x := range tt.w {
					got = append(got, dst.RGBAAt(x, y))
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// artFetcher is an artFetch over files by URL that counts its reads.
type artFetcher struct {
	mu    sync.Mutex
	files map[string][]byte
	reads map[string]int
}

func (f *artFetcher) fetch(_ context.Context, url string, limit int64) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads[url]++
	b, ok := f.files[url]
	if !ok {
		return nil, fmt.Errorf("fetch: status 404: %w", core.ErrNotFound)
	}

	return b[:min(int64(len(b)), limit)], nil
}

func TestLoadArtwork(t *testing.T) {
	t.Parallel()
	small := encJPEG(t, noise(200, 200), 90)
	heavy := encJPEG(t, noise(2400, 1600), 98)
	require.Greater(t, len(heavy), artMaxBytes)
	tests := []struct {
		name    string
		file    []byte // nil: not found
		errIs   error
		reduced bool // a line tells of the reduction
		cached  bool // the second load reads nothing
	}{
		{name: "small cover kept as it is", file: small, cached: true},
		{name: "heavy cover reduced", file: heavy, reduced: true, cached: true},
		{name: "corrupt cover", file: []byte{0xff, 0xd8, 0xff, 0, 1, 2}, errIs: core.ErrInvalid},
		{name: "over the read limit", file: make([]byte, artReadMaxBytes+10), errIs: core.ErrInvalid},
		{name: "not found", errIs: core.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			const url = "https://cdn.example.test/covers/c1.jpg"
			f := &artFetcher{files: map[string][]byte{}, reads: map[string]int{}}
			if tt.file != nil {
				f.files[url] = tt.file
			}
			var c artCache
			var lines []string
			logf := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
			for range 2 {
				out, err := loadArtwork(context.Background(), &c, url, f.fetch, logf)
				if tt.errIs != nil {
					require.ErrorIs(t, err, tt.errIs)
					assert.Nil(t, out)

					continue
				}
				require.NoError(t, err)
				assert.LessOrEqual(t, len(out), artMaxBytes)
				if !tt.reduced {
					assert.Equal(t, tt.file, out)
				}
			}
			if tt.cached {
				assert.Equal(t, 1, f.reads[url])
			} else {
				assert.Equal(t, 2, f.reads[url], "a failure is read again")
			}
			if tt.reduced {
				require.Len(t, lines, 1)
				assert.Contains(t, lines[0], "reduced")
			} else {
				assert.Empty(t, lines)
			}
		})
	}
}

func TestLoadArtworkCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &artFetcher{files: map[string][]byte{"u": encJPEG(t, noise(10, 10), 90)}, reads: map[string]int{}}
	var c artCache
	_, err := loadArtwork(ctx, &c, "u", f.fetch, nil)
	require.ErrorIs(t, err, context.Canceled)
	_, ok := c.get("u")
	assert.False(t, ok)
}

func TestArtCacheKeepsTheLast(t *testing.T) {
	t.Parallel()
	var c artCache
	for i := range artKept + 2 {
		c.put(fmt.Sprintf("u%d", i), []byte{byte(i)})
	}
	for i := range artKept + 2 {
		b, ok := c.get(fmt.Sprintf("u%d", i))
		if i < 2 {
			assert.False(t, ok, "u%d was the oldest", i)

			continue
		}
		require.True(t, ok, "u%d", i)
		assert.Equal(t, []byte{byte(i)}, b)
	}
	c.put("u5", []byte{0x55})
	b, _ := c.get("u5")
	assert.Equal(t, []byte{0x55}, b)
	c.put("", []byte{1})
	_, ok := c.get("")
	assert.False(t, ok)
}

func TestArtCacheIgnoresTheSignature(t *testing.T) {
	t.Parallel()
	var c artCache
	c.put("https://cdn.example.test/covers/c1.jpg?Expires=1&Signature=a", []byte{1})
	tests := []struct {
		url string
		hit bool
	}{
		{url: "https://cdn.example.test/covers/c1.jpg?Expires=2&Signature=b", hit: true},
		{url: "https://cdn.example.test/covers/c1.jpg", hit: true},
		{url: "https://cdn.example.test/covers/c1.jpg#top", hit: true},
		{url: "https://cdn.example.test/covers/c2.jpg?Expires=1&Signature=a"},
		{url: "https://other.example.test/covers/c1.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			t.Parallel()
			_, ok := c.get(tt.url)
			assert.Equal(t, tt.hit, ok)
		})
	}
}

func BenchmarkFitArtworkHeavy(b *testing.B) {
	in := encJPEG(b, noise(2400, 1600), 98)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := fitArtwork(in); err != nil {
			b.Fatal(err)
		}
	}
}
