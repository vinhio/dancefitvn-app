//go:build !android && (!ios || tvos)

package sys

import (
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestBiometricUnsupported checks the other systems (tvOS, the desktop,
// this Mac) have no biometric sheet, and Biometric hides its button.
func TestBiometricUnsupported(t *testing.T) {
	t.Parallel()

	kind, err := Biometry()
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.Equal(t, BiometryNone, kind)

	cancel, err := Authenticate(func(fn func()) { fn() }, BioPrompt{Reason: "r"}, func(error) {
		t.Error("done of a refused call")
	})
	require.ErrorIs(t, err, core.ErrUnsupported)
	assert.Nil(t, cancel)

	b := NewBiometric(func(fn func()) { fn() })
	assert.Equal(t, BiometryNone, b.Kind())
	assert.False(t, b.Available())
	require.ErrorIs(t, b.Authenticate(t.Context(), BioPrompt{Reason: "r"}), core.ErrUnsupported)
}
