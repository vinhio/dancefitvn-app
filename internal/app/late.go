// Adapted from jplayer-mobile/internal/app/late.go at 49de3c2 (lateMedia,
// lateSession). Trimmed: no Analyser, no Rejoiner (no live streams), no
// surface free hook; onMade hands the made media to the app (the quality
// limit of the iPhone's engine).

package app

import (
	"context"
	"strings"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// lateMedia stands for the system's playback until it is made, after the
// window's first frame and the audio session's warm-up: on iOS the
// session alone takes ~60 ms (local/p4/engine-ios.md), which the first
// frame would wait for. It is the media.Engine (and media.Backgrounder
// and media.TimeReader) the player plays through, and its lateSession the
// player's session.
//
// Until make, what the player asks is kept: the last Load (a local file
// that is not there fails at once, as both engines' first step does),
// the volume, the mute, the speed and the background; make makes the
// media and replays them. What cannot wait (Play, Pause, Seek, the
// Surface, the session's Activate) makes the media at once. The tokens
// are the engine's, shifted so those handed out before make go on: an
// event's token is the engine's plus offset. It lives on the UI thread.
type lateMedia struct {
	cfg      media.Config
	newMedia newMediaFunc
	logf     func(string, ...any)
	sys      platformMedia // what make made
	made     bool
	closed   bool
	// onMade, when set, is told once the playback is made, whatever made
	// it (the first frame, a Play, the screen of the video).
	onMade func(pm platformMedia)

	events     event.Topic[media.Event]
	sessEvents event.Topic[media.SessionEvent]
	forwardFn  func(media.Event)        // made once
	sessFn     func(media.SessionEvent) // made once
	unsubs     []func()
	offset     uint64 // the player's token less the engine's

	// What waits for make.
	tok       uint64 // the token of the last Load or Stop
	load      *lateLoad
	vol, rate float64
	muted     bool
	back      bool
	set       lateSet
}

// lateLoad is a Load waiting for make.
type lateLoad struct {
	ctx   context.Context //nolint:containedctx // replayed as Load got it
	src   media.Source
	opts  media.LoadOptions
	token uint64
}

// lateSet says which settings wait for make.
type lateSet struct {
	vol, rate, muted, back bool
}

// lateSession is the session of a lateMedia: its events are the system
// session's, once made.
type lateSession struct {
	m *lateMedia
}

var (
	_ media.Engine       = (*lateMedia)(nil)
	_ media.Backgrounder = (*lateMedia)(nil)
	_ media.TimeReader   = (*lateMedia)(nil)
	_ media.Session      = lateSession{}
)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newLateMedia returns the stand-in of the playback newMedia makes on
// cfg; logf logs.
func newLateMedia(cfg media.Config, newMedia newMediaFunc, logf func(string, ...any)) *lateMedia {
	m := &lateMedia{cfg: cfg, newMedia: newMedia, logf: logf}
	m.forwardFn = m.forward
	m.sessFn = m.sessEvents.Publish

	return m
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// make makes the system's playback, the fake engine when it fails
// (logged), and replays what waited: the Load, then the speed, the volume
// and the mute, as the player sets them after a Load, then the
// background. A replayed Load that fails tells it as an EventError. It
// does nothing once made, or closed.
func (m *lateMedia) make() {
	if m.made || m.closed {
		return
	}
	m.made = true
	t0 := time.Now()
	pm := makeMedia(m.cfg, m.newMedia, m.logf)
	m.sys = pm
	if pm.sess != nil {
		m.unsubs = append(m.unsubs, pm.sess.Events().Subscribe(m.sessFn))
	}
	m.offset = m.tok // the engine's token 0 is the last handed out
	var loadErr error
	if l := m.load; l != nil {
		m.load = nil
		var t uint64
		t, loadErr = pm.eng.Load(l.ctx, l.src, l.opts)
		m.offset = l.token - t
	}
	m.unsubs = append(m.unsubs, pm.eng.Events().Subscribe(m.forwardFn))
	if loadErr != nil {
		m.events.Publish(media.Event{Kind: media.EventError, Token: m.tok, Err: loadErr}) // the Load's: m.tok
	}
	m.replay(pm.eng)
	m.logf("dancefitvn: media %s made late in %.1f ms", pm.name, ms(time.Since(t0)))
	if m.onMade != nil {
		m.onMade(pm)
	}
}

// Load keeps src for make, or loads it.
func (m *lateMedia) Load(ctx context.Context, src media.Source, opts media.LoadOptions) (uint64, error) {
	if m.made {
		t0 := time.Now()
		t, err := m.sys.eng.Load(ctx, src, opts)
		m.logf("dancefitvn: media %s load in %.1f ms", m.sys.name, ms(time.Since(t0))) // R-P7-9: the UI thread's share

		return t + m.offset, err
	}
	m.tok++
	m.load = nil
	if err := enginekit.OpenError(enginekit.PhaseOpening, !strings.Contains(src.URL, "://"), src.URL, nil); err != nil {
		return m.tok, err
	}
	m.load = &lateLoad{ctx: ctx, src: src, opts: opts, token: m.tok}

	return m.tok, nil
}

// Play makes the media and plays.
func (m *lateMedia) Play() error {
	m.make()
	if !m.made || m.closed {
		return nil
	}

	return m.sys.eng.Play()
}

// Pause makes the media and pauses.
func (m *lateMedia) Pause() error {
	m.make()
	if !m.made || m.closed {
		return nil
	}

	return m.sys.eng.Pause()
}

// Stop forgets the Load waiting, or stops.
func (m *lateMedia) Stop() error {
	if m.made {
		return m.sys.eng.Stop()
	}
	m.tok++
	m.load = nil

	return nil
}

// Seek makes the media and seeks.
func (m *lateMedia) Seek(seconds float64) error {
	m.make()
	if !m.made || m.closed {
		return nil
	}

	return m.sys.eng.Seek(seconds)
}

// SetVolume keeps the volume for make, or sets it.
func (m *lateMedia) SetVolume(v float64) error {
	if m.made {
		return m.sys.eng.SetVolume(v)
	}
	m.vol, m.set.vol = v, true

	return nil
}

// SetMuted keeps the mute for make, or sets it.
func (m *lateMedia) SetMuted(on bool) error {
	if m.made {
		return m.sys.eng.SetMuted(on)
	}
	m.muted, m.set.muted = on, true

	return nil
}

// SetRate keeps the speed for make, or sets it.
func (m *lateMedia) SetRate(r float64) error {
	if m.made {
		return m.sys.eng.SetRate(r)
	}
	m.rate, m.set.rate = r, true

	return nil
}

// SetBackground keeps the background for make, or tells the engine when
// it follows the lifecycle.
func (m *lateMedia) SetBackground(on bool) {
	if !m.made {
		m.back, m.set.back = on, true

		return
	}
	if b, ok := m.sys.eng.(media.Backgrounder); ok {
		b.SetBackground(on)
	}
}

// ReadTime reads the engine's position of the player's load token
// (media.TimeReader); false before make, which no load has played yet,
// or from an engine that cannot tell.
func (m *lateMedia) ReadTime(token uint64) (float64, bool) {
	if !m.made || m.closed || token < m.offset {
		return 0, false
	}
	r, ok := m.sys.eng.(media.TimeReader)
	if !ok {
		return 0, false
	}

	return r.ReadTime(token - m.offset)
}

// Events returns the topic the engine's events reach the player on.
func (m *lateMedia) Events() *event.Topic[media.Event] { return &m.events }

// Video makes the media and returns the engine's surface; nil once
// closed without having been made.
func (m *lateMedia) Video() *media.Surface {
	m.make()
	if !m.made {
		return nil
	}

	return m.sys.eng.Video()
}

// Close releases the engine, if made, and keeps make from making it.
func (m *lateMedia) Close() error {
	if m.closed {
		return nil
	}
	m.closed = true
	for _, u := range m.unsubs {
		u()
	}
	m.unsubs = nil
	if !m.made {
		return nil
	}

	return m.sys.eng.Close()
}

// Activate makes the media and activates the system's session.
func (s lateSession) Activate() error {
	s.m.make()
	if s.m.sys.sess == nil {
		return nil
	}

	return s.m.sys.sess.Activate()
}

// Events returns the topic the session's events reach the player on.
func (s lateSession) Events() *event.Topic[media.SessionEvent] { return &s.m.sessEvents }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// forward hands an event of the engine to the player, with its token
// shifted. It allocates nothing: a tick goes through it.
func (m *lateMedia) forward(ev media.Event) {
	ev.Token += m.offset
	m.events.Publish(ev)
}

// replay sets on eng what waited for it.
func (m *lateMedia) replay(eng media.Engine) {
	if m.set.rate {
		m.logErr("rate", eng.SetRate(m.rate))
	}
	if m.set.vol {
		m.logErr("volume", eng.SetVolume(m.vol))
	}
	if m.set.muted {
		m.logErr("mute", eng.SetMuted(m.muted))
	}
	if b, ok := eng.(media.Backgrounder); ok && m.set.back {
		b.SetBackground(m.back)
	}
	m.set = lateSet{}
}

// logErr logs a setting the engine refused.
func (m *lateMedia) logErr(what string, err error) {
	if err != nil {
		m.logf("dancefitvn: media: %s: %v", what, err)
	}
}
