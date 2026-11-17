//go:build !ios

package sys

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDeviceNameFallsBack: without UIKit the name is "Apple TV".
func TestDeviceNameFallsBack(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Apple TV", DeviceName())
}
