//go:build !tvos

package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestPerfFlinger flings the list of Courses (M04) twice on both phones:
// the first fling moves it down the list and glides on after the finger,
// the second brings it back up; the run logs its start and end.
func TestPerfFlinger(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, nil, "")
		r.mount(ph, "M04", false)
		file := filepath.Join(must(r.deps.CacheDir()), shotFolder, "M04.layout.json")
		r.drive("the layout of M04", func() bool {
			_, err := os.Stat(file)

			return err == nil
		})
		s := firstScroll(r.sh)
		require.NotNil(t, s, "the list of Courses scrolls")
		f := newPerfFlinger("T", r.tt.View(), 2, flingLong, r.log.add)
		f.start()
		lifted := s.Offset().Y
		assert.Positive(t, lifted, "the finger moved the list")
		r.drive("the first glide", func() bool { return f.next == 2 })
		r.drive("the flings", f.Done)
		assert.True(t, hasLine(r.log.all(), "dfv: perf scroll start T t="), "%v", r.log.all())
		assert.True(t, hasLine(r.log.all(), "dfv: perf scroll end T t="), "%v", r.log.all())
		for range 60 { // the bounce at the top settles
			r.tt.Elapse(kittest.Frame)
		}
		assert.InDelta(t, 0, s.Offset().Y, 1, "back at the top")
	})
}

// TestFlingGlides checks one fling of flingWindow on each list of
// PERF-SCROLL: the list goes on past where the finger left it (the detail's,
// which scrolls 272 DIPs on the 393×852 phone, with the short drag).
func TestFlingGlides(t *testing.T) {
	t.Parallel()

	tests := map[string]float32{"M04": flingLong, "M05": flingShort}
	for id, dist := range tests {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, nil, "")
			r.mount(kittest.Phones[0], id, false)
			file := filepath.Join(must(r.deps.CacheDir()), shotFolder, id+".layout.json")
			r.drive("the layout of "+id, func() bool {
				_, err := os.Stat(file)

				return err == nil
			})
			s := firstScroll(r.sh)
			require.NotNil(t, s)
			v := r.tt.View()
			h := v.Root().Node().WindowFrame().H
			flingWindow(v, geom.Pt(150, h*0.72), geom.Pt(150, h*(0.72-dist)), time.Second)
			lifted := s.Offset().Y
			assert.Less(t, lifted, s.MaxOffset().Y, "the drag leaves room to glide")
			for range 30 {
				r.tt.Elapse(kittest.Frame)
			}
			assert.Greater(t, s.Offset().Y, lifted, "the list glides on")
		})
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// firstScroll returns the first visible scroll under root that can move
// down, nil when there is none.
func firstScroll(root ui.Widget) *ui.Scroll {
	b := root.Node()
	if b.Hidden {
		return nil
	}
	if s, ok := root.(*ui.Scroll); ok && s.MaxOffset().Y > 0 {
		return s
	}
	for ch := range b.Children() {
		if s := firstScroll(ch); s != nil {
			return s
		}
	}

	return nil
}

// perfStateKnown tells whether the phone's shot states hold id.
func perfStateKnown(id string) bool {
	_, ok := shotStates[id]

	return ok
}
