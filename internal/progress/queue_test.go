package progress_test

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
)

func newQueue(t *testing.T) (*progress.Queue, *fakeAPI, *store.Memory) {
	t.Helper()
	a, m := &fakeAPI{}, store.NewMemory()
	q, err := progress.NewQueue(m, a)
	require.NoError(t, err)

	return q, a, m
}

func item(id string, pos int, at time.Duration) progress.Item {
	return progress.Item{LessonID: id, PositionSec: pos, DurationSec: 220, ClientTime: t0.Add(at)}
}

func TestQueueDedupesPerLesson(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		add  []progress.Item
		want []progress.Item
	}{
		{"newer replaces", []progress.Item{item("a", 1, 0), item("a", 9, time.Second)}, []progress.Item{item("a", 9, time.Second)}},
		{"older ignored", []progress.Item{item("a", 9, time.Second), item("a", 1, 0)}, []progress.Item{item("a", 9, time.Second)}},
		{"two lessons keep order", []progress.Item{item("a", 1, 0), item("b", 2, 0), item("a", 3, time.Second)}, []progress.Item{item("a", 3, time.Second), item("b", 2, 0)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, _, _ := newQueue(t)
			for _, it := range tc.add {
				require.NoError(t, q.Add(it))
			}
			require.Equal(t, tc.want, q.Items())
		})
	}
}

func TestQueuePersists(t *testing.T) {
	t.Parallel()
	q, a, m := newQueue(t)
	require.NoError(t, q.Add(item("a", 1, 0)))
	require.NoError(t, q.Add(item("b", 2, 0)))
	q2, err := progress.NewQueue(m, a)
	require.NoError(t, err)
	require.Equal(t, q.Items(), q2.Items())
}

func TestQueueCorruptDocumentIsEmpty(t *testing.T) {
	t.Parallel()
	m := store.NewMemory()
	m.Put(progress.QueueKey, []byte("{not json"))
	q, err := progress.NewQueue(m, &fakeAPI{})
	require.NoError(t, err)
	require.Zero(t, q.Len())
}

func TestQueueCorruptBackendStartsEmptyAndOverwrites(t *testing.T) {
	t.Parallel()
	m := store.NewMemory()
	m.LoadErr = fmt.Errorf("both slots bad: %w", core.ErrInvalid)
	q, err := progress.NewQueue(m, &fakeAPI{})
	require.NoError(t, err)
	require.Zero(t, q.Len())
	m.LoadErr = nil
	require.NoError(t, q.Add(item("a", 1, 0)))
	data, err := m.Load(progress.QueueKey)
	require.NoError(t, err)
	require.Contains(t, string(data), `"a"`)
}

func TestQueueBackendErrors(t *testing.T) {
	t.Parallel()
	m := store.NewMemory()
	m.LoadErr = fmt.Errorf("disk")
	_, err := progress.NewQueue(m, &fakeAPI{})
	require.Error(t, err)
	_, err = progress.NewQueue(nil, &fakeAPI{})
	require.Error(t, err)

	m2 := store.NewMemory()
	m2.SaveErr = fmt.Errorf("full")
	q, err := progress.NewQueue(m2, &fakeAPI{})
	require.NoError(t, err)
	require.Error(t, q.Add(item("a", 1, 0)))
}

func TestFlushBatchesOf100(t *testing.T) {
	t.Parallel()
	q, a, m := newQueue(t)
	for i := range 250 {
		require.NoError(t, q.Add(item("l_"+strconv.Itoa(i), i, 0)))
	}
	res, err := q.Flush(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int{100, 100, 50}, a.batchSizes())
	require.Equal(t, progress.FlushResult{Sent: 250, Accepted: 250}, res)
	require.Zero(t, q.Len())
	require.Equal(t, "l_0", a.batches[0][0].LessonID, "oldest first")
	data, err := m.Load(progress.QueueKey)
	require.NoError(t, err)
	var doc struct{ Items []progress.Item }
	require.NoError(t, json.Unmarshal(data, &doc))
	require.Empty(t, doc.Items, "persisted empty")
}

func TestFlushDropsRejected(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	a.batchFn = func(_ int, r model.BatchRequest) (model.BatchResult, error) {
		return model.BatchResult{Accepted: 1, Rejected: []jsontext.Value{jsontext.Value(`{"lesson_id":"b","code":"not_enrolled"}`)}}, nil
	}
	require.NoError(t, q.Add(item("a", 1, 0)))
	require.NoError(t, q.Add(item("b", 2, 0)))
	res, err := q.Flush(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, res.Sent)
	require.Equal(t, 1, res.Accepted)
	require.Len(t, res.Rejected, 1)
	require.JSONEq(t, `{"lesson_id":"b","code":"not_enrolled"}`, string(res.Rejected[0]))
	require.Zero(t, q.Len(), "rejected items are dropped, not retried")
}

