package download

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// DefaultMaxConcurrent is how many downloads run at once when
// Options.MaxConcurrent is 0 (AppConfig.Download.MaxConcurrent).
const DefaultMaxConcurrent = 2

// idRule is what a lesson id may be: it names a file.
var idRule = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// Source gives the signed URL of a lesson's MP3; *api.Client implements it.
type Source interface {
	Audio(ctx context.Context, lessonID string) (model.Audio, error)
}

// EventKind tells what an Event reports.
type EventKind int

// The kinds of Event.
const (
	EventUnknown  EventKind = iota
	EventProgress           // Done of Total bytes, at most 4 per second per job
	EventFinished           // the file is in place and listed
	EventPaused             // network cut or Wi-Fi rule; Err says which; ResumeAll continues
	EventFailed             // Err is ErrNoSpace, ErrChecksum or a refusal
)

// Event is a change of a job, delivered on the UI thread.
type Event struct {
	Kind     EventKind
	LessonID string
	Done     int64
	Total    int64
	Err      error
}

// CourseInfo is what Start records about the course of a lesson, for the
// screens that show a download without the API (offline, signed out).
type CourseInfo struct {
	ID      string
	Name    string
	Lessons int // number of lessons of the course
	// CoverURL is the course's cover_url, kept for the offline list.
	CoverURL string
}

// Job is a snapshot of a download that is not finished.
type Job struct {
	Meta
	LessonID string
	CourseID string
	State    State
	Done     int64
	Total    int64
	Err      error
}

// Options configures NewManager.
type Options struct {
	// Client gives the audio URLs; required.
	Client Source
	// HTTP fetches the files; nil is a client with a 30 s header timeout
	// and no overall timeout.
	HTTP *http.Client
	// Dir is <data>/audio; required.
	Dir string
	// MaxConcurrent is how many jobs run at once; 0 is 2.
	MaxConcurrent int
	// Allowed returns nil when a download may use the network now, and an
	// error (ErrWiFiOnly) when it may not. It must not block nor call the
	// Manager. Nil allows all.
	Allowed func() error
	// FreeSpace returns the free bytes of the data volume (K11). Nil skips
	// the check.
	FreeSpace func() (int64, error)
	// Clock times the progress events and stamps the index; required.
	Clock clock.Clock
	// Dispatch runs fn on the UI thread; nil runs it on the caller's
	// goroutine.
	Dispatch func(fn func())
	// UserID owns the downloads: List and Path show only its files.
	UserID string
	// OnEvent receives the events on the UI thread.
	OnEvent func(Event)
	// Logf receives diagnostic lines (resume at N, restart, failure). It
	// is called from any goroutine and never gets a URL or a token. Nil
	// logs nothing.
	Logf func(format string, args ...any)
}

