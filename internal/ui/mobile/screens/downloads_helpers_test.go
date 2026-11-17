//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/require"
)

// The texts of the toasts of the downloads (locales/vi.json).
const (
	toastNoNet    = "Không có mạng để tải"
	toastCanceled = "Đã huỷ tải"
	toastStarted  = "Đang tải file âm thanh…"
	toastDeleted  = "Đã xoá file âm thanh"
	toastNoSpace  = "Không đủ bộ nhớ"
	toastWaitWiFi = "Đang chờ Wi‑Fi để tải"
	toastFailed   = "Tải lỗi, thử lại"
)

// atDownloads boots a signed-in app on the demo downloads (the files of lessons 1,
// 2, 3, 4 and 6 of Set 35 and 7 at 64 %) and opens screen 11, the root of its tab.
// setups run on the Fake before the screen opens; real asks for the phone's own
// text engine.
func atDownloads(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.FakeDownloads)) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	for _, s := range setups {
		s(f.Dl())
	}
	r := shelltest.MountRig(tb, ph, real, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	r.Shell.Go(nav.Route{Screen: env.RouteDownloads}, true)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteDownloads, r.Route())

	return r
}

// downloadsOf returns screen 11 showing.
func downloadsOf(tb testing.TB, r *shelltest.Rig) *screens.Downloads {
	tb.Helper()
	d, ok := r.Shell.Nav().Screen().(*screens.Downloads)
	require.True(tb, ok, "the screen showing is Downloads")

	return d
}

// slideIn lets a sheet that was just opened slide up.
func slideIn(r *shelltest.Rig) { r.Tester.Elapse(400 * time.Millisecond) }

// emptied is a setup that leaves no file and no download.
func emptied(d *shelltest.FakeDownloads) { d.Clear() }
