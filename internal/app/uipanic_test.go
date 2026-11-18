package app

import (
	"context"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRecoveredUIWorkGoesOn checks, on a Tester recovering panics as the
// app's host does under safe.Install (gDesk G16), that a ui.Go then that panics and a posted function that
// panics are reported with where they ran, and that the UI work queued
// after them still runs, in order.
func TestRecoveredUIWorkGoesOn(t *testing.T) {
	t.Parallel()

	w := &ui.Base{}
	tt, err := ui.NewTester(ui.Column(0, w), 393, 852)
	require.NoError(t, err)
	t.Cleanup(tt.Close)
	var where []string
	tt.RecoverPanics(func(p *core.Panic) { where = append(where, p.Where) })

	var got []string
	ui.Go(w, func(context.Context) (int, error) { return 1, nil }, func(*ui.Context, int, error) { panic("then") })
	tt.Post(func() { panic("post") })
	tt.RunUntilIdle()
	ui.Go(w, func(context.Context) (string, error) { return "later then", nil }, func(_ *ui.Context, s string, _ error) {
		got = append(got, s)
	})
	tt.RunUntilIdle()
	tt.Post(func() { got = append(got, "later post") })
	tt.RunUntilIdle()

	assert.ElementsMatch(t, []string{"ui.Go then", "ui.Tester post"}, where)
	assert.Equal(t, []string{"later then", "later post"}, got)
}
