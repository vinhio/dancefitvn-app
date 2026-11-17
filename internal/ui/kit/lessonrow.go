// Built on rowText of row.go, which is adapted from
// jplayer-mobile/internal/ui/kit/row.go at 49de3c2 (a pressable row, its Describe
// and AccessibleLabel); its swipe, long press and reorder are not in the
// prototype. The rest is written from lesRow (index.html:382-393).

package kit

import (
	"math"
	"strconv"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// LessonState is how a lesson row stands: the classes `cur` and `w` of lesRow
// (index.html:382-393).
type LessonState uint8

// The states. LessonStateUnknown reads as LessonNormal.
const (
	LessonStateUnknown LessonState = iota
	// LessonNormal is a plain `.les`.
	LessonNormal
	// LessonCurrent is `.les.cur`: the lesson being watched, on a yellow tint.
	LessonCurrent
	// LessonWatched is `.les.w`: a lesson already watched, its number --tx3.
	LessonWatched
)

// LessonMode is where a lesson row is shown: the `mode` of lesRow.
type LessonMode uint8

// The modes. LessonModeUnknown reads as LessonOnline.
const (
	LessonModeUnknown LessonMode = iota
	// LessonOnline ends the row with the download button and the triangle.
	LessonOnline
	// LessonOffline (`mode==='off'`) ends it with the play disc of a downloaded
	// lesson or the cloud of one that needs the network.
	LessonOffline
	// LessonModeLocked is the row of the locked course (MS.locked,
	// index.html:410): the number and the text, and Trailing at the end (the
	// `chip.y` of the preview, the 18 px lock) in place of the download button
	// and the triangle.
	LessonModeLocked
)

// LessonDownload is the download of a lesson: `S.dl[k]` of lesRow.
type LessonDownload uint8

// The downloads. LessonDownloadUnknown reads as DlNone.
const (
	LessonDownloadUnknown LessonDownload = iota
	// DlNone is no file: the download icon in --tx3.
	DlNone
	// DlIng is a download under way: the progress ring and the cancel cross,
	// in --pr.
	DlIng
	// DlDone is a downloaded file: the download icon in --ok.
	DlDone
	// DlUnavailable is a lesson with no audio file to download (the course data
	// say has_audio false, or the server answered not_found for it this session):
	// the button is gone and its room stays, so the rows keep their boxes. The
	// prototype has no such lesson.
	DlUnavailable
)

// The numbers of .les (index.html:91-103, :388-391).
const (
	// lesMinHeight is `.les{min-height:58px}`, border included (border-box).
	lesMinHeight float32 = 58
	// lesGap is `.les{gap:12px}`.
	lesGap float32 = 12
	// lesBorder is `.les{border-bottom:1px solid var(--bd)}`; `.les.cur` makes it
	// transparent but it stays (:93).
	lesBorder float32 = 1
	// lesNo, lesNoBorder, lesNoSize are `.les .no{width:36px;height:36px;
	// border:1.5px solid var(--bd);font-size:13px;font-weight:800}`.
	lesNo, lesNoBorder, lesNoSize float32 = 36, 1.5, 13
	// lesCurPad, lesCurRadius are `.les.cur{margin:0 -8px;padding:0 8px;
	// border-radius:12px}`.
	lesCurPad, lesCurRadius float32 = 8, 12
	// lesTitleSize, lesMetaSize are `.les .tt b{font-size:15px;font-weight:600}`
	// and `.les .tt span{font-size:12px}`.
	lesTitleSize, lesMetaSize float32 = 15, 12
	// lesDl is `.les .dlb{width:34px;height:34px}`.
	lesDl float32 = 34
	// lesDlIcon, lesDlStroke, lesDlStrokeOn are `ic('dl',20,d?2.2:1.8)` (:389).
	lesDlIcon, lesDlStroke, lesDlStrokeOn float32 = 20, 1.8, 2.2
	// lesRingInset, lesRingBox are `.les .dlb .pr{position:absolute;inset:3px}` and
	// its `viewBox="0 0 36 36"` with a circle of r 15 and a stroke of 3 (:388).
	lesRingInset, lesRingBox, lesRingR, lesRingStroke float32 = 3, 36, 15, 3
	// lesRingLen is the `stroke-dasharray="${94.2*p} 94.2"` length of the ring.
	lesRingLen float32 = 94.2
	// lesCross, lesCrossStroke are `ic('x',12,3)` over the ring (:388).
	lesCross, lesCrossStroke float32 = 12, 3
	// lesTri is `<svg viewBox="0 0 24 24" style="width:16px">` (:391).
	lesTri float32 = 16
	// lesPlay is the offline play disc `width:32px;height:32px` with `padding:8px
	// 7px 8px 10px` around the PLAY icon (:391).
	lesPlay, lesPlayPadTop, lesPlayPadLeft, lesPlayPadRight float32 = 32, 8, 10, 7
	// lesCloud is `ic('cloud')`, 22 (:391).
	lesCloud float32 = 22
	// lesDim is `.les.dim{opacity:.38}`.
	lesDim float32 = .38
)

// The names layout.json gives the parts of a lesson row.
const (
	nameLesNo = "span.no"
	nameLesTT = "span.tt"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// LessonRow is `.les` (index.html:91-103, lesRow :382-393): the row of a lesson,
// at least 58 tall with a 1 px --bd line under it, 12 between its parts: the 36
// round number (`.no`), the text (title 15/600 on one line with an ellipsis,
// meta 12 in --tx2) taking the room, and, online, the 34 download button and a 16
// triangle; offline, the 32 play disc of a downloaded lesson or the cloud.
//
// The current lesson (`.cur`) is tinted rgba(255,208,40,.12), rounded 12, padded 8
// on each side with the number filled --pr and the meta in --pr; it is meant to
// sit 8 outside its column (`margin:0 -8px`: Bleed, LessonList). A watched lesson
// has its number in --tx3. Dim draws the row at .38 opacity.
//
// Set the Name to the data key (`playVideo:set35-3`, `offTap:set35-3`), DlName to
// the download button's (`dlTap:set35-3`) and PlayName to the disc's
// (`playAudio:set35-3`). OnPress is the row's tap, OnDl the download button's, OnPlay
// the disc's.
type LessonRow struct {
	ui.Pressable
	// No is the lesson number, shown with two digits.
	No int
	// Title is the song, Meta the line under it, built by the caller.
	Title, Meta string
	// State is current, watched or plain.
	State LessonState
	// Mode is online or offline.
	Mode LessonMode
	// Dim draws the row at .38.
	Dim bool
	// Download is the state of the lesson's file; Progress, 0 to 1, how far a
	// download under way is.
	Download LessonDownload
	Progress float32
	// OnDl is called when the download button is pressed; OnPlay when the offline
	// play disc is.
	OnDl, OnPlay func()
	// DlName and PlayName are the keys of layout.json for those two buttons.
	DlName, PlayName string
	// DlLabel and PlayLabel name them for assistive technologies; CancelLabel and
	// DoneLabel are the download button's while its file is coming (it cancels)
	// and once it is on the device.
	DlLabel, PlayLabel, CancelLabel, DoneLabel string
	// Lesson names the row's lesson for the two buttons ("Bài 03 · Stink and
	// Dutty"): their value, so that "Tải xuống" says which lesson it downloads.
	Lesson string
	// Trailing ends the row in LessonModeLocked; set it before the first layout.
	Trailing ui.Widget

	no    lessonNo
	text  rowText
	dl    lessonDl
	play  lessonPlay
	tri   Icon
	cloud Icon

	noText   string // No as two digits
	noFor    int
	attached bool
	// dlFor is the Download the button last laid out for: the button is kept laid
	// out when its constraints do not change, so a new state must unmark it
	dlFor LessonDownload
	// dimmed, c and p paint the row under its layer: made once.
	dimmed func()
	c      *ui.Context
	p      *ui.Painter
}

// lessonNo is `.no`: the number in a circle.
type lessonNo struct {
	ui.Base

	label Label
	row   *LessonRow
}

// lessonDl is `.dlb`: the download button, a 34 DIP circle.
type lessonDl struct {
	ui.Pressable

	row   *LessonRow
	icon  Icon
	ring  scene.Path // the arc of the progress, in the button's coordinates
	ringP float32    // the progress the arc was built for
	// trackPath is the ring's whole circle.
	trackPath scene.Path
}

// lessonPlay is the offline play disc.
type lessonPlay struct {
	ui.Pressable

	row *LessonRow
}

// LessonList is a column of LessonRows that gives the current one its
// `margin:0 -8px`: a row with Bleed is as wide as the column plus 16 and starts 8
// left of it. Add the rows in order.
type LessonList struct {
	ui.Base
}

var (
	_ ui.Describer        = (*LessonRow)(nil)
	_ ui.ChildDescriber   = (*LessonRow)(nil)
	_ ui.SubtreeDescriber = (*LessonRow)(nil)
	_ ui.ChildActor       = (*LessonRow)(nil)
	_ ui.Labeler          = (*LessonRow)(nil)
	_ ui.Labeler          = (*lessonDl)(nil)
	_ ui.Labeler          = (*lessonPlay)(nil)
	_ ui.Describer        = (*lessonDl)(nil)
	_ ui.Describer        = (*lessonPlay)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Bleed returns how far the row reaches out of its column on each side: 8 for
// the current lesson, 0 for the others.
func (r *LessonRow) Bleed() float32 {
	if r.State == LessonCurrent {
		return lesCurPad
	}

	return 0
}

// SetProgress moves the download ring to p of the circle, clamped to 0 and 1; the
// same value changes nothing. It does not allocate once the ring has been drawn.
func (r *LessonRow) SetProgress(p float32) {
	p = min(max(p, 0), 1)
	if p != r.Progress {
		r.Progress = p
		r.dl.Repaint()
	}
}

// AccessibleLabel returns the title.
func (r *LessonRow) AccessibleLabel() string { return r.Title }

// Describe describes the row as a button named by its title, valued by its meta;
// a row with a button of its own (the download button, the play disc) is a list
// item, whose buttons are elements under it (DescribeChildren).
func (r *LessonRow) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Pressable.Describe(c, n)
	n.Value = r.Meta
	if !r.dl.Hidden || !r.play.Hidden {
		n.Role = platform.RoleListItem
	}
}

// DescribeChildren adds the download button and the play disc, when shown, as
// elements under the row (a button holds no elements, a list item does).
func (r *LessonRow) DescribeChildren(c *ui.Context, add func(platform.SemanticNode)) {
	DescribeButtons(c, r, add, &r.dl, &r.play)
}

// DescribesSubtree tells that the buttons are all the row has to add.
func (r *LessonRow) DescribesSubtree() bool { return true }

// PerformChildAction presses the button of key.
func (r *LessonRow) PerformChildAction(_ *ui.Context, key uint64, a platform.SemanticAction) bool {
	return PressButton(a, key, &r.dl, &r.play)
}

// AccessibleLabel returns the download button's label: it cancels while the file
// is coming, and says that the file is there once it is.
func (d *lessonDl) AccessibleLabel() string {
	switch {
	case d.row.Download == DlIng && d.row.CancelLabel != "":
		return d.row.CancelLabel
	case d.row.Download == DlDone && d.row.DoneLabel != "":
		return d.row.DoneLabel
	default:
		return d.row.DlLabel
	}
}

// Describe describes the button as a button valued by the lesson it is for.
func (d *lessonDl) Describe(c *ui.Context, n *platform.SemanticNode) {
	d.Pressable.Describe(c, n)
	n.Value = d.row.Lesson
}

// AccessibleLabel returns the play disc's label.
func (p *lessonPlay) AccessibleLabel() string { return p.row.PlayLabel }

// Describe describes the disc as a button valued by the lesson it plays.
func (p *lessonPlay) Describe(c *ui.Context, n *platform.SemanticNode) {
	p.Pressable.Describe(c, n)
	n.Value = p.row.Lesson
}

// Layout fills the width it gets and is at least 58 tall; the number, the text and
// the trailing parts are centered in the height above the border.
func (r *LessonRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	r.init()
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	r.sync()
	if r.dlFor != r.Download {
		// the download button's icon (glyph, size, ink) is made in its own Layout, which
		// the cache of Context.Layout skips for the same constraints: a file that
		// finished while the row was laid out showed the old icon next to the new count
		r.dlFor = r.Download
		r.dl.Relayout()
	}
	pad := r.Bleed()
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))

	// the trailing parts, in order from the left
	var trail []trailPart
	var tbuf [3]trailPart
	trail = tbuf[:0]
	trailW, trailH := float32(0), float32(0)
	add := func(w ui.Widget, h float32) {
		s := c.Layout(w, loose)
		if h < 0 {
			h = s.H
		}
		trail = append(trail, trailPart{w: w, s: s, h: h})
		if len(trail) > 1 {
			trailW += lesGap
		}
		trailW += s.W
		trailH = max(trailH, h)
	}
	switch {
	case r.Mode == LessonModeLocked:
		if r.Trailing != nil {
			add(r.Trailing, -1) // the box is the part's own height
		}
	case r.Mode == LessonOffline:
		if r.Download == DlDone {
			add(&r.play, lesPlay)
		} else {
			add(&r.cloud, lesCloud+inlineStrut)
		}
	default:
		if r.Download == DlUnavailable {
			// no download button, and its room stays: the triangle does not move
			c.Layout(&r.dl, ui.Tight(geom.Size{}))
			trail = append(trail, trailPart{w: &r.dl, s: geom.Sz(lesDl, lesDl), h: lesDl})
			trailW += lesDl
			trailH = max(trailH, lesDl)
		} else {
			add(&r.dl, lesDl)
		}
		add(&r.tri, lesTri+inlineStrut)
	}

	inner := w - 2*pad
	avail := max(inner-lesNo-lesGap-trailW-lesGap, 0)
	r.text.strut = strut(c)
	r.text.hasSub = true
	r.text.title.SetText(r.Title)
	r.text.sub.SetText(r.Meta)
	tt := c.Layout(&r.text, ui.Constraints{Max: geom.Sz(avail, ui.Unbounded)})

	content := max(lesNo, tt.H, trailH)
	h := max(lesMinHeight, content+lesBorder)
	ch := h - lesBorder
	size := cs.Constrain(geom.Sz(w, h))
	c.Layout(&r.no, ui.Tight(geom.Sz(lesNo, lesNo)))
	c.Place(&r.no, geom.Pt(pad, centeredY(ch, lesNo)))
	c.Place(&r.text, geom.Pt(pad+lesNo+lesGap, centeredY(ch, tt.H)))
	x := size.W - pad - trailW
	for i := range trail {
		t := trail[i]
		c.Place(t.w, geom.Pt(x, centeredY(ch, t.h)))
		x += t.s.W + lesGap
	}

	return size
}

