//go:build !tvos

package kitdemo

import (
	"fmt"
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM05 is the course detail (MS.detail, index.html:396-399): the cover, the
// two round buttons, the title block, the "now watching" card and the ten
// lesson rows, with the yellow button pinned at the bottom.
func pageM05() Page {
	return Page{ID: "M05", Title: "Course detail", Build: func(d *Data) Screen { return screen(d.detail()) }}
}

// pageToast is M-toast: the detail screen with the toast over it.
func pageToast() Page {
	return Page{ID: "M-toast", Title: "Toast", Build: func(d *Data) Screen {
		st := newStage(d.detail())
		st.toast.Show(d.T("toast.downloaded") + " " + format.No(d.currentLesson().No))

		return Screen{Root: st}
	}}
}

// pageM13 is the offline course (MS.offline, :417): the offline bar, the back
// button, the title and the lesson rows in offline mode.
func pageM13() Page {
	return Page{ID: "M13", Title: "Offline course", Build: func(d *Data) Screen { return screen(d.offline()) }}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// detail lays out M05 at the boxes of M05.layout.json.
func (d *Data) detail() *canvas {
	cv := newCanvas()
	set := d.Set35
	cur := d.currentLesson()

	hero := poster.ForCourse(set)
	hero.Radius, hero.NoTitle = 0, true
	cv.box(poster.New(hero, 393, 300), 0, 0, 393, 300)
	cv.fit(d.back("back", true), 20, 54, 40)
	more := kit.NewRoundButton(icon.More, true, d.T("a11y.more"))
	cv.fit(more, 333, 54, 40)

	eb := kit.Eyebrow(d.T("course.kind") + " " + strings.ToLower(set.CategoryLabel) + " · " + fmt.Sprint(lessonCount(set)) + " " + d.T("lesson.unitHeading"))
	eb.Ink, eb.Name = theme.InkPrimary, "div.eb.acc"
	cv.at(eb, 20, 214, 353)
	name := kit.Display(set.Name, 36)
	cv.at(name, 20, 236, 353)
	sub := kit.Muted("“"+set.Tagline+"” · "+fmt.Sprint(lessonCount(set))+" "+d.T("lesson.unitLower")+" · "+fmt.Sprint(minutes(set))+" "+d.T("lesson.minutes"), 13)
	sub.Name = "div.muted"
	cv.at(sub, 20, 278, 353)

	// the "now watching" card (:398): the title, the counts, the "Tải hết" chip.
	cv.box(newCard("div.card"), 20, 310, 353, 68)
	cv.at(&kit.Label{Text: d.T("detail.watching") + " " + format.No(cur.No), Size: 14, Weight: 700}, 86, 325, 176.56)
	counts := kit.Muted(fmt.Sprintf("%d/%d %s · %d/%d %s", watchedCount(set), lessonCount(set), d.T("lesson.watchedLower"), downloadedCount(set), lessonCount(set), d.T("detail.audioDone")), 12)
	counts.Name = "div.muted"
	cv.at(counts, 86, 348, 176.56)
	all := kit.NewChip(kit.ChipKindUnknown, d.T("detail.downloadAll"))
	all.Name, all.Icon, all.IconSize = "dlAll:"+ProtoID(set.ID), icon.Dl, 18
	all.Height, all.Radius, all.Size, all.Bg, all.Fg = 36, 18, 13, tokens.Dark.Surface2, tokens.Dark.Text
	cv.fit(all, 274.56, 326, 84.44)

	cv.at(d.lessonRows(false), 20, 435, 353)

	play := kit.NewButton(kit.ButtonP, d.T("detail.continueLesson")+" "+format.No(cur.No), icon.Play)
	play.Name, play.IconSize = "playVideo:"+lessonKey(set, cur)+"#3", 18
	cv.pin(play, 20, 760, 353)

	return cv
}

// currentLesson is the lesson of Set 35 under way, the first when none is.
func (d *Data) currentLesson() model.Lesson {
	for _, l := range d.Set35.Lessons {
		if current(l) {
			return l
		}
	}

	return d.Set35.Lessons[0]
}

// offline lays out M13 at the boxes of M13.layout.json.
func (d *Data) offline() *canvas {
	cv := newCanvas()
	set := d.Set35
	bar := kit.NewOfflineBar(d.T("offline.bar"))
	cv.at(bar, 0, 54, 393)
	cv.fit(d.back("back", false), 20, 104, 40)
	name := kit.Display(set.Name, 30)
	cv.at(name, 20, 160, 353)
	hint := kit.Muted(d.T("offline.hint"), 13)
	hint.Name = "p.muted"
	cv.at(hint, 20, 196, 353)
	cv.at(d.lessonRows(true), 20, 218, 353)

	return cv
}
