package kit

import (
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The numbers of .top (index.html:83, :477).
const (
	// topHeight is `.top{height:52px}`.
	topHeight float32 = 52
	// topTitleSize is `<b style="font-size:17px">` (:477); the bold is 700.
	topTitleSize float32 = 17
	// topSpacer is `<span style="width:40px">`, the empty end that keeps the
	// title in the middle (:477).
	topSpacer float32 = 40
)

// The names layout.json gives the top bar and its title.
const (
	nameTop     = "div.top"
	nameTopBack = "back"
	// nameTopTitle is empty: the title is a `<b>` the capture does not record
	// (M15B names the `b` of the rows only).
	nameTopTitle = ""
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// TopBar is `.top` (index.html:83, :477): the 52 tall row of a screen with a
// back button, the title and an empty 40 DIP end, `space-between`: the 40 round
// back button (`.rb`, a left chevron) at the start, the title in 17/700 in the
// middle, centered on the row.
type TopBar struct {
	ui.Base
	// Title is the screen's title.
	Title string
	// BackLabel names the back button for assistive technologies.
	BackLabel string
	// OnBack is called when the back button is pressed.
	OnBack func()

	back     RoundButton
	title    Label
	attached bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// NewTopBar returns a top bar with a title, the back button's label and its
// action.
func NewTopBar(title, backLabel string, onBack func()) *TopBar {
	return &TopBar{Title: title, BackLabel: backLabel, OnBack: onBack}
}

// SetTitle changes the title, laying the bar out again; the same text changes
// nothing.
func (t *TopBar) SetTitle(s string) {
	if s != t.Title {
		t.Title = s
		t.Relayout()
	}
}

// Layout takes the width it gets and 52 of height; the back button is at the
// start and the title between it and the empty end, centered on the row.
func (t *TopBar) Layout(c *ui.Context, cs ui.Constraints) geom.Size {
	if !t.attached {
		t.attached = true
		t.Name = nameTop
		t.back.Name, t.back.Icon = nameTopBack, icon.Cl
		t.title.Name = nameTopTitle
		t.back.OnPress = t.pressBack
		t.Add(&t.back, &t.title)
	}
	t.back.Label = t.BackLabel
	labelStyle{Role: theme.RoleBody, Size: topTitleSize, Weight: 700, Ink: theme.InkText, Ellipsis: true}.apply(&t.title)
	t.title.Heading = 1 // the screen's title, as the h1 of a page
	t.title.SetText(t.Title)
	w := cs.Max.W
	if w >= ui.Unbounded {
		w = 0
	}
	size := cs.Constrain(geom.Sz(w, topHeight))
	bs := c.Layout(&t.back, ui.Loose(geom.Sz(ui.Unbounded, ui.Unbounded)))
	c.Place(&t.back, geom.Pt(0, centeredY(size.H, bs.H)))
	room := max(size.W-bs.W-topSpacer, 0)
	ts := c.Layout(&t.title, ui.Loose(geom.Sz(room, ui.Unbounded)))
	c.Place(&t.title, geom.Pt(bs.W+(room-ts.W)/2, centeredY(size.H, ts.H)))

	return size
}

// pressBack forwards the back button's press to OnBack.
func (t *TopBar) pressBack() {
	if t.OnBack != nil {
		t.OnBack()
	}
}
