package theme

import "github.com/gdeskdev/desk/foundation/core/color"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Ink is a color of the palette a text or an icon is drawn in: the CSS
// variables the prototype colors text with (--tx, --tx2, --tx3, --pr, --on,
// --dg, --ok, --info, --warn) and #fff. Widgets keep the Ink, not the color,
// and read it when they paint.
type Ink uint8

// The inks. InkUnknown reads as InkText, the body color (index.html:13
// `color:var(--tx)`).
const (
	InkUnknown Ink = iota
	// InkText is --tx.
	InkText
	// InkText2 is --tx2, the muted text.
	InkText2
	// InkText3 is --tx3, the faint text.
	InkText3
	// InkPrimary is --pr, the yellow.
	InkPrimary
	// InkOnPrimary is --on, the text over --pr.
	InkOnPrimary
	// InkDanger is --dg.
	InkDanger
	// InkSuccess is --ok.
	InkSuccess
	// InkInfo is --info.
	InkInfo
	// InkWarning is --warn.
	InkWarning
	// InkWhite is #fff, as `.btn.r` and `.chip.k` color text.
	InkWhite
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Of returns the color of the ink in the look. It does not allocate.
func (i Ink) Of(l *Look) color.Color {
	switch i {
	case InkText2:
		return l.T.Text2
	case InkText3:
		return l.T.Text3
	case InkPrimary:
		return l.T.Primary
	case InkOnPrimary:
		return l.T.OnPrimary
	case InkDanger:
		return l.T.Danger
	case InkSuccess:
		return l.T.Success
	case InkInfo:
		return l.T.Info
	case InkWarning:
		return l.T.Warning
	case InkWhite:
		return color.White
	case InkUnknown, InkText:
		return l.T.Text
	default:
		return l.T.Text
	}
}
