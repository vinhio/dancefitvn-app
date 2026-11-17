//go:build !tvos

// The downloads of the phone, wired (P5.1c): env.DownloadManager over the
// user's download.Manager, the rules of the network and of "Chỉ tải qua
// Wi‑Fi" (03 section 2, Q-P5-2), the background task that lets a download
// run on while the app is away (F5a), and the network of the mock mode
// (the downloads' HTTP client cut with the device's network, F-5). The
// pattern of the network's part is jplayer-mobile/internal/app/network.go
// and app.go:423 (networkChanged) at 49de3c2 (network_mobile.go); the rest
// is new: jPlayer downloads nothing.

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The variables of a developer's build for the downloads (devenv.go).
const (
	// dlKBpsEnv is the speed, in KiB a second, the downloads' bodies are
	// read at (the 481 KB sample at 64 takes 7.5 s), so that a cut of the
	// network lands mid-file.
	dlKBpsEnv = "DFV_DL_KBPS"
	// badSHAEnv, "1", makes the API's sha256 of every audio file wrong (and
	// only it, not the size: P5 F-4): each download ends in ErrChecksum.
	badSHAEnv = "DFV_BADSHA"
)

// bgTaskName is the name of the background task of the downloads.
const bgTaskName = "dancefitvn.downloads"

// The reasons a download waits that are not the network's nor Wi-Fi's.
var (
	// errDlPaused is Pause: the jobs stopped by the user, to go on at Resume.
	errDlPaused = fmt.Errorf("app: downloads: paused: %w", core.ErrClosed)
	// errDlSignedOut is a sign-out: the bytes and the list are kept (Q-P5-8).
	errDlSignedOut = fmt.Errorf("app: downloads: %w", api.ErrSignedOut)
	// errDlBackground is the end of the time iOS gave the app in the
	// background (F5a): the downloads resume as it comes back.
	errDlBackground = fmt.Errorf("app: downloads: background time over: %w", core.ErrClosed)
	// errDlNoNetwork is the device with no network (Allowed).
	errDlNoNetwork = fmt.Errorf("app: downloads: no network: %w", api.ErrOffline)
	// errDlNoUser is a Start with nobody signed in.
	errDlNoUser = fmt.Errorf("app: downloads: no user: %w", core.ErrUnsupported)
)

// wireDownloads is env.DownloadManager over the download manager of the
// user: the files and the jobs as the screens show them are kept on the UI
// thread, read again from the manager when the list changes, so that State
// (each row, each event) asks nothing of the disk and allocates nothing;
// a progress event moves the numbers of its job only. Every method is for
// the UI thread.
type wireDownloads struct {
	w      *mobileWire
	events event.Topic[env.DownloadEvent]

	items  []env.DownloadItem // the files, oldest first
	jobs   []env.DownloadItem // the downloads under way, in start order
	itemAt map[string]int     // lesson to its index in items
	jobAt  map[string]int     // lesson to its index in jobs
	// noAudio holds the lessons the server has no audio for (a download ended in
	// ErrNoAudio): State reads them as unavailable for the rest of the run.
	noAudio map[string]struct{}
	total   int64 // the sum of the sizes of items
	active  int   // the jobs queued or running

	free, size int64
	spaceOK    bool

	away  bool   // the app is in the background
	bgEnd func() // ends the background task held, nil while none is
}

// dlTransport is the downloads' HTTP transport: it fails the requests, and
// the bodies being read, while the device has no network (down) — the API
// of the mock and its media are in the process, which no airplane mode
// cuts (P5 F-5) — and reads the bodies at kbps KiB a second at most when
// set (a developer's DFV_DL_KBPS).
type dlTransport struct {
	inner http.RoundTripper
	down  func() bool
	kbps  int
}

// dlBody is a body of dlTransport.
type dlBody struct {
	io.ReadCloser
	ctx  context.Context //nolint:containedctx // the request's, which ends the waits of the throttle
	down func() bool
	kbps int
}

// badSHASource is download.Source with the sha256 of every answer turned
// wrong, its size kept (a developer's DFV_BADSHA).
type badSHASource struct{ inner download.Source }

