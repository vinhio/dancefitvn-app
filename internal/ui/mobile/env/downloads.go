//go:build !tvos

// Adapted from jplayer-mobile/internal/ui/views/port.go at 49de3c2 (the
// pattern: an interface the screens read the app through, a topic they listen
// to, the errors of the core packages re-exported so the screens import none of
// them). The model is internal/download (docs/plan/05 section 6).

package env

import (
	"errors"
	"fmt"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The errors of a download, the ones of internal/download (errors.Is finds
// either name), so that a screen imports no package but env for them.
var (
	// ErrWiFiOnly is Start on a network that is not Wi-Fi while "Chỉ tải qua
	// Wi‑Fi" is on: the lessons are queued paused and start on Wi-Fi (K23).
	ErrWiFiOnly = download.ErrWiFiOnly
	// ErrNoSpace is a device with less free space than the files need
	// ("Không đủ bộ nhớ", 03 section 9): nothing was queued.
	ErrNoSpace = download.ErrNoSpace
	// ErrChecksum is a finished file that is not the server's ("Tải lỗi, thử
	// lại"): the file is gone.
	ErrChecksum = download.ErrChecksum
	// ErrNoAudio is a lesson whose audio the server does not have (not_found of
	// lessons.audio): the download ends in it, the lesson reads as
	// kit.DlUnavailable for the rest of the run and no retry is offered.
	ErrNoAudio = download.ErrNoAudio
)

// DownloadItem is one lesson of the downloads as the screens show it: a file on
// the device (State DlDone, the rows of screen 11) or a download under way
// (State DlIng, its running cards). It carries the display data the download
// kept at Start, so that screen 11 and the offline screen show it with no
// API (offline, signed out).
type DownloadItem struct {
	LessonID, CourseID string
	// CourseName is the name of the course ("DANCEFIT SET 35") and
	// CourseLessons the number of lessons it has ("5/10 bài").
	CourseName    string
	CourseLessons int
	// CoverURL is the course's cover_url (K14), "" when the download was made before
	// the app kept it.
	CoverURL string
	// Title, Tag and No are the lesson's: "{NN} · {title}", "{tag} · {mm:ss}".
	Title, Tag string
	No         int
	// DurationSec is the lesson's length.
	DurationSec int
	// State is kit.DlDone for a file and kit.DlIng for a download under way
	// (also while it is paused).
	State kit.LessonDownload
	// Bytes is the size of the file as it is on the device (Q-P5-5); for a
	// download under way the size the job reports, the catalog's
	// (audio_size_bytes) while it is not known yet.
	Bytes int64
	// Done is how much of Bytes is in; Bytes for a file.
	Done int64
	// Progress is Done over Bytes, 0 to 1.
	Progress float32
	// Paused is a download that waits (the network is gone or is not Wi-Fi,
	// the app is in the background); Err says why: ErrWiFiOnly, api.ErrOffline.
	Paused bool
	// Err is why a download is paused or failed; nil otherwise.
	Err error
	// Damaged is a file whose bytes are the right size but that the system could
	// not play: the lesson streams in its place (K49) and the screens mark it in
	// --dg. The file stays until the user deletes it.
	Damaged bool
}

// CourseGroup is the downloaded lessons of one course, in the order they were
// downloaded: a group of screen 11 under its eyebrow "{course} · {n}/{total}
// bài".
type CourseGroup struct {
	CourseID, CourseName string
	// CourseLessons is the number of lessons of the course (the "/10").
	CourseLessons int
	// Items are the files of the group, oldest first.
	Items []DownloadItem
	// Bytes is the sum of their sizes.
	Bytes int64
}

// DownloadEventKind says what a DownloadEvent reports.
type DownloadEventKind uint8

// The kinds of DownloadEvent. DownloadEventUnknown is the zero value and tells
// nothing.
const (
	DownloadEventUnknown DownloadEventKind = iota
	// DownloadEventProgress is Done of Total bytes of a lesson, at most 4 a
	// second per lesson: update the ring and the text, rebuild nothing.
	DownloadEventProgress
	// DownloadEventChanged is the list changing: a download started, was
	// forgotten, a file was deleted, jobs paused or resumed. Read Items and
	// Jobs again.
	DownloadEventChanged
	// DownloadEventFinished is the file being in place and listed.
	DownloadEventFinished
	// DownloadEventPaused is a download waiting (the network cut, or the rule
	// Wi-Fi only); Err says which.
	DownloadEventPaused
	// DownloadEventFailed is a download that ended in error: Err is ErrNoSpace,
	// ErrChecksum or a refusal. The job stays listed until Forget.
	DownloadEventFailed
	// DownloadEventSpace is the free space having been read again (RefreshSpace).
	DownloadEventSpace
)

// DownloadEvent is a change of the downloads, delivered on the UI thread.
type DownloadEvent struct {
	Kind     DownloadEventKind
	LessonID string
	// No is the lesson's number (the toast "Đã tải xong bài NN").
	No          int
	Done, Total int64
	Err         error
}

// DownloadManager is the audio files of the device as the screens read and drive
// them: what Services.Downloads returns. Every method is for the UI thread and
// returns at once (the work is the app's, off it), the lists are copies the
// caller may keep.
type DownloadManager interface {
	// State is the download of a lesson and, while it is under way, how far it
	// is (0 to 1).
	State(lessonID string) (kit.LessonDownload, float32)
	// DoneCount is how many lessons have their audio on the device.
	DoneCount() int
	// Items are the files on the device, oldest first; Groups(Items) makes the
	// groups of screen 11. Screens call it on DownloadEventChanged, not each
	// frame.
	Items() []DownloadItem
	// Jobs are the downloads under way (queued, running, paused or failed and
	// not forgotten) in the order they were started.
	Jobs() []DownloadItem
	// TotalBytes is the sum of the sizes of Items (the summary of screen 11).
	TotalBytes() int64
	// FreeSpace is the free and the total space of the volume of the audio
	// folder, the last reading (cheap); ok is false while there is none or the
	// device cannot tell (Q-P5-9: show nothing). Ask RefreshSpace to renew it.
	FreeSpace() (free, total int64, ok bool)
	// RefreshSpace reads the space again off the UI thread; the answer is a
	// DownloadEventSpace. Screen 11 and sheet 09 call it when they open.
	RefreshSpace()
	// Start queues the audio of lessons of course c, in that order (`startDl`).
	// Lessons that are done or under way are skipped. It returns ErrWiFiOnly
	// when the rule Wi-Fi only holds and the network is not Wi-Fi (the lessons
	// are queued paused: they start on Wi-Fi), ErrNoSpace when the files do not
	// fit (nothing is queued), api.ErrOffline with no network (nothing is
	// queued); any other error is a refusal to start.
	Start(c model.Course, lessons []model.Lesson) error
	// Pause stops every download under way, keeping its bytes; Resume goes on
	// with them (the rules of Wi-Fi only and the network still apply).
	Pause()
	Resume()
	// Forget ends a download under way (the cross of a running row) and drops
	// its bytes; it returns at once.
	Forget(lessonID string)
	// Delete removes the file of a lesson from the device.
	Delete(lessonID string) error
	// Events tells what changed.
	Events() *event.Topic[DownloadEvent]
}

// DamagedFiles is what a DownloadManager may add: the files the system could not
// play (K49). Env.DownloadDamaged, Damaged find it; a manager that does not have it
// has no damaged file.
type DamagedFiles interface {
	// Damaged says whether the file of the lesson on the device does not play. It
	// allocates nothing.
	Damaged(lessonID string) bool
}

// WiFiPrefs is the preference "Chỉ tải qua Wi‑Fi" (`S.prefs.wifi`), which the
// Prefs of the services may keep besides the ones of Prefs. It is separate from
// Prefs so that a Prefs that has none still serves; Env.WiFiOnly finds it.
type WiFiPrefs interface {
	// WiFiOnly says whether downloads wait for Wi-Fi.
	WiFiOnly() bool
	// SetWiFiOnly sets it (persists, and tells the download rules).
	SetWiFiOnly(on bool)
}

// DownloadChoice is the radio of sheet 09 (`S.m.dlChoice`).
type DownloadChoice uint8

// The choices. DownloadChoiceUnknown reads as DownloadChoiceOne.
const (
	DownloadChoiceUnknown DownloadChoice = iota
	// DownloadChoiceOne is "Chỉ bài này".
	DownloadChoiceOne
	// DownloadChoiceAll is "Cả khoá …".
	DownloadChoiceAll
)

// DownloadSheetParams says what sheet 09 opens on: the course (its lessons in
// order), `sheetK` the lesson the choice "Chỉ bài này" is about, and the radio.
type DownloadSheetParams struct {
	Course   model.Course
	LessonID string
	Choice   DownloadChoice
}

// DeleteSheetParams says what sheet 14 asks about: the lesson whose file would
// go.
type DeleteSheetParams struct {
	CourseID, LessonID string
	No                 int
}

// nopManager is the DownloadManager of services that have none (Downloads
// returns nil): the lists are empty, nothing starts.
type nopManager struct {
	events event.Topic[DownloadEvent]
}

var _ DownloadManager = (*nopManager)(nil)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Downloads is the download manager of the services (Services.Downloads), or,
// when they have none (it returns nil), a manager with nothing in it whose Start
// fails with core.ErrUnsupported. Never nil, so a screen reads it without a check.
func (e *Env) Downloads() DownloadManager {
	if m := e.Services.Downloads(); m != nil {
		return m
	}
	if e.nopDl == nil {
		e.nopDl = &nopManager{}
	}

	return e.nopDl
}

// Downloader is Downloads, named for the screens that drive the downloads.
func (e *Env) Downloader() DownloadManager { return e.Downloads() }

// Foreground tells whether the app is in front of the user: the services' answer
// when they are a Foregrounder, true otherwise.
func (e *Env) Foreground() bool {
	if f, ok := e.Services.(Foregrounder); ok {
		return f.Foreground()
	}

	return true
}

// WatchForeground calls fn on the UI thread each time the app comes back to the
// front after it was away: the services' watch when they are a ForegroundWatcher,
// none otherwise. stop is never nil.
func (e *Env) WatchForeground(fn func()) (stop func()) {
	if w, ok := e.Services.(ForegroundWatcher); ok {
		return w.WatchForeground(fn)
	}

	return func() {}
}

// WiFiOnly is the preference "Chỉ tải qua Wi‑Fi": the services' when their
// Prefs keep it, else in memory, on at first (the prototype's default).
func (e *Env) WiFiOnly() bool {
	if p, ok := e.Prefs().(WiFiPrefs); ok {
		return p.WiFiOnly()
	}

	return !e.wifiOff
}

// SetWiFiOnly sets "Chỉ tải qua Wi‑Fi".
func (e *Env) SetWiFiOnly(on bool) {
	if p, ok := e.Prefs().(WiFiPrefs); ok {
		p.SetWiFiOnly(on)

		return
	}
	e.wifiOff = !on
}

// State is none.
func (*nopManager) State(string) (kit.LessonDownload, float32) { return kit.DlNone, 0 }

// DoneCount is 0.
func (*nopManager) DoneCount() int { return 0 }

// Items is empty.
func (*nopManager) Items() []DownloadItem { return nil }

// Jobs is empty.
func (*nopManager) Jobs() []DownloadItem { return nil }

// TotalBytes is 0.
func (*nopManager) TotalBytes() int64 { return 0 }

// FreeSpace is not known.
func (*nopManager) FreeSpace() (free, total int64, ok bool) { return 0, 0, false }

// RefreshSpace does nothing.
func (*nopManager) RefreshSpace() {}

// Start refuses: there is no download manager.
func (*nopManager) Start(model.Course, []model.Lesson) error {
	return fmt.Errorf("env: downloads: %w", core.ErrUnsupported)
}

// Pause and Resume do nothing.
func (*nopManager) Pause()  {}
func (*nopManager) Resume() {}

// Forget does nothing.
func (*nopManager) Forget(string) {}

// Delete refuses.
func (*nopManager) Delete(string) error {
	return fmt.Errorf("env: downloads: %w", core.ErrUnsupported)
}

// Events is a topic nobody publishes on.
func (n *nopManager) Events() *event.Topic[DownloadEvent] { return &n.events }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// Group makes the groups of screen 11 from the files: one per course in the
// order its first file was downloaded, the files of a group in their order
// (`$P:454`).
func Group(items []DownloadItem) []CourseGroup {
	var out []CourseGroup
	for _, it := range items {
		i := -1
		for j := range out {
			if out[j].CourseID == it.CourseID {
				i = j

				break
			}
		}
		if i < 0 {
			out = append(out, CourseGroup{CourseID: it.CourseID, CourseName: it.CourseName, CourseLessons: it.CourseLessons})
			i = len(out) - 1
		}
		out[i].Items = append(out[i].Items, it)
		out[i].Bytes += it.Bytes
	}

	return out
}

// LessonsToDownload returns the lessons of c whose audio a download would
// fetch: those that have one and are neither done nor under way (`startDl`
// skips the lessons that have an entry, Q-P5-10 counts only those with audio),
// with the sum of their catalog sizes.
func LessonsToDownload(c model.Course, d DownloadManager) (lessons []model.Lesson, bytes int64) {
	for _, l := range c.Lessons {
		if !l.HasAudio {
			continue
		}
		if st, _ := d.State(l.ID); st == kit.DlDone || st == kit.DlIng || st == kit.DlUnavailable {
			continue
		}
		lessons = append(lessons, l)
		bytes += l.AudioSizeBytes
	}

	return lessons, bytes
}

// FirstMissing returns the id of the lesson sheet 09 opens "Cả khoá" on: the
// first lesson of c with an audio and no entry, else the first lesson (`dlAll`, `$P:599`).
// "" for a course with no lesson.
func FirstMissing(c model.Course, d DownloadManager) string {
	for _, l := range c.Lessons {
		if st, _ := d.State(l.ID); st != kit.DlDone && st != kit.DlIng && st != kit.DlUnavailable && l.HasAudio {
			return l.ID
		}
	}
	if len(c.Lessons) > 0 {
		return c.Lessons[0].ID
	}

	return ""
}

// DownloadOf is the download of a lesson as a row or a button shows it: the
// manager's state, but a lesson that has no audio (hasAudio, its has_audio, is
// false) is kit.DlUnavailable unless its file is on the device or coming. progress
// is as State's. It allocates nothing.
func DownloadOf(d DownloadManager, lessonID string, hasAudio bool) (st kit.LessonDownload, progress float32) {
	st, progress = d.State(lessonID)
	if !hasAudio && st != kit.DlDone && st != kit.DlIng {
		return kit.DlUnavailable, 0
	}

	return st, progress
}

// Damaged tells whether the file of lessonID on the device is one the system could
// not play (K49); false for a manager that does not tell. It allocates nothing.
func Damaged(d DownloadManager, lessonID string) bool {
	f, ok := d.(DamagedFiles)

	return ok && f.Damaged(lessonID)
}

// NoAudio tells whether err is the server having no audio for the lesson.
func NoAudio(err error) bool { return errors.Is(err, ErrNoAudio) }

// NoSpace tells whether err is the device being full, wrapped or not.
func NoSpace(err error) bool { return errors.Is(err, ErrNoSpace) }

// WiFiOnlyErr tells whether err is the rule Wi-Fi only holding back a download.
func WiFiOnlyErr(err error) bool { return errors.Is(err, ErrWiFiOnly) }

// Offline tells whether err is the network being down.
func Offline(err error) bool { return errors.Is(err, api.ErrOffline) }
