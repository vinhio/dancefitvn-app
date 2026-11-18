//go:build !tvos

package app

import (
	"fmt"
	"strings"
	"testing"

	"dancefitvn-app/internal/download"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/mobile/env"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCorruptDownloadedAudioIsStreamedInstead plays a downloaded lesson whose file
// is the right size but that the system cannot open or decode (a damaged disk): the
// first failure of an unopenable file asks for a new address, which is now the
// API's, and plays; a file that does not decode shows the error state once, and
// the next press plays the stream. The file is never handed over a second time.
// (docs/plan/03 section 9: "Không phát được bài này. Thử lại sau", retry)
func TestCorruptDownloadedAudioIsStreamedInstead(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantError bool // the error state shows before the retry
	}{
		{"cannot open the source", fmt.Errorf("avf: AVFoundationErrorDomain -11828: %w", media.ErrSource), false},
		{"cannot decode", fmt.Errorf("amp: error what 1 extra -1007: %w", media.ErrDecode), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			pr := newPlayRig(t, "", nil)
			pr.signIn()
			c, err := pr.w.Course(t.Context(), demoCourseID)
			require.NoError(t, err)
			m := pr.w.downloads()
			require.NoError(t, m.Start(c.Lessons[2], download.CourseInfo{ID: demoCourseID}))
			var path string
			pr.drive("the download", func() bool {
				p, ok := m.Path("l_3503")
				path = p

				return ok
			})
			list := env.PlayerLessons(c)
			p := pr.player()
			p.PlayAudio(list[2], list)
			pr.drive("the file loaded", func() bool { return len(pr.loads()) == 1 })
			require.Equal(t, path, pr.loads()[0].Src.URL, "the file on the device first")
			pr.eng.Fail(tc.err)

			if tc.wantError {
				pr.state(player.StateError)
				require.Len(t, pr.loads(), 1, "a file that does not decode is not loaded again by itself")
				p.PlayAudio(list[2], list)
			}
			pr.state(player.StatePlaying)
			loads := pr.loads()
			require.Len(t, loads, 2)
			assert.True(t, strings.Contains(loads[1].Src.URL, "/audio/l_3503.mp3"), "the stream: %s", loads[1].Src.URL)
			assert.NotEqual(t, path, loads[1].Src.URL)

			// the rest of the run streams it, and deleting the file forgets the verdict
			res := wireResolver{w: pr.w}
			u, err := res.AudioURL(t.Context(), "l_3503")
			require.NoError(t, err)
			assert.Contains(t, u, "/audio/l_3503.mp3")
		})
	}
}
