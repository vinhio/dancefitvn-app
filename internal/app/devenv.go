package app

import (
	"os"
	"strconv"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The variables of a developer's build (devBuild), which the shot script
// sets (scripts/shot.sh).
const (
	// stateEnv names the state to draw at launch: "M02", "M03B" ...
	// (states_mobile.go).
	stateEnv = "DFV_STATE"
	// settleEnv is how long, in milliseconds, the drawn state is let rest
	// after its last request before its layout is written; 1500 when unset.
	settleEnv = "DFV_SETTLE"
	// offlineEnv, "1", makes every request fail as a dead network does.
	offlineEnv = "DFV_OFFLINE"
	// shotLogEnv, "1", also logs each layout as base64 lines ("dfv:L <ID>
	// <i>/<n> <chunk>"), where the cache folder cannot be read (no adb
	// root).
	shotLogEnv = "DFV_SHOT_LOG"
	// engineEnv, "real", keeps the system's engine in the TV's playback
	// states (TV07 to TV10) in place of the fake one, which plays the mock's
	// media from their start (states_tv.go): the keep-awake and the
	// background of local/plans/P7.md P7.5c on the simulator.
	engineEnv = "DFV_ENGINE"
	// frameLogEnv, "1", logs each frame's timing, each key press and a
	// mem line a second on the console (framelog.go); the frames are timed
	// with GDESK_FRAMESTATS=1 as well.
	frameLogEnv = "DFV_FRAMELOG"
	// a11yEnv, "1", dumps the accessibility of the state once its layout
	// is written: <ID>.a11y.json beside it (devaccess.go,
	// scripts/a11y-dump.sh); a11yMoveEnv lists the keys pressed after it,
	// a dump after each ("right,down": <ID>-m1, <ID>-m2), for the TV's
	// focus.
	a11yEnv     = "DFV_A11Y"
	a11yMoveEnv = "DFV_A11Y_MOVE"
	// perfMinEnv is how many minutes PERF-VIDEO10 plays (10 when unset);
	// perfFlingsEnv how many flings PERF-SCROLL makes (20 when unset)
	// (devperf.go, scripts/perf-video.sh, perf-scroll.sh).
	perfMinEnv    = "DFV_PERF_MIN"
	perfFlingsEnv = "DFV_PERF_FLINGS"
	// The downloads' and the network's: DFV_NETWORK (network_mobile.go),
	// DFV_DL_KBPS and DFV_BADSHA (downloads_mobile.go).
)

// defaultSettle is the rest of a state before its shot.
const defaultSettle = 1500 * time.Millisecond

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// devEnv returns the value of a variable of a developer's build; "" in the
// build that ships, which reads none.
func devEnv(key string) string {
	if !devBuild {
		return ""
	}

	return os.Getenv(key)
}

// settleTime is DFV_SETTLE, or defaultSettle when unset or not a number of
// milliseconds.
func settleTime() time.Duration {
	ms, err := strconv.Atoi(devEnv(settleEnv))
	if err != nil || ms < 0 {
		return defaultSettle
	}

	return time.Duration(ms) * time.Millisecond
}
