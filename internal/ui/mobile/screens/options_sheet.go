//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/sheets/entry.go at 49de3c2 (content
// that holds the widgets it follows the player with, a column of items with the
// room before each) and the kitdemo's M07 page, which lays this content out at
// the boxes of M07.layout.json. The sheet is SH.options (index.html:505-512).

package screens

import (
	"strconv"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The room between the parts of SH.options (index.html:505-512).
const (
	// optSegTop is `.eb{margin-bottom:10px}` under "Tốc độ phát" (:506).
	optSegTop float32 = 10
	// optRowsTop is `<div style="margin-top:10px">` over the rows (:507).
	optRowsTop float32 = 10
	// optDoneTop is `<button class="btn p" style="margin-top:18px">` (:511).
	optDoneTop float32 = 18
)

// optionsSheet is the content of SH.options: the header, "Tốc độ phát" over the
// five speeds, the rows "Lật gương", "Lặp đoạn A–B" and "Thanh đếm 8 nhịp" with
// their switches, "Chất lượng video" (where the platform can choose it, K21) and
// the button "Xong". It follows the player while it is open: the speed, the
// switches and the loop's ends show what the player holds.
type optionsSheet struct {
	ui.Base

	e     *env.Env
	pb    env.Playback
	prefs env.Prefs

	items   []optItem
	seg     *kit.Segmented
	mirror  *kit.Row
	loop    *kit.Row
	beat    *kit.Row
	quality *kit.Row // nil where the quality cannot be chosen
	unsub   func()

	loopSub string // the text the loop row's sub line was made for
}

// optItem is a part of the column and the room before it.
type optItem struct {
	w      ui.Widget
	before float32
}

var (
	_ ui.Mounter   = (*optionsSheet)(nil)
	_ ui.Unmounter = (*optionsSheet)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newOptionsSheet returns the content of the options sheet over the player; close
// closes the sheet (the header's cross and "Xong"). quality adds the row of the
// video quality, which only iOS can honor (K21).
func newOptionsSheet(e *env.Env, pb env.Playback, closeFn func(), quality bool) *optionsSheet {
	s := &optionsSheet{e: e, pb: pb, prefs: e.Prefs()}

	header := kit.NewSheetHeader(e.T("options.title"), e.T("a11y.close"), closeFn)
	eb := heading(kit.Eyebrow(e.T("options.speed")), 2)

	s.seg = kit.NewSegmented()
	s.seg.NamePrefix = "speed:"
	for _, r := range player.SheetRates {
		s.seg.Options = append(s.seg.Options, speedText(r))
		s.seg.Keys = append(s.seg.Keys, strconv.FormatFloat(r, 'f', -1, 64))
	}
	s.seg.OnSelect = func(i int) { pb.SetRate(player.SheetRates[i]) }

	row := func(name string, ic icon.Name, title, sub string, press func()) *kit.Row {
		r := kit.NewRow(ic, title, sub)
		r.Name, r.Trailing, r.OnPress = name, kit.NewSwitch(false), press

		return r
	}
	s.mirror = row("mirror", icon.Mirror, e.T("options.mirror"), e.T("options.mirrorSub"), func() { pb.SetMirror(!pb.Mirror()); s.sync() })
	s.loop = row("loop", icon.Rep, e.T("options.loop"), "", func() { pb.SetLoop(!pb.Loop()); s.sync() })
	s.beat = row("beat", icon.Music, e.T("options.beat"), e.T("options.beatSub"), func() { pb.SetBeat(!pb.Beat()); s.sync() })

	done := kit.NewButton(kit.ButtonP, e.T("options.done"), icon.NameUnknown)
	done.Name, done.OnPress = "closeSheet", closeFn // `data-act="closeSheet"` (:511)

	s.items = []optItem{
		{w: header},
		{w: eb, before: kit.SheetHeaderGap},
		{w: s.seg, before: optSegTop},
		{w: s.mirror, before: optRowsTop},
		{w: s.loop},
		{w: s.beat},
	}
	if quality {
		q := kit.NewRow(icon.Hd, e.T("options.quality"), "")
		q.Name, q.Last, q.OnPress = "quality", true, func() { s.prefs.CycleQuality(); s.sync() }
		s.quality = q
		s.items = append(s.items, optItem{w: q})
	} else {
		s.beat.Last = true
	}
	s.items = append(s.items, optItem{w: done, before: optDoneTop})
	for _, it := range s.items {
		s.Add(it.w)
	}
	s.sync()

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mounted follows the player while the sheet is open.
func (s *optionsSheet) Mounted(*ui.Context) {
	if s.unsub == nil {
		s.unsub = s.pb.Changes().Subscribe(s.onChange)
	}
	s.sync()
}

// Unmounted stops following it.
func (s *optionsSheet) Unmounted(*ui.Context) {
	if s.unsub != nil {
		s.unsub()
		s.unsub = nil
	}
}

// Layout stacks the parts, each as wide as the sheet's content, with the room
// before it.
func (s *optionsSheet) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = cs.Min.W
	}
	tight := ui.Constraints{Min: geom.Sz(w, 0), Max: geom.Sz(w, ui.Unbounded)}
	y := float32(0)
	for _, it := range s.items {
		y += it.before
		sz := c.Layout(it.w, tight)
		c.Place(it.w, geom.Pt(0, y))
		y += sz.H
	}

	return cs.Constrain(geom.Sz(w, y))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onChange follows the player's options.
func (s *optionsSheet) onChange(ch player.Change) {
	switch ch.Kind {
	case player.ChangeRate, player.ChangeOptions, player.ChangeDuration, player.ChangeLesson:
		s.sync()
	case player.ChangeUnknown, player.ChangeState, player.ChangeTime, player.ChangeSeek, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// sync makes the widgets show what the player and the preferences hold.
func (s *optionsSheet) sync() {
	pb := s.pb
	sel := -1
	for i, r := range player.SheetRates {
		if r == pb.Rate() {
			sel = i
		}
	}
	s.seg.Select(sel)
	setSwitch(s.mirror, pb.Mirror())
	setSwitch(s.loop, pb.Loop())
	setSwitch(s.beat, pb.Beat())
	// `${fmt(p.a)} → ${fmt(p.b)} · lặp liên tục` (:508)
	sub := s.e.Fill("options.loopSubTpl", "a", format.Clock(pb.A()), "b", format.Clock(pb.B())) + " · " + s.e.T("options.loopContinuous")
	if sub != s.loopSub {
		s.loopSub = sub
		s.loop.Sub = sub
		s.loop.Relayout()
	}
	if s.quality != nil {
		v := s.e.T(qualityKey(s.prefs.Quality())) + " ›"
		if v != s.quality.Value {
			s.quality.Value = v
			s.quality.Relayout()
		}
	}
}

// setSwitch turns the switch of a row on or off.
func setSwitch(r *kit.Row, on bool) {
	if sw, ok := r.Trailing.(*kit.Switch); ok {
		sw.SetOn(on)
	}
}

// qualityKey is the locale key of a quality: "Tự động", "1080p", "720p", "480p".
func qualityKey(q store.Quality) string {
	switch q {
	case store.QualityP1080:
		return "options.q1080"
	case store.QualityP720:
		return "options.q720"
	case store.QualityP480:
		return "options.q480"
	case store.QualityUnknown, store.QualityAuto:
		return "options.qualityAuto"
	default:
		return "options.qualityAuto"
	}
}

// qualityRowShown tells whether the phone offers the video quality, as the
// preferences say (Prefs.QualitySupported): iOS caps the resolution of its
// items, while Android's MediaPlayer cannot choose a variant of an HLS stream
// (K21, Q-P4-3), so there the row is not shown. It only leaves a row out, and
// lays out nothing another way (D2).
func qualityRowShown(e *env.Env) bool { return e.Prefs().QualitySupported() }
