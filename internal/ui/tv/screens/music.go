//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/audio.go (a screen over
// the player that follows its Changes while it is mounted) and from this
// package's player.go (the Play/Pause key kept while the screen is up, the read of
// the course for the lessons the player walks). The screen is TS.music
// (index.html:694-699, docs/plan/P7 section 4.2); where its audio comes from when
// the user arrives from the nav bar is Q-P7-10.

package screens

import (
	"context"
	"strings"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
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

// The numbers of TS.music (index.html:694-699), in DIPs.
const (
	// musicPosterX, musicPosterY, musicPosterS, musicPosterR, musicPosterTS are
	// `poster(l.cid,560,560,{r:34,ts:78})` at `left:150px;top:220px`.
	musicPosterX, musicPosterY, musicPosterS, musicPosterR, musicPosterTS float32 = 150, 220, 560, 34, 78
	// musicBlockX, musicBlockY, musicBlockW are `left:800px;top:250px;width:1000px`.
	musicBlockX, musicBlockY, musicBlockW float32 = 800, 250, 1000
	// musicEyebrowSize, musicEyebrowSpacing are `font-size:24px;font-weight:700;
	// letter-spacing:2.4px`.
	musicEyebrowSize, musicEyebrowSpacing float32 = 24, 2.4
	// musicTitleTop, musicTitleSize are `.d` at `font-size:88px;font-weight:800;
	// margin-top:24px`.
	musicTitleTop, musicTitleSize float32 = 24, 88
	// musicSubMargin, musicSubSize are `p.muted` at `font-size:30px;margin:20px 0`.
	musicSubMargin, musicSubSize float32 = 20, 30
	// musicTimesTop, musicTimesSize are `font-size:26px;margin-top:14px` of the
	// clock (`.mono.muted`).
	musicTimesTop, musicTimesSize float32 = 14, 26
	// musicRowTop, musicRowGap are `display:flex;gap:30px;align-items:center;
	// margin-top:40px` of the buttons.
	musicRowTop, musicRowGap float32 = 40, 30
)

// The keys of the controls: `data-fid` of TS.music.
const (
	keyMusicPrev   = "mu0"
	keyMusicToggle = "mu1"
	keyMusicNext   = "mu2"
)

// Names layout.json knows the parts by: `div.d` and `p.muted` are the capture's, and
// the clock is `div.mono.muted`.
const (
	nameMusicEyebrow = "tv.music.eyebrow"
	nameMusicTitle   = "div.d"
	nameMusicSub     = "p.muted"
	nameMusicTimes   = "div.mono.muted"
)

// Music is TS.music (TV09): the lesson's audio file, the cover at the left and at
// the right the lesson, a wave of 120 bars that fills as it plays, the clock, and
// the buttons `mu0` (‹ Bài trước), `mu1` (the play disc) and `mu2` (Bài sau ›).
// `mu0` has the focus first; the Play/Pause key toggles while it is up.
//
// Arriving from the nav bar it plays the audio of the lesson the player holds, at
// the position the video had, and goes on only if it was playing (`$P:735-736`);
// with no lesson loaded it takes the one the home continues, at its position,
// paused (Q-P7-10). Until the audio is there the screen is the nav bar over the
// background (K12).
type Music struct {
	ui.Base

	e  *env.Env
	pb env.Playback

	bg     tvkit.Backdrop
	body   *musicBody
	nav    *tvkit.Nav
	anchor *waitAnchor

	list    []player.Lesson // the lessons the player walks, once known
	lesson  string          // the id the labels were made for
	sec     int             // the whole second the clock shows
	have    bool            // the content is on screen
	started bool            // the audio was asked for

	unsub, removePlay func()
	rd, avatar        reader // the read of the lesson to play, the read of the user's picture
}

// musicBody is the content under the nav bar: the cover, the text block, the wave
// and the buttons. A focused control paints last (F-12).
type musicBody struct {
	ui.Base

	cover   *poster.Poster
	eyebrow *kit.Label
	title   *kit.Label
	sub     *kit.Label
	wave    *tvkit.Wave
	times   *clockRow
	pos     *kit.Label
	dur     *kit.Label
	prev    *tvkit.Button
	toggle  *tvkit.Disc
	next    *tvkit.Button
}

var (
	_ ui.Mounter   = (*Music)(nil)
	_ ui.Unmounter = (*Music)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewMusic returns TV09 over the player of the services: the nav bar and the
// background until the audio is there.
func NewMusic(e *env.Env) *Music {
	s := &Music{e: e, pb: e.Playback(), body: &musicBody{}, sec: -1}
	// radial-gradient(circle at 25% 50%, rgba(255,208,40,.22), transparent 50%), var(--bg) (:694)
	s.bg.Layers = []kit.Radial{{Shape: kit.RadialCircle, CX: .25, CY: .5, Color: tokens.Dark.Primary.WithAlpha(.22), End: .5}}
	s.nav = newNavBar(e, tabMusic)
	s.anchor = newWaitAnchor(e, keyMusicPrev)
	// The nav bar is the top of the page, so a screen reader reads it first; it is
	// painted last, over the content (Paint).
	s.Add(&s.bg, s.nav, s.body, s.anchor)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`mu0`..`mu2`), nil when there is none yet.
func (s *Music) Item(key string) *tvkit.Item {
	b := s.body
	switch key {
	case keyMusicPrev:
		if b.prev != nil {
			return &b.prev.Item
		}
	case keyMusicToggle:
		if b.toggle != nil {
			return &b.toggle.Item
		}
	case keyMusicNext:
		if b.next != nil {
			return &b.next.Item
		}
	}

	return nil
}

// Nav returns the nav bar.
func (s *Music) Nav() *tvkit.Nav { return s.nav }

// Wave returns the wave, nil until the audio is there.
func (s *Music) Wave() *tvkit.Wave { return s.body.wave }

// Toggle returns the play disc, nil until the audio is there.
func (s *Music) Toggle() *tvkit.Disc { return s.body.toggle }

// Eyebrow returns the kicker ("FILE ÂM THANH · SET 35 · BÀI 03"), nil until the audio
// is there.
func (s *Music) Eyebrow() *kit.Label {
	if s.body == nil {
		return nil
	}

	return s.body.eyebrow
}

// Loaded tells whether the content is on screen.
func (s *Music) Loaded() bool { return s.have }

// Mounted follows the player, keeps the Play/Pause key and starts the audio with
// the next turn: a view that is being mounted has no host yet, and ui.Go would run
// a read on the UI thread then.
func (s *Music) Mounted(c *ui.Context) {
	if !s.avatar.Done() && !s.avatar.Busy() {
		c.AfterFunc(0, s.loadAvatar)
	}
	if s.pb == nil {
		return
	}
	if s.unsub == nil {
		s.unsub = s.pb.Changes().Subscribe(s.onChange)
	}
	if s.removePlay == nil && s.e.Act != nil {
		s.removePlay = s.e.Act.OnPlayPause(s.pb.Toggle)
	}
	if !s.started {
		c.AfterFunc(0, s.start)
	}
}

// Unmounted stops following, gives the key back and ends the reads. The audio goes
// on: only the Shell pauses it.
func (s *Music) Unmounted(*ui.Context) {
	for _, f := range [...]*func(){&s.unsub, &s.removePlay} {
		if *f != nil {
			(*f)()
			*f = nil
		}
	}
	s.rd.Stop()
	s.avatar.Stop()
}

// Paint draws the background and the content, then the nav bar over them.
func (s *Music) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintOver(c, p, &s.Base, s.nav) }

// Layout fills the room: the background, the content and the nav bar over it.
func (s *Music) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	for _, w := range [...]ui.Widget{&s.bg, s.body, s.nav} {
		c.Layout(w, ui.Tight(size))
		c.Place(w, geom.Point{})
	}
	if s.anchor != nil {
		c.Layout(s.anchor, ui.Loose(geom.Sz(1, 1)))
		c.Place(s.anchor, geom.Pt(size.W/2, size.H/2))
	}

	return size
}