var (
	_ env.DownloadManager = (*wireDownloads)(nil)
	_ http.RoundTripper   = (*dlTransport)(nil)
	_ download.Source     = badSHASource{}
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newWireDownloads returns the downloads of w, empty until a user signs in.
func newWireDownloads(w *mobileWire) *wireDownloads {
	return &wireDownloads{w: w, itemAt: map[string]int{}, jobAt: map[string]int{}, noAudio: map[string]struct{}{}}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// State is the download of a lesson of the user: done, under way (and how
// far), unavailable (the server has no audio for it), or none. It allocates
// nothing.
func (d *wireDownloads) State(lessonID string) (kit.LessonDownload, float32) {
	if _, ok := d.itemAt[lessonID]; ok {
		return kit.DlDone, 1
	}
	if i, ok := d.jobAt[lessonID]; ok {
		return kit.DlIng, d.jobs[i].Progress
	}
	if _, ok := d.noAudio[lessonID]; ok {
		return kit.DlUnavailable, 0
	}

	return kit.DlNone, 0
}

// DoneCount is how many lessons of the user have their audio.
func (d *wireDownloads) DoneCount() int { return len(d.items) }

// Items are the files of the user, oldest first.
func (d *wireDownloads) Items() []env.DownloadItem { return slices.Clone(d.items) }

// Jobs are the downloads under way, in the order they were started.
func (d *wireDownloads) Jobs() []env.DownloadItem { return slices.Clone(d.jobs) }

// TotalBytes is the sum of the sizes of the files.
func (d *wireDownloads) TotalBytes() int64 { return d.total }

// FreeSpace is the last reading of the space of the audio folder's volume.
func (d *wireDownloads) FreeSpace() (free, total int64, ok bool) {
	return d.free, d.size, d.spaceOK
}

// RefreshSpace reads the space again off the UI thread; DownloadEventSpace
// tells the answer.
func (d *wireDownloads) RefreshSpace() {
	w := d.w
	if w.deps.FreeSpace == nil {
		return
	}
	w.background(func() {
		dir := w.audioDir()
		if dir == "" {
			return
		}
		free, size, err := w.deps.FreeSpace(dir)
		w.dispatch(func() {
			if err != nil {
				w.deps.Logf("dancefitvn: downloads: %v", err)
				d.free, d.size, d.spaceOK = 0, 0, false
			} else {
				d.free, d.size, d.spaceOK = free, size, true
			}
			d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventSpace})
		})
	})
}

// Start queues the audio of lessons of c that is neither on the device nor
// under way. With no network nothing is queued (api.ErrOffline); when the
// free space is known and the files do not fit, neither (ErrNoSpace); on
// mobile data with "Chỉ tải qua Wi‑Fi" the lessons wait for Wi-Fi
// (ErrWiFiOnly, K23).
func (d *wireDownloads) Start(c model.Course, lessons []model.Lesson) error {
	m := d.w.downloads()
	if m == nil {
		return errDlNoUser
	}
	if !d.w.online.Online() || d.w.netKind() == sys.NetworkNone {
		return fmt.Errorf("app: downloads: start: %w", api.ErrOffline)
	}
	todo := make([]model.Lesson, 0, len(lessons))
	var need int64
	for _, l := range lessons {
		// a lesson with no audio, or that the server has none for, is not queued; the
		// Shell tells the user before it asks (StartDownloads)
		if st, _ := d.State(l.ID); !l.HasAudio || st == kit.DlDone || st == kit.DlIng || st == kit.DlUnavailable {
			continue
		}
		todo = append(todo, l)
		need += l.AudioSizeBytes
	}
	if len(todo) == 0 {
		return nil
	}
	if d.spaceOK && need > d.free {
		return fmt.Errorf("app: downloads: start: %w", env.ErrNoSpace)
	}
	info := download.CourseInfo{ID: c.ID, Name: c.Name, Lessons: c.LessonCount, CoverURL: c.CoverURL}
	if info.Lessons == 0 {
		info.Lessons = len(c.Lessons)
	}
	var wait, failed error
	for _, l := range todo {
		err := m.Start(l, info)
		switch {
		case err == nil:
		case errors.Is(err, download.ErrWiFiOnly):
			wait = err
		case errors.Is(err, api.ErrOffline): // queued paused: it starts with the network
		case failed == nil:
			failed = err
		default:
		}
	}
	d.changed("")
	switch {
	case failed != nil:
		return fmt.Errorf("app: downloads: start: %w", failed)
	case wait != nil:
		return fmt.Errorf("app: downloads: start: %w", env.ErrWiFiOnly)
	default:
		return nil
	}
}

