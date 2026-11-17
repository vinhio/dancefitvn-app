package progress

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/store"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// QueueKey is the store.Backend key of the offline queue
// (05 §6: progress-queue).
const QueueKey = "progress-queue"

// BatchSize is the most items one POST /progress/batch carries.
const BatchSize = 100

// BatchSender sends positions saved offline; *api.Client implements it.
type BatchSender interface {
	SendProgressBatch(ctx context.Context, r model.BatchRequest) (model.BatchResult, error)
}

// Item is the last known position of one lesson, waiting to be sent.
type Item struct {
	LessonID    string    `json:"lesson_id"`
	PositionSec int       `json:"position_sec"`
	DurationSec int       `json:"duration_sec"`
	ClientTime  time.Time `json:"client_time"`
}

// FlushResult is what Flush did.
type FlushResult struct {
	// Sent is how many items left the queue (accepted and rejected).
	Sent int
	// Accepted is how many of them the server accepted.
	Accepted int
	// Rejected holds the raw rejected[] entries of the server (their shape
	// is not documented, Q-P1-9). The items are already dropped from the
	// queue; callers log only len(Rejected).
	Rejected []jsontext.Value
}

// Queue keeps the positions saved while offline, one per lesson (the
// latest client_time wins), on a store.Backend. It is safe for concurrent
// use; it calls the Backend and the sender from the caller's goroutine.
type Queue struct {
	backend store.Backend
	sender  BatchSender

	mu     sync.Mutex
	items  []Item          // insertion order, one per lesson
	sent   map[string]Item // last item sent per lesson, this run
	flight *flush
}

// flush is a Flush in progress, shared by the callers that arrive meanwhile.
type flush struct {
	done chan struct{}
	res  FlushResult
	err  error
}

// document is the persisted form.
type document struct {
	Items []Item `json:"items"`
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewQueue loads the queue from b. A missing key is an empty queue; an
// unreadable one is dropped (a few lost positions are cheaper than a stuck
// app).
func NewQueue(b store.Backend, s BatchSender) (*Queue, error) {
	if b == nil || s == nil {
		return nil, fmt.Errorf("progress: backend and sender are required: %w", core.ErrInvalid)
	}
	q := &Queue{backend: b, sender: s, sent: map[string]Item{}}
	data, err := b.Load(QueueKey)
	switch {
	case errors.Is(err, core.ErrNotFound):
		return q, nil
	case errors.Is(err, core.ErrInvalid):
		// Both Mirror slots are bad: start empty; the next save overwrites.
		slog.Warn("progress: the saved queue is corrupt; starting empty")

		return q, nil
	case err != nil:
		return nil, fmt.Errorf("progress: load queue: %w", err)
	}
	var doc document
	if json.Unmarshal(data, &doc) == nil {
		for _, it := range doc.Items {
			q.put(it)
		}
	}

	return q, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Add records it as the position of its lesson. An older client_time than
// the one queued is ignored, and so is an item equal to the last one sent.
func (q *Queue) Add(it Item) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if last, ok := q.sent[it.LessonID]; ok && sameSpot(last, it) {
		return nil
	}
	if !q.put(it) {
		return nil
	}

	return q.save()
}

// Len returns how many lessons wait.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.items)
}

// Items returns a copy of the waiting items.
func (q *Queue) Items() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()

	return append([]Item(nil), q.items...)
}

// Clear drops every waiting item and persists the empty queue: the
// positions of a user that signed out must not be sent as the next user
// (call Flush first when they are still worth sending). The memory of what
// was sent goes too. An item added later is queued as usual; a Flush under
// way ends normally, and what it sent is not put back.
func (q *Queue) Clear() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
	q.sent = map[string]Item{}

	return q.save()
}

// Flush sends the queue by POST /progress/batch in chunks of BatchSize.
// Sent items leave the queue, the ones the server rejected too; on any
// error the chunk and the rest stay. A call that arrives while another
// runs waits for it and returns its result (single flight).
func (q *Queue) Flush(ctx context.Context) (FlushResult, error) {
	q.mu.Lock()
	if f := q.flight; f != nil {
		q.mu.Unlock()
		select {
		case <-f.done:
			return f.res, f.err
		case <-ctx.Done():
			return FlushResult{}, ctx.Err()
		}
	}
	f := &flush{done: make(chan struct{})}
	q.flight = f
	q.mu.Unlock()

	f.res, f.err = q.run(ctx)

	q.mu.Lock()
	q.flight = nil
	q.mu.Unlock()
	close(f.done)

	return f.res, f.err
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// run is the body of a Flush.
func (q *Queue) run(ctx context.Context) (FlushResult, error) {
	var res FlushResult
	for {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		chunk := q.next()
		if len(chunk) == 0 {
			return res, nil
		}
		req := model.BatchRequest{Items: make([]model.BatchItem, len(chunk))}
		for i, it := range chunk {
			req.Items[i] = model.BatchItem(it)
		}
		out, err := q.sender.SendProgressBatch(ctx, req)
		if err != nil {
			return res, fmt.Errorf("progress: flush: %w", err)
		}
		res.Sent += len(chunk)
		res.Accepted += out.Accepted
		res.Rejected = append(res.Rejected, out.Rejected...)
		if err := q.done(chunk); err != nil {
			return res, err
		}
	}
}

// next returns the next chunk to send.
func (q *Queue) next() []Item {
	q.mu.Lock()
	defer q.mu.Unlock()

	return append([]Item(nil), q.items[:min(len(q.items), BatchSize)]...)
}

// done removes the chunk that was sent; an item replaced by a newer one
// meanwhile stays.
func (q *Queue) done(chunk []Item) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, sent := range chunk {
		q.sent[sent.LessonID] = sent
		for i, cur := range q.items {
			if cur.LessonID == sent.LessonID {
				if cur.ClientTime.Equal(sent.ClientTime) && sameSpot(cur, sent) {
					q.items = append(q.items[:i], q.items[i+1:]...)
				}

				break
			}
		}
	}

	return q.save()
}

// put keeps it unless an item at least as new is there, and reports
// whether it changed the queue.
func (q *Queue) put(it Item) bool {
	for i, cur := range q.items {
		if cur.LessonID != it.LessonID {
			continue
		}
		if it.ClientTime.Before(cur.ClientTime) {
			return false
		}
		q.items[i] = it

		return true
	}
	q.items = append(q.items, it)

	return true
}

// save writes the queue to the backend; q.mu is held.
func (q *Queue) save() error {
	data, err := json.Marshal(document{Items: q.items})
	if err != nil {
		return fmt.Errorf("progress: encode queue: %w", err)
	}
	if err := q.backend.Save(QueueKey, data); err != nil {
		return fmt.Errorf("progress: save queue: %w", err)
	}

	return nil
}

// sameSpot reports whether two items say the same position.
func sameSpot(a, b Item) bool {
	return a.LessonID == b.LessonID && a.PositionSec == b.PositionSec && a.DurationSec == b.DurationSec
}
