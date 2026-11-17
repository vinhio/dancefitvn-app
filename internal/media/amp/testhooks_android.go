// Adapted from jplayer-mobile/internal/media/amp/testhooks_android.go at 49de3c2.
// PlaybackParams is DanceFITVN's own (P0.7: the pitch kept at a speed).

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/enginekit"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Position reads the position of the player of e, an engine of this
// package, in seconds, as its tick does; false for another engine or a
// player that has none to tell. For device tests only (the time while
// the background stops the tick); UI thread.
func Position(m media.Engine) (float64, bool) {
	e, ok := m.(*engine)
	if !ok {
		return 0, false
	}
	l := e.cur
	if l == nil || !l.prepared || l.mp == 0 || !l.state.Allows(enginekit.CallGetCurrentPosition) {
		return 0, false
	}
	if err := jni.Do(e.tickDo); err != nil {
		return 0, false
	}

	return float64(e.pos) / 1000, true
}

// PlaybackParams reads the speed and the pitch the player of e, an engine
// of this package, plays at now (MediaPlayer.getPlaybackParams); false for
// another engine or a player not prepared. For device tests and the
// spike only (the pitch kept at 0.5 and 1.5, docs/plan/04-media-engine.md
// §3.2); UI thread. It looks its methods up at each call.
func PlaybackParams(m media.Engine) (speed, pitch float32, ok bool) {
	e, isAMP := m.(*engine)
	if !isAMP {
		return 0, 0, false
	}
	l := e.cur
	if l == nil || !l.prepared || l.mp == 0 || l.state == enginekit.MPError || l.state == enginekit.MPEnd {
		return 0, 0, false
	}
	err := jni.Do(func(env jni.Env) error {
		get, err := env.Method(ids.player, "getPlaybackParams", "()Landroid/media/PlaybackParams;")
		if err != nil {
			return err
		}
		getSpeed, err := env.Method(ids.params, "getSpeed", "()F")
		if err != nil {
			return err
		}
		getPitch, err := env.Method(ids.params, "getPitch", "()F")
		if err != nil {
			return err
		}
		p, err := env.CallObject(l.mp, get)
		if err != nil {
			return err
		}
		defer env.DeleteLocalRef(p)
		if speed, err = env.CallFloat(p, getSpeed); err != nil {
			return err
		}
		pitch, err = env.CallFloat(p, getPitch)

		return err
	})

	return speed, pitch, err == nil
}
