//go:build !tvos

// The rules of docs/plan/03 section 2 end to end, and the live refresh of the
// download state on the screens that show it (P5.3, M13).

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVideoGivesWayToOffline checks 03 section 2 on screen 06 (`$P:810-813`): the
// network going stops the video and puts the course's offline screen in its place,
// the offline bar on it; the network back puts the detail where it was.
func TestVideoGivesWayToOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		require.True(t, pb.Playing())
		depth := r.Shell.Nav().Depth()
		pb.ClearCalls()

		r.Fake.SetOnline(false)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Equal(t, env.RouteOffline, r.Route())
		assert.False(t, pb.Playing(), "the video stops")
		assert.Equal(t, player.StatePaused, pb.State())
		assert.Positive(t, pb.Count("Pause"))
		assert.Equal(t, depth, r.Shell.Nav().Depth(), "the stack is as it was")
		assert.True(t, r.Tester.Shows("Không có mạng · Đang dùng chế độ ngoại tuyến"))
		assert.True(t, r.Tester.Shows("DanceFIT Set 35"))

		r.Fake.SetOnline(true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Equal(t, env.RouteDetail, r.Route())
	})
}

// TestAudioGoesOnOffline checks that the network going leaves 08 alone (its file may
// be on the device) and that the chip says so.
func TestAudioGoesOnOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atAudio(t, ph, false)
		r.Fake.SetOnline(false)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		assert.Equal(t, env.RouteAudio, r.Route())
		r.Fake.SetOnline(true)
		r.Settle()
		assert.Equal(t, env.RouteAudio, r.Route())
	})
}

// TestDoneStaysOffline checks the prototype's `offTog`, which leaves screen 10 alone.
func TestDoneStaysOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atDone(t, ph, false)
		r.Fake.SetOnline(false)
		r.Settle()
		assert.Equal(t, env.RouteDone, r.Route())
		r.Fake.SetOnline(true)
		r.Settle()
		assert.Equal(t, env.RouteDone, r.Route())
	})
}

// TestDetailFollowsDownloads checks the live refresh of the detail: the ring of a
// lesson that starts, moves with the progress events and becomes the done mark with
// the finish; the count "n/10 audio đã tải" follows the files; a lesson canceled goes
// back to no file. The rows are the same widgets throughout.
func TestDetailFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		rows := detailOf(t, r).Content().Rows()
		dl := r.Fake.Dl()
		require.True(t, r.Tester.Shows("2/10 đã xem · 5/10 audio đã tải"))
		require.Equal(t, kit.DlNone, rows[4].Download)
		require.Equal(t, kit.DlIng, rows[6].Download, "7 is at 64 %")
		assert.InDelta(t, .64, rows[6].Progress, .001)

		dl.SetDownload("l_3505", kit.DlIng, .1)
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlIng, rows[4].Download, "05 started")
		dl.SetDownload("l_3505", kit.DlIng, .55) // a progress
		r.Tester.Elapse(kittest.Frame)
		assert.InDelta(t, .55, rows[4].Progress, .001)
		dl.SetDownload("l_3507", kit.DlIng, .9)
		assert.InDelta(t, .9, rows[6].Progress, .001)

		dl.FinishDownload("l_3505")
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlDone, rows[4].Download)
		assert.True(t, r.Tester.Shows("2/10 đã xem · 6/10 audio đã tải"), "the count follows")

		dl.Forget("l_3507")
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlNone, rows[6].Download)
		require.NoError(t, dl.Delete("l_3501"))
		r.Tester.Elapse(kittest.Frame)
		assert.Equal(t, kit.DlNone, rows[0].Download)
		assert.True(t, r.Tester.Shows("2/10 đã xem · 5/10 audio đã tải"))
		assert.Same(t, rows[0], detailOf(t, r).Content().Rows()[0])
	})
}

// TestDetailProgressEventDoesNotAllocate checks that a progress event the detail
// takes, moving a ring, allocates nothing, and nor does one that changes nothing.
func TestDetailProgressEventDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atDetail(t, ph, false, "set-35")
		warmDetail(r)
		topic := r.Fake.Dl().Events()
		p := int64(0)
		ev := func() {
			p = (p + 1) % 100
			topic.Publish(env.DownloadEvent{Kind: env.DownloadEventProgress, LessonID: "l_3507", Done: p, Total: 100})
		}
		ev()
		assert.Zero(t, testing.AllocsPerRun(50, ev), ph.Name+": the ring moves")
		changed := func() { topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged}) }
		changed()
		assert.Zero(t, testing.AllocsPerRun(50, changed), ph.Name+": nothing changed")
	}
}

