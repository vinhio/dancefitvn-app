//go:build tvos

package screens_test

import (
	"fmt"
	"testing"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/nav"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// badgeServices returns services whose GET /me holds n badges, named from fake data.
func badgeServices(t testing.TB, n int) *tvtest.Services {
	t.Helper()
	svc := tvtest.NewPlayerServices(t)
	svc.MeData.Badges = nil
	for i := range n {
		svc.MeData.Badges = append(svc.MeData.Badges, model.Badge{Code: fmt.Sprintf("hdv_b%d", i), Name: fmt.Sprintf("HUY_HIEU_%d", i)})
	}

	return svc
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestSettingsBadgesTakeTurns is K44: with more badges than the three of the
// prototype the card shows pages of them, every MedalPageTime the next, so that
// every badge is on the page showing within one round; three or fewer are the
// prototype's single row and never turn.
func TestSettingsBadgesTakeTurns(t *testing.T) {
	t.Parallel()
	for _, n := range []int{1, 3, 4, 8, 12, 20} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			t.Parallel()
			r := openFast(t, badgeServices(t, n), env.RouteSettings, false) // motion reduced: a page turns at once
			r.settle()
			row := r.settings().Card().Row()
			pages := (n + tvkit.MedalsPerPage - 1) / tvkit.MedalsPerPage
			require.Equal(t, pages, row.Pages())
			shown := map[int]bool{}
			for range pages {
				for i := range n {
					if row.PageOf(i) == row.Page() {
						shown[i] = true
					}
				}
				r.tt.Elapse(tvkit.MedalPageTime)
			}
			assert.Len(t, shown, n, "every badge was on the page showing")
			assert.Zero(t, row.Page(), "a round of the pages is back at the first")
			if n <= tvkit.MedalsPerPage {
				assert.Equal(t, 1, row.Pages(), "the prototype's row")
			}
		})
	}
}

// TestSettingsBadgesAreReadWhicheverPageShows: the screen reader finds the name of
// every badge on every page, and the dots are not read.
func TestSettingsBadgesAreReadWhicheverPageShows(t *testing.T) {
	t.Parallel()
	const n = 8
	r := openFast(t, badgeServices(t, n), env.RouteSettings, false)
	r.settle()
	row := r.settings().Card().Row()
	for range row.Pages() {
		got := map[string]bool{}
		for _, node := range r.tt.Semantics().Nodes {
			got[node.Label] = true
		}
		for i := range n {
			assert.True(t, got[kit.MedalText(fmt.Sprintf("HUY_HIEU_%d", i))], "badge %d read at page %d", i, row.Page())
		}
		r.tt.Elapse(tvkit.MedalPageTime)
	}
}

// TestSettingsBadgesDotsStayOnTheCard: the dots under a paged row, and everything
// of it, are over the hint and inside the card.
func TestSettingsBadgesDotsStayOnTheCard(t *testing.T) {
	t.Parallel()
	for _, n := range []int{4, 8, 20} {
		r := openFast(t, badgeServices(t, n), env.RouteSettings, false)
		r.settle()
		card := r.settings().Card()
		cf, rf := card.Node().WindowFrame(), card.Row().Node().WindowFrame()
		hint, ok := r.layout().Find("tv.settings.hint")
		require.True(t, ok)
		assert.LessOrEqual(t, rf.Y+rf.H, float32(hint.Y)+.5, "n=%d the row over the hint", n)
		assert.LessOrEqual(t, rf.Y+rf.H, cf.Y+cf.H, "n=%d the row on the card", n)
		assert.LessOrEqual(t, float32(hint.Y+hint.H), cf.Y+cf.H, "n=%d the hint on the card", n)
	}
}

// TestSettingsBadgesStopWhenTheScreenIsLeft: the screen left, the pages no more turn
// (nothing keeps a timer going under another screen).
func TestSettingsBadgesStopWhenTheScreenIsLeft(t *testing.T) {
	t.Parallel()
	r := openFast(t, badgeServices(t, 8), env.RouteSettings, false)
	r.settle()
	row := r.settings().Card().Row()
	r.tt.Elapse(tvkit.MedalPageTime)
	require.Equal(t, 1, row.Page())
	r.sh.Go(nav.Route{Screen: env.RouteHome}, true)
	r.settle()
	r.tt.Elapse(3 * tvkit.MedalPageTime)
	assert.Equal(t, 1, row.Page(), "no turn after the screen was left")
}

// TestSettingsBadgesReducedMotion: with the system's Reduce Motion on, the pages
// still turn, with no fade.
func TestSettingsBadgesReducedMotion(t *testing.T) {
	t.Parallel()
	r := openFast(t, badgeServices(t, 8), env.RouteSettings, false)
	r.settle()
	r.tt.SetPreferences(platform.Preferences{ReduceMotion: true})
	row := r.settings().Card().Row()
	r.tt.Elapse(tvkit.MedalPageTime + time.Second/10)
	assert.Equal(t, 1, row.Page())
	assert.False(t, row.Fading())
}
