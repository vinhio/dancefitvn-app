//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/detail.go (the course
// read after Mounted, one at a time, dropped when the screen is left) for the TV;
// the lessons slide as the prototype's scrollTV does (tvkit.Slide). The numbers
// are TS.detail's (index.html:675-679).

package screens

import (
	"context"
	"strconv"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/focus"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.detail (index.html:675-679), in DIPs.
const (
	// detailPosterX, detailPosterY, detailPosterW, detailPosterH, detailPosterTS
	// are `poster(id,760,428,{r:24,ts:80})` at `left:90px;top:120px`.
	detailPosterX, detailPosterY, detailPosterW, detailPosterH, detailPosterTS float32 = 90, 120, 760, 428, 80
	// detailPosterR is the poster's `r:24`.
	detailPosterR float32 = 24
	// detailBlockX, detailBlockY, detailBlockW are `left:90px;top:590px;width:780px`.
	detailBlockX, detailBlockY, detailBlockW float32 = 90, 590, 780
	// detailEyebrowSize, detailEyebrowSpacing are `font-size:24px;font-weight:800;
	// letter-spacing:2px`.
	detailEyebrowSize, detailEyebrowSpacing float32 = 24, 2
	// detailNameTop, detailNameSize are `.d` at `font-size:84px;font-weight:800;
	// margin-top:22px`.
	detailNameTop, detailNameSize float32 = 22, 84
	// detailNameLines is how many lines the name may take before it ends in an
	// ellipsis (the prototype has no limit; 3 lines would push the buttons off the screen).
	detailNameLines = 2
	// detailQuoteTop, detailQuoteSize are `p.muted` at `font-size:34px;margin-top:14px`.
	detailQuoteTop, detailQuoteSize float32 = 14, 34
	// detailNoteTop, detailNoteSize are `font-size:26px;color:var(--tx3);margin-top:20px`.
	detailNoteTop, detailNoteSize float32 = 20, 26
	// detailRowTop, detailRowGap are `display:flex;gap:30px;margin-top:40px`.
	detailRowTop, detailRowGap float32 = 40, 30
	// detailPlayIcon is the PLAY span of `d1`, `width:30px`.
	detailPlayIcon float32 = 30
	// detailListX, detailListY, detailListRight are the list's clip, `left:960px;
	// right:90px;top:110px;bottom:0`.
	detailListX, detailListY, detailListRight float32 = 960, 110, 90
	// detailHeadSize, detailHeadSpacing, detailHeadBottom are the heading, `font-size:
	// 26px;font-weight:800;letter-spacing:3px;margin:0 0 24px`.
	detailHeadSize, detailHeadSpacing, detailHeadBottom float32 = 26, 3, 24
	// detailSquare, detailSquareGap are the red square before it, `width:12px;
	// height:12px;margin-right:14px`.
	detailSquare, detailSquareGap float32 = 12, 14
	// detailListPad, detailRowPitch are `#tvList{padding:8px}` and the 88 of a row
	// with its 12 of margin.
	detailListPad, detailRowPitch float32 = 8, 100
)

// The keys of the controls: `d1`, `d2` and the rows `dl*`.
const (
	keyDetailPlay = "d1"
	keyDetailBack = "d2"
	keyDetailRow  = "dl"
)

// Names layout.json knows the parts by.
const (
	nameDetailName  = "div.d"
	nameDetailQuote = "p.muted"
	nameDetailList  = "tv.detail.list"
	nameDetailHead  = "tv.detail.head"
	nameDetailNote  = "tv.detail.note"
	nameDetailEyeb  = "tv.detail.eyebrow"
)

// Detail is TS.detail (TV06): a course with its poster, name, tagline and the two
// buttons `d1` (plays the lesson to go on with, "Tiếp tục bài NN", or the first,
// "Xem bài 01") and `d2` (Quay lại), and the list of its lessons at the right,
// `dl0`.., each of which plays. The list slides over its heading as the focus goes
// down it. Menu goes back. While the course is read it shows the background alone
// (K12); a read that fails shows TV11 (K28). `d1` has the focus first.
//
// The lesson `d1` goes on with is the one the player has when it is of this course
// (`S.pt.l.cid===id`), else the one the API says to resume (`resume.lesson_id`),
// else the first.
type Detail struct {
	ui.Base

	e  *env.Env
	id string

	body   *detailBody
	anchor *waitAnchor

	course model.Course
	list   []player.Lesson
	have   bool

	rd reader // the read of the course
}

// detailBody is the poster, the text block, the buttons and the list.
type detailBody struct {
	ui.Base

	poster  *poster.Poster
	eyebrow *kit.Label
	name    *kit.Label
	quote   *kit.Label // nil for a course without a tagline
	note    *kit.Label
	play    *tvkit.Button
	back    *tvkit.Button
	slide   *tvkit.Slide
}

// detailHead is the heading of the list: the red square and "BÀI HỌC", with the 24
// of margin under it that the list starts after.
type detailHead struct {
	ui.Base

	label kit.Label
}

// detailList is `#tvList`: the lesson rows, 100 apart, the focused one painted
// over the next (F-12).
type detailList struct {
	ui.Base

	rows []*tvkit.Row
}

var (
	_ ui.Mounter   = (*Detail)(nil)
	_ ui.Unmounter = (*Detail)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewDetail returns TV06 of the course id: the background alone until the course
// is read.
func NewDetail(e *env.Env, id string) *Detail {
	s := &Detail{e: e, id: id, body: &detailBody{}}
	s.anchor = newWaitAnchor(e, keyDetailPlay)
	s.Add(s.body, s.anchor)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`d1`, `d2`, `dl0`..), nil when there is
// none yet.
func (s *Detail) Item(key string) *tvkit.Item {
	b := s.body
	switch key {
	case keyDetailPlay:
		if b.play != nil {
			return &b.play.Item
		}
	case keyDetailBack:
		if b.back != nil {
			return &b.back.Item
		}
	default:
		if b.slide != nil {
			if l, ok := b.slide.Content().(*detailList); ok {
				for _, r := range l.rows {
					if r.Key == key {
						return &r.Item
					}
				}
			}
		}
	}

	return nil
}

// Slide returns the list's clip, nil until the course is read.
func (s *Detail) Slide() *tvkit.Slide { return s.body.slide }

// Course returns the course shown, the zero course until it is read.
func (s *Detail) Course() model.Course { return s.course }

// Loaded tells whether the content is on screen.
func (s *Detail) Loaded() bool { return s.have }

// Mounted starts reading with the next turn.
func (s *Detail) Mounted(c *ui.Context) {
	if !s.have && !s.rd.Busy() {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted ends the read in flight.
func (s *Detail) Unmounted(*ui.Context) { s.rd.Stop() }

// Layout fills the room with the content.
func (s *Detail) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(s.body, ui.Tight(size))
	c.Place(s.body, geom.Point{})
	if s.anchor != nil {
		c.Layout(s.anchor, ui.Loose(geom.Sz(1, 1)))
		c.Place(s.anchor, geom.Pt(size.W/2, size.H/2))
	}

	return size
}

// Layout places the poster, flows the text block under it as CSS does and puts
// the list's clip at the right, down to the bottom edge.
func (b *detailBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if b.poster == nil {
		return size
	}
	c.Layout(b.poster, ui.Tight(geom.Sz(detailPosterW, detailPosterH)))
	c.Place(b.poster, geom.Pt(detailPosterX, detailPosterY))

	block := ui.Constraints{Min: geom.Sz(detailBlockW, 0), Max: geom.Sz(detailBlockW, ui.Unbounded)}
	y := detailBlockY
	es := c.Layout(b.eyebrow, block)
	c.Place(b.eyebrow, geom.Pt(detailBlockX, y))
	y += es.H + detailNameTop
	ns := c.Layout(b.name, block)
	c.Place(b.name, geom.Pt(detailBlockX, y))
	y += ns.H
	if b.quote != nil {
		y += detailQuoteTop
		qs := c.Layout(b.quote, block)
		c.Place(b.quote, geom.Pt(detailBlockX, y))
		y += qs.H
	}
	y += detailNoteTop // the margins of an empty tagline and of the note meet: the larger, 20
	ts := c.Layout(b.note, block)
	c.Place(b.note, geom.Pt(detailBlockX, y))
	y += ts.H + detailRowTop
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	ps := c.Layout(b.play, free)
	c.Place(b.play, geom.Pt(detailBlockX, y))
	c.Layout(b.back, free)
	c.Place(b.back, geom.Pt(detailBlockX+ps.W+detailRowGap, y))

	c.Layout(b.slide, ui.Tight(geom.Sz(size.W-detailListRight-detailListX, size.H-detailListY)))
	c.Place(b.slide, geom.Pt(detailListX, detailListY))

	return size
}

// Paint draws the content, the focused control last.
func (b *detailBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// Layout puts the red square and the text on one line, the square's bottom on the
// text's baseline (an inline block sits on it), and is as tall as the line and the
// margin under it.
func (h *detailHead) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	ls := c.Layout(&h.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&h.label, geom.Pt(detailSquare+detailSquareGap, 0))

	return cs.Constrain(geom.Sz(cs.Max.W, ls.H+detailHeadBottom))
}

// LayoutStyle is the heading's text style and its words.
func (h *detailHead) LayoutStyle(c *ui.Context) (kit.CSS, string) { return h.label.LayoutStyle(c) }

// Paint draws the square, then the text.
func (h *detailHead) Paint(c *ui.Context, p *ui.Painter) {
	red := theme.From(c).T.Red
	p.Fill(geom.XYWH(0, h.label.Baseline()-detailSquare, detailSquare, detailSquare), scene.Solid(red), 0)
	h.Base.Paint(c, p)
}

// Layout puts the rows in a column, 100 apart, in the padding of `#tvList`.
func (l *detailList) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := max(cs.Max.W-2*detailListPad, 0)
	y := detailListPad
	for _, r := range l.rows {
		c.Layout(r, ui.Tight(geom.Sz(w, 88)))
		c.Place(r, geom.Pt(detailListPad, y))
		y += detailRowPitch
	}

	return cs.Constrain(geom.Sz(cs.Max.W, y+detailListPad))
}

// Paint draws the rows, the focused one last.
func (l *detailList) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &l.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads GET /courses/{id}.
func (s *Detail) load() {
	if s.have {
		return
	}
	read(&s.rd, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, s.id)
	}, func(c *ui.Context, course model.Course, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.show(c, course)
	})
}

