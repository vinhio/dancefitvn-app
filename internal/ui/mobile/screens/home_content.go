//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/views/home.go at 49de3c2 (homeStack: a
// column whose children each sit a number of DIPs under the one before, the
// margins of the sections; the hscroll row of cards; the body built once from the
// data and replaced when the data changes). The sections and numbers are MS.home's
// (index.html:360-369).

package screens

import (
	"strconv"
	"strings"
	"time"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of MS.home's column (index.html:360-369), in DIPs.
const (
	// homeSide is `#m .scroll{padding:0 20px}`: the column's side padding, which the
	// strip takes back (`margin:0 -20px`).
	homeSide float32 = 20
	// homeTop, homeHeaderBottom are the header's `margin:14px 0 18px`.
	homeTop, homeHeaderBottom float32 = 14, 18
	// homeStatsTop is the stats row's `margin-top:16px`; with no continue card
	// above it the header's 18 stands (the two margins collapse into the larger).
	homeStatsTop float32 = 16
	// homeSecTop, homeSecBottom are `.sec{margin:22px 0 12px}`.
	homeSecTop, homeSecBottom float32 = 22, 12
	// homeStripMax is `COURSE_ORDER.slice(0,6)` (index.html:365); homeStripGap the
	// strip's `gap:12px`.
	homeStripMax            = 6
	homeStripGap    float32 = 12
	homeEyebrowHour         = 12 // greeting by the hour: before 12 morning,
	homeAfternoon           = 18 // before 18 afternoon, else evening (Q-P3-15)
)

// The key of the lesson stat's label when there is no course to name.
const statLessonsKey = "lesson.unit"

// homeContent is what Home scrolls: the header, the continue card, the three
// numbers, the strip of courses and the update, each 14, 18, 16 ... DIPs under
// the one before, as MS.home's blocks are; the strip alone runs the screen's whole
// width.
type homeContent struct {
	ui.Base

	header   *homeHeader
	cont     *continueCard // nil: no lesson to continue (Q-P3-16)
	stats    *ui.Flex
	offNum   *kit.Label // the number of "Audio offline", which follows the downloads
	secMine  *secHead
	strip    ui.Scroll
	row      *ui.Flex
	cards    []*kit.CourseCard
	secNew   *secHead // nil: no update
	update   *updateCard
	stripTop float32 // where the strip is, for the tests
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newHomeContent returns the column for the answer of GET /me/home. now is the
// device's clock.
func newHomeContent(e *env.Env, h model.Home, now time.Time) *homeContent {
	k := &homeContent{}
	k.header = newHomeHeader(e, eyebrowText(e, now), h.GreetingName, func() {
		e.Act.Go(nav.Route{Screen: env.RouteAccount}, true)
	})
	k.Add(k.header)

	short := ""
	if c := h.Continue; c != nil {
		short = model.Course{Name: c.Course.Name}.ShortName()
		k.cont = newContinueCard(e, c, func() { e.Act.PlayVideo(c.Course.ID, c.Lesson.ID, c.Lesson.No) })
		k.Add(k.cont)
	}

	lessons := e.T(statLessonsKey)
	if short != "" {
		lessons = e.Fill("home.statLessonsTpl", "short", short)
	}
	courses, _ := newStat(itoa(h.Stats.CoursesTotal), e.T("home.stats.courses"), true)
	inCourse, _ := newStat(itoa(h.Stats.LessonsInCurrentCourse), lessons, false)
	offline, offNum := newStat(itoa(e.Downloads().DoneCount()), e.T("home.stats.offline"), false)
	k.offNum = offNum
	k.stats = ui.Row(statGap, courses, inCourse, offline)
	k.stats.Align = ui.CrossStretch
	k.Add(k.stats)

	k.secMine = newSecHead(e.T("home.mycourses"), e.T("home.seeAll"), "go.courses", func() {
		e.Act.Go(nav.Route{Screen: env.RouteCourses}, true)
	})
	k.Add(k.secMine)

	k.row = ui.Row(homeStripGap)
	k.row.Align = ui.CrossStart
	k.row.Padding = geom.Insets{Left: homeSide, Right: homeSide} // `padding:0 20px`
	for _, c := range h.MyCourses[:min(len(h.MyCourses), homeStripMax)] {
		k.cards = append(k.cards, k.newCard(e, c))
	}
	for _, cc := range k.cards {
		k.row.Add(cc)
	}
	k.strip.Axes, k.strip.HideBars = ui.ScrollHorizontal, true // `overflow-x:auto;scrollbar-width:none`
	k.strip.Add(k.row)
	k.Add(&k.strip)

	if len(h.Updates) > 0 {
		u := h.Updates[0].Course // `data-set="set36"`: the one update the prototype shows
		k.secNew = newSecHead(e.T("home.updates"), "", "", nil)
		k.update = newUpdateCard(e, u, func() { e.Act.Detail(u.ID) })
		k.Add(k.secNew, k.update)
	}

	return k
}

// newCard returns courseCardH for c: the poster, the name, the tagline in curly
// quotes, the category and the number of lessons; a tap opens the course.
func (k *homeContent) newCard(e *env.Env, c model.Course) *kit.CourseCard {
	cc := &kit.CourseCard{
		Poster:   CoursePoster(e, c),
		Name:     c.Name,
		Tag:      c.CategoryLabel + " · " + itoa(c.LessonCount) + " " + e.T("course.lessonsUpper"),
		Category: CardCategory(c.Category),
	}
	if c.Tagline != "" {
		cc.Quote = "“" + c.Tagline + "”"
	}
	cc.Node().Name = "go.detail:" + CourseKey(c.ID) // `data-go="detail" data-set`
	cc.OnPress = func() { e.Act.Detail(c.ID) }

	return cc
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// syncOffline shows how many lessons have their audio on the device now ("Audio
// offline", `S.dl` done), changing the label only when the number did.
func (k *homeContent) syncOffline(done int) {
	var buf [20]byte
	if s := strconv.AppendInt(buf[:0], int64(done), 10); string(s) != k.offNum.Text {
		k.offNum.SetText(string(s))
	}
}

// Strip returns the scroll of the courses, for the tests.
func (k *homeContent) Strip() *ui.Scroll { return &k.strip }

// Layout puts the blocks in a column 20 from the sides, the strip across the whole
// width.
func (k *homeContent) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	inner := max(w-2*homeSide, 0)
	col := ui.Constraints{Min: geom.Sz(inner, 0), Max: geom.Sz(inner, ui.Unbounded)}
	y := homeTop
	put := func(wd ui.Widget) float32 {
		s := c.Layout(wd, col)
		c.Place(wd, geom.Pt(homeSide, y))

		return s.H
	}
	y += put(k.header) + homeHeaderBottom
	if k.cont != nil {
		y += put(k.cont) + homeStatsTop
	}
	y += put(k.stats) + homeSecTop
	y += put(k.secMine) + homeSecBottom
	k.stripTop = y
	ss := c.Layout(&k.strip, ui.Loose(geom.Sz(w, ui.Unbounded)))
	c.Place(&k.strip, geom.Pt(0, y))
	y += ss.H
	if k.update != nil {
		y += homeSecTop
		y += put(k.secNew) + homeSecBottom
		y += put(k.update)
	}

	return cs.Constrain(geom.Sz(w, y)) // the page adds the 120 under it
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// eyebrowText is "{weekday}, {dd/MM} · {greeting}" for the clock now (Q-P3-15: before
// 12 morning, before 18 afternoon, else evening).
func eyebrowText(e *env.Env, now time.Time) string {
	greet := "home.evening"
	switch h := now.Hour(); {
	case h < homeEyebrowHour:
		greet = "home.morning"
	case h < homeAfternoon:
		greet = "home.afternoon"
	default: // evening, already in greet
	}

	return e.Fill("home.eyebrowTpl",
		"weekday", e.T("home.weekday."+strconv.Itoa(format.Weekday(now))),
		"date", format.DayMonth(now),
		"greeting", e.T(greet))
}

// itoa is strconv.Itoa.
func itoa(n int) string { return strconv.Itoa(n) }

// upper is strings.ToUpper, which writes Vietnamese capitals.
func upper(s string) string { return strings.ToUpper(s) }
