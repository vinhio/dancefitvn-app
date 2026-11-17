// Adapted from jplayer-mobile/e2e/sys/platform_android_test.go at 49de3c2
// (shownActivity, driverTurns, driver, topActivity, expectShown,
// cancelShown, expectGone; the file pickers and the share sheet left
// out; onPhone added).

//go:build android && cgo

package sys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// shownActivity are the words one of which the activity each service
// starts has in its name, as dumpsys shows it: the Photo Picker of
// MediaProvider (API 33+; GET_CONTENT before it opens DocumentsUI).
var shownActivity = map[shown][]string{
	shownImage: {"photopicker", "PhotoPicker", "documentsui"},
}

// driverTurns numbers the commands sent to devtest-android.sh's driver.
var driverTurns atomic.Int32

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// driver has devtest-android.sh do cmd from the Mac, where adb can turn
// the emulator's networks off and on, and returns its answer: the app
// logs "devtest: <n> <cmd>", the script, which reads the log each second,
// runs it and pushes its answer to devtest-<n>.txt in the media folder.
// The commands of this package: wifi on|off, data on|off, airplane
// on|off, top (the resumed activity, from dumpsys), back (the Back key).
func driver(t *testing.T, cmd string) string {
	t.Helper()

	n := driverTurns.Add(1)
	fmt.Printf("devtest: %d %s\n", n, cmd)
	answer := filepath.Join(mediaDir(), fmt.Sprintf("devtest-%d.txt", n))
	var got []byte
	eventually(t, within, "the driver's answer to "+cmd, func() bool {
		b, err := os.ReadFile(answer) //nolint:gosec // G304: the driver's answer in the media folder
		got = b

		return err == nil && len(b) > 0
	})

	return strings.TrimSpace(string(got))
}

// onPhone tells whether the tests run on a real phone, where the driver
// changes nothing of the system (devtest-android.sh pushes phone.txt).
func onPhone() bool {
	_, err := os.Stat(filepath.Join(mediaDir(), "phone.txt"))

	return err == nil
}

// topActivity returns the resumed activity, as dumpsys tells it.
func topActivity(t *testing.T) string {
	t.Helper()

	return driver(t, "top")
}

// expectShown waits until the activity of what is the resumed one.
func expectShown(t *testing.T, what shown) {
	t.Helper()

	words := shownActivity[what]
	var top string
	for range 10 {
		top = topActivity(t)
		for _, w := range words {
			if strings.Contains(top, w) {
				t.Logf("shown: %s", top)

				return
			}
		}
	}
	require.FailNow(t, "not shown", "want one of %v, top is %q", words, top)
}

// cancelShown backs out of what shows, as the user's Back does.
func cancelShown(t *testing.T, _ shown) {
	t.Helper()

	require.Equal(t, "ok", driver(t, "back"))
}

// expectGone waits until the app's own activity is the resumed one again.
func expectGone(t *testing.T) {
	t.Helper()

	for range 10 {
		if top := topActivity(t); strings.Contains(top, appID) {
			return
		}
	}
	require.FailNow(t, "the app is not back in front")
}
