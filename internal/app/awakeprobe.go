package app

import (
	"dancefitvn-app/internal/sys"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// probedAwake is an awakeSink that logs, after each call, what the system
// holds: the keep-awake probe of a developer's build (local/plans/P7.md
// P7.5c): "dfv: keep-awake <op> asked=<on> idleTimerDisabled=<read>",
// read from UIApplication (sys.IdleTimerDisabled), so that a run on the
// simulator shows the screen saver held while a video plays and let go
// after a pause or Back.
type probedAwake struct {
	inner awakeSink
	read  func() (bool, error)
	logf  func(string, ...any)
}

var _ awakeSink = probedAwake{}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// devAwake returns sink, wrapped by the probe in a developer's build.
func devAwake(sink awakeSink, logf func(string, ...any)) awakeSink {
	if !devBuild {
		return sink
	}

	return probedAwake{inner: sink, read: sys.IdleTimerDisabled, logf: logf}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Set calls the sink, then logs the system's value.
func (p probedAwake) Set(on bool) error {
	err := p.inner.Set(on)
	p.report("set", on, err)

	return err
}

// Reapply calls the sink, then logs the system's value.
func (p probedAwake) Reapply() error {
	err := p.inner.Reapply()
	p.report("reapply", sys.KeepingAwake(), err)

	return err
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// report logs what was asked and what the system holds now.
func (p probedAwake) report(op string, asked bool, err error) {
	held, rerr := p.read()
	switch {
	case err != nil:
		p.logf("dfv: keep-awake %s asked=%t failed: %v", op, asked, err)
	case rerr != nil:
		p.logf("dfv: keep-awake %s asked=%t idleTimerDisabled=? (%v)", op, asked, rerr)
	default:
		p.logf("dfv: keep-awake %s asked=%t idleTimerDisabled=%t", op, asked, held)
	}
}
