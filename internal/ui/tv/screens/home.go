//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/detail.go (the read after
// Mounted, one at a time, dropped when the screen is left) and from
// jplayer-tv/internal/ui/views/home.go at 781bee5 (a hero over its art with the
// shelf under it, laid out from the data when it arrives); the numbers are
// TS.home's (index.html:662-670).

package screens

import (
	"context"
	"strconv"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.home (index.html:662-670), in DIPs.
const (
	// homeHeroX, homeHeroW, homeHeroH are the hero art, `poster(cid,1360,780,
	// {r:0,title:false})` at `left:560px;top:0`.
	homeHeroX, homeHeroW, homeHeroH float32 = 560, 1360, 780
	// homeShadeStop1..3 are the stops of the side shade, `linear-gradient(90deg,
	// var(--bg) 30%, rgba(20,21,24,.85) 45%, transparent 75%)`; homeShadeMid is the
	// alpha of the middle one.
	homeShadeStop1, homeShadeStop2, homeShadeStop3, homeShadeMid float32 = .30, .45, .75, .85
	// homeFloorTop, homeFloorStop are the bottom shade, `inset:480px 0 0` with
	// `linear-gradient(transparent, var(--bg) 60%)`.
	homeFloorTop, homeFloorStop float32 = 480, .60
	// homeBlockX, homeBlockY are `left:90px;top:240px` of the text block.
	homeBlockX, homeBlockY float32 = 90, 240
	// homeChipH, homeChipR, homeChipPad, homeChipSize, homeChipSpacing are the
	// chip's `height:44px;border-radius:22px;padding:0 20px;font-size:22px;
	// letter-spacing:1px`.
	homeChipH, homeChipR, homeChipPad, homeChipSize, homeChipSpacing float32 = 44, 22, 20, 22, 1
	// homeMetaTop, homeMetaSize, homeMetaSpacing are `font-size:28px;
	// font-weight:700;letter-spacing:2px;margin-top:40px`.
	homeMetaTop, homeMetaSize, homeMetaSpacing float32 = 40, 28, 2
	// homeTitleTop, homeTitleSize are `.d` at `font-size:110px;font-weight:800;
	// margin-top:18px`.
	homeTitleTop, homeTitleSize float32 = 18, 110
	// homeSubTop, homeSubBottom, homeSubSize are `p.muted` at `font-size:30px;
	// margin:22px 0 16px`.
	homeSubTop, homeSubBottom, homeSubSize float32 = 22, 16, 30
	// homeProgW, homeProgH are `.prog` at `width:560px;height:8px`.
	homeProgW, homeProgH float32 = 560, 8
	// homeRowTop, homeRowGap are `display:flex;gap:30px;margin-top:44px`.
	homeRowTop, homeRowGap float32 = 44, 30
	// homePlayIcon is the PLAY span of `h1`, `width:30px`.
	homePlayIcon float32 = 30
	// homeShelfX, homeShelfY, homeShelfSize are the heading of the shelf, `.d` at
	// `left:90px;top:790px` with a span of `font-size:36px`.
	homeShelfX, homeShelfY, homeShelfSize float32 = 90, 790, 36
	// homeTilesY, homeTileW, homeTileH, homeTileTS, homeTileGap are the shelf,
	// `left:90px;top:840px;gap:40px` of `tposter(id,420,236,44,...)`.
	homeTilesY, homeTileW, homeTileH, homeTileTS, homeTileGap float32 = 840, 420, 236, 44, 40
	// homeTiles is how many courses the shelf shows, `COURSE_ORDER.slice(0,4)`.
	homeTiles = 4
	// homeDisplaySpacing is `.d{letter-spacing:.6px}` (:45).
	homeDisplaySpacing float32 = .6
)

// The keys of the controls: `data-fid` of TS.home.
const (
	keyHomeResume  = "h1"
	keyHomeLessons = "h2"
	keyHomeTile    = "hc"
)

// Names layout.json knows the parts by: `div.d` of the title and of the shelf's
// heading and `p.muted` are the capture's; the meta line has no class.
const (
	nameHomeTitle = "div.d"
	nameHomeSub   = "p.muted"
	nameHomeMeta  = "tv.home.meta"
	nameHomeShade = "tv.home.shade"
	nameHomeProg  = "div.prog"
)

// Home is TS.home (TV04): the course the user left off, large over its art with
// the lesson, how far they were and the two buttons `h1` (Tiếp tục xem, plays it)
// and `h2` (the course's lessons, TV06), and under it the first four of "Khoá học
// của tôi", `hc0`..`hc3`, each to TV06 of its course. It is the first screen after
// the TV is paired; `h1` has the focus first. While the data is read it shows the
// nav bar alone (K12); a read that fails shows TV11 (K28).
//
// With no lesson to continue (a new account) the hero is the first lesson of the
// first course and `h1` reads "Xem bài 01" without the "đang xem" part (Q-P7-9).
type Home struct {
	ui.Base

	e *env.Env

	body   *homeBody
	nav    *tvkit.Nav
	anchor *waitAnchor

	hero   homeHero      // what the hero shows once the data came
	course *model.Course // the hero's course with its lessons, nil until read
	have   bool          // the data came

	rd, rc, avatar reader // the reads of the home, of the hero's course and of the user's picture
	playAfter      bool   // play when the course arrives
}

// homeHero is the hero's lesson: what `S.pt.l` and `S.pt.pos` are in TS.home.
type homeHero struct {
	course model.CourseRef
	lesson model.LessonRef
	pos    int  // seconds into the lesson
	resume bool // a lesson to continue, else the first one of the first course
}

// homeBody is the screen's content under the nav bar: the hero, its shades, the
// text block, the two buttons and the shelf. A focused control paints last (F-12).
type homeBody struct {
	ui.Base

	hero  *poster.Poster
	shade *homeShade

	chip  *kit.Chip
	meta  *kit.Label
	title *kit.Label
	sub   *kit.Label
	prog  *kit.Progress
	play  *tvkit.Button
	list  *tvkit.Button

	shelf *kit.Label
	tiles []*tvkit.PosterTile
}

// homeShade is the two gradients over the hero: the side shade across the screen
// and the floor shade from y 480 down. The paints are made when the size changes,
// not per frame.
type homeShade struct {
	ui.Base

	side, floor scene.Paint
	size        geom.Size
}

var (
	_ ui.Mounter   = (*Home)(nil)
	_ ui.Unmounter = (*Home)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewHome returns TV04, with the nav bar and the waiting state until the home is
// read.
func NewHome(e *env.Env) *Home {
	s := &Home{e: e, body: &homeBody{}}
	s.nav = newNavBar(e, tabHome)
	s.anchor = newWaitAnchor(e, keyHomeResume)
	// The nav bar is the top of the page, so a screen reader reads it first; it is
	// painted last, over the hero (Paint).
	s.Add(s.nav, s.body, s.anchor)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`h1`, `h2`, `hc0`..), nil when there is
// none yet.
func (s *Home) Item(key string) *tvkit.Item {
	b := s.body
	switch key {
	case keyHomeResume:
		if b.play != nil {
			return &b.play.Item
		}
	case keyHomeLessons:
		if b.list != nil {
			return &b.list.Item
		}
	default:
		for _, t := range b.tiles {
			if t.Key == key {
				return &t.Item
			}
		}
	}

	return nil
}

// Nav returns the nav bar.
func (s *Home) Nav() *tvkit.Nav { return s.nav }

// Loaded tells whether the content is on screen.
func (s *Home) Loaded() bool { return s.have }

// Mounted starts reading with the next turn: a view that is being mounted has no
// host yet, and ui.Go would run the read on the UI thread then.
func (s *Home) Mounted(c *ui.Context) {
	if !s.avatar.Done() && !s.avatar.Busy() {
		c.AfterFunc(0, s.loadAvatar)
	}
	if !s.have && !s.rd.Busy() {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted ends the reads in flight.
func (s *Home) Unmounted(*ui.Context) {
	s.rd.Stop()
	s.rc.Stop()
	s.avatar.Stop()
	s.playAfter = false
}

// Paint draws the content, then the nav bar over it: the hero's art reaches up to
// the top of the screen, under the bar.
func (s *Home) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintOver(c, p, &s.Base, s.nav) }

// Layout fills the room with the content and the nav bar over it.
func (s *Home) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
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

// Layout places the hero and its shades, flows the text block as CSS does and
// puts the shelf under it.
func (b *homeBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if b.hero != nil {
		b.layoutHero(c, size)
	}
	if b.shelf != nil {
		b.layoutShelf(c)
	}

	return size
}

// layoutHero places the hero and its shades and flows the text block as CSS does.
func (b *homeBody) layoutHero(c *ui.Context, size geom.Size) {
	c.Layout(b.hero, ui.Tight(geom.Sz(homeHeroW, homeHeroH)))
	c.Place(b.hero, geom.Pt(homeHeroX, 0))
	c.Layout(b.shade, ui.Tight(size))
	c.Place(b.shade, geom.Point{})

	// the block is absolutely positioned: it shrinks to its widest line, and the
	// lines that are blocks fill that width
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	cw := c.Layout(b.chip, free).W
	mw := c.Layout(b.meta, free).W
	tw := c.Layout(b.title, free).W
	sw := c.Layout(b.sub, free).W
	pw := c.Layout(b.play, free).W
	lw := c.Layout(b.list, free).W
	width := max(cw, mw, tw, sw, homeProgW, pw+homeRowGap+lw)
	width = min(width, size.W-homeBlockX)
	block := ui.Constraints{Min: geom.Sz(width, 0), Max: geom.Sz(width, ui.Unbounded)}

	y := homeBlockY
	cs2 := c.Layout(b.chip, free)
	c.Place(b.chip, geom.Pt(homeBlockX, y))
	y += cs2.H + homeMetaTop
	ms := c.Layout(b.meta, block)
	c.Place(b.meta, geom.Pt(homeBlockX, y))
	y += ms.H + homeTitleTop
	ts := c.Layout(b.title, block)
	c.Place(b.title, geom.Pt(homeBlockX, y))
	y += ts.H + homeSubTop
	ss := c.Layout(b.sub, block)
	c.Place(b.sub, geom.Pt(homeBlockX, y))
	y += ss.H + homeSubBottom
	c.Layout(b.prog, ui.Tight(geom.Sz(homeProgW, homeProgH)))
	c.Place(b.prog, geom.Pt(homeBlockX, y))
	y += homeProgH + homeRowTop
	ps := c.Layout(b.play, free)
	c.Place(b.play, geom.Pt(homeBlockX, y))
	ls := c.Layout(b.list, free)
	c.Place(b.list, geom.Pt(homeBlockX+ps.W+homeRowGap, y+(ps.H-ls.H)/2))
}

// Paint draws the content, the focused control last.
func (b *homeBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// Layout is the whole room.
func (h *homeShade) Layout(_ *ui.Context, cs ui.Constraints) geom.Size {
	return cs.Constrain(cs.Max)
}

// Paint draws the side shade over the room and the floor shade from y 480.
func (h *homeShade) Paint(c *ui.Context, p *ui.Painter) {
	r := h.Bounds()
	if h.size != r.Size() {
		h.size = r.Size()
		bg := theme.From(c).T.Bg
		clear := bg.WithAlpha(0)
		// linear-gradient(90deg, var(--bg) 30%, rgba(20,21,24,.85) 45%, transparent 75%)
		h.side = scene.Linear(geom.Point{}, geom.Pt(r.W, 0),
			scene.Stop{Color: bg, Offset: homeShadeStop1},
			scene.Stop{Color: bg.WithAlpha(homeShadeMid), Offset: homeShadeStop2},
			scene.Stop{Color: clear, Offset: homeShadeStop3})
		// linear-gradient(transparent, var(--bg) 60%) over inset:480px 0 0
		fh := max(r.H-homeFloorTop, 0)
		h.floor = scene.Linear(geom.Pt(0, homeFloorTop), geom.Pt(0, homeFloorTop+fh),
			scene.Stop{Color: clear, Offset: 0},
			scene.Stop{Color: bg, Offset: homeFloorStop})
	}
	p.Fill(r, h.side, 0)
	p.Fill(geom.XYWH(0, homeFloorTop, r.W, max(r.H-homeFloorTop, 0)), h.floor, 0)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads GET /me/home; with no lesson to continue it reads the first
// course too, whose first lesson is the hero (Q-P7-9).
func (s *Home) load() {
	if s.have {
		return
	}
	type result struct {
		home   model.Home
		course *model.Course
	}
	read(&s.rd, s, func(ctx context.Context) (result, error) {
		h, err := s.e.Home(ctx)
		if err != nil {
			return result{}, err
		}
		r := result{home: h}
		if h.Continue == nil && len(h.MyCourses) > 0 {
			c, err := s.e.Course(ctx, h.MyCourses[0].ID)
			if err != nil {
				return result{}, err
			}
			r.course = &c
		}

		return r, nil
	}, func(c *ui.Context, r result, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.show(c, r.home, r.course)
	})
}

// loadAvatar reads the picture of the nav bar.
func (s *Home) loadAvatar() { loadAvatar(s, &s.avatar, s.e, s.nav) }

// show builds the content from the home. first is the first course, read when
// there is nothing to continue.
func (s *Home) show(c *ui.Context, h model.Home, first *model.Course) {
	s.hero, s.course = homeHero{}, nil
	switch {
	case h.Continue != nil:
		s.hero = homeHero{course: h.Continue.Course, lesson: h.Continue.Lesson, pos: h.Continue.PositionSec, resume: true}
	case first != nil && len(first.Lessons) > 0:
		l := first.Lessons[0]
		s.hero = homeHero{
			course: model.CourseRef{ID: first.ID, Name: first.Name, Tagline: first.Tagline, CoverURL: first.CoverURL, LessonCount: first.LessonCount},
			lesson: model.LessonRef{ID: l.ID, No: l.No, Title: l.Title, Tag: l.Tag, DurationSec: l.DurationSec},
		}
		s.course = first
		s.e.RememberCourse(*first)
	}
	s.have = true
	s.body.fill(s.e, s.hero, h, s)
	s.body.Relayout()
	if s.anchor != nil {
		focusFrom(c, s.anchor, s.firstControl())
		s.Remove(s.anchor) // the control it stood for is there now
		s.anchor = nil
	}
	s.Relayout()
	if s.course == nil && s.hero.lesson.ID != "" {
		s.fetch(false) // the lessons the player walks, read while the user looks
	}
}

// firstControl is the control the default focus rule finds first, the first that
// is not in the nav bar (renderTV, index.html:723): `h1`, else the first tile of a
// home with no lesson to show, else the first nav item, `fs[0]`.
func (s *Home) firstControl() ui.Widget {
	switch {
	case s.body.play != nil:
		return s.body.play
	case len(s.body.tiles) > 0:
		return s.body.tiles[0]
	default:
		return s.nav.Item(0)
	}
}

// fill makes the widgets of the content for the hero and the shelf.
func (b *homeBody) fill(e *env.Env, hero homeHero, h model.Home, s *Home) {
	b.Clear()
	b.tiles = b.tiles[:0]
	b.hero, b.play, b.list = nil, nil, nil
	n := h.Stats.CoursesTotal
	if n == 0 {
		n = len(h.MyCourses)
	}
	if hero.lesson.ID != "" {
		b.fillHero(e, hero, s)
	}
	b.shelf = &kit.Label{
		Text: e.Fill("tv.home.myCoursesTpl", "n", strconv.Itoa(n)), Role: theme.RoleDisplay, Size: homeShelfSize,
		Weight: 700, LineHeight: 1, Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkText,
	}
	b.shelf.Name = nameHomeTitle
	b.shelf.Heading = 2 // the title of a section
	b.Add(b.shelf)
	for i, course := range h.MyCourses[:min(len(h.MyCourses), homeTiles)] {
		spec := poster.ForCourse(course)
		spec.Radius, spec.TitleSize = tvkit.PosterRadius, homeTileTS
		t := tvkit.NewPosterTile(keyHomeTile+strconv.Itoa(i), spec, homeTileW, homeTileH)
		t.Poster.SetCover(e.Cover, course.CoverURL) // the real cover once loaded (K14)
		id := course.ID
		t.OnSelect = func() { openCourse(e, id) }
		t.SetReading(course.Name, e.Fill("tv.home.lessonsTpl", "n", strconv.Itoa(course.LessonCount))) // the poster is decoration
		b.tiles = append(b.tiles, t)
		b.Add(t)
	}
}

// fillHero makes the hero art, the shades, the text block and the two buttons.
func (b *homeBody) fillHero(e *env.Env, hero homeHero, s *Home) {
	b.hero = poster.New(poster.Spec{ID: hero.course.ID, Radius: 0, NoTitle: true}, homeHeroW, homeHeroH)
	b.hero.SetCover(e.Cover, hero.course.CoverURL) // the real cover once loaded (K14)
	b.shade = &homeShade{}
	b.shade.Name = nameHomeShade
	b.chip = kit.NewChip(kit.ChipY, e.T("home.continue"))
	b.chip.Height, b.chip.Radius, b.chip.PadX, b.chip.Size, b.chip.Spacing = homeChipH, homeChipR, homeChipPad, homeChipSize, homeChipSpacing
	b.meta = &kit.Label{
		Text: e.Fill("tv.home.metaTpl", "course", hero.course.Name, "nn", pad2(hero.lesson.No),
			"n", strconv.Itoa(hero.course.LessonCount), "tag", hero.lesson.Tag),
		Size: homeMetaSize, Weight: 700, Spacing: homeMetaSpacing, Upper: true, Ink: theme.InkPrimary,
	}
	b.meta.Name = nameHomeMeta
	b.title = &kit.Label{
		Text: hero.lesson.Title, Role: theme.RoleDisplay, Size: homeTitleSize, Weight: 800,
		LineHeight: 1, Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkText,
	}
	b.title.Name = nameHomeTitle
	b.title.Heading = 1 // the lesson to continue, the title of the screen
	b.sub = &kit.Label{Text: heroSub(e, hero), Size: homeSubSize, Ink: theme.InkText2}
	b.sub.Name = nameHomeSub
	b.prog = &kit.Progress{Height: homeProgH, Label: e.T("a11y.position")} // how far the lesson was watched
	b.prog.Name = nameHomeProg
	if d := hero.lesson.DurationSec; d > 0 {
		b.prog.SetValue(float32(hero.pos) / float32(d))
		b.prog.Spoken = format.Clock(float64(hero.pos)) + " / " + format.Clock(float64(d)) // "01:12 / 03:40"
	}
	label := e.T("tv.home.resume")
	if !hero.resume {
		label = e.T("tv.detail.first")
	}
	b.play = tvkit.NewButton(keyHomeResume, label, icon.Play)
	b.play.IconSize, b.play.PlainIcon = homePlayIcon, true
	b.play.OnSelect = s.playContinue
	b.list = tvkit.NewButton(keyHomeLessons, e.Fill("tv.home.lessonsTpl", "n", strconv.Itoa(hero.course.LessonCount)), icon.List)
	id := hero.course.ID
	b.list.OnSelect = func() { openCourse(e, id) }
	b.Add(b.hero, b.shade, b.chip, b.meta, b.title, b.sub, b.prog, b.play, b.list)
}

// heroSub is the muted line: the quote, the length and how far the user is.
func heroSub(e *env.Env, hero homeHero) string {
	dur := format.Clock(float64(hero.lesson.DurationSec))
	q := hero.course.Tagline
	switch {
	case hero.resume && q != "":
		return e.Fill("tv.home.subTpl", "quote", q, "dur", dur, "pos", format.Clock(float64(hero.pos)))
	case hero.resume:
		return e.Fill("tv.home.subNoQuoteTpl", "dur", dur, "pos", format.Clock(float64(hero.pos)))
	case q != "":
		return e.Fill("tv.home.subNewTpl", "quote", q, "dur", dur)
	default:
		return dur
	}
}

// layoutShelf places the shelf's heading and the four tiles.
func (b *homeBody) layoutShelf(c *ui.Context) {
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	c.Layout(b.shelf, free)
	c.Place(b.shelf, geom.Pt(homeShelfX, homeShelfY))
	x := homeShelfX
	for _, t := range b.tiles {
		ts := c.Layout(t, ui.Tight(geom.Sz(homeTileW, homeTileH)))
		c.Place(t, geom.Pt(x, homeTilesY))
		x += ts.W + homeTileGap
	}
}

// openCourse is `data-tgo="detail" data-set=<id>` (index.html:665, :667): TV06 of
// the course over this screen.
func openCourse(e *env.Env, id string) {
	if e.Act != nil {
		e.Act.Go(nav.Route{Screen: env.RouteDetail, Params: id}, false)
	}
}

// playContinue is `data-tact="play"` of `h1` (`playLes(key(S.pt.l))`): the lesson
// plays and TV07 opens. The player walks the lessons of the course, so they are
// read first when they have not come yet.
func (s *Home) playContinue() {
	if s.course != nil {
		s.playHero()

		return
	}
	s.fetch(true)
}

// playHero plays the hero's lesson from where the user left it.
func (s *Home) playHero() {
	list := env.PlayerLessons(*s.course)
	i := env.LessonIndex(list, s.hero.lesson.ID)
	if i < 0 {
		return
	}
	l := list[i]
	if s.hero.resume {
		l.ResumeSec = s.hero.pos
	}
	playLesson(s.e, l, list)
}

// fetch reads the hero's course, to play it when it arrives if play is set; one
// read at a time.
func (s *Home) fetch(play bool) {
	s.playAfter = s.playAfter || play
	if s.rc.Busy() || s.hero.course.ID == "" {
		return
	}
	id := s.hero.course.ID
	started := read(&s.rc, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, id)
	}, func(_ *ui.Context, c model.Course, err error) {
		want := s.playAfter
		s.playAfter = false
		if err != nil {
			if want { // a prefetch that failed is not told; a Select that waits is
				failed(s.e, err)
			}

			return
		}
		s.course = &c
		s.e.RememberCourse(c)
		if want {
			s.playHero()
		}
	})
	if !started { // outside a view nothing runs, and nothing is waiting for the play
		s.playAfter = false
	}
}
