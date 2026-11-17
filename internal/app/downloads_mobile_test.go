//go:build !tvos

package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mediaAudio is the folder of the files the mock's media server answers a
// lesson's audio with (<lesson>.mp3, else _sample.mp3).
const mediaAudio = mockDir + "/media/audio"

// slowKBps reads a lesson's file (481 KB) in about 4 s: a test cuts the network
// while it is under way.
const slowKBps = 128

// fakeNet is a netWatch whose kind the test sets, telling the change on the
// test's goroutine (its UI thread).
type fakeNet struct {
	mu     sync.Mutex
	kind   sys.NetworkKind
	fn     func(sys.NetworkKind)
	closed bool
}

// fakeTask records the background tasks of the downloads.
type fakeTask struct {
	begun, ended int
	expired      func()
	err          error
}

// dlRig is the phone's services with the downloads over the mock's media
// server, a fake network, a fake free space and a recorded background task.
type dlRig struct {
	*rig
	net    *fakeNet
	task   *fakeTask
	free   int64
	events []env.DownloadEvent
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDlRig returns the services, not signed in, on a Wi-Fi network; the
// knobs are the state's.
func newDlRig(t *testing.T, dir string, knobs wireKnobs) *dlRig {
	t.Helper()

	return newDlRigOn(t, nil, dir, knobs)
}

// newDlRigOn is newDlRig over the vault of secrets (a second launch).
func newDlRigOn(t *testing.T, secrets *memSecrets, dir string, knobs wireKnobs) *dlRig {
	t.Helper()
	r := newRig(t, secrets, dir)
	d := &dlRig{rig: r, net: &fakeNet{kind: sys.NetworkWiFi}, task: &fakeTask{}, free: 1 << 40}
	r.deps.Network = func(func(func())) netWatch { return d.net }
	r.deps.FreeSpace = func(string) (int64, int64, error) { return d.free, 2 << 40, nil }
	r.deps.AudioDir = func() (string, error) { return filepath.Join(dir, "nobackup", "audio"), nil }
	if dir == "" {
		r.deps.AudioDir = nil
	}
	r.deps.BackgroundTask = func(_ string, expired func()) (func(), error) {
		if d.task.err != nil {
			return nil, d.task.err
		}
		d.task.begun++
		d.task.expired = expired

		return func() { d.task.ended++ }, nil
	}
	r.wire(knobs)
	r.w.dls.Events().Subscribe(func(ev env.DownloadEvent) { d.events = append(d.events, ev) })

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestDownloadsAllowedRule checks the rule the manager asks before each
// transfer (03 section 2, Q-P5-2, Q-P5-3): no network or the app offline
// waits for the network; mobile data with "Chỉ tải qua Wi‑Fi" waits for
// Wi-Fi; anything else, an unknown network too, may download.
func TestDownloadsAllowedRule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		online   bool
		wifiOnly bool
		kind     sys.NetworkKind
		want     error
	}{
		{"wifi", true, true, sys.NetworkWiFi, nil},
		{"wired", true, true, sys.NetworkWired, nil},
		{"other", true, true, sys.NetworkOther, nil},
		{"unknown", true, true, sys.NetworkUnknown, nil},
		{"cellular wifi only", true, true, sys.NetworkCellular, download.ErrWiFiOnly},
		{"cellular any network", true, false, sys.NetworkCellular, nil},
		{"none", true, false, sys.NetworkNone, api.ErrOffline},
		{"none wifi only", true, true, sys.NetworkNone, api.ErrOffline},
		{"offline on wifi", false, true, sys.NetworkWiFi, api.ErrOffline},
		{"offline on cellular", false, false, sys.NetworkCellular, api.ErrOffline},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newDlRig(t, "", wireKnobs{})
			d.net.kind = tc.kind
			d.w.online.Set(tc.online)
			d.w.wifiOnly.Store(tc.wifiOnly)
			err := d.w.allowed()
			if tc.want == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.want)
			}
		})
	}
}