// trailPart is a part at the end of a row, with the height of its box (an inline
// svg's includes the strut).
type trailPart struct {
	w ui.Widget
	s geom.Size
	h float32
}

// Paint draws the row: its tint and the line under it, under a layer when it is
// dim.
func (r *LessonRow) Paint(c *ui.Context, p *ui.Painter) {
	if !r.Dim {
		r.paint(c, p)

		return
	}
	if r.dimmed == nil {
		r.dimmed = func() { r.paint(r.c, r.p) }
	}
	r.c, r.p = c, p
	p.Layer(lesDim, 0, r.dimmed)
	r.c, r.p = nil, nil
}

// paint draws the tint of the current lesson or the line under the others, the
// focus ring and the parts.
func (r *LessonRow) paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := SnapBox(p, r.Bounds()) // `.les`: whole-CSS-px box (Chrome)
	if r.State == LessonCurrent {
		p.Fill(b, scene.Solid(look.C.LessonCur), lesCurRadius)
	} else {
		p.Fill(geom.XYWH(0, b.H-lesBorder, b.W, lesBorder), scene.Solid(look.T.Border), 0)
	}
	if r.FocusVisible() {
		focusRing(p, b, 0, look.T.Primary)
	}
	r.Base.Paint(c, p)
}

// Layout centers the number in the circle.
func (n *lessonNo) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(lesNo, lesNo))
	ls := c.Layout(&n.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&n.label, geom.Pt(centeredY(size.W, ls.W), centeredY(size.H, ls.H)))

	return size
}

