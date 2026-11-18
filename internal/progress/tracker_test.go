package progress_test

import (
	"context"
	"dancefitvn-app/internal/safe"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
)

var t0 = time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC)

// rig is a Tracker over fakes.
type rig struct {
	fake  *clock.Fake
	api   *fakeAPI
	queue *progress.Queue
	ui    *ui
	tr    *progress.Tracker
	mem   *store.Memory
	saved []model.ProgressResult
	won   []string
}

func newRig(t *testing.T) *rig {
	t.Helper()
	r := &rig{fake: clock.NewFake(t0), api: &fakeAPI{}, ui: newUI(), mem: store.NewMemory()}
	q, err := progress.NewQueue(r.mem, r.api)
	require.NoError(t, err)
	r.queue = q
	r.tr = progress.NewTracker(progress.Options{
		Sender: r.api, Queue: q, Clock: r.fake, Dispatch: r.ui.dispatch, DeviceID: "d_102",
		OnSaved:   func(res model.ProgressResult) { r.saved = append(r.saved, res) },
		OnWatched: func(id string) { r.won = append(r.won, id) },
	})
	t.Cleanup(r.tr.Close)

	return r
}

func TestSavesEvery10s(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		ticks   int
		playing bool
		want    []int // PositionSec of each save
	}{
		{"playing 25 s saves at 10 s and 20 s", 100, true, []int{10, 20}},
		{"not before 10 s", 39, true, nil},
		{"paused ticks never save", 100, false, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t)
			pos := 0.0
			for range tc.ticks {
				r.tr.Update("l_3503", "set-35", pos, 220, tc.playing)
				if tc.playing && r.fake.Now().Sub(t0)%(10*time.Second) == 0 && r.fake.Now().After(t0) {
					r.ui.drain(1) // the save due at this tick finished
				}
				r.fake.Advance(250 * time.Millisecond)
				if tc.playing {
					pos += 0.25
				}
			}
			var got []int
			r.api.mu.Lock()
			for _, s := range r.api.saves {
				got = append(got, s.req.PositionSec)
			}
			r.api.mu.Unlock()
			require.Equal(t, tc.want, got)
			if len(tc.want) > 0 {
				require.Equal(t, "d_102", r.api.saves[0].req.DeviceID)
				require.Equal(t, "l_3503", r.api.saves[0].lessonID)
				require.Equal(t, 220, r.api.saves[0].req.DurationSec)
				require.Equal(t, t0.Add(10*time.Second), r.api.saves[0].req.ClientTime)
			}
			require.Len(t, r.saved, len(tc.want))
		})
	}
}

func TestSaveOnPauseAndLeave(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   func(r *rig)
		want []int
	}{
		{"pause", func(r *rig) { r.tr.Pause() }, []int{12}},
		{"leave", func(r *rig) { r.tr.Leave() }, []int{12}},
		{"pause then leave at same spot saves once", func(r *rig) { r.tr.Pause(); r.ui.drain(1); r.tr.Leave() }, []int{12}},
		{"pause, move, leave saves twice", func(r *rig) {
			r.tr.Pause()
			r.ui.drain(1)
			r.tr.Update("l_3503", "set-35", 15.4, 220, false)
			r.tr.Leave()
		}, []int{12, 15}},
		{"another lesson leaves the first", func(r *rig) { r.tr.Update("l_3504", "set-35", 1, 200, true) }, []int{12}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t)
			r.tr.Update("l_3503", "set-35", 12.6, 220, true)
			tc.do(r)
			r.tr.Close()
			var got []int
			for _, s := range r.api.saves {
				got = append(got, s.req.PositionSec)
			}
			require.Equal(t, tc.want, got)
		})
	}
}

func TestSaveSkipsUnknownDuration(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 0, 0, true)
	r.tr.Pause()
	r.tr.Leave()
	require.Zero(t, r.api.saveCount())
}

func TestWatchedThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		pos, dur float64
		want     bool
	}{
		{"197 of 220", 197, 220, false},
		{"198 of 220", 198, 220, true},
		{"exact", 90, 100, true},
		{"just below", 89.99, 100, false},
		{"duration unknown", 50, 0, false},
		{"start", 0, 0, false},
		{"past the end", 230, 220, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, progress.Watched(tc.pos, tc.dur, 0.9))
			r := newRig(t)
			r.tr.Update("l_3503", "set-35", tc.pos, tc.dur, false)
			r.tr.Update("l_3503", "set-35", tc.pos, tc.dur, false)
			if tc.want {
				require.Equal(t, []string{"l_3503"}, r.won, "once per visit")
			} else {
				require.Empty(t, r.won)
			}
		})
	}
}