// Pause stops every download under way, keeping its bytes.
func (d *wireDownloads) Pause() { d.pauseAll(errDlPaused) }

// Resume goes on with the downloads paused; the rules of the network and of
// Wi-Fi only still hold.
func (d *wireDownloads) Resume() {
	if m := d.w.downloads(); m != nil {
		m.ResumeAll()
		d.changed("")
	}
}

// Forget ends a download under way and drops its bytes, at once (the
// transfer's goroutine removes them).
func (d *wireDownloads) Forget(lessonID string) {
	if m := d.w.downloads(); m != nil {
		m.Forget(lessonID)
		d.changed(lessonID)
	}
}

// Delete removes the file of a lesson (a download of it under way is
// forgotten first, so that nothing waits on the UI thread).
func (d *wireDownloads) Delete(lessonID string) error {
	m := d.w.downloads()
	if m == nil {
		return errDlNoUser
	}
	m.Forget(lessonID)
	err := m.Delete(lessonID)
	d.changed(lessonID)
	d.RefreshSpace()
	if err != nil {
		return fmt.Errorf("app: downloads: delete: %w", err)
	}

	return nil
}

// Events tells what changed, on the UI thread.
func (d *wireDownloads) Events() *event.Topic[env.DownloadEvent] { return &d.events }

// RoundTrip fails the request while the network is down, else hands it on
// with its body cut, and slowed, as the network is.
func (t *dlTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.down() {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: errNoNetwork}
	}
	res, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	res.Body = &dlBody{ReadCloser: res.Body, ctx: req.Context(), down: t.down, kbps: t.kbps}

	return res, nil
}

// Read fails once the network is down, and reads at kbps at most.
func (b *dlBody) Read(p []byte) (int, error) {
	if b.down() {
		return 0, &net.OpError{Op: "read", Net: "tcp", Err: errNoNetwork}
	}
	if b.kbps <= 0 {
		return b.ReadCloser.Read(p)
	}
	rate := b.kbps << 10
	if most := max(rate/10, 1); len(p) > most { // a tenth of a second at a time
		p = p[:most]
	}
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		t := time.NewTimer(time.Duration(n) * time.Second / time.Duration(rate))
		select {
		case <-t.C:
		case <-b.ctx.Done():
			t.Stop()

			return n, b.ctx.Err()
		}
	}

	return n, err
}

