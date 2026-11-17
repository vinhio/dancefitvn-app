package download_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
)

// metaLesson is a lesson with everything screens 11 and 13 show.
func metaLesson(id string, no int) model.Lesson {
	return model.Lesson{ID: id, No: no, Title: "Bài " + id, Tag: "Khởi động", DurationSec: 600 + no, HasAudio: true, AudioSizeBytes: 9_000_000}
}

var setInfo = download.CourseInfo{ID: "set-35", Name: "Nền tảng", Lessons: 12}

// cutOnce makes the first request drop after 100 000 bytes.
func cutOnce(s *srv) {
	s.cutAt = func(n int) int {
		if n == 1 {
			return 100_000
		}

		return 0
	}
}

// holdFirst is a media server whose first request sends first bytes and
// then waits for the client to go; the next ones are served in full with
// Range. It records the Range headers.
type holdFirst struct {
	*httptest.Server
	mu     sync.Mutex
	ranges []string
}

func newHoldFirst(t *testing.T, body []byte, first int) *holdFirst {
	t.Helper()
	h := &holdFirst{}
	h.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.ranges = append(h.ranges, r.Header.Get("Range"))
		n := len(h.ranges)
		h.mu.Unlock()
		if n == 1 {
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body[:first])
			w.(http.Flusher).Flush()
			<-r.Context().Done()

			return
		}
		http.ServeContent(w, r, "x.mp3", t0, strings.NewReader(string(body)))
	}))
	t.Cleanup(h.Close)

	return h
}

func (h *holdFirst) got() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	return append([]string(nil), h.ranges...)
}

// eventually polls cond without sleeping long; it fails after 5 s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout: %s", what)
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
}

// finished waits until every lesson has an EventFinished, in any order.
func finished(t *testing.T, e *events, ids ...string) {
	t.Helper()
	eventually(t, "finished events", func() bool {
		seen := map[string]bool{}
		for _, ev := range e.snapshot() {
			if ev.Kind == download.EventFinished {
				seen[ev.LessonID] = true
			}
		}

		return len(seen) >= len(ids) && func() bool {
			for _, id := range ids {
				if !seen[id] {
					return false
				}
			}

			return true
		}()
	})
}

func partSize(dir, id string) int64 {
	st, err := os.Stat(filepath.Join(dir, id+".mp3.part"))
	if err != nil {
		return -1
	}

	return st.Size()
}

func TestMetadataPersistedAndListedByUser(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	require.NoError(t, r.mgr.Start(metaLesson("l_3501", 1), setInfo))
	require.NoError(t, r.mgr.Start(metaLesson("l_3502", 2), setInfo))
	finished(t, r.ev, "l_3501", "l_3502")
	r.mgr.Close()

	want := download.Meta{No: 1, Title: "Bài l_3501", Tag: "Khởi động", DurationSec: 601, CatalogBytes: 9_000_000, CourseName: "Nền tảng", CourseLessons: 12}
	tests := []struct {
		name string
		user string
		want int
	}{
		{"owner sees both", "u1", 2},
		{"other user sees none", "u2", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := r.open(t, r.dir, tc.user)
			list := m.List()
			require.Len(t, list, tc.want)
			if tc.want == 0 {
				_, ok := m.Lookup("l_3501")
				require.False(t, ok)

				return
			}
			e1, ok := m.Lookup("l_3501")
			require.True(t, ok)
			require.Equal(t, want, e1.Meta)
			require.Equal(t, "set-35", e1.CourseID)
			e, ok := m.Lookup("l_3502")
			require.True(t, ok)
			require.Equal(t, "Bài l_3502", e.Title)
			require.Equal(t, 2, e.No)
			require.Equal(t, int64(50_000), e.Size)
		})
	}
}

