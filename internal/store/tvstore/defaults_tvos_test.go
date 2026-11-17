// Adapted from jplayer-tv/internal/store/tvstore/defaults_tvos_test.go at 781bee5.

//go:build ios && tvos

package tvstore_test

import (
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/ffi/objc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/store/tvstore"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestDefaultsRoundTrip(t *testing.T) {
	t.Parallel()

	d, prefix := newDefaults(t)
	cases := []struct {
		name string
		data []byte
	}{
		{"json", []byte(`{"seq":1,"data":{}}`)},
		{"nul and non-utf8", []byte{'a', 0, 0xff, 'b'}},
		{"vietnamese", []byte(`"Băng từ"`)},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			key := "k" + strconv.Itoa(i)
			t.Cleanup(func() { remove(prefix + key) })
			require.NoError(t, d.Save(key, c.data))
			got, err := d.Load(key)
			require.NoError(t, err)
			assert.Equal(t, c.data, got)
		})
	}
}

func TestDefaultsMissingIsNotFound(t *testing.T) {
	t.Parallel()

	d, _ := newDefaults(t)
	_, err := d.Load("absent")
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestDefaultsForeignValueIsInvalid(t *testing.T) {
	t.Parallel()

	d, prefix := newDefaults(t)
	t.Cleanup(func() { remove(prefix + "text") })
	objc.WithPool(func() {
		k, v := objc.NSString(prefix+"text"), objc.NSString("not data")
		defer objc.Release(k)
		defer objc.Release(v)
		objc.Call(standard(), objc.Sel("setObject:forKey:"), uintptr(v), uintptr(k))
	})
	_, err := d.Load("text")
	require.ErrorIs(t, err, core.ErrInvalid)
	assert.Contains(t, err.Error(), "String")
}

func TestDefaultsRejectsEmptyData(t *testing.T) {
	t.Parallel()

	d, _ := newDefaults(t)
	for _, data := range [][]byte{nil, {}} {
		assert.ErrorIs(t, d.Save("empty", data), core.ErrInvalid)
	}
	_, err := d.Load("empty")
	assert.ErrorIs(t, err, core.ErrNotFound)
}

func TestDefaultsOverwriteAndSync(t *testing.T) {
	t.Parallel()

	d, prefix := newDefaults(t)
	t.Cleanup(func() { remove(prefix + "over") })
	require.NoError(t, d.Save("over", []byte("1")))
	require.NoError(t, d.Save("over", []byte("22")))
	require.NoError(t, d.Sync())
	got, err := d.Load("over")
	require.NoError(t, err)
	assert.Equal(t, []byte("22"), got)
}

func TestDefaultsNilIsInvalid(t *testing.T) {
	t.Parallel()

	var d *tvstore.Defaults
	_, err := d.Load("x")
	require.ErrorIs(t, err, core.ErrInvalid)
	require.ErrorIs(t, d.Save("x", []byte("1")), core.ErrInvalid)
	require.ErrorIs(t, d.Sync(), core.ErrInvalid)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

var prefixes atomic.Int64

// newDefaults returns a Defaults under a prefix of its own.
func newDefaults(t *testing.T) (*tvstore.Defaults, string) {
	t.Helper()

	prefix := "dancefitvn.tvstore.test." + strconv.FormatInt(prefixes.Add(1), 10) + "."
	d, err := tvstore.NewDefaults(prefix)
	require.NoError(t, err)

	return d, prefix
}

func standard() objc.ID {
	return objc.Call(objc.ID(objc.GetClass("NSUserDefaults")), objc.Sel("standardUserDefaults"))
}

// remove deletes key from NSUserDefaults.
func remove(key string) {
	objc.WithPool(func() {
		k := objc.NSString(key)
		defer objc.Release(k)
		objc.Call(standard(), objc.Sel("removeObjectForKey:"), uintptr(k))
	})
}
