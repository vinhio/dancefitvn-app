//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/screens/login.go (the screen
// that owns a request and its timer, built in its Mounted and stopped in its
// Unmounted) and tvtest's fake pairing; jplayer-tv has no pairing screen.

package screens

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/qr"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of TS.code (index.html:651-659), in DIPs.
const (
	// codeLeft, codeTop, codeWidth are `left:160px;top:130px;width:820px` of
	// the left block.
	codeLeft, codeTop, codeWidth float32 = 160, 130, 820
	// codeTitleSize is `.d` at `font-size:72px` (weight 700 from `.d`).
	codeTitleSize float32 = 72
	// codeSubSize, codeSubTop, codeSubBottom are `p.muted` at `font-size:28px;
	// margin:18px 0 50px`.
	codeSubSize, codeSubTop, codeSubBottom float32 = 28, 18, 50
	// codeStepSize, codeStepGap, codeStepRow, codeStepDisc: a step is
	// `display:flex;gap:26px;align-items:center;margin-bottom:30px;
	// font-size:32px;font-weight:600` around a disc of 60.
	codeStepSize, codeStepGap, codeStepRow, codeStepDisc float32 = 32, 26, 30, 60
	// codeDiscSize is the digit of the disc, Barlow `font-size:34px;font-weight:800`.
	codeDiscSize float32 = 34
	// codeBoxTop, codeBoxW, codeBoxH, codeBoxRadius, codeBoxBorder are the code
	// box: `margin-top:40px;width:760px;height:190px;border-radius:28px;
	// border:2px solid var(--bd)`.
	codeBoxTop, codeBoxW, codeBoxH, codeBoxRadius, codeBoxBorder float32 = 40, 760, 190, 28, 2
	// codeValueSize, codeValueSpacing are the code, `font-size:120px;
	// letter-spacing:18px;font-weight:800`.
	codeValueSize, codeValueSpacing float32 = 120, 18
	// codeExpTop, codeExpSize, codeExpIcon are the expiry line: `margin-top:26px;
	// font-size:26px` with a clock of 28.
	codeExpTop, codeExpSize, codeExpIcon float32 = 26, 26, 28
	// inlineIconSpace is the space after an `ic(n,28)` icon at font-size 26, the
	// one of ` Mã hết hạn` (:655) and of the player's status lines: the capture
	// has the icon at x 163-184 and the text's ink at 198, so the text box is at
	// 160+28+7. A measured " " is ~0 (the layout trims it), hence the constant.
	inlineIconSpace float32 = 7
	// codeDivX, codeDivY, codeDivW, codeDivH are the divider
	// `left:1070px;top:300px;width:2px;height:560px`.
	codeDivX, codeDivY, codeDivW, codeDivH float32 = 1070, 300, 2, 560
	// codeColX, codeColY, codeColW are the right column `left:1290px;top:250px;
	// width:340px`.
	codeColX, codeColY, codeColW float32 = 1290, 250, 340
	// codeQRTitleTop, codeQRTitleSize are "Hoặc quét mã QR": `font-size:36px;
	// font-weight:700;margin-top:30px`.
	codeQRTitleTop, codeQRTitleSize float32 = 30, 36
	// codeQRSubTop, codeQRSubSize are the muted line under it: `font-size:26px;
	// margin-top:10px`.
	codeQRSubTop, codeQRSubSize float32 = 10, 26
	// codeQRSubFloor is the share of codeQRSubSize the address is shrunk to at most so
	// that it fits the column (not in the prototype, whose address is shorter).
	codeQRSubFloor float32 = .75
	// codeQRSubLine is the pitch of the hint's lines at codeQRSubSize, `line-height:
	// normal` of 26 px (TV02's box is 62 high for two).
	codeQRSubLine float32 = 31
	// codeBackY is where `c2` is: the capture has it at y 880 under `c1`
	// (`margin-top:50px`, then `margin-top:20px`); `c1` is gone (K2) and `c2`
	// keeps its place (Q-P7-5).
	codeBackY float32 = 880
	// expiryCache is how many seconds of the countdown's text are kept: a code
	// lives 300 s; a longer one is formatted at each tick.
	expiryCache = 1800
	// codeTick is the countdown's period (K8: a 1 s tick).
	codeTick = time.Second
)

