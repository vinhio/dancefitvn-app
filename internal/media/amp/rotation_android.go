// Adapted from jplayer-mobile/internal/media/amp/probe_android.go at 49de3c2.
// Only the rotation read is kept: the prober is dropped (docs/plan/04-media-engine.md §1).

//go:build android && cgo

package amp

import (
	"dancefitvn-app/internal/media/enginekit"
	"strconv"
	"strings"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// readRotation reads, off the UI thread, how far the frames of l's video
// turn to stand upright (METADATA_KEY_VIDEO_ROTATION, clockwise: the
// decoder hands them unrotated), and sets it on the Surface while l is
// the current load. The goroutine ends as the retriever returns; Close
// does not wait for it, and its result then finds no load.
func (e *engine) readRotation(l *load) {
	rawURL, local := l.src.URL, l.local
	go func() {
		deg := 0
		err := jni.Do(func(env jni.Env) error {
			m, err := openRetriever(env, rawURL, local)
			if err != nil {
				return err
			}
			defer closeRetriever(env, m)
			deg = metaInt(env, m, keyVideoRotation)

			return nil
		})
		if err != nil || deg <= 0 {
			return
		}
		e.cfg.Dispatch(func() {
			if e.cur == l {
				e.video.SetRotation(deg)
			}
		})
	}()
}

// openRetriever returns, as a local reference, a MediaMetadataRetriever
// set to a file, a content:// document (with the app's Context) or a
// link.
func openRetriever(env jni.Env, rawURL string, local bool) (jni.Object, error) {
	m, err := env.NewObject(ids.mmr, ids.mmrNew)
	if err != nil {
		return 0, err
	}
	if enginekit.IsContent(rawURL) {
		uri, err := parseURI(env, rawURL)
		if err == nil {
			err = env.CallVoid(m, ids.mmrSetURI, jni.Ref(jni.Context()), jni.Ref(uri))
			env.DeleteLocalRef(uri)
		}
		if err != nil {
			closeRetriever(env, m)

			return 0, err
		}

		return m, nil
	}
	s, err := env.NewString(rawURL)
	if err != nil {
		env.DeleteLocalRef(m)

		return 0, err
	}
	defer env.DeleteLocalRef(s)
	if local {
		err = env.CallVoid(m, ids.mmrSetPath, jni.Ref(s))
	} else {
		var headers jni.Object
		if headers, err = env.NewObject(ids.hashMap, ids.hashMapNew); err == nil {
			err = env.CallVoid(m, ids.mmrSetURL, jni.Ref(s), jni.Ref(headers))
			env.DeleteLocalRef(headers)
		}
	}
	if err != nil {
		closeRetriever(env, m)

		return 0, err
	}

	return m, nil
}

// closeRetriever releases a retriever and its local reference.
func closeRetriever(env jni.Env, m jni.Object) {
	if err := env.CallVoid(m, ids.mmrRelease); err != nil {
		logf("amp: MediaMetadataRetriever.release: %v", err)
	}
	env.DeleteLocalRef(m)
}

// metaString returns the metadata of key, "" when there is none.
func metaString(env jni.Env, m jni.Object, key int32) string {
	o, err := env.CallObject(m, ids.mmrExtract, jni.Int(key))
	if err != nil || o == 0 {
		return ""
	}
	defer env.DeleteLocalRef(o)
	s, _ := env.GoString(o)

	return s
}

// metaInt returns the metadata of key as a number, 0 when there is none.
func metaInt(env jni.Env, m jni.Object, key int32) int {
	n, err := strconv.Atoi(strings.TrimSpace(metaString(env, m, key)))
	if err != nil {
		return 0
	}

	return n
}
