//go:build !tvos

package shelltest

import (
	"fmt"
	"slices"
	"sync"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The demo downloads of the prototype (`S.dl`): lessons 1, 2, 3, 4 and 6 of the
// course have their audio, 7 is at 64 %; the free space is 18,2 GB of a disk
// that is 71 % full (K11, M11's bar).
const (
	demoCourse   = "set-35"
	demoIng      = 7
	demoIngShare = float32(.64)
	// demoFree and demoTotal: 18 200 000 000 free of 62 758 620 690 is 71 % used.
	demoFree  int64 = 18_200_000_000
	demoTotal int64 = 62_758_620_690
)

// demoDone are the lesson numbers of the demo course with their audio.
var demoDone = [...]int{1, 2, 3, 4, 6}

// FakeDownloads is env.DownloadManager in memory. It starts as the prototype's
// demo state (the files of lessons 1, 2, 3, 4 and 6 of Set 35, 7 at 64 %, 18,2
// GB free); Clear empties it. Start behaves as the app's manager does (Wi-Fi
// only on mobile data queues the lessons paused and returns env.ErrWiFiOnly, a
// full device queues nothing), and the knobs make the rest happen: progress,
// finish, fail, pause, the free space. Every method is safe for concurrent use;
// events are published on the caller's goroutine, which the tests make the UI
// thread. Get it with Fake.Dl.
type FakeDownloads struct {
	f *Fake

	mu       sync.Mutex
	known    map[string]env.DownloadItem // the display data of every lesson seen
	items    []env.DownloadItem
	jobs     []env.DownloadItem
	free     int64
	total    int64
	spaceOK  bool
	cellular bool
	noAudio  map[string]struct{} // lessons the server has no audio for (FailDownload with env.ErrNoAudio)

	started, cancelled, deleted []string
	pauses, resumes, refreshes  int
	startErr, deleteErr         error

	topic event.Topic[env.DownloadEvent]
}

var (
	_ env.DownloadManager = (*FakeDownloads)(nil)
	_ env.Foregrounder    = (*Fake)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newFakeDownloads returns the demo state over the course of the prototype (nil
// when the mock folder has none: then the state is empty).
func newFakeDownloads(f *Fake, c *model.Course) *FakeDownloads {
	d := &FakeDownloads{f: f, known: map[string]env.DownloadItem{}, noAudio: map[string]struct{}{}, free: demoFree, total: demoTotal, spaceOK: true}
	if c == nil {
		return d
	}
	for _, l := range c.Lessons {
		d.known[l.ID] = itemOf(*c, l)
	}
	for _, no := range demoDone {
		if l := lessonNo(c, no); l != nil {
			it := d.known[l.ID]
			it.State, it.Done, it.Progress = kit.DlDone, it.Bytes, 1
			d.items = append(d.items, it)
		}
	}
	if l := lessonNo(c, demoIng); l != nil {
		it := d.known[l.ID]
		it.State, it.Progress = kit.DlIng, demoIngShare
		it.Done = int64(float32(it.Bytes) * demoIngShare)
		d.jobs = append(d.jobs, it)
	}

	return d
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Dl returns the downloads of the Fake with their knobs.
func (f *Fake) Dl() *FakeDownloads { return f.dl }

// Downloads is the fake's download manager.
func (f *Fake) Downloads() env.DownloadManager { return f.dl }

// Foreground is env.Foregrounder: true (the app is in front) until
// SetForeground(false).
func (f *Fake) Foreground() bool {
	f.mu.Lock()
	defer f.mu.Unlock()

	return !f.background
}

// SetForeground says whether the app is in front (the default) or in the
// background, as the app's lifecycle does.
func (f *Fake) SetForeground(on bool) {
	f.mu.Lock()
	back := on && f.background // a return to the front
	f.background = !on
	fns := append([]func(){}, f.fronts...)
	f.mu.Unlock()
	if back {
		for _, fn := range fns {
			fn()
		}
	}
}

// WatchForeground is env.ForegroundWatcher: fn is called by the SetForeground(true)
// that follows a SetForeground(false).
func (f *Fake) WatchForeground(fn func()) (stop func()) {
	f.mu.Lock()
	f.fronts = append(f.fronts, fn)
	n := len(f.fronts)
	f.mu.Unlock()

	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if n <= len(f.fronts) {
			f.fronts[n-1] = func() {}
		}
	}
}

// State is the download of a lesson.
func (d *FakeDownloads) State(lessonID string) (kit.LessonDownload, float32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i := indexOf(d.items, lessonID); i >= 0 {
		return kit.DlDone, 1
	}
	if i := indexOf(d.jobs, lessonID); i >= 0 {
		return kit.DlIng, d.jobs[i].Progress
	}
	if _, ok := d.noAudio[lessonID]; ok {
		return kit.DlUnavailable, 0
	}

	return kit.DlNone, 0
}

// DoneCount is how many lessons have their audio.
func (d *FakeDownloads) DoneCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.items)
}

// Items are the files, oldest first.
func (d *FakeDownloads) Items() []env.DownloadItem {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.items)
}

