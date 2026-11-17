package progress_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFlushNeverSendsALessonTwice checks 07 §2 (progress: batches of at most
// 100, no duplicate sent): a queue fed with repeated saves of 130 lessons
// sends each lesson once over all its batches, with the newest position.
func TestFlushNeverSendsALessonTwice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lessons int
		rounds  int
		sizes   []int
	}{
		{"exactly one batch", 100, 3, []int{100}},
		{"one over", 101, 2, []int{100, 1}},
		{"130 lessons saved 4 times", 130, 4, []int{100, 30}},
		{"250 lessons", 250, 2, []int{100, 100, 50}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			q, a, _ := newQueue(t)
			for round := range tc.rounds {
				for i := range tc.lessons {
					require.NoError(t, q.Add(item("l_"+strconv.Itoa(i), round*10+i, time.Duration(round)*time.Second)))
				}
			}
			require.Equal(t, tc.lessons, q.Len(), "one entry per lesson")
			_, err := q.Flush(context.Background())
			require.NoError(t, err)

			require.Equal(t, tc.sizes, a.batchSizes())
			seen := map[string]int{}
			for _, batch := range a.batches {
				require.LessOrEqual(t, len(batch), 100)
				for _, it := range batch {
					seen[it.LessonID]++
					i, err := strconv.Atoi(it.LessonID[2:])
					require.NoError(t, err)
					require.Equal(t, (tc.rounds-1)*10+i, int(it.PositionSec), "newest save wins")
				}
			}
			require.Len(t, seen, tc.lessons)
			for id, n := range seen {
				require.Equal(t, 1, n, "%s sent more than once", id)
			}
		})
	}
}
