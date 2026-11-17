//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/offline.go (a screen that
// says the network is gone and offers to try again) for the TV: TS.error
// (index.html:713-717), where `tretry` goes back when the network is there and
// says so when it is not (:743).

package screens

import (
	"context"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.error (index.html:713-717), in DIPs.
const (
	// errorDiscSize, errorDiscTop are the disc, `width:220px;height:220px;
	// margin:170px auto 0`.
	errorDiscSize, errorDiscTop float32 = 220, 170
	// errorIconSize, errorIconStroke are `ic('wifi',110,1.6)`.
	errorIconSize, errorIconStroke float32 = 110, 1.6
	// errorTitleTop, errorTitleSize are `.d` at `font-size:80px;font-weight:800;
	// margin-top:60px`.
	errorTitleTop, errorTitleSize float32 = 60, 80
	// errorBodyTop, errorBodySize are `p.muted` at `font-size:32px;margin-top:24px`.
	errorBodyTop, errorBodySize float32 = 24, 32
	// errorRowTop, errorRowGap are `display:flex;gap:30px;justify-content:center;
	// margin-top:60px`.
	errorRowTop, errorRowGap float32 = 60, 30
	// errorCodeTop, errorCodeSize are `font-size:24px;color:var(--tx3);margin-top:60px`.
	errorCodeTop, errorCodeSize float32 = 60, 24
)

// errorDiscBg is the disc's `background:rgba(255,90,95,.16)`.
var errorDiscBg = color.RGBA(255, 90, 95, .16)

// The keys of the controls: `data-fid` of TS.error.
const (
	keyErrorRetry = "e1"
	keyErrorBack  = "e2"
)

// Names layout.json knows the parts by: `div.d` and `p.muted` are the capture's;
// the disc is a div without a class and the code line a `p` without one.
const (
	nameErrorTitle = "div.d"
	nameErrorBody  = "p.muted"
	nameErrorCode  = "tv.error.code"
)

// Error is TS.error (TV11), "Không thể kết nối": the Wi-Fi in a red disc, what
// happened, `e1` (Thử lại) and `e2` (Quay lại), and under them the error code with
// the hotline of the support. The TV shows it when a lesson is asked for with the
// network gone, when the network goes while a lesson is up (`$P:739`, `$P:812`) and,
// as the prototype does not have it, when a screen's data cannot be read (K28).
//
// `e1` goes back, which builds the screen it was over again, when the network is
// there; with none it says "Vẫn chưa có mạng". `e1` has the focus first.
type Error struct {
	ui.Base

	e *env.Env

	bg    tvkit.Backdrop
	disc  *errorDisc
	title *kit.Label
	body  *kit.Label
	retry *tvkit.Button
	back  *tvkit.Button
	code  *kit.Label
	hotl  string
	rd    reader // the read of the hotline
}

// errorDisc is the 220 round with the Wi-Fi in it.
type errorDisc struct {
	ui.Base

	icon *kit.Icon
}

var (
	_ ui.Mounter   = (*Error)(nil)
	_ ui.Unmounter = (*Error)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewError returns TV11.
func NewError(e *env.Env) *Error {
	s := &Error{e: e}
	// radial-gradient(circle at 50% 35%, rgba(229,56,59,.35), transparent 45%), var(--bg) (:713)
	s.bg.Layers = []kit.Radial{{Shape: kit.RadialCircle, CX: .5, CY: .35, Color: tokens.Dark.Red.WithAlpha(.35), End: .45}}
	s.disc = &errorDisc{icon: kit.NewIcon(icon.Wifi, errorIconSize, theme.InkDanger)}
	s.disc.icon.Stroke = errorIconStroke
	s.disc.Add(s.disc.icon)
	s.title = &kit.Label{
		Text: e.T("tv.error.title"), Role: theme.RoleDisplay, Size: errorTitleSize, Weight: 800, LineHeight: 1, Spacing: homeDisplaySpacing,
		Upper: true, Align: ui.TextAlignCenter, Ink: theme.InkText,
	}
	s.title.Name = nameErrorTitle
	s.title.Heading = 1 // the title of the screen
	s.body = &kit.Label{Text: e.T("tv.error.body"), Size: errorBodySize, Align: ui.TextAlignCenter, Ink: theme.InkText2}
	s.body.Name = nameErrorBody
	s.retry = tvkit.NewButton(keyErrorRetry, e.T("common.retry"), icon.Ref)
	s.retry.OnSelect = s.onRetry // `tretry`
	s.back = tvkit.NewButton(keyErrorBack, e.T("tv.back"), icon.NameUnknown)
	s.back.OnSelect = func() { // `tback`
		if e.Act != nil {
			e.Act.Back()
		}
	}
	s.code = &kit.Label{Text: e.T("tv.error.code"), Size: errorCodeSize, Align: ui.TextAlignCenter, Ink: theme.InkText3}
	s.code.Name = nameErrorCode
	s.Add(&s.bg, s.disc, s.title, s.body, s.retry, s.back, s.code)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`e1`, `e2`).
func (s *Error) Item(key string) *tvkit.Button {
	switch key {
	case keyErrorRetry:
		return s.retry
	case keyErrorBack:
		return s.back
	default:
		return nil
	}
}

// CodeText returns the error line as shown.
func (s *Error) CodeText() string { return s.code.Text }

// Mounted reads the support's hotline for the error line.
func (s *Error) Mounted(c *ui.Context) {
	if s.hotl == "" && !s.rd.Busy() {
		c.AfterFunc(0, s.load)
	}
}

// Unmounted ends the read.
func (s *Error) Unmounted(*ui.Context) { s.rd.Stop() }

// Layout flows the screen as CSS does: every part centred in the width.
func (s *Error) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(&s.bg, ui.Tight(size))
	c.Place(&s.bg, geom.Point{})
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))
	full := ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)}

	y := errorDiscTop
	ds := c.Layout(s.disc, ui.Tight(geom.Sz(errorDiscSize, errorDiscSize)))
	c.Place(s.disc, geom.Pt((size.W-ds.W)/2, y))
	y += ds.H + errorTitleTop
	ts := c.Layout(s.title, full)
	c.Place(s.title, geom.Pt(0, y))
	y += ts.H + errorBodyTop
	bs := c.Layout(s.body, full)
	c.Place(s.body, geom.Pt(0, y))
	y += bs.H + errorRowTop

	rs := c.Layout(s.retry, free)
	es := c.Layout(s.back, free)
	row := max(rs.H, es.H)
	x := (size.W - rs.W - errorRowGap - es.W) / 2
	c.Place(s.retry, geom.Pt(x, y+(row-rs.H)/2))
	c.Place(s.back, geom.Pt(x+rs.W+errorRowGap, y+(row-es.H)/2))
	y += row + errorCodeTop
	c.Layout(s.code, full)
	c.Place(s.code, geom.Pt(0, y))

	return size
}

