//go:build !tvos

package screens

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shownLaidOut checks that w is on the screen of tt: it has a frame and its text is
// painted.
func shownLaidOut(t *testing.T, tt *ui.Tester, w ui.Widget, text string) {
	t.Helper()
	tt.Elapse(kittest.Frame)
	f := w.Node().WindowFrame()
	assert.Positivef(t, f.W, "%q has a width", text)
	assert.Positivef(t, f.H, "%q has a height", text)
	assert.Truef(t, tt.Shows(text), "%q is painted", text)
}

// TestBlockStackLaysOutWhatWasHidden checks the stack of 16 and of 18: a part hidden
// takes no room and has no layout kept, and laid out when shown again, whether it was
// flipped with kit.SetHidden or by its Hidden field and a layout of the stack.
func TestBlockStackLaysOutWhatWasHidden(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := &kit.Label{Text: "alpha", Size: 16}
		b := &kit.Label{Text: "beta", Size: 16}
		d := &kit.Label{Text: "delta", Size: 16}
		body := &connectBody{}
		body.stack.items = []blockItem{{w: a}, {w: b, top: 8}, {w: d, top: 8}}
		for _, it := range body.stack.items {
			body.stack.Add(it.w)
		}
		body.Add(&body.stack)
		tt := kittest.Mount(t, ph, body)
		below := d.WindowFrame().Y
		shownLaidOut(t, tt, b, "beta")

		kit.SetHidden(b, true)
		tt.Elapse(kittest.Frame)
		assert.False(t, tt.Shows("beta"))
		assert.Less(t, d.WindowFrame().Y, below, "the room goes with it")
		assert.Zero(t, b.WindowFrame().H, "no frame kept")
		kit.SetHidden(b, false)
		shownLaidOut(t, tt, b, "beta")
		assert.Equal(t, below, d.WindowFrame().Y)

		// the field and a layout of the stack, as the screens did
		b.Hidden = true
		body.stack.Relayout()
		tt.Elapse(kittest.Frame)
		assert.Zero(t, b.WindowFrame().H, "the stack leaves no frame for what it skips")
		b.Hidden = false
		body.stack.Relayout()
		shownLaidOut(t, tt, b, "beta")
	})
}

// TestSheetColumnLaysOutWhatWasHidden is the same for the column of the download sheet.
func TestSheetColumnLaysOutWhatWasHidden(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := &kit.Label{Text: "alpha", Size: 16}
		b := &kit.Label{Text: "beta", Size: 16}
		d := &kit.Label{Text: "delta", Size: 16}
		col := &sheetColumn{items: []sheetItem{{w: a}, {w: b, before: 8}, {w: d, before: 8}}}
		col.Add(a, b, d)
		tt := kittest.Mount(t, ph, col)
		below := d.WindowFrame().Y
		shownLaidOut(t, tt, b, "beta")

		kit.SetHidden(b, true)
		tt.Elapse(kittest.Frame)
		assert.False(t, tt.Shows("beta"))
		assert.Less(t, d.WindowFrame().Y, below)
		assert.Zero(t, b.WindowFrame().H)
		kit.SetHidden(b, false)
		shownLaidOut(t, tt, b, "beta")
		assert.Equal(t, below, d.WindowFrame().Y)
	})
}

// TestProfileCardSinceComesAndGoes checks the "since" line of screen 15: left out
// with no day, laid out and painted when a day comes back.
func TestProfileCardSinceComesAndGoes(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		p := newProfileCard("", nil)
		u := model.User{FirstName: "An", LastName: "Nguyen", Email: "an@example.test"}
		p.set(u, "")
		tt := kittest.Mount(t, ph, p)
		require.True(t, p.since.Hidden)
		short := p.WindowFrame().H

		p.set(u, "Tham gia 2024")
		shownLaidOut(t, tt, p.since, "Tham gia 2024")
		assert.GreaterOrEqual(t, p.WindowFrame().H, short)

		p.set(u, "")
		tt.Elapse(kittest.Frame)
		assert.False(t, tt.Shows("Tham gia 2024"))

		p.set(u, "Tham gia 2025")
		shownLaidOut(t, tt, p.since, "Tham gia 2025")
	})
}

// TestChipsRowSwapsItsChips checks the chip of a downloaded file and the one of a
// streamed file, each laid out and painted every time it is shown.
func TestChipsRowSwapsItsChips(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newChipsRow("Đã tải 12 MB", "Phát trực tuyến")
		tt := kittest.Mount(t, ph, r)
		shownLaidOut(t, tt, r.stream, "Phát trực tuyến")
		for range 2 {
			r.show(true)
			shownLaidOut(t, tt, r.okc, "Đã tải 12 MB")
			assert.False(t, tt.Shows("Phát trực tuyến"))
			r.show(false)
			shownLaidOut(t, tt, r.stream, "Phát trực tuyến")
			assert.False(t, tt.Shows("Đã tải 12 MB"))
		}
	})
}
