// Adapted from jplayer-tv/internal/media/avf/interrupt_test.go at 781bee5.

package avftv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeInterruption(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		typ, opts     uint64
		ok            bool
		began, resume bool
	}{
		{"began", 1, 0, true, true, false},
		{"began ignores options", 1, 1, true, true, false},
		{"ended, stay paused", 0, 0, true, false, false},
		{"ended, should resume", 0, 1, true, false, true},
		{"ended, other option bits", 0, 2, true, false, false},
		{"ended, resume among others", 0, 3, true, false, true},
		{"unknown type", 2, 1, false, false, false},
		{"huge type", 1 << 40, 0, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			ev, ok := decodeInterruption(c.typ, c.opts)
			assert.Equal(t, c.ok, ok)
			began, resume := ev.split()
			assert.Equal(t, c.began, began)
			assert.Equal(t, c.resume, resume)
		})
	}
}

func TestInterruptEventSplit(t *testing.T) {
	t.Parallel()

	cases := map[interruptEvent][2]bool{
		interruptEventUnknown: {false, false},
		interruptBegan:        {true, false},
		interruptEnded:        {false, false},
		interruptEndedResume:  {false, true},
		interruptEvent(99):    {false, false},
	}
	for ev, want := range cases {
		began, resume := ev.split()
		assert.Equal(t, want, [2]bool{began, resume}, "event %d", ev)
	}
}

func TestInterruptQueueKeepsOrderAndPostsOnce(t *testing.T) {
	t.Parallel()

	var q interruptQueue
	assert.True(t, q.push(interruptBegan), "the first event posts a drain")
	assert.False(t, q.push(interruptEndedResume), "one drain is queued already")
	assert.False(t, q.push(interruptBegan))
	var got [][2]bool
	q.deliver(false, func(b, r bool) { got = append(got, [2]bool{b, r}) })
	assert.Equal(t, [][2]bool{{true, false}, {false, true}, {true, false}}, got, "in order")

	got = nil
	q.deliver(false, func(b, r bool) { got = append(got, [2]bool{b, r}) })
	assert.Empty(t, got, "taken once")
	assert.True(t, q.push(interruptEnded), "after a drain the next event posts again")
}

func TestInterruptQueueFullReplacesLast(t *testing.T) {
	t.Parallel()

	var q interruptQueue
	for range interruptCap {
		q.push(interruptBegan)
	}
	q.push(interruptEndedResume)
	var got [][2]bool
	q.deliver(false, func(b, r bool) { got = append(got, [2]bool{b, r}) })
	assert.Len(t, got, interruptCap)
	assert.Equal(t, [2]bool{false, true}, got[interruptCap-1], "the latest event is never lost")
}

func TestInterruptQueueClosedOrNoHandlerDropsEvents(t *testing.T) {
	t.Parallel()

	var q interruptQueue
	q.push(interruptBegan)
	called := false
	q.deliver(true, func(bool, bool) { called = true })
	assert.False(t, called, "closed")
	q.push(interruptBegan)
	q.deliver(false, nil)
	q.deliver(false, func(bool, bool) { called = true })
	assert.False(t, called, "taken by the drain without a handler")
}

func TestInterruptDeliverDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun counts the whole process's allocations
	var q interruptQueue
	n := 0
	fn := func(bool, bool) { n++ }
	allocs := testing.AllocsPerRun(100, func() {
		q.push(interruptBegan)
		q.push(interruptEndedResume)
		q.deliver(false, fn)
	})
	assert.Zero(t, allocs)
	assert.Positive(t, n)
}