// Layout puts the Wi-Fi in the middle of the disc.
func (d *errorDisc) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(errorDiscSize, errorDiscSize))
	is := c.Layout(d.icon, ui.Tight(geom.Sz(errorIconSize, errorIconSize)))
	c.Place(d.icon, geom.Pt((size.W-is.W)/2, (size.H-is.H)/2))

	return size
}

// Paint draws the disc and the Wi-Fi.
func (d *errorDisc) Paint(c *ui.Context, p *ui.Painter) {
	r := d.Bounds()
	p.Fill(r, scene.Solid(errorDiscBg), r.W/2)
	d.Base.Paint(c, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onRetry is `tretry` (index.html:743): with no network the toast says so; with it
// the screen goes back, which builds the screen under it again.
func (s *Error) onRetry() {
	if s.e.Act == nil {
		return
	}
	if !s.e.Online() {
		s.e.Act.Toast(s.e.T("tv.toast.stillOffline"))

		return
	}
	s.e.Act.Back()
}

// load reads the hotline and writes it in the error line; with no answer (the
// network is what failed) the line is the code alone.
func (s *Error) load() {
	read(&s.rd, s, func(ctx context.Context) (model.AppConfig, error) {
		return s.e.Config(ctx)
	}, func(_ *ui.Context, cfg model.AppConfig, err error) {
		if err != nil || cfg.Support.Hotline == "" {
			return
		}
		s.hotl = cfg.Support.Hotline
		s.code.SetText(s.e.Fill("tv.error.codeTpl", "hotline", s.hotl))
	})
}
