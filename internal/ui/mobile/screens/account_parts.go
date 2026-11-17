//go:build !tvos

// Adapted from dancefitvn-app/internal/ui/mobile/kitdemo/pages_account.go (the
// boxes of M15 that the demo lays out at absolute positions, here the flow that
// gives them: the same numbers, each from its selector in index.html:463-475).

package screens

import (
	"strings"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/tokens"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of the profile card, in DIPs (index.html:464).
const (
	// profPad, profGap, profRadius are `padding:16px;gap:14px;border-radius:20px`.
	profPad, profGap, profRadius float32 = 16, 14, 20
	// profNameSize is `.d` at `font-size:22px`.
	profNameSize float32 = 22
	// profSinceTop, profSinceSize are `font-size:12px;font-weight:600;margin-top:6px`.
	profSinceTop, profSinceSize float32 = 6, 12
	// profMailTop, profMailSize, profMailAlpha are `font-size:12px;opacity:.8;margin-top:2px`.
	profMailTop, profMailSize float32 = 2, 12
	profMailAlpha             float32 = .8
	// profAvatar is `photo(76,'#141518')`.
	profAvatar float32 = 76
)

// nameAvatarBtn is `data-act="sheet" data-v="avatar"`, what layout.json calls the avatar button.
const nameAvatarBtn = "sheet:avatar"

// badgeGap is `display:flex;gap:10px` of the row of badges (:468).
const badgeGap float32 = 10

// badgeCols is the most badges on a line, the prototype's three (`BADGES`).
const badgeCols = 3

// badgeAccent is the colour of a badge's ribbon and rim, `BADGES` (index.html:311),
// by the API's badge code; a code it does not know gets badgeAccentOther (the grey
// of the medal's "HDV").
var badgeAccent = map[string]color.Color{
	"hdv_dancefit": color.RGB(0x7a, 0xc9, 0x43), // BADGES[0]
	"hdv_stepx":    color.RGB(0xff, 0x7a, 0x1a), // BADGES[1]
	"hdv_bodyx":    color.RGB(0xe5, 0x38, 0x3b), // BADGES[2]
}

// badgeAccentOther is the accent of a badge with a code that is not in badgeAccent.
var badgeAccentOther = color.RGB(0x8a, 0x8f, 0x98)

// profileCard is the box of the avatar and the three lines of text (:464): `--pr`
// 20 rounded, the avatar button at the start, the name, the "since" line and the
// email centred on its height.
type profileCard struct {
	ui.Base

	avatar            *kit.AvatarButton
	name, since, mail *kit.Label
}

// badgeRow is the row of badges (:468): `display:flex;gap:10px`, each `flex:1`, as
// tall as the tallest.
type badgeRow struct {
	ui.Base

	badges []*kit.Badge
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newProfileCard returns the card; onAvatar is the avatar button's tap
// (`data-act="sheet" data-v="avatar"`).
func newProfileCard(avatarLabel string, onAvatar func()) *profileCard {
	p := &profileCard{}
	// `photo(76,'#141518')`: the ring is --on
	p.avatar = kit.NewAvatarButton(profAvatar, tokens.Dark.OnPrimary, avatarLabel)
	p.avatar.Name, p.avatar.Camera, p.avatar.OnPress = nameAvatarBtn, true, onAvatar
	p.name = kit.Display("", profNameSize)
	p.name.Ink = theme.InkOnPrimary
	p.since = &kit.Label{Size: profSinceSize, Weight: 600, Ink: theme.InkOnPrimary}
	// `opacity:.8` on a line of --on text: the colour with that alpha
	p.mail = &kit.Label{Size: profMailSize, Ellipsis: true, Color: tokens.Dark.OnPrimary.WithAlpha(profMailAlpha)}
	p.Add(p.avatar, p.name, p.since, p.mail)

	return p
}

// newBadgeRow returns an empty row.
func newBadgeRow() *badgeRow { return &badgeRow{} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// set shows the user: the name as the prototype writes it ("{last} {first}"), the
// day the user joined and the email; the "since" line is left out with no day.
func (p *profileCard) set(u model.User, sinceText string) {
	name := strings.TrimSpace(u.LastName + " " + u.FirstName)
	if name == "" {
		name = u.FullName
	}
	p.name.SetText(name)
	p.since.SetText(sinceText)
	kit.SetHidden(p.since, sinceText == "")
	p.mail.SetText(u.Email)
}

// Layout puts the avatar at the start and the text beside it, centred on the
// avatar's height.
func (p *profileCard) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	w := cs.Max.W
	as := c.Layout(p.avatar, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(p.avatar, geom.Pt(profPad, profPad))
	x := profPad + as.W + profGap
	room := max(w-profPad-x, 0)
	loose := ui.Loose(geom.Sz(room, ui.Unbounded))
	// the text is a column (`min-width:0`) as wide as its widest line, and each line
	// is a block of that width: the name's box is as wide as the "since" line
	colW := c.Layout(p.name, loose).W
	if !p.since.Hidden {
		colW = max(colW, c.Layout(p.since, loose).W)
	}
	colW = min(max(colW, c.Layout(p.mail, loose).W), room)
	block := ui.Constraints{Min: geom.Sz(colW, 0), Max: geom.Sz(colW, ui.Unbounded)}
	ns := c.Layout(p.name, block)
	col := ns.H
	var ss, ms geom.Size
	if !p.since.Hidden {
		ss = c.Layout(p.since, block)
		col += profSinceTop + ss.H
	}
	ms = c.Layout(p.mail, block)
	col += profMailTop + ms.H
	h := max(as.H, col)
	y := profPad + (h-col)/2
	c.Place(p.name, geom.Pt(x, y))
	y += ns.H
	if !p.since.Hidden {
		y += profSinceTop
		c.Place(p.since, geom.Pt(x, y))
		y += ss.H
	}
	y += profMailTop
	c.Place(p.mail, geom.Pt(x, y))

	return cs.Constrain(geom.Sz(w, h+2*profPad))
}

// Paint fills the card with --pr.
func (p *profileCard) Paint(c *ui.Context, pt *ui.Painter) {
	pt.Fill(p.Bounds(), scene.Solid(theme.From(c).T.Primary), profRadius)
	p.Base.Paint(c, pt)
}

// set replaces the badges.
func (r *badgeRow) set(badges []*kit.Badge) {
	for _, b := range r.badges {
		r.Remove(b)
	}
	r.badges = badges
	for _, b := range badges {
		r.Add(b)
	}
	r.Relayout()
}

// Layout shares the width between the badges, the gaps kept, and makes those of a
// line as tall as the tallest (`align-items:stretch`). The prototype has three: more
// than badgeCols go on further lines of the same cells (a row of five at 393 would
// leave a cell narrower than its 72 DIP medal), one gap apart.
func (r *badgeRow) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	n := len(r.badges)
	w := cs.Max.W
	if n == 0 {
		return cs.Constrain(geom.Sz(w, 0))
	}
	cols := min(n, badgeCols)
	each := max((w-float32(cols-1)*badgeGap)/float32(cols), 0)
	var y float32
	for from := 0; from < n; from += cols {
		line := r.badges[from:min(from+cols, n)]
		var h float32
		for _, b := range line {
			h = max(h, c.Layout(b, ui.Constraints{Max: geom.Sz(each, ui.Unbounded)}).H)
		}
		for i, b := range line {
			c.Layout(b, ui.Tight(geom.Sz(each, h)))
			c.Place(b, geom.Pt(float32(i)*(each+badgeGap), y))
		}
		y += h + badgeGap
	}

	return cs.Constrain(geom.Sz(w, y-badgeGap))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// accentOf returns the accent of the medal drawn for the badge code.
func accentOf(code string) color.Color {
	if c, ok := badgeAccent[code]; ok {
		return c
	}

	return badgeAccentOther
}

// medalSub is the small text under "HDV" on a medal drawn without a picture: the
// code without its "hdv_" prefix, as the prototype writes it (`BADGES`).
func medalSub(code string) string { return strings.ToUpper(strings.TrimPrefix(code, "hdv_")) }
