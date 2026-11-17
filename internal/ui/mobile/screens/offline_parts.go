//go:build !tvos

// The column of screen 13 is written from MS.offline (index.html:461) and its M13
// capture (local/shots/prototype-kit/mobile/M13.layout.json).

package screens

import (
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.offline's column (index.html:461), in DIPs.
const (
	// offBackTop is `<div style="margin-top:10px">` around the back button; the
	// button is 40 (`.rb`).
	offBackTop, offBackSize float32 = 10, 40
	// offNameTop, offNameBottom, offNameSize are the course name's `.d` at
	// `font-size:30px;margin:16px 0 6px`.
	offNameTop, offNameBottom, offNameSize float32 = 16, 6, 30
	// offHintSize, offHintBottom are the hint's `font-size:13px;margin-bottom:6px`.
	offHintSize, offHintBottom float32 = 13, 6
	// offEmptyTop, offEmptySize are the line of a course with no lesson to show
	// (not in the prototype, whose course always lists its lessons).
	offEmptyTop, offEmptySize float32 = 24, 14
)

// offBody is what the offline screen scrolls: the back button, the course's name,
// the hint and the lessons, each as MS.offline's blocks are.
type offBody struct {
	ui.Base

	back  *kit.RoundButton
	name  *kit.Label
	hint  *kit.Label
	list  *kit.LessonList
	empty *kit.Label // shown when there is no lesson to list
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newOffBody returns the column with no name and no lesson yet.
func newOffBody(e *env.Env) *offBody {
	b := &offBody{
		back:  newLightBack(e), // `backBtn()`: the --sf disc
		name:  heading(kit.Display("", offNameSize), 1),
		hint:  kit.Muted(e.T("offline.hint"), offHintSize),
		list:  &kit.LessonList{},
		empty: kit.Muted(e.T("downloads.emptyTitle"), offEmptySize),
	}
	b.name.Wrap = true
	b.hint.Name, b.hint.Wrap = "p.muted", true
	b.empty.Hidden = true
	b.Add(b.back, b.name, b.hint, b.list, b.empty)

	return b
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// setName shows the course's name; an empty one leaves no line for it.
func (b *offBody) setName(s string) {
	if s != b.name.Text {
		b.name.SetText(s)
	}
	kit.SetHidden(b.name, s == "")
}

// setEmpty shows or hides the line of a course with no lesson.
func (b *offBody) setEmpty(on bool) {
	kit.SetHidden(b.empty, !on)
	b.Relayout()
}

// Layout stacks the parts as MS.offline's block flow does: the back button 10
// from the top, the name 16 under it, the hint 6 under the name, the rows 6 under
// the hint.
func (b *offBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	col := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	y := offBackTop
	c.Layout(b.back, ui.Tight(geom.Sz(offBackSize, offBackSize)))
	c.Place(b.back, geom.Pt(0, y))
	y += offBackSize
	y += offNameTop
	if !b.name.Hidden {
		ns := c.Layout(b.name, col)
		c.Place(b.name, geom.Pt(0, y))
		y += ns.H + offNameBottom
	}
	hs := c.Layout(b.hint, col)
	c.Place(b.hint, geom.Pt(0, y))
	y += hs.H + offHintBottom
	ls := c.Layout(b.list, col)
	c.Place(b.list, geom.Pt(0, y))
	y += ls.H
	if !b.empty.Hidden {
		es := c.Layout(b.empty, col)
		c.Place(b.empty, geom.Pt(0, y+offEmptyTop))
		y += offEmptyTop + es.H
	}

	return cs.Constrain(geom.Sz(w, y))
}
