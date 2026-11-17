package download_test

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
)

func TestRangeResumeAfterDrop(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	r.srv.cutAt = func(n int) int {
		if n == 1 {
			return 100_000
		}

		return 0
	}
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	ev := r.ev.wait(t, download.EventPaused, "l_3503")
	require.ErrorIs(t, ev.Err, api.ErrOffline)

	part := filepath.Join(r.dir, "l_3503.mp3.part")
	st, err := os.Stat(part)
	require.NoError(t, err)
	have := st.Size()
	require.Positive(t, have)
	require.Less(t, have, int64(250_000))
	require.Equal(t, download.StatePaused, r.mgr.Jobs()[0].State)

	r.mgr.ResumeAll()
	r.ev.wait(t, download.EventFinished, "l_3503")

	require.Equal(t, []string{"", "bytes=" + strconv.FormatInt(have, 10) + "-"}, r.srv.requests())
	require.Equal(t, []string{"", `"v1"`}, r.srv.ifRanges(), "If-Range carries the ETag seen")
	path, ok := r.mgr.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, r.body, readAll(t, path))
	require.NoFileExists(t, part)
	require.Empty(t, r.mgr.Jobs())
	require.Equal(t, 1, r.src.count(), "URL still fresh on resume")
}

func TestResumesPartFromDisk(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	require.NoError(t, os.WriteFile(filepath.Join(r.dir, "l_3503.mp3.part"), r.body[:1000], 0o600))
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.Equal(t, []string{"bytes=1000-"}, r.srv.requests())
	require.Equal(t, []string{""}, r.srv.ifRanges(), "no validator after a restart of the app")
	path, _ := r.mgr.Path("l_3503")
	require.Equal(t, r.body, readAll(t, path))
}

func TestServerIgnoresRange(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	r.srv.cutAt = func(n int) int {
		if n == 1 {
			return 100_000
		}

		return 0
	}
	r.srv.ignoreRange = func(int) bool { return true }
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventPaused, "l_3503")
	r.mgr.ResumeAll()
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.Len(t, r.srv.requests(), 2)
	require.NotEmpty(t, r.srv.requests()[1], "the client did ask for a range")
	path, _ := r.mgr.Path("l_3503")
	require.Equal(t, r.body, readAll(t, path), "200 restarted from zero, no duplicated prefix")
}

func TestRangeProblemsRestart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		setup func(s *srv)
	}{
		{"416 on the range", func(s *srv) {
			s.status = func(n int, r *http.Request) int {
				if r.Header.Get("Range") != "" {
					return http.StatusRequestedRangeNotSatisfiable
				}

				return 0
			}
		}},
		{"206 with another start", func(s *srv) { s.wrongStart = func(int) bool { return true } }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 250_000)
			tc.setup(r.srv)
			require.NoError(t, os.WriteFile(filepath.Join(r.dir, "l_3503.mp3.part"), r.body[:1000], 0o600))
			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			r.ev.wait(t, download.EventFinished, "l_3503")
			require.Equal(t, []string{"bytes=1000-", ""}, r.srv.requests())
			path, _ := r.mgr.Path("l_3503")
			require.Equal(t, r.body, readAll(t, path))
		})
	}
}

func TestChecksumMismatchDeletes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		mod  func(a *model.Audio)
	}{
		{"wrong sha256", func(a *model.Audio) { a.SHA256 = "00" + a.SHA256[2:] }},
		{"wrong size", func(a *model.Audio) { a.SizeBytes -= 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 100_000)
			base := r.audio
			r.audio = func(n int, id string) model.Audio { a := base(n, id); tc.mod(&a); return a }
			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			ev := r.ev.wait(t, download.EventFailed, "l_3503")
			require.ErrorIs(t, ev.Err, download.ErrChecksum)
			require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3"))
			require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3.part"))
			_, ok := r.mgr.Path("l_3503")
			require.False(t, ok)
			require.Empty(t, r.mgr.List())
			jobs := r.mgr.Jobs()
			require.Len(t, jobs, 1)
			require.Equal(t, download.StateFailed, jobs[0].State)
			require.ErrorIs(t, jobs[0].Err, download.ErrChecksum)
		})
	}
}

func TestFailedJobRetriesWithStart(t *testing.T) {
	t.Parallel()
	r := newRig(t, 100_000)
	good := r.audio
	r.audio = func(n int, id string) model.Audio {
		a := good(n, id)
		if n == 1 {
			a.SHA256 = "00" + a.SHA256[2:]
		}

		return a
	}
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFailed, "l_3503")
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	_, ok := r.mgr.Path("l_3503")
	require.True(t, ok)
}

