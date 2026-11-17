package kit_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/theme"

	"github.com/gdeskdev/desk/foundation/core/color"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
)

// TestSwitchBoxesMatchPrototype compares a switch with the `span.sw` of M07:
// 51 x 31, named by its state.
func TestSwitchBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		on, off := kit.NewSwitch(true), kit.NewSwitch(false)
		on2 := kit.NewSwitch(true)
		mountStage(t, ph, placeAt(on, 322, 542, 0), placeAt(off, 322, 594, 0), placeAt(on2, 322, 646, 0))
		rectNear(t, protoBox(t, "M07", "span.sw.on"), frameOf(on), .01, "span.sw.on")
		rectNear(t, protoBox(t, "M07", "span.sw"), frameOf(off), .01, "span.sw")
		rectNear(t, protoBox(t, "M07", "span.sw.on#2"), frameOf(on2), .01, "span.sw.on#2")
		assert.Equal(t, "span.sw.on", on.Name)
		assert.Equal(t, "span.sw", off.Name)
	})
}

// TestSwitchPaint checks the colors and the knob at rest, on and off: --sf2
// with a white knob 4 in, --pr with an --on knob 24 in (the CSS left of 3 and 23
// plus the 1 px border), and the knob's top 3.5 down.
func TestSwitchPaint(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		on, off := kit.NewSwitch(true), kit.NewSwitch(false)
		tt := mountStage(t, ph, placeAt(on, 20, 20, 0), placeAt(off, 20, 80, 0))
		img := tt.Snapshot()
		x, y, d := on.Knob()
		assert.InDelta(t, 24, x, 0)
		assert.InDelta(t, 3.5, y, 0)
		assert.InDelta(t, 24, d, 0)
		x, _, _ = off.Knob()
		assert.InDelta(t, 4, x, 0)
		pixelNear(t, img, 20+8, 20+15, look.T.Primary, 2, "on track")
		pixelNear(t, img, 20+36, 20+15, look.T.OnPrimary, 2, "on knob")
		pixelNear(t, img, 20+2, 80+15, look.T.Surface2, 2, "off track, left of the knob")
		pixelNear(t, img, 20+16, 80+15, color.White, 2, "off knob")
		pixelNear(t, img, 20+38, 80+15, look.T.Surface2, 2, "off track, right of the knob")
		// the border: 1 px of --bd round the off track, --pr round the on one
		pixelNear(t, img, 20+25, 80, look.T.Border, 4, "off border")
		pixelNear(t, img, 20+25, 20, look.T.Primary, 4, "on border")
	})
}

// TestSwitchAnimates checks that turning a switch on moves the knob and the
// colors over 200 ms on CSS `ease`, and that a switch built in a state does not
// animate in.
func TestSwitchAnimates(t *testing.T) {
	t.Parallel()
	look := palette()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		sw := kit.NewSwitch(false)
		tt := mountStage(t, ph, placeAt(sw, 20, 20, 0))
		kittest.PinMotion(tt)
		tt.Elapse(kittest.Frame)
		x0, _, _ := sw.Knob()
		assert.InDelta(t, 4, x0, 0, "built off: at rest at once")

		sw.SetOn(true)
		assert.Equal(t, "span.sw.on", sw.Name)
		tt.Elapse(100 * time.Millisecond)
		x, _, _ := sw.Knob()
		// the animation starts at the first paint after SetOn, a frame after it,
		// so 100 ms later it is between 5/12 and 1/2 of the way in time
		lo, hi := 4+20*theme.CSSEase(5./12)-.5, 4+20*theme.CSSEase(.5)+.5
		assert.GreaterOrEqual(t, x, lo, "not behind the curve")
		assert.LessOrEqual(t, x, hi, "not ahead of it")
		// the track is between --sf2 and --pr, as far as the knob is
		mid := look.T.Surface2.Mix(look.T.Primary, (x-4)/20)
		pixelNear(t, tt.Snapshot(), 20+8, 20+15, mid, 6, "track mid-way")

		tt.Elapse(100*time.Millisecond + 2*kittest.Frame)
		x, _, _ = sw.Knob()
		assert.InDelta(t, 24, x, .01)
		pixelNear(t, tt.Snapshot(), 20+8, 20+15, look.T.Primary, 2, "track on")

		sw.SetOn(false)
		tt.Elapse(100 * time.Millisecond)
		x, _, _ = sw.Knob()
		assert.Less(t, x, float32(24-5), "moving back")
		assert.Greater(t, x, float32(4+1), "not there yet")
		tt.Elapse(150 * time.Millisecond)
		x, _, _ = sw.Knob()
		assert.InDelta(t, 4, x, .01)
	})
}

// TestSwitchSemantics checks that a switch is a switch, on or off.
func TestSwitchSemantics(t *testing.T) {
	t.Parallel()
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		on, off := kit.NewSwitch(true), kit.NewSwitch(false)
		tt := mountStage(t, ph, placeAt(on, 20, 20, 0), placeAt(off, 20, 80, 0))
		var states []platform.Toggled
		for _, n := range tt.Semantics().Nodes {
			if n.Role == platform.RoleSwitch {
				states = append(states, n.Toggle)
			}
		}
		assert.Equal(t, []platform.Toggled{platform.ToggledOn, platform.ToggledOff}, states)
	})
}

// TestSwitchPaintDoesNotAllocate checks that painting switches at rest and a
// switch with its animation running allocates nothing.
func TestSwitchPaintDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		rest, moving := kit.NewSwitch(true), kit.NewSwitch(false)
		tt := mountStage(t, ph, placeAt(rest, 20, 20, 0), placeAt(moving, 20, 80, 0))
		kittest.PinMotion(tt)
		tt.Repaint()
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint at rest", ph.Name)
		moving.SetOn(true)
		tt.Elapse(60 * time.Millisecond)
		x, _, _ := moving.Knob()
		assert.Greater(t, x, float32(4), "the animation runs")
		assert.Less(t, x, float32(24), "the animation is not over")
		assert.Zero(t, testing.AllocsPerRun(50, tt.Repaint), "%s: a repaint with the knob moving", ph.Name)
	}
}