// Manager downloads lesson MP3s: at most MaxConcurrent at once, the rest
// FIFO, each resumable with a Range request and verified by sha256. It is
// safe for concurrent use.
type Manager struct {
	opts Options
	idx  *index
	ctx  context.Context //nolint:containedctx // lifetime of every job
	stop context.CancelFunc
	wg   sync.WaitGroup

	mu       sync.Mutex
	jobs     map[string]*job
	waiting  []*job
	cleaning map[string]chan struct{} // lesson -> closed when its forgotten job's files are gone
	noAudio  map[string]struct{}      // lessons the server has no audio for (ErrNoAudio), until the app restarts
	running  int
	seq      int
	closed   bool
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewManager opens the index in opts.Dir (a missing one is empty).
func NewManager(opts Options) (*Manager, error) {
	if opts.Client == nil || opts.Dir == "" || opts.Clock == nil {
		return nil, fmt.Errorf("download: client, dir and clock are required: %w", core.ErrInvalid)
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = DefaultMaxConcurrent
	}
	if opts.HTTP == nil {
		tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the stdlib default
		tr.ResponseHeaderTimeout = 30 * time.Second
		opts.HTTP = &http.Client{Transport: tr}
	}
	if opts.Dispatch == nil {
		opts.Dispatch = func(fn func()) { fn() }
	}
	if err := os.MkdirAll(opts.Dir, 0o750); err != nil {
		return nil, fmt.Errorf("download: folder: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{opts: opts, idx: openIndex(opts.Dir), ctx: ctx, stop: cancel, jobs: map[string]*job{}, cleaning: map[string]chan struct{}{}, noAudio: map[string]struct{}{}}
	m.restore()

	return m, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Start queues the download of a lesson's MP3 and records its display
// metadata. A lesson already downloaded by this user, or already queued or
// running, is left alone; a paused or failed one is queued again. When the
// Allowed rule refuses (Wi-Fi only on mobile data) the job is kept paused
// with that error as its reason, EventPaused is sent, the error is
// returned, and ResumeAll starts it once the rule allows (Q-P5-2).
func (m *Manager) Start(lesson model.Lesson, course CourseInfo) error {
	if !idRule.MatchString(lesson.ID) || !lesson.HasAudio {
		return fmt.Errorf("download: lesson %q has no audio: %w", lesson.ID, core.ErrInvalid)
	}
	if e, ok := m.idx.get(lesson.ID, m.opts.UserID); ok && fileSize(m.finalPath(lesson.ID)) == e.Size {
		return nil
	}
	if m.Unavailable(lesson.ID) {
		return fmt.Errorf("download: lesson %q: %w", lesson.ID, ErrNoAudio)
	}
	meta := Meta{
		No: lesson.No, Title: lesson.Title, Tag: lesson.Tag, DurationSec: lesson.DurationSec, CatalogBytes: lesson.AudioSizeBytes,
		CourseName: course.Name, CourseLessons: course.Lessons, CoverURL: course.CoverURL,
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()

		return fmt.Errorf("download: manager closed: %w", core.ErrClosed)
	}
	j := m.jobs[lesson.ID]
	switch {
	case j == nil:
		m.seq++
		j = &job{lessonID: lesson.ID, courseID: course.ID, meta: meta, seq: m.seq, lastDone: -1, after: m.cleaning[lesson.ID]}
		m.jobs[lesson.ID] = j
	case j.state == StateQueued || j.state == StateRunning:
		m.mu.Unlock()

		return nil
	default:
		j.courseID, j.meta = course.ID, meta
		j.err, j.lastDone, j.hasAudio = nil, -1, false // a retry asks for a fresh URL
	}
	m.savePendingLocked(j)
	m.enqueueLocked(j)
	evs := m.pumpLocked()
	var refused error
	if j.state == StatePaused {
		refused = j.err
	}
	m.mu.Unlock()
	m.emit(evs...)

	return refused
}

// Unavailable tells whether the server answered not_found for the lesson's audio
// in this run: Start refuses it with ErrNoAudio and the screens show no download
// for it. The mark is not kept: a new run asks again.
func (m *Manager) Unavailable(lessonID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.noAudio[lessonID]

	return ok
}

// PauseAll pauses every queued and running job with reason as its Err
// (ErrWiFiOnly when Wi-Fi went away, a sign-out, ...) and sends EventPaused
// for each. Part files and pending records stay; ResumeAll or Start
// continue them, here or after a restart. It does not wait for the
// transfers to stop.
func (m *Manager) PauseAll(reason error) {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()

		return
	}
	m.waiting = nil
	var evs []Event
	for _, j := range m.sortedJobsLocked() {
		if j.state != StateQueued && j.state != StateRunning {
			continue
		}
		if j.active {
			j.stopped = true
			j.cancel()
		}
		j.state, j.err = StatePaused, reason
		evs = append(evs, Event{Kind: EventPaused, LessonID: j.lessonID, Done: j.done, Total: j.total, Err: reason})
	}
	m.mu.Unlock()
	m.emit(evs...)
}

// ResumeAll queues the paused jobs again, in their original order (the
// network is back, or the Wi-Fi rule is satisfied).
func (m *Manager) ResumeAll() {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()

		return
	}
	var paused []*job
	for _, j := range m.jobs {
		if j.state == StatePaused {
			paused = append(paused, j)
		}
	}
	slices.SortFunc(paused, func(a, b *job) int { return a.seq - b.seq })
	for _, j := range paused {
		m.enqueueLocked(j)
	}
	evs := m.pumpLocked()
	m.mu.Unlock()
	m.emit(evs...)
}

// Cancel stops the job of a lesson, deletes its partial file and forgets
// it. It returns when the transfer has stopped and the file is gone; the UI
// thread uses Forget instead. A finished download is not touched (Delete
// removes it).
func (m *Manager) Cancel(lessonID string) {
	if done := m.forget(lessonID); done != nil {
		<-done
	}
}

// Forget is Cancel without the wait: the job is gone from Jobs and from the
// pending list when it returns, and the transfer goroutine (or a helper)
// removes the part file afterwards. A Start of the same lesson right away
// waits for that removal before it writes.
func (m *Manager) Forget(lessonID string) { m.forget(lessonID) }

// Delete cancels the job of a lesson and removes the user's index entry and,
// when no other user holds the lesson, its file.
func (m *Manager) Delete(lessonID string) error {
	if !idRule.MatchString(lessonID) {
		return fmt.Errorf("download: lesson %q: %w", lessonID, core.ErrInvalid)
	}
	m.Cancel(lessonID)

	// The file is shared by the users who downloaded the lesson: the last
	// one to delete it removes it.
	var ferr error
	if !m.idx.othersHold(lessonID, m.opts.UserID) {
		ferr = removeFile(m.finalPath(lessonID))
	}

	return errors.Join(ferr, m.idx.remove(lessonID, m.opts.UserID))
}

// List returns the finished downloads of the user, oldest first.
func (m *Manager) List() []Entry {
	all := m.idx.all()

	return slices.DeleteFunc(all, func(e Entry) bool { return e.UserID != m.opts.UserID })
}

// Path returns the file of a finished download of the user.
func (m *Manager) Path(lessonID string) (string, bool) {
	e, ok := m.idx.get(lessonID, m.opts.UserID)
	if !ok {
		return "", false
	}
	p := m.finalPath(lessonID)
	if fileSize(p) != e.Size {
		return "", false
	}

	return p, true
}

// Lookup returns the finished download of a lesson of the user, with its
// display metadata.
func (m *Manager) Lookup(lessonID string) (Entry, bool) {
	e, ok := m.idx.get(lessonID, m.opts.UserID)
	if !ok {
		return Entry{}, false
	}

	return e, true
}

// Jobs returns the downloads that are not finished, in start order. Jobs
// restored from a previous run are paused, with Done = bytes on disk.
func (m *Manager) Jobs() []Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	js := m.sortedJobsLocked()
	out := make([]Job, len(js))
	for i, j := range js {
		out[i] = Job{Meta: j.meta, LessonID: j.lessonID, CourseID: j.courseID, State: j.state, Done: j.done, Total: j.total, Err: j.err}
	}

	return out
}

// Close stops every transfer and waits for them. Part files stay, so that
// the next run can resume them; no event is sent for the stopped jobs.
func (m *Manager) Close() {
	m.mu.Lock()
	m.closed = true
	m.mu.Unlock()
	m.stop()
	m.wg.Wait()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// restore loads the pending jobs of the user as paused ones: the .part file
// says how far each got.
func (m *Manager) restore() {
	for _, p := range m.idx.pendingOf(m.opts.UserID) {
		if !idRule.MatchString(p.LessonID) {
			continue
		}
		if e, ok := m.idx.get(p.LessonID, p.UserID); ok && fileSize(m.finalPath(p.LessonID)) == e.Size {
			continue // finished by a run that died before it dropped the record
		}
		m.seq = max(m.seq, p.Seq)
		m.jobs[p.LessonID] = &job{
			lessonID: p.LessonID, courseID: p.CourseID, meta: p.Meta, seq: p.Seq, state: StatePaused, lastDone: -1,
			done: fileSize(m.partPath(p.LessonID)), total: p.CatalogBytes, validator: p.Validator,
		}
	}
}

// forget removes the job of a lesson; the returned channel is closed when
// its part file is gone (nil when there was no job).
func (m *Manager) forget(lessonID string) chan struct{} {
	m.mu.Lock()
	j := m.jobs[lessonID]
	if j == nil {
		m.mu.Unlock()

		return nil
	}
	delete(m.jobs, lessonID)
	m.waiting = slices.DeleteFunc(m.waiting, func(x *job) bool { return x == j })
	j.cancelled = true
	if err := m.idx.removePending(lessonID, m.opts.UserID); err != nil {
		m.logf("download: %s pending: %v", lessonID, err)
	}
	done := make(chan struct{})
	if m.closed { // nothing runs any more
		m.mu.Unlock()
		m.removePart(lessonID)
		close(done)

		return done
	}
	var finished chan struct{}
	if j.active {
		j.cancel()
		finished = j.finished
	}
	m.cleaning[lessonID] = done
	m.wg.Add(1)
	m.mu.Unlock()
	go func() {
		defer m.wg.Done()
		if finished != nil {
			<-finished
		}
		m.removePart(lessonID)
		m.mu.Lock()
		if m.cleaning[lessonID] == done {
			delete(m.cleaning, lessonID)
		}
		m.mu.Unlock()
		close(done)
	}()

	return done
}

// removePart deletes the part file of a lesson unless another user's pending
// download is using it (the part is shared like the finished file).
func (m *Manager) removePart(lessonID string) {
	if !m.idx.othersPending(lessonID, m.opts.UserID) {
		_ = removeFile(m.partPath(lessonID))
	}
}

// savePendingLocked writes the pending record of j.
func (m *Manager) savePendingLocked(j *job) {
	p := Pending{LessonID: j.lessonID, CourseID: j.courseID, UserID: m.opts.UserID, Seq: j.seq, Validator: j.validator, Meta: j.meta}
	if err := m.idx.putPending(p); err != nil {
		m.logf("download: %s pending: %v", j.lessonID, err)
	}
}

// sortedJobsLocked returns the jobs in start order.
func (m *Manager) sortedJobsLocked() []*job {
	js := make([]*job, 0, len(m.jobs))
	for _, j := range m.jobs {
		js = append(js, j)
	}
	slices.SortFunc(js, func(a, b *job) int { return a.seq - b.seq })

	return js
}

// enqueueLocked puts j at the end of the waiting line.
func (m *Manager) enqueueLocked(j *job) {
	j.state = StateQueued
	m.waiting = append(m.waiting, j)
}

// pumpLocked starts waiting jobs while there is a free slot. A job the
// Allowed rule refuses is paused instead; the events of that are returned
// for the caller to emit once it has unlocked.
func (m *Manager) pumpLocked() (evs []Event) {
	for m.running < m.opts.MaxConcurrent {
		// A job whose paused run has not stopped yet waits for its slot.
		i := slices.IndexFunc(m.waiting, func(x *job) bool { return !x.active })
		if i < 0 {
			break
		}
		j := m.waiting[i]
		m.waiting = slices.Delete(m.waiting, i, i+1)
		if m.opts.Allowed != nil {
			if err := m.opts.Allowed(); err != nil {
				j.state, j.err = StatePaused, err
				evs = append(evs, Event{Kind: EventPaused, LessonID: j.lessonID, Done: j.done, Total: j.total, Err: err})

				continue
			}
		}
		ctx, cancel := context.WithCancel(m.ctx)
		j.state, j.cancel, j.finished, j.active = StateRunning, cancel, make(chan struct{}), true
		m.running++
		m.wg.Add(1)
		go m.run(ctx, j)
	}

	return evs
}

// run is the goroutine of one job.
func (m *Manager) run(ctx context.Context, j *job) {
	defer m.wg.Done()
	err := m.execute(ctx, j)

	m.mu.Lock()
	m.running--
	j.active = false
	j.stopped = false
	var ev Event
	switch {
	case j.cancelled: // forget removes the part
	case err == nil:
		delete(m.jobs, j.lessonID)
		m.waiting = slices.DeleteFunc(m.waiting, func(x *job) bool { return x == j })
		if !m.closed {
			ev = Event{Kind: EventFinished, LessonID: j.lessonID, Done: j.done, Total: j.total}
		}
	case m.closed:
		j.state = StatePaused
	case j.state != StateRunning: // PauseAll or a restart already set the state
	case isOffline(err):
		j.state, j.err = StatePaused, err
		ev = Event{Kind: EventPaused, LessonID: j.lessonID, Done: j.done, Total: j.total, Err: err}
	default:
		j.state, j.err = StateFailed, err
		if errors.Is(err, ErrNoAudio) {
			m.noAudio[j.lessonID] = struct{}{}
		}
		m.logf("download: %s failed: %v", j.lessonID, withoutURL(err))
		if rerr := m.idx.removePending(j.lessonID, m.opts.UserID); rerr != nil {
			m.logf("download: %s pending: %v", j.lessonID, rerr)
		}
		ev = Event{Kind: EventFailed, LessonID: j.lessonID, Done: j.done, Total: j.total, Err: err}
	}
	j.cancel()
	close(j.finished)
	evs := m.pumpLocked()
	m.mu.Unlock()
	if ev.Kind != EventUnknown {
		m.emit(ev)
	}
	m.emit(evs...)
}

// emit hands events to the UI thread; call it without m.mu held.
func (m *Manager) emit(evs ...Event) {
	if m.opts.OnEvent == nil {
		return
	}
	for _, ev := range evs {
		m.opts.Dispatch(func() { m.opts.OnEvent(ev) })
	}
}