// The keys layout.json knows the parts by (the capture's `div.d`, `p.muted`,
// `div.d.acc`); the parts that have no class in the prototype carry a name of
// their own.
const (
	nameCodeTitle  = "div.d"
	nameCodeSub    = "p.muted"
	nameCodeBox    = "div.d.acc"
	nameCodeValue  = "tv.code.value"
	nameCodeStep   = "tv.code.step"
	nameCodeDisc   = "tv.code.disc"
	nameCodeDiv    = "tv.code.divider"
	nameCodeQR     = "tv.code.qr"
	nameCodeQRHead = "tv.code.qrTitle"
	keyCodeBack    = "c2"
)

// Code is TS.code (TV02): how to link this Apple TV, the code to type in the
// phone's app and the QR of the same address, with the time the code has left
// (K8). It starts the pairing when it is mounted and stops it when it is
// unmounted, so leaving the screen ends the polls. The pairing polls every
// poll_interval_sec (3 s); when the code runs out a new one arrives through
// the same hook and the screen shows it. A confirmation on the phone goes to
// TV04 with a toast; a failed request is TV11 over the screen (Q-P7-15), a
// rate limit (429) is waited out by the pairing without a word. The only control is `c2` "Quay lại"
// (the prototype's `c1` is K2), which has the focus.
type Code struct {
	ui.Base

	e *env.Env

	bg      tvkit.Backdrop
	title   *kit.Label
	sub     *kit.Label
	steps   [3]codeStep
	box     *ui.Box
	value   *codeValue
	expiry  expiryLine
	divider *ui.Box
	qr      *tvkit.QR
	qrTitle *kit.Label
	qrSub   *kit.Label
	back    *tvkit.Button
	// subProbe measures the hint under the QR, in no tree.
	subProbe kit.Label

	// The pairing under way, and the countdown's state. All on the UI thread.
	ctx    *ui.Context
	pair   env.Pairing
	have   bool   // a code is on screen
	qrURL  string // the address the QR was made from
	timer  ui.Timer
	tickFn func()
	texts  []string // the countdown's text by seconds left, made on first use
}

// codeStep is one of the three steps: the disc with its number and the text.
type codeStep struct {
	disc *stepDisc
	num  *kit.Label
	text *kit.Label
}

// stepDisc is the disc of a step, `rgba(255,208,40,.16)` under its number. It is
// decoration for a screen reader: the three lines are read in order, and the
// number says nothing the order does not.
type stepDisc struct {
	ui.Box
	kit.Decor
}

// codeValue is the code in its box, `div.d.acc` of TS.code: the letters and
// digits, read one by one under the name of what it is, as a code is spelled
// ("Mã liên kết, 7 K 4 M 2 P"), not as the word it looks like.
type codeValue struct {
	kit.Label

	name string // what a screen reader calls the code
}

// expiryLine is the muted `p` under the code box: the clock icon, then the text
// on one line, the text's baseline on the icon's bottom (an inline svg sits on
// the baseline). It is `p.muted#2` of the capture.
type expiryLine struct {
	ui.Base
	icon  *kit.Icon
	label *kit.Label
}

