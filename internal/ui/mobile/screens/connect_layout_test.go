//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
)

// hint is the line "Mã hết hạn sau 04:18 · gợi ý: 7K4M2P" of M16 and the clock before it
// that the app does not draw (K24): the phone cannot know the code's expiry, and the
// prototype's "gợi ý" is a tester's aid. The clock is `svg.i#2` of the prototype and
// the button's icon `svg.i#3`; here the button's is `svg.i#2`.
var hint = map[string]bool{"p.muted#2": true, "svg.i#2": true, "svg.i#3": true}

// TestConnectBoxesMatchPrototype compares M16 (empty), M16-code (7K4M2P typed) and
// M16-linked with the prototype's boxes within 1 DIP on the iPhone's size; the lines
// of text Core Text sets a little narrower than Chrome are compared by position.
func TestConnectBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	t.Run("M16", func(t *testing.T) {
		r, _ := atConnect(t, kittest.Phones[0], true)
		lay := r.Dump("M16")
		checkAgainstProto(t, "M16", lay, 1, hint)
		assert.False(t, hasKey(lay, "p.muted#2"), "K24: no expiry line")
		// the button's icon is the prototype's svg.i#3
		want, _ := shelltest.ProtoLayout(t, "M16").Find("svg.i#3")
		got := element(t, lay, "svg.i#2")
		assert.InDelta(t, want.X, got.X, 1)
		assert.InDelta(t, want.Y, got.Y, 1)
		assert.Equal(t, "span.codebox.f", element(t, lay, "span.codebox.f").Key)
	})
	t.Run("M16-code", func(t *testing.T) {
		r, c := atConnect(t, kittest.Phones[0], true)
		typeCode(r, c, "7K4M2P")
		r.Tester.Elapse(kittest.Frame)
		checkAgainstProto(t, "M16-code", r.Dump("M16-code"), 1, hint)
	})
	t.Run("M16-linked", func(t *testing.T) {
		r, _ := atConnectLinked(t, kittest.Phones[0], true)
		checkAgainstProto(t, "M16-linked", r.Dump("M16-linked"), 1, nil)
	})
}

// TestConnectOnTheWiderPhone checks the three states on the 412 wide phone against the
// iPhone's (PLAN D2: one layout): the same column from the top, the content centred, the
// button 372 wide and `bottom:38px` of the taller window.
func TestConnectOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	states := []struct {
		id     string
		open   func(testing.TB, int) *shelltest.Rig
		full   []string
		anchor []string
		right  float64
	}{
		{"M16", func(tb testing.TB, i int) *shelltest.Rig { r, _ := atConnect(tb, kittest.Phones[i], true); return r },
			[]string{"div.top", "div.d", "p.muted", "input"}, []string{"linkTV", "svg.i#2"}, 373},
		{"M16-linked", func(tb testing.TB, i int) *shelltest.Rig {
			r, _ := atConnectLinked(tb, kittest.Phones[i], true)
			return r
		},
			[]string{"div.top", "div.d", "p.muted", "unlinkTV"}, nil, 373},
	}
	for _, st := range states {
		a, b := st.open(t, 0).Dump(st.id), st.open(t, 1).Dump(st.id)
		widerPhone(t, a, b, st.full, st.anchor, st.right)
		// what is centred is 9.5 further right (half the 19 the window is wider)
		for _, key := range []string{"div.eb", "div.d.acc", "span.codebox"} {
			if !hasKey(a, key) {
				continue
			}
			ea, eb := element(t, a, key), element(t, b, key)
			assert.InDelta(t, ea.X+9.5, eb.X, .6, st.id+" "+key+": stays in the middle")
		}
	}
	r, _ := atConnect(t, kittest.Phones[1], true)
	assert.InDelta(t, 892-38-54, element(t, r.Dump("x"), "linkTV").Y, .01, "bottom:38px")
}
