//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/done.go (the end-of-lesson
// countdown: a timer made once, re-armed each second, stopped when the screen is
// left; the watched count and the next lesson read from the course). The screen is
// TS.upnext (index.html:700-705) and its 1 s interval (:788-790).

package screens

import (
	"context"
	"strconv"
	"time"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/poster"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.upnext (index.html:700-705), in DIPs.
const (
	// upNextBlockX, upNextBlockY are the left block, `left:90px;top:200px`.
	upNextBlockX, upNextBlockY float32 = 90, 200
	// upNextChipH, upNextChipR, upNextChipPad, upNextChipSize, upNextChipIcon,
	// upNextChipStroke are the chip's `height:48px;border-radius:24px;font-size:24px;
	// padding:0 22px` and `ic('check',26,2.6)`.
	upNextChipH, upNextChipR, upNextChipPad, upNextChipSize, upNextChipIcon, upNextChipStroke float32 = 48, 24, 22, 24, 26, 2.6
	// upNextCheerTop, upNextCheerSize are `.d` at `font-size:72px;font-weight:800;
	// margin-top:40px`.
	upNextCheerTop, upNextCheerSize float32 = 40, 72
	// upNextSubTop, upNextSubBottom, upNextSubSize are `p.muted` at `font-size:30px;
	// margin:20px 0 70px`.
	upNextSubTop, upNextSubBottom, upNextSubSize float32 = 20, 70, 30
	// upNextColGap is the buttons' `gap:24px`.
	upNextColGap float32 = 24
	// upNextPlayIcon is the PLAY span of `u1`, `width:30px`.
	upNextPlayIcon float32 = 30
	// upNextCardX, upNextCardY, upNextCardW, upNextCardH, upNextCardR are the next
	// lesson's picture, `poster(l.cid,690,388,{r:22,title:false})` at `left:1100px;
	// top:200px`.
	upNextCardX, upNextCardY, upNextCardW, upNextCardH, upNextCardR float32 = 1100, 200, 690, 388, 22
	// upNextDiscInset is the disc's `right:30px;top:30px`.
	upNextDiscInset float32 = 30
	// upNextEyebrowTop, upNextEyebrowSize are `.eb` at `font-size:24px;margin-top:40px`.
	upNextEyebrowTop, upNextEyebrowSize float32 = 40, 24
	// upNextNextTop, upNextNextSize are `font-size:44px;font-weight:800;margin-top:16px`.
	upNextNextTop, upNextNextSize float32 = 16, 44
	// upNextMetaTop, upNextMetaSize are `p.muted` at `font-size:28px;margin-top:12px`.
	upNextMetaTop, upNextMetaSize float32 = 12, 28
	// upNextCount is the seconds the countdown starts at, `cdT=5`; upNextTick is the
	// interval, `setInterval(...,1000)`.
	upNextCount = tvkit.CountdownMax
	upNextTick  = time.Second
)

// The keys of the controls: `data-fid` of TS.upnext.
const (
	keyUpNextPlay   = "u1"
	keyUpNextCourse = "u2"
)

// Names layout.json knows the parts by: the chip, `div.d`, the eyebrow and the
// `p.muted` are the capture's; the bold line of the next lesson has no class.
const (
	nameUpNextCheer = "div.d"
	nameUpNextSub   = "p.muted"
	nameUpNextNext  = "tv.upnext.next"
	nameUpNextMeta  = "p.muted"
)

// UpNext is TS.upnext (TV10), the screen a video that played to its end gives way
// to: the finished lesson, "Tuyệt vời, {first}!" with how many lessons of the
// course are watched, `u1` (Xem ngay) and `u2` (Về khoá học), and, when there is a
// next lesson, its picture with the seconds left before it plays, counting 5 to 0
// once a second. At 0 the next lesson plays if "Tự phát bài tiếp theo" is on and TV07
// takes this screen's place, with the focus on `p1`; with it off the count reaches
// 0 and nothing plays (Q-P4.5-1). A course on its last lesson has no next card, no
// `u1` and no countdown, and `u2` has the focus. Leaving the screen stops the count.
type UpNext struct {
	ui.Base

	e  *env.Env
	pb env.Playback

	body   *upNextBody
	anchor *waitAnchor

	list   []player.Lesson
	next   int // the index of the next lesson in list, -1 for none
	left   int // the seconds shown
	ctx    *ui.Context
	timer  ui.Timer
	tickFn func()
	rd     reader // the read of the course
}

// upNextBody is the content: the left block and the card of the next lesson.
type upNextBody struct {
	ui.Base

	chip   *kit.Chip
	cheer  *kit.Label
	sub    *kit.Label
	play   *tvkit.Button // nil on the last lesson
	course *tvkit.Button

	card   *poster.Poster // the next lesson's picture; nil on the last lesson
	disc   *tvkit.Countdown
	eyebrw *kit.Label
	title  *kit.Label
	meta   *kit.Label
}

var (
	_ ui.Mounter   = (*UpNext)(nil)
	_ ui.Unmounter = (*UpNext)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewUpNext returns TV10 for the lesson the player holds. Without a player it is a
// bare screen with `u1`, so that Menu still goes back; with a course that was not
// read yet it waits for it.
func NewUpNext(e *env.Env) ui.Widget {
	pb := e.Playback()
	if pb == nil {
		return newBareScreen(e, keyUpNextPlay)
	}
	s := &UpNext{e: e, pb: pb, body: &upNextBody{}, next: -1, left: e.UpNextLeft()}
	s.tickFn = s.tick
	s.Add(s.body)
	if c, ok := e.CachedCourse(pb.Lesson().CourseID); ok {
		s.list = env.PlayerLessons(c)
		s.build()
	} else {
		s.anchor = newWaitAnchor(e, keyUpNextPlay)
		s.Add(s.anchor)
	}

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`u1`, `u2`), nil when there is none.
func (s *UpNext) Item(key string) *tvkit.Button {
	switch {
	case key == keyUpNextPlay && s.body.play != nil:
		return s.body.play
	case key == keyUpNextCourse && s.body.course != nil:
		return s.body.course
	default:
		return nil
	}
}

// Seconds returns the seconds the countdown shows.
func (s *UpNext) Seconds() int { return s.left }

// Counting tells whether the countdown is running.
func (s *UpNext) Counting() bool { return s.timer.Pending() }

// Restart runs the countdown again from 5, which a countdown that reached 0 does
// not do by itself.
func (s *UpNext) Restart() {
	s.timer.Stop()
	s.timer = ui.Timer{}
	s.left = upNextCount
	s.e.SetUpNextLeft(s.left)
	if s.body.disc != nil {
		s.body.disc.Set(s.left)
	}
	s.arm()
}

// NextLesson returns the next lesson, false on the last.
func (s *UpNext) NextLesson() (player.Lesson, bool) {
	if s.next < 0 || s.next >= len(s.list) {
		return player.Lesson{}, false
	}

	return s.list[s.next], true
}

// Mounted starts the countdown, or the read of the course it waits for.
func (s *UpNext) Mounted(c *ui.Context) {
	s.ctx = c
	if s.anchor != nil && !s.rd.Busy() {
		c.AfterFunc(0, s.load)

		return
	}
	s.arm()
}

// Unmounted stops the countdown and the read.
func (s *UpNext) Unmounted(*ui.Context) {
	s.e.SetUpNextLeft(s.left) // shown again, the count goes on from here (`cdT`)
	s.timer.Stop()
	s.timer = ui.Timer{}
	s.rd.Stop()
}

// Layout fills the room and places the left block and the card.
func (s *UpNext) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(s.body, ui.Tight(size))
	c.Place(s.body, geom.Point{})
	if s.anchor != nil {
		c.Layout(s.anchor, ui.Loose(geom.Sz(1, 1)))
		c.Place(s.anchor, geom.Pt(size.W/2, size.H/2))
	}

	return size
}

// Layout flows the left block as CSS does, the block as wide as its widest line,
// and puts the card and its texts at the right.
func (b *upNextBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if b.chip == nil {
		return size
	}
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	// the block is absolutely positioned: it shrinks to its widest line
	width := max(c.Layout(b.chip, free).W, c.Layout(b.cheer, free).W, c.Layout(b.sub, free).W)
	block := ui.Constraints{Min: geom.Sz(width, 0), Max: geom.Sz(width, ui.Unbounded)}
	y := upNextBlockY
	cs1 := c.Layout(b.chip, free)
	c.Place(b.chip, geom.Pt(upNextBlockX, y))
	y += cs1.H + upNextCheerTop
	hs := c.Layout(b.cheer, block)
	c.Place(b.cheer, geom.Pt(upNextBlockX, y))
	y += hs.H + upNextSubTop
	ss := c.Layout(b.sub, block)
	c.Place(b.sub, geom.Pt(upNextBlockX, y))
	y += ss.H + upNextSubBottom
	if b.play != nil {
		ps := c.Layout(b.play, free)
		c.Place(b.play, geom.Pt(upNextBlockX, y))
		y += ps.H + upNextColGap
	}
	c.Layout(b.course, free)
	c.Place(b.course, geom.Pt(upNextBlockX, y))

	if b.card != nil {
		b.layoutCard(c)
	}

	return size
}

// layoutCard places the next lesson's picture, the disc over its corner and the
// three lines under it.
func (b *upNextBody) layoutCard(c *ui.Context) {
	c.Layout(b.card, ui.Tight(geom.Sz(upNextCardW, upNextCardH)))
	c.Place(b.card, geom.Pt(upNextCardX, upNextCardY))
	ds := c.Layout(b.disc, ui.Tight(geom.Sz(tvkit.CountdownSize, tvkit.CountdownSize)))
	c.Place(b.disc, geom.Pt(upNextCardX+upNextCardW-upNextDiscInset-ds.W, upNextCardY+upNextDiscInset))
	block := ui.Constraints{Min: geom.Sz(upNextCardW, 0), Max: geom.Sz(upNextCardW, ui.Unbounded)}
	y := upNextCardY + upNextCardH + upNextEyebrowTop
	es := c.Layout(b.eyebrw, block)
	c.Place(b.eyebrw, geom.Pt(upNextCardX, y))
	y += es.H + upNextNextTop
	ts := c.Layout(b.title, block)
	c.Place(b.title, geom.Pt(upNextCardX, y))
	y += ts.H + upNextMetaTop
	c.Layout(b.meta, block)
	c.Place(b.meta, geom.Pt(upNextCardX, y))
}

// Paint draws the content, the focused control last.
func (b *upNextBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// load reads the course of the lesson, for the next lesson and the count of the
// watched ones.
func (s *UpNext) load() {
	if s.anchor == nil { // the course is read already
		return
	}
	cid := s.pb.Lesson().CourseID
	read(&s.rd, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, cid)
	}, func(c *ui.Context, course model.Course, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.e.RememberCourse(course)
		s.list = env.PlayerLessons(course)
		s.build()
		if s.anchor != nil {
			focusFrom(c, s.anchor, s.firstControl())
			s.Remove(s.anchor)
			s.anchor = nil
		}
		s.Relayout()
		s.arm()
	})
}

