//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/require"
)

// The texts of screen 16 and of its toasts (locales/vi.json).
const (
	toastTVLinked    = "Đã kết nối Apple TV"
	toastCodeWrong   = "Mã không đúng. Kiểm tra lại mã trên TV"
	toastCodeExpired = "Mã đã hết hạn. Lấy mã mới trên TV"
)

// atConnect opens screen 16 from screen 15, as a tap on its row "Kết nối Apple TV" does,
// with the Apple TV not linked (the prototype's start: `tvLinked:false`). setups run on
// the Fake before the app boots.
func atConnect(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) (*shelltest.Rig, *screens.Connect) {
	tb.Helper()
	r := atAccount(tb, ph, real, setups...)

	return toConnect(tb, r)
}

// atConnectLinked is atConnect with the TV of the mock linked (`d_201`), the start of
// M16-linked.
func atConnectLinked(tb testing.TB, ph kittest.Phone, real bool) (*shelltest.Rig, *screens.Connect) {
	tb.Helper()

	return atConnect(tb, ph, real, func(f *shelltest.Fake) { f.Account().SetTVLinked(true) })
}

// toConnect taps the row "Kết nối Apple TV" of screen 15.
func toConnect(tb testing.TB, r *shelltest.Rig) (*shelltest.Rig, *screens.Connect) {
	tb.Helper()
	tapKey(tb, r, "go.connect")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteConnect, r.Route())
	c, ok := r.Shell.Nav().Screen().(*screens.Connect)
	require.True(tb, ok, "the screen showing is Connect")

	return r, c
}

// typeCode focuses the hidden field of the boxes and types s after what it holds.
func typeCode(r *shelltest.Rig, c *screens.Connect, s string) {
	r.Tester.View().Focus(c.Boxes().Field())
	r.Tester.Type(s)
}

// hasKey tells whether the dump has an element with the key.
func hasKey(lay layoutdump.Layout, key string) bool {
	_, ok := lay.Find(key)

	return ok
}
