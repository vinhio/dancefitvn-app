package progress

import (
	"context"
	"errors"
	"sync"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Defaults of AppConfig.Progress (05 §2).
const (
	DefaultInterval  = 10 * time.Second
	DefaultThreshold = 0.9
)

// Sender saves one position; *api.Client implements it.
type Sender interface {
	SaveProgress(ctx context.Context, lessonID string, r model.ProgressRequest) (model.ProgressResult, error)
}

// Options configures NewTracker.
type Options struct {
	// Sender sends PUT /lessons/{id}/progress; required.
	Sender Sender
	// Queue takes the saves that could not reach the server; required.
	Queue *Queue
	// Clock is the UI thread's clock; required.
	Clock clock.Clock
	// Dispatch runs fn on the UI thread; required.
	Dispatch func(fn func())
	// Interval is the save cadence while playing; 0 is DefaultInterval
	// (AppConfig.Progress.SaveIntervalSec).
	Interval time.Duration
	// Threshold is the watched ratio; 0 is DefaultThreshold
	// (AppConfig.Progress.WatchedThreshold).
	Threshold float64
	// DeviceID is sent as device_id; may be empty.
	DeviceID string
	// OnSaved runs on the UI thread with the answer of a save.
	OnSaved func(model.ProgressResult)
	// OnWatched runs on the UI thread, once per lesson visit, when the
	// position first reaches the threshold.
	OnWatched func(lessonID string)
}

// Tracker saves the position of the lesson being played: every Interval
// while playing, on Pause and on Leave. All methods run on the UI thread.
// A save is sent on a goroutine, one at a time; a newer position of a lesson
// replaces the one of that lesson that waits (other lessons keep theirs, sent
// in order); offline and 5xx saves go to the Queue.
type Tracker struct {
	opts Options
	ctx  context.Context //nolint:containedctx // lifetime of the sends
	stop context.CancelFunc
	wg   sync.WaitGroup

	// UI thread state.
	lesson   string
	course   string
	pos, dur float64
	watched  bool
	lastSave time.Time
	savedPos int // last position handed to a save of this lesson, -1 none
	sending  bool
	pending  []snapshot // waiting saves, at most one per lesson, oldest first

	// open counts the saves that are in flight or held, until finish ends
	// them; idle is closed while open is 0. Guarded by mu; read by Wait from
	// any goroutine.
	mu   sync.Mutex
	open int
	idle chan struct{}
}

// snapshot is one save to send.
type snapshot struct {
	lessonID string
	req      model.ProgressRequest
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewTracker returns a Tracker; Close ends it.
func NewTracker(o Options) *Tracker {
	if o.Interval <= 0 {
		o.Interval = DefaultInterval
	}
	if o.Threshold <= 0 {
		o.Threshold = DefaultThreshold
	}
	ctx, cancel := context.WithCancel(context.Background())

	idle := make(chan struct{})
	close(idle)

	return &Tracker{opts: o, ctx: ctx, stop: cancel, savedPos: -1, idle: idle}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Update is the 250 ms tick of the player: the position and duration in
// seconds and whether it plays. It does not allocate unless it starts a
// save.
func (t *Tracker) Update(lessonID, courseID string, posSec, durSec float64, playing bool) {
	if lessonID != t.lesson {
		t.Leave()
		t.lesson, t.course = lessonID, courseID
		t.watched = false
		t.savedPos = -1
		t.lastSave = t.opts.Clock.Now()
	}
	t.pos, t.dur = posSec, durSec
	if !t.watched && Watched(posSec, durSec, t.opts.Threshold) {
		t.watched = true
		if t.opts.OnWatched != nil {
			t.opts.OnWatched(lessonID)
		}
	}
	if !playing {
		return
	}
	if now := t.opts.Clock.Now(); now.Sub(t.lastSave) >= t.opts.Interval {
		t.save(now)
	}
}

// Pause saves the current position now (the player paused or ended).
func (t *Tracker) Pause() {
	if t.lesson != "" {
		t.save(t.opts.Clock.Now())
	}
}

// Leave saves the current position and forgets the lesson (the player
// screen closes).
func (t *Tracker) Leave() {
	if t.lesson == "" {
		return
	}
	t.save(t.opts.Clock.Now())
	t.lesson, t.course = "", ""
	t.pos, t.dur = 0, 0
	t.savedPos = -1
}

// Watched reports whether pos has reached threshold of dur (198 of 220 at
// 0.9 is watched). An unknown duration is never watched.
func Watched(posSec, durSec, threshold float64) bool {
	return durSec > 0 && posSec >= threshold*durSec
}

// Wait blocks until no save is in flight or held (every dispatched finish
// has run), or ctx ends (its error), or the Tracker is closed
// (context.Canceled). It may be called from any goroutine except the UI
// thread while a save is open: the end of a save runs on the UI thread. Idle,
// it does not block or allocate.
func (t *Tracker) Wait(ctx context.Context) error {
	for {
		t.mu.Lock()
		idle, open := t.idle, t.open
		t.mu.Unlock()
		if open == 0 {
			return nil
		}
		select {
		case <-idle:
		case <-ctx.Done():
			return ctx.Err()
		case <-t.ctx.Done():
			return t.ctx.Err()
		}
	}
}

// Close stops the sends in flight and waits for them. After it, Update
// does nothing useful; call it when the app shuts down.
func (t *Tracker) Close() {
	t.stop()
	t.wg.Wait()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// save sends the current position unless it is the one last sent.
func (t *Tracker) save(now time.Time) {
	t.lastSave = now
	pos := int(max(t.pos, 0))
	if t.dur <= 0 || pos == t.savedPos { // nothing loaded yet, or nothing new
		return
	}
	t.savedPos = pos
	s := snapshot{lessonID: t.lesson, req: model.ProgressRequest{
		PositionSec: pos,
		DurationSec: int(max(t.dur, 0)),
		ClientTime:  now,
		DeviceID:    t.opts.DeviceID,
	}}
	if t.sending {
		t.hold(s)

		return
	}
	t.launch(s)
}

// hold keeps s as the waiting save of its lesson (latest wins).
func (t *Tracker) hold(s snapshot) {
	for i := range t.pending {
		if t.pending[i].lessonID == s.lessonID {
			t.pending[i] = s

			return
		}
	}
	t.pending = append(t.pending, s)
	t.opened(1)
}

// opened adds n to the open saves.
func (t *Tracker) opened(n int) {
	t.mu.Lock()
	if t.open == 0 {
		t.idle = make(chan struct{})
	}
	t.open += n
	t.mu.Unlock()
}

// closed removes n from the open saves and wakes Wait at 0.
func (t *Tracker) closed(n int) {
	t.mu.Lock()
	t.open -= n
	if t.open == 0 {
		close(t.idle)
	}
	t.mu.Unlock()
}

// launch starts the goroutine that sends s.
func (t *Tracker) launch(s snapshot) {
	t.sending = true
	t.opened(1)
	t.wg.Add(1)
	go t.send(s)
}

// send saves s, queues it when offline, and flushes the queue after a
// success (the network is back).
func (t *Tracker) send(s snapshot) {
	defer t.wg.Done()
	res, err := t.opts.Sender.SaveProgress(t.ctx, s.lessonID, s.req)
	if errors.Is(err, api.ErrOffline) || errors.Is(err, api.ErrServer) {
		_ = t.opts.Queue.Add(Item{LessonID: s.lessonID, PositionSec: s.req.PositionSec, DurationSec: s.req.DurationSec, ClientTime: s.req.ClientTime})
	}
	if err == nil && t.opts.Queue.Len() > 0 {
		_, _ = t.opts.Queue.Flush(t.ctx)
	}
	t.opts.Dispatch(func() { t.finish(res, err) })
}

// finish runs on the UI thread when a send ends.
func (t *Tracker) finish(res model.ProgressResult, err error) {
	t.sending = false
	if err == nil && t.opts.OnSaved != nil {
		t.opts.OnSaved(res)
	}
	if len(t.pending) > 0 && t.ctx.Err() == nil {
		s := t.pending[0]
		n := copy(t.pending, t.pending[1:])
		t.pending[n] = snapshot{}
		t.pending = t.pending[:n]
		t.sending = true
		t.wg.Add(1)
		go t.send(s) // already counted as held
	} else if n := len(t.pending); n > 0 {
		clear(t.pending)
		t.pending = t.pending[:0]
		t.closed(n) // the Tracker is closed: held saves are dropped
	}
	t.closed(1)
}
