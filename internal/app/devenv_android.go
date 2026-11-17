//go:build android

// Adapted from jplayer-mobile/internal/app/env_android.go at 49de3c2 (the
// pattern: set the environment variables a developer's script stands for,
// am start passing no environment to an app). jPlayer read system
// properties with cgo; here they are the launch intent's string extras,
// read with JNI (jni.Intent), so no cgo enters the app package.

package app

import (
	"log"
	"os"

	"github.com/gdeskdev/desk/bridge/backend/android/jni"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// devEnvKeys are the variables of a developer's build (devenv.go) that
// applyLaunchEnv reads from the launch intent's extras, and gDesk's frame
// timing (ui.FrameStatsEnv, read as the view is made, after them): the
// frame log and the performance states of scripts/perf-*.sh. They live in
// this file, their only reader, so that a lint pass over another OS does
// not count them unused.
var devEnvKeys = [...]string{
	stateEnv, settleEnv, offlineEnv, shotLogEnv, networkEnv, dlKBpsEnv, badSHAEnv, apiEnv, liveRoutesEnv,
	frameLogEnv, perfMinEnv, perfFlingsEnv, pairCodeEnv, ui.FrameStatsEnv,
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// applyLaunchEnv sets, in a developer's build, the variables of
// devEnvKeys from the string extras of the intent that started the
// activity (am start --es DFV_STATE M02), each one the environment does
// not hold already. The build that ships reads nothing.
func applyLaunchEnv() {
	if !devBuild {
		return
	}
	err := jni.Do(func(env jni.Env) error {
		intent, err := jni.Intent(env)
		if err != nil || intent == 0 {
			return err
		}
		defer env.DeleteLocalRef(intent)
		c, err := env.FindClass("android/content/Intent")
		if err != nil {
			return err
		}
		defer env.DeleteLocalRef(jni.Object(c))
		get, err := env.Method(c, "getStringExtra", "(Ljava/lang/String;)Ljava/lang/String;")
		if err != nil {
			return err
		}
		for _, key := range devEnvKeys {
			if _, set := os.LookupEnv(key); set {
				continue
			}
			if v := stringExtra(env, intent, get, key); v != "" {
				_ = os.Setenv(key, v)
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("dancefitvn: launch extras: %v", err)
	}
}

// stringExtra returns the string extra key of intent, "" when there is
// none; its local references are deleted at once (one per key).
func stringExtra(env jni.Env, intent jni.Object, get jni.MethodID, key string) string {
	k, err := env.NewString(key)
	if err != nil {
		return ""
	}
	defer env.DeleteLocalRef(k)
	o, err := env.CallObject(intent, get, jni.Ref(k))
	if err != nil || o == 0 {
		return ""
	}
	defer env.DeleteLocalRef(o)
	s, _ := env.GoString(o)

	return s
}