// Jobs are the downloads under way, in the order they started.
func (d *FakeDownloads) Jobs() []env.DownloadItem {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.jobs)
}

// TotalBytes is the sum of the sizes of the files.
func (d *FakeDownloads) TotalBytes() int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	var n int64
	for _, it := range d.items {
		n += it.Bytes
	}

	return n
}

// FreeSpace is what SetSpace set.
func (d *FakeDownloads) FreeSpace() (free, total int64, ok bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.free, d.total, d.spaceOK
}

// RefreshSpace counts the request and answers with DownloadEventSpace at once.
func (d *FakeDownloads) RefreshSpace() {
	d.mu.Lock()
	d.refreshes++
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventSpace})
}

// Start queues the lessons not done or under way, as the manager does: the
// knob SetStartErr first, then no network (api.ErrOffline), then free space
// (env.ErrNoSpace), then the lessons; on mobile data with the preference "Chỉ
// tải qua Wi‑Fi" they are queued paused and env.ErrWiFiOnly is returned.
func (d *FakeDownloads) Start(c model.Course, lessons []model.Lesson) error {
	d.mu.Lock()
	if err := d.startErr; err != nil {
		d.mu.Unlock()

		return err
	}
	if !d.f.Online() {
		d.mu.Unlock()

		return fmt.Errorf("shelltest: start: %w", api.ErrOffline)
	}
	var todo []env.DownloadItem
	var need int64
	for _, l := range lessons {
		if indexOf(d.items, l.ID) >= 0 || indexOf(d.jobs, l.ID) >= 0 {
			continue
		}
		it := itemOf(c, l)
		d.known[l.ID] = it
		todo = append(todo, it)
		need += it.Bytes
	}
	if d.spaceOK && need > d.free {
		d.mu.Unlock()

		return fmt.Errorf("shelltest: start: %w", env.ErrNoSpace)
	}
	wait := d.cellular && d.f.prefs.WiFiOnly()
	for _, it := range todo {
		it.State = kit.DlIng
		if wait {
			it.Paused, it.Err = true, env.ErrWiFiOnly
		}
		d.jobs = append(d.jobs, it)
		d.started = append(d.started, it.LessonID)
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
	if wait && len(todo) > 0 {
		return fmt.Errorf("shelltest: start: %w", env.ErrWiFiOnly)
	}

	return nil
}

// Pause pauses every download under way.
func (d *FakeDownloads) Pause() {
	d.mu.Lock()
	d.pauses++
	for i := range d.jobs {
		d.jobs[i].Paused = true
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
}

// Resume goes on with the downloads paused, unless the rule Wi-Fi only holds.
func (d *FakeDownloads) Resume() {
	d.mu.Lock()
	d.resumes++
	wait := d.cellular && d.f.prefs.WiFiOnly()
	for i := range d.jobs {
		if !wait && d.jobs[i].Paused {
			d.jobs[i].Paused, d.jobs[i].Err = false, nil
		}
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
}

// Forget drops a download under way and records it.
func (d *FakeDownloads) Forget(lessonID string) {
	d.mu.Lock()
	d.cancelled = append(d.cancelled, lessonID)
	if i := indexOf(d.jobs, lessonID); i >= 0 {
		d.jobs = slices.Delete(d.jobs, i, i+1)
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})
}

// Delete removes the file of a lesson, or fails with SetDeleteErr; it records
// the call either way.
func (d *FakeDownloads) Delete(lessonID string) error {
	d.mu.Lock()
	d.deleted = append(d.deleted, lessonID)
	if err := d.deleteErr; err != nil {
		d.mu.Unlock()

		return err
	}
	if i := indexOf(d.items, lessonID); i >= 0 {
		d.items = slices.Delete(d.items, i, i+1)
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})

	return nil
}

// Events is the topic the fake publishes to.
func (d *FakeDownloads) Events() *event.Topic[env.DownloadEvent] { return &d.topic }

// ----- knobs ---------------------------------------------------------

// Clear removes every file and download: the state of a first launch (screen 12).
func (d *FakeDownloads) Clear() {
	d.mu.Lock()
	d.items, d.jobs = nil, nil
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
}

// SetDownload puts a lesson of the demo course (or one Start has seen) in a
// state: DlNone removes it, DlIng makes it a download at share p of its size,
// DlDone makes it a file, DlUnavailable says the server has no audio for it. It publishes DownloadEventChanged; a download that was
// under way and still is publishes DownloadEventProgress.
func (d *FakeDownloads) SetDownload(lessonID string, state kit.LessonDownload, p float32) {
	d.mu.Lock()
	it, ok := d.known[lessonID]
	if !ok {
		d.mu.Unlock()
		panic("shelltest: SetDownload: unknown lesson " + lessonID)
	}
	ev := env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID, No: it.No}
	was := indexOf(d.jobs, lessonID)
	if i := indexOf(d.items, lessonID); i >= 0 {
		d.items = slices.Delete(d.items, i, i+1)
	}
	if was >= 0 {
		d.jobs = slices.Delete(d.jobs, was, was+1)
	}
	switch state {
	case kit.DlDone:
		it.State, it.Done, it.Progress = kit.DlDone, it.Bytes, 1
		d.items = append(d.items, it)
	case kit.DlIng:
		it.State, it.Progress, it.Done = kit.DlIng, p, int64(float32(it.Bytes)*p)
		if was >= 0 {
			d.jobs = slices.Insert(d.jobs, was, it)
			ev.Kind, ev.Done, ev.Total = env.DownloadEventProgress, it.Done, it.Bytes
		} else {
			d.jobs = append(d.jobs, it)
		}
	case kit.DlUnavailable:
		d.noAudio[lessonID] = struct{}{}
	case kit.LessonDownloadUnknown, kit.DlNone:
	default:
	}
	d.mu.Unlock()
	d.topic.Publish(ev)
}

