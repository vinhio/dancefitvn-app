//go:build !tvos

package shell_test

import (
	"errors"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
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

// downloadSheet returns the content of the sheet 09 that is open.
func downloadSheet(t *testing.T, r *shelltest.Rig) *screens.DownloadSheet {
	t.Helper()
	require.True(t, r.Shell.Nav().Sheet().IsOpen(), "a sheet is open")
	s, ok := r.Shell.Nav().Sheet().Content().(*screens.DownloadSheet)
	require.True(t, ok, "it is sheet 09")

	return s
}

// shows runs the toast in (a frame) and tells whether msg shows.
func shows(r *shelltest.Rig, msg string) bool {
	r.Tester.Elapse(100 * ms)

	return r.Tester.Shows(msg)
}

// TestDlTapByState checks `dlTap` (03 section 3, `$P:598`) for a lesson with no
// file, one under way and one downloaded, online and offline: sheet 09 on "Chỉ
// bài này", the cancel with its toast, sheet 14.
func TestDlTapByState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lesson  string // l_3505 has no file, l_3507 is at 64 %, l_3503 is a file
		no      int
		offline bool
		check   func(t *testing.T, r *shelltest.Rig)
	}{
		{name: "none online opens sheet 09 on one", lesson: "l_3505", no: 5, check: func(t *testing.T, r *shelltest.Rig) {
			s := downloadSheet(t, r)
			assert.Equal(t, "l_3505", s.Params.LessonID)
			assert.Equal(t, env.DownloadChoiceOne, s.Params.Choice)
			assert.Equal(t, "set-35", s.Params.Course.ID)
			assert.Empty(t, r.Fake.Dl().Started(), "nothing starts before the confirm")
		}},
		{name: "none offline toasts", lesson: "l_3505", no: 5, offline: true, check: func(t *testing.T, r *shelltest.Rig) {
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastNoNet))
		}},
		{name: "running is canceled", lesson: "l_3507", no: 7, check: func(t *testing.T, r *shelltest.Rig) {
			assert.Equal(t, []string{"l_3507"}, r.Fake.Dl().Cancelled())
			st, _ := r.Fake.Dl().State("l_3507")
			assert.Equal(t, kit.DlNone, st)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastCanceled))
		}},
		{name: "running is canceled offline too", lesson: "l_3507", no: 7, offline: true, check: func(t *testing.T, r *shelltest.Rig) {
			assert.Equal(t, []string{"l_3507"}, r.Fake.Dl().Cancelled())
			assert.True(t, shows(r, toastCanceled))
		}},
		{name: "done opens sheet 14", lesson: "l_3503", no: 3, check: func(t *testing.T, r *shelltest.Rig) {
			require.True(t, r.Shell.Nav().Sheet().IsOpen())
			s, ok := r.Shell.Nav().Sheet().Content().(*screens.DeleteSheet)
			require.True(t, ok, "it is sheet 14")
			assert.Equal(t, env.DeleteSheetParams{CourseID: "set-35", LessonID: "l_3503", No: 3}, s.Params)
			assert.Empty(t, r.Fake.Dl().Deleted(), "nothing goes before the confirm")
		}},
		{name: "done opens sheet 14 offline", lesson: "l_3503", no: 3, offline: true, check: func(t *testing.T, r *shelltest.Rig) {
			_, ok := r.Shell.Nav().Sheet().Content().(*screens.DeleteSheet)
			assert.True(t, ok)
		}},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, _ := atCourse(t, ph)
				if tc.offline {
					r.Fake.SetOnline(false)
					r.Settle()
				}
				// the detail swaps to the offline screen with the network; the action is the same
				r.Shell.DlTap("set-35", tc.lesson, tc.no)
				r.Settle()
				tc.check(t, r)
			})
		}
	})
}

