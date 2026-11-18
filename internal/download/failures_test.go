package download_test

import (
	"testing"

	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/safe"

	"github.com/stretchr/testify/require"
)

// TestPanicInATransferFailsOnlyThatJob makes the work of one lesson panic (a bad
// answer that a parser did not expect): that job ends as a failed one, with the
// error the toast "Tải lỗi, thử lại" is for, and the next lesson downloads.
// (docs/plan/03 section 9; a panic on a worker goroutine would end the app)
func TestPanicInATransferFailsOnlyThatJob(t *testing.T) {
	t.Parallel()
	safe.Expect("download.run")

	r := newRig(t, 20_000)
	r.src.fn = func(n int, id string) (model.Audio, error) {
		if id == "l_3503" {
			panic("index out of range [3] with length 1")
		}

		return r.audio(n, id), nil
	}
	require.NoError(t, r.mgr.Start(lesson("l_3503"), course("set-35")))
	ev := r.ev.wait(t, download.EventFailed, "l_3503")
	require.ErrorIs(t, ev.Err, safe.ErrPanic)

	require.NoError(t, r.mgr.Start(lesson("l_3502"), course("set-35")))
	r.ev.wait(t, download.EventFinished, "l_3502")
}