var (
	_ ui.Mounter   = (*Code)(nil)
	_ ui.Unmounter = (*Code)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewCode returns TV02.
func NewCode(e *env.Env) *Code {
	s := &Code{e: e}
	s.tickFn = s.tick
	// radial-gradient(circle at 20% 10%, rgba(255,208,40,.22), transparent 50%), var(--bg) (:651)
	s.bg.Layers = []kit.Radial{{Shape: kit.RadialCircle, CX: .2, CY: .1, Color: tokens.Dark.Primary.WithAlpha(.22), End: .5}}
	s.Add(&s.bg)

	s.title = &kit.Label{
		Text: e.T("tv.code.title"), Role: theme.RoleDisplay, Size: codeTitleSize, Weight: 700,
		Spacing: .6, LineHeight: 1, Upper: true, Ink: theme.InkText,
	}
	s.title.Name = nameCodeTitle
	s.title.Heading = 1 // the title of the screen
	s.sub = &kit.Label{Text: e.T("tv.code.sub"), Size: codeSubSize, Wrap: true, Ink: theme.InkText2}
	s.sub.Name = nameCodeSub
	s.Add(s.title, s.sub)

	for i := range s.steps {
		st := &s.steps[i]
		st.num = &kit.Label{
			Text: strconv.Itoa(i + 1), Role: theme.RoleDisplay, Size: codeDiscSize, Weight: 800, Ink: theme.InkPrimary,
		}
		st.disc = &stepDisc{Box: ui.Box{
			Width: codeStepDisc, Height: codeStepDisc, Radius: codeStepDisc / 2, Align: ui.AlignCenter,
			Background: scene.Solid(tokens.Dark.Primary.WithAlpha(.16)), // rgba(255,208,40,.16)
		}}
		st.disc.Name = nameCodeDisc
		st.disc.Add(st.num)
		st.text = &kit.Label{
			Text: e.T("tv.code.step" + strconv.Itoa(i+1)), Size: codeStepSize, Weight: 600, Ink: theme.InkText,
		}
		st.text.Name = nameCodeStep
		s.Add(st.disc, st.text)
	}

	s.value = &codeValue{name: e.T("a11y.pairCode"), Label: kit.Label{
		Role: theme.RoleDisplay, Size: codeValueSize, Weight: 800, Spacing: codeValueSpacing, Upper: true, Ink: theme.InkPrimary,
	}}
	s.value.Name = nameCodeValue
	s.box = &ui.Box{
		Width: codeBoxW, Height: codeBoxH, Radius: codeBoxRadius, Align: ui.AlignCenter,
		Background:  scene.Solid(tokens.Dark.Surface),
		Border:      geom.Insets{Top: codeBoxBorder, Right: codeBoxBorder, Bottom: codeBoxBorder, Left: codeBoxBorder},
		BorderColor: tokens.Dark.Border,
	}
	s.box.Name = nameCodeBox
	s.box.Add(s.value)
	s.Add(s.box)

	s.expiry.Name = nameCodeSub
	s.expiry.icon = kit.NewIcon(icon.Clock, codeExpIcon, theme.InkText2)
	s.expiry.label = &kit.Label{Size: codeExpSize, Ink: theme.InkText2}
	s.expiry.Add(s.expiry.icon, s.expiry.label)
	s.expiry.Hidden = true // until there is a code
	s.Add(&s.expiry)

	s.divider = &ui.Box{Width: codeDivW, Height: codeDivH, Background: scene.Solid(tokens.Dark.Border)}
	s.divider.Name = nameCodeDiv
	s.Add(s.divider)

	s.qr = tvkit.NewQR()
	s.qr.Name = nameCodeQR
	s.qrTitle = &kit.Label{
		Text: e.T("tv.code.qrTitle"), Size: codeQRTitleSize, Weight: 700, Align: ui.TextAlignCenter, Ink: theme.InkText,
	}
	s.qrTitle.Name = nameCodeQRHead
	s.qrSub = &kit.Label{
		Text: e.T("tv.code.qrSub"), Size: codeQRSubSize, Wrap: true, Align: ui.TextAlignCenter, Ink: theme.InkText2,
	}
	s.qrSub.Name = nameCodeSub
	s.back = tvkit.NewButton(keyCodeBack, e.T("tv.back"), icon.NameUnknown)
	s.back.Size = tvkit.ButtonTiny
	s.back.OnSelect = s.onBack
	s.Add(s.qr, s.qrTitle, s.qrSub, s.back)

	return s
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Item returns the control of the key (`c2`), nil when there is none.
func (s *Code) Item(key string) *tvkit.Button {
	if key == s.back.Key {
		return s.back
	}

	return nil
}

// QR returns the QR box.
func (s *Code) QR() *tvkit.QR { return s.qr }

// CodeText returns the code on screen, "" before the first.
func (s *Code) CodeText() string { return s.value.Text }

// ExpiryText returns the countdown's line as shown, "" while it is hidden.
func (s *Code) ExpiryText() string {
	if s.expiry.Hidden {
		return ""
	}

	return s.expiry.label.Text
}

// HintBreaksWord tells whether the hint under the QR has a line that starts inside a
// word (the address "dancefitvn.com/mobile/tv" cut at a "/").
func (s *Code) HintBreaksWord() bool { return s.qrSub.BreaksWord() }

// Mounted starts the pairing and the countdown.
func (s *Code) Mounted(c *ui.Context) {
	if s.pair != nil {
		return
	}
	s.ctx = c
	s.pair = s.e.Pair(env.PairHooks{OnState: s.onState, OnSignedIn: s.onSignedIn, OnError: s.onError})
	if s.pair == nil {
		return
	}
	if st, ok := s.pair.Current(); ok { // a hook called before Pair returned
		s.apply(st)
	}
	s.timer = c.AfterFunc(codeTick, s.tickFn)
}

// Unmounted ends the polls and the countdown: leaving TV02 stops the pairing.
func (s *Code) Unmounted(*ui.Context) {
	s.timer.Stop()
	s.timer = ui.Timer{}
	if s.pair != nil {
		s.pair.Stop()
		s.pair = nil
	}
}

// Layout flows the left block as CSS does, then puts the divider and the right
// column where the prototype has them.
func (s *Code) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(cs.Max)
	c.Layout(&s.bg, ui.Tight(size))
	c.Place(&s.bg, geom.Point{})

	// left block: every line is a block of the column's width
	block := ui.Constraints{Min: geom.Sz(codeWidth, 0), Max: geom.Sz(codeWidth, ui.Unbounded)}
	y := codeTop
	ts := c.Layout(s.title, block)
	c.Place(s.title, geom.Pt(codeLeft, y))
	y += ts.H + codeSubTop
	ss := c.Layout(s.sub, block)
	c.Place(s.sub, geom.Pt(codeLeft, y))
	y += ss.H + codeSubBottom
	for i := range s.steps {
		st := &s.steps[i]
		ds := c.Layout(st.disc, ui.Tight(geom.Sz(codeStepDisc, codeStepDisc)))
		tx := c.Layout(st.text, ui.Loose(geom.Sz(codeWidth-codeStepDisc-codeStepGap, ui.Unbounded)))
		row := max(ds.H, tx.H)
		c.Place(st.disc, geom.Pt(codeLeft, y+(row-ds.H)/2))
		c.Place(st.text, geom.Pt(codeLeft+ds.W+codeStepGap, y+(row-tx.H)/2))
		y += row
		if i < len(s.steps)-1 {
			y += codeStepRow
		}
	}
	// the last step's margin-bottom (30) collapses into the box's margin-top (40)
	y += max(codeStepRow, codeBoxTop)
	bs := c.Layout(s.box, ui.Tight(geom.Sz(codeBoxW, codeBoxH)))
	c.Place(s.box, geom.Pt(codeLeft, y))
	y += bs.H + codeExpTop
	if s.expiry.Hidden {
		c.Layout(&s.expiry, ui.Tight(geom.Size{}))
	} else {
		c.Layout(&s.expiry, ui.Constraints{Min: geom.Sz(codeWidth, 0), Max: geom.Sz(codeWidth, ui.Unbounded)})
	}
	c.Place(&s.expiry, geom.Pt(codeLeft, y))

	c.Layout(s.divider, ui.Tight(geom.Sz(codeDivW, codeDivH)))
	c.Place(s.divider, geom.Pt(codeDivX, codeDivY))

	// right column, text-align:center in 340
	col := ui.Constraints{Min: geom.Sz(codeColW, 0), Max: geom.Sz(codeColW, ui.Unbounded)}
	y = codeColY
	qs := c.Layout(s.qr, ui.Loose(geom.Sz(codeColW, ui.Unbounded)))
	c.Place(s.qr, geom.Pt(codeColX+(codeColW-qs.W)/2, y))
	y += qs.H + codeQRTitleTop
	hs := c.Layout(s.qrTitle, col)
	c.Place(s.qrTitle, geom.Pt(codeColX, y))
	y += hs.H + codeQRSubTop
	// the line breaks at the spaces of the sentence, never inside the address
	// ("dancefitvn.com/mobile/" over "tv"): smaller, down to the floor, when the column
	// is narrower than the address
	s.subProbe.Role, s.subProbe.Ink = s.qrSub.Role, s.qrSub.Ink
	s.subProbe.SetText(s.qrSub.Text)
	if size := kit.WordSize(c, &s.subProbe, codeQRSubSize, codeQRSubFloor, codeColW); size != s.qrSub.Size {
		s.qrSub.Size = size
		s.qrSub.LineHeight = 0
		if size != codeQRSubSize { // the lines keep the pitch of the 26 px ones: the box keeps its height
			s.qrSub.LineHeight = codeQRSubLine / size
		}
		s.qrSub.Relayout()
	}
	c.Layout(s.qrSub, col)
	c.Place(s.qrSub, geom.Pt(codeColX, y))
	bt := c.Layout(s.back, ui.Loose(geom.Sz(codeColW, ui.Unbounded)))
	c.Place(s.back, geom.Pt(codeColX+(codeColW-bt.W)/2, codeBackY))

	return size
}

// Describe describes the code as a text named "Mã liên kết" whose value is the
// code spelled out; the dash is not spoken.
func (v *codeValue) Describe(c *ui.Context, n *platform.SemanticNode) {
	v.Label.Describe(c, n)
	n.Label, n.Value = v.name, spell(v.Text)
}

// Layout puts the clock and the text on one line, the text's baseline on the
// clock's bottom edge.
func (l *expiryLine) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if l.Hidden {
		return cs.Constrain(geom.Size{})
	}
	is := c.Layout(l.icon, ui.Tight(geom.Sz(codeExpIcon, codeExpIcon)))
	ts := c.Layout(l.label, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	base := l.label.Baseline()
	top := max(is.H, base) // the line box above the baseline
	c.Place(l.icon, geom.Pt(0, top-is.H))
	c.Place(l.label, geom.Pt(is.W+inlineIconSpace, top-base))

	return cs.Constrain(geom.Sz(cs.Max.W, top+(ts.H-base)))
}

// LayoutStyle is `p.muted`: the text style of the line and its text.
func (l *expiryLine) LayoutStyle(c *ui.Context) (kit.CSS, string) { return l.label.LayoutStyle(c) }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// spell writes a code one character at a time, "7K4-M2P" as "7 K 4 M 2 P", so
// that a screen reader says each, not a word; "" for no code.
func spell(code string) string {
	var b strings.Builder
	for _, r := range code {
		if r == '-' || unicode.IsSpace(r) {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}

	return b.String()
}

// onState is a new code: the first, or the one after an expiry.
func (s *Code) onState(st pairing.State) {
	if s.pair == nil {
		return
	}
	s.apply(st)
}

// apply shows a code: its text, the QR when the address changed, the time left.
func (s *Code) apply(st pairing.State) {
	s.have = true
	s.value.SetText(st.DisplayCode)
	if st.QRURL != s.qrURL {
		s.qrURL = st.QRURL
		code, err := qr.Encode([]byte(st.QRURL), qr.LevelM)
		if err != nil { // an address no symbol holds: the box stays white
			code = nil
		}
		s.qr.Set(code)
	}
	s.showExpiry()
}

// onSignedIn is the code confirmed on the phone: the session is kept, TV04
// takes the place of the stack and a toast greets the account.
func (s *Code) onSignedIn(email string) {
	if s.pair == nil || s.e.Act == nil {
		return
	}
	s.e.Act.Go(nav.Route{Screen: env.RouteHome}, true)
	s.e.Act.Toast(s.e.Fill("tv.toast.signedInTpl", "email", email))
}

// onError is a request that failed and that the pairing tries again by itself.
// The prototype has no failing pairing, so the plan's answer (Q-P7-15) is the one
// of the other screens: TV11 over this screen, whose "Thử lại" pairs again (the
// polls stop while it is up). A 429 is the one exception: the server asks to wait
// and the pairing waits retry_after by itself, which nobody needs to be told.
func (s *Code) onError(err error) {
	if s.pair == nil || s.e.Act == nil {
		return
	}
	var apiErr *api.Error
	if s.e.Online() && errors.As(err, &apiErr) && apiErr.Status == http.StatusTooManyRequests {
		return
	}
	s.e.Act.Go(nav.Route{Screen: env.RouteError}, false)
}

// onBack is `data-tact="tback"` of `c2`.
func (s *Code) onBack() {
	if s.e.Act != nil {
		s.e.Act.Back()
	}
}

// tick is the countdown's second.
func (s *Code) tick() {
	s.timer = ui.Timer{}
	if s.pair == nil {
		return
	}
	s.showExpiry()
	s.timer = s.ctx.AfterFunc(codeTick, s.tickFn)
}

// showExpiry writes the time left, rounded up to the second (a code with 299.4 s
// left reads 05:00 until it has 299 s), or hides the line while there is no code.
func (s *Code) showExpiry() {
	if !s.have || s.pair == nil {
		kit.SetHidden(&s.expiry, true)

		return
	}
	secs := int(math.Ceil(s.pair.Remaining().Seconds()))
	kit.SetHidden(&s.expiry, false)
	s.expiry.label.SetText(s.expiryText(secs))
}

// expiryText is "Mã hết hạn sau mm:ss · mã mới sẽ tự tạo" for secs left. The
// texts of the first expiryCache seconds are made once and kept, so the tick of
// a code that has run before allocates nothing.
func (s *Code) expiryText(secs int) string {
	secs = max(secs, 0)
	if secs >= expiryCache {
		return s.e.Fill("tv.code.expiresTpl", "t", format.Clock(float64(secs)))
	}
	if len(s.texts) == 0 {
		s.texts = make([]string, expiryCache)
	}
	if s.texts[secs] == "" {
		s.texts[secs] = s.e.Fill("tv.code.expiresTpl", "t", format.Clock(float64(secs)))
	}

	return s.texts[secs]
}
