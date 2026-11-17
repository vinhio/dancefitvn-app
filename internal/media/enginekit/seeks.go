// Adapted from jplayer-mobile/internal/media/enginekit/seeks.go at 49de3c2.

package enginekit

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// SeekCount counts the seeks asked of a player and those it told done, to
// tell whether one is under way (the engine's ReadTime then tells where it
// goes, as the player may still tell the position it left). A player that
// lives across loads (AVPlayer) would count in the next load a completion
// it never told, or told late, of a load gone: each load starts from a
// baseline (Rebase; F5 of M9.R, as amp counts per load). The zero
// SeekCount has none under way. UI thread.
type SeekCount struct {
	asked, done int
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Ask counts a seek asked. A completion of an earlier load told after
// the baseline does not count for it.
func (c *SeekCount) Ask() {
	c.done = min(c.done, c.asked)
	c.asked++
}

// Done counts a completion told.
func (c *SeekCount) Done() { c.done++ }

// Rebase starts the count of a new load: the seeks of the loads before
// it are over, told or not.
func (c *SeekCount) Rebase() { c.done = c.asked }

// Pending tells whether a seek asked is not told done.
func (c *SeekCount) Pending() bool { return c.asked > c.done }