// TestDownloadsStartToDelete walks one download: Start queues it, the file
// lands whole (the media file's sha256) in the system's folder with its display
// data, the screens hear Changed then Finished with the lesson's number, and
// Delete removes it.
func TestDownloadsStartToDelete(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newDlRig(t, dir, wireKnobs{})
	c := d.signIn()
	l := lessonOf(t, c, 5)
	dm := d.manager()

	require.NoError(t, dm.Start(c, []model.Lesson{l}))
	st, _ := dm.State(l.ID)
	assert.Equal(t, kit.DlIng, st, "under way at once")
	require.Len(t, dm.Jobs(), 1)
	assert.Equal(t, "Hãy Hát Lên", dm.Jobs()[0].Title)
	d.drive("the file", func() bool { st, _ := dm.State(l.ID); return st == kit.DlDone })

	items := dm.Items()
	require.Len(t, items, 1)
	assert.Equal(t, env.DownloadItem{
		LessonID: l.ID, CourseID: c.ID, CourseName: c.Name, CourseLessons: 10, CoverURL: c.CoverURL, Title: l.Title, Tag: l.Tag, No: 5,
		DurationSec: l.DurationSec, State: kit.DlDone, Bytes: mediaBytes(t, l.ID), Done: mediaBytes(t, l.ID), Progress: 1,
	}, items[0])
	assert.Equal(t, mediaBytes(t, l.ID), dm.TotalBytes())
	assert.Equal(t, 1, dm.DoneCount())
	assert.Empty(t, dm.Jobs())
	file := filepath.Join(dir, "nobackup", "audio", l.ID+".mp3")
	assert.Equal(t, mediaSHA(t, l.ID), fileSHA(t, file), "the system's folder (sys.PrepareAudioDir)")
	kinds := d.kinds()
	assert.Equal(t, env.DownloadEventChanged, kinds[0])
	fin := d.last(env.DownloadEventFinished)
	assert.Equal(t, 5, fin.No, "the toast's number (Lookup's)")
	assert.Equal(t, l.ID, fin.LessonID)

	require.NoError(t, dm.Delete(l.ID))
	assert.Empty(t, dm.Items())
	assert.Zero(t, dm.DoneCount())
	assert.NoFileExists(t, file)
	assert.Equal(t, env.DownloadEventChanged, d.kinds()[len(d.kinds())-1])
	d.drive("the space read again", func() bool { return d.last(env.DownloadEventSpace).Kind == env.DownloadEventSpace })
	free, total, ok := dm.FreeSpace()
	assert.True(t, ok)
	assert.Equal(t, int64(1<<40), free)
	assert.Equal(t, int64(2<<40), total)
}

// TestDownloadsEventsSeeTheFilesAsTheyAre checks the order of a finish: when a
// subscriber (the detail screen) hears any event, State and Items already show the
// files the event is about, in the foreground and in the background, with the lifecycle
// moving to Active after (a row painted from a cache read before the reload showed
// the old icon on the iPhone simulator).
func TestDownloadsEventsSeeTheFilesAsTheyAre(t *testing.T) {
	t.Parallel()

	for _, bg := range []bool{false, true} {
		name := "foreground"
		if bg {
			name = "background"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
			c := d.signIn()
			l := lessonOf(t, c, 4)
			dm := d.manager()
			var seen []string // what each event found of the lesson, as the screen reads it
			d.w.dls.Events().Subscribe(func(ev env.DownloadEvent) {
				if ev.Kind == env.DownloadEventProgress {
					return
				}
				st, _ := dm.State(l.ID)
				n := 0
				for _, it := range dm.Items() {
					if it.LessonID == l.ID {
						n++
					}
				}
				seen = append(seen, fmt.Sprintf("%d:%d:%d", ev.Kind, st, n))
			})
			require.NoError(t, dm.Start(c, []model.Lesson{l}))
			d.drive("some bytes", func() bool { return d.jobDone(l.ID) > 0 })
			if bg {
				d.w.lifecycle(platform.LifecycleBackground)
			}
			d.drive("the file", func() bool { st, _ := dm.State(l.ID); return st == kit.DlDone })
			if bg {
				d.w.lifecycle(platform.LifecycleActive)
			}
			want := fmt.Sprintf("%d:%d:1", env.DownloadEventFinished, kit.DlDone)
			assert.Contains(t, seen, want, "the finish is heard with the file in")
			assert.NotContains(t, seen, fmt.Sprintf("%d:%d:0", env.DownloadEventFinished, kit.DlDone))
			assert.Equal(t, 1, dm.DoneCount())
		})
	}
}

