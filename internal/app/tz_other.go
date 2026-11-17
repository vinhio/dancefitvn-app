//go:build !ios && !android

package app

import (
	"fmt"
	"time"

	"github.com/gdeskdev/desk/foundation/core"
)

// localZone reports core.ErrUnsupported: on this system Go reads the zone
// itself (the Mac's tests run here).
func localZone() (*time.Location, error) {
	return nil, fmt.Errorf("local zone: %w", core.ErrUnsupported)
}
