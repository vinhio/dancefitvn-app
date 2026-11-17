// Adapted from jplayer-mobile/internal/sys/a11ydump.go at 49de3c2: the
// iOS dump serves tvOS too (GOOS=ios with -tags tvos), where the focused
// element carries the selected trait (gDesk bridge/backend/uikit).

package sys

import (
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// A11yElement is an element of the window as the system's screen reader
// finds it: on iOS and tvOS, one UIAccessibilityElement the canvas view hands
// VoiceOver (gDesk's uikit bridge, one per node of the content's tree).
type A11yElement struct {
	// Label and Value are what VoiceOver speaks: the element's name, and
	// what it holds (a slider's position, a toggle's "1" or "0").
	Label string `json:"label"`
	Value string `json:"value,omitempty"`
	// Traits names the UIAccessibilityTraits it carries, in the order of
	// UIAccessibilityConstants.h: "button", "header", "adjustable",
	// "toggle", "selected", "notEnabled", ...
	Traits []string `json:"traits,omitempty"`
	// TraitBits is the raw UIAccessibilityTraits value.
	TraitBits uint64 `json:"traitBits"`
	// Frame is where it is, in the screen's points (accessibilityFrame).
	// UIKit computes it only with its accessibility runtime loaded, as
	// VoiceOver, Accessibility Inspector or the simulator's setting
	// ApplicationAccessibilityEnabled (com.apple.Accessibility) load it:
	// zero otherwise.
	Frame geom.Rect `json:"frame"`
	// InView is where it is in its container's points
	// (accessibilityFrameInContainerSpace, which UIKit turns into Frame),
	// zero for an element that has none.
	InView geom.Rect `json:"inView"`
	// Depth is how deep it lies among the containers below the canvas
	// view: 0 for the view's own elements.
	Depth int `json:"depth"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AccessibilityDump returns the accessibility elements of the window of
// c as the system reads them, in VoiceOver's reading order: on iOS and tvOS, the
// accessibilityElements of the canvas view (and of any container among
// them), each with its label, value, traits and frame. It checks the
// uikit bridge on a simulator, where no VoiceOver runs (a developer's
// tool, local/plans/P8.md P8.3d). Call it on the UI thread. Elsewhere than iOS it fails
// with core.ErrUnsupported: Android's content has no accessibility
// bridge (G10), a desktop's is read by its own tools.
func AccessibilityDump(c platform.Canvas) ([]A11yElement, error) {
	return accessibilityDump(c)
}