// TestVideoSideButtonFollowsDownloads checks the live label of the side button:
// "Đã tải" with the file, "Âm thanh" once it is deleted, "Đang tải" while it comes.
func TestVideoSideButtonFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		dl := r.Fake.Dl()
		assert.True(t, r.Tester.Shows("Đã tải"), "lesson 3 has its file")
		require.NoError(t, dl.Delete("l_3503"))
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("Âm thanh"))
		assert.False(t, r.Tester.Shows("Đã tải"))
		dl.SetDownload("l_3503", kit.DlIng, .2)
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("Đang tải"))
		dl.FinishDownload("l_3503")
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("Đã tải"))
	})
}

// TestAudioChipFollowsDownloads checks the chip of 08: "Đã tải · size" with the file
// on the device, "Đang phát trực tuyến" without it, as the file goes and comes.
func TestAudioChipFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atAudio(t, ph, false)
		dl := r.Fake.Dl()
		assert.False(t, r.Tester.Shows("Đang phát trực tuyến"))
		require.NoError(t, dl.Delete("l_3503"))
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("Đang phát trực tuyến"), "streaming")
		dl.SetDownload("l_3503", kit.DlDone, 1)
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, r.Tester.Shows("Đang phát trực tuyến"))
	})
}

// TestHomeAudioOfflineFollowsDownloads checks the number of "Audio offline" on Home
// following the files, in place.
func TestHomeAudioOfflineFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		dl := r.Fake.Dl()
		require.True(t, r.Tester.Shows("Audio offline"))
		require.True(t, r.Tester.Shows("5"))
		require.NoError(t, dl.Delete("l_3501"))
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("4"))
		dl.SetDownload("l_3505", kit.DlDone, 1)
		dl.SetDownload("l_3507", kit.DlDone, 1)
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows("6"))
	})
}

// TestHomeOfflineEventDoesNotAllocate checks that an event of the downloads that
// leaves the number of Home as it was allocates nothing.
func TestHomeOfflineEventDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	r := atHome(t, kittest.Phones[0], false)
	warmDetail(r)
	topic := r.Fake.Dl().Events()
	changed := func() { topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged}) }
	changed()
	assert.Zero(t, testing.AllocsPerRun(50, changed))
}

// TestVideoControlsStayWithTheDownloadSheet checks F-14: the sheet the shell opens
// from the side button (sheet 14 for the file of lesson 3) keeps the controls
// shown past the 3 s, and closing it starts the 3 s again.
func TestVideoControlsStayWithTheDownloadSheet(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		tap(r, named(r.Tester.View().Root(), "dlTap:set35-3", 1))
		r.Tester.Elapse(300 * time.Millisecond)
		require.True(t, r.Shell.Nav().Sheet().IsOpen(), "sheet 14 is open")
		r.Tester.Elapse(10 * time.Second)
		assert.True(t, v.Shown(), "the sheet keeps the controls")
		assert.InDelta(t, 1, v.Opacity(), 0)

		r.Shell.Back()
		r.Tester.Elapse(600 * time.Millisecond)
		require.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, v.Shown(), "just closed")
		r.Tester.Elapse(ms3s)
		assert.False(t, v.Shown(), "3 s after it closed the controls hide")
	})
}

// TestDownloadsPausedCardWaits checks K23: the card of a paused download reads "Đang
// chờ Wi‑Fi để tải" in place of its progress text, the bar keeping its share, and
// goes back to the progress when it resumes.
func TestDownloadsPausedCardWaits(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		assert.True(t, r.Tester.Shows("6,3 / 9,8 MB · 64%"))
		assert.False(t, r.Tester.Shows(toastWaitWiFi))
		r.Fake.Dl().PauseDownload("l_3507", env.ErrWiFiOnly)
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows(toastWaitWiFi))
		assert.False(t, r.Tester.Shows("6,3 / 9,8 MB · 64%"))
		r.Tester.Elapse(3 * time.Second) // the shell's toast of the same words (2200 ms) is gone; the card's line stays
		assert.True(t, r.Tester.Shows(toastWaitWiFi))
		r.Fake.Dl().PauseDownload("l_3507", api.ErrOffline)
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Tester.Shows(toastWaitWiFi), "paused for the network: the same line")
		r.Fake.Dl().Resume()
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, r.Tester.Shows(toastWaitWiFi))
		assert.True(t, r.Tester.Shows("6,3 / 9,8 MB · 64%"))
	})
}
