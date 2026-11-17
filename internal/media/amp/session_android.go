// Adapted from jplayer-mobile/internal/media/amp/session_android.go at 49de3c2.

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"
	"fmt"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// NoisyActions are the broadcasts the session takes for headphones
// pulled out (media.Noisy): the system's AUDIO_BECOMING_NOISY. The system
// alone may send that one, so a device test adds an action of its own
// before NewSession, and sends it to the app.
var NoisyActions = []string{jni.ActionAudioBecomingNoisy}

// errFocusRefused is a request of the audio focus the system turned down
// (a call is on): the engine plays on, as the prototype ignores a play()
// refused.
var errFocusRefused = fmt.Errorf("amp: audio focus refused: %w", core.ErrClosed)

// session is the media.Session on AudioManager's audio focus and the
// AUDIO_BECOMING_NOISY broadcast. UI thread, but for the listeners, which
// hand their news to it.
type session struct {
	cfg     media.Config
	events  event.Topic[media.SessionEvent]
	audio   jni.Object // AudioManager, global
	request jni.Object // AudioFocusRequest, global
	focus   *jni.Proxy
	noisy   *jni.Receiver
	noisyFn func() // made once

	audioFocus enginekit.Focus     // the focus held, lost for a while or for good
	requests   int                 // requestAudioFocus calls (SessionStatsOf)
	requestFn  func(jni.Env) error // made once: requestFocus
	granted    int32               // what requestFocus was answered
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewSession returns the app's audio session: a request of the focus for
// media (AUDIOFOCUS_GAIN, music), and the receiver of NoisyActions on the
// application context, so it outlives the activity. The system lowers
// the volume by itself for a short sound (CAN_DUCK, Android 8+: Q2), so
// the session reports no Duck. cfg.Dispatch is required. It lives as long
// as the process, which Close ends for tests.
func NewSession(cfg media.Config) (media.Session, error) {
	if cfg.Dispatch == nil {
		return nil, fmt.Errorf("amp: new session: no dispatch: %w", core.ErrInvalid)
	}
	s := &session{cfg: cfg}
	s.noisyFn = func() { s.events.Publish(media.SessionEvent{Kind: media.Noisy}) }
	s.requestFn = s.requestFocus
	err := jni.Do(func(env jni.Env) error {
		if err := lookup(env); err != nil {
			return err
		}
		ctx := jni.Context()
		if ctx == 0 {
			return fmt.Errorf("no application context: %w", core.ErrUnsupported)
		}
		name, err := env.NewString("audio") // Context.AUDIO_SERVICE
		if err != nil {
			return err
		}
		am, err := env.CallObject(ctx, ids.getSystemService, jni.Ref(name))
		env.DeleteLocalRef(name)
		if err != nil {
			return err
		}
		s.audio = newGlobal(env, am)
		env.DeleteLocalRef(am)
		if s.focus, err = jni.NewProxy(env, []string{"android/media/AudioManager$OnAudioFocusChangeListener"},
			[]string{"onAudioFocusChange"}, s.onChange); err != nil {
			return err
		}
		liveProxies.Add(1)
		if s.request, err = focusRequest(env, s.focus.Object()); err != nil {
			return err
		}
		s.noisy, err = jni.RegisterReceiver(env, NoisyActions, s.onNoisy)

		return err
	})
	if err != nil {
		_ = s.Close()

		return nil, fmt.Errorf("amp: new session: %w", err)
	}

	return s, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Activate takes the audio focus, unless the app holds it: after a loss
// for a while (LOSS_TRANSIENT) it asks again, since the user's play must
// not wait for the system's gain (M4.5 D6). A request refused (a call is
// on) is errFocusRefused; the engine plays on.
func (s *session) Activate() error {
	if !s.audioFocus.Wanted() {
		return nil
	}
	err := jni.Do(s.requestFn)
	s.requests++
	if err != nil {
		return fmt.Errorf("amp: request audio focus: %w", err)
	}
	if s.granted != focusGranted {
		return errFocusRefused
	}
	s.audioFocus.Granted()

	return nil
}

// Events returns the topic the session publishes to, on the UI thread.
func (s *session) Events() *event.Topic[media.SessionEvent] { return &s.events }

// Close gives the focus back and drops the listeners. The app keeps its
// session for the life of the process; tests close theirs.
func (s *session) Close() error {
	if s.noisy != nil {
		_ = s.noisy.Close()
		s.noisy = nil
	}
	return jni.Do(func(env jni.Env) error {
		if s.request != 0 && s.audio != 0 && s.audioFocus.Held() {
			_, _ = env.CallInt(s.audio, ids.abandonFocus, jni.Ref(s.request))
		}
		s.audioFocus.Abandoned()
		deleteGlobal(env, s.request)
		deleteGlobal(env, s.audio)
		s.request, s.audio = 0, 0
		if s.focus != nil {
			_ = s.focus.Close()
			s.focus = nil
			liveProxies.Add(-1)
		}

		return nil
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onChange is the focus listener, on Java's main thread.
func (s *session) onChange(_ jni.Env, _ int, args jni.Args) jni.Value {
	change := args.Int(0)
	s.cfg.Dispatch(func() { s.changed(change) })

	return 0
}

// onNoisy is the receiver of NoisyActions, on Java's main thread.
func (s *session) onNoisy(jni.Env, jni.Object) { s.cfg.Dispatch(s.noisyFn) }

// requestFocus is Activate's call into Java: the request of the focus, its
// answer in s.granted.
func (s *session) requestFocus(env jni.Env) error {
	var err error
	s.granted, err = env.CallInt(s.audio, ids.requestFocus, jni.Ref(s.request))

	return err
}

// changed follows the focus (D14, enginekit.Focus): a loss for a while is
// InterruptBegin and its gain InterruptEnd that resumes; a loss for good is
// InterruptBegin then InterruptEnd that does not; CAN_DUCK is the system's
// to handle. UI thread.
func (s *session) changed(change int32) {
	news := s.audioFocus.Change(change)
	if news.Begin {
		s.events.Publish(media.SessionEvent{Kind: media.InterruptBegin})
	}
	if news.End {
		s.events.Publish(media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: news.Resume})
	}
}

// focusRequest returns, as a global reference, the request of the focus
// for media with listener.
func focusRequest(env jni.Env, listener jni.Object) (jni.Object, error) {
	attrs, err := audioAttributes(env, contentMusic)
	if err != nil {
		return 0, err
	}
	defer deleteGlobal(env, attrs)
	b, err := env.NewObject(ids.focusBuilder, ids.focusBuilderNew, jni.Int(enginekit.FocusGain))
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(b)
	for _, c := range []struct {
		m   jni.MethodID
		arg jni.Object
	}{{ids.focusAttrs, attrs}, {ids.focusListener, listener}} {
		r, err := env.CallObject(b, c.m, jni.Ref(c.arg))
		if err != nil {
			return 0, err
		}
		env.DeleteLocalRef(r)
	}
	r, err := env.CallObject(b, ids.focusBuild)
	if err != nil {
		return 0, err
	}
	defer env.DeleteLocalRef(r)

	return newGlobal(env, r), nil
}