// TestDownloadsStartRefusals checks what Start queues nothing for: no
// network, files that do not fit the free space, nobody signed in; and what
// it skips: a lesson done or under way, one with no audio.
func TestDownloadsStartRefusals(t *testing.T) {
	t.Parallel()

	t.Run("nobody signed in", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		err := d.manager().Start(model.Course{ID: "set-35"}, []model.Lesson{{ID: "l_3505", HasAudio: true}})
		assert.ErrorIs(t, err, core.ErrUnsupported)
		assert.ErrorIs(t, d.manager().Delete("l_3505"), core.ErrUnsupported)
	})
	t.Run("no network", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		c := d.signIn()
		d.net.set(sys.NetworkNone)
		d.q.drain()
		err := d.manager().Start(c, c.Lessons[:2])
		assert.ErrorIs(t, err, api.ErrOffline)
		assert.True(t, env.Offline(err))
		assert.Empty(t, d.manager().Jobs())
	})
	t.Run("no space", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		d.free = 1 << 20
		c := d.signIn()
		d.drive("the space", func() bool { _, _, ok := d.manager().FreeSpace(); return ok })
		err := d.manager().Start(c, c.Lessons[:1])
		assert.ErrorIs(t, err, env.ErrNoSpace)
		assert.True(t, env.NoSpace(err))
		assert.Empty(t, d.manager().Jobs())
	})
	t.Run("nothing to do", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		c := d.signIn()
		mute := lessonOf(t, c, 1)
		mute.HasAudio = false
		require.NoError(t, d.manager().Start(c, []model.Lesson{mute}))
		assert.Empty(t, d.manager().Jobs())
	})
}

// TestDownloadsNetworkCutResumes checks 03 section 2 with a download under
// way: the device's network gone is offline at once (the mock's transport and
// the downloads' cut), the download pauses with its bytes; the network back
// is probed at once, and the download goes on from its bytes with a Range
// request (P5 acceptance), to the media file's sha256.
func TestDownloadsNetworkCutResumes(t *testing.T) {
	t.Parallel()

	d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
	c := d.signIn()
	l := lessonOf(t, c, 5)
	dm := d.manager()
	require.NoError(t, dm.Start(c, []model.Lesson{l}))
	d.drive("some bytes", func() bool { return d.jobDone(l.ID) > 0 })

	d.net.set(sys.NetworkNone)
	assert.False(t, d.w.Online(), "offline at once")
	assert.True(t, d.w.tr.down(), "the mock's transport cut")
	d.drive("paused", func() bool { j := dm.Jobs(); return len(j) == 1 && j[0].Paused })
	assert.ErrorIs(t, dm.Jobs()[0].Err, api.ErrOffline)
	have := d.jobDone(l.ID)
	assert.Positive(t, have)
	assert.Less(t, have, mediaBytes(t, l.ID))
	_, err := d.w.Home(t.Context())
	assert.ErrorIs(t, err, api.ErrOffline, "the API is cut too")

	d.net.set(sys.NetworkWiFi)
	d.drive("online again", d.w.Online)
	d.drive("the file", func() bool { st, _ := dm.State(l.ID); return st == kit.DlDone })
	assert.True(t, slices.ContainsFunc(d.log.all(), func(s string) bool {
		return strings.HasPrefix(s, "download: "+l.ID+" resume at ") && strings.HasSuffix(s, "(206)")
	}), "a Range request: %v", d.log.all())
	path, ok := d.w.downloads().Path(l.ID)
	require.True(t, ok)
	assert.Equal(t, mediaSHA(t, l.ID), fileSHA(t, path))
}

