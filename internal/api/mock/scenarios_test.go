package mock

import (
	"context"
	"sync"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScenarioErrors(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	for _, c := range []struct{ route, name string }{
		{"nope", "default"},
		{"me.home", "nope"},
		{"", "x"},
	} {
		assert.ErrorIs(t, tr.SetScenario(c.route, c.name), core.ErrInvalid, "%v", c)
	}
	assert.Empty(t, tr.Scenarios())
}

func TestScenarioSetAndReset(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	require.NoError(t, tr.SetScenario("me.home", "server_error"))
	assert.Equal(t, map[string]string{"me.home": "server_error"}, tr.Scenarios())
	resp, err := tr.Do(context.Background(), get("/me/home"))
	require.NoError(t, err)
	assert.Equal(t, 500, resp.Status)

	got := tr.Scenarios()
	got["x"] = "y" // a copy
	assert.Len(t, tr.Scenarios(), 1)

	require.NoError(t, tr.SetScenario("me.home", "default"))
	assert.Empty(t, tr.Scenarios())
	resp, err = tr.Do(context.Background(), get("/me/home"))
	require.NoError(t, err)
	assert.Equal(t, 200, resp.Status)
}

func TestLoadScenarios(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	require.NoError(t, tr.SetScenario("app.config", "upgrade_required"))
	require.NoError(t, tr.LoadScenarios([]byte(`{"me.home":"server_error","me.get":"default"}`)))
	assert.Equal(t, map[string]string{"me.home": "server_error"}, tr.Scenarios())

	for _, bad := range []string{`{"me.home":"nope"}`, `{"nope":"x"}`, `[1]`, `{`, `{"me.get":"token_expired","me.home":"nope"}`} {
		assert.ErrorIs(t, tr.LoadScenarios([]byte(bad)), core.ErrInvalid, bad)
	}
	// a failed load changes nothing.
	assert.Equal(t, map[string]string{"me.home": "server_error"}, tr.Scenarios())
}

func TestScenarioConcurrent(t *testing.T) {
	t.Parallel()
	tr := newTransport(t)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 50 {
				name := "default"
				if (i+j)%2 == 0 {
					name = "server_error"
				}
				assert.NoError(t, tr.SetScenario("me.home", name))
				resp, err := tr.Do(context.Background(), get("/me/home"))
				if assert.NoError(t, err) {
					assert.Contains(t, []int{200, 500}, resp.Status)
				}
				_ = tr.Scenarios()
				assert.NoError(t, tr.LoadScenarios([]byte(`{"me.home":"server_error"}`)))
			}
		}()
	}
	wg.Wait()
}

func TestNewErrors(t *testing.T) {
	t.Parallel()
	_, err := New(emptyFS{}, Options{})
	assert.Error(t, err)
}
