package player

import (
	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/media"
	"errors"
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ErrNotEnrolled is the server refusing the address of a lesson the user is
// not enrolled in (403 not_enrolled): the shell goes to 05B (03 §9).
var ErrNotEnrolled = fmt.Errorf("player: not enrolled: %w", core.ErrInvalid)

// codeNotEnrolled is the error code of the API for ErrNotEnrolled.
const codeNotEnrolled = "not_enrolled"

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// resolveError wraps the failure of the Resolver: not_enrolled becomes
// ErrNotEnrolled, the rest is kept as it is.
func resolveError(err error) error {
	var ae *api.Error
	if errors.As(err, &ae) && ae.Code == codeNotEnrolled {
		return fmt.Errorf("%w: %w", ErrNotEnrolled, err)
	}

	return err
}

// retryable reports whether a failure of the engine may be an address that
// expired: the network or the source, not a decode or a missing file.
func retryable(err error) bool {
	return errors.Is(err, media.ErrNetwork) || errors.Is(err, media.ErrSource)
}

// fail ends the load with err: the state Error and the change that tells why.
func (p *Player) fail(err error) {
	if p.closed {
		return
	}
	was := p.state
	p.stopLoad()
	p.loaded, p.want, p.interrupted = false, false, false
	p.err = err
	p.logf("player: lesson %s failed: %v", p.lesson.ID, err)
	if was == StatePlaying {
		p.savePause()
	}
	p.setState(StateError)
	p.changes.Publish(Change{Kind: ChangeError, Err: err})
}