func TestCancelRemovesPart(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 1)
	r := newRig(t, 250_000)
	// The server sends 100 000 bytes and then waits for the client to go.
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	srvDone := newBlockingServer(t, r.body, 100_000, hold, started)
	base := r.audio
	r.audio = func(n int, id string) model.Audio { a := base(n, id); a.URL = srvDone.URL + "/x.mp3"; return a }

	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	<-started
	r.mgr.Cancel("l_3503")
	require.Empty(t, r.mgr.Jobs())
	require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3.part"))
	require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3"))
	for _, e := range r.ev.snapshot() {
		require.NotEqual(t, download.EventFailed, e.Kind)
		require.NotEqual(t, download.EventFinished, e.Kind)
	}
	r.mgr.Cancel("unknown") // no-op
}

func TestMaxTwoConcurrent(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	arrived := make(chan int, 8)
	release := make(chan struct{}, 8)
	r.srv.gate = func(n int, _ *http.Request) {
		arrived <- n
		<-release
	}
	for _, id := range []string{"l_3501", "l_3502", "l_3503"} {
		require.NoError(t, r.mgr.Start(lesson(id), course("set-35")))
	}
	<-arrived
	<-arrived
	states := func() map[download.State]int {
		m := map[download.State]int{}
		for _, j := range r.mgr.Jobs() {
			m[j.State]++
		}

		return m
	}
	require.Equal(t, map[download.State]int{download.StateRunning: 2, download.StateQueued: 1}, states())
	require.Equal(t, "l_3503", r.mgr.Jobs()[2].LessonID, "FIFO: the third waits")

	release <- struct{}{} // one finishes
	<-arrived             // the third starts
	require.Equal(t, int32(2), r.srv.maxActive.Load())
	release <- struct{}{}
	release <- struct{}{}
	for range 3 {
		r.ev.wait(t, download.EventFinished, "")
	}
	require.Equal(t, int32(2), r.srv.maxActive.Load(), "never more than two at once")
	require.Len(t, r.mgr.List(), 3)
}

func TestMaxConcurrentFromOptions(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000, func(o *download.Options) { o.MaxConcurrent = 1 })
	arrived := make(chan int, 8)
	release := make(chan struct{}, 8)
	r.srv.gate = func(n int, _ *http.Request) { arrived <- n; <-release }
	require.NoError(t, r.mgr.Start(lesson("l_3501"), course("c")))
	require.NoError(t, r.mgr.Start(lesson("l_3502"), course("c")))
	<-arrived
	require.Equal(t, download.StateQueued, r.mgr.Jobs()[1].State)
	release <- struct{}{}
	<-arrived
	release <- struct{}{}
	r.ev.wait(t, download.EventFinished, "l_3502")
	require.Equal(t, int32(1), r.srv.maxActive.Load())
}

func TestWiFiOnlyBlocks(t *testing.T) {
	t.Parallel()
	t.Run("start refused", func(t *testing.T) {
		t.Parallel()
		wifi := false
		r := newRig(t, 50_000, func(o *download.Options) {
			o.Allowed = func() error {
				if wifi {
					return nil
				}

				return download.ErrWiFiOnly
			}
		})
		err := r.mgr.Start(lesson("l_3503"), course("set-35"))
		require.ErrorIs(t, err, download.ErrWiFiOnly)
		jobs := r.mgr.Jobs()
		require.Len(t, jobs, 1, "kept, waiting for Wi-Fi (Q-P5-2)")
		require.Equal(t, download.StatePaused, jobs[0].State)
		require.ErrorIs(t, jobs[0].Err, download.ErrWiFiOnly)
		require.ErrorIs(t, r.ev.wait(t, download.EventPaused, "l_3503").Err, download.ErrWiFiOnly)
		require.Empty(t, r.srv.requests(), "no request without Wi-Fi")
		wifi = true
		r.mgr.ResumeAll()
		r.ev.wait(t, download.EventFinished, "l_3503")
	})
	t.Run("lost while waiting", func(t *testing.T) {
		t.Parallel()
		var allowed atomicBool
		allowed.set(true)
		r := newRig(t, 50_000, func(o *download.Options) {
			o.MaxConcurrent = 1
			o.Allowed = func() error {
				if allowed.get() {
					return nil
				}

				return download.ErrWiFiOnly
			}
		})
		arrived := make(chan int, 4)
		release := make(chan struct{}, 4)
		r.srv.gate = func(n int, _ *http.Request) { arrived <- n; <-release }
		require.NoError(t, r.mgr.Start(lesson("l_3501"), course("c")))
		require.NoError(t, r.mgr.Start(lesson("l_3502"), course("c")))
		<-arrived
		allowed.set(false) // Wi-Fi gone while the second waits
		release <- struct{}{}
		ev := r.ev.wait(t, download.EventPaused, "l_3502")
		require.ErrorIs(t, ev.Err, download.ErrWiFiOnly)
		require.Equal(t, 1, len(r.srv.requests()), "the second never hit the network")
		allowed.set(true)
		r.mgr.ResumeAll()
		<-arrived
		release <- struct{}{}
		r.ev.wait(t, download.EventFinished, "l_3502")
	})
}