func TestOldIndexWithoutMetadataStillReads(t *testing.T) {
	t.Parallel()
	r := newRig(t, 1000)
	dir := t.TempDir()
	old := `{"items":[{"lesson_id":"l_3503","course_id":"set-35","user_id":"u1","size":1000,"sha256":"ab","downloaded_at":"2026-11-14T09:00:00Z"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.json"), []byte(old), 0o600))
	m := r.open(t, dir, "u1")
	list := m.List()
	require.Len(t, list, 1)
	require.Equal(t, download.Meta{}, list[0].Meta)
	require.Equal(t, "l_3503", list[0].LessonID)
	require.Empty(t, m.Jobs())
}

func TestPendingSurvivesRestart(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		resume func(m *download.Manager) error
	}{
		{"ResumeAll", func(m *download.Manager) error { m.ResumeAll(); return nil }},
		{"Start again", func(m *download.Manager) error { return m.Start(metaLesson("l_3503", 3), setInfo) }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 250_000)
			cutOnce(r.srv)
			require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
			r.ev.wait(t, download.EventPaused, "l_3503")
			have := partSize(r.dir, "l_3503")
			require.Positive(t, have)
			r.mgr.Close()

			m2 := r.open(t, r.dir, "u1")
			jobs := m2.Jobs()
			require.Len(t, jobs, 1)
			j := jobs[0]
			require.Equal(t, download.StatePaused, j.State)
			require.Equal(t, have, j.Done)
			require.Equal(t, "set-35", j.CourseID)
			require.Equal(t, "Bài l_3503", j.Title)
			require.Equal(t, "Nền tảng", j.CourseName)
			require.Equal(t, 12, j.CourseLessons)
			require.Equal(t, int64(9_000_000), j.Total, "catalog size until the real one is known")
			require.Len(t, r.srv.requests(), 1, "nothing starts by itself")

			require.NoError(t, tc.resume(m2))
			r.ev.wait(t, download.EventFinished, "l_3503")
			require.Equal(t, []string{"", "bytes=" + strconv.FormatInt(have, 10) + "-"}, r.srv.requests())
			p, ok := m2.Path("l_3503")
			require.True(t, ok)
			require.Equal(t, r.body, readAll(t, p))
			e, _ := m2.Lookup("l_3503")
			require.Equal(t, "Nền tảng", e.CourseName)
			require.Empty(t, m2.Jobs())

			m2.Close()
			m3 := r.open(t, r.dir, "u1")
			require.Empty(t, m3.Jobs(), "finished jobs are not pending any more")
			require.Len(t, m3.List(), 1)
		})
	}
}

func TestPendingBelongsToItsUser(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	cutOnce(r.srv)
	require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
	r.ev.wait(t, download.EventPaused, "l_3503")
	r.mgr.PauseAll(nil) // sign-out: pause and keep (Q-P5-8)
	r.mgr.Close()

	other := r.open(t, r.dir, "u2")
	require.Empty(t, other.Jobs())
	other.Close()

	again := r.open(t, r.dir, "u1")
	require.Len(t, again.Jobs(), 1)
	again.ResumeAll()
	r.ev.wait(t, download.EventFinished, "l_3503")
}

func TestPauseAll(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		reason error
	}{
		{"wifi lost", download.ErrWiFiOnly},
		{"sign-out", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, 250_000, func(o *download.Options) { o.MaxConcurrent = 1 })
			hf := newHoldFirst(t, r.body, 100_000)
			base := r.audio
			r.audio = func(n int, id string) model.Audio { a := base(n, id); a.URL = hf.URL + "/x.mp3"; return a }
			require.NoError(t, r.mgr.Start(metaLesson("l_3501", 1), setInfo))
			require.NoError(t, r.mgr.Start(metaLesson("l_3502", 2), setInfo)) // queued
			eventually(t, "part written", func() bool { return partSize(r.dir, "l_3501") == 100_000 })

			r.mgr.PauseAll(tc.reason)
			for _, id := range []string{"l_3501", "l_3502"} {
				ev := r.ev.wait(t, download.EventPaused, id)
				require.Equal(t, tc.reason, ev.Err)
			}
			for _, j := range r.mgr.Jobs() {
				require.Equal(t, download.StatePaused, j.State)
				require.Equal(t, tc.reason, j.Err)
			}
			require.Equal(t, int64(100_000), partSize(r.dir, "l_3501"), "parts are kept")

			r.mgr.ResumeAll() // may come before the cancelled transfer has returned
			finished(t, r.ev, "l_3501", "l_3502")
			got := hf.got()
			require.Len(t, got, 3)
			require.ElementsMatch(t, []string{"", "bytes=100000-", ""}, got)
			p, _ := r.mgr.Path("l_3501")
			require.Equal(t, r.body, readAll(t, p))
		})
	}
}

func TestForgetDoesNotWaitForTheTransfer(t *testing.T) {
	t.Parallel()
	slow := &slowTransport{want: make(chan struct{}), grant: make(chan struct{})}
	r := newRig(t, 100_000, func(o *download.Options) { o.HTTP = &http.Client{Transport: slow} })
	require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
	<-slow.want // the transfer is inside a read that the test controls

	returned := make(chan struct{})
	go func() { r.mgr.Forget("l_3503"); close(returned) }()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Forget waited for the transfer")
	}
	require.Empty(t, r.mgr.Jobs())

	slow.grant <- struct{}{} // the read returns; the transfer sees the cancel and ends
	eventually(t, "part removed", func() bool { return partSize(r.dir, "l_3503") < 0 })
	require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3"))
	for _, e := range r.ev.snapshot() {
		require.NotEqual(t, download.EventFailed, e.Kind)
		require.NotEqual(t, download.EventFinished, e.Kind)
	}
	r.mgr.Forget("unknown") // no-op
	r.mgr.Close()
	require.Empty(t, r.open(t, r.dir, "u1").Jobs(), "forgotten jobs are not pending")
}

func TestForgetThenStartAgain(t *testing.T) {
	t.Parallel()
	r := newRig(t, 250_000)
	hf := newHoldFirst(t, r.body, 100_000)
	base := r.audio
	r.audio = func(n int, id string) model.Audio { a := base(n, id); a.URL = hf.URL + "/x.mp3"; return a }
	require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
	eventually(t, "part written", func() bool { return partSize(r.dir, "l_3503") == 100_000 })
	r.mgr.Forget("l_3503")
	require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
	r.ev.wait(t, download.EventFinished, "l_3503")
	require.Equal(t, []string{"", ""}, hf.got(), "the second run starts from zero: the old part was removed first")
	p, _ := r.mgr.Path("l_3503")
	require.Equal(t, r.body, readAll(t, p))
}

func TestFailedAndCancelledAreNotPending(t *testing.T) {
	t.Parallel()
	t.Run("failed", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, 50_000)
		base := r.audio
		r.audio = func(n int, id string) model.Audio { a := base(n, id); a.SHA256 = strings.Repeat("0", 64); return a }
		require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
		require.ErrorIs(t, r.ev.wait(t, download.EventFailed, "l_3503").Err, download.ErrChecksum)
		r.mgr.Close()
		require.Empty(t, r.open(t, r.dir, "u1").Jobs())
	})
	t.Run("cancelled", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, 250_000)
		cutOnce(r.srv)
		require.NoError(t, r.mgr.Start(metaLesson("l_3503", 3), setInfo))
		r.ev.wait(t, download.EventPaused, "l_3503")
		r.mgr.Cancel("l_3503")
		r.mgr.Close()
		require.Empty(t, r.open(t, r.dir, "u1").Jobs())
		require.NoFileExists(t, filepath.Join(r.dir, "l_3503.mp3.part"))
	})
}

// lines collects Logf output.
type lines struct {
	mu  sync.Mutex
	all []string
}

func (l *lines) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.all = append(l.all, sprintf(format, args...))
}

func (l *lines) joined() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return strings.Join(l.all, "\n")
}

func TestLogLines(t *testing.T) {
	t.Parallel()
	t.Run("range resume", func(t *testing.T) {
		t.Parallel()
		var l lines
		r := newRig(t, 250_000, func(o *download.Options) { o.Logf = l.logf })
		cutOnce(r.srv)
		require.NoError(t, r.mgr.Start(lesson("l_3503"), course("c")))
		r.ev.wait(t, download.EventPaused, "l_3503")
		have := partSize(r.dir, "l_3503")
		r.mgr.ResumeAll()
		r.ev.wait(t, download.EventFinished, "l_3503")
		require.Contains(t, l.joined(), "download: l_3503 resume at "+strconv.FormatInt(have, 10)+" (206)")
	})
	t.Run("restart", func(t *testing.T) {
		t.Parallel()
		var l lines
		r := newRig(t, 250_000, func(o *download.Options) { o.Logf = l.logf })
		r.srv.ignoreRange = func(int) bool { return true }
		require.NoError(t, os.WriteFile(filepath.Join(r.dir, "l_3503.mp3.part"), r.body[:1000], 0o600))
		require.NoError(t, r.mgr.Start(lesson("l_3503"), course("c")))
		r.ev.wait(t, download.EventFinished, "l_3503")
		require.Contains(t, l.joined(), "download: l_3503 restart (200)")
	})
	t.Run("failed", func(t *testing.T) {
		t.Parallel()
		var l lines
		r := newRig(t, 50_000, func(o *download.Options) { o.Logf = l.logf })
		r.srv.status = func(int, *http.Request) int { return http.StatusNotFound }
		require.NoError(t, r.mgr.Start(lesson("l_3503"), course("c")))
		r.ev.wait(t, download.EventFailed, "l_3503")
		eventually(t, "log", func() bool { return strings.Contains(l.joined(), "download: l_3503 failed: ") })
	})
	t.Run("no signed url in logs or errors", func(t *testing.T) {
		t.Parallel()
		var l lines
		r := newRig(t, 50_000, func(o *download.Options) { o.Logf = l.logf })
		base := r.audio
		r.audio = func(n int, id string) model.Audio {
			a := base(n, id)
			a.URL += "?sig=SECRET&exp=1"

			return a
		}
		r.srv.Close() // connection refused: a url.Error with the signed URL
		require.NoError(t, r.mgr.Start(lesson("l_3503"), course("c")))
		ev := r.ev.wait(t, download.EventPaused, "l_3503")
		require.ErrorIs(t, ev.Err, api.ErrOffline)
		require.NotContains(t, ev.Err.Error(), "SECRET")
		require.NotContains(t, ev.Err.Error(), "sig=")
		require.NotContains(t, l.joined(), "sig=")
	})
}

func TestFinishedEventPerJob(t *testing.T) {
	t.Parallel()
	r := newRig(t, 50_000)
	ids := []string{"l_3501", "l_3502", "l_3503", "l_3504"}
	for i, id := range ids {
		require.NoError(t, r.mgr.Start(metaLesson(id, i+1), setInfo))
	}
	finished(t, r.ev, ids...)
	r.mgr.Close()
	count := map[string]int{}
	for _, e := range r.ev.snapshot() {
		if e.Kind == download.EventFinished {
			count[e.LessonID]++
		}
	}
	require.Len(t, count, len(ids))
	for id, n := range count {
		require.Equal(t, 1, n, id)
	}
}

func TestWiFiWaitQueueAutoStarts(t *testing.T) {
	t.Parallel()
	var wifi atomicBool
	r := newRig(t, 50_000, func(o *download.Options) {
		o.Allowed = func() error {
			if wifi.get() {
				return nil
			}

			return download.ErrWiFiOnly
		}
	})
	for i, id := range []string{"l_3501", "l_3502"} {
		require.ErrorIs(t, r.mgr.Start(metaLesson(id, i+1), setInfo), download.ErrWiFiOnly)
	}
	for _, j := range r.mgr.Jobs() {
		require.Equal(t, download.StatePaused, j.State)
		require.ErrorIs(t, j.Err, download.ErrWiFiOnly)
	}
	r.mgr.Close()

	// The wait survives a restart; Wi-Fi comes back, ResumeAll starts both.
	m2 := r.open(t, r.dir, "u1", func(o *download.Options) {
		o.Allowed = func() error {
			if wifi.get() {
				return nil
			}

			return download.ErrWiFiOnly
		}
	})
	require.Len(t, m2.Jobs(), 2)
	m2.ResumeAll()
	for _, j := range m2.Jobs() {
		require.ErrorIs(t, j.Err, download.ErrWiFiOnly, "still on mobile data")
	}
	wifi.set(true)
	m2.ResumeAll()
	finished(t, r.ev, "l_3501", "l_3502")
}
