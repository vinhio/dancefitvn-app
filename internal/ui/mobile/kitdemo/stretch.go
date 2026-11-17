//go:build !tvos

package kitdemo

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// This file is the horizontal half of cmd/shotcmp's stretch rule
// (cmd/shotcmp/stretch.go, 07 section 3.3), kept in step with it by hand: the
// boxes of a page are written for the prototype's 393 wide phone, and on a wider
// one every box goes where CSS would put it. An element is placed against its
// container, the nearest earlier one that holds it (the screen for the
// outermost): anchored to both sides (each margin within 24 of the screen's, 32
// of a container's) it grows with the container; to the right only it moves with
// the right edge; to the left only it stays; centred (equal margins) it moves by
// half, as does a box below 70 % of its container with equal margins. In a row
// of unequal siblings on one line (16 apart at most) that reaches the container's
// right side, the widest is flexible (it grows), what is before it stays and what
// is after it moves; a row of two or more siblings of one size, 48 wide at
// least, from one side of the container to the other, shares the growth equally.

// hClass is how an element sits in its container.
type hClass uint8

const (
	hUnknown hClass = iota // zero value: not classified yet
	hNone                  // fixed in the middle of the container
	hLeft                  // anchored to the left
	hRight                 // anchored to the right
	hBoth                  // anchored to both sides
	hMid                   // centred
	hFlex                  // the flexible one of a row
	hShare                 // one of a row that shares the growth
)

const (
	// nestedMargin is the margin of the rule inside a container.
	nestedMargin float32 = 40
	// narrowShare is the share of its container below which an element with
	// equal margins is centred rather than stretched.
	narrowShare float32 = 0.7
	// wideShare is the share of its container from which an element anchored to
	// the right only is a flexible column and not a fixed box.
	wideShare float32 = 0.5
	// minShareWidth is the narrowest an element of an equal row can be.
	minShareWidth float32 = 48
	// rowGap is the widest gap between the elements of a row.
	rowGap float32 = 16
)

