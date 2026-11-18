//go:build tvos

package screens

import (
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
	"dancefitvn-app/internal/ui/tv/tvkit"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of the settings card (index.html:707-710), in DIPs.
const (
	// cardX, cardY, cardW, cardH, cardR are `left:90px;top:190px;width:600px;
	// height:840px;border-radius:32px`.
	cardX, cardY, cardW, cardH, cardR float32 = 90, 190, 600, 840, 32
	// cardHeadH, cardHeadGap are the header, `height:340px` with `gap:24px`.
	cardHeadH, cardHeadGap float32 = 340, 24
	// cardPhoto is `photo(200,'#141518')`.
	cardPhoto float32 = 200
	// cardNameSize is `.d` at `font-size:46px;font-weight:800`.
	cardNameSize float32 = 46
	// cardPad is the centre block's `padding:30px 30px 0`.
	cardPad float32 = 30
	// cardSinceSize, cardMailSize, cardMailTop are `font-size:26px;font-weight:600`
	// and the email's `font-size:26px;margin-top:8px`.
	cardSinceSize, cardMailSize, cardMailTop float32 = 26, 26, 8
	// cardBadgesTop, cardBadgesBottom, cardBadgesX, cardBadgesSize are `.eb` at
	// `font-size:22px;margin:40px 50px 10px`.
	cardBadgesTop, cardBadgesBottom, cardBadgesX, cardBadgesSize float32 = 40, 10, 50, 22
	// cardMedalsPad is the row's `padding:0 30px`.
	cardMedalsPad float32 = 30
	// cardHintTop, cardHintSize are `font-size:22px;color:var(--tx3);margin-top:34px`.
	cardHintTop, cardHintSize float32 = 34, 22
	// cardHintBottom is the room kept under the hint when the badges leave no more
	// (not in the prototype, whose three medals leave 72).
	cardHintBottom float32 = 16
)

// Names layout.json knows the parts by: `div.d` of the name, `div.muted` of the
// email and `div.eb` are the capture's.
const (
	nameCardName   = "div.d"
	nameCardSince  = "tv.settings.since"
	nameCardMail   = "div.muted"
	nameCardBadges = "div.eb"
	nameCardHint   = "tv.settings.hint"
)

// settingsCard is the profile card of TV12: the header with the photo and the name,
// the "member since" line and the email, the badges and the hint.
type settingsCard struct {
	ui.Base

	photo  *kit.Avatar
	name   *kit.Label
	since  *kit.Label
	mail   *kit.Label
	badges *kit.Label
	medals []*tvkit.Medal
	row    *tvkit.MedalRow // the medals, in pages when they are many
	hint   *kit.Label
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newSettingsCard makes the card for the user, the day he joined ("" for none) and
// his badges.
func newSettingsCard(u model.User, since, badgesTitle, hint string, badges []model.Badge) *settingsCard {
	c := &settingsCard{}
	c.photo = kit.NewAvatar(cardPhoto, tokens.Dark.OnPrimary) // photo(200,'#141518')
	c.name = &kit.Label{
		Text: u.LastName + " " + u.FirstName, Role: theme.RoleDisplay, Size: cardNameSize, Weight: 800, LineHeight: 1,
		Spacing: homeDisplaySpacing, Upper: true, Ink: theme.InkOnPrimary,
	}
	c.name.Name = nameCardName
	c.name.Heading = 1 // the account, the title of the card
	c.since = &kit.Label{Text: since, Size: cardSinceSize, Weight: 600, Align: ui.TextAlignCenter, Ink: theme.InkText}
	c.since.Name = nameCardSince
	c.since.Hidden = since == ""
	c.mail = &kit.Label{Text: u.Email, Size: cardMailSize, Align: ui.TextAlignCenter, Ink: theme.InkText2}
	c.mail.Name = nameCardMail
	c.badges = kit.Eyebrow(badgesTitle)
	c.badges.Size = cardBadgesSize
	c.badges.Name = nameCardBadges
	c.hint = &kit.Label{Text: hint, Size: cardHintSize, Align: ui.TextAlignCenter, Ink: theme.InkText3}
	c.hint.Name = nameCardHint
	c.Add(c.photo, c.name, c.since, c.mail, c.badges)
	for _, b := range badges {
		m := tvkit.NewMedal(b.Name, accentOf(b.Code), medalSub(b.Code))
		c.medals = append(c.medals, m)
	}
	c.row = tvkit.NewMedalRow(c.medals)
	c.Add(c.row)
	c.Add(c.hint) // after the medals: the order of a reading is the order of the card

	return c
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Medals returns the badges, all of them, whatever page shows.
func (c *settingsCard) Medals() []*tvkit.Medal { return c.medals }

// Row returns the row of the badges.
func (c *settingsCard) Row() *tvkit.MedalRow { return c.row }

// Layout flows the card as CSS does.
func (c *settingsCard) Layout(ctx *ui.Context, cs ui.Constraints) geom.Size {
	size := cs.Constrain(geom.Sz(cardW, cardH))
	free := ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded))

	// the header: the photo over the name, centred on the 340
	ps := ctx.Layout(c.photo, ui.Tight(geom.Sz(cardPhoto, cardPhoto)))
	ns := ctx.Layout(c.name, free)
	y := (cardHeadH - (ps.H + cardHeadGap + ns.H)) / 2
	ctx.Place(c.photo, geom.Pt((size.W-ps.W)/2, y))
	y += ps.H + cardHeadGap
	ctx.Place(c.name, geom.Pt((size.W-ns.W)/2, y))

	// the centre block, 30 of padding: a line of text per row
	inner := ui.Constraints{Min: geom.Sz(size.W-2*cardPad, 0), Max: geom.Sz(size.W-2*cardPad, ui.Unbounded)}
	y = cardHeadH + cardPad
	if !skip(ctx, c.since) {
		ss := ctx.Layout(c.since, inner)
		ctx.Place(c.since, geom.Pt(cardPad, y))
		y += ss.H + cardMailTop
	}
	ms := ctx.Layout(c.mail, inner)
	ctx.Place(c.mail, geom.Pt(cardPad, y))
	y += ms.H + cardBadgesTop
	bw := size.W - 2*cardBadgesX
	bs := ctx.Layout(c.badges, ui.Constraints{Min: geom.Sz(bw, 0), Max: geom.Sz(bw, ui.Unbounded)})
	ctx.Place(c.badges, geom.Pt(cardBadgesX, y))
	y += bs.H + cardBadgesBottom

	// the medals, `justify-content:space-around` in the row's padding; more than the
	// prototype's three take turns in the room of three (K44)
	rw := size.W - 2*cardMedalsPad
	rs := ctx.Layout(c.row, ui.Constraints{Min: geom.Sz(rw, 0), Max: geom.Sz(rw, ui.Unbounded)})
	ctx.Place(c.row, geom.Pt(cardMedalsPad, y))
	row := rs.H
	y += row + cardHintTop
	hs := ctx.Layout(c.hint, ui.Constraints{Min: geom.Sz(size.W, 0), Max: geom.Sz(size.W, ui.Unbounded)})
	// the hint stays on the card, whatever the badges took (`overflow:hidden`)
	ctx.Place(c.hint, geom.Pt(0, min(y, size.H-cardHintBottom-hs.H)))

	return size
}

// Paint draws the card: --sf, its top 340 in --pr, the corners rounded, and what
// is on it (`overflow:hidden`).
func (c *settingsCard) Paint(ctx *ui.Context, p *ui.Painter) {
	look := theme.From(ctx)
	r := c.Bounds()
	p.Fill(r, scene.Solid(look.T.Surface), cardR)
	p.Fill(geom.XYWH(0, 0, r.W, cardHeadH+cardR), scene.Solid(look.T.Primary), cardR)
	p.Fill(geom.XYWH(0, cardHeadH, r.W, cardR), scene.Solid(look.T.Surface), 0)
	c.Base.Paint(ctx, p)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// badgeAccent is the colour of a badge's ribbon and rim, `BADGES` (index.html:311),
// by the API's badge code; a code it does not know gets the grey of the medal's
// "HDV".
var badgeAccent = map[string]color.Color{
	"hdv_dancefit": color.RGB(0x7a, 0xc9, 0x43), // BADGES[0]
	"hdv_stepx":    color.RGB(0xff, 0x7a, 0x1a), // BADGES[1]
	"hdv_bodyx":    color.RGB(0xe5, 0x38, 0x3b), // BADGES[2]
}

// accentOf returns the accent of the medal drawn for the badge code.
func accentOf(code string) color.Color {
	if c, ok := badgeAccent[code]; ok {
		return c
	}

	return color.RGB(0x8a, 0x8f, 0x98)
}

// medalSub is the small text under "HDV" on a medal drawn without a picture: the
// code without its "hdv_" prefix, upper case, as the prototype writes it.
func medalSub(code string) string {
	return upper(strings.TrimPrefix(code, "hdv_"))
}
