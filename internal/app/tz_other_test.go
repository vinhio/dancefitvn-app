//go:build !ios && !android

package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Where the system does not tell its zone (the Mac's tests), SetLocalZone
// leaves time.Local alone.
func TestSetLocalZoneLeavesTheZoneWhereTheSystemSaysNothing(t *testing.T) {
	t.Parallel()

	before := time.Local
	SetLocalZone()
	assert.Same(t, before, time.Local)
}
