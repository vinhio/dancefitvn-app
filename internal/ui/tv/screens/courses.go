//go:build tvos

// Adapted from jplayer-tv/internal/ui/views/grid.go at 781bee5 (a grid of cards
// laid out in rows from the data, the focused card painted over its neighbours)
// with the scroll of the prototype's scrollTV in place of jPlayer's ui.Scroll
// (tvkit.Slide); the numbers are TS.courses' (index.html:671-674).

package screens

import (
	"context"
	"strconv"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.courses (index.html:671-674), in DIPs.
const (
	// coursesTitleX, coursesTitleY, coursesTitleSize are the heading, `.d` at
	// `left:90px;top:160px` with a span of `font-size:60px`.
	coursesTitleX, coursesTitleY, coursesTitleSize float32 = 90, 160, 60
	// coursesChipsX, coursesChipsY, coursesChipsGap are the filters, `left:90px;
	// top:250px;gap:16px`.
	coursesChipsX, coursesChipsY, coursesChipsGap float32 = 90, 250, 16
	// coursesClipX, coursesClipY are the grid's clip, `left:90px;right:0;top:350px;
	// bottom:0`.
	coursesClipX, coursesClipY float32 = 90, 350
	// coursesCols, coursesCellW, coursesColGap, coursesRowGap, coursesPadTop,
	// coursesPadBottom are `#tvGrid{grid-template-columns:repeat(3,540px);
	// gap:40px 60px;padding:10px 0 60px}`.
	coursesCols                                                                         = 3
	coursesCellW, coursesColGap, coursesRowGap, coursesPadTop, coursesPadBottom float32 = 540, 60, 40, 10, 60
	// coursesPosterH, coursesPosterTS are `poster(id,540,304,{r:20,ts:54})`.
	coursesPosterH, coursesPosterTS float32 = 304, 54
	// coursesCaptionTop, coursesCaptionSize are the name under the poster,
	// `font-size:28px;font-weight:700;margin-top:24px`.
	coursesCaptionTop, coursesCaptionSize float32 = 24, 28
	// coursesMetaTop, coursesMetaSize, coursesMetaSpacing are the category line,
	// `font-size:22px;font-weight:800;letter-spacing:1.5px;margin-top:8px`.
	coursesMetaTop, coursesMetaSize, coursesMetaSpacing float32 = 8, 22, 1.5
)

// The filters, `['Tất cả','Thường','Đặc biệt','Giới thiệu']` (:673).
const coursesFilters = 4

// filterLabels are the locale keys of the filters' names, in order.
var filterLabels = [coursesFilters]string{"filter.all", "filter.regular", "filter.special", "filter.intro"}

// The keys of the controls: `cf*` the filters, `cg*` the grid's tiles.
const (
	keyCoursesFilter = "cf"
	keyCoursesTile   = "cg"
)

// Names layout.json knows the parts by.
const (
	nameCoursesTitle   = "div.d"
	nameCoursesGrid    = "tv.courses.grid"
	nameCoursesCaption = "tv.courses.caption"
	nameCoursesMeta    = "tv.courses.meta"
)

// Courses is TS.courses (TV05): the user's courses as a grid of three columns
// under four filters. Select on a filter keeps the focus on it and shows the
// courses of its category from the top; Select on a course opens its lessons
// (TV06). The grid slides as the focus goes down it (tvkit.Slide, `scrollTV`).
// While the courses are read it shows the nav bar alone (K12); a read that fails
// shows TV11 (K28). `cf0` has the focus first.
type Courses struct {
	ui.Base

	e *env.Env

	body   *coursesBody
	nav    *tvkit.Nav
	anchor *waitAnchor

	page   model.CoursePage
	filter int  // the filter in force, 0 to 3
	have   bool // the courses came

	rd, avatar reader // the read of the courses, the read of the user's picture
}

// coursesBody is the heading, the filters and the grid under the nav bar.
type coursesBody struct {
	ui.Base

	title *kit.Label
	chips [coursesFilters]*tvkit.FilterChip
	slide *tvkit.Slide
}

// courseGrid is `#tvGrid`: the courses' cells in three columns. The focused tile
// is painted last, over the captions of the row below (F-12).
type courseGrid struct {
	ui.Base

	cells []*courseCell
}

// courseCell is one cell of the grid: the tile, the name under it and the line of
// category and lessons.
type courseCell struct {
	tile    *tvkit.PosterTile
	caption *kit.Label
	meta    *kit.Label
	cat     model.Category // picks the color of the category line
}

var (
	_ ui.Mounter   = (*Courses)(nil)
	_ ui.Unmounter = (*Courses)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCourses returns TV05, with the nav bar and the waiting state until the
// courses are read.
func NewCourses(e *env.Env) *Courses {
	s := &Courses{e: e, body: &coursesBody{}}
	s.nav = newNavBar(e, tabCourses)
	s.anchor = newWaitAnchor(e, keyCoursesFilter+"0")
	// The nav bar is the top of the page, so a screen reader reads it first; it is
	// painted last, over the grid that slides under it (Paint).
	s.Add(s.nav, s.body, s.anchor)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`cf0`.., `cg0`..), nil when there is none.
func (s *Courses) Item(key string) *tvkit.Item {
	b := s.body
	for _, ch := range b.chips {
		if ch != nil && ch.Key == key {
			return &ch.Item
		}
	}
	if b.slide != nil {
		if g, ok := b.slide.Content().(*courseGrid); ok {
			for _, cell := range g.cells {
				if cell.tile.Key == key {
					return &cell.tile.Item
				}
			}
		}
	}

	return nil
}

// Chip returns filter i (`cf<i>`), nil until the courses are read.
func (s *Courses) Chip(i int) *tvkit.FilterChip {
	if i < 0 || i >= coursesFilters {
		return nil
	}

	return s.body.chips[i]
}

// Nav returns the nav bar.
func (s *Courses) Nav() *tvkit.Nav { return s.nav }

// Slide returns the grid's clip, nil until the courses are read.
func (s *Courses) Slide() *tvkit.Slide { return s.body.slide }

// Filter returns the filter in force, 0 (Tất cả) to 3 (Giới thiệu).
func (s *Courses) Filter() int { return s.filter }

// Cells returns how many courses the grid shows.
func (s *Courses) Cells() int {
	if s.body.slide == nil {
		return 0
	}
	g, _ := s.body.slide.Content().(*courseGrid)
	if g == nil {
		return 0
	}

	return len(g.cells)
}

// Loaded tells whether the content is on screen.
func (s *Courses) Loaded() bool { return s.have }

// Mounted starts reading with the next turn.
func (s *Courses) Mounted(c *ui.Context) {
	if !s.avatar.Done() && !s.avatar.Busy() {
		c.AfterFunc(0, s.loadAvatar)
	}
	if !s.have && !s.rd.Busy() {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted ends the reads in flight.
func (s *Courses) Unmounted(*ui.Context) {
	s.rd.Stop()
	s.avatar.Stop()
}

// Paint draws the content, then the nav bar over it: the grid slides under the bar.
func (s *Courses) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintOver(c, p, &s.Base, s.nav) }

// Layout fills the room with the content and the nav bar over it.
func (s *Courses) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(s.body, ui.Tight(size))
	c.Place(s.body, geom.Point{})
	c.Layout(s.nav, ui.Tight(size))
	c.Place(s.nav, geom.Point{})
	if s.anchor != nil {
		c.Layout(s.anchor, ui.Loose(geom.Sz(1, 1)))
		c.Place(s.anchor, geom.Pt(size.W/2, size.H/2))
	}

	return size
}

// Layout places the heading, the filters in a row and the grid's clip, which
// runs to the right and bottom edges.
func (b *coursesBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if b.title == nil {
		return size
	}
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	c.Layout(b.title, free)
	c.Place(b.title, geom.Pt(coursesTitleX, coursesTitleY))
	x := coursesChipsX
	for _, ch := range b.chips {
		cs := c.Layout(ch, free)
		c.Place(ch, geom.Pt(x, coursesChipsY))
		x += cs.W + coursesChipsGap
	}
	c.Layout(b.slide, ui.Tight(geom.Sz(size.W-coursesClipX, size.H-coursesClipY)))
	c.Place(b.slide, geom.Pt(coursesClipX, coursesClipY))

	return size
}

// Paint draws the content, the focused filter last.
func (b *coursesBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// Layout puts the cells in three columns of 540, the gaps and the padding of
// `#tvGrid`, each cell as tall as its tile and the two lines under it.
func (g *courseGrid) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	y := coursesPadTop
	for i := 0; i < len(g.cells); i += coursesCols {
		rowH := float32(0)
		for j := i; j < min(i+coursesCols, len(g.cells)); j++ {
			x := float32(j-i) * (coursesCellW + coursesColGap)
			rowH = max(rowH, g.cells[j].layout(c, x, y))
		}
		y += rowH
		if i+coursesCols < len(g.cells) {
			y += coursesRowGap
		}
	}
	y += coursesPadBottom

	return cs.Constrain(geom.Sz(cs.Max.W, y))
}

// Paint draws the cells, the focused tile last.
func (g *courseGrid) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &g.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads GET /me/courses with every page.
func (s *Courses) load() {
	if s.have {
		return
	}
	read(&s.rd, s, func(ctx context.Context) (model.CoursePage, error) {
		return s.e.Courses(ctx)
	}, func(c *ui.Context, page model.CoursePage, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.show(c, page)
	})
}

// loadAvatar reads the picture of the nav bar.
func (s *Courses) loadAvatar() { loadAvatar(s, &s.avatar, s.e, s.nav) }

// show builds the heading, the filters and the grid from the courses.
func (s *Courses) show(c *ui.Context, page model.CoursePage) {
	s.page, s.have = page, true
	b := s.body
	b.title = &kit.Label{
		Text: s.e.T("courses.title"), Role: theme.RoleDisplay, Size: coursesTitleSize, Weight: 700,
		LineHeight: 1, Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkText,
	}
	b.title.Name = nameCoursesTitle
	b.title.Heading = 1 // the title of the screen
	b.Add(b.title)
	for i := range b.chips {
		ch := tvkit.NewFilterChip(keyCoursesFilter+strconv.Itoa(i), s.chipText(i))
		ch.SetSelected(i == s.filter)
		ch.OnSelect = func() { s.setFilter(i) }
		b.chips[i] = ch
		b.Add(ch)
	}
	b.slide = tvkit.NewSlide(nil, s.newGrid())
	b.slide.Name = nameCoursesGrid
	b.Add(b.slide)
	b.Relayout()
	if s.anchor != nil {
		focusFrom(c, s.anchor, b.chips[0])
		s.Remove(s.anchor)
		s.anchor = nil
	}
	s.Relayout()
}

// chipText is "{label} · {count}" of filter i.
func (s *Courses) chipText(i int) string {
	return s.e.Fill("tv.courses.chipTpl", "label", s.e.T(filterLabels[i]), "n", strconv.Itoa(s.count(i)))
}

// count is how many courses filter i holds, from the counts of the answer.
func (s *Courses) count(i int) int {
	n := s.page.Counts
	switch i {
	case 0:
		return n.All
	case 1:
		return n.Regular
	case 2:
		return n.Special
	default:
		return n.Intro
	}
}

// inFilter tells whether a course of category cat shows under filter i.
func inFilter(i int, cat model.Category) bool {
	switch i {
	case 1:
		return cat == model.CategoryRegular
	case 2:
		return cat == model.CategorySpecial
	case 3:
		return cat == model.CategoryIntro
	default:
		return true
	}
}

// setFilter is `data-tact="tfilter"` (:741): the filter changes and the grid
// shows its courses from the top. The focus stays on the chip: the chip is the
// same widget, as renderTV finds it again by its key.
func (s *Courses) setFilter(i int) {
	if i == s.filter {
		return
	}
	s.filter = i
	for j, ch := range s.body.chips {
		ch.SetSelected(j == i)
	}
	s.body.Relayout()
	s.body.slide.SetContent(s.newGrid())
}

// newGrid makes the cells of the courses under the filter in force.
func (s *Courses) newGrid() *courseGrid {
	g := &courseGrid{}
	for _, course := range s.page.Items {
		if !inFilter(s.filter, course.Category) {
			continue
		}
		cell := s.newCell(len(g.cells), course)
		g.cells = append(g.cells, cell)
		g.Add(cell.tile, cell.caption, cell.meta)
	}

	return g
}

// newCell makes the cell of course number i: the tile `cg<i>`, the name with its
// quote and the category line.
func (s *Courses) newCell(i int, course model.Course) *courseCell {
	spec := poster.ForCourse(course)
	spec.Radius, spec.TitleSize = tvkit.PosterRadius, coursesPosterTS
	tile := tvkit.NewPosterTile(keyCoursesTile+strconv.Itoa(i), spec, coursesCellW, coursesPosterH)
	tile.Poster.SetCover(s.e.Cover, course.CoverURL) // the real cover once loaded (K14)
	id := course.ID
	tile.OnSelect = func() { openCourse(s.e, id) }
	tile.SetReading(course.Name, s.e.Fill("tv.home.lessonsTpl", "n", strconv.Itoa(course.LessonCount))) // the poster is decoration
	name := course.Name
	if course.Tagline != "" {
		name = s.e.Fill("tv.courses.captionTpl", "name", course.Name, "quote", course.Tagline)
	}
	capLabel := &kit.Label{Text: name, Size: coursesCaptionSize, Weight: 700, Ellipsis: true, Ink: theme.InkText}
	capLabel.Name = nameCoursesCaption
	meta := &kit.Label{
		Text: s.e.Fill("tv.courses.metaTpl", "cat", course.CategoryLabel, "n", strconv.Itoa(course.LessonCount)),
		Size: coursesMetaSize, Weight: 800, Spacing: coursesMetaSpacing, Upper: true,
	}
	meta.Name = nameCoursesMeta

	return &courseCell{tile: tile, caption: capLabel, meta: meta, cat: course.Category}
}

// layout places the cell at (x, y) and returns its height: the poster, 24, the
// name, 8, the category line.
func (cell *courseCell) layout(c *ui.Context, x, y float32) float32 {
	c.Layout(cell.tile, ui.Tight(geom.Sz(coursesCellW, coursesPosterH)))
	c.Place(cell.tile, geom.Pt(x, y))
	cy := y + coursesPosterH + coursesCaptionTop
	cs := c.Layout(cell.caption, ui.Constraints{Max: geom.Sz(coursesCellW, ui.Unbounded)})
	c.Place(cell.caption, geom.Pt(x, cy))
	cy += cs.H + coursesMetaTop
	cell.meta.Color = categoryInk(theme.From(c), cell.cat) // --info, --ok or #FF6B6E
	ms := c.Layout(cell.meta, ui.Constraints{Max: geom.Sz(coursesCellW, ui.Unbounded)})
	c.Place(cell.meta, geom.Pt(x, cy))

	return cy + ms.H - y
}
