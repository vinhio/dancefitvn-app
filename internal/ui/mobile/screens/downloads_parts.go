//go:build !tvos

// The parts of screen 11 are written from MS.downloads (index.html:450-460) and
// its M11 capture (local/shots/prototype-kit/mobile/M11.layout.json). The way a
// block's margins collapse into the room between two blocks follows the stacks of
// courses_content.go (jplayer-mobile/internal/ui/views/home.go at 49de3c2).

package screens

import (
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The margins of the blocks of MS.downloads (index.html:452-458), in DIPs.
const (
	// dlTitleTop, dlTitleBottom, dlTitleBottomEmpty are the `.h1`'s `margin:16px
	// 0 8px` (:455) and `margin:16px 0` of the empty screen (:451).
	dlTitleTop, dlTitleBottom, dlTitleBottomEmpty float32 = 16, 8, 16
	// dlSummarySize is the summary's `font-size:13px` (:455).
	dlSummarySize float32 = 13
	// dlCardMargin is the storage card's `margin:14px 0` (:456).
	dlCardMargin float32 = 14
	// dlRunTop, dlRunBottom are the eyebrow "Đang tải · n"'s `margin:18px 0 8px`
	// (:457); dlRunCardBottom the running card's `margin-bottom:8px`.
	dlRunTop, dlRunBottom, dlRunCardBottom float32 = 18, 8, 8
	// dlGroupTop, dlGroupBottom are the group eyebrow's `margin:20px 0 4px` (:459).
	dlGroupTop, dlGroupBottom float32 = 20, 4
)

// The measures of the storage card (index.html:456).
const (
	// dlStoragePad is `.card{padding:14px}`.
	dlStoragePad float32 = 14
	// dlStorageIcon is `ic('store',20)`; dlStorageSize the `<b>`'s and the
	// `font-size:14px` of the line, dlStorageValue the value's `font-size:12px`.
	dlStorageIcon, dlStorageSize, dlStorageValue float32 = 20, 14, 12
	// dlStorageGap is the space between the icon and the text: Inter's at 14 px.
	dlStorageGap float32 = 4
	// dlStorageLine is the height of the line: the 20 icon on the baseline of
	// the 14 px text and the text's descent under it (M11: `span.muted` 23).
	dlStorageLine float32 = 23
	// dlStorageBar, dlStorageBarTop are `.prog` at `margin-top:12px;height:8px`.
	dlStorageBar, dlStorageBarTop float32 = 8, 12
)

// The measures of a running card (index.html:458).
const (
	// dlRunPad, dlRunGap are `padding:12px;gap:12px`.
	dlRunPad, dlRunGap float32 = 12, 12
	// dlPoster, dlPosterR are `poster(c,44,44,{r:10,...})`; dlPosterIcon the
	// `ic('dl',20,2.2)` over it (the row's is `ic('hp',20,2)`).
	dlPoster, dlPosterR, dlPosterIcon  float32 = 44, 10, 20
	dlPosterDlStroke, dlPosterHpStroke float32 = 2.2, 2
	// dlRunTitleSize is `<b style="font-size:14px">`.
	dlRunTitleSize float32 = 14
	// dlRunBarTop, dlRunBarBottom are `.prog` at `margin:7px 0 5px`.
	dlRunBarTop, dlRunBarBottom float32 = 7, 5
	// dlRunSubSize is the text's `font-size:12px`.
	dlRunSubSize float32 = 12
	// dlRunCancel, dlRunCancelIcon are `.rb` (40) and its `ic('x',18)`.
	dlRunCancel, dlRunCancelIcon float32 = 40, 18
)

// The measures of a file's row (index.html:460).
const (
	// dlRowMin is `.row` at `min-height:64px`; dlRowGap its `gap:12px`; dlRowBorder
	// the line under it (the last row of the screen has none).
	dlRowMin, dlRowGap, dlRowBorder float32 = 64, 12, 1
	// dlRowTitleSize, dlRowTitleWeight are `<b style="font-size:14px">` of
	// `.row .tt b{font-weight:600}`; dlRowSubSize `.row .tt span{font-size:12px}`.
	dlRowTitleSize, dlRowSubSize float32 = 14, 12
	// dlRowValSize is `.val` at `font-size:12px`.
	dlRowValSize float32 = 12
	// dlMoreW, dlMoreH, dlMorePad, dlMoreIcon are the `button` of `askDelete`: 6 of
	// padding around the 22 `ic('more')` on a 26 line.
	dlMoreW, dlMoreH, dlMorePad, dlMoreIcon float32 = 34, 38, 6, 22
)

// blockItem is a part of a blockStack with the margins above and under it.
type blockItem struct {
	w           ui.Widget
	top, bottom float32
}

// blockStack stacks its parts as the block flow does: each as wide as the room,
// the margin under one and the margin over the next collapsing into the larger.
// A hidden part takes no room.
type blockStack struct {
	ui.Base

	items []blockItem
}

// storageCard is `.card` of "Bộ nhớ máy" (:456): the store icon and the text at
// the left, "Trống 18,2 GB" at the right, the 8 tall bar of the used share under
// them.
type storageCard struct {
	ui.Base

	// freePre and freePost are the text of "Trống {gb}" around the size.
	freePre, freePost string

	icon  *kit.Icon
	title *kit.Label
	value *kit.Label
	bar   *kit.Progress
}

// runCard is the card of a download under way (:458): `.card` around a runRow.
type runCard struct {
	ui.Base

	// LessonID is the lesson the card is for.
	LessonID string

	card *kit.Card
	row  *runRow
	// wait is the sub line of a paused download, "Đang chờ Wi‑Fi để tải" (K23,
	// Q-P5.2-1): the bar keeps the share it reached.
	wait string
}

// runRow is the card's content: the poster with the arrow, the lesson, the bar and
// the text of the progress, the cross that cancels.
type runRow struct {
	ui.Base

	poster *poster.Poster
	text   *runText
	cancel *kit.RoundButton
	buf    []byte // the text of the progress being made, reused
}

// runText is the card's text: the title on the strut, the bar 7 under it, the
// progress text 5 under that.
type runText struct {
	ui.Base

	title *inlineLine
	bar   *kit.Progress
	sub   *kit.Label
}

// fileRow is the row of a downloaded file (:460): the poster with the headphones,
// the lesson, its size and the button of its options; a tap plays the file.
type fileRow struct {
	ui.Pressable

	poster *poster.Poster
	text   *fileText
	val    *kit.Label
	more   *moreButton
	last   bool // the last row of the screen has no line under it
	label  string
}

// fileText is `span.tt`: the title as a block over the tag and length of the
// lesson, a line of the page's strut.
type fileText struct {
	ui.Base

	title *kit.Label
	sub   *kit.Label
}

// moreButton is `<button data-act="askDelete" style="color:var(--tx2);padding:
// 6px">` around `ic('more')`.
type moreButton struct {
	ui.Pressable

	icon  *kit.Icon
	label string
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newStorageCard returns the card of the device's space with nothing read yet.
func newStorageCard(e *env.Env) (*kit.Card, *storageCard) {
	s := &storageCard{
		icon:  kit.NewIcon(icon.Store, dlStorageIcon, theme.InkText),
		title: &kit.Label{Text: e.T("downloads.storage"), Size: dlStorageSize, Weight: 700}, // `${ic} Bộ nhớ máy`
		value: kit.Muted("", dlStorageValue),
		bar:   &kit.Progress{Height: dlStorageBar, Label: e.T("downloads.storage")},
	}
	s.freePre, s.freePost, _ = strings.Cut(e.T("downloads.freeTpl"), "{gb}")
	s.value.Name = "span.muted"
	s.bar.Fill = tokens.Dark.Text3 // `<i style="background:var(--tx3)">`
	s.Add(s.icon, s.title, s.value, s.bar)
	card := kit.NewCard(s)
	card.Padding = geom.Uniform(dlStoragePad)

	return card, s
}

// newRunCard returns the card of a download under way.
func newRunCard(e *env.Env, it env.DownloadItem) *runCard {
	runTitle := Fill(e.T("downloads.itemTpl"), "no", format.No(it.No), "title", it.Title)
	r := &runRow{
		poster: dlPosterOf(e, it, icon.Dl, dlPosterDlStroke),
		text: &runText{
			title: newInline(&kit.Label{Text: runTitle, Size: dlRunTitleSize, Weight: 700}),
			bar:   &kit.Progress{Label: runTitle},
			sub:   kit.Muted("", dlRunSubSize),
		},
		cancel: kit.NewRoundButton(icon.X, false, e.T("a11y.cancelDownload")),
		buf:    make([]byte, 0, 48),
	}
	r.text.sub.Name = "span.muted"
	r.text.Add(r.text.title, r.text.bar, r.text.sub)
	r.cancel.Name, r.cancel.IconSize = "dlTap:"+LessonKey(it.CourseID, it.No), dlRunCancelIcon // `data-act="dlTap" data-k`
	id, course, no := it.LessonID, it.CourseID, it.No
	r.cancel.OnPress = func() { e.Act.DlTap(course, id, no) }
	r.cancel.Bg = tokens.Dark.Surface2 // `<button class="rb" style="background:var(--sf2)">`
	r.Add(r.poster, r.text, r.cancel)
	card := kit.NewCard(r)
	card.Padding = geom.Uniform(dlRunPad)
	k := &runCard{LessonID: it.LessonID, card: card, row: r}
	if it.Paused {
		k.wait = e.T("toast.waitWiFi")
	}
	k.Add(card)
	k.set(it.Done, it.Bytes, it.Progress)

	return k
}

// newFileRow returns the row of a downloaded file; last drops the line under it.
func newFileRow(e *env.Env, it env.DownloadItem, last bool) *fileRow {
	title := Fill(e.T("downloads.itemTpl"), "no", format.No(it.No), "title", it.Title)
	k := LessonKey(it.CourseID, it.No)
	r := &fileRow{
		poster: dlPosterOf(e, it, icon.Hp, dlPosterHpStroke),
		text: &fileText{
			title: &kit.Label{Text: title, Size: dlRowTitleSize, Weight: 600, Wrap: true},
			sub:   kit.Muted(format.Join(it.Tag, format.Clock(float64(it.DurationSec))), dlRowSubSize), // `${l.tag} · ${fmt(l.dur)}`
		},
		val:   kit.Muted(format.Size(it.Bytes), dlRowValSize),
		more:  &moreButton{icon: kit.NewIcon(icon.More, dlMoreIcon, theme.InkText2), label: e.T("a11y.fileOptions")},
		last:  last,
		label: title,
	}
	r.text.Name, r.text.title.Name = "span.tt", "b"
	r.text.sub.Wrap = true
	r.text.Add(r.text.title, r.text.sub)
	r.val.Name = "span.val"
	r.more.Name = "askDelete:" + k // `data-act="askDelete" data-k`
	r.more.Look = noDecoration
	r.more.Add(r.more.icon)
	r.Name = "playAudio:" + k // `data-act="playAudio" data-k`
	r.Look = noDecoration
	course, id, no := it.CourseID, it.LessonID, it.No
	r.OnPress = func() { e.Act.PlayAudio(course, id, no) }
	r.more.OnPress = func() { e.Act.AskDelete(course, id, no) }
	r.Add(r.poster, r.text, r.val, r.more)

	return r
}

// dlPosterOf returns the 44 poster of the item's course with no title and no figure, the
// icon g over it in white (`poster(c,44,44,{r:10,title:false,fig:false,extra})`).
func dlPosterOf(e *env.Env, it env.DownloadItem, g icon.Name, stroke float32) *poster.Poster {
	p := poster.New(poster.Spec{ID: it.CourseID, Radius: dlPosterR, NoTitle: true, NoFigure: true}, dlPoster, dlPoster)
	p.SetCover(e.Cover, it.CoverURL) // the real cover once loaded (K14)
	ic := kit.NewIcon(g, dlPosterIcon, theme.InkWhite)
	ic.Stroke = stroke
	p.SetExtra(ic)

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Layout stacks the parts across the room, the margins between them collapsed.
func (s *blockStack) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	y, prev := float32(0), float32(0)
	for _, it := range s.items {
		if it.w.Node().Hidden {
			c.Layout(it.w, ui.Tight(geom.Size{})) // no room, and no layout kept: shown again, it lays out afresh

			continue
		}
		y += max(prev, it.top)
		sz := c.Layout(it.w, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
		c.Place(it.w, geom.Pt(0, y))
		y += sz.H
		prev = it.bottom
	}

	return cs.Constrain(geom.Sz(w, y+prev))
}

// set shows the free space: the text at the right and the share used as the bar;
// with none known (Q-P5-9) the text is empty and the bar 0. The same reading
// changes nothing and does not allocate.
func (s *storageCard) set(free, total int64, ok bool) {
	var share float32
	var buf [32]byte
	text := buf[:0]
	if ok && total > 0 {
		share = float32(total-free) / float32(total)
		text = append(text, s.freePre...) // downloads.freeTpl, filled without allocating
		text = format.AppendGB(text, free)
		text = append(text, s.freePost...)
	}
	if string(text) != s.value.Text {
		s.value.SetText(string(text))
	}
	s.bar.SetValue(share)
}

// Layout puts the icon and the title on one baseline, the value at the right end
// of the line with the line's height, the bar under them.
func (s *storageCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	is := c.Layout(s.icon, ui.Tight(geom.Sz(dlStorageIcon, dlStorageIcon)))
	c.Place(s.icon, geom.Point{})
	c.Layout(s.title, ui.Loose(geom.Sz(w, ui.Unbounded)))
	// the 20 icon stands on the baseline of the line: the text's is as far down
	c.Place(s.title, geom.Pt(is.W+dlStorageGap, is.H-s.title.Baseline()))
	vs := c.Layout(s.value, ui.Constraints{Min: geom.Sz(0, dlStorageLine), Max: geom.Sz(w, dlStorageLine)})
	c.Place(s.value, geom.Pt(w-vs.W, 0))
	c.Layout(s.bar, ui.Tight(geom.Sz(w, dlStorageBar)))
	c.Place(s.bar, geom.Pt(0, dlStorageLine+dlStorageBarTop))

	return cs.Constrain(geom.Sz(w, dlStorageLine+dlStorageBarTop+dlStorageBar))
}

// set moves the bar to done of total and writes the text "6,3 / 9,8 MB · 64%"; the
// text is set only when it changes, so an event that moves nothing allocates
// nothing. A job whose total is not known yet shows the share the item reports.
func (k *runCard) set(done, total int64, share float32) {
	r := k.row
	frac := float64(share)
	if total > 0 {
		frac = float64(done) / float64(total)
	}
	r.text.bar.SetValue(float32(frac))
	if k.wait != "" {
		if k.wait != r.text.sub.Text {
			r.text.sub.SetText(k.wait)
		}

		return
	}
	r.buf = format.AppendProgress(r.buf[:0], frac, total)
	if string(r.buf) != r.text.sub.Text {
		r.text.sub.SetText(string(r.buf))
	}
}

// Layout is the card's size.
func (k *runCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	sz := c.Layout(k.card, cs)
	c.Place(k.card, geom.Point{})

	return cs.Constrain(sz)
}

// Layout puts the poster, the text, which takes the room, and the cross, 12 apart,
// centered in the row's height.
func (r *runRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	c.Layout(r.poster, ui.Tight(geom.Sz(dlPoster, dlPoster)))
	c.Layout(r.cancel, ui.Tight(geom.Sz(dlRunCancel, dlRunCancel)))
	tw := max(w-dlPoster-dlRunGap-dlRunGap-dlRunCancel, 0)
	ts := c.Layout(r.text, ui.Constraints{Min: geom.Sz(tw, 0), Max: geom.Sz(tw, ui.Unbounded)})
	h := max(dlPoster, ts.H, dlRunCancel)
	c.Place(r.poster, geom.Pt(0, (h-dlPoster)/2))
	c.Place(r.text, geom.Pt(dlPoster+dlRunGap, (h-ts.H)/2))
	c.Place(r.cancel, geom.Pt(w-dlRunCancel, (h-dlRunCancel)/2))

	return cs.Constrain(geom.Sz(w, h))
}

// Layout stacks the title on the strut, the bar and the line of the text.
func (t *runText) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	col := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	ts := c.Layout(t.title, col)
	c.Place(t.title, geom.Point{})
	y := ts.H + dlRunBarTop
	c.Layout(t.bar, ui.Tight(geom.Sz(w, 5)))
	c.Place(t.bar, geom.Pt(0, y))
	y += 5 + dlRunBarBottom
	// the text is an inline run on a line of the page's 16 px strut
	st := strut(c)
	t.sub.Name = "span.muted"
	c.Layout(t.sub, ui.Loose(geom.Sz(w, ui.Unbounded)))
	c.Place(t.sub, geom.Pt(0, y+st.Baseline-t.sub.Baseline()))

	return cs.Constrain(geom.Sz(w, y+st.Height))
}

// AccessibleLabel returns the lesson's title.
func (r *fileRow) AccessibleLabel() string { return r.label }

// Describe describes the row as a list item named by the lesson's title, valued by
// its tag, length and size; its button of options is an element under it
// (DescribeChildren), which a plain button could not hold.
func (r *fileRow) Describe(c *ui.Context, n *platform.SemanticNode) {
	r.Pressable.Describe(c, n)
	n.Role = platform.RoleListItem
	n.Value = r.text.sub.Text + ", " + r.val.Text
}

// DescribeChildren adds the button of options as an element under the row.
func (r *fileRow) DescribeChildren(c *ui.Context, add func(platform.SemanticNode)) {
	kit.DescribeButtons(c, r, add, r.more)
}

// DescribesSubtree tells that the button is all the row has to add.
func (r *fileRow) DescribesSubtree() bool { return true }

// PerformChildAction presses the button of options.
func (r *fileRow) PerformChildAction(_ *ui.Context, key uint64, a platform.SemanticAction) bool {
	return kit.PressButton(a, key, r.more)
}

// Layout puts the poster, the text, which takes the room, the size and the button,
// 12 apart, centered in the height of the row (at least 64, with its line).
func (r *fileRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	border := dlRowBorder
	if r.last {
		border = 0
	}
	loose := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	c.Layout(r.poster, ui.Tight(geom.Sz(dlPoster, dlPoster)))
	vs := c.Layout(r.val, loose)
	ms := c.Layout(r.more, ui.Tight(geom.Sz(dlMoreW, dlMoreH)))
	tw := max(w-dlPoster-dlRowGap-dlRowGap-vs.W-dlRowGap-ms.W, 0)
	ts := c.Layout(r.text, ui.Constraints{Max: geom.Sz(tw, ui.Unbounded)})
	h := max(dlRowMin, max(dlPoster, ts.H, ms.H)+border)
	ch := h - border
	c.Place(r.poster, geom.Pt(0, (ch-dlPoster)/2))
	c.Place(r.text, geom.Pt(dlPoster+dlRowGap, (ch-ts.H)/2))
	end := w - ms.W
	c.Place(r.more, geom.Pt(end, (ch-ms.H)/2))
	end -= dlRowGap + vs.W
	c.Place(r.val, geom.Pt(end, (ch-vs.H)/2))

	return cs.Constrain(geom.Sz(w, h))
}

// Paint draws the line under the row and the parts.
func (r *fileRow) Paint(c *ui.Context, p *ui.Painter) {
	if !r.last {
		b := r.Bounds()
		p.Fill(geom.XYWH(0, b.H-dlRowBorder, b.W, dlRowBorder), scene.Solid(theme.From(c).T.Border), 0)
	}
	r.Base.Paint(c, p)
}

// Layout stacks the title as a block over the line of the tag: the line sits on
// the strut, 20 tall, as the sub line of a kit.Row does.
func (t *fileText) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	ts := c.Layout(t.title, ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)})
	c.Place(t.title, geom.Point{})
	st := strut(c)
	c.Layout(t.sub, ui.Loose(geom.Sz(w, ui.Unbounded)))
	c.Place(t.sub, geom.Pt(0, ts.H+st.Baseline-t.sub.Baseline()))

	return cs.Constrain(geom.Sz(w, ts.H+st.Height))
}

// AccessibleLabel names the button.
func (b *moreButton) AccessibleLabel() string { return b.label }

// Layout takes the size of the button and puts the icon inside its padding.
func (b *moreButton) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(b.icon, ui.Tight(geom.Sz(dlMoreIcon, dlMoreIcon)))
	c.Place(b.icon, geom.Pt(dlMorePad, dlMorePad))

	return cs.Constrain(geom.Sz(dlMoreW, dlMoreH))
}
