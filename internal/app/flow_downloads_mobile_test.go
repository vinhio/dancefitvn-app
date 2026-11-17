//go:build !tvos

// End-to-end flows of the downloads and of the offline mode (P5.5): the real
// shell and screens over the real services, the mock API and its media
// server (through the downloads' own client, dlTransport), the virtual time
// of the Tester, a fake network, free space and background task, the fake
// engine, on both phones. The shape (a flow that drives frames between
// steps, a rig of fakes of the system under the services) is adapted from
// jplayer-mobile/internal/ui/views/flow_videos_test.go and flow_test.go at
// 49de3c2; the transfers' real time is the mock's (DFV_DL_KBPS throttle),
// never a sleep of the test.

package app

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"

	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The texts of the flows (locales/vi.json).
const (
	txtOfflineBar = "Không có mạng · Đang dùng chế độ ngoại tuyến"
	txtStarted    = "Đang tải file âm thanh…"
	txtCanceled   = "Đã huỷ tải"
	txtDeleted    = "Đã xoá file âm thanh"
	txtNoSpace    = "Không đủ bộ nhớ"
	txtWaitWiFi   = "Đang chờ Wi‑Fi để tải"
	txtFailed     = "Tải lỗi, thử lại"
	txtNoAudio    = "Bài này chưa có file âm thanh"
	txtNoVideo    = "Video bài 05 cần có mạng để xem"
)

// flowKBps is the speed of the downloads of a flow that must be caught on
// the way: a frame of the whole shell costs real time, which the throttle
// of the transfers (the mock's DFV_DL_KBPS) counts, so a file (481 KB)
// takes about 15 s, many frames.
const flowKBps = 32

// dlFlow is a playFlow with a fake network, a free space the test sets and
// the folder of the downloads under the phone's folder.
type dlFlow struct {
	*playFlow
	net   *fakeNet
	free  atomic.Int64
	vault *memSecrets
	dir   string
	audio string
}

