// Adapted from jplayer-mobile/internal/app/tz.go at 49de3c2.

package app

import (
	"errors"
	"log"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// SetLocalZone makes time.Local the system's time zone. Go leaves it UTC
// on iOS and Android (time/zoneinfo_ios.go and zoneinfo_android.go:
// initLocal), so that the greeting of Home and the date under it, read in
// the local time as new Date() of the prototype is, came out hours off.
// Call it once, first thing in Run, before any goroutine reads the time.
// A zone the system does not tell leaves time.Local as it is, logged.
func SetLocalZone() {
	loc, err := localZone()
	if err != nil {
		if !isUnsupported(err) {
			log.Printf("dancefitvn: time zone: %v", err)
		}

		return
	}
	time.Local = loc
	log.Printf("dancefitvn: time zone %s", loc)
}

// isUnsupported reports whether err says the system cannot tell the zone.
func isUnsupported(err error) bool { return errors.Is(err, core.ErrUnsupported) }