// TestDlAllPicksTheFirstMissing checks `dlAll` (`$P:602`): sheet 09 on "Cả khoá",
// its lesson the first without a file (lesson 1 when every lesson has one), and
// the toast offline.
func TestDlAllPicksTheFirstMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		done []int // lessons with a file, 7 is running in the demo
		want string
	}{
		{name: "demo: 5 is the first missing", done: nil, want: "l_3505"},
		{name: "nothing downloaded", done: []int{}, want: "l_3501"},
		{name: "1 to 6 done and 7 running", done: []int{1, 2, 3, 4, 5, 6}, want: "l_3508"},
		{name: "every lesson done", done: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, want: "l_3501"},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, _ := atCourse(t, ph)
				if tc.done != nil {
					r.Fake.Dl().Clear()
					for _, no := range tc.done {
						r.Fake.Dl().SetDownload(lessonID(no), kit.DlDone, 1)
					}
					if len(tc.done) == 6 {
						r.Fake.Dl().SetDownload(lessonID(7), kit.DlIng, .1)
					}
				}
				r.Shell.DlAll("set-35")
				r.Settle()
				s := downloadSheet(t, r)
				assert.Equal(t, tc.want, s.Params.LessonID)
				assert.Equal(t, env.DownloadChoiceAll, s.Params.Choice)
			})
		}

		t.Run("offline toasts", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Fake.SetOnline(false)
			r.Settle()
			r.Shell.DlAll("set-35")
			r.Settle()
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.True(t, shows(r, toastNoNet))
		})

		t.Run("a course not read yet is fetched", func(t *testing.T) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			r := shelltest.MountRig(t, ph, false, f, shell.Options{})
			r.Boot()
			_, cached := r.Shell.Env().CachedCourse("set-35")
			require.False(t, cached)
			r.Shell.DlAll("set-35")
			r.Settle()
			assert.Equal(t, "l_3505", downloadSheet(t, r).Params.LessonID)
		})
	})
}

// lessonID is the id of lesson no of Set 35 in the mock.
func lessonID(no int) string {
	if no == 10 {
		return "l_3510"
	}

	return "l_350" + string(rune('0'+no))
}

// TestStartDownloads checks `dlConfirm` (`$P:600`) and its rules of 03 section 9:
// what it starts, in order, skipping the lessons done or under way; the toast;
// the sheet closing; no space; Wi-Fi only on mobile data (K23); offline; a
// refusal.
func TestStartDownloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(f *shelltest.Fake)
		all       bool // every lesson of the course, else lesson 5 only
		toast     string
		started   []string
		sheetOpen bool
		paused    bool // the queued jobs wait
	}{
		{name: "one lesson", toast: toastStarted, started: []string{"l_3505"}},
		{name: "the whole course skips done and running", all: true, toast: toastStarted,
			started: []string{"l_3505", "l_3508", "l_3509", "l_3510"}},
		{name: "no space", setup: func(f *shelltest.Fake) { f.Dl().SetSpace(1_000_000, 62_000_000_000) },
			toast: toastNoSpace, sheetOpen: true},
		{name: "space for the files and no more", setup: func(f *shelltest.Fake) { f.Dl().SetSpace(8_400_000, 62_000_000_000) },
			toast: toastStarted, started: []string{"l_3505"}},
		{name: "space unknown starts", setup: func(f *shelltest.Fake) { f.Dl().ClearSpace() },
			toast: toastStarted, started: []string{"l_3505"}},
		{name: "wifi only on mobile data queues paused", setup: func(f *shelltest.Fake) { f.Dl().SetOnWiFi(false) },
			toast: toastWaitWiFi, started: []string{"l_3505"}, paused: true},
		{name: "mobile data with the preference off downloads", setup: func(f *shelltest.Fake) {
			f.Dl().SetOnWiFi(false)
			f.FakePrefs().SetWiFiOnly(false)
		}, toast: toastStarted, started: []string{"l_3505"}},
		{name: "start says offline", setup: func(f *shelltest.Fake) { f.Dl().SetStartErr(api.ErrOffline) },
			toast: toastNoNet, sheetOpen: true},
		{name: "a refusal", setup: func(f *shelltest.Fake) { f.Dl().SetStartErr(errors.New("403")) },
			toast: toastFailed, sheetOpen: true},
		{name: "start says no space", setup: func(f *shelltest.Fake) { f.Dl().SetStartErr(env.ErrNoSpace) },
			toast: toastNoSpace, sheetOpen: true},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, _ := atCourse(t, ph)
				if tc.setup != nil {
					tc.setup(r.Fake)
				}
				c, _ := r.Shell.Env().CachedCourse("set-35")
				r.Shell.DlAll("set-35")
				r.Settle()
				require.True(t, r.Shell.Nav().Sheet().IsOpen())
				lessons := c.Lessons
				if !tc.all {
					lessons = c.Lessons[4:5]
				}
				r.Shell.StartDownloads(c, lessons)
				assert.Equal(t, tc.sheetOpen, r.Shell.Nav().Sheet().IsOpen(), "the sheet")
				assert.True(t, shows(r, tc.toast), tc.toast)
				assert.Equal(t, append([]string(nil), tc.started...), append([]string(nil), r.Fake.Dl().Started()...))
				if tc.paused {
					for _, j := range r.Fake.Dl().Jobs() {
						if j.LessonID == "l_3505" {
							assert.True(t, j.Paused)
							assert.ErrorIs(t, j.Err, env.ErrWiFiOnly)
						}
					}
				}
			})
		}
	})
}