// TestDownloadsWiFiOnly checks K23 (Q-P5-2): on mobile data with "Chỉ tải
// qua Wi‑Fi" a download waits (ErrWiFiOnly) and starts on Wi-Fi; turning the
// preference off starts it on mobile data; Wi-Fi to mobile data pauses one
// under way.
func TestDownloadsWiFiOnly(t *testing.T) {
	t.Parallel()

	t.Run("waits for wifi", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		c := d.signIn()
		l := lessonOf(t, c, 5)
		d.net.set(sys.NetworkCellular)
		require.True(t, d.w.Prefs().(env.WiFiPrefs).WiFiOnly(), "on at first") //nolint:forcetypeassert // checked by the compiler
		err := d.manager().Start(c, []model.Lesson{l})
		require.ErrorIs(t, err, env.ErrWiFiOnly)
		assert.True(t, env.WiFiOnlyErr(err))
		d.drive("paused", func() bool { j := d.manager().Jobs(); return len(j) == 1 && j[0].Paused })
		assert.ErrorIs(t, d.manager().Jobs()[0].Err, env.ErrWiFiOnly)

		d.net.set(sys.NetworkWiFi)
		d.drive("the file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
	})
	t.Run("preference off", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{})
		c := d.signIn()
		l := lessonOf(t, c, 5)
		d.net.set(sys.NetworkCellular)
		require.ErrorIs(t, d.manager().Start(c, []model.Lesson{l}), env.ErrWiFiOnly)
		prefs := d.w.Prefs().(env.WiFiPrefs) //nolint:forcetypeassert // checked by the compiler
		prefs.SetWiFiOnly(false)
		assert.False(t, prefs.WiFiOnly())
		d.drive("the file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
		d.drive("saved", func() bool { return !d.w.prefStore.Load().Get().WiFiOnly })
	})
	t.Run("mobile data mid-way", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
		c := d.signIn()
		l := lessonOf(t, c, 5)
		require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
		d.drive("some bytes", func() bool { return d.jobDone(l.ID) > 0 })
		d.net.set(sys.NetworkCellular)
		d.drive("paused", func() bool { j := d.manager().Jobs(); return len(j) == 1 && j[0].Paused })
		assert.ErrorIs(t, d.manager().Jobs()[0].Err, env.ErrWiFiOnly)
		assert.True(t, d.w.Online(), "mobile data is online")
		d.w.Prefs().(env.WiFiPrefs).SetWiFiOnly(true) //nolint:forcetypeassert // checked by the compiler
		assert.True(t, d.manager().Jobs()[0].Paused, "still on mobile data")
	})
}

// TestDownloadsBadSHA checks the developer's DFV_BADSHA: the size is right,
// the sha256 is not, so the download fails with ErrChecksum and leaves no
// file and no part (03 section 9: "Tải lỗi, thử lại").
func TestDownloadsBadSHA(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newDlRig(t, dir, wireKnobs{badSHA: true})
	c := d.signIn()
	l := lessonOf(t, c, 6)
	require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
	d.drive("the failure", func() bool { return d.last(env.DownloadEventFailed).Kind == env.DownloadEventFailed })
	ev := d.last(env.DownloadEventFailed)
	assert.ErrorIs(t, ev.Err, env.ErrChecksum)
	assert.Equal(t, 6, ev.No)
	d.manager().Forget(l.ID) // what the shell does
	d.drive("forgotten", func() bool { return len(d.manager().Jobs()) == 0 })
	audio := filepath.Join(dir, "nobackup", "audio")
	assert.NoFileExists(t, filepath.Join(audio, l.ID+".mp3"))
	d.drive("no part", func() bool { _, err := os.Stat(filepath.Join(audio, l.ID+".mp3.part")); return err != nil })
}