// dlOpts are what a dlFlow starts with.
type dlOpts struct {
	secrets *memSecrets // the phone's vault; nil is a new phone with a session
	dir     string
	kind    sys.NetworkKind // at launch; the zero is Wi-Fi
	kbps    int             // the downloads' speed, KiB a second; 0 is as fast as the disk
	badSHA  bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDlFlow returns the flow of a signed-in coach whose downloads go to the
// phone's folder, on the network of o.kind.
func newDlFlow(t *testing.T, ph kittest.Phone, o dlOpts) *dlFlow {
	t.Helper()
	if o.secrets == nil {
		o.secrets, o.dir = &memSecrets{vals: map[string][]byte{}}, t.TempDir()
		seedSession(t, o.secrets, o.dir, sys.BiometryNone)
	}
	if o.kind == sys.NetworkUnknown {
		o.kind = sys.NetworkWiFi
	}
	df := &dlFlow{net: &fakeNet{kind: o.kind}, vault: o.secrets, dir: o.dir, audio: filepath.Join(o.dir, "nobackup", "audio")}
	df.free.Store(1 << 40)
	df.playFlow = newPlayFlowOn(t, ph, 220, o.secrets, o.dir, func(d *mobileDeps) {
		d.Network = func(func(func())) netWatch { return df.net }
		d.FreeSpace = func(string) (int64, int64, error) { return df.free.Load(), 2 << 40, nil }
		d.AudioDir = func() (string, error) { return df.audio, nil }
		d.BackgroundTask = func(string, func()) (func(), error) { return func() {}, nil }
	}, wireKnobs{dlKBps: o.kbps, badSHA: o.badSHA})

	return df
}

// seedDownloads is a launch of its own where the user signed in and
// downloaded the lessons numbered nos of Set 35, as a user who did so on
// another day; the phone's vault and folders are returned.
func seedDownloads(t *testing.T, nos ...int) (*memSecrets, string) {
	t.Helper()
	secrets, dir := &memSecrets{vals: map[string][]byte{}}, t.TempDir()
	d := newDlRigOn(t, secrets, dir, wireKnobs{})
	c := d.signIn()
	for _, no := range nos {
		l := lessonOf(t, c, no)
		require.NoError(t, d.manager().Start(c, []model.Lesson{l}))
		d.drive("the seeded file", func() bool { st, _ := d.manager().State(l.ID); return st == kit.DlDone })
	}
	d.w.close()

	return secrets, dir
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFlowDownloadOneToDelete follows a user through a whole download (03
// sections 3 and 8, 07 section 2): a tap on the cloud of lesson 05 opens
// sheet 09, the confirm queues it with its toast, the bytes come in (the
// detail's state, screen 11's "Đang tải · 1"), "Đã tải xong bài 05" shows
// when it is done, screen 11 lists it, 08 plays it and its chip tells the
// bytes on the device, and sheet 14 deletes it.
func TestFlowDownloadOneToDelete(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
		f.openDetail()
		const id = "l_3505"
		size := mediaBytes(t, id)

		f.tapName("dlTap:set35-5")
		f.drive("sheet 09", func() bool { return f.sh.Nav().Sheet().IsOpen() })
		f.tt.Elapse(400 * time.Millisecond)
		f.shows("Chỉ bài này")
		f.shows("Chỉ tải qua Wi‑Fi")
		f.tapName("dlConfirm")
		f.shows(txtStarted)
		f.drive("sheet 09 closed", func() bool { return !f.sh.Nav().Sheet().IsOpen() })

		// under way: the detail's state and screen 11
		f.bytesIn(id)
		st, p := f.sh.Env().Downloads().State(id)
		assert.Equal(t, kit.DlIng, st)
		assert.Greater(t, p, float32(0))
		assert.Less(t, p, float32(1))
		assert.Less(t, f.dlDone(id), size)
		f.sh.Go(nav.Route{Screen: env.RouteDownloads}, true) // the tab bar is not on the detail
		f.until(env.RouteDownloads)
		f.shows("Đang tải · 1")

		// done: the toast (in front), the row, the files' sha256
		f.shows("Đã tải xong bài 05")
		f.drive("the file", func() bool { st, _ := f.sh.Env().Downloads().State(id); return st == kit.DlDone })
		f.drive("the row", func() bool { return widgetNth(f.sh, "playAudio:set35-5", 1) != nil })
		f.gone("Đang tải · 1")
		f.settleFor(30) // the screen's own motion is over
		path, ok := f.w.downloads().Path(id)
		require.True(t, ok)
		assert.Equal(t, filepath.Join(f.audio, id+".mp3"), path)
		assert.Equal(t, mediaSHA(t, id), fileSHA(t, path))

		// 08: its chip tells the real bytes, not the catalog's
		f.tapName("playAudio:set35-5")
		f.until(env.RouteAudio)
		f.shows("Đã tải · " + format.Size(size))
		f.drive("it plays", func() bool { return f.sh.Env().Playback().State() == player.StatePlaying })
		loads := f.loads()
		assert.Equal(t, path, loads[len(loads)-1].Src.URL, "the file on the device")
		require.True(t, f.back())
		f.until(env.RouteDownloads)

		// 14: askDelete, delConfirm
		f.tapName("askDelete:set35-5")
		f.drive("sheet 14", func() bool { return f.sh.Nav().Sheet().IsOpen() })
		f.tt.Elapse(2300 * time.Millisecond) // the toast, over the button as in the prototype, is gone
		f.tapName("delConfirm")
		f.shows(txtDeleted)
		f.drive("the row gone", func() bool { return widgetNth(f.sh, "playAudio:set35-5", 1) == nil })
		assert.NoFileExists(t, path)
		assert.Zero(t, f.w.dls.DoneCount())
		st, _ = f.sh.Env().Downloads().State(id)
		assert.Equal(t, kit.DlNone, st)
	})
}

// TestFlowDownloadCancel checks that the cloud of a download under way
// cancels it with "Đã huỷ tải" and leaves no file (03 section 3, `dlTap`).
func TestFlowDownloadCancel(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
		f.openDetail()
		f.startDownload(5)
		f.bytesIn("l_3505")
		f.tapName("dlTap:set35-5")
		f.shows(txtCanceled)
		f.drive("no job", func() bool { return len(f.w.dls.Jobs()) == 0 })
		st, _ := f.sh.Env().Downloads().State("l_3505")
		assert.Equal(t, kit.DlNone, st)
		assert.Zero(t, f.w.dls.DoneCount())
		assert.NoFileExists(t, filepath.Join(f.audio, "l_3505.mp3"))
	})
}