// TestWiFiBackStartsTheQueue checks K23: the lesson queued on mobile data waits,
// and goes on when the network is Wi-Fi.
func TestWiFiBackStartsTheQueue(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atCourse(t, ph)
		r.Fake.Dl().SetOnWiFi(false)
		c, _ := r.Shell.Env().CachedCourse("set-35")
		r.Shell.StartDownloads(c, c.Lessons[4:5])
		jobs := r.Fake.Dl().Jobs()
		require.Len(t, jobs, 2) // 7 was running; it is paused by the mobile data too
		assert.True(t, jobs[0].Paused)
		r.Fake.Dl().SetOnWiFi(true)
		for _, j := range r.Fake.Dl().Jobs() {
			assert.False(t, j.Paused, j.LessonID)
		}
	})
}

// TestDeleteDownload checks `delConfirm` (`$P:604`): the file goes, the sheet
// closes, the toast shows; a failure closes the sheet and logs.
func TestDeleteDownload(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("deleted", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Shell.AskDelete("set-35", "l_3503", 3)
			require.True(t, r.Shell.Nav().Sheet().IsOpen())
			r.Shell.DeleteDownload("l_3503")
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Equal(t, []string{"l_3503"}, r.Fake.Dl().Deleted())
			assert.Equal(t, 4, r.Fake.Dl().DoneCount())
			st, _ := r.Fake.Dl().State("l_3503")
			assert.Equal(t, kit.DlNone, st)
			assert.True(t, shows(r, toastDeleted))
		})
		t.Run("a failure shows no toast", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Fake.Dl().SetDeleteErr(errors.New("disk"))
			r.Shell.AskDelete("set-35", "l_3503", 3)
			r.Shell.DeleteDownload("l_3503")
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.False(t, shows(r, toastDeleted))
			assert.Equal(t, 5, r.Fake.Dl().DoneCount())
		})
	})
}

// TestDownloadEventsToast checks what the shell does with the events (Q-P5-4,
// 03 section 9): "Đã tải xong bài NN" on a finish; "Không đủ bộ nhớ" or "Tải lỗi,
// thử lại" on a failure, the download forgotten so that its row is none again; a
// pause says nothing.
func TestDownloadEventsToast(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("finished", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Fake.Dl().FinishDownload("l_3507")
			assert.True(t, shows(r, "Đã tải xong bài 07"))
			assert.Equal(t, 6, r.Fake.Dl().DoneCount())
		})
		for name, tc := range map[string]struct {
			err   error
			toast string
		}{
			"checksum":  {err: env.ErrChecksum, toast: toastFailed},
			"no space":  {err: env.ErrNoSpace, toast: toastNoSpace},
			"a refusal": {err: errors.New("403"), toast: toastFailed},
		} {
			t.Run("failed "+name, func(t *testing.T) {
				r, _ := atCourse(t, ph)
				r.Fake.Dl().FailDownload("l_3507", tc.err)
				assert.True(t, shows(r, tc.toast))
				assert.Equal(t, []string{"l_3507"}, r.Fake.Dl().Cancelled(), "forgotten")
				st, _ := r.Fake.Dl().State("l_3507")
				assert.Equal(t, kit.DlNone, st)
			})
		}
		t.Run("finished in the background says nothing (Q-P5-4)", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Fake.SetForeground(false)
			r.Fake.Dl().FinishDownload("l_3507")
			assert.False(t, shows(r, "Đã tải xong bài 07"), "no stale toast for the user coming back")
			assert.Equal(t, 6, r.Fake.Dl().DoneCount(), "the file is listed all the same")
			st, _ := r.Fake.Dl().State("l_3507")
			assert.Equal(t, kit.DlDone, st)
			r.Fake.SetForeground(true)
			r.Tester.Elapse(3 * time.Second)
			assert.False(t, r.Tester.Shows("Đã tải xong bài 07"), "and none when it is back")

			r.Fake.Dl().SetDownload("l_3508", kit.DlIng, .5)
			r.Fake.Dl().FinishDownload("l_3508")
			assert.True(t, shows(r, "Đã tải xong bài 08"), "in front again: the toast")
		})
		t.Run("paused for the network says nothing", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			r.Fake.Dl().PauseDownload("l_3507", api.ErrOffline)
			r.Tester.Elapse(100 * ms)
			assert.False(t, r.Tester.Shows(toastFailed))
			assert.Empty(t, r.Fake.Dl().Cancelled())
		})
	})
}