func TestFlushKeepsOnError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
	}{
		{"offline", fmt.Errorf("%w: %w", api.ErrOffline, offlineErr)},
		{"server error", fmt.Errorf("x: %w", api.ErrServer)},
		{"signed out", api.ErrSignedOut},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			q, a, m := newQueue(t)
			a.batchFn = func(n int, _ model.BatchRequest) (model.BatchResult, error) {
				if n == 2 {
					return model.BatchResult{}, tc.err
				}

				return model.BatchResult{Accepted: 100}, nil
			}
			for i := range 150 {
				require.NoError(t, q.Add(item("l_"+strconv.Itoa(i), i, 0)))
			}
			res, err := q.Flush(context.Background())
			require.ErrorIs(t, err, tc.err)
			require.Equal(t, 100, res.Sent, "first chunk went")
			require.Equal(t, 50, q.Len(), "second chunk stays")
			q2, err := progress.NewQueue(m, a)
			require.NoError(t, err)
			require.Equal(t, 50, q2.Len(), "and is persisted")
		})
	}
}

func TestNoResendAfterSuccess(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	require.NoError(t, q.Add(item("a", 5, 0)))
	_, err := q.Flush(context.Background())
	require.NoError(t, err)
	require.NoError(t, q.Add(item("a", 5, time.Minute)), "same spot, later time")
	require.Zero(t, q.Len())
	require.NoError(t, q.Add(item("a", 6, time.Minute)))
	require.Equal(t, 1, q.Len())
	_, err = q.Flush(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int{1, 1}, a.batchSizes())
}

func TestFlushEmptyDoesNotCall(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	res, err := q.Flush(context.Background())
	require.NoError(t, err)
	require.Zero(t, res.Sent)
	require.Empty(t, a.batchSizes())
}

func TestFlushSingleFlight(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	a.gate = make(chan struct{})
	a.entered = make(chan struct{}, 4)
	require.NoError(t, q.Add(item("a", 1, 0)))

	var wg sync.WaitGroup
	results := make([]progress.FlushResult, 5)
	errs := make([]error, 5)
	wg.Add(1)
	go func() { defer wg.Done(); results[0], errs[0] = q.Flush(context.Background()) }()
	<-a.entered // the first is inside the sender
	for i := 1; i < 5; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errs[i] = q.Flush(context.Background()) }()
	}
	// The others must join the first, not call the sender: give them the chance, then release.
	close(a.gate)
	wg.Wait()
	for i := range 5 {
		require.NoError(t, errs[i])
	}
	require.Equal(t, []int{1}, a.batchSizes()[:1])
	require.Equal(t, 1, results[0].Sent)
}

func TestFlushWaiterHonoursContext(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	a.gate = make(chan struct{})
	a.entered = make(chan struct{}, 4)
	require.NoError(t, q.Add(item("a", 1, 0)))
	done := make(chan error, 1)
	go func() { _, err := q.Flush(context.Background()); done <- err }()
	<-a.entered
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := q.Flush(ctx)
	require.ErrorIs(t, err, context.Canceled)
	close(a.gate)
	require.NoError(t, <-done)
}

func TestFlushKeepsItemUpdatedMeanwhile(t *testing.T) {
	t.Parallel()
	q, a, _ := newQueue(t)
	require.NoError(t, q.Add(item("a", 1, 0)))
	a.batchFn = func(n int, _ model.BatchRequest) (model.BatchResult, error) {
		if n == 1 {
			require.NoError(t, q.Add(item("a", 50, time.Minute))) // the player saved again offline
		}

		return model.BatchResult{Accepted: 1}, nil
	}
	_, err := q.Flush(context.Background())
	require.NoError(t, err)
	require.Equal(t, []int{1, 1}, a.batchSizes(), "the newer position went in its own chunk")
	require.Equal(t, 50, a.batches[1][0].PositionSec)
	require.Zero(t, q.Len())
}

func TestQueueClear(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		sent  bool
		items []progress.Item
	}{
		{"empty queue", false, nil},
		{"waiting items", false, []progress.Item{item("a", 5, 0), item("b", 6, 0)}},
		{"forgets what was sent", true, []progress.Item{item("a", 5, 0)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			q, a, m := newQueue(t)
			for _, it := range tc.items {
				require.NoError(t, q.Add(it))
			}
			if tc.sent {
				_, err := q.Flush(context.Background())
				require.NoError(t, err)
			}
			require.NoError(t, q.Clear())
			require.Zero(t, q.Len())

			again, err := progress.NewQueue(m, a)
			require.NoError(t, err)
			require.Zero(t, again.Len(), "the empty queue is persisted")

			for _, it := range tc.items {
				require.NoError(t, q.Add(it))
			}
			require.Equal(t, len(tc.items), q.Len(), "the same item is queued again after a Clear")
		})
	}
}