func TestNoSpace(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		free   func() (int64, error)
		fails  bool
		hitSrv bool
	}{
		{"less than needed", func() (int64, error) { return 49_999, nil }, true, false},
		{"exactly enough", func() (int64, error) { return 50_000, nil }, false, true},
		{"unknown free space does not block", func() (int64, error) { return 0, errors.New("unavailable") }, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 50_000, func(o *download.Options) { o.FreeSpace = tc.free })
			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			if tc.fails {
				ev := r.ev.wait(t, download.EventFailed, "l_3503")
				require.ErrorIs(t, ev.Err, download.ErrNoSpace)
				require.Empty(t, r.srv.requests())
				require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3.part"))

				return
			}
			r.ev.wait(t, download.EventFinished, "l_3503")
		})
	}
}

func TestProgressThrottled(t *testing.T) {
	t.Parallel()
	r := newRig(t, 100_000)
	want, grant := make(chan struct{}), make(chan struct{})
	base := r.audio
	r.audio = func(n int, id string) model.Audio { return base(n, id) }
	r.mgr = r.open(t, t.TempDir(), "u1", func(o *download.Options) {
		o.HTTP = &http.Client{Transport: &slowTransport{want: want, grant: grant}}
	})
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	for range 11 { // ten reads of 10 000 bytes and the read that sees EOF
		<-want // the previous chunk is fully handled
		r.fake.Advance(50 * time.Millisecond)
		grant <- struct{}{}
	}
	r.ev.wait(t, download.EventFinished, "l_3503")

	var dones []int64
	var at []download.Event
	for _, e := range r.ev.snapshot() {
		if e.Kind == download.EventProgress {
			dones = append(dones, e.Done)
			at = append(at, e)
		}
	}
	// 50 ms per chunk: events at 0 (start), 250 ms and 500 ms; the closing
	// one equals the last and is not repeated.
	require.Equal(t, []int64{0, 50_000, 100_000}, dones)
	require.Equal(t, int64(100_000), at[len(at)-1].Total)
}

func TestExpiredURLRefetched(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		wait      time.Duration
		wantCalls int
	}{
		{"expired while paused", 2 * time.Hour, 2},
		{"within the last 30 s counts as expired", time.Hour - 10*time.Second, 2},
		{"still fresh", 10 * time.Minute, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 250_000)
			r.srv.cutAt = func(n int) int {
				if n == 1 {
					return 100_000
				}

				return 0
			}
			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			r.ev.wait(t, download.EventPaused, "l_3503")
			r.fake.Advance(tc.wait)
			r.mgr.ResumeAll()
			r.ev.wait(t, download.EventFinished, "l_3503")
			require.Equal(t, tc.wantCalls, r.src.count())
		})
	}
}

func TestRefusedURLRefetched(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	r.srv.status = func(n int, _ *http.Request) int {
		if n == 1 {
			return http.StatusForbidden
		}

		return 0
	}
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.Equal(t, 2, r.src.count(), "403 on the file: ask for a new URL")
}

func TestMediaErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		is     error
	}{
		{"500", http.StatusInternalServerError, api.ErrServer},
		{"404", http.StatusNotFound, nil},
		{"400", http.StatusBadRequest, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 50_000)
			r.srv.status = func(int, *http.Request) int { return tc.status }
			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			ev := r.ev.wait(t, download.EventFailed, "l_3503")
			require.Error(t, ev.Err)
			if tc.is != nil {
				require.ErrorIs(t, ev.Err, tc.is)
			}
			require.NotErrorIs(t, ev.Err, api.ErrOffline)
		})
	}
}

func TestAudioErrorPausesWhenOffline(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	r.src.fn = func(n int, id string) (model.Audio, error) {
		if n == 1 {
			return model.Audio{}, fmt.Errorf("%w: down", api.ErrOffline)
		}

		return r.audio(n, id), nil
	}
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventPaused, "l_3503")
	r.mgr.ResumeAll()
	r.ev.wait(t, download.EventFinished, "l_3503")
}

func TestIndexSurvivesReopen(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	r.mgr.Close()

	m2 := r.open(t, r.dir, "u1")
	list := m2.List()
	require.Len(t, list, 1)
	require.Equal(t, download.Entry{LessonID: "l_3503", CourseID: "set-35", UserID: "u1", Size: 50_000, SHA256: r.sum, DownloadedAt: t0}, list[0])
	p, ok := m2.Path("l_3503")
	require.True(t, ok)
	require.Equal(t, filepath.Join(r.dir, "l_3503.mp3"), p)

	require.NoError(t, m2.Start(lesson("l_3503"), course("set-35")), "already downloaded: nothing to do")
	require.Empty(t, m2.Jobs())
	require.Len(t, r.srv.requests(), 1)
}

