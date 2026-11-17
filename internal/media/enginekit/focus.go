// Adapted from jplayer-mobile/internal/media/enginekit/focus.go at 49de3c2.

package enginekit

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The changes of the audio focus an OnAudioFocusChangeListener is told
// (android.media.AudioManager's AUDIOFOCUS_* values).
const (
	FocusGain          int32 = 1  // AUDIOFOCUS_GAIN
	FocusLoss          int32 = -1 // AUDIOFOCUS_LOSS
	FocusLossTransient int32 = -2 // AUDIOFOCUS_LOSS_TRANSIENT
	FocusLossDuck      int32 = -3 // AUDIOFOCUS_LOSS_TRANSIENT_CAN_DUCK
)

// Focus follows the app's audio focus on Android (04 §3.3, D14 of M4): what
// the session's requests and the system's changes make of it, and what the
// session tells the player. The zero Focus is not held. UI thread.
type Focus struct {
	held      bool // the app was granted the focus and did not lose it for good
	transient bool // lost for a while: InterruptBegin was told
}

// FocusNews is what a change of the focus tells the player: an
// InterruptBegin, an InterruptEnd, and whether that end resumes.
type FocusNews struct {
	Begin, End, Resume bool
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Wanted tells that Activate must request the focus: the app does not hold
// it, or lost it for a while (another app's short sound, a call), when
// the user's play takes it back rather than waiting for the system's gain.
func (f *Focus) Wanted() bool { return !f.held || f.transient }

// Granted records a request the system granted: the app holds the focus,
// and a transient loss is over, so the system's gain to come, if any,
// resumes nothing (the user played already).
func (f *Focus) Granted() { f.held, f.transient = true, false }

// Abandoned records that the app gave the focus back.
func (f *Focus) Abandoned() { f.held, f.transient = false, false }

// Held tells that the app holds the focus, lost for a while or not.
func (f *Focus) Held() bool { return f.held }

// Change records a change the listener was told and returns its news: a
// loss for a while is InterruptBegin, and its gain InterruptEnd that
// resumes; a loss for good is InterruptBegin then InterruptEnd that does
// not; CAN_DUCK is the system's to handle (it lowers the volume itself,
// Android 8+), and so is any other value.
func (f *Focus) Change(c int32) FocusNews {
	switch c {
	case FocusLossTransient:
		f.transient = true

		return FocusNews{Begin: true}
	case FocusLoss:
		f.held, f.transient = false, false

		return FocusNews{Begin: true, End: true}
	case FocusGain:
		f.held = true
		if f.transient {
			f.transient = false

			return FocusNews{End: true, Resume: true}
		}

		return FocusNews{}
	case FocusLossDuck:
		return FocusNews{}
	default:
		return FocusNews{}
	}
}