// Layout places the cover, flows the text block as CSS does and puts the buttons
// under it.
func (b *musicBody) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	if b.cover == nil {
		return size
	}
	c.Layout(b.cover, ui.Tight(geom.Sz(musicPosterS, musicPosterS)))
	c.Place(b.cover, geom.Pt(musicPosterX, musicPosterY))

	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	block := ui.Constraints{Min: geom.Sz(musicBlockW, 0), Max: geom.Sz(musicBlockW, ui.Unbounded)}
	y := musicBlockY
	es := c.Layout(b.eyebrow, block)
	c.Place(b.eyebrow, geom.Pt(musicBlockX, y))
	y += es.H + musicTitleTop
	ts := c.Layout(b.title, block)
	c.Place(b.title, geom.Pt(musicBlockX, y))
	y += ts.H + musicSubMargin
	ss := c.Layout(b.sub, block)
	c.Place(b.sub, geom.Pt(musicBlockX, y))
	y += ss.H + musicSubMargin
	ws := c.Layout(b.wave, ui.Constraints{Min: geom.Sz(musicBlockW, 0), Max: geom.Sz(musicBlockW, ui.Unbounded)})
	c.Place(b.wave, geom.Pt(musicBlockX, y))
	y += ws.H + musicTimesTop
	ts2 := c.Layout(b.times, block)
	c.Place(b.times, geom.Pt(musicBlockX, y))
	y += ts2.H + musicRowTop

	// the buttons are centred on the disc's height
	bs := c.Layout(b.prev, free)
	disc := c.Layout(b.toggle, ui.Tight(geom.Sz(tvkit.DiscSize, tvkit.DiscSize)))
	ns := c.Layout(b.next, free)
	row := max(bs.H, disc.H, ns.H)
	x := musicBlockX
	c.Place(b.prev, geom.Pt(x, y+(row-bs.H)/2))
	x += bs.W + musicRowGap
	c.Place(b.toggle, geom.Pt(x, y+(row-disc.H)/2))
	x += disc.W + musicRowGap
	c.Place(b.next, geom.Pt(x, y+(row-ns.H)/2))

	return size
}

