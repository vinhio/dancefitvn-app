package player

import (
	"context"
	"dancefitvn-app/internal/media"
	"math"
	"slices"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PlayVideo plays the video of l, whose course is list (03 §3 playVideo).
// Another lesson starts at its resume position (when it is under 90 % of the
// length, else at 0) with A = round(dur·0.33) and B = A + 28; the same
// lesson goes on where it is. The options stay.
func (p *Player) PlayVideo(l Lesson, list []Lesson) { p.start3(l, list, KindVideo) }

// PlayAudio plays the sound of l (03 §3 playAudio): another lesson starts at
// 0 and leaves A and B alone, as the prototype does.
func (p *Player) PlayAudio(l Lesson, list []Lesson) { p.start3(l, list, KindAudio) }

// Prev plays the lesson before the one loaded, at 0, as it is playing or
// paused; A, B and Loop stay (Q-P4-4). It does nothing at the first lesson.
func (p *Player) Prev() { p.step(-1, p.want) }

// Next plays the lesson after the one loaded, as Prev does.
func (p *Player) Next() { p.step(1, p.want) }

// AutoAdvance plays the next lesson when the preference "Tự phát bài tiếp
// theo" is on and the course has one; the end-of-lesson screen calls it at the
// end of its countdown. It reports whether it moved on.
func (p *Player) AutoAdvance() bool {
	if p.o.AutoNext != nil && !p.o.AutoNext() {
		return false
	}

	return p.step(1, true)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// start3 is PlayVideo and PlayAudio.
func (p *Player) start3(l Lesson, list []Lesson, k Kind) {
	if p.closed || l.ID == "" {
		return
	}
	p.list = slices.Clone(list)
	if l.ID == p.lesson.ID {
		p.lesson = l
		switch {
		case k == p.kind && (p.state == StateLoading || p.state == StatePlaying || p.state == StatePaused || p.state == StateEnded):
			p.detached = false
			p.Play()
		default: // the other media of the lesson, or a failed load: from where it was
			start := p.pos
			if p.state == StateEnded || (p.dur > 0 && start >= p.dur-1) {
				start = 0
			}
			p.kind = k
			p.ensureAB()
			p.begin(start, false, true)
		}

		return
	}
	p.saveLeave()
	p.lesson, p.kind = l, k
	p.dur = float64(l.DurationSec)
	var start float64
	if k == KindVideo {
		start = resumeStart(l)
		p.a, p.b = defaultAB(p.dur)
	} else {
		p.ensureAB()
	}
	p.begin(start, start > 0, true)
}

// resumeStart is where a video begins: the resume position of the course
// data when it is under 90 % of the length, else 0. A watched lesson starts
// over.
func resumeStart(l Lesson) float64 {
	r := float64(l.ResumeSec)
	if l.Watched || r <= 0 {
		return 0
	}
	if l.DurationSec > 0 && r >= WatchedRatio*float64(l.DurationSec) {
		return 0
	}

	return r
}

// defaultAB is A = round(dur·0.33) and B = A + 28 ($P:595).
func defaultAB(dur float64) (a, b float64) {
	a = math.Round(dur * loopStart)

	return a, a + loopLength
}

// ensureAB gives A and B their default when no lesson has set them.
func (p *Player) ensureAB() {
	if p.a == 0 && p.b == 0 {
		p.a, p.b = defaultAB(p.dur)
	}
}

// step plays the lesson delta places on in the list, at 0; auto says whether
// it plays. It reports whether there was such a lesson.
func (p *Player) step(delta int, auto bool) bool {
	i := p.index()
	j := i + delta
	if p.closed || i < 0 || j < 0 || j >= len(p.list) {
		return false
	}
	p.saveLeave()
	p.lesson = p.list[j]
	p.dur = float64(p.lesson.DurationSec)
	p.begin(0, false, auto)

	return true
}

// stopLoad drops the load under way: the answer of the Resolver that comes
// later is stale.
func (p *Player) stopLoad() {
	p.gen++
	if p.stopGen != nil {
		p.stopGen()
		p.stopGen = nil
	}
}

// begin starts loading the lesson of p at start: it asks the Resolver for the
// address, off the UI thread. check asks to look at start once the engine's
// length is known.
func (p *Player) begin(start float64, check, auto bool) {
	p.stopLoad()
	if p.loaded {
		p.loaded = false // the events of the old media are of no interest now
		_ = p.o.Engine.Pause()
	}
	p.start, p.pos, p.checkStart = start, start, check
	p.retried, p.err = false, nil
	p.want, p.interrupted, p.detached = auto, false, false
	p.emit(ChangeLesson)
	p.setState(StateLoading)
	p.resolve()
}

// reload asks for a new address of the lesson and loads it at the position
// reached; the lesson and its options stay.
func (p *Player) reload(auto bool) {
	p.stopLoad()
	p.loaded = false
	p.start, p.checkStart = p.pos, false
	p.err = nil
	p.want = auto
	p.setState(StateLoading)
	p.resolve()
}

// resolve asks the Resolver for the address of the lesson and kind loaded.
func (p *Player) resolve() {
	gen, id, kind := p.gen, p.lesson.ID, p.kind
	lctx, lcancel := context.WithCancel(p.ctx)
	p.stopGen = lcancel
	work := func(c context.Context) {
		defer lcancel()
		defer context.AfterFunc(c, lcancel)()
		var url string
		var err error
		if kind == KindAudio {
			url, err = p.o.Resolver.AudioURL(lctx, id)
		} else {
			url, err = p.o.Resolver.VideoURL(lctx, id)
		}
		p.o.Dispatch(func() { p.onResolved(gen, url, err) })
	}
	if p.o.Go != nil {
		p.o.Go(work)

		return
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		work(p.ctx)
	}()
}

// onResolved is the answer of the Resolver, on the UI thread.
func (p *Player) onResolved(gen uint64, url string, err error) {
	if p.closed || gen != p.gen {
		return
	}
	p.stopGen = nil
	if err != nil {
		p.fail(resolveError(err))

		return
	}
	p.url = url
	p.loadURL()
}

// loadURL gives the engine the address, to begin at p.start.
func (p *Player) loadURL() {
	if p.o.Session != nil {
		if err := p.o.Session.Activate(); err != nil {
			p.logf("player: audio session: %v", err)
		}
	}
	src := media.Source{URL: p.url, AudioOnly: p.kind == KindAudio}
	tok, err := p.o.Engine.Load(p.ctx, src, media.LoadOptions{Auto: p.want, Start: p.start})
	if err != nil {
		p.fail(err)

		return
	}
	p.token, p.loaded = tok, true
	_ = p.o.Engine.SetRate(p.rate)
}