// show builds the content from the course.
func (s *Detail) show(c *ui.Context, course model.Course) {
	s.course, s.have = course, true
	s.e.RememberCourse(course)
	s.list = env.PlayerLessons(course)
	b := s.body
	e := s.e
	n := course.LessonCount
	if n == 0 {
		n = len(course.Lessons)
	}

	spec := poster.ForCourse(course)
	spec.Radius, spec.TitleSize = detailPosterR, detailPosterTS
	b.poster = poster.New(spec, detailPosterW, detailPosterH)
	b.poster.SetCover(e.Cover, course.CoverURL) // the real cover once loaded (K14)
	b.eyebrow = &kit.Label{
		Text: e.Fill("tv.detail.eyebrowTpl", "cat", course.CategoryLabel, "n", strconv.Itoa(n)),
		Size: detailEyebrowSize, Weight: 800, Spacing: detailEyebrowSpacing, Upper: true, Ink: theme.InkPrimary,
	}
	b.eyebrow.Name = nameDetailEyeb
	b.name = &kit.Label{
		Text: course.Name, Role: theme.RoleDisplay, Size: detailNameSize, Weight: 800,
		LineHeight: 1, Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkText,
		// `.d` is a block of the 780 px column (:677): a long name wraps inside it
		// instead of running over the lesson list. The prototype's names are one
		// line, so it sets no limit; two lines keep the buttons on the screen.
		Wrap: true, MaxLines: detailNameLines, Ellipsis: true,
	}
	b.name.Name = nameDetailName
	b.name.Heading = 1 // the course, the title of the screen
	b.quote = nil
	if course.Tagline != "" {
		b.quote = &kit.Label{Text: "“" + course.Tagline + "”", Size: detailQuoteSize, Ink: theme.InkText2}
		b.quote.Name = nameDetailQuote
	}
	b.note = &kit.Label{Text: e.T("tv.detail.note"), Size: detailNoteSize, Ink: theme.InkText3}
	b.note.Name = nameDetailNote

	target := s.target()
	label := e.T("tv.detail.first")
	if target.resume {
		label = e.Fill("tv.detail.resumeTpl", "nn", pad2(s.list[target.index].No))
	}
	b.play = tvkit.NewButton(keyDetailPlay, label, icon.Play)
	b.play.IconSize, b.play.PlainIcon = detailPlayIcon, true
	b.play.OnSelect = func() { s.playAt(target.index) }
	b.back = tvkit.NewButton(keyDetailBack, e.T("tv.back"), icon.NameUnknown)
	b.back.OnSelect = func() {
		if e.Act != nil {
			e.Act.Back()
		}
	}
	b.back.OnMove = s.toList // Right from the last button goes to the first lesson in sight (K33)

	head := &detailHead{}
	head.Name = nameDetailHead
	head.label.Text, head.label.Size, head.label.Weight = e.T("tv.detail.lessons"), detailHeadSize, 800
	head.label.Spacing, head.label.Ink = detailHeadSpacing, theme.InkText
	head.label.Heading = 2 // the title of the list
	head.Add(&head.label)
	rows := &detailList{}
	rows.Name = "tvList"
	for i, l := range course.Lessons {
		sub := e.Fill("tv.detail.rowTpl", "tag", l.Tag, "dur", format.Clock(float64(l.DurationSec)))
		if l.Progress.Watched || e.WatchedLocal(l.ID) {
			sub = format.Join(sub, e.T("lesson.watched"))
		}
		row := tvkit.NewRow(keyDetailRow+strconv.Itoa(i), pad2(l.No), l.Title, sub)
		row.OnSelect = func() { s.playAt(i) }
		rows.rows = append(rows.rows, row)
		rows.Add(row)
	}
	b.slide = tvkit.NewSlide(head, rows)
	b.slide.Name = nameDetailList
	b.Add(b.poster, b.eyebrow, b.name) // reading order is the screen's: the quote follows the name
	if b.quote != nil {
		b.Add(b.quote)
	}
	b.Add(b.note, b.play, b.back, b.slide)
	b.Relayout()
	if s.anchor != nil {
		focusFrom(c, s.anchor, b.play)
		s.Remove(s.anchor)
		s.anchor = nil
	}
	s.Relayout()
}

