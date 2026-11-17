//go:build !tvos

package kitdemo

import (
	"fmt"
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM03 is the home screen (MS.home, index.html:360-368): the greeting, the
// continue card, the three numbers, the strip of courses, the update and the
// tab bar.
func pageM03() Page {
	return Page{ID: "M03", Title: "Home", Build: func(d *Data) Screen { return screen(d.home()) }}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// home lays out M03 at the boxes of M03.layout.json, in document order.
func (d *Data) home() *canvas {
	cv := newCanvas()
	cont := d.Home.Continue
	set := d.Set35

	cv.fit(kit.Eyebrow(d.T("home.eyebrow")), 20, 68, 229.78)
	cv.at(kit.Display(d.T("home.greeting")+" "+d.Home.GreetingName, 28), 20, 90, 229.78)
	av := kit.NewAvatarButton(44, tokens.Dark.Primary, d.T("tabs.account"))
	av.Name = "go.account"
	cv.fit(av, 329, 69, 44)

	// the continue card (continueCard, :357-358): the cover, the chip, the
	// title and the progress bar over it.
	hero := poster.ForCourse(set)
	hero.Radius, hero.NoTitle = 20, true
	cv.box(poster.New(hero, 353, 232), 20, 136, 353, 232)
	chip := kit.NewChip(kit.ChipY, d.T("home.continue"))
	chip.Weight, chip.Spacing = 800, 1
	cv.fit(chip, 36, 152, 108.38)
	title := kit.Display(cont.Lesson.Title, 26)
	cv.at(title, 36, 311, 257)
	prog := &kit.Progress{Track: color.RGBA(255, 255, 255, .3)}
	prog.SetValue(float32(cont.PositionSec) / float32(cont.Lesson.DurationSec))
	cv.at(prog, 36, 347, 257)

	// the three numbers (:361): `.card` flex:1 gap 8 padding 14.
	stats := [3]struct {
		n     int
		label string
		acc   bool
	}{
		{d.Home.Stats.CoursesTotal, d.T("home.stats.courses"), true},
		{d.Home.Stats.LessonsInCurrentCourse, d.T("lesson.unit") + " · " + shortName(set), false},
		{downloadedCount(set), d.T("home.stats.offline"), false},
	}
	// x of the cards: 20, 140.33, 260.67, each 112.33 wide (M03.layout.json)
	cardW := (353 - 2*8) / float32(3)
	for i, s := range stats {
		x := 20 + float32(i)*(cardW+8)
		cv.box(newCard("div.card"), x, 384, cardW, 75)
		num := kit.Display(fmt.Sprint(s.n), 26)
		if s.acc {
			num.Ink = theme.InkPrimary
			num.Name = "div.d.acc"
		}
		cv.at(num, x+14, 398, cardW-28)
		m := kit.Muted(s.label, 12)
		m.Name = "div.muted"
		cv.at(m, x+14, 430, cardW-28)
	}

	// the section head (`.sec`, :88-90).
	cv.fit(kit.SecTitle(d.T("home.mycourses")), 20, 481, 133.19)
	more := kit.SecLink(d.T("home.seeAll"))
	more.Name = "go.courses"
	cv.fit(more, 306.39, 484, 66.61)

	// the strip (courseCardH, :371): six cards 156 wide, 12 apart.
	for i, c := range d.Home.MyCourses[:6] {
		cv.at(d.courseCard(c), 20+float32(i)*168, 512, 156)
	}

	cv.fit(kit.SecTitle(d.T("home.updates")), 20, 717, 102.11)
	// the update card (:367): a 72 poster, the name, the line, the chip.
	if len(d.Home.Updates) > 0 {
		u := d.Home.Updates[0].Course
		cv.box(newCard("go.detail:"+ProtoID(u.ID)), 20, 747, 353, 96)
		ps := poster.ForCourse(u)
		ps.Radius, ps.TitleSize = 12, 14
		cv.box(poster.New(ps, 72, 72), 32, 759, 72, 72)
		cv.fit(&kit.Label{Text: u.Name, Size: 15, Weight: 700}, 116, 759, 245)
		sub := kit.Muted(u.Tagline+" · "+d.T("home.notEnrolled"), 12)
		sub.Name = "div.muted"
		cv.at(sub, 116, 783, 245)
		chip := kit.NewChip(chipKind(u.Category), strings.ToUpper(u.CategoryLabel)+" · "+fmt.Sprint(u.LessonCount)+" "+d.T("course.lessonsUpper"))
		cv.fit(chip, 116, 807, 109.05)
	}

	// go.account and go.courses are taken by the avatar and the link: the bar's
	// tabs are the second of each.
	cv.pin(d.tabs(0, map[int]string{1: "go.courses#2", 3: "go.account#2"}), 0, 768, 393)

	return cv
}

// courseCard returns courseCardH for c: the poster, the name, the tagline in
// curly quotes, the category and the number of lessons.
func (d *Data) courseCard(c model.Course) *kit.CourseCard {
	cc := &kit.CourseCard{
		Poster:   coursePoster(c),
		Name:     c.Name,
		Quote:    quoted(c.Tagline),
		Tag:      c.CategoryLabel + " · " + fmt.Sprint(c.LessonCount) + " " + d.T("course.lessonsUpper"),
		Category: category(c.Category),
	}
	cc.Node().Name = "go.detail:" + ProtoID(c.ID) // Name is the course's name here

	return cc
}

// quoted puts a tagline in curly quotes (`'“'+c.quote+'”'`, :371), a blank
// line when there is none.
func quoted(tagline string) string {
	if tagline == "" {
		return ""
	}

	return "“" + tagline + "”"
}
