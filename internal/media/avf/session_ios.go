// Adapted from jplayer-mobile/internal/media/avf/session_ios.go at 49de3c2.

//go:build ios && !tvos && cgo

package avf

import (
	"dancefitvn-app/internal/media"
	"fmt"
	"log"
	"unsafe"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SessionStats are a session's counters, for the device tests.
type SessionStats struct {
	// Activations is how many times Activate made the session active.
	Activations int
	// Active tells that the session is active now.
	Active bool
	// Resets counts the resets of the media services the session heard;
	// Suspensions the interruptions it dropped as the app's suspension.
	Resets, Suspensions int
}

// session is the media.Session of iOS: AVAudioSession, category playback
// (04 §2.3). It lives as long as the process: its observer is never
// removed.
type session struct {
	cfg    media.Config
	events event.Topic[media.SessionEvent]

	shared   objc.ID // [AVAudioSession sharedInstance], not owned
	observer objc.ID // owned, for the life of the process

	active      bool // UI thread
	activations int
	// resets counts the media services' resets (UI thread): an engine
	// whose count is behind makes a new AVPlayer at its next Load.
	resets, suspensions int
	// onResetFn and onSuspendedFn are onReset and onSuspended, made once.
	onResetFn, onSuspendedFn func()
	// nsErr is the out-parameter of the session's calls: a field of a heap
	// object, as objc.Call wants its pointers.
	nsErr objc.ID
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSession returns the audio session, its category set to playback, mode
// default, once (setting it does not stop other apps' sound; Activate does).
// It hears interruptions, route changes and the resets of the media
// services. Any goroutine may call it (the app's warmSession makes it off
// the UI thread, as it takes ~60 ms on the iPhone): initAVF runs once
// (sync.OnceValue), objc.WithPool pins the goroutine to its thread while
// its pool lives, the observer's registry has its lock, AVAudioSession and
// NSNotificationCenter are thread-safe, and the notifications the
// observer hears reach the session on the UI thread (cfg.Dispatch). The
// session's methods are the UI thread's.
func NewSession(cfg media.Config) (media.Session, error) {
	if cfg.Dispatch == nil {
		return nil, fmt.Errorf("avf: session config without dispatch: %w", core.ErrInvalid)
	}
	if err := initAVF(); err != nil {
		return nil, err
	}
	s := &session{cfg: cfg}
	s.onResetFn, s.onSuspendedFn = s.onReset, s.onSuspended
	var err error
	objc.WithPool(func() {
		s.shared = objc.Call(av.cls.session, av.sel.sharedInstance)
		if s.shared == 0 {
			err = fmt.Errorf("avf: no audio session: %w", core.ErrNotFound)

			return
		}
		if err = s.setCategory(); err != nil {
			return
		}
		s.observer = objc.New(av.cls.sessionObserver)
		watchSession(s.observer, s)
		c := center()
		objc.Call(c, av.sel.addObserver, uintptr(s.observer), uintptr(av.sel.interrupted), uintptr(av.interruption), 0)
		objc.Call(c, av.sel.addObserver, uintptr(s.observer), uintptr(av.sel.routeChanged), uintptr(av.routeChange), 0)
		objc.Call(c, av.sel.addObserver, uintptr(s.observer), uintptr(av.sel.mediaServicesReset), uintptr(av.mediaReset), 0)
	})
	if err != nil {
		return nil, err
	}

	return s, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Activate makes the session active, which stops other apps' sound, unless
// it is already; the engine calls it before it plays. An interruption makes
// it inactive again. UI thread.
func (s *session) Activate() error {
	if s.active {
		return nil
	}
	var err error
	objc.WithPool(func() {
		s.nsErr = 0
		if !objc.CallBool(s.shared, av.sel.setActive, 1, uintptr(unsafe.Pointer(&s.nsErr))) { //nolint:gosec // G103: a field of a heap object
			d := describe(s.nsErr)
			err = fmt.Errorf("avf: activate the audio session: %s %d: %w", d.domain, d.code, core.ErrClosed)
		}
	})
	if err != nil {
		return err
	}
	s.active = true
	s.activations++

	return nil
}

// Events returns the topic the session publishes to, on the UI thread.
func (s *session) Events() *event.Topic[media.SessionEvent] { return &s.events }

// SessionStatsOf returns the counters of s, a session of this package
// (false for another).
func SessionStatsOf(s media.Session) (SessionStats, bool) {
	ss, ok := s.(*session)
	if !ok {
		return SessionStats{}, false
	}

	return SessionStats{
		Activations: ss.activations, Active: ss.active,
		Resets: ss.resets, Suspensions: ss.suspensions,
	}, true
}

// resetsOf returns how many resets of the media services sess heard, 0
// for a session of another package (the engine then never renews its
// player).
func resetsOf(sess media.Session) int {
	if s, ok := sess.(*session); ok {
		return s.resets
	}

	return 0
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onEvent publishes ev, on the UI thread: an interruption that begins makes
// the session inactive (04 §2.3).
func (s *session) onEvent(ev media.SessionEvent) {
	if ev.Kind == media.InterruptBegin {
		s.active = false
	}
	s.events.Publish(ev)
}

// onReset follows a reset of the media services, on the UI thread: the
// session is set up again (its category, as Apple's QA1749 asks) and is
// inactive until the next Activate; the count tells the engines to make a
// new player; then MediaReset has the player load its media again.
func (s *session) onReset() {
	s.active = false
	s.resets++
	var err error
	objc.WithPool(func() { err = s.setCategory() })
	if err != nil {
		log.Printf("dancefitvn: %v", err)
	}
	s.events.Publish(media.SessionEvent{Kind: media.MediaReset})
}

// onSuspended follows an interruption that only tells that the system
// suspended the app (M4.5 D5): the session is inactive, the player is not
// told. UI thread.
func (s *session) onSuspended() {
	s.active = false
	s.suspensions++
}

// setCategory sets the session's category to playback, mode default. In a
// pool.
func (s *session) setCategory() error {
	s.nsErr = 0
	if !objc.CallBool(s.shared, av.sel.setCategory, uintptr(av.category), uintptr(av.mode), 0,
		uintptr(unsafe.Pointer(&s.nsErr))) { //nolint:gosec // G103: a field of a heap object
		d := describe(s.nsErr)

		return fmt.Errorf("avf: audio session category: %s %d: %w", d.domain, d.code, core.ErrUnsupported)
	}

	return nil
}