// detailTarget is the lesson `d1` plays: its place in the course's list, and
// whether it goes on with one (else it starts the first).
type detailTarget struct {
	index  int
	resume bool
}

// target picks the lesson `d1` plays: the player's when it is of this course, the
// API's resume lesson, else the first (`S.pt.l.cid===id?S.pt.l:L[0]`, :677).
func (s *Detail) target() detailTarget {
	if pb := s.e.Playback(); pb != nil {
		if l := pb.Lesson(); l.ID != "" && l.CourseID == s.course.ID {
			if i := env.LessonIndex(s.list, l.ID); i >= 0 {
				return detailTarget{index: i, resume: true}
			}
		}
	}
	if r := s.course.Resume; r != nil {
		if i := env.LessonIndex(s.list, r.LessonID); i >= 0 {
			return detailTarget{index: i, resume: true}
		}
	}

	return detailTarget{}
}

// toList is the arrow Right on `d2`, the last of the buttons (K33): the focus goes to
// the first lesson row in sight, the top of the list. The prototype's pick scores the
// rows from the centre of the button and so lands on the row at the button's height
// (`dl7`, or the last row of a short course: the buttons sit at the foot of the left
// column, the rows start at the top), which is where a user on a remote does not look
// for it. The other arrows are left to the focus layer.
func (s *Detail) toList(d focus.Dir) bool {
	b := s.body
	v := s.View()
	if d != focus.DirRight || v == nil || b.slide == nil {
		return false
	}
	l, ok := b.slide.Content().(*detailList)
	if !ok || len(l.rows) == 0 {
		return false
	}
	clip := b.slide.WindowFrame()
	pick := l.rows[0]
	for _, r := range l.rows {
		if f := r.WindowFrame(); f.Max().Y > clip.Y+detailListPad && f.Y < clip.Max().Y {
			pick = r

			break
		}
	}
	v.Context().Focus(pick)

	return true
}

// playAt is `data-tact="playLes"` (:677-678): lesson i of the course plays.
func (s *Detail) playAt(i int) {
	if i < 0 || i >= len(s.list) {
		return
	}
	playLesson(s.e, s.list[i], s.list)
}
