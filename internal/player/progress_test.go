package player_test

import (
	"context"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// *progress.Tracker is the Saver of the app.
var _ player.Saver = (*progress.Tracker)(nil)

// sender is a progress.Sender that tells each save on a channel.
type sender struct {
	mu    sync.Mutex
	saves []save
	told  chan struct{}
}

type save struct {
	lesson string
	req    model.ProgressRequest
}

func (s *sender) SaveProgress(_ context.Context, id string, r model.ProgressRequest) (model.ProgressResult, error) {
	s.mu.Lock()
	s.saves = append(s.saves, save{id, r})
	s.mu.Unlock()
	s.told <- struct{}{}

	return model.ProgressResult{LessonID: id, PositionSec: r.PositionSec}, nil
}

func (s *sender) all() []save {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]save(nil), s.saves...)
}

func (s *sender) SendProgressBatch(context.Context, model.BatchRequest) (model.BatchResult, error) {
	return model.BatchResult{}, nil
}

// trackerRig is a Player saving through a real Tracker over a fake sender.
type trackerRig struct {
	*rig
	snd *sender
	ui  chan func()
	won []string
}

func newTrackerRig(t *testing.T) *trackerRig {
	t.Helper()
	tr := &trackerRig{snd: &sender{told: make(chan struct{}, 16)}, ui: make(chan func(), 16)}
	q, err := progress.NewQueue(store.NewMemory(), tr.snd)
	require.NoError(t, err)
	// the tracker shares the clock of the engine, made by the rig below
	var tracker *progress.Tracker
	tr.rig = newRig(t, rigOpts{eng: fullLength().eng, saver: saverFunc(func() player.Saver { return tracker })})
	tracker = progress.NewTracker(progress.Options{
		Sender: tr.snd, Queue: q, Clock: tr.clk, Dispatch: func(f func()) { tr.ui <- f },
		OnWatched: func(id string) { tr.won = append(tr.won, id) },
	})
	t.Cleanup(tracker.Close)

	return tr
}

// saverFunc hands the Saver over once it exists.
type saverFunc func() player.Saver

func (f saverFunc) Update(l, c string, pos, dur float64, playing bool) {
	f().Update(l, c, pos, dur, playing)
}
func (f saverFunc) Pause() { f().Pause() }
func (f saverFunc) Leave() { f().Leave() }

// await waits for a save to reach the sender and runs the tracker's answer
// on this goroutine, the UI thread.
func (tr *trackerRig) await() {
	<-tr.snd.told
	(<-tr.ui)()
}

func TestSavesThroughTheTracker(t *testing.T) {
	t.Parallel()

	tr := newTrackerRig(t)
	tr.playVideo(0)
	tr.p.SetRate(1)

	// every 10 s while playing
	tr.tick(39)
	assert.Empty(t, tr.snd.all(), "not before 10 s")
	tr.tick(1)
	tr.await()
	require.Len(t, tr.snd.all(), 1)
	assert.Equal(t, "l_1", tr.snd.all()[0].lesson)
	assert.Equal(t, 10, tr.snd.all()[0].req.PositionSec)
	assert.Equal(t, 220, tr.snd.all()[0].req.DurationSec)

	// on pause
	tr.tick(20) // 15 s
	tr.p.Pause()
	tr.await()
	require.Len(t, tr.snd.all(), 2)
	assert.Equal(t, 15, tr.snd.all()[1].req.PositionSec)

	// paused ticks do not save, nor does leaving at the same second twice
	tr.p.Play()
	tr.tick(4) // 16 s
	tr.p.Leave()
	tr.await()
	require.Len(t, tr.snd.all(), 3)
	assert.Equal(t, 16, tr.snd.all()[2].req.PositionSec)
	assert.Empty(t, tr.won)
}

func TestWatchedAtNinetyPercent(t *testing.T) {
	t.Parallel()

	tr := newTrackerRig(t)
	tr.playVideo(1)
	tr.p.Seek(197)
	assert.Empty(t, tr.won)
	tr.p.Seek(198)
	assert.Equal(t, []string{"l_2"}, tr.won)
	tr.p.Seek(210)
	assert.Equal(t, []string{"l_2"}, tr.won, "once")

	// the end is saved at the full length
	tr.eng.Finish()
	tr.await()
	got := tr.snd.all()
	require.NotEmpty(t, got)
	assert.Equal(t, "l_2", got[len(got)-1].lesson)
	assert.Equal(t, 220, got[len(got)-1].req.PositionSec)
	assert.Equal(t, 220, got[len(got)-1].req.DurationSec)
}

func TestChangingLessonSavesTheOldOne(t *testing.T) {
	t.Parallel()

	tr := newTrackerRig(t)
	tr.playVideo(0)
	tr.p.SetRate(1)
	tr.tick(12) // 3 s
	tr.p.Next()
	tr.await()
	tr.settle()

	require.Len(t, tr.snd.all(), 1)
	assert.Equal(t, "l_1", tr.snd.all()[0].lesson)
	assert.Equal(t, 3, tr.snd.all()[0].req.PositionSec)
}

// countSaver counts, and allocates nothing.
type countSaver struct{ updates, pauses, leaves int }

func (c *countSaver) Update(string, string, float64, float64, bool) { c.updates++ }
func (c *countSaver) Pause()                                        { c.pauses++ }
func (c *countSaver) Leave()                                        { c.leaves++ }

// tickRig is a playing video with one listener of the changes and a Saver that
// does not allocate; the loop is on and B is far.
func tickRig(t testing.TB, loop bool) (*rig, media.Event, *int) {
	t.Helper()
	r := newRig(t, rigOpts{eng: fullLength().eng, saver: &countSaver{}, quiet: true})
	r.playVideo(0)
	if loop {
		r.p.SetLoop(true)
	}
	n := new(int)
	r.p.Changes().Subscribe(func(player.Change) { *n++ })

	return r, media.Event{Kind: media.EventTime, Token: r.eng.Token(), Time: 10}, n
}

func TestTickDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun cannot run in parallel
	for _, loop := range []bool{false, true} {
		r, ev, n := tickRig(t, loop)
		require.Equal(t, player.StatePlaying, r.p.State())
		r.eng.Emit(ev) // warm: the topic builds its snapshot
		before := *n
		allocs := testing.AllocsPerRun(200, func() { r.eng.Emit(ev) })

		assert.Zero(t, allocs, "loop=%v", loop)
		assert.Greater(t, *n, before)
		assert.InDelta(t, 10, r.p.Pos(), 0)
	}
}

func TestTickPastBLoopsBack(t *testing.T) {
	t.Parallel()

	r, _, _ := tickRig(t, true)
	r.eng.Emit(media.Event{Kind: media.EventTime, Token: r.eng.Token(), Time: r.p.B()})
	assert.InDelta(t, r.p.A(), r.p.Pos(), 0)
	assert.Equal(t, 2, r.count(fake.OpSeek))
}

func BenchmarkTick(b *testing.B) {
	r, ev, _ := tickRig(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		r.eng.Emit(ev)
	}
}
