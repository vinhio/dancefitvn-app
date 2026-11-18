// Pattern from jplayer-mobile/internal/ui/nav/screen.go:131,317 at 49de3c2 (a
// page's scroll is vertical with no bars, and bounces past its ends on every
// system, IOSONLY.md D7).

package kit

import "github.com/gdeskdev/desk/framework/ui"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PageScroll sets s up as the vertical scroll of a phone's page or sheet:
// vertical only, no bars (`scrollbar-width:none`, index.html:74), and a rubber
// band past the top and the bottom (Scroll.Bounce, UIScrollView's: K36). It is
// the one place that turns the bounce on, so that every phone screen scrolls
// alike on iPhone and Android; the TV has no ui.Scroll (tvkit.Slide), and no
// phone screen has a horizontal strip any more (Home's became a grid, K39).
func PageScroll(s *ui.Scroll) {
	s.Axes, s.HideBars, s.Bounce = ui.ScrollVertical, true, true
}
