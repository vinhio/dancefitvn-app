//go:build !tvos

package kitdemo

import (
	"fmt"
	"strings"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM04 is "Khoá học của tôi" (MS.courses, index.html:374-380): the title,
// the filter, the search field, the 15 course cards with their posters and
// chips, and the tab bar.
func pageM04() Page {
	return Page{ID: "M04", Title: "My courses", Build: func(d *Data) Screen { return screen(d.courses()) }}
}

// pagePosters is K-posters: the posters of 02 section 5 in the sizes the
// prototype draws them (local/tools/capture_kit.py POSTERS), all of Set 35.
func pagePosters() Page {
	return Page{ID: "K-posters", Title: "Posters", Build: func(d *Data) Screen { return screen(d.posters()) }}
}

// pageIcons is K-icons: every icon of the prototype's IC table at 24 px, white
// (capture_kit.py ICONS: inline-block, margin 4).
func pageIcons() Page {
	return Page{ID: "K-icons", Title: "Icons", Build: func(*Data) Screen { return screen(icons()) }}
}

// pageTV05 is the poster of the TV's course list (TV05, tvGrid): fifteen
// 540 x 304 posters with a 54 px title at their boxes of TV05.layout.json, on
// the 1920 x 1080 stage.
func pageTV05() Page {
	return Page{ID: "TV05", Title: "TV posters", TV: true, Build: func(d *Data) Screen { return screen(d.tvPosters()) }}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// courses lays out M04 at the boxes of M04.layout.json.
func (d *Data) courses() *canvas {
	cv := newCanvas()
	h1 := kit.H1(d.T("courses.title"))
	h1.Size = 30 // `style="font-size:30px"` (:375)
	cv.at(h1, 20, 70, 353)

	seg := kit.NewSegmented(d.T("filter.all")+" "+fmt.Sprint(d.Courses.Counts.All), d.T("filter.regular"), d.T("filter.special"), d.T("filter.intro"))
	seg.Keys = []string{d.T("filter.all"), d.T("filter.regular"), d.T("filter.special"), d.T("filter.intro")}
	seg.NamePrefix = "filter:"
	cv.at(seg, 20, 114, 353)

	search := kit.NewInput(icon.Search, d.T("courses.search"))
	search.Search = true
	cv.at(search, 20, 168, 353)

	shown := kit.Muted(d.T("courses.shown")+" "+fmt.Sprint(len(d.Courses.Items))+" "+d.T("courses.coursesUnit"), 12)
	shown.Name = "div.muted"
	cv.at(shown, 20, 226, 353)

	// a card (:377): 104 tall, 12 apart, the 80 poster at 12, the text at 126.
	for i, c := range d.Courses.Items {
		y := 251 + float32(i)*116
		cv.box(newCard("go.detail:"+ProtoID(c.ID)), 20, y, 353, 104)
		ps := poster.ForCourse(c)
		ps.Radius, ps.TitleSize = 12, 13
		cv.box(poster.New(ps, 80, 80), 32, y+12, 80, 80)
		cv.fit(&kit.Label{Text: c.Name, Size: 15, Weight: 700}, 126, y+12, 235)
		count := &kit.Label{Text: fmt.Sprint(c.LessonCount) + " " + d.T("lesson.unitLower"), Size: 12, Ink: theme.InkText3, Align: ui.TextAlignEnd}
		cv.at(count, 126, y+12, 235)
		quote := "—"
		if c.Tagline != "" {
			quote = "“" + c.Tagline + "”"
		}
		q := kit.Muted(quote, 13)
		q.Name = "div.muted"
		cv.at(q, 126, y+35, 235)
		cv.fit(kit.NewChip(chipKind(c.Category), strings.ToUpper(c.CategoryLabel)), 126, y+60, 66.83)
		if c.IsNew {
			cv.fit(kit.NewChip(kit.ChipY, d.T("course.new")), 197.33, y+60, 39.92)
		}
	}
	cv.pin(d.tabs(1, nil), 0, 768, 393)

	return cv
}

// posters lays out K-posters at its boxes.
func (d *Data) posters() *canvas {
	cv := newCanvas()
	c := d.Set35
	// w, h, r, ts of capture_kit.py POSTERS; the box y of K-posters.layout.json.
	for _, p := range []struct {
		x, y, w, h, r, ts float32
	}{
		{6, 6, 353, 232, 20, 0}, {6, 244, 156, 100, 16, 20}, {6, 350, 80, 80, 12, 13}, {6, 436, 72, 72, 12, 14},
		{6, 514, 44, 44, 10, 0}, {6, 564, 301, 301, 26, 44}, {6, 871, 393, 300, 0, 0},
	} {
		s := poster.ForCourse(c)
		s.Radius, s.TitleSize, s.NoTitle = p.r, p.ts, p.ts == 0
		cv.box(poster.New(s, p.w, p.h), p.x, p.y, p.w, p.h)
	}

	return cv
}

// icons lays out K-icons: the icons in the order of IC, twelve to a row, 24
// wide with 4 around.
func icons() *canvas {
	cv := newCanvas()
	for i, n := range icon.Names()[:icProto] {
		ic := kit.NewIcon(n, 24, theme.InkWhite)
		cv.box(ic, 4+float32(i%12)*32, 4+float32(i/12)*36, 24, 24)
	}

	return cv
}

// icProto is how many icons the prototype's IC table has: the rest of
// icon.Names are the filled ones (play, pause, the lesson triangle).
const icProto = 42

// tvPosters lays out TV05: the posters of the fifteen courses, three to a row.
func (d *Data) tvPosters() *canvas {
	cv := newCanvas()
	cv.design = TVSize
	for i, c := range d.Courses.Items {
		s := poster.ForCourse(c)
		s.Radius, s.TitleSize = 20, 54
		x := 90 + float32(i%3)*600
		y := 360 + float32(i/3)*436
		cv.box(poster.New(s, 540, 304), x, y, 540, 304)
	}

	return cv
}
