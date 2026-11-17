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

// privacyURL is the mock's links.privacy (app/config.json).
const privacyURL = "https://dancefitvn.com/chinh-sach-bao-mat"

// privacyText is the row of K32.
const privacyText = "Chính sách bảo mật"

// withConfig keeps the mock's app config but for edit.
func withConfig(edit func(*model.AppConfig)) func(*shelltest.Fake) {
	return func(f *shelltest.Fake) { f.SetConfigEdit(edit) }
}

// TestAccountPrivacyRow checks K32 on both phones: the row "Chính sách bảo mật" closes the card
// of the app with the chevron and the line under "File âm thanh đã tải", and a tap opens
// links.privacy through the same service as the forgot link, and nothing else (no sheet, no
// route, no toast).
func TestAccountPrivacyRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false, withConfig(nil))
		a := accountOf(t, r)
		require.True(t, a.PrivacyShown())
		assert.True(t, r.Tester.Shows(privacyText))
		assert.True(t, a.Privacy().Chevron)
		assert.True(t, a.Privacy().Last, "the last row of its card")
		assert.Equal(t, "link:privacy", a.Privacy().Name)

		lay := r.Dump("M15-privacy")
		dl, priv := element(t, lay, "go.downloads"), element(t, lay, "link:privacy")
		assert.Greater(t, priv.Y, dl.Y+dl.H-1, "under the downloaded files")
		assert.InDelta(t, dl.H, priv.H, 0.01, "a row like the others")
		assert.InDelta(t, dl.X, priv.X, 0.01)
		assert.InDelta(t, dl.W, priv.W, 0.01)

		scrollInto(t, r, a.Scroll(), "link:privacy")
		tapKey(t, r, "link:privacy")
		assert.Equal(t, []string{privacyURL}, r.Fake.OpenedURLs())
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Equal(t, env.RouteAccount, r.Route())
		assert.Equal(t, 0, r.Shell.Nav().Depth())
	})
}

// TestAccountPrivacyRowIsHiddenWithoutALink checks that an empty links.privacy (or blanks) leaves
// the card as the prototype's M15 has it (TestAccountBoxesMatchPrototype compares it), the row
// of the downloaded files last again.
func TestAccountPrivacyRowIsHiddenWithoutALink(t *testing.T) {
	t.Parallel()

	for _, link := range []string{"", "   "} {
		kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
			r := atAccount(t, ph, false, withConfig(func(c *model.AppConfig) { c.Links.Privacy = link }))
			a := accountOf(t, r)
			assert.False(t, a.PrivacyShown(), "%q", link)
			assert.False(t, r.Tester.Shows(privacyText))
			_, ok := r.Dump("M15").Find("link:privacy")
			assert.False(t, ok)
			assert.True(t, element(t, r.Dump("M15"), "go.downloads").H > 0)
		})
	}
}

// TestAccountPrivacyRowShiftsOnlyWhatIsBelow checks that the row moves nothing above it: with it,
// every element of M15 down to the card's last row is where the prototype's M15 has it (the row
// of the downloaded files gets its 1 px line, which moves what is inside it by half a pixel), the
// red "Đăng xuất" and the version line are one row lower.
func TestAccountPrivacyRowShiftsOnlyWhatIsBelow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		without := atAccount(t, ph, false)
		with := atAccount(t, ph, false, withConfig(nil))
		a, b := without.Dump("M15"), with.Dump("M15")
		priv := element(t, b, "link:privacy")
		dl := element(t, a, "go.downloads")
		shift := element(t, b, "sheet:logout").Y - element(t, a, "sheet:logout").Y
		assert.InDelta(t, element(t, a, "p.muted").Y+shift, element(t, b, "p.muted").Y, 0.01, "the version line moves with it")
		assert.InDelta(t, priv.H, shift, 0.01, "the card grows by the row")
		tabs := element(t, a, "div.tabs").Y
		for _, ea := range a.Elements {
			eb, ok := b.Find(ea.Key)
			require.Truef(t, ok, "%s", ea.Key)
			switch {
			case ea.Y >= tabs: // the tab bar is on the window, not on the page (its icons are numbered again)
			case ea.Y+ea.H <= dl.Y:
				assert.InDelta(t, ea.Y, eb.Y, 0.01, ea.Key)
			case ea.Y < dl.Y+dl.H: // inside the row that is not the last any more
				assert.InDelta(t, ea.Y, eb.Y, 0.51, ea.Key)
			default:
				assert.InDelta(t, ea.Y+shift, eb.Y, 0.51, "%s moves down by the row", ea.Key)
			}
		}
	})
}

// TestAccountPrivacyRowAfterTheConfigArrives checks the launch with no network: the row is
// hidden while the app config is not known, and shows when the screen has asked for it once
// the network is back.
func TestAccountPrivacyRowAfterTheConfigArrives(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shell.Options{}, signed(), func(_ *testing.T, f *shelltest.Fake) { f.SetOnline(false) })
		r.Boot()
		_, known := r.Shell.Env().Cfg()
		require.False(t, known, "no config at launch")
		r.Fake.SetOnline(true)
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		a := accountOf(t, r)
		assert.True(t, a.PrivacyShown(), "the screen asked for the config")
		assert.True(t, r.Tester.Shows(privacyText))
		assert.Equal(t, 2, r.Fake.Calls("app.config"), "the boot's try that failed, and the screen's")
	})
}

// TestAccountPrivacyRowStaysHiddenOffline checks that a config that cannot be had leaves the row
// hidden and the screen as it was.
func TestAccountPrivacyRowStaysHiddenOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := newRig(t, ph, false, shell.Options{}, signed(), func(_ *testing.T, f *shelltest.Fake) { f.SetOnline(false) })
		r.Boot()
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		a := accountOf(t, r)
		assert.False(t, a.PrivacyShown())
		assert.False(t, r.Tester.Shows(privacyText))
	})
}

// TestAccountPrivacySteadyFrameDoesNotAllocate checks that a frame of screen 15 with the row
// allocates nothing, on both phones.
func TestAccountPrivacySteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atAccount(t, ph, false, withConfig(nil))
		require.True(t, accountOf(t, r).PrivacyShown())
		for range 5 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