// Paint draws the content, the focused control last.
func (b *musicBody) Paint(c *ui.Context, p *ui.Painter) { tvkit.PaintRaised(c, p, &b.Base) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// loadAvatar reads the picture of the nav bar.
func (s *Music) loadAvatar() { loadAvatar(s, &s.avatar, s.e, s.nav) }

// start finds the audio to show: the lesson loaded is played as audio, else the
// home's.
func (s *Music) start() {
	if s.started || s.View() == nil { // asked once; detached, the next Mounted asks again
		return
	}
	s.started = true
	l := s.pb.Lesson()
	switch {
	case l.ID == "":
		s.fromHome()
	case s.pb.Kind() == player.KindAudio:
		s.show()
	default:
		s.switchTo(l)
	}
}

// switchTo plays the audio of the lesson at the position the video has, and goes
// on only if it was playing (`S.pt.kind='audio'`, `S.pt.playing=g==='music'&&
// S.pt.playing`, :735-736). The player walks the course's lessons, so they are
// read first when nobody has.
func (s *Music) switchTo(l player.Lesson) {
	if c, ok := s.e.CachedCourse(l.CourseID); ok {
		s.toAudio(l, env.PlayerLessons(c), s.pb.Pos())

		return
	}
	pos := s.pb.Pos()
	read(&s.rd, s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, l.CourseID)
	}, func(_ *ui.Context, c model.Course, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		s.e.RememberCourse(c)
		s.toAudio(l, env.PlayerLessons(c), pos)
	})
}

