package download

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
)

type nopSource struct{}

func (nopSource) Audio(context.Context, string) (model.Audio, error) { return model.Audio{}, nil }

// TestProgressThrottledDoesNotAllocate covers the path taken for almost
// every chunk of a body: inside the 250 ms window nothing is sent.
func TestProgressThrottledDoesNotAllocate(t *testing.T) {
	fake := clock.NewFake(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	events := 0
	m, err := NewManager(Options{Client: nopSource{}, Dir: t.TempDir(), Clock: fake, OnEvent: func(Event) { events++ }})
	require.NoError(t, err)
	t.Cleanup(m.Close)
	j := &job{lessonID: "l_1", lastDone: -1}
	m.progress(j, 0, 1_000_000, true) // opens the window
	require.Equal(t, 1, events)

	n := int64(0)
	allocs := testing.AllocsPerRun(1000, func() {
		n += 32 << 10
		m.progress(j, n, 1_000_000, false)
	})
	require.Zero(t, allocs)
	require.Equal(t, 1, events, "no event inside the window")
}

func BenchmarkProgressThrottled(b *testing.B) {
	fake := clock.NewFake(time.Date(2026, 11, 14, 9, 0, 0, 0, time.UTC))
	m, err := NewManager(Options{Client: nopSource{}, Dir: b.TempDir(), Clock: fake, OnEvent: func(Event) {}})
	require.NoError(b, err)
	b.Cleanup(m.Close)
	j := &job{lessonID: "l_1", lastDone: -1}
	m.progress(j, 0, 1_000_000, true)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		m.progress(j, int64(i), 1_000_000, false)
	}
}
