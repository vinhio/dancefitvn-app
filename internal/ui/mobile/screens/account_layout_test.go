//go:build !tvos

package screens_test

import (
	"slices"
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAccountBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atAccount(t, kittest.Phones[0], true)
	checkAgainstProto(t, "M15", r.Dump("M15"), 1, nil)
}

func TestInfoBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, _ := atInfo(t, kittest.Phones[0], true)
	// The two name fields (labels, boxes, inputs) are the one deviation, K29: the prototype's second field runs
	// off the screen; TestInfoNameRowKeepsToTheColumnOnBothPhones checks it.
	checkAgainstProto(t, "M15B", r.Dump("M15B"), 1, map[string]bool{"label": true, "div.inp": true, "input": true, "label#2": true, "div.inp#2": true, "input#2": true})
}

func TestPasswordBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atAccount(t, kittest.Phones[0], true)
	tapKey(t, r, "sheet:password")
	slideIn(r)
	lay := r.Dump("M15B-password")
	// the title's box is text Core Text sets 1,3 narrower than Chrome (the kerning
	// row of PROGRESS section 5)
	checkAgainstProto(t, "M15B-password", lay, 1, map[string]bool{"span.d": true})
	w, _ := shelltest.ProtoLayout(t, "M15B-password").Find("span.d")
	g := element(t, lay, "span.d")
	assert.InDelta(t, w.W, g.W, 1.5)
}

// widerPhone checks a state of the 412 wide phone against the iPhone's (the login's
// way, PLAN D2: one layout): the same column from the top (y of every element within
// 1.5 DIPs, the engines' line boxes differing by a fraction), what spans the gutters
// 40 narrower than the screen is 372, and what sits at the right edge moved by the 19
// the screen is wider.
func widerPhone(t *testing.T, ios, android layoutdump.Layout, full, anchored []string, right float64) {
	t.Helper()
	require.Len(t, android.Elements, len(ios.Elements))
	for i, ea := range ios.Elements {
		eb := android.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		switch {
		case ea.Key == "div#m" || ea.Key == "div.scr" || ea.Key == "div.scroll": // the window's own boxes
			assert.InDelta(t, ea.H+40, eb.H, 1.5, ea.Key+": h")
		case slices.Contains(anchored, ea.Key): // the tab bar, the button: `bottom:` of the window
			assert.InDelta(t, ea.Y+40, eb.Y, 1.5, ea.Key+": y from the bottom")
			assert.InDelta(t, ea.H, eb.H, 1.5, ea.Key+": h")
		default:
			assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
			assert.InDelta(t, ea.H, eb.H, 1.5, ea.Key+": h")
		}
		if ea.X+ea.W == right {
			assert.InDelta(t, ea.X+ea.W+19, eb.X+eb.W, 0.5, ea.Key+": moves with the right edge")
		}
	}
	for _, key := range full {
		e := element(t, android, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 412-40, e.W, 0.01, key+": width")
	}
}

// tabBarKeys are the parts of the tab bar in the dump of screen 15.
var tabBarKeys = []string{"div.tabs", "go.home", "go.courses", "go.downloads#2", "go.account",
	"span.pill", "span.pill#2", "span.pill#3", "span.pill#4", "svg.i#14", "svg.i#15", "svg.i#16", "svg.i#17"}

// TestAccountOnTheWiderPhone checks screen 15 on the 412 wide phone against the
// iPhone's.
func TestAccountOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	a := atAccount(t, kittest.Phones[0], true).Dump("M15")
	b := atAccount(t, kittest.Phones[1], true).Dump("M15")
	widerPhone(t, a, b, []string{"div.h1", "div.eb", "div.eb#2", "div.eb#3", "div.card", "div.card#2", "sheet:logout", "p.muted"}, tabBarKeys, 373)
	// the three badges share the 372 in thirds with the gaps of 10: 117.33 each
	for i, key := range []string{"div.badge", "div.badge#2", "div.badge#3"} {
		e := element(t, b, key)
		assert.InDelta(t, 117.33, e.W, 0.01, key+": a third of the room")
		assert.InDelta(t, 20+float64(i)*127.33, e.X, 0.02, key+": x")
	}
}

