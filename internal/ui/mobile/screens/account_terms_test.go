//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// termsURL is a links.terms as the mobile API answers it (the mock lacks the key).
const termsURL = "https://www.dancefitvn.com/dieu-khoan-su-dung"

// termsText is the row of K34.
const termsText = "Điều khoản sử dụng"

// withTerms sets links.terms over the mock's app config.
func withTerms(link string) func(*model.AppConfig) {
	return func(c *model.AppConfig) { c.Links.Terms = link }
}

// TestAccountTermsRow checks K34 on both phones: the row "Điều khoản sử dụng" is right under
// "Chính sách bảo mật", closes the card of the app (the privacy row gets its line), and a tap
// opens links.terms through the same service as the privacy row, and nothing else.
func TestAccountTermsRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false, withConfig(withTerms(termsURL)))
		a := accountOf(t, r)
		require.True(t, a.PrivacyShown())
		require.True(t, a.TermsShown())
		assert.True(t, r.Tester.Shows(termsText))
		assert.True(t, a.TermsRow().Chevron)
		assert.True(t, a.TermsRow().Last, "the last row of its card")
		assert.False(t, a.Privacy().Last, "the privacy row is not the last any more")
		assert.Equal(t, "link:terms", a.TermsRow().Name)

		lay := r.Dump("M15-terms")
		priv, terms := element(t, lay, "link:privacy"), element(t, lay, "link:terms")
		assert.InDelta(t, priv.Y+priv.H, terms.Y, 0.51, "directly under the privacy row")
		assert.InDelta(t, priv.H, terms.H, 0.51, "a row like the others")
		assert.InDelta(t, priv.X, terms.X, 0.01)
		assert.InDelta(t, priv.W, terms.W, 0.01)

		scrollInto(t, r, a.Scroll(), "link:terms")
		tapKey(t, r, "link:terms")
		assert.Equal(t, []string{termsURL}, r.Fake.OpenedURLs())
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Equal(t, env.RouteAccount, r.Route())
		assert.Equal(t, 0, r.Shell.Nav().Depth())
	})
}

// TestAccountTermsRowIsHiddenWithoutALink checks that a config without links.terms (the mock's)
// or with blanks leaves the card as K32 has it: the privacy row last.
func TestAccountTermsRowIsHiddenWithoutALink(t *testing.T) {
	t.Parallel()

	for _, link := range []string{"", "   "} {
		kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
			r := atAccount(t, ph, false, withConfig(withTerms(link)))
			a := accountOf(t, r)
			assert.False(t, a.TermsShown(), "%q", link)
			assert.True(t, a.PrivacyShown())
			assert.True(t, a.Privacy().Last, "the privacy row closes the card again")
			assert.False(t, r.Tester.Shows(termsText))
			_, ok := r.Dump("M15").Find("link:terms")
			assert.False(t, ok)
		})
	}
}

// TestAccountTermsRowWithoutPrivacy checks that the terms row alone follows the downloaded
// files' row and closes the card.
func TestAccountTermsRowWithoutPrivacy(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false, withConfig(func(c *model.AppConfig) {
			c.Links.Privacy, c.Links.Terms = "", termsURL
		}))
		a := accountOf(t, r)
		assert.False(t, a.PrivacyShown())
		require.True(t, a.TermsShown())
		assert.True(t, a.TermsRow().Last)
		lay := r.Dump("M15-terms")
		dl, terms := element(t, lay, "go.downloads"), element(t, lay, "link:terms")
		assert.InDelta(t, dl.Y+dl.H, terms.Y, 0.51, "directly under the downloaded files")
	})
}

// TestAccountTermsRowAfterTheConfigArrives checks the launch with no network: the row is
// hidden while the app config is not known, and shows when the screen has asked for it.
func TestAccountTermsRowAfterTheConfigArrives(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shell.Options{}, signed(), func(_ *testing.T, f *shelltest.Fake) {
			f.SetOnline(false)
			f.SetConfigEdit(withTerms(termsURL))
		})
		r.Boot()
		_, known := r.Shell.Env().Cfg()
		require.False(t, known, "no config at launch")
		r.Fake.SetOnline(true)
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		a := accountOf(t, r)
		assert.True(t, a.TermsShown(), "the screen asked for the config")
		assert.True(t, a.TermsRow().Last)
		assert.False(t, a.Privacy().Last)
		assert.True(t, r.Tester.Shows(termsText))
	})
}

// TestAccountTermsSteadyFrameDoesNotAllocate checks that a frame of screen 15 with both link
// rows allocates nothing, on both phones.
func TestAccountTermsSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atAccount(t, ph, false, withConfig(withTerms(termsURL)))
		require.True(t, accountOf(t, r).TermsShown())
		for range 5 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
