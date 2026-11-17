// Adapted from jplayer-tv/internal/ui/kit/hush.go at 781bee5 (Hush; its Spoken and
// Say are not taken).

package kit

import (
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Hush is embedded by a widget that, while Silent says so, has nothing of its
// subtree described to a screen reader: the prototype's `inert`, which takes an
// element out of the accessibility tree as well as out of the touches (the
// screen and the tab bar under an open sheet, the controls of the video while
// they are hidden, `.scr.vid.hid .vui{pointer-events:none}`). The widget still
// paints and still lays out; only the reading skips it. Silent is asked only when
// a screen reader reads the tree, never in a frame or on a touch.
//
// A widget that embeds Hush must not be a ui.Describer itself, or its own element
// would still be read.
type Hush struct {
	// Silent tells whether the subtree is left out of a reading; nil never.
	Silent func() bool
}

var (
	_ ui.ChildDescriber   = (*Hush)(nil)
	_ ui.SubtreeDescriber = (*Hush)(nil)
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// DescribeChildren adds nothing: a hushed widget has no elements of its own.
func (*Hush) DescribeChildren(*ui.Context, func(platform.SemanticNode)) {}

// DescribesSubtree tells that the subtree is left out while Silent says so.
func (h *Hush) DescribesSubtree() bool { return h.Silent != nil && h.Silent() }
