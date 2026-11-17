// Adapted from jplayer-mobile/internal/ui/gesture/exempt_test.go at 49de3c2.

package gesture

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// exemptBox is a box that is exempt while its flag is set, and may catch
// presses in a narrower place than its frame.
type exemptBox struct {
	ui.Box
	exempt bool
	hit    func(p geom.Point) bool
}

func (b *exemptBox) DragExempt() bool { return b.exempt }

func (b *exemptBox) HitTest(p geom.Point) bool { return b.hit == nil || b.hit(p) }

// exemptFix is a sheet body with the widgets the tests press.
type exemptFix struct {
	tt                         *ui.Tester
	body                       *exemptBox
	rng, plain, input, sliding *exemptBox
}

func newExemptFix(t *testing.T) *exemptFix {
	t.Helper()
	// A sheet body of 200 x 400 DIPs at (50, 100): a plain box in a column
	// with a range at y 20-60, and an input at 100-160 that is hidden
	// first, a box that is exempt only while sliding at 200-260.
	body := &exemptBox{}
	body.Width, body.Height = 200, 400
	rng := &exemptBox{exempt: true}
	rng.Width, rng.Height = 200, 40
	input := &exemptBox{exempt: true}
	input.Width, input.Height = 200, 60
	sliding := &exemptBox{}
	sliding.Width, sliding.Height = 200, 60
	plain := &exemptBox{}
	plain.Width, plain.Height = 200, 40
	col := &ui.Flex{Vertical: true, Gap: 0}
	col.Padding = geom.Insets{Top: 20}
	col.Add(rng, plain, input, sliding)
	body.Add(col)
	root := &ui.Box{Padding: geom.Insets{Left: 50, Top: 100}}
	root.Add(body)
	tt, err := ui.NewTester(root, 400, 600)
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	tt.Repaint()

	return &exemptFix{tt: tt, body: body, rng: rng, plain: plain, input: input, sliding: sliding}
}

func TestExempted(t *testing.T) {
	t.Parallel()

	f := newExemptFix(t)
	body, rng, input, sliding := f.body, f.rng, f.input, f.sliding
	at := func(x, y float32) geom.Point { return geom.Pt(x, y) }
	tests := []struct {
		name string
		p    geom.Point
		want bool
	}{
		{"on the range", at(100, 100+20+10), true},
		{"on a plain row", at(100, 100+20+40+10), false},
		{"on the input", at(100, 100+20+80+10), true},
		{"on the box that is not sliding", at(100, 100+20+140+10), false},
		{"outside the body", at(10, 130), false},
		{"in the body's empty part", at(100, 100+380), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Exempted(body, tc.p))
		})
	}

	t.Run("a row sliding is exempt", func(t *testing.T) {
		sliding.exempt = true
		assert.True(t, Exempted(body, at(100, 100+20+140+10)))
	})
	t.Run("a hidden widget is not", func(t *testing.T) {
		input.Hidden = true
		f.tt.Repaint()
		assert.False(t, Exempted(body, at(100, 100+20+80+10)))
		input.Hidden = false
	})
	t.Run("a widget whose hit test refuses the place is not", func(t *testing.T) {
		rng.hit = func(geom.Point) bool { return false }
		assert.False(t, Exempted(body, at(100, 100+20+10)))
		rng.hit = nil
	})
	t.Run("an ancestor of the hit widget counts, up to the root", func(t *testing.T) {
		body.exempt = true
		assert.True(t, Exempted(body, at(100, 100+20+40+10)))
		body.exempt = false
	})
	t.Run("a nil root", func(t *testing.T) {
		assert.False(t, Exempted(nil, at(0, 0)))
	})
}

func TestExemptedDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	f := newExemptFix(t)
	assert.Zero(t, testing.AllocsPerRun(100, func() { Exempted(f.body, geom.Pt(100, 100+20+80+10)) }))
	assert.Zero(t, testing.AllocsPerRun(100, func() { Exempted(f.body, geom.Pt(100, 100+20+40+10)) }))
}
