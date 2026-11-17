// Adapted from jplayer-mobile/e2e/engine/driver_test.go at 49de3c2.
// Changed: the answers read from replyDir (iOS: the temporary folder;
// Android: the media folder), bg.txt read for the phase on Android.

//go:build (ios && !tvos) || (android && cgo)

package engine

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/media"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// driverWithin is how long the device script's driver may take to answer.
const driverWithin = 40 * time.Second

// bgPhase is how long the background phase lasts by default; DFV_E2E_BG
// (through devtest-ios.sh; bg.txt of the media folder, pushed by
// devtest-android.sh) gives it in seconds.
const bgPhase = 15 * time.Second

// driverTurns numbers the commands sent to the device script's driver.
var driverTurns atomic.Int32

// lifecycle follows the app's lifecycle as the app does (internal/app):
// the engine goes to the background with the app and comes back with it.
type lifecycle struct {
	mu    sync.Mutex
	state platform.LifecycleState
	since time.Time // when state came
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// driver has the device script do cmd from the Mac (another app in front,
// the app back in front, a screenshot) and returns its answer: the test
// logs "devtest: <n> <cmd>", the script, which reads the log each second,
// runs it and writes its answer to devtest-<n>.txt in the app's temporary
// folder. It reports false when no answer came: the test runs without the
// script.
func driver(t *testing.T, cmd string) (string, bool) {
	t.Helper()

	n := driverTurns.Add(1)
	answer := filepath.Join(replyDir(), fmt.Sprintf("devtest-%d.txt", n))
	// The app's data outlives a run: an answer, or a screenshot, of the
	// run before must not pass for this one's.
	_ = os.Remove(answer)
	if name, ok := strings.CutPrefix(cmd, "shot-"); ok {
		_ = os.Remove(filepath.Join(replyDir(), "shot-"+name+".png"))
	}
	fmt.Printf("devtest: %d %s\n", n, cmd)
	deadline := time.Now().Add(driverWithin)
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(answer); err == nil && len(b) > 0 { //nolint:gosec // G304: the driver's answer
			return strings.TrimSpace(string(b)), true
		}
		<-tick.C
	}

	return "", false
}

// screenshot has the driver take a screenshot of the simulator
// (local/p4/shots/ios-<name>.png on the Mac) and returns it decoded; it
// skips the test without the script's driver.
func screenshot(t *testing.T, name string) image.Image {
	t.Helper()

	reply, ok := driver(t, "shot-"+name)
	if !ok {
		t.Skip("needs the driver of e2e/devtest-ios.sh or e2e/devtest-android.sh")
	}
	require.Equal(t, "ok", reply)
	f, err := os.Open(filepath.Join(replyDir(), "shot-"+name+".png")) //nolint:gosec // G304: the driver's copy
	require.NoError(t, err)
	defer func() { _ = f.Close() }()
	img, err := png.Decode(f)
	require.NoError(t, err)

	return img
}

// watchLifecycle follows the app's lifecycle until the test ends, and has
// the engine follow it (media.Backgrounder), as the app does.
func watchLifecycle(t *testing.T, h *harness) *lifecycle {
	t.Helper()

	lc := &lifecycle{state: platform.LifecycleActive, since: time.Now()}
	var remove func()
	h.onUI(t, func() {
		remove = h.app.OnLifecycle(func(s platform.LifecycleState) {
			lc.mu.Lock()
			lc.state, lc.since = s, time.Now()
			lc.mu.Unlock()
			if b, ok := h.eng.(media.Backgrounder); ok {
				switch s {
				case platform.LifecycleBackground:
					b.SetBackground(true)
				case platform.LifecycleActive:
					b.SetBackground(false)
				case platform.LifecycleUnknown, platform.LifecycleInactive:
				default:
				}
			}
		})
	})
	t.Cleanup(func() { h.onUI(t, remove) })

	return lc
}

// await waits until the app is in s, and returns when it came there;
// false after within.
func (lc *lifecycle) await(s platform.LifecycleState, within time.Duration) (time.Time, bool) {
	deadline := time.Now().Add(within)
	tick := time.NewTicker(50 * time.Millisecond)
	defer tick.Stop()
	for time.Now().Before(deadline) {
		lc.mu.Lock()
		got, since := lc.state, lc.since
		lc.mu.Unlock()
		if got == s {
			return since, true
		}
		<-tick.C
	}

	return time.Time{}, false
}

// goBackground has the driver put another app in front (cmd "home") and
// waits for the app's background; it skips the test without the script's
// driver.
func (lc *lifecycle) goBackground(t *testing.T) {
	t.Helper()

	if _, ok := driver(t, "home"); !ok {
		t.Skip("needs the driver of e2e/devtest-ios.sh or e2e/devtest-android.sh")
	}
	_, ok := lc.await(platform.LifecycleBackground, 15*time.Second)
	require.True(t, ok, "in the background after home")
}

// goForeground has the driver bring the app back to the front ("front"),
// and returns when it was active again.
func (lc *lifecycle) goForeground(t *testing.T) time.Time {
	t.Helper()

	_, ok := driver(t, "front")
	require.True(t, ok, "the driver's answer to front")
	at, ok := lc.await(platform.LifecycleActive, 15*time.Second)
	require.True(t, ok, "active again")

	return at
}

// phaseLength returns how long the background phase lasts.
func phaseLength() time.Duration {
	if n, err := strconv.Atoi(os.Getenv("DFV_E2E_BG")); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	if b, err := os.ReadFile(filepath.Join(replyDir(), "bg.txt")); err == nil { //nolint:gosec // G304: the script's file
		if n, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}

	return bgPhase
}