// toAudio asks the player for the audio of l at pos and shows it.
func (s *Music) toAudio(l player.Lesson, list []player.Lesson, pos float64) {
	playing := s.pb.Playing()
	s.list = list
	s.pb.PlayAudio(l, list, pos)
	if !playing {
		s.pb.Pause()
	}
	s.show()
}

// fromHome takes the lesson the home continues, at its position, paused: the
// player holds none yet (Q-P7-10). Nothing to continue leaves the screen as it is.
func (s *Music) fromHome() {
	type result struct {
		course model.Course
		lesson string
		pos    int
		ok     bool
	}
	read(&s.rd, s, func(ctx context.Context) (result, error) {
		h, err := s.e.Home(ctx)
		if err != nil || h.Continue == nil {
			return result{}, err
		}
		c, err := s.e.Course(ctx, h.Continue.Course.ID)
		if err != nil {
			return result{}, err
		}

		return result{course: c, lesson: h.Continue.Lesson.ID, pos: h.Continue.PositionSec, ok: true}, nil
	}, func(_ *ui.Context, r result, err error) {
		if err != nil {
			failed(s.e, err)

			return
		}
		if !r.ok {
			return
		}
		s.e.RememberCourse(r.course)
		list := env.PlayerLessons(r.course)
		i := env.LessonIndex(list, r.lesson)
		if i < 0 {
			return
		}
		s.toAudio(list[i], list, float64(r.pos))
	})
}

// show builds the content for the lesson the player holds, once, and gives the
// focus to `mu0` if the stand-in has it.
func (s *Music) show() {
	if s.have {
		return
	}
	s.have = true
	e, pb := s.e, s.pb
	b := s.body
	b.cover = poster.New(poster.Spec{Radius: musicPosterR, TitleSize: musicPosterTS}, musicPosterS, musicPosterS)
	b.eyebrow = &kit.Label{Size: musicEyebrowSize, Weight: 700, Spacing: musicEyebrowSpacing, Upper: true, Wrap: true, Ink: theme.InkPrimary}
	b.eyebrow.Name = nameMusicEyebrow
	b.title = &kit.Label{
		Role: theme.RoleDisplay, Size: musicTitleSize, Weight: 800, LineHeight: 1, Spacing: homeDisplaySpacing,
		Upper: true, Wrap: true, Ink: theme.InkText,
	}
	b.title.Name = nameMusicTitle
	b.title.Heading = 1 // the title of the screen
	b.sub = &kit.Label{Size: musicSubSize, Ink: theme.InkText2}
	b.sub.Name = nameMusicSub
	b.wave = tvkit.NewWave()
	b.wave.Label = e.T("a11y.position")
	b.pos = &kit.Label{Role: theme.RoleMono, Size: musicTimesSize, Ink: theme.InkText2}
	b.dur = &kit.Label{Role: theme.RoleMono, Size: musicTimesSize, Ink: theme.InkText2}
	b.times = newClockRow(nameMusicTimes, b.pos, b.dur)

	b.prev = tvkit.NewButton(keyMusicPrev, e.T("tv.music.prev"), icon.NameUnknown)
	b.prev.OnSelect = func() { // `tprev`
		if pb.HasPrev() {
			pb.Prev()
		}
	}
	b.prev.Item.Label = spoken(e.T("tv.music.prev")) // without the chevron a screen reader would read out
	b.toggle = tvkit.NewDisc(keyMusicToggle, icon.Play)
	b.toggle.PlayLabel, b.toggle.PauseLabel = e.T("a11y.play"), e.T("a11y.pause")
	b.toggle.OnSelect = pb.Toggle // `tplay`
	b.next = tvkit.NewButton(keyMusicNext, e.T("tv.music.next"), icon.NameUnknown)
	b.next.OnSelect = func() { // `tnext`
		if pb.HasNext() {
			pb.Next()
		}
	}
	b.next.Item.Label = spoken(e.T("tv.music.next"))
	b.Add(b.cover, b.eyebrow, b.title, b.sub, b.wave, b.times, b.prev, b.toggle, b.next)
	s.syncLesson()
	s.syncState()
	s.syncLive()
	b.Relayout()
	if s.anchor != nil {
		if v := s.View(); v != nil {
			focusFrom(v.Context(), s.anchor, b.prev)
		}
		s.Remove(s.anchor)
		s.anchor = nil
	}
	s.Relayout()
}

