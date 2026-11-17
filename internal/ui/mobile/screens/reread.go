//go:build !tvos

package screens

import (
	"time"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// rereadDelays are the waits before a screen whose first read failed reads again by
// itself (after the "Thử lại" of Q-P3-18 is on show): the network that the system
// cut while the app was away is back within seconds, and the read made again then
// shows the content with nobody tapping. A third failure leaves the button alone.
var rereadDelays = [...]time.Duration{2 * time.Second, 6 * time.Second}

// reread schedules the automatic reads of a screen's first load.
type reread struct {
	tries int
	timer ui.Timer
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// schedule arranges the next automatic read, run on the UI thread after the wait of
// the try; it does nothing once the tries are used up.
func (r *reread) schedule(c *ui.Context, run func()) {
	r.timer.Stop()
	if c == nil || r.tries >= len(rereadDelays) {
		return
	}
	d := rereadDelays[r.tries]
	r.tries++
	r.timer = c.AfterFunc(d, run)
}

// reset forgets the tries and the wait: a read that was asked for (the button, the
// network back, the app back in front) starts the count again.
func (r *reread) reset() {
	r.tries = 0
	r.timer.Stop()
}

// stop ends the wait, as leaving the screen does.
func (r *reread) stop() { r.timer.Stop() }