// Paint draws the circle: filled --pr on the current lesson, with a 1.5 border in
// --bd (--pr on the current one).
func (n *lessonNo) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	r := n.Bounds()
	col := look.T.Border
	if n.row.State == LessonCurrent {
		col = look.T.Primary
		p.Fill(r, scene.Solid(col), r.W/2)
	}
	p.Border(r, geom.Uniform(lesNoBorder), col, r.W/2)
	n.Base.Paint(c, p)
}

// Layout makes the button its 34 DIP circle with the icon in the middle.
func (d *lessonDl) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(lesDl, lesDl))
	d.sync()
	is := c.Layout(&d.icon, ui.Tight(geom.Sz(d.icon.Size, d.icon.Size)))
	c.Place(&d.icon, geom.Pt(centeredY(size.W, is.W), centeredY(size.H, is.H)))

	return size
}

// Paint draws the progress ring of a download under way, then the icon.
func (d *lessonDl) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	if d.row.Download == DlIng {
		b := d.Bounds()
		k := (b.W - 2*lesRingInset) / lesRingBox
		cx, cy := b.W/2, b.H/2
		stroke := scene.Stroke{Width: lesRingStroke * k}
		p.StrokePath(d.track(cx, cy, lesRingR*k), stroke, scene.Solid(look.T.Surface2))
		if d.row.Progress > 0 {
			d.arc(cx, cy, lesRingR*k, d.row.Progress)
			p.StrokePath(&d.ring, stroke, scene.Solid(look.T.Primary))
		}
	}
	if d.FocusVisible() {
		focusRing(p, d.Bounds(), d.Bounds().W/2, look.T.Primary)
	}
	d.Base.Paint(c, p)
}

