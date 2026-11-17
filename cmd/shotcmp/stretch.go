package main

import "math"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Stretch is the rule of 07 section 3.3 for a screen bigger than the
// prototype's (Android 412 x 892 against 393 x 852): it works out, from the
// prototype's boxes alone, the box each element should have there, the way
// CSS lays the same page out.
//
// Widths. An element is placed against its container, the smallest element
// before it in document order that holds it (the screen for the outermost):
// anchored to both sides (each margin within Margin) it grows with the
// container, anchored to the right only it moves with the right edge, to the
// left only it stays, centred (equal margins) it moves by half. Besides, an
// element whose right edge touches (12 or less) a sibling that moves is the
// flexible one between them (`flex:1`: the text of a row, an input) and grows
// by the container's growth; a row of two or more siblings of the same size,
// side by side and spanning the container (the four tabs, the segmented
// buttons, the badges), shares the growth equally.
//
// Heights. An element whose bottom edge is on the screen's (the tab bar, a
// sheet) or, when it is nearly as wide as the screen, one of Bottoms above it (the bottom bar of the detail pages, `padding:20px
// 20px 38px`; the toast, `bottom:100px`) moves by DH; one on both the top and the bottom edge (the dim)
// grows by DH; one centred moves by DH/2. What follows an anchored element in
// document order and lies inside its box is its content and moves with it (the
// rows of a sheet, the icon of the sticky button); the elements behind it (a
// list scrolling under the sticky button) do not.
type Stretch struct {
	DW     float64 // app screen width minus prototype screen width
	ProtoW float64
	Margin float64
	DH     float64 // app screen height minus prototype screen height
	ProtoH float64
	// Bottoms are how far above the bottom edge elements sit that are positioned
	// from it (the sticky button 38, the toast 100, in the prototype's CSS).
	Bottoms []float64
	// Moved are the keys of elements whose own margins do not say how they move
	// (the splash is a flex column centred on the screen, its children's margins
	// are not equal; its bar is `bottom:120px` and 120 wide): the share of DH each
	// one moves by, 0.5 for the column, 1 for the bar.
	Moved map[string]float64
	// BottomExtra is how much taller than the prototype's 34 the system's bottom
	// inset is on the device (3-button navigation 48: 14; gesture 24: -10): the
	// bar on the screen's bottom edge (the tab bar, `padding-bottom:34px`) grows by
	// it and what is in it keeps its distance from the bar's top.
	BottomExtra float64
}

// hClass is how an element sits in its container.
type hClass uint8

const (
	hUnknown hClass = iota // zero value: not classified yet
	hNone                  // fixed in the middle of the container
	hLeft                  // anchored to the left
	hRight                 // anchored to the right
	hBoth                  // anchored to both sides
	hMid                   // centred
	hFlex                  // between a fixed left and a sibling that moves
	hShare                 // one of a row that shares the growth
	hSpread                // one of a row spread between its two ends (`justify-content:space-between`)
)

// nestedMargin is the margin of the stretch rule inside a container: a row's
// right padding 20, plus the 16 wide trailing triangle's share, comes to 28; a current lesson's row, 8 wider each side, 36.
const nestedMargin = 40

// spreadPad is the page's side padding (`padding:0 20px`) in front of a space-around row.
const spreadPad = 20

// narrowShare is the share of its container below which an element with equal
// margins is centred rather than stretched.
const narrowShare = 0.7

// wideShare is the share of its container from which an element anchored to the
// right only is a flexible column and not a fixed box that follows the edge.
const wideShare = 0.5