// TestFlowDownloadFinishedToastIsForegroundOnly checks Q-P5-4: a file that
// finishes while the app is in the background gives no toast; one in front
// does.
func TestFlowDownloadFinishedToastIsForegroundOnly(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
		f.openDetail()
		f.startDownload(5)
		f.tt.Elapse(2300 * time.Millisecond) // the "started" toast is gone
		require.NotNil(t, f.life)
		f.life(platform.LifecycleBackground)
		f.drive("the file", func() bool { st, _ := f.sh.Env().Downloads().State("l_3505"); return st == kit.DlDone })
		f.settleFor(3)
		f.gone("Đã tải xong bài 05")

		f.life(platform.LifecycleActive)
		f.startDownload(4)
		f.shows("Đã tải xong bài 04")
	})
}

// TestFlowDownloadWiFiOnly checks K23 (03 section 2.2, Q-P5-2) through the
// shell: on mobile data with "Chỉ tải qua Wi‑Fi" the confirm closes the
// sheet with "Đang chờ Wi‑Fi để tải" (one toast, not "Đang tải file…"), the
// job waits paused, Wi-Fi starts it and it finishes with its toast; Wi-Fi to
// mobile data in the middle of a file pauses it and toasts once.
func TestFlowDownloadWiFiOnly(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("waits for wifi", func(t *testing.T) {
			f := newDlFlow(t, ph, dlOpts{kind: sys.NetworkCellular})
			f.openDetail()
			require.True(t, f.sh.Env().WiFiOnly(), "the preference is on at first")
			f.startDownloadWith(5, txtWaitWiFi)
			f.gone(txtStarted)
			f.drive("paused", func() bool { j := f.w.dls.Jobs(); return len(j) == 1 && j[0].Paused })
			assert.ErrorIs(t, f.w.dls.Jobs()[0].Err, env.ErrWiFiOnly)
			assert.Zero(t, f.dlDone("l_3505"), "no byte on mobile data")
			assert.True(t, f.w.Online(), "mobile data is online")

			f.tt.Elapse(2300 * time.Millisecond)
			f.settleFor(30)
			f.gone(txtWaitWiFi) // one toast for the pause, not one for each event

			f.net.set(sys.NetworkWiFi)
			f.shows("Đã tải xong bài 05")
			f.drive("the file", func() bool { st, _ := f.sh.Env().Downloads().State("l_3505"); return st == kit.DlDone })
			assert.Equal(t, mediaSHA(t, "l_3505"), fileSHA(t, filepath.Join(f.audio, "l_3505.mp3")))
		})
		t.Run("mobile data in the middle", func(t *testing.T) {
			f := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
			f.openDetail()
			f.startDownload(5)
			f.bytesIn("l_3505")
			f.tt.Elapse(2300 * time.Millisecond)
			f.net.set(sys.NetworkCellular)
			f.shows(txtWaitWiFi)
			f.drive("paused", func() bool { j := f.w.dls.Jobs(); return len(j) == 1 && j[0].Paused })
			assert.ErrorIs(t, f.w.dls.Jobs()[0].Err, env.ErrWiFiOnly)

			f.net.set(sys.NetworkWiFi)
			f.shows("Đã tải xong bài 05")
		})
	})
}