// TestDownloadsSignOutKeeps checks Q-P5-8: a sign-out pauses the download
// and keeps its bytes and its record, the screens see nothing; the same user
// back goes on with it (F5).
func TestDownloadsSignOutKeeps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	d := newDlRig(t, dir, wireKnobs{dlKBps: slowKBps})
	c := d.signIn()
	l := lessonOf(t, c, 5)
	require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
	d.drive("some bytes", func() bool { return d.jobDone(l.ID) > 0 })

	require.NoError(t, d.w.Logout(t.Context()))
	d.q.drain()
	assert.Nil(t, d.w.downloads())
	assert.Empty(t, d.manager().Jobs(), "nobody signed in: nothing shows")
	st, _ := d.manager().State(l.ID)
	assert.Equal(t, kit.DlNone, st)
	part := filepath.Join(dir, "nobackup", "audio", l.ID+".mp3.part")
	assert.FileExists(t, part, "the bytes are kept")
	index, err := os.ReadFile(filepath.Join(dir, "nobackup", "audio", "index.json"))
	require.NoError(t, err)
	assert.Contains(t, string(index), `"pending"`, "the record is kept")

	require.NoError(t, d.w.Login(t.Context(), "coach@example.invalid", "x"))
	d.drive("the file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
	assert.NoFileExists(t, part)
}

// TestDownloadsBackgroundTask checks F5a: in the background a download under
// way holds a background task, given back when it finishes; the end of the
// time pauses it, and the app active again goes on with it.
func TestDownloadsBackgroundTask(t *testing.T) {
	t.Parallel()

	t.Run("held then given back", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
		c := d.signIn()
		d.w.lifecycle(platform.LifecycleBackground)
		assert.Zero(t, d.task.begun, "nothing under way: no task")
		l := lessonOf(t, c, 5)
		require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
		assert.Equal(t, 1, d.task.begun, "a download in the background holds one")
		d.drive("the file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
		assert.Equal(t, 1, d.task.ended, "given back when it is done")
		assert.Equal(t, 1, d.task.begun)
	})
	t.Run("expiry pauses, active resumes", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
		c := d.signIn()
		l := lessonOf(t, c, 5)
		require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
		d.drive("some bytes", func() bool { return d.jobDone(l.ID) > 0 })
		d.w.lifecycle(platform.LifecycleBackground)
		require.Equal(t, 1, d.task.begun)
		d.task.expired() // the system's time is over; sys ends the task after it
		j := d.manager().Jobs()
		require.Len(t, j, 1)
		assert.True(t, j[0].Paused)
		assert.Equal(t, 1, d.task.begun, "a paused download holds no task")

		d.w.lifecycle(platform.LifecycleActive)
		d.drive("the file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
	})
	t.Run("no time granted", func(t *testing.T) {
		t.Parallel()

		d := newDlRig(t, "", wireKnobs{dlKBps: slowKBps})
		d.task.err = core.ErrClosed
		c := d.signIn()
		require.NoError(t, d.manager().Start(c, c.Lessons[4:5]))
		d.w.lifecycle(platform.LifecycleBackground)
		assert.True(t, slices.ContainsFunc(d.log.all(), func(s string) bool { return strings.Contains(s, "background task") }))
	})
}

