package player_test

import (
	"bytes"
	"context"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/media/fake"
	"dancefitvn-app/internal/player"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMain fails the run when a test leaves a goroutine of player behind.
func TestMain(m *testing.M) {
	code := m.Run()
	if code == 0 && leaked() {
		_, _ = os.Stderr.WriteString("goroutine leak in package player\n")
		code = 1
	}
	os.Exit(code)
}

func leaked() bool {
	for range 500 {
		var buf bytes.Buffer
		_ = pprof.Lookup("goroutine").WriteTo(&buf, 2)
		if !strings.Contains(buf.String(), "internal/player.") {
			return false
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}

	return true
}

var t0 = time.Date(2026, 11, 15, 9, 0, 0, 0, time.UTC)

// course makes the five lessons of a course, 220 s each.
func course() []player.Lesson {
	out := make([]player.Lesson, 5)
	for i := range out {
		out[i] = player.Lesson{
			CourseID: "set-35", CourseName: "Set 35", ID: fmt.Sprintf("l_%d", i+1), Title: fmt.Sprintf("Bai %d", i+1),
			No: i + 1, Count: len(out), DurationSec: 220,
		}
	}

	return out
}

func videoURL(id string) string { return "https://x.test/video/" + id + ".m3u8" }
func audioURL(id string) string { return "https://x.test/audio/" + id + ".mp3" }

// resolver is a scripted player.Resolver.
type resolver struct {
	mu     sync.Mutex
	videos []string // lesson ids asked, in order
	audios []string
	// err, when set, answers the n-th (1-based) call of VideoURL or AudioURL.
	err map[int]error
	n   int
	// urls, when set, gives the address of the n-th call instead of the default.
	urls map[int]string
}

func (r *resolver) answer(id string, def func(string) string) (string, error) {
	r.n++
	if err := r.err[r.n]; err != nil {
		return "", err
	}
	if u, ok := r.urls[r.n]; ok {
		return u, nil
	}

	return def(id), nil
}

func (r *resolver) VideoURL(_ context.Context, id string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.videos = append(r.videos, id)

	return r.answer(id, videoURL)
}

func (r *resolver) AudioURL(_ context.Context, id string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.audios = append(r.audios, id)

	return r.answer(id, audioURL)
}

func (r *resolver) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.n
}

// saver is a player.Saver that records what it is told.
type saver struct {
	updates []update
	pauses  int
	leaves  int
	// seeked lists the seeks the user made (SeekNoter), and order the calls in
	// the order they came: "seek" and "update".
	seeked []update
	order  []string
}

type update struct {
	lesson, course string
	pos, dur       float64
	playing        bool
}

func (s *saver) Update(l, c string, pos, dur float64, playing bool) {
	s.updates = append(s.updates, update{l, c, pos, dur, playing})
	s.order = append(s.order, "update")
}

func (s *saver) Seeked(l string, pos, dur float64) {
	s.seeked = append(s.seeked, update{lesson: l, pos: pos, dur: dur})
	s.order = append(s.order, "seek")
}
func (s *saver) Pause() { s.pauses++ }
func (s *saver) Leave() { s.leaves++ }

// rig is a Player over a fake engine, a fake clock, a scripted Resolver and a
// recording Saver. The Resolver runs in the caller (Go), its answer is queued
// by Dispatch and brought to the UI thread (the test) by settle.
type rig struct {
	t     testing.TB
	clk   *clock.Fake
	eng   *fake.Engine
	ses   *fake.Session
	res   *resolver
	sav   *saver
	p     *player.Player
	queue []func()
	mu    sync.Mutex
	got   []player.Change
	auto  bool
}

type rigOpts struct {
	eng     fake.Options
	noSaver bool
	res     *resolver
	autoOff bool
	// saver replaces the recording one; quiet leaves the change log off.
	saver player.Saver
	quiet bool
}

func newRig(t testing.TB, o rigOpts) *rig {
	t.Helper()
	r := &rig{t: t, clk: clock.NewFake(t0), ses: &fake.Session{}, res: o.res, sav: &saver{}, auto: !o.autoOff}
	if r.res == nil {
		r.res = &resolver{}
	}
	r.eng = fake.New(media.Config{Clock: r.clk}, o.eng)
	po := player.Options{
		Engine: r.eng, Session: r.ses, Resolver: r.res,
		AutoNext: func() bool { return r.auto },
		Dispatch: func(f func()) { r.mu.Lock(); r.queue = append(r.queue, f); r.mu.Unlock() },
		Go:       func(work func(context.Context)) { work(context.Background()) },
	}
	switch {
	case o.saver != nil:
		po.Saver = o.saver
	case !o.noSaver:
		po.Saver = r.sav
	}
	r.p = player.New(po)
	if !o.quiet {
		r.p.Changes().Subscribe(func(c player.Change) { r.got = append(r.got, c) })
	}
	t.Cleanup(r.p.Close)

	return r
}

// settle runs the queued answers and the engine's pending calls.
func (r *rig) settle() {
	r.t.Helper()
	for range 10 {
		r.mu.Lock()
		q := r.queue
		r.queue = nil
		r.mu.Unlock()
		for _, f := range q {
			f()
		}
		r.clk.Advance(0)
		r.mu.Lock()
		n := len(r.queue)
		r.mu.Unlock()
		if n == 0 {
			return
		}
	}
}

// tick lets n ticks of the engine (250 ms each) pass.
func (r *rig) tick(n int) {
	for range n {
		r.clk.Advance(fake.DefaultTick)
	}
}

// playing loads and plays lesson i (0-based) of the course as a video.
func (r *rig) playVideo(i int) {
	r.t.Helper()
	l := course()
	r.p.PlayVideo(l[i], l)
	r.settle()
}

func (r *rig) playAudio(i int) {
	r.t.Helper()
	l := course()
	r.p.PlayAudio(l[i], l)
	r.settle()
}

// loads lists the Load calls of the engine.
func (r *rig) loads() []fake.Call {
	var out []fake.Call
	for _, c := range r.eng.Calls() {
		if c.Op == fake.OpLoad {
			out = append(out, c)
		}
	}

	return out
}

// count returns how many calls of op the engine got.
func (r *rig) count(op fake.Op) int {
	n := 0
	for _, c := range r.eng.Calls() {
		if c.Op == op {
			n++
		}
	}

	return n
}

// kinds lists the kinds of Change so far.
func (r *rig) kinds() []player.ChangeKind {
	var out []player.ChangeKind
	for _, c := range r.got {
		out = append(out, c.Kind)
	}

	return out
}

func (r *rig) has(k player.ChangeKind) bool {
	for _, c := range r.got {
		if c.Kind == k {
			return true
		}
	}

	return false
}

func requireState(t testing.TB, p *player.Player, want player.State) {
	t.Helper()
	require.Equal(t, want, p.State())
}