func TestOfflineSaveQueues(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	r.api.saveFn = func(int) (model.ProgressResult, error) {
		return model.ProgressResult{}, fmt.Errorf("%w: %w", api.ErrOffline, offlineErr)
	}
	r.tr.Update("l_3503", "set-35", 42, 220, true)
	r.tr.Pause()
	r.ui.drain(1)
	require.Equal(t, 1, r.queue.Len())
	it := r.queue.Items()[0]
	require.Equal(t, progress.Item{LessonID: "l_3503", PositionSec: 42, DurationSec: 220, ClientTime: t0}, it)
	require.Empty(t, r.saved, "no OnSaved on failure")
	require.NotZero(t, r.mem.Saves, "queue persisted")
}

func TestSuccessFlushesQueue(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	require.NoError(t, r.queue.Add(progress.Item{LessonID: "l_3501", PositionSec: 5, DurationSec: 100, ClientTime: t0}))
	r.tr.Update("l_3503", "set-35", 42, 220, true)
	r.tr.Pause()
	r.ui.drain(1)
	require.Equal(t, []int{1}, r.api.batchSizes())
	require.Zero(t, r.queue.Len())
}

func TestSavesAreSerialAndLatestWins(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	r.api.saveFn = func(n int) (model.ProgressResult, error) {
		entered <- struct{}{}
		if n == 1 {
			<-release
		}

		return model.ProgressResult{}, nil
	}
	r.tr.Update("l_3503", "set-35", 10, 220, true)
	r.tr.Pause() // save 1 in flight
	<-entered
	r.tr.Update("l_3503", "set-35", 20, 220, false)
	r.tr.Pause() // waits
	r.tr.Update("l_3503", "set-35", 30, 220, false)
	r.tr.Pause() // replaces the waiting one
	require.Equal(t, 1, r.api.saveCount())
	close(release)
	r.ui.drain(1) // first done, second launched
	<-entered
	r.ui.drain(1)
	require.Equal(t, 2, r.api.saveCount())
	require.Equal(t, 30, r.api.saves[1].req.PositionSec)
}

func TestSaveQueuesOnServerError(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		err    error
		queued int
	}{
		"5xx":        {fmt.Errorf("%w: boom", api.ErrServer), 1},
		"signed out": {fmt.Errorf("%w", api.ErrSignedOut), 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t)
			r.api.saveFn = func(int) (model.ProgressResult, error) { return model.ProgressResult{}, tc.err }
			r.tr.Update("l_3503", "set-35", 42, 220, true)
			r.tr.Pause()
			r.ui.drain(1)
			require.Equal(t, tc.queued, r.queue.Len())
		})
	}
}

// TestPendingIsPerLesson: A@100 in flight, A moves on, then B and C start;
// the waiting A save must survive B's.
func TestPendingIsPerLesson(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 8)
	r.api.saveFn = func(n int) (model.ProgressResult, error) {
		entered <- struct{}{}
		if n == 1 {
			<-release
		}

		return model.ProgressResult{}, nil
	}
	r.tr.Update("l_A", "c", 100, 220, true)
	r.tr.Pause() // A@100 in flight
	<-entered
	r.tr.Update("l_A", "c", 200, 220, true)
	r.tr.Update("l_B", "c", 0, 300, true) // Leave(A) holds A@200
	r.tr.Update("l_C", "c", 0, 300, true) // Leave(B) holds B@0
	r.tr.Update("l_C", "c", 5, 300, true)
	r.tr.Leave() // holds C@5
	require.Equal(t, 1, r.api.saveCount())
	close(release)
	for range 3 {
		r.ui.drain(1)
		<-entered
	}
	r.ui.drain(1)
	require.Equal(t, 4, r.api.saveCount())
	got := make([]string, 0, 4)
	pos := make([]int, 0, 4)
	for _, s := range r.api.saves {
		got = append(got, s.lessonID)
		pos = append(pos, s.req.PositionSec)
	}
	require.Equal(t, []string{"l_A", "l_A", "l_B", "l_C"}, got)
	require.Equal(t, []int{100, 200, 0, 5}, pos)
	require.Zero(t, r.queue.Len())
}