// onChange follows the player.
func (s *Music) onChange(ch player.Change) {
	if !s.have {
		return
	}
	switch ch.Kind {
	case player.ChangeLesson, player.ChangeDuration:
		s.syncLesson()
		s.syncLive()
	case player.ChangeState:
		s.syncState()
	case player.ChangeTime, player.ChangeSeek:
		s.syncLive()
	case player.ChangeError: // the audio did not load: the disc shows "play" again (the state change before this one) and a press retries
		if s.e.Act != nil {
			s.e.Act.Toast(s.e.T("player.err.load"))
		}
	case player.ChangeUnknown, player.ChangeRate, player.ChangeOptions, player.ChangeEnded:
	default:
	}
}

// syncLesson sets the cover and the texts of the lesson the player holds.
func (s *Music) syncLesson() {
	l := s.pb.Lesson()
	b := s.body
	if l.ID != s.lesson {
		s.lesson = l.ID
		s.sec = -1
		b.cover.SetSpec(poster.Spec{ID: l.CourseID, Title: l.CourseShort, Radius: musicPosterR, TitleSize: musicPosterTS})
		b.cover.SetCover(s.e.Cover, l.CoverURL) // a lesson has no picture: its course's cover (K14)
	}
	b.eyebrow.SetText(keepTail(s.e.Fill("tv.music.eyebrowTpl", "course", l.CourseName, "nn", pad2(l.No))))
	b.title.SetText(l.Title)
	dur := s.pb.Dur()
	if dur <= 0 {
		dur = float64(l.DurationSec)
	}
	b.sub.SetText(s.e.Fill("tv.detail.rowTpl", "tag", l.Tag, "dur", format.Clock(dur)))
	if txt := format.Clock(dur); txt != b.dur.Text {
		b.dur.SetText(txt)
		s.sec = -1 // the reading of the wave holds the length: made again
	}
}

// syncState sets the disc's glyph: PAUSE while it plays, else PLAY.
func (s *Music) syncState() {
	if s.pb.Playing() {
		s.body.toggle.SetGlyph(icon.Pause)
	} else {
		s.body.toggle.SetGlyph(icon.Play)
	}
}

// syncLive sets the wave and the clock; it changes a widget only when its value
// changed and allocates nothing.
func (s *Music) syncLive() {
	pos, dur := s.pb.Pos(), s.pb.Dur()
	if dur <= 0 {
		dur = float64(s.pb.Lesson().DurationSec)
	}
	share := float32(0)
	if dur > 0 {
		share = float32(pos / dur)
	}
	s.body.wave.SetPos(share)
	if sec := int(pos); sec != s.sec {
		s.sec = sec
		clock := format.Clock(pos)
		s.body.pos.SetText(clock)
		s.body.wave.SetSpoken(clock + " / " + s.body.dur.Text) // "01:12 / 03:40", once a second
	}
}

// nbsp is the no-break space.
const nbsp = "\u00a0"

// keepTail glues the end of the kicker, "<last word> · BÀI 02", with no-break spaces,
// so that a long course name wraps before it and the number never stands alone on a
// line ("BÀI" over "02"). A text with no " · " is as it was.
func keepTail(text string) string {
	i := strings.LastIndex(text, " · ")
	if i < 0 {
		return text
	}
	head, tail := text[:i], strings.ReplaceAll(text[i:], " ", nbsp)
	if j := strings.LastIndexByte(head, ' '); j >= 0 {
		head = head[:j] + nbsp + head[j+1:]
	}

	return head + tail
}