// TestFlowAirplane follows 03 section 2 with a download under way and a file
// on the device (07 section 2): the network gone pauses the download with
// its bytes and moves the detail to 13 with the offline bar; a video needs
// the network; the downloaded lesson plays from its file with no request to
// the server; the network back returns to the detail and the download goes
// on with one Range request, ending with the file's sha256 and its toast.
func TestFlowAirplane(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		secrets, dir := seedDownloads(t, 3)
		f := newDlFlow(t, ph, dlOpts{secrets: secrets, dir: dir, kbps: flowKBps})
		f.homeLoaded()
		f.tab(1)
		f.coursesLoaded() // cached before the network goes
		f.tab(0)
		f.openDetail()
		f.startDownload(5)
		f.bytesIn("l_3505")

		// the network goes
		f.net.set(sys.NetworkNone)
		assert.False(t, f.w.Online())
		f.until(env.RouteOffline)
		f.shows(txtOfflineBar)
		f.drive("paused", func() bool { j := f.w.dls.Jobs(); return len(j) == 1 && j[0].Paused })
		assert.ErrorIs(t, f.w.dls.Jobs()[0].Err, api.ErrOffline)
		have := f.dlDone("l_3505")
		require.Positive(t, have)
		require.Less(t, have, mediaBytes(t, "l_3505"))
		f.settleFor(60)
		assert.Equal(t, have, f.dlDone("l_3505"), "the ring is frozen")
		f.sh.PlayVideo(demoCourseID, "l_3505", 5)
		f.shows(txtNoVideo)
		f.sh.DlAll(demoCourseID)
		f.shows("Không có mạng để tải")

		// the downloaded lesson plays from the device
		audioCalls := f.st.calledN("lessons.audio")
		f.sh.OffTap(demoCourseID, "l_3503", 3)
		f.until(env.RouteAudio)
		pb := f.sh.Env().Playback()
		f.drive("the file plays", func() bool { return pb.State() == player.StatePlaying })
		path, ok := f.w.downloads().Path("l_3503")
		require.True(t, ok)
		loads := f.loads()
		assert.Equal(t, path, loads[len(loads)-1].Src.URL, "the local file, not a URL")
		assert.True(t, loads[len(loads)-1].Src.AudioOnly)
		assert.False(t, strings.HasPrefix(loads[len(loads)-1].Src.URL, "http"))
		assert.Equal(t, audioCalls, f.st.calledN("lessons.audio"), "no request for the audio")
		f.shows("Đã tải · " + format.Size(mediaBytes(t, "l_3503")))
		require.True(t, f.back())
		f.until(env.RouteOffline)
		assert.Equal(t, have, f.dlDone("l_3505"), "the download stayed paused")

		// the network is back: the detail, a Range request, the whole file
		f.net.set(sys.NetworkWiFi)
		f.drive("online again", f.w.Online)
		f.until(env.RouteDetail)
		f.drive("the file", func() bool { st, _ := f.sh.Env().Downloads().State("l_3505"); return st == kit.DlDone })
		resumes := 0
		for _, line := range f.log.all() {
			if strings.HasPrefix(line, "download: l_3505 resume at ") {
				resumes++
				assert.True(t, strings.HasSuffix(line, "(206)"), "a 206 from the offset: %s", line)
			}
		}
		assert.Equal(t, 1, resumes, "one Range request: %v", f.log.all())
		path, ok = f.w.downloads().Path("l_3505")
		require.True(t, ok)
		assert.Equal(t, mediaSHA(t, "l_3505"), fileSHA(t, path), "sha256 of the whole file")
		assert.NoFileExists(t, path+".part")
	})
}

// TestFlowAirplaneProgressIsSentOnReconnect checks 05 section 3 with a
// downloaded lesson: the position of a lesson played while the network is
// gone waits in the queue and is sent once in a batch when it is back.
func TestFlowAirplaneProgressIsSentOnReconnect(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		secrets, dir := seedDownloads(t, 3)
		f := newDlFlow(t, ph, dlOpts{secrets: secrets, dir: dir})
		f.startLesson3()
		q := f.w.queue.Load()
		require.NotNil(t, q)

		f.net.set(sys.NetworkNone)
		f.tt.Elapse(12 * time.Second)
		f.drive("a save queued", func() bool { return q.Len() >= 1 })
		assert.Zero(t, f.st.calledN("progress.batch"), "nothing is sent offline")
		f.until(env.RouteOffline)

		f.net.set(sys.NetworkWiFi)
		f.drive("online again", f.w.Online)
		f.drive("the flush", func() bool { return q.Len() == 0 && f.st.calledN("progress.batch") == 1 })
		f.settleFor(30)
		assert.Equal(t, 1, f.st.calledN("progress.batch"), "one batch")
	})
}