func TestListFiltersByUser(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")

	other := r.open(t, r.dir, "u2")
	require.Empty(t, other.List(), "files of the first user stay, hidden from the second")
	_, ok := other.Path("l_3503")
	require.False(t, ok)
	require.FileExists(t, filepath.Join(r.dir, "l_3503.mp3"), "logout keeps files")
	require.Len(t, r.mgr.List(), 1)
	_, ok = r.mgr.Path("l_3503")
	require.True(t, ok)

	// The same user signing in again sees them.
	again := r.open(t, r.dir, "u1")
	require.Len(t, again.List(), 1)
}

func TestDeleteRemovesFileAndEntry(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.NoError(t, r.mgr.Delete("l_3503"))
	require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3"))
	require.Empty(t, r.mgr.List())
	require.NoError(t, r.mgr.Delete("l_3503"), "twice is fine")
	require.Error(t, r.mgr.Delete("../x"))
}

func TestStartRejects(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	tests := []struct {
		name string
		l    model.Lesson
	}{
		{"path in id", model.Lesson{ID: "../etc", HasAudio: true}},
		{"empty id", model.Lesson{HasAudio: true}},
		{"no audio", model.Lesson{ID: "l_3503"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Error(t, r.mgr.Start(tc.l, course("c")))
		})
	}
	require.Empty(t, r.mgr.Jobs())
}

func TestCloseKeepsPartAndSendsNoEvent(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	started := make(chan struct{}, 1)
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	blocking := newBlockingServer(t, r.body, 100_000, hold, started)
	base := r.audio
	r.audio = func(n int, id string) model.Audio { a := base(n, id); a.URL = blocking.URL + "/x.mp3"; return a }
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	<-started
	r.mgr.Close()
	require.Error(t, r.mgr.Start(lesson("l_3504"), course("set-35")))
	for _, e := range r.ev.snapshot() {
		require.Equal(t, download.EventProgress, e.Kind)
	}
}

func TestNewManagerValidates(t *testing.T) {
	t.Parallel()
	_, err := download.NewManager(download.Options{})
	require.Error(t, err)
	_, err = download.NewManager(download.Options{Client: &source{}, Dir: t.TempDir()})
	require.Error(t, err, "clock is required")
	m, err := download.NewManager(download.Options{Client: &source{}, Dir: t.TempDir(), Clock: clock.NewFake(t0)})
	require.NoError(t, err)
	m.Close()
}

func TestCorruptIndexIsEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.json"), []byte("{oops"), 0o600))
	m, err := download.NewManager(download.Options{Client: &source{}, Dir: dir, Clock: clock.NewFake(t0)})
	require.NoError(t, err)
	defer m.Close()
	require.Empty(t, m.List())
}

// TestAudioNotFoundMarksTheLessonUnavailable checks a lesson whose audio the server
// does not have (GET /lessons/{id}/audio answers 404 not_found): the job fails with
// ErrNoAudio, the lesson is unavailable for the run, Start refuses it without asking
// the server again, and the other lessons are not touched.
func TestAudioNotFoundMarksTheLessonUnavailable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		fail func(r *rig)
	}{
		{"audio answer", func(r *rig) {
			r.src.fn = func(n int, id string) (model.Audio, error) {
				if id == "l_3501" {
					return model.Audio{}, fmt.Errorf("api: http 404 not_found: %w", core.ErrNotFound)
				}

				return r.audio(n, id), nil
			}
		}},
		{"file", func(r *rig) {
			r.srv.status = func(_ int, req *http.Request) int {
				if strings.Contains(req.URL.Path, "l_3501") {
					return http.StatusNotFound
				}

				return 0
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 50_000)
			tc.fail(r)
			require.False(t, r.mgr.Unavailable("l_3501"))
			require.NoError(t, r.mgr.Start(lesson("l_3501"), course("set-35")))
			ev := r.ev.wait(t, download.EventFailed, "l_3501")
			require.ErrorIs(t, ev.Err, download.ErrNoAudio)
			require.ErrorIs(t, ev.Err, core.ErrNotFound)
			require.True(t, r.mgr.Unavailable("l_3501"))
			require.False(t, r.mgr.Unavailable("l_3503"))

			calls := r.src.count()
			require.ErrorIs(t, r.mgr.Start(lesson("l_3501"), course("set-35")), download.ErrNoAudio)
			require.Equal(t, calls, r.src.count(), "the server is not asked again")

			require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
			r.ev.wait(t, download.EventFinished, "l_3503")
		})
	}
}