// Layout makes the disc 32 DIPs.
func (l *lessonPlay) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(geom.Sz(lesPlay, lesPlay))
}

// Paint draws the --pr disc and the play triangle in --on, in the 15 x 16 box the
// padding leaves.
func (l *lessonPlay) Paint(c *ui.Context, p *ui.Painter) {
	look := theme.From(c)
	b := l.Bounds()
	p.Fill(b, scene.Solid(look.T.Primary), b.W/2)
	box := geom.XYWH(lesPlayPadLeft, lesPlayPadTop, b.W-lesPlayPadLeft-lesPlayPadRight, b.H-2*lesPlayPadTop)
	side := min(box.W, box.H)                                                                                                      // the svg is 100 % of the box, meeting: a square
	icon.Draw(p, icon.Play, snapOrigin(p, geom.XYWH(box.X+(box.W-side)/2, box.Y+(box.H-side)/2, side, side)), look.T.OnPrimary, 0) // `svg.i`: whole-CSS-px origin (Chrome)
	if l.FocusVisible() {
		focusRing(p, b, b.W/2, look.T.Primary)
	}
}

// Layout stacks the rows, each as wide as the column, a current one 8 wider on
// each side.
func (l *LessonList) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	y := float32(0)
	for ch := range l.Children() {
		bleed := float32(0)
		if b, ok := ch.(interface{ Bleed() float32 }); ok {
			bleed = b.Bleed()
		}
		s := c.Layout(ch, ui.Constraints{Min: geom.Sz(w+2*bleed, 0), Max: geom.Sz(w+2*bleed, ui.Unbounded)})
		c.Place(ch, geom.Pt(-bleed, y))
		y += s.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// init adds the parts the first time and gives them the names of layout.json.
func (r *LessonRow) init() {
	if r.attached {
		return
	}
	r.attached = true
	r.Look = noLook
	r.no.row, r.dl.row, r.play.row = r, r, r
	r.no.Name, r.text.Name, r.text.title.Name = nameLesNo, nameLesTT, nameRowTitle
	// The trailing triangle is a plain `svg` (no class) in the prototype, which the
	// capture does not record: it stays unnamed (the cloud is `ic('cloud')`, an `svg.i`).
	r.dl.icon.Name, r.cloud.Name = iconName, iconName
	r.dl.Look, r.play.Look = noLook, noLook
	r.dl.OnPress, r.play.OnPress = r.pressDl, r.pressPlay
	r.dl.Add(&r.dl.icon)
	r.no.Add(&r.no.label)
	r.text.Add(&r.text.title, &r.text.sub)
	r.Add(&r.no, &r.text, &r.dl, &r.play, &r.tri, &r.cloud)
	if r.Trailing != nil {
		r.Add(r.Trailing)
	}
}

// sync makes the parts match the fields: the number, the colors, which trailing
// parts show.
func (r *LessonRow) sync() {
	if r.noFor != r.No || r.noText == "" {
		r.noFor = r.No
		r.noText = twoDigits(r.No)
	}
	r.dl.Name, r.play.Name = r.DlName, r.PlayName
	cur, watched := r.State == LessonCurrent, r.State == LessonWatched
	noInk := theme.InkText
	switch {
	case cur:
		noInk = theme.InkOnPrimary
	case watched:
		noInk = theme.InkText3
	default: // a plain row: the number in --tx
	}
	labelStyle{Role: theme.RoleBody, Size: lesNoSize, Weight: 800, Ink: noInk}.apply(&r.no.label)
	r.no.label.SetText(r.noText)
	labelStyle{Role: theme.RoleBody, Size: lesTitleSize, Weight: 600, Ink: theme.InkText, Ellipsis: true}.apply(&r.text.title)
	metaInk := theme.InkText2
	if cur {
		metaInk = theme.InkPrimary
	}
	labelStyle{Role: theme.RoleBody, Size: lesMetaSize, Ink: metaInk, Wrap: true}.apply(&r.text.sub)

	off, locked := r.Mode == LessonOffline, r.Mode == LessonModeLocked
	r.dl.Hidden = off || locked || r.Download == DlUnavailable // out of the tree, a screen reader finds no button
	r.tri.Hidden = off || locked
	r.play.Hidden = !off || r.Download != DlDone
	r.cloud.Hidden = !off || r.Download == DlDone
	triInk, glyph := theme.InkText, icon.Tri
	if cur {
		triInk, glyph = theme.InkPrimary, icon.Trifill
	}
	r.tri.Glyph, r.tri.Size, r.tri.Ink = glyph, lesTri, triInk
	r.cloud.Glyph, r.cloud.Size, r.cloud.Ink = icon.Cloud, lesCloud, theme.InkText2
}

// sync makes the download button show the state: the icon and its color.
func (d *lessonDl) sync() {
	ink, glyph, size, stroke := theme.InkText3, icon.Dl, lesDlIcon, lesDlStroke
	switch d.row.Download {
	case DlIng:
		ink, glyph, size, stroke = theme.InkPrimary, icon.X, lesCross, lesCrossStroke
	case DlDone:
		ink, stroke = theme.InkSuccess, lesDlStrokeOn
	case DlNone, DlUnavailable, LessonDownloadUnknown:
	default:
	}
	d.icon.Glyph, d.icon.Size, d.icon.Stroke, d.icon.Ink = glyph, size, stroke, ink
}

// track returns the path of the ring's track, the whole circle, remade only when
// the button's size changes.
func (d *lessonDl) track(cx, cy, r float32) *scene.Path {
	d.trackPath.Reset()
	d.trackPath.Circle(cx, cy, r)

	return &d.trackPath
}

// pressDl and pressPlay forward the buttons' presses.
func (r *LessonRow) pressDl() {
	if r.OnDl != nil {
		r.OnDl()
	}
}

func (r *LessonRow) pressPlay() {
	if r.OnPlay != nil {
		r.OnPlay()
	}
}

// twoDigits returns n as at least two digits, `String(n).padStart(2,'0')`.
func twoDigits(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		return "0" + s
	}

	return s
}