// Audio is the inner source's answer with a sha256 that no file has.
func (s badSHASource) Audio(ctx context.Context, lessonID string) (model.Audio, error) {
	a, err := s.inner.Audio(ctx, lessonID)
	if err != nil {
		return a, err
	}
	if a.SHA256 == "" || a.SHA256[0] != '0' {
		a.SHA256 = "0" + a.SHA256[min(1, len(a.SHA256)):]
	} else {
		a.SHA256 = "1" + a.SHA256[1:]
	}

	return a, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onEvent is the manager's OnEvent, on the UI thread: a progress moves its
// job's numbers (and allocates nothing), the others read the list again;
// each is told on to the screens, with the lesson's number.
func (d *wireDownloads) onEvent(ev download.Event) {
	out := env.DownloadEvent{LessonID: ev.LessonID, Done: ev.Done, Total: ev.Total, Err: ev.Err}
	switch ev.Kind {
	case download.EventProgress:
		i, ok := d.jobAt[ev.LessonID]
		if !ok {
			return
		}
		j := &d.jobs[i]
		j.Done = ev.Done
		if ev.Total > 0 {
			j.Bytes = ev.Total
		}
		if j.Bytes > 0 {
			j.Progress = min(float32(float64(j.Done)/float64(j.Bytes)), 1)
		}
		out.Kind, out.No, out.Total = env.DownloadEventProgress, j.No, j.Bytes
		d.events.Publish(out)

		return
	case download.EventFinished:
		out.Kind = env.DownloadEventFinished
		if m := d.w.downloads(); m != nil {
			if e, ok := m.Lookup(ev.LessonID); ok {
				out.No = e.No
			}
		}
		d.RefreshSpace()
	case download.EventPaused:
		out.Kind = env.DownloadEventPaused
	case download.EventFailed:
		out.Kind = env.DownloadEventFailed
		if errors.Is(ev.Err, download.ErrNoAudio) {
			d.noAudio[ev.LessonID] = struct{}{}
		}
		d.w.deps.Logf("dancefitvn: downloads: %s failed: %v", ev.LessonID, ev.Err)
	case download.EventUnknown:
		return
	default:
		return
	}
	if out.No == 0 {
		if i, ok := d.jobAt[ev.LessonID]; ok {
			out.No = d.jobs[i].No
		}
	}
	d.reload()
	d.events.Publish(out)
}

// changed reads the list again and tells the screens it changed.
func (d *wireDownloads) changed(lessonID string) {
	d.reload()
	d.events.Publish(env.DownloadEvent{Kind: env.DownloadEventChanged, LessonID: lessonID})
}

// reload reads the files and the jobs of the user from the manager (none
// with nobody signed in), then holds or lets go of the background task.
func (d *wireDownloads) reload() {
	d.items, d.jobs, d.total, d.active = d.items[:0], d.jobs[:0], 0, 0
	clear(d.itemAt)
	clear(d.jobAt)
	if m := d.w.downloads(); m != nil {
		for _, e := range m.List() {
			if _, ok := m.Path(e.LessonID); !ok {
				continue // the index lists a file that is not whole on the disk
			}
			d.itemAt[e.LessonID] = len(d.items)
			d.items = append(d.items, fileItem(e))
			d.total += e.Size
		}
		for _, j := range m.Jobs() {
			if j.State == download.StateQueued || j.State == download.StateRunning {
				d.active++
			}
			d.jobAt[j.LessonID] = len(d.jobs)
			d.jobs = append(d.jobs, jobItem(j))
		}
	}
	d.background()
}

// pauseAll pauses every download for reason.
func (d *wireDownloads) pauseAll(reason error) {
	if m := d.w.downloads(); m != nil {
		m.PauseAll(reason)
		d.changed("")
	}
}

// lifecycle follows the app: in the background a download under way holds
// a background task (F5a); active again, the task is let go and the
// downloads paused go on (the expiry paused them, the network may be back).
func (d *wireDownloads) lifecycle(state platform.LifecycleState) {
	switch state {
	case platform.LifecycleBackground:
		d.away = true
		d.background()
	case platform.LifecycleActive:
		d.away = false
		d.background()
		d.Resume()
	case platform.LifecycleUnknown, platform.LifecycleInactive:
	default:
	}
}

// background holds the background task while the app is away and a
// download is queued or running, and ends it otherwise.
func (d *wireDownloads) background() {
	want := d.away && d.active > 0
	switch {
	case want && d.bgEnd == nil:
		begin := d.w.deps.BackgroundTask
		if begin == nil {
			return
		}
		end, err := begin(bgTaskName, d.expired)
		if err != nil {
			if !errors.Is(err, core.ErrUnsupported) {
				d.w.deps.Logf("dancefitvn: downloads: background task: %v", err)
			}

			return
		}
		d.bgEnd = end
		d.w.deps.Logf("dancefitvn: downloads: background task begun, %d under way", d.active)
	case !want && d.bgEnd != nil:
		end := d.bgEnd
		d.bgEnd = nil
		end()
		d.w.deps.Logf("dancefitvn: downloads: background task ended, %d under way", d.active)
	default:
	}
}

// expired is the end of the background time: the downloads pause, their
// bytes kept, and go on when the app is active again.
func (d *wireDownloads) expired() {
	d.bgEnd = nil
	d.w.deps.Logf("dancefitvn: downloads: background time over, %d paused", d.active)
	d.pauseAll(errDlBackground)
}

// reset forgets the list: nobody is signed in.
func (d *wireDownloads) reset() { d.changed("") }

// fileItem is the screens' view of a file.
func fileItem(e download.Entry) env.DownloadItem {
	return env.DownloadItem{
		LessonID: e.LessonID, CourseID: e.CourseID, CourseName: e.CourseName, CourseLessons: e.CourseLessons, CoverURL: e.CoverURL,
		Title: e.Title, Tag: e.Tag, No: e.No, DurationSec: e.DurationSec,
		State: kit.DlDone, Bytes: e.Size, Done: e.Size, Progress: 1,
	}
}

// jobItem is the screens' view of a download under way: its size is the
// job's, the catalog's while the job knows none.
func jobItem(j download.Job) env.DownloadItem {
	it := env.DownloadItem{
		LessonID: j.LessonID, CourseID: j.CourseID, CourseName: j.CourseName, CourseLessons: j.CourseLessons, CoverURL: j.CoverURL,
		Title: j.Title, Tag: j.Tag, No: j.No, DurationSec: j.DurationSec,
		State: kit.DlIng, Bytes: j.Total, Done: j.Done, Paused: j.State == download.StatePaused, Err: j.Err,
	}
	if it.Bytes <= 0 {
		it.Bytes = j.CatalogBytes
	}
	if it.Bytes > 0 {
		it.Progress = min(float32(float64(it.Done)/float64(it.Bytes)), 1)
	}

	return it
}

// lifecycle follows the app's standing, on the UI thread: the playback's
// (player_mobile.go), then the downloads'.
func (w *mobileWire) lifecycle(state platform.LifecycleState) {
	w.pb.lifecycle(state)
	w.dls.lifecycle(state)
	switch state {
	case platform.LifecycleBackground:
		w.wasAway = true
	case platform.LifecycleActive:
		if !w.wasAway {
			return // the first Active of a launch is not a return
		}
		w.wasAway = false
		for _, wt := range w.watchList() {
			if wt.front != nil {
				wt.front()
			}
		}
	case platform.LifecycleUnknown, platform.LifecycleInactive:
	default:
	}
}

// netKind is the kind of the device's network now (NetworkUnknown off a
// phone).
func (w *mobileWire) netKind() sys.NetworkKind {
	if w.net == nil {
		return sys.NetworkUnknown
	}

	return w.net.Kind()
}

// allowed is the downloads' Allowed rule (03 section 2, Q-P5-2, Q-P5-3):
// no network, or the app offline, is api.ErrOffline; mobile data with "Chỉ
// tải qua Wi‑Fi" on is download.ErrWiFiOnly; any other network, an
// unknown one too, is allowed. It blocks nothing; any goroutine.
func (w *mobileWire) allowed() error {
	k := w.netKind()
	if k == sys.NetworkNone || !w.online.Online() {
		return errDlNoNetwork
	}
	if k == sys.NetworkCellular && w.wifiOnly.Load() {
		return download.ErrWiFiOnly
	}

	return nil
}

// netChanged follows a change of the kind of network, on the UI thread
// (03 section 2): none is offline at once and the downloads pause; a
// network back while offline is probed at once (the client's answer sets
// online, and online resumes the downloads); online, mobile data with "Chỉ
// tải qua Wi‑Fi" pauses the downloads and any other network resumes them
// (K23). An unknown kind tells nothing. While the API is the mock, the
// kind also cuts its transport and the downloads' (F-5).
func (w *mobileWire) netChanged(k sys.NetworkKind) {
	if !w.built.Load() || k == sys.NetworkUnknown {
		return // build applies the kind of the moment
	}
	w.deps.Logf("dancefitvn: network %s: online %t, wifi only %t", k, w.online.Online(), w.wifiOnly.Load())
	w.tr.setNetDown(k == sys.NetworkNone)
	switch {
	case k == sys.NetworkNone:
		w.dls.pauseAll(errDlNoNetwork)
		w.online.Set(false)
	case !w.online.Online():
		w.probe.now()
	case k == sys.NetworkCellular && w.wifiOnly.Load():
		w.dls.pauseAll(download.ErrWiFiOnly)
	default:
		w.dls.Resume()
	}
}

// wifiRuleChanged follows "Chỉ tải qua Wi‑Fi" set to on: on mobile data
// the downloads pause, else they go on. UI thread.
func (w *mobileWire) wifiRuleChanged(on bool) {
	if on && w.netKind() == sys.NetworkCellular {
		w.dls.pauseAll(download.ErrWiFiOnly)

		return
	}
	w.dls.Resume()
}

// useDownloads opens the download manager of user, closing another user's,
// and resumes the jobs a run before left (F5) when the network allows.
// Off the UI thread: the index is read from the disk.
func (w *mobileWire) useDownloads(user string) {
	w.mu.Lock()
	if w.closed || (w.dl != nil && w.dlUser == user) {
		w.mu.Unlock()

		return
	}
	old := w.dl
	w.dl, w.dlUser = nil, user
	w.mu.Unlock()
	if old != nil {
		old.PauseAll(errDlSignedOut)
		old.Close()
	}
	var src download.Source = w.client
	if w.knobs.badSHA {
		src = badSHASource{inner: src}
		w.deps.Logf("dancefitvn: downloads: every sha256 made wrong (%s)", badSHAEnv)
	}
	opts := download.Options{
		Client: src, HTTP: w.dlHTTP(), Dir: w.audioDir(), Clock: w.clk, Dispatch: w.dispatch, UserID: user,
		Allowed: w.allowed, OnEvent: w.dls.onEvent, Logf: w.deps.Logf,
	}
	if c := w.appConfig.Load(); c != nil {
		opts.MaxConcurrent = c.Download.MaxConcurrent
	}
	if w.deps.FreeSpace != nil {
		opts.FreeSpace = func() (int64, error) {
			free, _, err := w.deps.FreeSpace(opts.Dir)

			return free, err
		}
	}
	m, err := download.NewManager(opts)
	if err != nil {
		w.deps.Logf("dancefitvn: downloads: %v", err)

		return
	}
	w.mu.Lock()
	if w.closed || w.dlUser != user {
		w.mu.Unlock()
		m.Close()

		return
	}
	w.dl = m
	w.mu.Unlock()
	if n := len(m.Jobs()); n > 0 {
		w.deps.Logf("dancefitvn: downloads: %d left by a run before", n)
	}
	w.dispatch(func() {
		w.dls.changed("")
		w.dls.RefreshSpace()
		if w.allowed() == nil {
			w.dls.Resume()
		}
	})
}

// closeDownloads closes the manager of the user who signed out: the jobs
// pause, their bytes and records kept for when they sign in again
// (Q-P5-8), and the files stay (05 section 6). Any goroutine.
func (w *mobileWire) closeDownloads() {
	w.mu.Lock()
	m := w.dl
	w.dl, w.dlUser = nil, ""
	w.mu.Unlock()
	if m == nil {
		return
	}
	m.PauseAll(errDlSignedOut)
	m.Close()
	w.dispatch(w.dls.reset)
}

// downloads returns the download manager of the user, nil before a
// sign-in.
func (w *mobileWire) downloads() *download.Manager {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.dl
}

// audioDir is the folder of the downloads: the system's (sys.PrepareAudioDir,
// made by build), else <data>/audio; "" before build.
func (w *mobileWire) audioDir() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.audio
}