// firstControl is the control the default focus rule finds first: `u1`, else `u2`.
func (s *UpNext) firstControl() ui.Widget {
	if s.body.play != nil {
		return s.body.play
	}

	return s.body.course
}

// build makes the content for the lesson the player holds and the lessons of its
// course.
func (s *UpNext) build() {
	e, pb := s.e, s.pb
	l := pb.Lesson()
	b := s.body
	n := max(l.Count, len(s.list))
	s.next = -1
	if i := env.LessonIndex(s.list, l.ID); i >= 0 && i+1 < len(s.list) && pb.HasNext() {
		s.next = i + 1
	}

	b.chip = kit.NewChip(kit.ChipOkc, e.Fill("tv.upnext.doneTpl", "nn", pad2(l.No), "title", l.Title))
	b.chip.Height, b.chip.Radius, b.chip.PadX, b.chip.Size = upNextChipH, upNextChipR, upNextChipPad, upNextChipSize
	b.chip.Icon, b.chip.IconSize, b.chip.IconStroke = icon.Check, upNextChipIcon, upNextChipStroke
	first := ""
	if u, ok := e.SessionUser(); ok {
		first = u.FirstName
	}
	b.cheer = &kit.Label{
		Text: e.Fill("tv.upnext.cheerTpl", "first", first), Role: theme.RoleDisplay, Size: upNextCheerSize, Weight: 800,
		LineHeight: 1, Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkText,
	}
	b.cheer.Name = nameUpNextCheer
	b.cheer.Heading = 1 // the title of the screen
	b.sub = &kit.Label{
		Text: e.Fill("tv.upnext.watchedTpl", "course", l.CourseName, "n", strconv.Itoa(s.watched(l)), "total", strconv.Itoa(n)),
		Size: upNextSubSize, Ink: theme.InkText2,
	}
	b.sub.Name = nameUpNextSub
	b.Add(b.chip, b.cheer, b.sub)

	if s.next >= 0 {
		nx := s.list[s.next]
		b.play = tvkit.NewButton(keyUpNextPlay, e.T("tv.upnext.now"), icon.Play)
		b.play.IconSize, b.play.PlainIcon = upNextPlayIcon, true
		b.play.OnSelect = func() { playLesson(e, nx, s.list) } // `playLes`
		b.Add(b.play)
	} else {
		b.play = nil
	}
	b.course = tvkit.NewButton(keyUpNextCourse, e.T("done.toCourse"), icon.List)
	cid := l.CourseID
	b.course.OnSelect = func() { // `tgo detail`
		if e.Act != nil {
			e.Act.Go(nav.Route{Screen: env.RouteDetail, Params: cid}, false)
		}
	}
	b.Add(b.course)

	if s.next >= 0 {
		nx := s.list[s.next]
		b.card = poster.New(poster.Spec{ID: l.CourseID, Radius: upNextCardR, NoTitle: true}, upNextCardW, upNextCardH)
		b.card.SetCover(e.Cover, l.CoverURL) // a lesson has no picture: its course's cover (K14)
		b.disc = tvkit.NewCountdown(s.left)
		b.eyebrw = kit.Eyebrow(e.T("tv.upnext.auto"))
		b.eyebrw.Size = upNextEyebrowSize
		b.title = &kit.Label{
			Text: e.Fill("tv.upnext.nextTpl", "nn", pad2(nx.No), "title", nx.Title), Size: upNextNextSize, Weight: 800,
			Wrap: true, Ink: theme.InkText,
		}
		b.title.Name = nameUpNextNext
		b.meta = &kit.Label{
			Text: e.Fill("tv.upnext.metaTpl", "tag", nx.Tag, "dur", format.Clock(float64(nx.DurationSec))),
			Size: upNextMetaSize, Ink: theme.InkText2,
		}
		b.meta.Name = nameUpNextMeta
		b.Add(b.card, b.disc, b.eyebrw, b.title, b.meta)
	}
	b.Relayout()
}

