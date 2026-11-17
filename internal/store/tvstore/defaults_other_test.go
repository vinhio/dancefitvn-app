// Adapted from jplayer-tv/internal/store/tvstore/defaults_other_test.go at 781bee5.

//go:build !(ios && tvos)

package tvstore_test

import (
	"errors"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/store/tvstore"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestNewDefaultsIsUnsupportedHere(t *testing.T) {
	t.Parallel()

	d, err := tvstore.NewDefaults(tvstore.Prefix)
	require.Error(t, err)
	assert.Nil(t, d)
	assert.ErrorIs(t, err, core.ErrUnsupported)
	assert.ErrorIs(t, err, errors.ErrUnsupported)
}

func TestMethodsAreUnsupportedHere(t *testing.T) {
	t.Parallel()

	var d tvstore.Defaults
	cases := []struct {
		name string
		call func() error
	}{
		{"load", func() error { _, err := d.Load("settings"); return err }},
		{"save", func() error { return d.Save("settings", []byte("{}")) }},
		{"sync", d.Sync},
		{"load on nil", func() error { _, err := (*tvstore.Defaults)(nil).Load("settings"); return err }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			assert.ErrorIs(t, c.call(), core.ErrUnsupported)
		})
	}
}

func TestPrefixIsTheAppNamespace(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "dancefitvn.v1.", tvstore.Prefix)
}
