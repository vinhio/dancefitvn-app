// Adapted from jplayer-mobile/internal/app/media.go at 49de3c2
// (platformMedia, newMediaFunc, makeMedia, processSession). The prober,
// the store and the live parts are dropped; warmSession is new: it makes
// the process's audio session off the UI thread (avf.NewSession takes
// ~60 ms on the iPhone, local/p4/engine-ios.md).

package app

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// platformMedia is what a system gives to play with: its engine and its
// audio session (nil when it has none).
type platformMedia struct {
	name string // "avf", "amp" or "fake", for the log
	eng  media.Engine
	sess media.Session
}

// newMediaFunc makes the media of a system from its config;
// newPlatformMedia is the system's, the tests and the shot states pass
// the fake.
type newMediaFunc func(cfg media.Config) (platformMedia, error)

// The audio session of the process: iOS's observers of the session are
// never removed, and the system has one session per app, so the first
// window's stays for the life of the process. warming is the channel a
// warm-up under way closes when it ends.
var proc struct {
	mu      sync.Mutex
	sess    media.Session
	warming chan struct{}
	err     error
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// makeMedia makes the system's playback with newMedia, or the fake engine
// when it fails, which is logged unless the system has none, with the
// session it made.
func makeMedia(cfg media.Config, newMedia newMediaFunc, logf func(string, ...any)) platformMedia {
	pm, err := newMedia(cfg)
	if err != nil {
		if !errors.Is(err, core.ErrUnsupported) {
			logf("dancefitvn: media: %v; playing on the fake engine", err)
		}
		pm = platformMedia{name: "fake", eng: fake.New(cfg, fake.Options{}), sess: pm.sess}
	}

	return pm
}

// fakeMediaOf returns the media of the fake engine with o: a shot state's
// or a test's (wireKnobs.media, mobileDeps.NewMedia, tvKnobs.media).
func fakeMediaOf(o fake.Options) newMediaFunc {
	return func(cfg media.Config) (platformMedia, error) {
		return platformMedia{name: "fake", eng: fake.New(cfg, o), sess: &fake.Session{}}, nil
	}
}

// warmSession starts making the session of the process with newSession
// on a goroutine, unless it is made or under way; ready, when set, runs
// on the UI thread (cfg.Dispatch) once it is done, made or failed. The
// session keeps cfg: its events reach the UI thread through cfg.Dispatch.
// logf logs how long it took, on the UI thread too, as a Logf of the deps
// may not be safe off it.
func warmSession(cfg media.Config, newSession func(media.Config) (media.Session, error), ready func(), logf func(string, ...any)) {
	proc.mu.Lock()
	if proc.sess != nil || proc.warming != nil {
		proc.mu.Unlock()
		if ready != nil {
			cfg.Dispatch(ready)
		}

		return
	}
	done := make(chan struct{})
	proc.warming = done
	proc.mu.Unlock()
	go func() {
		t0 := time.Now()
		s, err := newSession(cfg)
		took := time.Since(t0)
		proc.mu.Lock()
		proc.sess, proc.err, proc.warming = s, err, nil
		proc.mu.Unlock()
		close(done)
		cfg.Dispatch(func() {
			logf("dancefitvn: media: audio session made off the UI thread in %.1f ms", ms(took))
			if ready != nil {
				ready()
			}
		})
	}()
}

// sessionWarm reports whether no warm-up of the session is under way:
// processSession would not wait.
func sessionWarm() bool {
	proc.mu.Lock()
	defer proc.mu.Unlock()

	return proc.warming == nil
}

// processSession returns the session of the process: the one made, or
// warmSession's, waited for when under way, or one made now by newSession
// with cfg. A failure is not kept: the next call tries again. logf logs
// a wait; call it on the UI thread.
func processSession(cfg media.Config, newSession func(media.Config) (media.Session, error), logf func(string, ...any)) (media.Session, error) {
	proc.mu.Lock()
	if w := proc.warming; w != nil {
		proc.mu.Unlock()
		t0 := time.Now()
		<-w
		logf("dancefitvn: media: waited %.1f ms for the audio session", ms(time.Since(t0)))
		proc.mu.Lock()
	}
	defer proc.mu.Unlock()
	if proc.sess != nil {
		return proc.sess, nil
	}
	if proc.err != nil {
		err := proc.err
		proc.err = nil

		return nil, err
	}
	s, err := newSession(cfg)
	if err != nil {
		return nil, err
	}
	proc.sess = s

	return s, nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// maxResolution is the largest picture of quality q, in pixels, for the
// HLS variants of the lessons (portrait 9:16: 1080×1920, 720×1280,
// 480×854, mock lessons/stream.json); 0×0 is no limit (auto).
func maxResolution(q store.Quality) (w, h int) {
	switch q {
	case store.QualityP1080:
		return 1080, 1920
	case store.QualityP720:
		return 720, 1280
	case store.QualityP480:
		return 480, 854
	case store.QualityUnknown, store.QualityAuto:
		return 0, 0
	default:
		return 0, 0
	}
}

// errNoEngine is what newPlatformMedia answers where the system has no
// engine: the fake plays instead.
var errNoEngine = fmt.Errorf("app: media engine: %w", core.ErrUnsupported)
