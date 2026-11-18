//go:build !tvos

// The phone's playback, wired: the system's engine (made late on the
// iPhone, lateMedia), the one player of the window (internal/player), the
// progress tracker and its offline queue (internal/progress), the
// preferences (store.PrefsStore), the keep-awake, the system's Now
// Playing (nowplaying_mobile.go) and the app's lifecycle. The pattern is
// jplayer-mobile/internal/app/media.go (mediaParts: openMedia,
// setBackground, close) and app.go:376-400 (lifecycle) at 49de3c2.

package app

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/safe"
	"dancefitvn-app/internal/store"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// playback is the window's playback: the media, the player over them,
// what saves the position, the preferences and the keep-awake. It lives
// on the UI thread, but for the goroutines of the Resolver and of the
// saves.
type playback struct {
	w      *mobileWire
	late   *lateMedia // nil when made at once
	sys    platformMedia
	eng    media.Engine // what the player plays through: late, or sys.eng
	player *player.Player
	saver  *wireSaver
	prefs  *wirePrefs
	opts   *optionsKeeper
	awake  *keepAwake
	np     *nowPlaying
	unsub  func() // stops onChange
	// framed: the window drew its first frame (late's make waits for it).
	framed bool
	closed bool
}

// wireResolver is player.Resolver over the API client and the downloads.
type wireResolver struct{ w *mobileWire }

// wireSaver is the player's Saver: the progress.Tracker, made at the
// first position told, once the queue is open and with the intervals of
// the app config read by then (05 section 2). UI thread.
type wireSaver struct {
	w *mobileWire
	t *progress.Tracker
}

// wireSender is progress.Sender and progress.BatchSender over the API
// client; a refused session signs out (auth.Manager.Handle).
type wireSender struct{ w *mobileWire }

// wirePrefs is env.Prefs over the PrefsStore: the values are read from
// the store once it is open, kept on the UI thread, and a change is
// saved off it (the save syncs the file). Before the store is open (no
// boot yet), the defaults of a first launch.
type wirePrefs struct {
	w      *mobileWire
	loaded bool
	p      store.Prefs
	// quality is the value the next save writes: the latest asked for,
	// whatever order the saves run in. "Chỉ tải qua Wi‑Fi" is the wire's
	// wifiOnly, which the downloads' rule reads on any goroutine.
	quality atomic.Int64
	// autoNext is the value the next save of "Tự phát bài tiếp theo"
	// writes, as quality.
	autoNext atomic.Bool
}

