package player_test

import (
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeekBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		from  float64
		delta float64
		want  float64
	}{
		{"back 10 from 5 clamps at 0", 5, -10, 0},
		{"forward 10 from 215 clamps at dur-1", 215, 10, 219},
		{"forward from the middle", 100, 10, 110},
		{"back from the middle", 100, -10, 90},
		{"at dur-1 stays", 219, 10, 219},
		{"at 0 stays", 0, -10, 0},
		{"exactly 10 back", 10, -10, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			r.p.Pause()
			r.p.Seek(tc.from)
			require.InDelta(t, tc.from, r.p.Pos(), 0)
			r.p.SeekBy(tc.delta)

			assert.InDelta(t, tc.want, r.p.Pos(), 0)
			assert.InDelta(t, tc.want, r.eng.Position(), 0)
			assert.True(t, r.has(player.ChangeSeek))
		})
	}
}

func TestSeekByReadsTheTruePosition(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.SetRate(1)
	r.p.Background(false)
	r.eng.SetBackground(true) // no ticks, the media plays on
	r.clk.Advance(30 * 1e9)
	require.InDelta(t, 0, r.p.Pos(), 0, "no tick told it")
	r.p.SeekBy(10)

	assert.InDelta(t, 40, r.p.Pos(), 0.001)
}

func TestSeekScrub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		to   float64
		want float64
	}{
		{"middle", 110, 110},
		{"before the start", -5, 0},
		{"past the end", 300, 220},
		{"the end", 220, 220},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			r.playVideo(0)
			r.p.Seek(tc.to)

			assert.InDelta(t, tc.want, r.p.Pos(), 0)
			assert.InDelta(t, tc.want, r.eng.Position(), 0)
		})
	}
}

func TestSeekBeforeTheAddressCameBeginsThere(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	l := course()
	r.p.PlayVideo(l[0], l)
	r.p.Seek(50)
	r.settle()

	require.Len(t, r.loads(), 1)
	assert.InDelta(t, 50, r.loads()[0].Load.Start, 0)
	assert.Zero(t, r.count(fake.OpSeek))
}

func TestSeekAfterTheEndPausesInsteadOfEnded(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.eng.Finish()
	requireState(t, r.p, player.StateEnded)
	r.p.Seek(10)

	requireState(t, r.p, player.StatePaused)
	assert.InDelta(t, 10, r.p.Pos(), 0)
}

func TestPlayPauseToggle(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.p.Play() // nothing loaded
	r.p.Pause()
	r.p.Toggle()
	requireState(t, r.p, player.StateIdle)
	assert.Empty(t, r.eng.Calls())

	r.playVideo(0)
	requireState(t, r.p, player.StatePlaying)
	r.p.Toggle()
	requireState(t, r.p, player.StatePaused)
	assert.False(t, r.p.Playing())
	r.p.Toggle()
	requireState(t, r.p, player.StatePlaying)
	assert.True(t, r.p.Playing())
}

func TestPauseWhileTheAddressIsOnItsWay(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	l := course()
	r.p.PlayVideo(l[0], l)
	r.p.Pause()
	requireState(t, r.p, player.StatePaused)
	r.settle()

	require.Len(t, r.loads(), 1)
	assert.False(t, r.loads()[0].Load.Auto)
	requireState(t, r.p, player.StatePaused)

	// and Play while another address is on its way plays when it came
	r.p.PlayVideo(l[1], l)
	r.p.Pause()
	r.p.Play()
	r.settle()
	assert.True(t, r.loads()[1].Load.Auto)
	requireState(t, r.p, player.StatePlaying)
}

func TestPlayAtTheEndStartsOver(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.eng.Finish()
	requireState(t, r.p, player.StateEnded)
	r.p.Play()

	requireState(t, r.p, player.StatePlaying)
	assert.InDelta(t, 0, r.p.Pos(), 0)
}

func TestPlayAppliesTheRateFirst(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.Pause()
	r.p.SetRate(1.5)
	r.eng.ClearCalls()
	r.p.Play()

	calls := r.eng.Calls()
	require.Len(t, calls, 2)
	assert.Equal(t, fake.OpRate, calls[0].Op)
	assert.InDelta(t, 1.5, calls[0].Value, 0)
	assert.Equal(t, fake.OpPlay, calls[1].Op)
}

func TestLeaveAndBackground(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		audio      bool
		act        func(p *player.Player)
		wantPaused bool
		wantLeaves int
		wantBack   *bool
	}{
		{"leave video pauses", false, func(p *player.Player) { p.Leave() }, true, 1, nil},
		{"leave audio plays on", true, func(p *player.Player) { p.Leave() }, false, 1, nil},
		{"background video pauses", false, func(p *player.Player) { p.Background(true) }, true, 0, ptr(true)},
		{"background audio plays on", true, func(p *player.Player) { p.Background(true) }, false, 0, ptr(true)},
		{"foreground again", false, func(p *player.Player) { p.Background(true); p.Background(false) }, true, 0, ptr(false)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRig(t, fullLength())
			if tc.audio {
				r.playAudio(0)
			} else {
				r.playVideo(0)
			}
			r.p.SetRate(1)
			r.tick(8)
			r.sav.pauses, r.sav.updates = 0, nil
			tc.act(r.p)

			if tc.wantPaused {
				requireState(t, r.p, player.StatePaused)
			} else {
				requireState(t, r.p, player.StatePlaying)
			}
			assert.Equal(t, tc.wantLeaves, r.sav.leaves)
			assert.GreaterOrEqual(t, r.sav.pauses+r.sav.leaves, 1, "the position is saved")
			require.NotEmpty(t, r.sav.updates)
			assert.InDelta(t, 2, r.sav.updates[len(r.sav.updates)-1].pos, 0.001)
			if tc.wantBack != nil {
				var last *bool
				for _, c := range r.eng.Calls() {
					if c.Op == fake.OpSetBackground {
						last = &c.Flag
					}
				}
				require.NotNil(t, last)
				assert.Equal(t, *tc.wantBack, *last)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestLeaveVideoStopsTheLateTicks(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.tick(4)
	r.p.Leave()
	n := len(r.sav.updates)
	require.Equal(t, 1, r.sav.leaves)
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: r.eng.Token(), Time: 9})
	assert.Len(t, r.sav.updates, n, "a late tick does not bring the lesson back")

	l := course()
	r.p.PlayVideo(l[0], l)
	r.tick(1)
	assert.Greater(t, len(r.sav.updates), n)
}

func TestLeaveAudioKeepsSaving(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playAudio(0)
	r.p.Leave()
	n := len(r.sav.updates)
	r.tick(2)

	assert.Greater(t, len(r.sav.updates), n)
	requireState(t, r.p, player.StatePlaying)
}

func TestClose(t *testing.T) {
	t.Parallel()

	r := newRig(t, fullLength())
	r.playVideo(0)
	r.p.Close()
	r.p.Close() // idempotent
	assert.Equal(t, 1, r.count(fake.OpStop))
	before := len(r.got)
	r.p.PlayVideo(course()[1], course())
	r.p.Play()
	r.p.Seek(5)
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: r.eng.Token(), Time: 9})
	assert.Len(t, r.got, before)
	assert.Equal(t, 1, r.res.calls())
}
