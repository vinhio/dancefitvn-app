// Adapted from jplayer-mobile/internal/ui/kit/a11y.go at 49de3c2 (Decor, DescribeButtons,
// PressButton); Spoken is not taken, the labels of this app are made where they are
// shown.

package kit

import (
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// What assistive technologies see of the kit (P8.3a). The prototype has no ARIA
// (`grep aria-` of index.html is empty), so what a screen reader makes of the
// markup is what the widgets tell:
//
//	.rb, .ib, .play-btn, button     button, named by its text or its Label
//	.row[data-act], .card           button (list item when it holds buttons of its own)
//	.switch, .rb row with a switch  switch, on or off (Toggle)
//	.seg button, .chip, .tabs       radio / tab, the chosen one FlagSelected
//	.scrub, .wave                   slider with Min, Max, Now, +-10 s by Increment/Decrement
//	.h1, .d of a page, .sh .d       heading, Level 1 (a section's title is Level 2)
//	poster, medal, beats, art       no element: decoration (Decor), or one element naming it
//
// Describers run only when the system reads the tree, never per frame.

// Decor is the embeddable of a widget that is decoration: no element, and
// nothing in its subtree is one (a poster with its title painted in, the
// numbers of the beats). It is a ChildDescriber that describes nothing and a
// SubtreeDescriber that says so. A widget that is a Describer too is one element
// and nothing under it (a badge: the caption, not its medal).
type Decor struct{}

var (
	_ ui.ChildDescriber   = Decor{}
	_ ui.SubtreeDescriber = Decor{}
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DescribeChildren adds nothing.
func (Decor) DescribeChildren(*ui.Context, func(platform.SemanticNode)) {}

// DescribesSubtree tells that the subtree needs no description.
func (Decor) DescribesSubtree() bool { return true }

// DescribeButtons is the DescribeChildren of a widget that is one element as a
// whole (a lesson row, a file row) and holds buttons of its own: each button that
// is not hidden is added as an element under the widget, in the order given, its
// key its place in buttons plus one. The widget is a list item (a button holds
// no elements), answers DescribesSubtree with true so that nothing else in it is
// read twice, and hands PerformChildAction to PressButton.
func DescribeButtons(c *ui.Context, owner ui.Widget, add func(platform.SemanticNode), buttons ...ui.Widget) {
	from := owner.Node().WindowFrame().Min()
	for i, b := range buttons {
		if b == nil || b.Node().Hidden {
			continue
		}
		n := platform.SemanticNode{
			ID:     uint64(i + 1),
			Parent: -1,
			Role:   platform.RoleButton,
			Label:  ui.LabelOf(b),
			Bounds: b.Node().WindowFrame().Translate(geom.Pt(-from.X, -from.Y)),
		}
		if d, ok := b.(ui.Describer); ok {
			d.Describe(c, &n)
		}
		if b.Node().Disabled {
			n.Flags |= platform.FlagDisabled
		}
		add(n)
	}
}

// PressButton is the PerformChildAction of DescribeButtons' widgets: it presses
// the button of key, and tells whether it took the action.
func PressButton(a platform.SemanticAction, key uint64, buttons ...ui.Widget) bool {
	if a != platform.ActPress || key == 0 || key > uint64(len(buttons)) {
		return false
	}
	b := buttons[key-1]
	if b == nil {
		return false
	}
	p, ok := b.(interface{ Press() })
	if !ok || b.Node().Disabled || b.Node().Hidden {
		return false
	}
	p.Press()

	return true
}
