package kit_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetHidden checks that a part hidden and shown again, inside a container whose
// layout is cached, takes its room and its frame back: the flip of Base.Hidden alone
// leaves the cached layout of its parents as it was (P6.6: the code boxes of 16 had a
// 0x0 frame after "Ngắt kết nối").
func TestSetHidden(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := &kit.Label{Text: "alpha", Size: 16}
		b := &kit.Label{Text: "beta", Size: 16}
		inner := ui.Column(0, b)
		outer := ui.Column(0, a, inner)
		tt := kittest.Mount(t, ph, outer)
		require.True(t, tt.Shows("beta"), "laid out and painted")
		full := inner.WindowFrame().H
		require.Positive(t, full)

		// hidden: the room goes with it
		assert.True(t, kit.SetHidden(b, true))
		tt.Elapse(kittest.Frame)
		assert.False(t, tt.Shows("beta"))
		assert.Zero(t, inner.WindowFrame().H, "the container has no room for it")

		// shown again: laid out afresh, painted
		assert.True(t, kit.SetHidden(b, false))
		tt.Elapse(kittest.Frame)
		assert.True(t, tt.Shows("beta"))
		assert.Equal(t, full, inner.WindowFrame().H)
		assert.Positive(t, b.WindowFrame().W)
		assert.Positive(t, b.WindowFrame().H)

		// the same state again changes nothing
		assert.False(t, kit.SetHidden(b, false))
	})
}

// TestSetHiddenDoesNotAllocate checks that setting the state a part has costs no
// allocation.
func TestSetHiddenDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	l := &kit.Label{Text: "x", Size: 16}
	assert.Zero(t, testing.AllocsPerRun(100, func() {
		kit.SetHidden(l, false)
	}))
}
