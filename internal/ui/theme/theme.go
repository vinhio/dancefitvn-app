// Adapted from jplayer-mobile/internal/ui/theme/theme.go at 49de3c2.

package theme

import (
	"dancefitvn-app/internal/tokens"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Look is the app's extension of ui.Theme: the one Night Stage look. It has
// no skins and no motion presets (PLAN Q1), so unlike jPlayer's it is built
// once and never changes. Install puts it in the theme with Theme.SetExt,
// From reads it.
type Look struct {
	// T is the palette of reference/design/tokens.json, the :root of the
	// prototype (index.html:10).
	T tokens.Palette
	// Faces names the faces of the three font roles for the text engine
	// this program draws with.
	Faces Faces
	// C holds the literal colors the prototype's CSS writes outside :root.
	C Colors
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// bodySize is the page's text size: `html,body` set none, so the 16 px of
// the browser applies (index.html:12-15, layout.json fs 16px on every
// unstyled box).
const bodySize float32 = 16

// fallback is the Look From returns for a view that installed none, built
// once so that reading it does not allocate.
var fallback = newLook(NamingUnknown)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// New returns the app's theme: gDesk's dark theme with the Night Stage
// colors and the Look installed, its default face Inter 16 / 400 named for
// naming (NamingUnknown reads as DefaultNaming).
func New(naming Naming) *ui.Theme {
	th := ui.DarkTheme()
	Install(th, naming)

	return th
}

// Install makes the Night Stage look the theme's: the colors and the text
// face that stock gDesk widgets read, and a Look in its extension.
//
// The default face is the page's own, Inter 16 / 400 (index.html:12, F-2 and
// F-14 of local/plans/P2.md): a Core Text face that names no family draws in
// SF, so a text that sets none must still fall on Inter.
func Install(th *ui.Theme, naming Naming) {
	l := newLook(naming)
	th.SetExt(l)

	p := &l.T
	th.Background = p.Bg
	th.Surface = p.Surface
	th.SurfaceHover = p.Surface2
	th.SurfacePress = p.Surface2
	th.Text = p.Text
	th.Muted = p.Text2
	th.Accent = p.Primary
	th.AccentHover = p.Primary
	th.AccentPress = p.Primary
	th.OnAccent = p.OnPrimary
	th.Border = p.Border
	th.Radius = tokens.RadiusMd
	th.Font = l.Faces.Face(RoleBody, bodySize, 400)
}

// From returns the Look installed in the context's theme, or the app's
// default Look when none is. It does not allocate.
func From(c *ui.Context) *Look {
	if l, ok := c.Theme().Ext[*Look](); ok && l != nil {
		return l
	}

	return fallback
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// newLook returns the Look for the text engine named by naming.
func newLook(naming Naming) *Look {
	return &Look{T: tokens.Dark, Faces: NewFaces(naming), C: literals}
}