// watched is how many lessons of the course are watched (`L.filter(x=>S.watched[
// key(x)]).length`): the ones the server says are, those this run watched, and the
// lesson just finished.
func (s *UpNext) watched(l player.Lesson) int {
	current := l.Watched
	if dur := s.pb.Dur(); dur > 0 && s.pb.Pos() >= player.WatchedRatio*dur {
		current = true
	}
	n, seen := 0, false
	for _, x := range s.list {
		isCur := x.ID == l.ID
		seen = seen || isCur
		if x.Watched || s.e.WatchedLocal(x.ID) || (isCur && current) {
			n++
		}
	}
	if !seen && current {
		n++
	}

	return n
}

// arm starts the countdown when there is a next lesson to count to.
func (s *UpNext) arm() {
	if s.next < 0 || s.ctx == nil || s.timer.Pending() || s.left <= 0 {
		return
	}
	s.timer = s.ctx.AfterFunc(upNextTick, s.tickFn)
}

// tick is a second of the countdown (`cdT--`): the disc shows it, and at 0 the next
// lesson plays when "Tự phát bài tiếp theo" is on.
func (s *UpNext) tick() {
	s.timer = ui.Timer{}
	if s.left <= 0 {
		return
	}
	s.left--
	s.body.disc.Set(s.left)
	if s.left > 0 {
		s.timer = s.ctx.AfterFunc(upNextTick, s.tickFn)

		return
	}
	if s.e.Prefs().AutoNext() {
		s.playNext()
	}
}

// playNext plays the next lesson from its start, as the player's own step does
// (`S.pt.pos=0`, :790): TV07 takes this screen's place, with `p1` focused.
func (s *UpNext) playNext() {
	nx, ok := s.NextLesson()
	if !ok {
		return
	}
	nx.ResumeSec = 0
	playLesson(s.e, nx, s.list)
}
