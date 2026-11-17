package avatar

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tap is one source pixel's share in an output pixel, along one axis.
type tap struct {
	at int     // the source column or row
	w  float32 // its share of the output pixel
}

// span is the taps of one output column or row: taps[start:start+n],
// and the share they cover together (less than 1 where the footprint
// reaches past the clip, the rest being the fill).
type span struct {
	start, n int
	in       float32
}

// rows hands the resampler the source's rows as premultiplied RGBA
// bytes, converting each through image/draw (whose fast paths read
// YCbCr, NRGBA, Gray and Paletted) into a few cached rows, so that a
// decoded photo is never held twice.
type rows struct {
	src   image.Image
	rgba  *image.RGBA // src itself when it is RGBA: no copy
	slots []*image.RGBA
	keys  []int
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// resample draws into dst the part r of src, scaled to dst's size, as an
// area average where it shrinks (each output pixel the mean of the
// source it covers) and a bilinear blend where it grows; what r covers
// outside clip (and outside src) is fill. dst's alpha and colours stay
// premultiplied.
func resample(dst *image.RGBA, src image.Image, r, clip Rect, fill color.RGBA) {
	b := src.Bounds()
	clip = Rect{
		X0: max(clip.X0, float64(b.Min.X)), Y0: max(clip.Y0, float64(b.Min.Y)),
		X1: min(clip.X1, float64(b.Max.X)), Y1: min(clip.Y1, float64(b.Max.Y)),
	}
	dw, dh := dst.Bounds().Dx(), dst.Bounds().Dy()
	xs, xt := spans(dw, r.X0, r.X1, clip.X0, clip.X1)
	ys, yt := spans(dh, r.Y0, r.Y1, clip.Y0, clip.Y1)
	src0 := newRows(src, maxTaps(ys)+1)
	acc := make([]float32, 4*dw)
	fr, fg, fb, fa := float32(fill.R), float32(fill.G), float32(fill.B), float32(fill.A)
	for j := range dh {
		clear(acc)
		sy := ys[j]
		for _, ty := range yt[sy.start : sy.start+sy.n] {
			row := src0.row(ty.at)
			for i := range dw {
				sx := xs[i]
				var cr, cg, cb, ca float32
				for _, tx := range xt[sx.start : sx.start+sx.n] {
					p := row[4*(tx.at-b.Min.X) : 4*(tx.at-b.Min.X)+4 : 4*(tx.at-b.Min.X)+4]
					cr += tx.w * float32(p[0])
					cg += tx.w * float32(p[1])
					cb += tx.w * float32(p[2])
					ca += tx.w * float32(p[3])
				}
				a := acc[4*i : 4*i+4 : 4*i+4]
				a[0] += ty.w * cr
				a[1] += ty.w * cg
				a[2] += ty.w * cb
				a[3] += ty.w * ca
			}
		}
		out := dst.Pix[dst.PixOffset(dst.Rect.Min.X, dst.Rect.Min.Y+j):]
		for i := range dw {
			out := out[4*i : 4*i+4 : 4*i+4]
			a := acc[4*i : 4*i+4 : 4*i+4]
			rest := 1 - xs[i].in*sy.in
			out[0] = clamp8(a[0] + rest*fr)
			out[1] = clamp8(a[1] + rest*fg)
			out[2] = clamp8(a[2] + rest*fb)
			out[3] = clamp8(a[3] + rest*fa)
		}
	}
}

// spans returns the taps of n output pixels covering [r0, r1) of the
// source, clipped to [c0, c1): output pixel i is centred on
// r0 + (i + ½)·s, s = (r1 − r0) / n. It reads max(s, 1) source pixels
// around its centre, so that it averages what it covers when s ≥ 1 and
// blends the two nearest pixels linearly when s < 1; a source pixel's
// weight is the length of its overlap with that window. The weights are
// then scaled to sum to the share of the pixel's own footprint
// (s long) inside the clip: the rest is the fill, so that the clip's
// edge stays sharp however large the zoom, and the picture's last pixel
// is held up to it.
func spans(n int, r0, r1, c0, c1 float64) ([]span, []tap) {
	s := (r1 - r0) / float64(n)
	f := max(s, 1)
	out := make([]span, n)
	taps := make([]tap, 0, n*(int(f)+2))
	for i := range n {
		c := r0 + (float64(i)+0.5)*s
		a, z := max(c-f/2, c0), min(c+f/2, c1)
		cover := (min(c+s/2, c1) - max(c-s/2, c0)) / s
		sp := span{start: len(taps)}
		var sum float64
		for at := int(math.Floor(a)); float64(at) < z; at++ {
			lo, hi := max(a, float64(at)), min(z, float64(at+1))
			if hi <= lo {
				continue
			}
			taps = append(taps, tap{at: at, w: float32(hi - lo)})
			sum += hi - lo
			sp.n++
		}
		if cover > 0 && sum > 0 {
			k := min(cover, 1) / sum
			for t := range taps[sp.start:] {
				taps[sp.start+t].w = float32(float64(taps[sp.start+t].w) * k)
			}
			sp.in = float32(min(cover, 1))
		} else {
			taps = taps[:sp.start]
			sp.n = 0
		}
		out[i] = sp
	}

	return out, taps
}

// maxTaps returns the most taps a span has.
func maxTaps(s []span) int {
	n := 0
	for _, sp := range s {
		n = max(n, sp.n)
	}

	return n
}

// newRows returns the rows of src with n rows cached.
func newRows(src image.Image, n int) *rows {
	r := &rows{src: src}
	if rgba, ok := src.(*image.RGBA); ok {
		r.rgba = rgba

		return r
	}
	w := src.Bounds().Dx()
	r.slots = make([]*image.RGBA, n)
	r.keys = make([]int, n)
	for i := range n {
		r.slots[i] = image.NewRGBA(image.Rect(0, 0, w, 1))
		r.keys[i] = math.MinInt
	}

	return r
}

// row returns source row y as premultiplied RGBA bytes, the first pixel
// that of the source's left edge.
func (r *rows) row(y int) []byte {
	b := r.src.Bounds()
	if r.rgba != nil {
		off := r.rgba.PixOffset(b.Min.X, y)

		return r.rgba.Pix[off : off+4*b.Dx()]
	}
	k := (y - b.Min.Y) % len(r.slots)
	if r.keys[k] != y {
		draw.Draw(r.slots[k], r.slots[k].Rect, r.src, image.Pt(b.Min.X, y), draw.Src)
		r.keys[k] = y
	}

	return r.slots[k].Pix
}

// clamp8 rounds v to a byte.
func clamp8(v float32) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 255:
		return 255
	default:
		return uint8(v + 0.5)
	}
}