// TestFlowDownloadBadSHA checks 03 section 9: a file whose sha256 is wrong
// is deleted, "Tải lỗi, thử lại" shows, and the lesson goes back to no
// file with nothing left on the disk or in the pending jobs.
func TestFlowDownloadBadSHA(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{badSHA: true})
		f.openDetail()
		f.startDownloadWith(5, "")
		f.shows(txtFailed)
		f.drive("forgotten", func() bool { return len(f.w.dls.Jobs()) == 0 })
		st, _ := f.sh.Env().Downloads().State("l_3505")
		assert.Equal(t, kit.DlNone, st)
		assert.Zero(t, f.w.dls.DoneCount())
		f.drive("no part", func() bool {
			_, err := os.Stat(filepath.Join(f.audio, "l_3505.mp3.part"))

			return os.IsNotExist(err)
		})
		assert.NoFileExists(t, filepath.Join(f.audio, "l_3505.mp3"))
		index, err := os.ReadFile(filepath.Join(f.audio, "index.json"))
		if err == nil {
			assert.NotContains(t, string(index), "l_3505", "not in the pending jobs")
		}
	})
}

// TestFlowDownloadAudioNotFound checks a lesson whose audio the server does not
// have (lessons.audio answers 404 not_found, the facts of the course data being
// null): the toast says the audio is not there instead of "Tải lỗi, thử lại", the
// lesson's download button is gone for the run (no retry), the other lessons keep
// theirs, and a stale tap does not open the sheet.
func TestFlowDownloadAudioNotFound(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{})
		f.openDetail()
		f.st.failNext("lessons.audio", 1, http.StatusNotFound, "errors/not_found.json")
		f.startDownloadWith(5, "")
		f.shows(txtNoAudio)
		f.gone(txtFailed)
		f.drive("forgotten", func() bool { return len(f.w.dls.Jobs()) == 0 })
		st, _ := f.sh.Env().Downloads().State("l_3505")
		assert.Equal(t, kit.DlUnavailable, st)
		f.drive("the button goes", func() bool { return widgetNth(f.sh, "dlTap:set35-5", 1) == nil })
		assert.NotNil(t, widgetNth(f.sh, "dlTap:set35-6", 1))
		assert.Zero(t, f.w.dls.DoneCount())
		assert.True(t, f.w.downloads().Unavailable("l_3505"))

		f.sh.DlTap(demoCourseID, "l_3505", 5)
		f.settleFor(1)
		assert.False(t, f.sh.Nav().Sheet().IsOpen())
		f.shows(txtNoAudio)
		assert.Equal(t, 1, f.st.calledN("lessons.audio"), "the server is asked once")
	})
}

// TestFlowDownloadNoSpace checks 03 section 9: with less free space than the
// files need, the confirm toasts "Không đủ bộ nhớ", the sheet stays and
// nothing is queued; with room it goes on.
func TestFlowDownloadNoSpace(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{})
		f.free.Store(1 << 20)
		f.openDetail()
		f.drive("the free space", func() bool { _, _, ok := f.sh.Env().Downloader().FreeSpace(); return ok })
		f.tapName("dlTap:set35-5")
		f.drive("sheet 09", func() bool { return f.sh.Nav().Sheet().IsOpen() })
		f.tt.Elapse(400 * time.Millisecond)
		f.tapName("dlConfirm")
		f.shows(txtNoSpace)
		f.gone(txtStarted)
		assert.True(t, f.sh.Nav().Sheet().IsOpen(), "the sheet stays")
		assert.Empty(t, f.w.dls.Jobs(), "nothing queued")
		f.settleFor(30)
		assert.Empty(t, f.w.dls.Jobs())
		assert.Zero(t, f.w.dls.DoneCount())
	})
}