// TestWireForeground checks Q-P5-4: the services are env.Foregrounder,
// in front before any lifecycle, not in front in the background, in front
// again once active; inactive (a sheet, a call) leaves the answer as it is.
func TestWireForeground(t *testing.T) {
	t.Parallel()

	bg, act, ina := platform.LifecycleBackground, platform.LifecycleActive, platform.LifecycleInactive
	tests := []struct {
		name   string
		states []platform.LifecycleState
		want   bool
	}{
		{"no lifecycle yet", nil, true},
		{"background", []platform.LifecycleState{bg}, false},
		{"background then active", []platform.LifecycleState{bg, act}, true},
		{"active", []platform.LifecycleState{act}, true},
		{"inactive", []platform.LifecycleState{ina}, true},
		{"background then inactive", []platform.LifecycleState{bg, ina}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newDlRig(t, "", wireKnobs{})
			for _, st := range tc.states {
				d.w.lifecycle(st)
			}
			var svc env.Services = d.w
			fg, ok := svc.(env.Foregrounder)
			require.True(t, ok)
			assert.Equal(t, tc.want, fg.Foreground())
			assert.Equal(t, tc.want, (&env.Env{Services: svc}).Foreground(), "the shell's answer")
		})
	}
}

// TestDownloadsRestoredJobsResume checks F5: a job a run before left (its
// part on the disk) is listed paused at the next launch and goes on by
// itself with the network.
func TestDownloadsRestoredJobsResume(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secrets := &memSecrets{vals: map[string][]byte{}}
	first := newDlRigOn(t, secrets, dir, wireKnobs{dlKBps: slowKBps})
	c := first.signIn()
	l := lessonOf(t, c, 5)
	require.NoError(t, first.manager().Start(c, []model.Lesson{l}))
	first.drive("some bytes", func() bool { return first.jobDone(l.ID) > 0 })
	first.w.close() // the process ends

	again := newDlRigOn(t, secrets, dir, wireKnobs{})
	signed, err := again.w.Restore(t.Context())
	require.NoError(t, err)
	require.True(t, signed)
	again.drive("the file", func() bool { st, _ := again.manager().State(l.ID); return st == kit.DlDone })
	assert.True(t, slices.ContainsFunc(again.log.all(), func(s string) bool { return strings.Contains(s, "left by a run before") }))
}

// TestDownloadProgressEventDoesNotAllocate is the gate of a progress event
// of the wire (4 a second per download): the job's numbers move and the
// screens hear it; State reads the cache.
func TestDownloadProgressEventDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	ds := progressFixture()
	var heard int64
	stop := ds.events.Subscribe(func(ev env.DownloadEvent) { heard = ev.Done })
	defer stop()
	ds.events.Publish(env.DownloadEvent{}) // the snapshot of the listeners, built once
	ev := download.Event{Kind: download.EventProgress, LessonID: "l_3507", Done: 1000, Total: 9_800_000}
	allocs := testing.AllocsPerRun(200, func() {
		ev.Done++
		ds.onEvent(ev)
		_, _ = ds.State("l_3507")
		_, _ = ds.State("l_3501")
	})
	assert.Zero(t, allocs)
	assert.Equal(t, ev.Done, heard, "the screens hear it")
	st, p := ds.State("l_3507")
	assert.Equal(t, kit.DlIng, st)
	assert.Greater(t, p, float32(0))
}

// BenchmarkDownloadProgressEvent measures a progress event of the wire.
func BenchmarkDownloadProgressEvent(b *testing.B) {
	ds := progressFixture()
	ev := download.Event{Kind: download.EventProgress, LessonID: "l_3507", Total: 9_800_000}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		ev.Done = int64(i)
		ds.onEvent(ev)
	}
}

// TestDlTransport checks the downloads' client of the mock mode: a request
// and a body being read fail as a lost network does (api.ErrOffline to the
// manager) while the network is down; DFV_DL_KBPS reads a tenth of a second
// at a time.
func TestDlTransport(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	down := false
	isDown := func() bool { mu.Lock(); defer mu.Unlock(); return down }
	body := strings.Repeat("x", 1<<20)
	inner := roundTrip(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	tr := &dlTransport{inner: inner, down: isDown, kbps: 1000}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1/a.mp3", http.NoBody)
	require.NoError(t, err)

	res, err := tr.RoundTrip(req)
	require.NoError(t, err)
	buf := make([]byte, 1<<20)
	n, err := res.Body.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, 1000<<10/10, n, "a tenth of a second of 1000 KiB/s")

	mu.Lock()
	down = true
	mu.Unlock()
	_, err = res.Body.Read(buf)
	assert.True(t, isNetErr(err), "the body is cut: %v", err)
	require.NoError(t, res.Body.Close())
	_, err = tr.RoundTrip(req)
	assert.True(t, isNetErr(err), "no request while down: %v", err)
}