var (
	_ player.Resolver      = wireResolver{}
	_ player.Saver         = (*wireSaver)(nil)
	_ player.SeekNoter     = (*wireSaver)(nil)
	_ progress.Sender      = wireSender{}
	_ progress.BatchSender = wireSender{}
	_ env.Prefs            = (*wirePrefs)(nil)
	_ env.WiFiPrefs        = (*wirePrefs)(nil)
	_ env.AutoNextPrefs    = (*wirePrefs)(nil)
	_ env.Playback         = (*player.Player)(nil)
	_ env.Services         = (*mobileWire)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newPlayback makes the window's playback over w: the media of the
// system (or of the knobs, or of the deps in a test), made late on the
// iPhone after the first frame and the audio session's warm-up; the
// player; the keep-awake. UI thread, at the window's opening.
func newPlayback(w *mobileWire) *playback {
	p := &playback{w: w}
	p.saver = &wireSaver{w: w}
	p.prefs = &wirePrefs{w: w, p: store.DefaultPrefs(true)}
	cfg := media.Config{Dispatch: w.dispatch, Clock: w.ui}
	newMedia := w.knobs.media
	if newMedia == nil {
		newMedia = w.deps.NewMedia
	}
	late := w.deps.MediaLate
	if newMedia == nil {
		logf := w.deps.Logf
		newMedia = func(cfg media.Config) (platformMedia, error) { return newPlatformMedia(cfg, logf) }
		late = mediaLate
	}
	var sess media.Session
	if late {
		p.late = newLateMedia(cfg, newMedia, w.deps.Logf)
		p.late.onMade = p.made
		p.eng, sess = p.late, lateSession{m: p.late}
		warmMedia(cfg, p.start, w.deps.Logf)
	} else {
		pm := makeMedia(cfg, newMedia, w.deps.Logf)
		p.made(pm)
		p.eng = pm.eng
		if pm.sess != nil {
			sess = pm.sess
		}
	}
	p.np = newPlayingOf(w, sess) // before the player: it hears an interruption first
	p.player = player.New(player.Options{
		Engine: p.eng, Session: sess, Resolver: wireResolver{w: w}, Saver: p.saver,
		AutoNext: p.prefs.AutoNext, Dispatch: w.dispatch, Logf: w.deps.Logf,
	})
	if p.np != nil {
		p.np.follow(p.player)
	}
	p.opts = newOptionsKeeper(p.player, w.prefStore.Load, w.background, w.deps.Logf)
	p.awake = newKeepAwake(w.deps.Awake, p.player, w.deps.Logf)
	p.unsub = p.player.Changes().Subscribe(p.onChange)

	return p
}

// newPlayingOf returns the bridge to the system's Now Playing of w
// (mobileDeps.NowPlaying) over the audio session sess; nil when w has
// none (a shot, a test without it).
func newPlayingOf(w *mobileWire, sess media.Session) *nowPlaying {
	if w.deps.NowPlaying == nil {
		return nil
	}
	texts, err := locales.Texts()
	if err != nil {
		w.deps.Logf("dancefitvn: now playing: %v", err)
	}

	return newNowPlaying(npOptions{
		sink: w.deps.NowPlaying, sess: sess, clk: w.clk, art: w.artwork, labels: npLabels(texts),
		ios: w.deps.Platform != "android", logf: w.deps.Logf,
	})
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Playback is the one player of the window. UI thread.
func (w *mobileWire) Playback() env.Playback { return w.pb.player }

// Prefs are the preferences of the user. UI thread.
func (w *mobileWire) Prefs() env.Prefs { return w.pb.prefs }

// firstFrame runs after each frame of the window (ui.AfterFrame): after
// the first, the media made late are made at the next turn (start),
// unless the session is still warming. It allocates nothing after the
// first. UI thread.
func (p *playback) firstFrame() {
	if p.framed {
		return
	}
	p.framed = true
	if p.late != nil {
		p.w.dispatch(p.start)
	}
}

// start makes the media made late once the first frame is drawn and the
// session warmed; what cannot wait made them already. UI thread.
func (p *playback) start() {
	if p.late == nil || p.closed || !p.framed || !sessionWarm() {
		return
	}
	p.late.make()
}

// lifecycle follows the app's standing (Q-P4-17, 04 section 2.5): in the
// background a video pauses, a sound plays on, the position is saved,
// the engine stops its tick and its pictures, and the screen is let go;
// active again, the clock catches up with the wall clock, the engine
// tells the time again, the screen is kept on for a video playing, and
// the positions saved offline are sent. UI thread.
func (p *playback) lifecycle(state platform.LifecycleState) {
	if p.closed {
		return
	}
	if l := p.player.Lesson(); l.ID != "" && state != platform.LifecycleInactive {
		p.w.deps.Logf("dancefitvn: lifecycle %v: %s kind %d state %d at %.2f s", state, l.ID, p.player.Kind(), p.player.State(), p.player.Pos())
	}
	switch state {
	case platform.LifecycleBackground:
		p.keepFrame() // before the engine lets the pictures go
		p.player.Background(true)
	case platform.LifecycleActive:
		p.w.ui.Resync()
		p.player.Background(false)
		p.w.flushProgress()
	case platform.LifecycleUnknown, platform.LifecycleInactive:
	default:
	}
	p.awake.lifecycle(state)
}

// close stops the player, the saves and the keep-awake, then releases the
// engine. The session stays the process's. UI thread.
func (p *playback) close() {
	if p.closed {
		return
	}
	p.closed = true
	p.unsub()
	p.awake.close()
	if p.np != nil {
		p.np.close()
	}
	p.player.Close()
	if p.saver.t != nil {
		p.saver.t.Close()
	}
	if err := p.eng.Close(); err != nil {
		p.w.deps.Logf("dancefitvn: media: close: %v", err)
	}
}

// signedOut follows the end of a session (any sign-out, or another user
// signing in): the lesson playing stops and the system's Now Playing is
// taken away, so that nothing of the user before shows on the lock
// screen. UI thread.
func (p *playback) signedOut() {
	if p.closed {
		return
	}
	if p.player.Lesson().ID != "" {
		p.player.Pause()
	}
	if p.np != nil {
		p.np.clear()
	}
}

// WatchedLocal tells whether lessonID was watched in this run: the
// tracker saw 90 % of it played (online or queued offline), or the server
// answered a save of it as watched. The course read earlier does not
// know it yet (R-P4-12): screens 05 and 10 count it. UI thread.
func (w *mobileWire) WatchedLocal(lessonID string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.watched[lessonID]
}

// markWatched remembers lessonID as watched in this run. UI thread.
func (w *mobileWire) markWatched(lessonID string) {
	if lessonID == "" {
		return
	}
	w.mu.Lock()
	if w.watched == nil {
		w.watched = map[string]bool{}
	}
	w.watched[lessonID] = true
	w.mu.Unlock()
}

// forgetWatched drops what this run watched, at the end of a session:
// the next user's lessons are their own. Any goroutine.
func (w *mobileWire) forgetWatched() {
	w.mu.Lock()
	clear(w.watched)
	w.mu.Unlock()
}

// artwork is the npArt of the app: the cover at url read and fitted off
// the UI thread (the mock's media server started first, loadArtwork),
// handed to done on it; nil when it could not be. A sign-out cancels it.
func (w *mobileWire) artwork(url string, done func(b []byte)) {
	w.background(func() {
		ctx, end := w.scoped(context.Background())
		defer end()
		ctx, cancel := context.WithTimeout(ctx, artTimeout)
		defer cancel()
		b, err := w.readArtwork(ctx, url)
		if err != nil {
			w.deps.Logf("dancefitvn: now playing: artwork: %v", err)
			b = nil
		}
		w.dispatch(func() { done(b) })
	})
}

// readArtwork reads the cover at url as the system takes it: a PNG or a
// JPEG of artMaxBytes at most (loadArtwork). Blocks.
func (w *mobileWire) readArtwork(ctx context.Context, url string) ([]byte, error) {
	if err := w.setup(ctx); err != nil {
		return nil, err
	}

	return loadArtwork(ctx, &artworks, url, w.fetchBytes, w.deps.Logf)
}

// VideoURL is GET /lessons/{id}/stream: the signed HLS address. Blocks.
func (r wireResolver) VideoURL(ctx context.Context, lessonID string) (string, error) {
	w := r.w
	if err := w.setup(ctx); err != nil {
		return "", err
	}
	ctx, done := w.scoped(ctx)
	defer done()
	s, err := w.client.Stream(ctx, lessonID)
	if err != nil {
		w.mgr.Handle(err)

		return "", err
	}

	return s.HLSURL, nil
}

// AudioURL is the file of a downloaded lesson when it is on the device
// (download.Manager.Path checks it is there, whole), else GET
// /lessons/{id}/audio (Q-P4.3-2). Blocks.
func (r wireResolver) AudioURL(ctx context.Context, lessonID string) (string, error) {
	w := r.w
	if err := w.setup(ctx); err != nil {
		return "", err
	}
	if m := w.downloads(); m != nil {
		if path, ok := m.Path(lessonID); ok && w.local.use(lessonID) {
			return path, nil
		}
	}
	ctx, done := w.scoped(ctx)
	defer done()
	a, err := w.client.Audio(ctx, lessonID)
	if err != nil {
		w.mgr.Handle(err)

		return "", err
	}

	return a.URL, nil
}

// Update is the player's tick: the tracker's, once it can be made. It
// allocates nothing in steady state.
func (s *wireSaver) Update(lessonID, courseID string, posSec, durSec float64, playing bool) {
	if t := s.tracker(); t != nil {
		t.Update(lessonID, courseID, posSec, durSec, playing)
	}
}

// Seeked tells the tracker that the user moved the position by hand
// (player.SeekNoter).
func (s *wireSaver) Seeked(lessonID string, posSec, durSec float64) {
	if t := s.tracker(); t != nil {
		t.Seeked(lessonID, posSec, durSec)
	}
}

// Pause saves now.
func (s *wireSaver) Pause() {
	if t := s.tracker(); t != nil {
		t.Pause()
	}
}

// Leave saves now and forgets the lesson.
func (s *wireSaver) Leave() {
	if t := s.tracker(); t != nil {
		t.Leave()
	}
}

// SaveProgress is PUT /lessons/{id}/progress. A save made with nobody
// signed in, or whose session ended while it was under way (a sign-out),
// fails with api.ErrSignedOut, which the tracker does not queue: the
// position of one user is never sent later with the token of the next.
func (s wireSender) SaveProgress(ctx context.Context, lessonID string, r model.ProgressRequest) (model.ProgressResult, error) {
	gen := s.w.sessGen.Load()
	if _, ok := s.w.mgr.Session(); !ok {
		return model.ProgressResult{}, errSaveSignedOut
	}
	res, err := s.w.client.SaveProgress(ctx, lessonID, r)
	s.w.mgr.Handle(err)
	if err != nil && s.w.sessGen.Load() != gen {
		return res, errSaveSignedOut
	}

	return res, err
}

// SendProgressBatch is POST /progress/batch.
func (s wireSender) SendProgressBatch(ctx context.Context, r model.BatchRequest) (model.BatchResult, error) {
	res, err := s.w.client.SendProgressBatch(ctx, r)
	s.w.mgr.Handle(err)

	return res, err
}

// AutoNext is "Tự phát bài tiếp theo".
func (p *wirePrefs) AutoNext() bool { return p.cur().AutoNext }

// Quality is "Chất lượng video".
func (p *wirePrefs) Quality() store.Quality { return p.cur().Quality }

// CycleQuality is `quality` ($P:610): the next quality of the phone's
// cycle, at once for the engine (the iPhone's caps its HLS variant,
// K21) and saved off the UI thread.
func (p *wirePrefs) CycleQuality() {
	cur := p.cur()
	q := cur.Quality.NextMobile()
	p.p.Quality = q
	p.quality.Store(int64(q))
	if pb := p.w.pb; pb != nil {
		pb.applyQuality(q)
	}
	ps := p.w.prefStore.Load()
	if ps == nil {
		return
	}
	p.w.background(func() {
		err := ps.Update(func(x *store.Prefs) { x.Quality = store.Quality(p.quality.Load()) })
		if err != nil {
			p.w.deps.Logf("dancefitvn: prefs: %v", err)
		}
	})
}

// QualitySupported tells whether the engine of this phone caps the HLS
// variant (the iPhone's; not Android's MediaPlayer, K21).
func (p *wirePrefs) QualitySupported() bool { return qualityLimited }

// WiFiOnly is "Chỉ tải qua Wi‑Fi" (`S.prefs.wifi`).
func (p *wirePrefs) WiFiOnly() bool { return p.w.wifiOnly.Load() }

// SetWiFiOnly sets "Chỉ tải qua Wi‑Fi": at once for the downloads' rule
// (on mobile data they pause, or go on: K23), saved off the UI thread; the
// remote default no longer replaces the user's choice.
func (p *wirePrefs) SetWiFiOnly(on bool) {
	w := p.w
	p.cur()
	p.p.WiFiOnly = on
	w.wifiOnly.Store(on)
	w.prefsFresh.Store(false)
	w.wifiRuleChanged(on)
	ps := w.prefStore.Load()
	if ps == nil {
		return
	}
	w.background(func() {
		if err := ps.Update(func(x *store.Prefs) { x.WiFiOnly = w.wifiOnly.Load() }); err != nil {
			w.deps.Logf("dancefitvn: prefs: %v", err)
		}
	})
}

// SetAutoNext sets "Tự phát bài tiếp theo" (`pref:auto`, 15): at once for
// the player (it reads AutoNext at each end), saved off the UI thread.
func (p *wirePrefs) SetAutoNext(on bool) {
	w := p.w
	p.cur()
	p.p.AutoNext = on
	p.autoNext.Store(on)
	ps := w.prefStore.Load()
	if ps == nil {
		return
	}
	w.background(func() {
		if err := ps.Update(func(x *store.Prefs) { x.AutoNext = p.autoNext.Load() }); err != nil {
			w.deps.Logf("dancefitvn: prefs: %v", err)
		}
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onChange keeps the frame of a video that stops showing new ones (a
// pause, its end, a failure: Q-P4.4-2), so that screen 06 shows it while
// the engine has none to give (media.Surface.Keep), and lets it go when
// another lesson or media loads. Nothing else: a tick is not looked at.
// UI thread.
func (p *playback) onChange(c player.Change) {
	switch c.Kind {
	case player.ChangeRate, player.ChangeOptions:
		p.opts.changed()
	case player.ChangeLesson:
		if s := p.surface(); s != nil {
			s.Unkeep()
		}
	case player.ChangeState:
		switch p.player.State() {
		case player.StatePaused, player.StateEnded, player.StateError:
			p.keepFrame()
		case player.StateUnknown, player.StateIdle, player.StateLoading, player.StatePlaying:
		default:
		}
		if st := p.player.State(); st == player.StatePlaying || st == player.StatePaused {
			p.w.local.played(p.player.Lesson().ID) // the file loaded
		}
	case player.ChangeUnknown, player.ChangeTime, player.ChangeDuration,
		player.ChangeSeek, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// keepFrame keeps the frame on screen of the video loaded: Leave's pause
// and a pause come through onChange, the background through lifecycle,
// before the engine frees its pictures. UI thread.
func (p *playback) keepFrame() {
	if p.player.Kind() != player.KindVideo || p.player.Lesson().ID == "" {
		return
	}
	if s := p.surface(); s != nil {
		s.Keep()
	}
}

// surface is the engine's picture of the video; nil while the media made
// late are not made (asking would make them).
func (p *playback) surface() *media.Surface {
	if p.late != nil && !p.late.made {
		return nil
	}

	return p.eng.Video()
}

// made takes the media made, at once or late: the quality limit of the
// preferences is put on the engine.
func (p *playback) made(pm platformMedia) {
	p.sys = pm
	p.applyQuality(p.prefs.Quality())
}

// applyQuality limits the engine's HLS variant to q, on the iPhone
// (Q-P4-3, K21); on Android and the fake it does nothing.
func (p *playback) applyQuality(q store.Quality) {
	if p.sys.eng == nil {
		return // made late, not yet: made applies it
	}
	w, h := maxResolution(q)
	if limitResolution(p.sys.eng, w, h) {
		p.w.deps.Logf("dancefitvn: media: quality limit %d×%d", w, h)
	}
}

// tracker returns the progress tracker, made now if the queue is open;
// nil before the services are made.
func (s *wireSaver) tracker() *progress.Tracker {
	if s.t != nil {
		return s.t
	}
	w := s.w
	q := w.queue.Load()
	if q == nil {
		return nil
	}
	o := progress.Options{
		Sender: wireSender{w: w}, Queue: q, Clock: w.ui, Dispatch: w.dispatch, DeviceID: w.installID,
		OnWatched: w.markWatched,
		OnSaved: func(r model.ProgressResult) {
			if r.Watched {
				w.markWatched(r.LessonID)
			}
		},
	}
	if c := w.appConfig.Load(); c != nil {
		o.Interval = time.Duration(c.Progress.SaveIntervalSec) * time.Second
		o.Threshold = c.Progress.WatchedThreshold
	}
	s.t = progress.NewTracker(o)
	w.tracker.Store(s.t)

	return s.t
}

// cur returns the preferences on the UI thread: the store's once open.
func (p *wirePrefs) cur() store.Prefs {
	if !p.loaded {
		if ps := p.w.prefStore.Load(); ps != nil {
			p.p, p.loaded = ps.Get(), true
		}
	}

	return p.p
}

// prefsOpened reads the preferences the store holds once it is open, and
// puts their quality on the engine. UI thread.
func (p *playback) prefsOpened() {
	if p.closed {
		return
	}
	p.applyQuality(p.prefs.cur().Quality)
	p.opts.opened()
}

// openPrefs opens the preferences and the progress queue on the state
// folder, off the UI thread (setup): the first read of a file can take a
// while. A queue that cannot be read is logged and leaves the saves
// without one: the tracker is not made. The window's playback hears of
// it on the UI thread.
func (w *mobileWire) openPrefs() {
	_, err := w.prefs.Load(store.PrefsKey)
	w.prefsFresh.Store(errors.Is(err, core.ErrNotFound))
	ps := store.OpenPrefs(w.prefs, store.DefaultPrefs(true).WiFiOnly)
	if err := ps.LoadErr(); err != nil {
		w.deps.Logf("dancefitvn: prefs: %v", err)
	}
	w.prefStore.Store(ps)
	w.wifiOnly.Store(ps.Get().WiFiOnly)
	w.dispatch(w.pb.prefsOpened)
	q, err := progress.NewQueue(w.prefs, wireSender{w: w})
	if err != nil {
		w.deps.Logf("dancefitvn: progress queue: %v", err)

		return
	}
	w.queue.Store(q)
}

// configured takes the app config the server answered, off the UI
// thread: the tracker's intervals, and the Wi-Fi only default of a first
// launch (store.Prefs: WiFiOnly starts as the remote configuration says).
func (w *mobileWire) configured(c model.AppConfig) {
	w.appConfig.Store(&c)
	ps := w.prefStore.Load()
	if ps == nil || !w.prefsFresh.CompareAndSwap(true, false) || ps.Get().WiFiOnly == c.Download.WiFiOnlyDefault {
		return
	}
	if err := ps.Update(func(p *store.Prefs) { p.WiFiOnly = c.Download.WiFiOnlyDefault }); err != nil {
		w.deps.Logf("dancefitvn: prefs: %v", err)
	}
	w.wifiOnly.Store(c.Download.WiFiOnlyDefault)
}

// flushProgress sends the positions saved offline (POST /progress/batch)
// on a goroutine, when there are some and a user is signed in: at the
// network's return and at each return to the foreground. Any goroutine.
func (w *mobileWire) flushProgress() {
	q := w.queue.Load()
	if q == nil || q.Len() == 0 {
		return
	}
	if _, ok := w.mgr.Session(); !ok {
		return
	}
	w.background(func() {
		ctx, done := w.scoped(context.Background())
		defer done()
		ctx, cancel := context.WithTimeout(ctx, flushTimeout)
		defer cancel()
		res, err := q.Flush(ctx)
		switch {
		case err != nil && !errors.Is(err, context.Canceled):
			w.deps.Logf("dancefitvn: progress flush: %d sent, %d left: %v", res.Sent, q.Len(), err)
		case res.Sent > 0:
			w.deps.Logf("dancefitvn: progress flush: %d sent, %d accepted, %d rejected", res.Sent, res.Accepted, len(res.Rejected))
		default:
		}
	})
}

// clearQueue empties the offline progress queue: at every sign-out, the
// forced ones and the change of user included, so that the positions of one
// user are never sent with the token of the next.
func (w *mobileWire) clearQueue() {
	q := w.queue.Load()
	if q == nil {
		return
	}
	if err := q.Clear(); err != nil {
		w.deps.Logf("dancefitvn: progress queue clear: %v", err)
	}
}

// background runs fn on a goroutine that close waits for; none once
// closed.
func (w *mobileWire) background(fn func()) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()

		return
	}
	w.bg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.bg.Done()
		safe.Run("app.background", fn)
	}()
}