// TestInfoOnTheWiderPhone checks screen 15B on the 412 wide phone: the pair of name
// fields is half of the 372 less the gap (K29), the other fields and the note are
// 372, the button is 372 wide over the end of the page.
func TestInfoOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	ra, _ := atInfo(t, kittest.Phones[0], true)
	rb, _ := atInfo(t, kittest.Phones[1], true)
	a, b := ra.Dump("M15B"), rb.Dump("M15B")
	widerPhone(t, a, b, []string{"div.top", "div.inp#3", "div.inp#4", "sheet:password", "div.card", "button.btn.p"}, []string{"button.btn.p", "svg.i#7"}, 373)
	assert.InDelta(t, element(t, a, "div.inp").X, element(t, b, "div.inp").X, 0.01, "div.inp: x")
	assert.InDelta(t, 180, element(t, b, "div.inp").W, 0.01, "div.inp: half of the 372 less the 12 gap")
	assert.InDelta(t, 20+180+12, element(t, b, "div.inp#2").X, 0.01, "div.inp#2: x")
	assert.InDelta(t, 180, element(t, b, "div.inp#2").W, 0.01, "div.inp#2: the width")
	assert.InDelta(t, 892-38-54, element(t, b, "button.btn.p").Y, 0.01, "bottom:38px")
}

// TestPasswordOnTheWiderPhone checks SH.password on the 412 wide phone: the panel is as
// wide as the window and rests on its bottom, the fields and the button are 372, the
// cross is at the right edge.
func TestPasswordOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	ra := atAccount(t, kittest.Phones[0], true)
	tapKey(t, ra, "sheet:password")
	slideIn(ra)
	rb := atAccount(t, kittest.Phones[1], true)
	tapKey(t, rb, "sheet:password")
	slideIn(rb)
	a, b := ra.Dump("M15B-password"), rb.Dump("M15B-password")
	pa, pb := element(t, a, "div.sheet"), element(t, b, "div.sheet")
	assert.InDelta(t, 412, pb.W, 0.01)
	assert.InDelta(t, 892, pb.Y+pb.H, 0.01, "the panel rests on the bottom")
	assert.InDelta(t, pa.H, pb.H, 1.5, "as tall as on the iPhone")
	for _, key := range []string{"div.sh", "div.inp", "div.inp#2", "div.inp#3", "pwSave"} {
		e := element(t, b, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 372, e.W, 0.01, key+": width")
	}
	cross := element(t, b, "closeSheet#2")
	assert.InDelta(t, 412-20-32, cross.X, 0.01, "the cross is at the right edge")
	// the same order of parts from the top of the panel
	for _, key := range []string{"div.grab", "div.sh", "label", "div.inp", "label#2", "div.inp#2", "label#3", "div.inp#3", "pwSave"} {
		ea, eb := element(t, a, key), element(t, b, key)
		assert.InDelta(t, ea.Y-pa.Y, eb.Y-pb.Y, 1.5, key+": distance from the top of the panel")
	}
}

// TestLogoutBoxesMatchPrototype lays SH.logout over the real screen 15 and compares M15-logout
// with the prototype within 1 DIP.
func TestLogoutBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atAccount(t, kittest.Phones[0], true)
	r.Shell.AskLogout()
	r.Settle()
	slideIn(r)
	checkAgainstProto(t, "M15-logout", r.Dump("M15-logout"), 1, map[string]bool{"div.d#2": true, "p.muted#2": true})
	want := shelltest.ProtoLayout(t, "M15-logout")
	lay := r.Dump("M15-logout")
	for _, key := range []string{"div.d#2", "p.muted#2"} { // text Core Text sets up to 1,3 narrower than Chrome
		w, _ := want.Find(key)
		g := element(t, lay, key)
		assert.InDelta(t, w.Y, g.Y, 1, key+": y")
		assert.InDelta(t, w.W, g.W, 1.5, key+": w")
	}
}