// arc makes the path of the progress: from 12 o'clock, clockwise, as long as p of
// the ring's dash (`stroke-dasharray="${94.2*p} 94.2"` of a circle whose
// circumference is 94.248), in cubic segments of at most 90 degrees.
func (d *lessonDl) arc(cx, cy, r, p float32) {
	if d.ringP == p && !d.ring.Empty() {
		return
	}
	d.ringP = p
	d.ring.Reset()
	sweep := float64(min(lesRingLen*p, lesRingLen)) / (2 * math.Pi * float64(lesRingR)) * 2 * math.Pi
	n := int(math.Ceil(sweep / (math.Pi / 2)))
	if n < 1 {
		n = 1
	}
	step := sweep / float64(n)
	k := 4.0 / 3.0 * math.Tan(step/4) * float64(r)
	a := -math.Pi / 2
	d.ring.MoveTo(cx+float32(float64(r)*math.Cos(a)), cy+float32(float64(r)*math.Sin(a)))
	for range n {
		b := a + step
		x0, y0 := math.Cos(a), math.Sin(a)
		x1, y1 := math.Cos(b), math.Sin(b)
		d.ring.CubicTo(
			cx+float32(float64(r)*x0-k*y0), cy+float32(float64(r)*y0+k*x0),
			cx+float32(float64(r)*x1+k*y1), cy+float32(float64(r)*y1-k*x1),
			cx+float32(float64(r)*x1), cy+float32(float64(r)*y1))
		a = b
	}
}
