// Adapted from jplayer-mobile/internal/media/fake/session.go at 49de3c2.

package fake

import (
	"dancefitvn-app/internal/media"

	"github.com/gdeskdev/desk/foundation/core/event"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Session is a media.Session a test drives with Emit.
type Session struct {
	// Err is what Activate returns.
	Err error
	// Activations counts the calls of Activate.
	Activations int

	events event.Topic[media.SessionEvent]
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Activate counts the call and returns Err.
func (s *Session) Activate() error {
	s.Activations++

	return s.Err
}

// Events returns the topic the session publishes to.
func (s *Session) Events() *event.Topic[media.SessionEvent] { return &s.events }

// Emit publishes ev, as the system would.
func (s *Session) Emit(ev media.SessionEvent) { s.events.Publish(ev) }