// span is the horizontal extent and the line of a box.
type span struct{ x, w, y, h float32 }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// planX returns, for each box (in document order), its x and width on a screen
// dw wider than the design width; labelled tells which boxes hold a text of
// their own (a button) and mid which are centred whatever else.
func planX(boxes []span, labelled, mid []bool, dw, designW float32) (xs, ws []float32) {
	n := len(boxes)
	xs, ws = make([]float32, n), make([]float32, n)
	for i, b := range boxes {
		xs[i], ws[i] = b.x, b.w
	}
	if dw == 0 || n == 0 {
		return xs, ws
	}
	// the container of every box.
	parent := make([]int, n)
	var stack []int
	for i := range boxes {
		for len(stack) > 0 && !holds(boxes[stack[len(stack)-1]], boxes[i]) {
			stack = stack[:len(stack)-1]
		}
		parent[i] = -1
		if len(stack) > 0 {
			parent[i] = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	screen := span{0, designW, 0, 0}
	pbox := func(i int) span {
		if parent[i] < 0 {
			return screen
		}
		return boxes[parent[i]]
	}
	margin := func(i int) float32 {
		if parent[i] < 0 {
			return stretchMargin
		}
		return nestedMargin
	}

	class := make([]hClass, n)
	for i, b := range boxes {
		l, r := b.x-pbox(i).x, pbox(i).x+pbox(i).w-(b.x+b.w)
		m := margin(i)
		switch {
		case l < -1 || r < -1:
			class[i] = hLeft
		case l <= m && r <= m && abs32(l-r) <= 1 && b.w < narrowShare*pbox(i).w:
			class[i] = hMid
		case l <= m && r <= m:
			class[i] = hBoth
		case r <= m && b.w >= wideShare*pbox(i).w:
			class[i] = hFlex
		case r <= m:
			class[i] = hRight
		case l <= m:
			class[i] = hLeft
		case abs32(l-r) <= 1:
			class[i] = hMid
		default:
			class[i] = hNone
		}
	}
	siblings := map[int][]int{}
	for i := range boxes {
		siblings[parent[i]] = append(siblings[parent[i]], i)
	}
	shareIdx, shareLen := make([]int, n), make([]int, n)
	for par, sib := range siblings {
		pb := screen
		m := stretchMargin
		if par >= 0 {
			pb, m = boxes[par], nestedMargin
		}
		for a := 0; a < len(sib); {
			b := a
			for b+1 < len(sib) {
				p, q := boxes[sib[b]], boxes[sib[b+1]]
				gap := q.x - (p.x + p.w)
				if abs32(p.w-q.w) > 0.5 || abs32(p.y-q.y) > 0.5 || abs32(p.h-q.h) > 0.5 || gap < -0.5 || gap > 12.5 {
					break
				}
				b++
			}
			if b > a {
				first, last := boxes[sib[a]], boxes[sib[b]]
				rr := pb.x + pb.w - (last.x + last.w)
				if first.w >= minShareWidth && first.x-pb.x <= m && rr <= m && rr >= -1 {
					for k := a; k <= b; k++ {
						class[sib[k]], shareIdx[sib[k]], shareLen[sib[k]] = hShare, k-a, b-a+1
					}
				}
			}
			a = b + 1
		}
	}
	for par, sib := range siblings {
		pb := screen
		m := stretchMargin
		if par >= 0 {
			pb, m = boxes[par], nestedMargin
		}
		for a := 0; a < len(sib); {
			b := a
			for b+1 < len(sib) {
				p, q := boxes[sib[b]], boxes[sib[b+1]]
				gap := q.x - (p.x + p.w)
				if gap < -0.5 || gap > rowGap+0.5 || q.y >= p.y+max(p.h, 1) || p.y >= q.y+max(q.h, 1) {
					break
				}
				b++
			}
			if b > a {
				wide, equal := a, true
				for k := a; k <= b; k++ {
					if abs32(boxes[sib[k]].w-boxes[sib[a]].w) > 0.5 {
						equal = false
					}
					if boxes[sib[k]].w > boxes[sib[wide]].w {
						wide = k
					}
				}
				last := boxes[sib[b]]
				rr := pb.x + pb.w - (last.x + last.w)
				if !equal && class[sib[wide]] != hShare && (rr <= m || boxes[sib[wide]].w >= 0.6*pb.w) && rr >= -1 && boxes[sib[wide]].w >= minShareWidth {
					for k := a; k <= b; k++ {
						switch {
						case k < wide:
							class[sib[k]] = hLeft
						case k == wide:
							class[sib[k]] = hFlex
						default:
							class[sib[k]] = hRight
						}
					}
				}
			}
			a = b + 1
		}
	}

	// what is left fixed in the middle of the container and, together, centred in
	// it is centred as a whole; so is an icon in a button with a text.
	for par, sib := range siblings {
		pb := screen
		if par >= 0 {
			pb = boxes[par]
		}
		used := map[int]bool{}
		for _, first := range sib {
			if class[first] != hNone || used[first] {
				continue
			}
			var group []int
			lo, hi := float32(math.Inf(1)), float32(math.Inf(-1))
			f := boxes[first]
			for _, i := range sib {
				b := boxes[i]
				if class[i] == hNone && !used[i] && b.y < f.y+max(f.h, 1) && f.y < b.y+max(b.h, 1) {
					group = append(group, i)
					used[i] = true
					lo, hi = min(lo, b.x), max(hi, b.x+b.w)
				}
			}
			if len(group) >= 2 && abs32((lo-pb.x)-(pb.x+pb.w-hi)) <= 1 {
				for _, i := range group {
					class[i] = hMid
				}
			}
		}
		if par >= 0 && labelled[par] {
			for _, i := range sib {
				if class[i] == hNone && boxes[i].w <= 32 {
					class[i] = hMid
				}
			}
		}
	}

	for i := range boxes {
		if mid[i] {
			class[i] = hMid
		}
	}

	for i, b := range boxes {
		cdx, cdw := float32(0), dw
		if parent[i] >= 0 {
			cdx, cdw = xs[parent[i]]-boxes[parent[i]].x, ws[parent[i]]-boxes[parent[i]].w
		}
		switch class[i] {
		case hBoth, hFlex:
			xs[i], ws[i] = b.x+cdx, b.w+cdw
		case hRight:
			xs[i] = b.x + cdx + cdw
		case hLeft, hNone:
			xs[i] = b.x + cdx
		case hMid:
			xs[i] = b.x + cdx + cdw/2
		case hUnknown:
			xs[i] = b.x + cdx
		case hShare:
			k := float32(shareLen[i])
			xs[i], ws[i] = b.x+cdx+float32(shareIdx[i])*cdw/k, b.w+cdw/k
		default:
			xs[i] = b.x + cdx
		}
	}

	return xs, ws
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// holds tells whether box a holds box b (half a unit of slack).
func holds(a, b span) bool {
	const eps = 0.5
	return b.x >= a.x-eps && b.y >= a.y-eps && b.x+b.w <= a.x+a.w+eps && b.y+b.h <= a.y+a.h+eps
}
