package player_test

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/player"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInterrupted(t *testing.T) {
	t.Parallel()

	begin := media.SessionEvent{Kind: media.InterruptBegin}
	endYes := media.SessionEvent{Kind: media.InterruptEnd, ShouldResume: true}
	endNo := media.SessionEvent{Kind: media.InterruptEnd}
	tests := []struct {
		name   string
		paused bool // the user paused before
		events []media.SessionEvent
		user   func(p *player.Player) // after the events
		want   bool
	}{
		{"begin", false, []media.SessionEvent{begin}, nil, true},
		{"begin of a user pause", true, []media.SessionEvent{begin}, nil, false},
		{"begin then end with resume", false, []media.SessionEvent{begin, endYes}, nil, false},
		{"begin then end without resume", false, []media.SessionEvent{begin, endNo}, nil, false},
		{"begin then user pause", false, []media.SessionEvent{begin}, func(p *player.Player) { p.Pause() }, false},
		{"begin then user play", false, []media.SessionEvent{begin}, func(p *player.Player) { p.Play() }, false},
		{"noisy is no interruption", false, []media.SessionEvent{{Kind: media.Noisy}}, nil, false},
		{"end alone", false, []media.SessionEvent{endYes}, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playAudio(0)
			if tc.paused {
				r.p.Pause()
			}
			for _, ev := range tc.events {
				r.ses.Emit(ev)
			}
			if tc.user != nil {
				tc.user(r.p)
			}
			assert.Equal(t, tc.want, r.p.Interrupted())
		})
	}
}

// TestInterruptedBeforeThePauseIsTold checks a listener of the pause an
// interruption makes already sees Interrupted.
func TestInterruptedBeforeThePauseIsTold(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playAudio(0)
	var seen []bool
	r.p.Changes().Subscribe(func(c player.Change) {
		if c.Kind == player.ChangeState && r.p.State() == player.StatePaused {
			seen = append(seen, r.p.Interrupted())
		}
	})
	r.ses.Emit(media.SessionEvent{Kind: media.InterruptBegin})

	require.NotEmpty(t, seen)
	for _, s := range seen {
		assert.True(t, s)
	}
}

func TestSyncPos(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		act  func(p *player.Player)
		want float64
	}{
		{"stale without it", func(*player.Player) {}, 0},
		{"sync reads the engine", func(p *player.Player) { p.SyncPos() }, 30},
		{"a rate change reads it first", func(p *player.Player) { p.SetRate(1.5) }, 30},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playAudio(0)
			r.p.SetRate(1)
			r.p.Background(true) // audio plays on
			r.eng.SetBackground(true)
			r.clk.Advance(30 * time.Second)
			require.InDelta(t, 0, r.p.Pos(), 0, "no tick told it")
			tc.act(r.p)

			assert.InDelta(t, tc.want, r.p.Pos(), 0.5)
		})
	}
}

func TestSyncPosWithoutLoad(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.SyncPos()

	assert.InDelta(t, 0, r.p.Pos(), 0)
}

func TestBackgroundIgnoresPlayOfVideo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		video bool
		back  bool
		call  func(p *player.Player)
		want  player.State
	}{
		{"video play in the background", true, true, (*player.Player).Play, player.StatePaused},
		{"video toggle in the background", true, true, (*player.Player).Toggle, player.StatePaused},
		{"video play in the foreground", true, false, (*player.Player).Play, player.StatePlaying},
		{"audio play in the background", false, true, (*player.Player).Play, player.StatePlaying},
		{"audio toggle in the background", false, true, (*player.Player).Toggle, player.StatePlaying},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			if tc.video {
				r.playVideo(0)
			} else {
				r.playAudio(0)
			}
			r.p.Pause()
			if tc.back {
				r.p.Background(true)
			}
			r.eng.ClearCalls()
			tc.call(r.p)
			r.settle()

			requireState(t, r.p, tc.want)
			assert.Equal(t, tc.want == player.StatePlaying, r.p.Playing())
		})
	}

	t.Run("video plays again in the foreground", func(t *testing.T) {
		t.Parallel()

		r := newRig(t, fullLength())
		r.playVideo(0)
		r.p.Background(true)
		r.p.Play()
		requireState(t, r.p, player.StatePaused)
		r.p.Background(false)
		r.p.Play()
		requireState(t, r.p, player.StatePlaying)
	})
}