// TestFlowDownloadSignOut checks Q-P5-8 through the shell: a sign-out keeps
// the bytes and the record of a download under way and shows nothing of it;
// the same user back goes on with it and finishes.
func TestFlowDownloadSignOut(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
		f.openDetail()
		f.startDownload(5)
		f.bytesIn("l_3505")

		require.NoError(t, f.w.Logout(t.Context()))
		f.q.drain()
		f.sh.Go(nav.Route{Screen: env.RouteLogin}, true) // the account's sign-out is P6: it goes to 02
		f.until(env.RouteLogin)
		assert.Empty(t, f.sh.Env().Downloader().Jobs(), "nobody signed in: nothing shows")
		assert.FileExists(t, filepath.Join(f.audio, "l_3505.mp3.part"), "the bytes are kept")
		index, err := os.ReadFile(filepath.Join(f.audio, "index.json"))
		require.NoError(t, err)
		assert.Contains(t, string(index), "l_3505", "the record is kept")

		f.settleFor(30) // the login screen is in
		f.signIn("coach@example.invalid", "secret1")
		f.until(env.RouteHome)
		f.drive("the file", func() bool { st, _ := f.sh.Env().Downloads().State("l_3505"); return st == kit.DlDone })
		assert.Equal(t, mediaSHA(t, "l_3505"), fileSHA(t, filepath.Join(f.audio, "l_3505.mp3")))
	})
}

// TestFlowDownloadRestart checks F5: a download under way when the app ends
// is listed again at the next launch and goes on from its part, with a
// Range request, without a tap.
func TestFlowDownloadRestart(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		first := newDlFlow(t, ph, dlOpts{kbps: flowKBps})
		first.openDetail()
		first.startDownload(5)
		first.bytesIn("l_3505")
		first.w.close() // the process ends
		part := filepath.Join(first.audio, "l_3505.mp3.part")
		assert.FileExists(t, part)

		again := newDlFlow(t, ph, dlOpts{secrets: first.vault, dir: first.dir})
		again.homeLoaded()
		again.drive("the file", func() bool {
			m := again.w.downloads()

			return m != nil && slices.ContainsFunc(again.w.dls.Items(), func(it env.DownloadItem) bool { return it.LessonID == "l_3505" })
		})
		assert.Equal(t, mediaSHA(t, "l_3505"), fileSHA(t, filepath.Join(again.audio, "l_3505.mp3")))
		assert.True(t, slices.ContainsFunc(again.log.all(), func(s string) bool {
			return strings.HasPrefix(s, "download: l_3505 resume at ") && strings.HasSuffix(s, "(206)")
		}), "a Range request: %v", again.log.all())
		assert.NoFileExists(t, part)
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// startDownload taps lesson no's cloud of the detail, confirms sheet 09 and
// waits for the toast of the start.
func (f *dlFlow) startDownload(no int) { f.startDownloadWith(no, txtStarted) }

// startDownloadWith is startDownload with the toast the start ends with; ""
// waits for none (a download that fails at once replaces the toast of the
// start before a slow frame sees it).
func (f *dlFlow) startDownloadWith(no int, toast string) {
	f.t.Helper()
	f.tapName("dlTap:set35-" + string(rune('0'+no)))
	f.drive("sheet 09", func() bool { return f.sh.Nav().Sheet().IsOpen() })
	f.tt.Elapse(400 * time.Millisecond)
	f.tapName("dlConfirm")
	if toast != "" {
		f.shows(toast)
	}
}

// dlDone is how many bytes of the lesson's download are in.
func (f *dlFlow) dlDone(lessonID string) int64 {
	for _, j := range f.w.dls.Jobs() {
		if j.LessonID == lessonID {
			return j.Done
		}
	}

	return 0
}

// bytesIn runs frames, the fake clock moving 300 ms with each (the downloads
// throttle their progress events by it, 4 a second), until the lesson's
// download has bytes in the list the screens read.
func (f *dlFlow) bytesIn(lessonID string) {
	f.t.Helper()
	f.drive("bytes of "+lessonID, func() bool {
		f.clk.Advance(300 * time.Millisecond)

		return f.dlDone(lessonID) > 0
	})
}
