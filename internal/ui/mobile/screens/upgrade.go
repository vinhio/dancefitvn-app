//go:build !tvos

package screens

import (
	"runtime"

	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The measures of the upgrade screen (docs/plan/03 section 9; the prototype has
// none, so they follow the login's: a `.d` title, a 14 muted body with the 1.45
// line height, the 54 button).
const (
	upgradeTop, upgradeTitleSize      float32 = 120, 28
	upgradeBodyTop, upgradeBodySize   float32 = 12, 14
	upgradeBodyLine, upgradeButtonTop float32 = 1.45, 28
)

// Upgrade is the screen of an app the server no longer accepts (HTTP 426 or
// `force_update`): the title, a sentence and the button that opens the store. It
// has no tab bar and no way past it.
type Upgrade struct {
	ui.Base

	e    *env.Env
	page *nav.Page
	url  map[string]string
	open *kit.Button
}

var (
	_ nav.Tabbed   = (*Upgrade)(nil)
	_ nav.Scroller = (*Upgrade)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewUpgrade returns the upgrade screen.
func NewUpgrade(e *env.Env, p env.UpgradeParams) *Upgrade {
	u := &Upgrade{e: e, url: p.StoreURL}
	body := kit.Muted(e.T("upgrade.body"), upgradeBodySize)
	body.LineHeight, body.Wrap = upgradeBodyLine, true
	u.open = kit.NewButton(kit.ButtonP, e.T("upgrade.button"), icon.NameUnknown)
	u.open.OnPress = u.openStore
	col := ui.Column(0,
		&ui.Box{Height: upgradeTop},
		heading(kit.Display(e.T("upgrade.title"), upgradeTitleSize), 1),
		&ui.Box{Height: upgradeBodyTop}, body,
		&ui.Box{Height: upgradeButtonTop}, u.open,
	)
	col.Align = ui.CrossStretch
	u.page = nav.NewPage(col, -1)
	u.Add(u.page)

	return u
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Button returns the update button.
func (u *Upgrade) Button() *kit.Button { return u.open }

// Tab is -1: no tab bar.
func (u *Upgrade) Tab() int { return -1 }

// Scroll returns the page's scroll.
func (u *Upgrade) Scroll() *ui.Scroll { return u.page.Scroll() }

// Layout fills the window with the page.
func (u *Upgrade) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	c.Layout(u.page, ui.Tight(cs.Max))
	c.Place(u.page, geom.Point{})

	return cs.Max
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// openStore opens the store page of this platform from the 426's body, else the
// website of the app config.
func (u *Upgrade) openStore() {
	if s := u.url[storeKey()]; s != "" {
		u.e.OpenURL(s)

		return
	}
	if cfg, ok := u.e.Cfg(); ok && cfg.Support.Website != "" {
		u.e.OpenURL(cfg.Support.Website)
	}
}

// storeKey is the key of the store page in the 426's body for this system:
// "android", else "ios" (a Mac running the tests is the closest to an iPhone).
func storeKey() string {
	if runtime.GOOS == "android" {
		return "android"
	}

	return "ios"
}
