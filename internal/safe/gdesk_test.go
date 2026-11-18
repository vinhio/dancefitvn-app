package safe

import (
	"strings"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandlePanic checks the app's handler of gDesk's panics: an expected
// entry point is logged with its value and stack and the app goes on; any
// other panics again in a test binary. SetLog is global, so it does not run
// in parallel.
func TestHandlePanic(t *testing.T) { //nolint:paralleltest // SetLog is global
	var mu sync.Mutex
	var told []string
	SetLog(func(_ string, args ...any) {
		mu.Lock()
		defer mu.Unlock()
		for _, a := range args {
			if s, ok := a.(string); ok {
				told = append(told, s)
			}
		}
	})
	t.Cleanup(func() { SetLog(nil) })

	Expect("test.gdesk")
	assert.NotPanics(t, func() { HandlePanic(&core.Panic{Where: "test.gdesk", Value: "boom", Stack: []byte("the stack")}) })
	mu.Lock()
	require.Len(t, told, 1)
	assert.True(t, strings.HasPrefix(told[0], "dancefitvn: panic in test.gdesk: boom\n"), told[0])
	assert.Contains(t, told[0], "the stack")
	mu.Unlock()

	unexpected := &core.Panic{Where: "test.gdesk-unexpected", Value: "bug"}
	assert.PanicsWithValue(t, unexpected, func() { HandlePanic(unexpected) })
}

// TestInstall checks that Install routes the panics gDesk recovers through
// HandlePanic: core.Recovered then logs an expected one and returns an error
// matching core.ErrPanic. It swaps the process's panic handler and SetLog,
// so it does not run in parallel.
func TestInstall(t *testing.T) { //nolint:paralleltest // swaps the process-wide panic handler
	var told int
	SetLog(func(string, ...any) { told++ })
	Install()
	t.Cleanup(func() {
		core.SetPanicHandler(nil)
		SetLog(nil)
	})

	Expect("test.installed")
	err := core.Recovered("test.installed", "boom")

	require.ErrorIs(t, err, core.ErrPanic)
	assert.Equal(t, 1, told)
}