// minShareWidth is the narrowest an element of a row that shares the growth
// can be: fixed size icons (24 wide) in a row are not flexible.
const minShareWidth = 48

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// plan returns the box the app should have for every element of the prototype,
// by its (name, index) key.
func (s Stretch) plan(elems []Element) map[string]Box {
	out := make(map[string]Box, len(elems))
	for _, e := range elems {
		out[e.match()] = Box{e.X, e.Y, e.W, e.H}
	}
	if s.DW == 0 && s.DH == 0 {
		return out
	}
	moves := s.vertical(elems)
	n := len(elems)
	box := func(i int) Box { return Box{elems[i].X, elems[i].Y, elems[i].W, elems[i].H} }
	screen := Box{0, 0, s.ProtoW, s.ProtoH}

	// the container of every element: the nearest open element that holds it.
	parent := make([]int, n)
	var stack []int
	for i := range elems {
		for len(stack) > 0 && !holds(box(stack[len(stack)-1]), box(i)) {
			stack = stack[:len(stack)-1]
		}
		parent[i] = -1
		if len(stack) > 0 {
			parent[i] = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	pbox := func(i int) Box {
		if parent[i] < 0 {
			return screen
		}
		return box(parent[i])
	}

	// pass 1: how each element sits against its container.
	class := make([]hClass, n)
	for i := range elems {
		class[i] = s.classify(box(i), pbox(i))
	}
	siblings := map[int][]int{}
	for i := range elems {
		siblings[parent[i]] = append(siblings[parent[i]], i)
	}
	shareIdx := make([]int, n)
	shareLen := make([]int, n)
	for par, sib := range siblings {
		pb := screen
		if par >= 0 {
			pb = box(par)
		}
		s.shares(sib, box, pb, class, shareIdx, shareLen)
	}
	// pass 2: in a row of unequal siblings that reaches the container's right
	// side, the widest is the flexible one (`flex:1`): what is before it stays,
	// what is after it moves.
	for par, sib := range siblings {
		pb := screen
		if par >= 0 {
			pb = box(par)
		}
		s.rows(sib, box, pb, class)
	}

	// pass 2a: a row spread between its two ends (the transport row of 06 and
	// 08): equal gaps, wider than a flex row's, the first item at the left
	// margin and the last at the right one; every gap takes the same growth.
	for par, sib := range siblings {
		pb := screen
		if par >= 0 {
			pb = box(par)
		}
		s.spreads(sib, box, pb, class, shareIdx, shareLen)
	}

	// pass 2b: what is left fixed in the middle of the container and, together,
	// centred in it (the six boxes of a code, the icon and text of a button) is
	// centred as a whole.
	for par, sib := range siblings {
		pb := screen
		if par >= 0 {
			pb = box(par)
		}
		// by lines: the elements that overlap the first one of the line vertically
		var used = map[int]bool{}
		for _, first := range sib {
			if class[first] != hNone || used[first] {
				continue
			}
			var group []int
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, i := range sib {
				if class[i] == hNone && !used[i] && box(i).Y < box(first).Y+box(first).H && box(first).Y < box(i).Y+box(i).H {
					group = append(group, i)
					used[i] = true
					lo, hi = math.Min(lo, box(i).X), math.Max(hi, box(i).X+box(i).W)
				}
			}
			if len(group) >= 2 && math.Abs((lo-pb.X)-(pb.X+pb.W-hi)) <= 1 {
				for _, i := range group {
					class[i] = hMid
				}
			}
		}
		// an icon in a button that has a text: the icon and the text are centred together.
		if par >= 0 && elems[par].Text != "" {
			for _, i := range sib {
				if class[i] == hNone && box(i).W <= 32 {
					class[i] = hMid
				}
			}
		}
	}

	// pass 3: the boxes, from the outside in (a container comes before what it holds).
	exp := make([]Box, n)
	for i := range elems {
		b := box(i)
		cdx, cdw := 0.0, s.DW
		pb := screen
		if parent[i] >= 0 {
			pe := exp[parent[i]]
			pb = box(parent[i])
			cdx, cdw = pe.X-pb.X, pe.W-pb.W
		}
		switch class[i] {
		case hBoth, hFlex:
			b.X += cdx
			b.W += cdw
		case hRight:
			b.X += cdx + cdw
		case hLeft, hNone:
			b.X += cdx
		case hMid:
			b.X += cdx + cdw/2
		case hShare:
			b.X += cdx + float64(shareIdx[i])*cdw/float64(shareLen[i])
			b.W += cdw / float64(shareLen[i])
		case hSpread:
			// shareIdx / shareLen: the item's share of the growth, see spreads
			b.X += cdx + float64(shareIdx[i])*cdw/float64(shareLen[i])
		default:
			b.X += cdx
		}
		_ = pb
		if m, ok := moves[elems[i].match()]; ok {
			b.Y += m.dy
			b.H += m.dh
		}
		exp[i] = b
		out[elems[i].match()] = b
	}
	return out
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// vMove is how an element moves and grows when the screen is taller.
type vMove struct{ dy, dh float64 }

// margin is how close to the edge of its container an element is anchored to
// it: Margin for the screen, nestedMargin inside an element (the padding of a
// card or row plus the hidden icon at the end of a row).
func (s Stretch) margin(pb Box) float64 {
	if pb.W == s.ProtoW && pb.H == s.ProtoH && pb.X == 0 && pb.Y == 0 {
		return s.Margin
	}
	return nestedMargin
}

// atBottom tells whether b, the distance of an element from the bottom edge, is one
// an element positioned from the bottom has.
func (s Stretch) atBottom(b float64) bool {
	for _, v := range s.Bottoms {
		if math.Abs(b-v) <= 1 {
			return true
		}
	}
	return false
}

// holds tells whether box a holds box b (half a unit of slack).
func holds(a, b Box) bool {
	const eps = 0.5
	return b.X >= a.X-eps && b.Y >= a.Y-eps && b.X+b.W <= a.X+a.W+eps && b.Y+b.H <= a.Y+a.H+eps
}

// classify places the box b against its container pb.
func (s Stretch) classify(b, pb Box) hClass {
	l, r := b.X-pb.X, (pb.X+pb.W)-(b.X+b.W)
	m := s.margin(pb)
	switch {
	case l < -1 || r < -1:
		// reaching past the container (a strip that scrolls): it does not follow an edge
		return hLeft
	case l <= m && r <= m && math.Abs(l-r) <= 1 && b.W < narrowShare*pb.W:
		// a small box in the middle (the pill of a tab, an icon in its button)
		return hMid
	case l <= m && r <= m:
		return hBoth
	case r <= m && b.W >= wideShare*pb.W:
		// a text column that fills what is left of the row
		return hFlex
	case r <= m:
		return hRight
	case l <= m:
		return hLeft
	case math.Abs(l-r) <= 1:
		return hMid
	}
	return hNone
}

// shares finds the runs of siblings (same parent) that share the growth: two
// or more of the same size on one line, at most 12 apart, from the left margin
// of the container to its right one.
func (s Stretch) shares(sib []int, box func(int) Box, pb Box, class []hClass, idx, n []int) {
	for a := 0; a < len(sib); {
		b := a
		for b+1 < len(sib) {
			p, q := box(sib[b]), box(sib[b+1])
			gap := q.X - (p.X + p.W)
			if math.Abs(p.W-q.W) > 0.5 || math.Abs(p.Y-q.Y) > 0.5 || math.Abs(p.H-q.H) > 0.5 || gap < -0.5 || gap > 12.5 {
				break
			}
			b++
		}
		if b > a {
			first, last := box(sib[a]), box(sib[b])
			if m := s.margin(pb); first.W >= minShareWidth && first.X-pb.X <= m && (pb.X+pb.W)-(last.X+last.W) <= m && (pb.X+pb.W)-(last.X+last.W) >= -1 {
				for k := a; k <= b; k++ {
					class[sib[k]], idx[sib[k]], n[sib[k]] = hShare, k-a, b-a+1
				}
			}
		}
		a = b + 1
	}
}

// spreads finds the runs of three or more siblings on one line whose gaps are
// equal (within 1) and wider than 16, and that are spread over their container
// the way flexbox does: `justify-content:space-between` (the first item at the
// container's left margin, the last at its right one) or `space-around` (those
// margins, past the page's side padding, are half a gap). Sizes stay; every gap
// takes the same growth. idx and n get the numerator and the denominator of the
// item's share of the container's growth: k/(items-1) between,
// (2k+1)/(2*items) around.
func (s Stretch) spreads(sib []int, box func(int) Box, pb Box, class []hClass, idx, n []int) {
	m := s.margin(pb)
	for a := 0; a < len(sib); a++ {
		b := a
		var gap float64
		for b+1 < len(sib) {
			p, q := box(sib[b]), box(sib[b+1])
			g := q.X - (p.X + p.W)
			if g <= 16.5 || q.Y >= p.Y+p.H || p.Y >= q.Y+q.H || (b > a && math.Abs(g-gap) > 1) {
				break
			}
			if b == a {
				gap = g
			}
			b++
		}
		if b-a < 2 {
			continue
		}
		first, last := box(sib[a]), box(sib[b])
		l, r := first.X-pb.X, (pb.X+pb.W)-(last.X+last.W)
		items := b - a + 1
		switch {
		case l <= m && r <= m && r >= -1:
			for k := a; k <= b; k++ {
				class[sib[k]], idx[sib[k]], n[sib[k]] = hSpread, k-a, items-1
			}
		case math.Abs(l-spreadPad-gap/2) <= 1.5 && math.Abs(r-spreadPad-gap/2) <= 1.5:
			for k := a; k <= b; k++ {
				class[sib[k]], idx[sib[k]], n[sib[k]] = hSpread, 2*(k-a)+1, 2*items
			}
		default:
			continue
		}
		a = b
	}
}

// rows finds the runs of siblings (same parent) on one line, at most 16 apart,
// that are not all of one size, and whose end is within the margin of the
// container's right side: the widest of a run grows, those before it keep
// their place, those after it move with the right edge.
func (s Stretch) rows(sib []int, box func(int) Box, pb Box, class []hClass) {
	m := s.margin(pb)
	for a := 0; a < len(sib); {
		b := a
		for b+1 < len(sib) {
			p, q := box(sib[b]), box(sib[b+1])
			gap := q.X - (p.X + p.W)
			if gap < -0.5 || gap > 16.5 || q.Y >= p.Y+p.H || p.Y >= q.Y+q.H {
				break
			}
			b++
		}
		if b > a {
			wide, equal := a, true
			for k := a; k <= b; k++ {
				if math.Abs(box(sib[k]).W-box(sib[a]).W) > 0.5 {
					equal = false
				}
				if box(sib[k]).W > box(sib[wide]).W {
					wide = k
				}
			}
			last := box(sib[b])
			if !equal && class[sib[wide]] != hShare && ((pb.X+pb.W)-(last.X+last.W) <= m || box(sib[wide]).W >= 0.6*pb.W) && (pb.X+pb.W)-(last.X+last.W) >= -1 && box(sib[wide]).W >= minShareWidth {
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

// vertical works out how the elements move when the screen is taller, by the
// rule in Stretch's comment; the elements that stay are not in the map.
func (s Stretch) vertical(elems []Element) map[string]vMove {
	moves := map[string]vMove{}
	if s.DH == 0 {
		return moves
	}
	for i, e := range elems {
		p := Box{e.X, e.Y, e.W, e.H}
		t, b := p.Y, s.ProtoH-(p.Y+p.H)
		var m vMove
		container := true
		switch {
		case t <= 1 && b <= 1 && b >= -1:
			m, container = vMove{dh: s.DH}, false
		case math.Abs(b) <= 1 && s.BottomExtra != 0:
			m = vMove{dy: s.DH - s.BottomExtra, dh: s.BottomExtra}
		case math.Abs(b) <= 1 || (p.W >= 0.8*s.ProtoW && s.atBottom(b)):
			m = vMove{dy: s.DH}
		case s.Moved[e.match()] != 0:
			m = vMove{dy: s.DH * s.Moved[e.match()]}
		case math.Abs(t-b) <= 1:
			m = vMove{dy: s.DH / 2}
		default:
			continue
		}
		moves[e.match()] = m
		if !container {
			continue
		}
		for _, c := range elems[i+1:] {
			if !holds(p, Box{c.X, c.Y, c.W, c.H}) {
				break
			}
			if _, own := moves[c.match()]; !own {
				moves[c.match()] = vMove{dy: m.dy}
				if m.dh == 0 || s.BottomExtra == 0 {
					moves[c.match()] = m
				}
			}
		}
	}
	return moves
}