// TestOffTapByState checks `offTap` (`$P:598`) for a lesson with its file, one
// under way and one with none, with the network and without it.
func TestOffTapByState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lesson  string
		no      int
		offline bool
		plays   bool
		toast   string
	}{
		{name: "done plays", lesson: "l_3504", no: 4, plays: true},
		{name: "done plays offline", lesson: "l_3504", no: 4, offline: true, plays: true},
		{name: "running toasts", lesson: "l_3507", no: 7, toast: "Video bài 07 cần có mạng để xem"},
		{name: "none toasts", lesson: "l_3509", no: 9, toast: "Video bài 09 cần có mạng để xem"},
		{name: "none toasts offline", lesson: "l_3509", no: 9, offline: true, toast: "Video bài 09 cần có mạng để xem"},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, pb := atCourse(t, ph)
				if tc.offline {
					r.Fake.SetOnline(false)
					r.Settle()
				}
				pb.ClearCalls()
				r.Shell.OffTap("set-35", tc.lesson, tc.no)
				r.Settle()
				if tc.plays {
					assert.Equal(t, env.RouteAudio, r.Route())
					assert.Equal(t, []string{"PlayAudio:" + tc.lesson}, pb.Calls())

					return
				}
				assert.Empty(t, pb.Calls())
				assert.True(t, shows(r, tc.toast))
			})
		}
	})
}

// TestDownloadsRouteIsATab checks the route 11: the tab "Tải xuống" opens it with
// the tab bar on it.
func TestDownloadsRouteIsATab(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _ := atCourse(t, ph)
		r.Shell.Go(nav.Route{Screen: env.RouteDownloads}, true)
		r.Settle()
		assert.Equal(t, env.RouteDownloads, r.Route())
		d, ok := r.Shell.Nav().Screen().(*screens.Downloads)
		assert.True(t, ok, "screen 11")
		if ok {
			assert.Equal(t, 2, d.Tab())
		}
	})
}

// TestWaitWiFiToastOncePerPause checks Q-P5-4's sibling (03 section 2, K23): the
// network going to mobile data with "Chỉ tải qua Wi‑Fi" on pauses every download
// under way, each telling DownloadEventPaused with ErrWiFiOnly, and "Đang chờ
// Wi‑Fi để tải" shows once for the transition, not once per job; it speaks again
// at the next one, after the downloads went on.
func TestWaitWiFiToastOncePerPause(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		// gone waits the toast out (2200 ms) so that a second one would show again.
		gone := func(r *shelltest.Rig) {
			r.Tester.Elapse(3 * time.Second)
			require.False(t, r.Tester.Shows(toastWaitWiFi), "the toast is gone")
		}
		twoRunning := func(t *testing.T) *shelltest.Rig {
			r, _ := atCourse(t, ph)
			dl := r.Fake.Dl()
			dl.SetDownload("l_3508", kit.DlIng, .2)
			dl.SetDownload("l_3509", kit.DlIng, .3)
			require.Len(t, dl.Jobs(), 3, "three downloads under way")
			r.Tester.Elapse(3 * time.Second)

			return r
		}

		t.Run("one toast for the jobs of a switch", func(t *testing.T) {
			r := twoRunning(t)
			r.Fake.Dl().SetOnWiFi(false) // three Paused events, ErrWiFiOnly
			assert.True(t, shows(r, toastWaitWiFi))
			for _, j := range r.Fake.Dl().Jobs() {
				assert.True(t, j.Paused, j.LessonID)
			}
		})
		t.Run("not again for a later job of the same pause", func(t *testing.T) {
			r := twoRunning(t)
			dl := r.Fake.Dl()
			dl.PauseDownload("l_3507", env.ErrWiFiOnly)
			assert.True(t, shows(r, toastWaitWiFi))
			gone(r)
			dl.PauseDownload("l_3508", env.ErrWiFiOnly)
			assert.False(t, shows(r, toastWaitWiFi), "the second job of the same transition")
			dl.PauseDownload("l_3509", env.ErrWiFiOnly)
			assert.False(t, shows(r, toastWaitWiFi), "nor the third")
		})
		t.Run("again at the next transition", func(t *testing.T) {
			r := twoRunning(t)
			dl := r.Fake.Dl()
			dl.SetOnWiFi(false)
			assert.True(t, shows(r, toastWaitWiFi))
			gone(r)
			dl.SetOnWiFi(true) // Wi-Fi is back: the downloads go on
			gone(r)
			dl.SetOnWiFi(false)
			assert.True(t, shows(r, toastWaitWiFi), "a new switch to mobile data")
		})
		t.Run("a start queued on mobile data and its paused events toast once", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			dl := r.Fake.Dl()
			dl.SetOnWiFi(false)
			c, _ := r.Shell.Env().CachedCourse("set-35")
			r.Shell.StartDownloads(c, c.Lessons[4:5])
			assert.True(t, shows(r, toastWaitWiFi))
			gone(r)
			dl.PauseDownload("l_3505", env.ErrWiFiOnly) // the manager's own event for the queued job
			assert.False(t, shows(r, toastWaitWiFi), "the start's toast was the one")
		})
		t.Run("other pauses say nothing", func(t *testing.T) {
			r := twoRunning(t)
			r.Fake.Dl().PauseDownload("l_3508", api.ErrOffline)
			assert.False(t, shows(r, toastWaitWiFi))
			r.Fake.Dl().PauseDownload("l_3509", errors.New("background time over"))
			assert.False(t, shows(r, toastWaitWiFi))
		})
	})
}