// FinishDownload makes a download under way a file and publishes
// DownloadEventFinished (then the app's wire would list it: Changed too).
func (d *FakeDownloads) FinishDownload(lessonID string) {
	d.mu.Lock()
	i := indexOf(d.jobs, lessonID)
	if i < 0 {
		d.mu.Unlock()
		panic("shelltest: FinishDownload: no download of " + lessonID)
	}
	it := d.jobs[i]
	d.jobs = slices.Delete(d.jobs, i, i+1)
	it.State, it.Done, it.Progress, it.Paused, it.Err = kit.DlDone, it.Bytes, 1, false, nil
	d.items = append(d.items, it)
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventFinished, LessonID: lessonID, No: it.No, Done: it.Bytes, Total: it.Bytes})
}

// FailDownload ends a download under way in error err and publishes
// DownloadEventFailed. The job stays listed until Forget, as in the manager.
func (d *FakeDownloads) FailDownload(lessonID string, err error) {
	d.mu.Lock()
	i := indexOf(d.jobs, lessonID)
	if i < 0 {
		d.mu.Unlock()
		panic("shelltest: FailDownload: no download of " + lessonID)
	}
	d.jobs[i].Err = err
	it := d.jobs[i]
	if env.NoAudio(err) { // as the app's wire does: the lesson is unavailable from now on
		d.noAudio[lessonID] = struct{}{}
	}
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventFailed, LessonID: lessonID, No: it.No, Done: it.Done, Total: it.Bytes, Err: err})
}

// PauseDownload pauses a download under way for reason err (env.ErrWiFiOnly,
// api.ErrOffline) and publishes DownloadEventPaused.
func (d *FakeDownloads) PauseDownload(lessonID string, err error) {
	d.mu.Lock()
	i := indexOf(d.jobs, lessonID)
	if i < 0 {
		d.mu.Unlock()
		panic("shelltest: PauseDownload: no download of " + lessonID)
	}
	d.jobs[i].Paused, d.jobs[i].Err = true, err
	it := d.jobs[i]
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventPaused, LessonID: lessonID, No: it.No, Done: it.Done, Total: it.Bytes, Err: err})
}

// SetSpace sets the free and the total space.
func (d *FakeDownloads) SetSpace(free, total int64) {
	d.mu.Lock()
	d.free, d.total, d.spaceOK = free, total, true
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventSpace})
}