// TestBadSHASource checks the developer's wrapper changes the sha256 of the
// answer only.
func TestBadSHASource(t *testing.T) {
	t.Parallel()

	tests := []string{strings.Repeat("a", 64), "0" + strings.Repeat("b", 63)}
	for _, sum := range tests {
		src := badSHASource{inner: audioSource{a: model.Audio{URL: "u", SizeBytes: 7, SHA256: sum}}}
		got, err := src.Audio(t.Context(), "l_1")
		require.NoError(t, err)
		assert.NotEqual(t, sum, got.SHA256)
		assert.Len(t, got.SHA256, 64)
		assert.Equal(t, int64(7), got.SizeBytes)
		assert.Equal(t, "u", got.URL)
	}
	_, err := badSHASource{inner: audioSource{err: api.ErrOffline}}.Audio(t.Context(), "l_1")
	assert.ErrorIs(t, err, api.ErrOffline)
}

// TestDownloadKnobs checks the variables a developer's build reads (none in
// the build the tests run).
func TestDownloadKnobs(t *testing.T) {
	t.Setenv(dlKBpsEnv, "64")
	t.Setenv(badSHAEnv, "1")
	var k wireKnobs
	dlKnobs(&k)
	if devBuild {
		assert.Equal(t, 64, k.dlKBps)
		assert.True(t, k.badSHA)
	} else {
		assert.Zero(t, k.dlKBps)
		assert.False(t, k.badSHA)
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// signIn signs in and returns Set 35, whose lessons the tests download.
func (d *dlRig) signIn() model.Course {
	d.t.Helper()
	ctx := d.t.Context()
	_, err := d.w.Restore(ctx)
	require.NoError(d.t, err)
	require.NoError(d.t, d.w.Login(ctx, "coach@example.invalid", "x"))
	d.drive("the downloads", func() bool { return d.w.downloads() != nil })
	d.q.drain()
	c, err := d.w.Course(ctx, demoCourseID)
	require.NoError(d.t, err)

	return c
}

// manager is the downloads the screens drive.
func (d *dlRig) manager() env.DownloadManager { return d.w.dls }

// jobDone is how many bytes of the lesson's download are in, as the manager
// knows it.
func (d *dlRig) jobDone(lessonID string) int64 {
	m := d.w.downloads()
	if m == nil {
		return 0
	}
	for _, j := range m.Jobs() {
		if j.LessonID == lessonID {
			return j.Done
		}
	}

	return 0
}

// kinds returns the kinds of the events heard.
func (d *dlRig) kinds() []env.DownloadEventKind {
	out := make([]env.DownloadEventKind, len(d.events))
	for i, ev := range d.events {
		out[i] = ev.Kind
	}

	return out
}

// last returns the last event of kind heard, the zero event when none.
func (d *dlRig) last(kind env.DownloadEventKind) env.DownloadEvent {
	for i := len(d.events) - 1; i >= 0; i-- {
		if d.events[i].Kind == kind {
			return d.events[i]
		}
	}

	return env.DownloadEvent{}
}

// progressFixture fills the cache of the wire's downloads with a file and
// a download under way, as reload would, without a manager.
func progressFixture() *wireDownloads {
	ds := newWireDownloads(&mobileWire{})
	ds.items = []env.DownloadItem{{LessonID: "l_3501", State: kit.DlDone, Bytes: 10, Done: 10, Progress: 1}}
	ds.itemAt["l_3501"] = 0
	ds.jobs = []env.DownloadItem{{LessonID: "l_3507", No: 7, State: kit.DlIng, Bytes: 9_800_000}}
	ds.jobAt["l_3507"] = 0

	return ds
}

// lessonOf returns the lesson numbered no of c.
func lessonOf(t *testing.T, c model.Course, no int) model.Lesson {
	t.Helper()
	i := slices.IndexFunc(c.Lessons, func(l model.Lesson) bool { return l.No == no })
	require.GreaterOrEqual(t, i, 0, "lesson %d", no)

	return c.Lessons[i]
}

// mediaBytes is the size of the file the media server answers for the
// lesson's audio.
func mediaBytes(t *testing.T, lessonID string) int64 {
	t.Helper()
	st, err := os.Stat(filepath.Join(mediaAudio, lessonID+".mp3"))
	require.NoError(t, err)

	return st.Size()
}

// mediaSHA is the sha256 of that file.
func mediaSHA(t *testing.T, lessonID string) string {
	t.Helper()

	return fileSHA(t, filepath.Join(mediaAudio, lessonID+".mp3"))
}

// fileSHA is the sha256 of a file, hex.
func fileSHA(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)

	return hex.EncodeToString(sum[:])
}

// isNetErr tells whether err is one the manager takes for a lost network.
func isNetErr(err error) bool {
	var ne interface{ Timeout() bool }

	return errors.As(err, &ne)
}

// Kind is the kind set.
func (n *fakeNet) Kind() sys.NetworkKind {
	n.mu.Lock()
	defer n.mu.Unlock()

	return n.kind
}

// OnChange keeps fn.
func (n *fakeNet) OnChange(fn func(sys.NetworkKind)) {
	n.mu.Lock()
	n.fn = fn
	n.mu.Unlock()
}

// Close marks the watch closed.
func (n *fakeNet) Close() {
	n.mu.Lock()
	n.closed = true
	n.mu.Unlock()
}

// set changes the kind and tells it, on the caller's goroutine (the test's
// UI thread).
func (n *fakeNet) set(k sys.NetworkKind) {
	n.mu.Lock()
	n.kind = k
	fn := n.fn
	n.mu.Unlock()
	if fn != nil {
		fn(k)
	}
}

// roundTrip is an http.RoundTripper of a func.
type roundTrip func(*http.Request) (*http.Response, error)

// RoundTrip calls the func.
func (f roundTrip) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// audioSource is a download.Source of one answer.
type audioSource struct {
	a   model.Audio
	err error
}

// Audio answers the answer.
func (s audioSource) Audio(context.Context, string) (model.Audio, error) { return s.a, s.err }

// TestWireWatchForeground checks defect 3.5: the services are an
// env.ForegroundWatcher that calls its watchers once for each return to the front
// (an Active that follows a Background); the first Active of a launch, an inactive
// moment and a second Active are not returns, and a stopped watch hears nothing.
func TestWireWatchForeground(t *testing.T) {
	t.Parallel()

	bg, act, ina := platform.LifecycleBackground, platform.LifecycleActive, platform.LifecycleInactive
	tests := []struct {
		name   string
		states []platform.LifecycleState
		want   int
	}{
		{"launch", []platform.LifecycleState{act}, 0},
		{"a return", []platform.LifecycleState{bg, act}, 1},
		{"a return after a sheet", []platform.LifecycleState{bg, ina, act}, 1},
		{"two returns", []platform.LifecycleState{act, bg, act, bg, act}, 2},
		{"active twice", []platform.LifecycleState{bg, act, act}, 1},
		{"inactive only", []platform.LifecycleState{ina, act}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := newDlRig(t, "", wireKnobs{})
			e := &env.Env{Services: d.w}
			n := 0
			stop := e.WatchForeground(func() { n++ })
			for _, st := range tc.states {
				d.w.lifecycle(st)
			}
			assert.Equal(t, tc.want, n)
			stop()
			d.w.lifecycle(bg)
			d.w.lifecycle(act)
			assert.Equal(t, tc.want, n, "a stopped watch hears nothing")
		})
	}
}
