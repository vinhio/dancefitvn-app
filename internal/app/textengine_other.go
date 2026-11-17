//go:build !android

package app

import (
	"fmt"

	"github.com/gdeskdev/desk/foundation/core"
)

// useAppTextEngine reports core.ErrUnsupported: Core Text, the engine of
// iOS and tvOS, draws emoji and composes marks itself.
func useAppTextEngine() error {
	return fmt.Errorf("app text engine: %w", core.ErrUnsupported)
}