// ClearSpace makes the free space unknown (Q-P5-9).
func (d *FakeDownloads) ClearSpace() {
	d.mu.Lock()
	d.free, d.total, d.spaceOK = 0, 0, false
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventSpace})
}

// SetOnWiFi says whether the network is Wi-Fi (the default) or mobile data. With
// the preference "Chỉ tải qua Wi‑Fi" on, mobile data pauses the downloads under
// way (ErrWiFiOnly) and Wi-Fi goes on with them, as the app's rules do (Q-P5-2).
func (d *FakeDownloads) SetOnWiFi(on bool) {
	d.mu.Lock()
	d.cellular = !on
	wifiOnly := d.f.prefs.WiFiOnly()
	var paused []env.DownloadEvent // as the manager's PauseAll tells, one per job
	for i := range d.jobs {
		switch {
		case !on && wifiOnly && !d.jobs[i].Paused:
			d.jobs[i].Paused, d.jobs[i].Err = true, env.ErrWiFiOnly
			it := d.jobs[i]
			paused = append(paused, env.DownloadEvent{Kind: env.DownloadEventPaused, LessonID: it.LessonID, No: it.No, Done: it.Done, Total: it.Bytes, Err: env.ErrWiFiOnly})
		case on && env.WiFiOnlyErr(d.jobs[i].Err):
			d.jobs[i].Paused, d.jobs[i].Err = false, nil
		default:
		}
	}
	d.mu.Unlock()
	for _, ev := range paused {
		d.topic.Publish(ev)
	}
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged})
}

// SetFileBytes sets the size of the file of a lesson as it is on the device
// (the entry of Items and the total), which is what Q-P5-5 says screens show and
// which differs from the catalog's audio_size_bytes by a few bytes. It
// publishes DownloadEventChanged; the lesson must be a file.
func (d *FakeDownloads) SetFileBytes(lessonID string, n int64) {
	d.mu.Lock()
	i := indexOf(d.items, lessonID)
	if i < 0 {
		d.mu.Unlock()
		panic("shelltest: SetFileBytes: no file of " + lessonID)
	}
	d.items[i].Bytes, d.items[i].Done = n, n
	d.mu.Unlock()
	d.topic.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})
}

// SetStartErr makes Start fail with err and queue nothing; nil puts it back.
func (d *FakeDownloads) SetStartErr(err error) {
	d.mu.Lock()
	d.startErr = err
	d.mu.Unlock()
}

// SetDeleteErr makes Delete fail with err; nil puts it back.
func (d *FakeDownloads) SetDeleteErr(err error) {
	d.mu.Lock()
	d.deleteErr = err
	d.mu.Unlock()
}

// Started returns the lessons Start queued, in order.
func (d *FakeDownloads) Started() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.started)
}

// Cancelled returns the lessons Forget was asked about, in order.
func (d *FakeDownloads) Cancelled() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.cancelled)
}

// Deleted returns the lessons Delete was asked about, in order.
func (d *FakeDownloads) Deleted() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.deleted)
}

// Calls returns how many times Pause, Resume and RefreshSpace were called.
func (d *FakeDownloads) Calls() (pauses, resumes, refreshes int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.pauses, d.resumes, d.refreshes
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// itemOf is the display data of lesson l of course c, as a download that has
// not started: Bytes is the catalog's size.
func itemOf(c model.Course, l model.Lesson) env.DownloadItem {
	n := c.LessonCount
	if n == 0 {
		n = len(c.Lessons)
	}

	return env.DownloadItem{
		LessonID: l.ID, CourseID: c.ID, CourseName: c.Name, CourseLessons: n, CoverURL: c.CoverURL,
		Title: l.Title, Tag: l.Tag, No: l.No, DurationSec: l.DurationSec, Bytes: l.AudioSizeBytes,
	}
}

// lessonNo returns the lesson numbered no of c, nil when it has none.
func lessonNo(c *model.Course, no int) *model.Lesson {
	for i := range c.Lessons {
		if c.Lessons[i].No == no {
			return &c.Lessons[i]
		}
	}

	return nil
}

// indexOf returns where the lesson is in list, -1 when it is not.
func indexOf(list []env.DownloadItem, lessonID string) int {
	return slices.IndexFunc(list, func(it env.DownloadItem) bool { return it.LessonID == lessonID })
}
