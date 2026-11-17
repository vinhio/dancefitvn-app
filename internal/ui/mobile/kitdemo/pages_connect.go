//go:build !tvos

package kitdemo

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/theme"
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// pageM16 is the Apple TV pairing screen with no code typed (MS.connect,
// index.html:497-502): the top bar, the six boxes and the disabled button.
func pageM16() Page {
	return Page{ID: "M16", Title: "Pair the TV", Build: func(d *Data) Screen { return screen(d.connect("")) }}
}

// pageM16Code is the same screen with the code typed in.
func pageM16Code() Page {
	return Page{ID: "M16-code", Title: "Pair the TV, code typed", Build: func(d *Data) Screen { return screen(d.connect(pairCode)) }}
}

// pairCode is the code the prototype's tester types (`7K4M2P`, :501).
const pairCode = "7K4M2P"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// connect lays out M16 at the boxes of M16.layout.json with the code typed.
func (d *Data) connect(code string) *canvas {
	cv := newCanvas()
	cv.at(kit.NewTopBar(d.T("account.connect"), d.T("a11y.back"), nil), 20, 54, 353)
	boxes := kit.NewCodeBoxes(d.T("connect.codeLabel"))
	boxes.SetValue(code)
	cv.at(boxes, 20, 372, 353)

	// the expiry line (:501) under the boxes: its `ic('clock',15)` is the only part drawn.
	cv.centre(kit.NewIcon(icon.Clock, 15, theme.InkText2), 71.09, 448, 15)

	link := kit.NewButton(kit.ButtonP, d.T("connect.button"), icon.Tv)
	link.Name = "linkTV"
	link.SetDisabled(len(code) < kit.CodeLength) // `style="opacity:.45"` (:502)
	cv.pin(link, 20, 760, 353)

	return cv
}
