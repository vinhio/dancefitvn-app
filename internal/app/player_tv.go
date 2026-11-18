//go:build tvos

// The Apple TV's playback, wired: the system's engine (avftv, made once the
// view is attached, through lateMedia), the one player of the window
// (internal/player), the progress tracker and its offline queue, the
// preferences, the keep-awake, the system's Now Playing and the app's
// lifecycle. The pattern is player_mobile.go (itself
// jplayer-mobile/internal/app/media.go and app.go:376-400 at 49de3c2); the
// plane and the first frame are jplayer-tv/internal/app/app.go:393-424 at
// 781bee5 (underlay_tv.go) and P7.5a's EventVideoSize.

package app

import (
	"context"
	"sync/atomic"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/avftv"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
	tvenv "dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// tvPicture is what the Apple TV's engine does beyond media.Engine: the
// native plane under the view, its mirror, and the quality limit
// (*avftv.Engine). The fake engine has none of it.
type tvPicture interface {
	Place(r geom.Rect)
	SetMirror(on bool)
	SetLimits(q avftv.Quality)
}

// tvPlayback is tvenv.Playback: the player, plus the plane of the video
// and whether its first frame is on it (K5). It republishes the player's
// changes on its own topic, with a ChangeState when the picture comes.
// It lives on the UI thread, but for the goroutines of the Resolver and of
// the saves.
type tvPlayback struct {
	w      *tvWire
	late   *lateMedia
	sess   *tvSession
	pic    tvPicture // the made engine's, nil before or for the fake
	player *player.Player
	saver  *tvSaver
	prefs  *tvPrefs
	awake  *keepAwake
	np     *nowPlaying

	changes event.Topic[player.Change]
	// place, mirror and limits wait for the engine (made in attached).
	place   geom.Rect
	mirror  bool
	limits  avftv.Quality
	picture bool   // the first frame of the load is on the plane
	picTok  uint64 // the token of the load whose first frame it is
	unsubs  [2]func()
	closed  bool
}

// tvSession is the media.Session of the Apple TV: avftv activates the
// audio session itself and tells its interruptions through
// Config.OnInterruption, which this publishes as the session's events, so
// that the player and Now Playing hear them as on the iPhone.
type tvSession struct {
	events event.Topic[media.SessionEvent]
}

// tvResolver is player.Resolver over the API client.
type tvResolver struct{ w *tvWire }

// tvSaver is the player's Saver: the progress.Tracker, made at the first
// position told once the queue is open. UI thread.
type tvSaver struct {
	w *tvWire
	t *progress.Tracker
}

// tvSender is progress.Sender and progress.BatchSender over the client.
type tvSender struct{ w *tvWire }

// tvPrefs is tvenv.Prefs over the PrefsStore on the TV's state store: read
// once it is open, kept on the UI thread, a change saved off it.
type tvPrefs struct {
	w      *tvWire
	loaded bool
	p      store.Prefs
	// quality and autoNext are the values the next save writes: the latest
	// asked for, whatever order the saves run in.
	quality  atomic.Int64
	autoNext atomic.Bool
}

var (
	_ tvenv.Playback       = (*tvPlayback)(nil)
	_ tvenv.Prefs          = (*tvPrefs)(nil)
	_ media.Session        = (*tvSession)(nil)
	_ player.Resolver      = tvResolver{}
	_ player.Saver         = (*tvSaver)(nil)
	_ player.SeekNoter     = (*tvSaver)(nil)
	_ progress.Sender      = tvSender{}
	_ progress.BatchSender = tvSender{}
	_ tvPicture            = (*avftv.Engine)(nil)
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newTVPlayback makes the window's playback over w: the engine stands
// behind a lateMedia until attached (avftv needs the view for its layer),
// then the player, the keep-awake and Now Playing. UI thread.
func newTVPlayback(w *tvWire) *tvPlayback {
	p := &tvPlayback{w: w, sess: &tvSession{}}
	p.saver = &tvSaver{w: w}
	p.prefs = &tvPrefs{w: w, p: store.DefaultPrefs(true)}
	cfg := media.Config{Dispatch: w.dispatch, Clock: w.ui}
	newMedia := w.knobs.media
	if newMedia == nil {
		newMedia = w.deps.NewMedia
	}
	if newMedia == nil {
		newMedia = func(cfg media.Config) (platformMedia, error) {
			return newTVPlatformMedia(cfg, w.deps.Enqueue, w.underlay, p.sess)
		}
	}
	p.late = newLateMedia(cfg, newMedia, w.deps.Logf)
	p.late.onMade = p.made
	if w.deps.NowPlaying != nil {
		texts, err := locales.Texts()
		if err != nil {
			w.deps.Logf("dancefitvn: now playing: %v", err)
		}
		p.np = newNowPlaying(npOptions{
			sink: w.deps.NowPlaying, sess: p.sess, clk: w.clk, art: w.artwork, labels: npLabels(texts),
			ios: true, logf: w.deps.Logf,
		})
	}
	p.player = player.New(player.Options{
		Engine: p.late, Session: p.sess, Resolver: tvResolver{w: w}, Saver: p.saver,
		AutoNext: p.prefs.AutoNext, Dispatch: w.dispatch, Logf: w.deps.Logf,
	})
	if p.np != nil {
		p.np.follow(p.player)
	}
	p.mirror = p.player.Mirror() // on at first, as the prototype's S.pm
	p.awake = newKeepAwake(w.deps.Awake, p.player, w.deps.Logf)
	p.unsubs[0] = p.player.Changes().Subscribe(p.onChange)
	p.unsubs[1] = p.late.Events().Subscribe(p.onEngine)

	return p
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PlayVideo plays the video of l (resume, A and B: the player's).
func (p *tvPlayback) PlayVideo(l player.Lesson, list []player.Lesson) {
	p.player.PlayVideo(l, list)
}

// PlayAudio plays the sound of l; another lesson starts at at (the music
// screen from the nav, Q-P7-10), the one loaded goes on where it is.
func (p *tvPlayback) PlayAudio(l player.Lesson, list []player.Lesson, at float64) {
	other := l.ID != p.player.Lesson().ID
	p.player.PlayAudio(l, list)
	if other && at > 0 {
		p.player.Seek(at)
	}
}

// Play plays.
func (p *tvPlayback) Play() { p.player.Play() }

// Pause pauses.
func (p *tvPlayback) Pause() { p.player.Pause() }

// Toggle is the Play/Pause key.
func (p *tvPlayback) Toggle() { p.player.Toggle() }

// SetRate sets the speed.
func (p *tvPlayback) SetRate(r float64) { p.player.SetRate(r) }

// SetMirror flips the picture (the engine's layer, onChange).
func (p *tvPlayback) SetMirror(on bool) { p.player.SetMirror(on) }

// SetLoop turns the A–B loop on (seeking to A) or off.
func (p *tvPlayback) SetLoop(on bool) { p.player.SetLoop(on) }

// SetBeat shows the beat bar or not.
func (p *tvPlayback) SetBeat(on bool) { p.player.SetBeat(on) }

// Prev plays the lesson before, at 0.
func (p *tvPlayback) Prev() { p.player.Prev() }

// Next plays the lesson after, at 0.
func (p *tvPlayback) Next() { p.player.Next() }

// Leave leaves the player: the position saved, the plane hidden.
func (p *tvPlayback) Leave() {
	p.player.Leave()
	p.Place(geom.Rect{})
}

// Lesson is the lesson loaded.
func (p *tvPlayback) Lesson() player.Lesson { return p.player.Lesson() }

// Kind is video or audio.
func (p *tvPlayback) Kind() player.Kind { return p.player.Kind() }

// State is where the load stands.
func (p *tvPlayback) State() player.State { return p.player.State() }

// Err is why the load failed.
func (p *tvPlayback) Err() error { return p.player.Err() }

// Playing says the media should be playing.
func (p *tvPlayback) Playing() bool { return p.player.Playing() }

// Pos is the position in seconds.
func (p *tvPlayback) Pos() float64 { return p.player.Pos() }

// Dur is the length in seconds.
func (p *tvPlayback) Dur() float64 { return p.player.Dur() }

// Rate is the speed.
func (p *tvPlayback) Rate() float64 { return p.player.Rate() }

// Mirror is the mirror option.
func (p *tvPlayback) Mirror() bool { return p.player.Mirror() }

// Loop is the loop option.
func (p *tvPlayback) Loop() bool { return p.player.Loop() }

// Beat is the beat bar option.
func (p *tvPlayback) Beat() bool { return p.player.Beat() }

// A is the loop's start.
func (p *tvPlayback) A() float64 { return p.player.A() }

// B is the loop's end.
func (p *tvPlayback) B() float64 { return p.player.B() }

// BeatIndex is the lit cell of the beat bar.
func (p *tvPlayback) BeatIndex() int { return p.player.BeatIndex() }

// HasPrev tells whether a lesson comes before.
func (p *tvPlayback) HasPrev() bool { return p.player.HasPrev() }

// HasNext tells whether a lesson comes after.
func (p *tvPlayback) HasNext() bool { return p.player.HasNext() }

// Changes tells the player's changes, and a ChangeState when the first
// frame comes to the plane.
func (p *tvPlayback) Changes() *event.Topic[player.Change] { return &p.changes }

// Place puts the video's plane at r in the window (empty hides it); it
// waits for the engine when not made yet.
func (p *tvPlayback) Place(r geom.Rect) {
	p.place = r
	if p.pic != nil {
		p.pic.Place(r)
	}
}

// PictureReady tells that the first frame of the video loaded is on the
// plane (the engine's EventVideoSize).
func (p *tvPlayback) PictureReady() bool { return p.picture }

// Activate does nothing: avftv activates the audio session at each load.
func (*tvSession) Activate() error { return nil }

// Events is the session's topic.
func (s *tvSession) Events() *event.Topic[media.SessionEvent] { return &s.events }

// interrupted is avftv's OnInterruption, on the UI thread.
func (s *tvSession) interrupted(began, shouldResume bool) {
	if began {
		s.events.Publish(media.SessionEvent{Kind: media.InterruptBegin})

		return
	}
	s.events.Publish(media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: shouldResume})
}

// VideoURL is GET /lessons/{id}/stream. Blocks.
func (r tvResolver) VideoURL(ctx context.Context, lessonID string) (string, error) {
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

// AudioURL is GET /lessons/{id}/audio: the TV keeps no file. Blocks.
func (r tvResolver) AudioURL(ctx context.Context, lessonID string) (string, error) {
	w := r.w
	if err := w.setup(ctx); err != nil {
		return "", err
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

// Update is the player's tick: the tracker's. It allocates nothing in
// steady state.
func (s *tvSaver) Update(lessonID, courseID string, posSec, durSec float64, playing bool) {
	if t := s.tracker(); t != nil {
		t.Update(lessonID, courseID, posSec, durSec, playing)
	}
}

// Seeked tells the tracker that the user moved the position by hand
// (player.SeekNoter).
func (s *tvSaver) Seeked(lessonID string, posSec, durSec float64) {
	if t := s.tracker(); t != nil {
		t.Seeked(lessonID, posSec, durSec)
	}
}

// Pause saves now.
func (s *tvSaver) Pause() {
	if t := s.tracker(); t != nil {
		t.Pause()
	}
}

// Leave saves now and forgets the lesson.
func (s *tvSaver) Leave() {
	if t := s.tracker(); t != nil {
		t.Leave()
	}
}

// SaveProgress is PUT /lessons/{id}/progress; a save with no session, or
// whose session ended under way, fails with api.ErrSignedOut, which the
// tracker does not queue.
func (s tvSender) SaveProgress(ctx context.Context, lessonID string, r model.ProgressRequest) (model.ProgressResult, error) {
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
func (s tvSender) SendProgressBatch(ctx context.Context, r model.BatchRequest) (model.BatchResult, error) {
	res, err := s.w.client.SendProgressBatch(ctx, r)
	s.w.mgr.Handle(err)

	return res, err
}

// AutoNext is "Tự phát bài tiếp theo".
func (p *tvPrefs) AutoNext() bool { return p.cur().AutoNext }

// SetAutoNext sets it, at once for the player, saved off the UI thread.
func (p *tvPrefs) SetAutoNext(on bool) {
	p.cur()
	p.p.AutoNext = on
	p.autoNext.Store(on)
	ps := p.w.prefStore.Load()
	if ps == nil {
		return
	}
	p.w.background(func() {
		if err := ps.Update(func(x *store.Prefs) { x.AutoNext = p.autoNext.Load() }); err != nil {
			p.w.deps.Logf("dancefitvn: prefs: %v", err)
		}
	})
}

// Quality is "Chất lượng video".
func (p *tvPrefs) Quality() store.Quality { return p.cur().Quality }

// CycleQualityTV is `quality` of TV12 (index.html:743): Tự động, 1080p,
// 720p, Tự động (a stored 480p goes to Tự động, Q-P7-19); the engine's
// limit at once, saved off the UI thread.
func (p *tvPrefs) CycleQualityTV() {
	q := p.cur().Quality.NextTV()
	p.p.Quality = q
	p.quality.Store(int64(q))
	p.w.pb.applyQuality(q)
	ps := p.w.prefStore.Load()
	if ps == nil {
		return
	}
	p.w.background(func() {
		if err := ps.Update(func(x *store.Prefs) { x.Quality = store.Quality(p.quality.Load()) }); err != nil {
			p.w.deps.Logf("dancefitvn: prefs: %v", err)
		}
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// attached makes the engine now that the view is there (lateMedia.make:
// the Load, the speed and the background asked meanwhile are replayed).
// UI thread.
func (p *tvPlayback) attached() {
	if p.closed {
		return
	}
	p.late.make()
}

// made takes the engine made: the plane, the mirror and the quality limit
// asked meanwhile are put on it.
func (p *tvPlayback) made(pm platformMedia) {
	p.w.deps.Logf("dancefitvn: media: %s engine made", pm.name)
	pic, ok := pm.eng.(tvPicture)
	if !ok {
		return
	}
	p.pic = pic
	pic.SetLimits(p.limits)
	pic.SetMirror(p.mirror)
	if !p.place.Empty() {
		pic.Place(p.place)
	}
}

// onChange republishes c, puts the mirror on the engine, and forgets the
// picture at each new lesson (onEngine: at each new load). A seek that
// buffers (the player's StateLoading) keeps it: the frame stays on the plane
// and the engine tells the first frame once a load. It allocates nothing
// (the tick included).
func (p *tvPlayback) onChange(c player.Change) {
	switch c.Kind {
	case player.ChangeLesson:
		p.picture = false
	case player.ChangeOptions:
		if m := p.player.Mirror(); m != p.mirror {
			p.mirror = m
			if p.pic != nil {
				p.pic.SetMirror(m)
			}
		}
	case player.ChangeUnknown, player.ChangeState, player.ChangeTime, player.ChangeDuration, player.ChangeRate,
		player.ChangeSeek, player.ChangeEnded, player.ChangeError:
	default:
	}
	p.changes.Publish(c)
}

// onEngine hears the engine's first frame of a load (EventVideoSize, once
// per load on avftv) and tells it as a ChangeState; an event of another
// load (a reload, the next lesson) forgets it, with a ChangeState too.
func (p *tvPlayback) onEngine(ev media.Event) {
	if p.picture && ev.Token != p.picTok {
		p.picture = false
		p.changes.Publish(player.Change{Kind: player.ChangeState})
	}
	if ev.Kind != media.EventVideoSize || p.picture || p.player.Kind() != player.KindVideo {
		return
	}
	p.picture, p.picTok = true, ev.Token
	p.w.deps.Logf("dancefitvn: media: picture %dx%d on the plane", ev.Width, ev.Height) // once a load
	p.changes.Publish(player.Change{Kind: player.ChangeState})
}

// lifecycle follows the app's standing (Q-P7-14, 04 section 2.5): in the
// background a video pauses, a sound plays on, the position is saved and
// the screen saver let come; active again, the clock catches up with the
// wall clock, the screen is kept on for a video playing, and the
// positions saved offline are sent. UI thread.
func (p *tvPlayback) lifecycle(state platform.LifecycleState) {
	if p.closed {
		return
	}
	if state == platform.LifecycleActive {
		// The engine told no time in the background while a sound played
		// on: the position is read from it before the line below logs it.
		p.player.SyncPos()
	}
	if l := p.player.Lesson(); l.ID != "" && state != platform.LifecycleInactive {
		p.w.deps.Logf("dancefitvn: lifecycle %v: %s kind %d state %d at %.2f s", state, l.ID, p.player.Kind(), p.player.State(), p.player.Pos())
	}
	switch state {
	case platform.LifecycleBackground:
		p.player.Background(true)
	case platform.LifecycleActive:
		p.w.ui.Resync()
		p.w.ui.Expose() // back from the screen saver: the whole picture again
		p.player.Background(false)
		p.w.flushProgress()
	case platform.LifecycleUnknown, platform.LifecycleInactive:
	default:
	}
	p.awake.lifecycle(state)
}

// signedOut follows the end of a session: the lesson pauses, the plane
// hides and Now Playing is taken away. UI thread.
func (p *tvPlayback) signedOut() {
	if p.closed {
		return
	}
	if p.player.Lesson().ID != "" {
		p.player.Pause()
	}
	p.Place(geom.Rect{})
	if p.np != nil {
		p.np.clear()
	}
}

// close stops the player, the saves, the keep-awake and Now Playing, then
// releases the engine. UI thread.
func (p *tvPlayback) close() {
	if p.closed {
		return
	}
	p.closed = true
	for _, u := range p.unsubs {
		if u != nil {
			u()
		}
	}
	p.awake.close()
	if p.np != nil {
		p.np.close()
	}
	p.player.Close()
	if p.saver.t != nil {
		p.saver.t.Close()
	}
	if err := p.late.Close(); err != nil {
		p.w.deps.Logf("dancefitvn: media: close: %v", err)
	}
}

// prefsOpened reads the stored preferences once open and puts their
// quality on the engine. UI thread.
func (p *tvPlayback) prefsOpened() {
	if p.closed {
		return
	}
	p.applyQuality(p.prefs.cur().Quality)
	p.devPrefs()
}

// applyQuality limits the engine's HLS variant to q (avftv.SetLimits);
// before the engine is made, made applies it.
func (p *tvPlayback) applyQuality(q store.Quality) {
	p.limits = tvQuality(q)
	if p.pic != nil {
		p.pic.SetLimits(p.limits)
	}
}

// tvQuality is avftv's quality of the preference.
func tvQuality(q store.Quality) avftv.Quality {
	switch q {
	case store.QualityP1080:
		return avftv.Quality1080
	case store.QualityP720:
		return avftv.Quality720
	case store.QualityP480:
		return avftv.Quality480
	case store.QualityUnknown, store.QualityAuto:
		return avftv.QualityAuto
	default:
		return avftv.QualityAuto
	}
}

// tracker returns the progress tracker, made now if the queue is open.
func (s *tvSaver) tracker() *progress.Tracker {
	if s.t != nil {
		return s.t
	}
	w := s.w
	q := w.queue.Load()
	if q == nil {
		return nil
	}
	o := progress.Options{
		Sender: tvSender{w: w}, Queue: q, Clock: w.ui, Dispatch: w.dispatch, DeviceID: w.installID,
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
func (p *tvPrefs) cur() store.Prefs {
	if !p.loaded {
		if ps := p.w.prefStore.Load(); ps != nil {
			p.p, p.loaded = ps.Get(), true
		}
	}

	return p.p
}

// artwork is the npArt of the TV: the cover at url read and fitted off
// the UI thread (loadArtwork), handed to done on it; nil when it could
// not be.
func (w *tvWire) artwork(url string, done func(b []byte)) {
	w.background(func() {
		ctx, end := w.scoped(context.Background())
		defer end()
		ctx, cancel := context.WithTimeout(ctx, artTimeout)
		defer cancel()
		b, err := loadArtwork(ctx, &artworks, url, w.fetchBytes, w.deps.Logf)
		if err != nil {
			w.deps.Logf("dancefitvn: now playing: artwork: %v", err)
			b = nil
		}
		w.dispatch(func() { done(b) })
	})
}