func TestTrackerUpdateDoesNotAllocate(t *testing.T) {
	r := newRig(t)
	r.tr.Update("l_3503", "set-35", 1, 220, true)
	pos := 1.0
	allocs := testing.AllocsPerRun(1000, func() {
		pos += 0.25 // the clock stands still: no save is due
		r.tr.Update("l_3503", "set-35", pos, 220, true)
	})
	require.Zero(t, allocs)
	require.Zero(t, r.api.saveCount())
}

func BenchmarkTrackerUpdate(b *testing.B) {
	fake := clock.NewFake(t0)
	a := &fakeAPI{}
	q, _ := progress.NewQueue(store.NewMemory(), a)
	tr := progress.NewTracker(progress.Options{Sender: a, Queue: q, Clock: fake, Dispatch: func(func()) {}})
	defer tr.Close()
	tr.Update("l_3503", "set-35", 1, 220, true)
	b.ReportAllocs()
	pos := 1.0
	for b.Loop() {
		pos += 0.25
		tr.Update("l_3503", "set-35", pos, 220, true)
	}
}

func TestTrackerWaitIdleReturnsAtOnce(t *testing.T) {
	r := newRig(t)
	require.NoError(t, r.tr.Wait(t.Context()))
	require.Zero(t, testing.AllocsPerRun(100, func() { _ = r.tr.Wait(context.Background()) }))
}

func TestTrackerWaitInFlight(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 4)
	r.api.saveFn = func(n int) (model.ProgressResult, error) {
		entered <- struct{}{}
		if n == 1 {
			<-release
		}

		return model.ProgressResult{}, nil
	}
	r.tr.Update("l_3503", "set-35", 10, 220, true)
	r.tr.Pause()
	<-entered
	r.tr.Update("l_3503", "set-35", 20, 220, false)
	r.tr.Pause() // held

	done := make(chan error, 1)
	go func() { done <- r.tr.Wait(context.Background()) }()
	select {
	case err := <-done:
		t.Fatalf("Wait returned early: %v", err)
	default:
	}
	close(release)
	r.ui.drain(1) // first finished, held one launched
	<-entered
	select {
	case err := <-done:
		t.Fatalf("Wait returned with a save held: %v", err)
	default:
	}
	r.ui.drain(1)
	require.NoError(t, <-done)
	require.NoError(t, r.tr.Wait(t.Context()))
}

func TestTrackerWaitContextEnds(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	r.api.saveFn = func(int) (model.ProgressResult, error) {
		entered <- struct{}{}
		<-release

		return model.ProgressResult{}, nil
	}
	r.tr.Update("l_3503", "set-35", 10, 220, true)
	r.tr.Pause()
	<-entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.True(t, errors.Is(r.tr.Wait(ctx), context.Canceled))
	close(release)
	r.ui.drain(1)
	require.NoError(t, r.tr.Wait(t.Context()))
}

func TestTrackerWaitAfterClose(t *testing.T) {
	t.Parallel()
	r := newRig(t)
	release := make(chan struct{})
	entered := make(chan struct{}, 1)
	r.api.saveFn = func(int) (model.ProgressResult, error) {
		entered <- struct{}{}
		<-release

		return model.ProgressResult{}, nil
	}
	r.tr.Update("l_3503", "set-35", 10, 220, true)
	r.tr.Pause()
	<-entered
	close(release)
	r.tr.Close()
	require.Error(t, r.tr.Wait(t.Context()))
}

// TestPanicInASaveEndsOnlyThatSave makes the sender panic while a position is
// saved: the save ends with an error, the tracker is free for the next one, and
// the app does not end. (docs/plan/03 section 9: a bad answer never ends the app)
func TestPanicInASaveEndsOnlyThatSave(t *testing.T) {
	t.Parallel()
	safe.Expect("progress.send")

	r := newRig(t)
	r.api.saveFn = func(n int) (model.ProgressResult, error) {
		if n == 1 {
			panic("answer: index out of range [0] with length 0")
		}

		return model.ProgressResult{LessonID: "l_3503", PositionSec: 50}, nil
	}
	r.tr.Update("l_3503", "set-35", 42, 220, true)
	r.tr.Pause()
	r.ui.drain(1)
	require.Empty(t, r.saved, "no OnSaved for the panicked save")

	r.tr.Update("l_3503", "set-35", 50, 220, true)
	r.tr.Pause()
	r.ui.drain(1)
	require.Len(t, r.saved, 1, "the next save goes through")
}
