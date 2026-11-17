//go:build !tvos

package app

import (
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestParseNetwork checks the values of DFV_NETWORK and of the file of
// scripts/net.sh.
func TestParseNetwork(t *testing.T) {
	t.Parallel()

	tests := map[string]int32{
		"none": int32(sys.NetworkNone), " airplane\n": int32(sys.NetworkNone), "off": int32(sys.NetworkNone),
		"wifi": int32(sys.NetworkWiFi), "Wi-Fi": int32(sys.NetworkWiFi),
		"cellular": int32(sys.NetworkCellular), "data\n": int32(sys.NetworkCellular),
		"auto": noForce, "": noForce, "5g": noForce,
	}
	for in, want := range tests {
		assert.Equal(t, want, parseNetwork(in), "%q", in)
	}
}

// TestDevNetwork checks the developer's network: the kind of DFV_NETWORK at
// launch, the file's over it while it exists ("auto" gives the system's
// back), each change of the kind forced told on the UI thread, and the
// system's changes told only while nothing is forced.
func TestDevNetwork(t *testing.T) {
	t.Parallel()

	q := &uiQueue{}
	base := &fakeNet{kind: sys.NetworkWiFi}
	file := filepath.Join(t.TempDir(), "dev", "network")
	l := &testLog{}
	d := newDevNetwork(base, func(fn func()) { q.enqueue(fn) }, file, "cellular", l.add)
	var told []sys.NetworkKind
	d.OnChange(func(k sys.NetworkKind) { told = append(told, k) })
	assert.Equal(t, sys.NetworkCellular, d.Kind(), "DFV_NETWORK")

	base.set(sys.NetworkWired)
	assert.Empty(t, told, "the system's change hides under the kind forced")

	require.NoError(t, os.MkdirAll(filepath.Dir(file), 0o700))
	require.NoError(t, os.WriteFile(file, []byte("none\n"), 0o600))
	d.poll()
	assert.Equal(t, sys.NetworkNone, d.Kind())
	assert.Empty(t, told, "told on the UI thread")
	q.drain()
	assert.Equal(t, []sys.NetworkKind{sys.NetworkNone}, told)
	d.poll()
	assert.Zero(t, q.drain(), "no change, nothing told")

	require.NoError(t, os.WriteFile(file, []byte("auto"), 0o600))
	d.poll()
	q.drain()
	assert.Equal(t, sys.NetworkWired, d.Kind(), "the system's")
	assert.Equal(t, []sys.NetworkKind{sys.NetworkNone, sys.NetworkWired}, told)
	base.set(sys.NetworkWiFi)
	assert.Equal(t, sys.NetworkWiFi, told[len(told)-1], "the system's changes told again")

	require.NoError(t, os.Remove(file))
	d.poll()
	q.drain()
	assert.Equal(t, sys.NetworkCellular, d.Kind(), "no file: DFV_NETWORK again")
	assert.NotEmpty(t, l.all())

	d.Close()
	d.Close()
	assert.True(t, base.closed)
}

// TestWatchNetworkOffPhone checks the watch where the system has none: no
// network known, never a change, and in the build that ships no override.
func TestWatchNetworkOffPhone(t *testing.T) {
	t.Parallel()

	l := &testLog{}
	n := watchNetwork(func(fn func()) { fn() }, l.add, filepath.Join(t.TempDir(), "network"), "none")
	defer n.Close()
	if devBuild {
		assert.Equal(t, sys.NetworkNone, n.Kind())
	} else {
		assert.Equal(t, sys.NetworkUnknown, n.Kind())
		assert.Empty(t, l.all(), "ErrUnsupported is not logged")
	}
	n.OnChange(func(sys.NetworkKind) {})
	n.OnChange(nil)
}

// TestNetworkDrivesOnline checks 03 section 2 with no download: no network
// is offline at once (the mock cut), an unknown kind changes nothing, the
// network back is probed at once and online again; a network gone at launch
// is offline from the start.
func TestNetworkDrivesOnline(t *testing.T) {
	t.Parallel()

	t.Run("changes", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		d.net.set(sys.NetworkNone) // before build: build applies it
		assert.True(t, d.w.Online(), "nothing built yet")
		d.net.kind = sys.NetworkWiFi
		d.signIn()
		_, err := d.w.Config(t.Context()) // in the cache: the probe's answer offline
		require.NoError(t, err)

		d.net.set(sys.NetworkNone)
		assert.False(t, d.w.Online())
		d.clk.Advance(probeSteps[0])
		d.drive("the probe answered from the cache", func() bool { return !d.w.apiReached.Load() })
		d.clk.Advance(probeSteps[1])
		d.q.drain()
		assert.False(t, d.w.Online(), "a cached answer is no server")
		d.net.set(sys.NetworkUnknown)
		assert.False(t, d.w.Online(), "unknown tells nothing")
		assert.True(t, d.w.tr.down())
		d.net.set(sys.NetworkCellular)
		assert.False(t, d.w.tr.down(), "the mock answers again")
		d.drive("online again", d.w.Online)
	})
	t.Run("none at launch", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		d.net.kind = sys.NetworkNone
		_, err := d.w.Restore(t.Context())
		require.NoError(t, err)
		assert.False(t, d.w.Online())
		_, err = d.w.Config(t.Context())
		assert.ErrorIs(t, err, api.ErrOffline)
	})
}

// TestSaveAfterSignOutIsNotQueued checks the carry-over of P4.9: a save of
// the position made once the session ended (the player paused by the
// sign-out) is not queued, so that it is never sent with the next user's
// token; the queue is emptied after the player was left.
func TestSaveAfterSignOutIsNotQueued(t *testing.T) {
	t.Parallel()

	pr := newPlayRig(t, "", nil)
	pr.signIn()
	q := pr.w.queue.Load()
	require.NotNil(t, q)
	pr.w.tr.setOffline(true)
	_, err := wireSender{w: pr.w}.SaveProgress(t.Context(), "l_3503", model.ProgressRequest{PositionSec: 12, DurationSec: 220})
	require.ErrorIs(t, err, api.ErrOffline, "signed in: a save offline is the network's")

	pr.w.mgr.SignOut(nil)
	_, err = wireSender{w: pr.w}.SaveProgress(t.Context(), "l_3503", model.ProgressRequest{PositionSec: 12, DurationSec: 220})
	require.ErrorIs(t, err, api.ErrSignedOut)
	require.NotErrorIs(t, err, api.ErrOffline, "the tracker does not queue it")
	pr.q.drain()
	assert.Zero(t, q.Len())
}
