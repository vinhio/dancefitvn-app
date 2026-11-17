// Adapted from jplayer-mobile/e2e/sys/network_android_test.go at 49de3c2.
// Changed: the package's import path; airplane mode on and off added;
// jni.HasJava logged; OnChange's changes counted; nothing switched on a
// real phone (onPhone).

//go:build android && cgo

package sys

import (
	"sync/atomic"
	"testing"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/stretchr/testify/assert"

	jsys "dancefitvn-app/internal/sys"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// networkSteps has devtest-android.sh's driver turn the emulator's Wi-Fi
// and mobile data (its emulated T-Mobile network) off and on with adb
// shell svc, and checks the kind after each: Wi-Fi off leaves mobile data
// (Cellular), both off nothing, mobile data back Cellular again, Wi-Fi
// back Wi-Fi, which Android prefers; then airplane mode on leaves
// nothing and off brings Wi-Fi back. Airplane mode is turned off, both
// networks on again, whatever the outcome. Whether gDesk's Java runs
// (jni.HasJava: without it no change is broadcast, only Kind answers)
// is logged.
func networkSteps(t *testing.T, n *jsys.Network) {
	t.Helper()

	java := jni.HasJava()
	t.Logf("gDesk's Java: %v (false: no change broadcasts, Kind is still asked)", java)
	if onPhone() {
		// A real phone's networks are its owner's: nothing is switched, the
		// kind the system gives now is logged and the switches are checked by
		// hand.
		t.Logf("real phone: network %s, cellular %v; Wi-Fi, mobile data and airplane mode switches are manual checks",
			n.Kind(), n.Cellular())
		assert.Equal(t, n.Kind() == jsys.NetworkCellular, n.Cellular())

		return
	}
	var heard atomic.Int32
	n.OnChange(func(jsys.NetworkKind) { heard.Add(1) })
	t.Cleanup(func() {
		driver(t, "airplane off")
		driver(t, "data on")
		driver(t, "wifi on")
	})
	for _, step := range []struct {
		cmd  string
		want jsys.NetworkKind
	}{
		{"wifi on", jsys.NetworkWiFi},
		{"data on", jsys.NetworkWiFi},
		{"wifi off", jsys.NetworkCellular},
		{"data off", jsys.NetworkNone},
		{"data on", jsys.NetworkCellular},
		{"wifi on", jsys.NetworkWiFi},
		{"airplane on", jsys.NetworkNone},
		{"airplane off", jsys.NetworkWiFi},
	} {
		assert.Equal(t, "ok", driver(t, step.cmd), step.cmd)
		eventually(t, within, step.cmd+": "+step.want.String(), func() bool { return n.Kind() == step.want })
		assert.Equal(t, step.want == jsys.NetworkCellular, n.Cellular(), step.cmd)
		t.Logf("%s: %s, cellular %v", step.cmd, n.Kind(), n.Cellular())
	}
	n.OnChange(nil)
	t.Logf("changes told to OnChange: %d", heard.Load())
	if java {
		assert.Positive(t, heard.Load(), "the broadcasts tell OnChange")
	}
}
