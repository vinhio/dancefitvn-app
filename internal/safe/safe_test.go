package safe

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGuards checks that a panic inside a guard becomes an error that wraps
// ErrPanic and core.ErrInvalid, that a normal return passes through, and that
// the panic is told with its name.
// (docs/plan/03 section 9: a broken file never ends the app)
func TestGuards(t *testing.T) { //nolint:paralleltest // SetLog is global
	var mu sync.Mutex
	var told []string
	SetLog(func(format string, args ...any) {
		mu.Lock()
		told = append(told, strings.TrimSpace(format))
		mu.Unlock()
	})
	t.Cleanup(func() { SetLog(nil) })

	plain := errors.New("plain")
	tests := []struct {
		name    string
		fn      func() error
		wantErr error
		panics  bool
	}{
		{"ok", func() error { return nil }, nil, false},
		{"error passes", func() error { return plain }, plain, false},
		{"string panic", func() error { panic("boom") }, ErrPanic, true},
		{"nil map write", func() error { var m map[string]int; m["a"] = 1; return nil }, ErrPanic, true},
		{"index out of range", func() error { var s []int; _ = s[len(told)+3]; return nil }, ErrPanic, true},
		{"error panic", func() error { panic(plain) }, ErrPanic, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name := "test." + strings.ReplaceAll(tc.name, " ", "-")
			Expect(name)
			err := Call(name, tc.fn)
			if tc.wantErr == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tc.wantErr)
			if tc.panics {
				assert.ErrorIs(t, err, core.ErrInvalid)
				assert.Contains(t, err.Error(), name)
			}
		})
	}

	Expect("test.value")
	v, err := Value("test.value", func() (int, error) { panic("x") })
	assert.Zero(t, v)
	assert.ErrorIs(t, err, ErrPanic)
	v, err = Value("test.value", func() (int, error) { return 7, nil })
	assert.Equal(t, 7, v)
	assert.NoError(t, err)

	Expect("test.run")
	Run("test.run", func() { panic("in goroutine") })
	mu.Lock()
	defer mu.Unlock()
	assert.NotEmpty(t, told, "the panic was told")
}

// TestUnexpectedPanicGoesOnInTests checks that a guard no test expects does
// not hide a programming error.
func TestUnexpectedPanicGoesOnInTests(t *testing.T) {
	t.Parallel()

	assert.Panics(t, func() { _ = Call("test.unexpected", func() error { panic("bug") }) })
}