// TestBackgroundToastsAreForTheAppInFront checks N5: like "Đã tải xong bài NN", the
// toasts of a download that failed and of one held back for Wi-Fi are for the app in
// front; in the background they would be stale when the user comes back. The
// download that failed is forgotten all the same (its row goes back to no file),
// and a toast speaks again once the app is in front.
func TestBackgroundToastsAreForTheAppInFront(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for name, tc := range map[string]struct {
			err   error
			toast string
		}{
			"failed":   {err: errors.New("403"), toast: toastFailed},
			"checksum": {err: env.ErrChecksum, toast: toastFailed},
			"no space": {err: env.ErrNoSpace, toast: toastNoSpace},
		} {
			t.Run("failed in the background says nothing: "+name, func(t *testing.T) {
				r, _ := atCourse(t, ph)
				dl := r.Fake.Dl()
				r.Fake.SetForeground(false)
				dl.FailDownload("l_3507", tc.err)
				assert.False(t, shows(r, tc.toast), "no stale toast")
				assert.Equal(t, []string{"l_3507"}, dl.Cancelled(), "forgotten all the same")
				st, _ := dl.State("l_3507")
				assert.Equal(t, kit.DlNone, st)

				r.Fake.SetForeground(true)
				r.Tester.Elapse(3 * time.Second)
				assert.False(t, r.Tester.Shows(tc.toast), "and none when it is back")
				dl.SetDownload("l_3508", kit.DlIng, .5)
				dl.FailDownload("l_3508", tc.err)
				assert.True(t, shows(r, tc.toast), "in front again: the toast")
			})
		}
		t.Run("held back for Wi-Fi in the background says nothing", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			dl := r.Fake.Dl()
			dl.SetDownload("l_3508", kit.DlIng, .2)
			r.Tester.Elapse(3 * time.Second)
			r.Fake.SetForeground(false)
			dl.SetOnWiFi(false) // the jobs under way pause with ErrWiFiOnly
			assert.False(t, shows(r, toastWaitWiFi), "no stale toast")
			for _, j := range dl.Jobs() {
				assert.True(t, j.Paused, j.LessonID+" is paused all the same")
			}

			r.Fake.SetForeground(true)
			r.Tester.Elapse(3 * time.Second)
			assert.False(t, r.Tester.Shows(toastWaitWiFi), "and none when it is back")
			dl.PauseDownload("l_3508", env.ErrWiFiOnly) // a pause the user is there for
			assert.True(t, shows(r, toastWaitWiFi), "in front again: the toast")
		})
		t.Run("a start queued on mobile data is the user's own and shows", func(t *testing.T) {
			r, _ := atCourse(t, ph)
			dl := r.Fake.Dl()
			dl.SetOnWiFi(false)
			c, _ := r.Shell.Env().CachedCourse("set-35")
			r.Shell.StartDownloads(c, c.Lessons[4:5])
			assert.True(t, shows(r, toastWaitWiFi))
		})
	})
}