// prepareAudioDir finds the folder of the downloads, off the UI thread, at
// build: the system's (moving the files of <data>/audio there once), else
// <data>/audio.
func (w *mobileWire) prepareAudioDir(data string) {
	dir := filepath.Join(data, audioFolder)
	if w.deps.AudioDir != nil {
		got, err := w.deps.AudioDir()
		if err != nil {
			w.deps.Logf("dancefitvn: downloads: %v", err)
		}
		if got != "" {
			dir = got
		}
	}
	w.mu.Lock()
	w.audio = dir
	w.mu.Unlock()
}

// dlHTTP is the client of the downloads: a 30 s wait for the headers, no
// limit on the body, cut with the network (dlTransport) and slowed by a
// developer's DFV_DL_KBPS.
func (w *mobileWire) dlHTTP() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone() //nolint:forcetypeassert // the stdlib default
	tr.ResponseHeaderTimeout = 30 * time.Second
	if w.knobs.dlKBps > 0 {
		w.deps.Logf("dancefitvn: downloads: read at %d KiB/s (%s)", w.knobs.dlKBps, dlKBpsEnv)
	}

	return &http.Client{Transport: &dlTransport{inner: tr, down: w.tr.down, kbps: w.knobs.dlKBps}}
}

// dlKnobs reads the variables of a developer's build for the downloads.
func dlKnobs(k *wireKnobs) {
	if n, err := strconv.Atoi(devEnv(dlKBpsEnv)); err == nil && n > 0 {
		k.dlKBps = n
	}
	if devEnv(badSHAEnv) == "1" {
		k.badSHA = true
	}
}
