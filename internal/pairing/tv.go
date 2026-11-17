package pairing

import (
	"context"
	"errors"
	"sync"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Defaults when the server leaves a value out.
const (
	defaultPollInterval = 3 * time.Second
	defaultRetryAfter   = 10 * time.Second
)

// codeExpired is the error code of 410 on poll.
const codeExpired = "pair_code_expired"

// Pairer is the TV's API: start and poll; *api.Client implements it.
type Pairer interface {
	StartPairing(ctx context.Context, dev model.DeviceInfo) (model.PairStart, error)
	PollPairing(ctx context.Context, code, pollSecret string) (model.PairPoll, error)
}

// State is what TV02 shows.
type State struct {
	Code        string
	DisplayCode string
	QRURL       string
	// ExpiresAt is on the injected clock: start time plus expires_in.
	ExpiresAt time.Time
}

// TVOptions configures NewTV.
type TVOptions struct {
	// Client is the API; required.
	Client Pairer
	// Clock schedules the polls and measures the expiry; required.
	Clock clock.Clock
	// Dispatch runs fn on the UI thread; required.
	Dispatch func(fn func())
	// Device describes this TV in POST /tv/pair/start.
	Device model.DeviceInfo
	// OnState runs on the UI thread with each new code.
	OnState func(State)
	// OnConfirmed runs on the UI thread, once, with the session of the
	// phone's account. The pairing has stopped.
	OnConfirmed func(model.Session)
	// OnError runs on the UI thread when a request failed and will be
	// retried (offline, rate limited, server error); may be nil.
	OnError func(error)
}

// TV is the TV side of the pairing (03 §7.4). Start, Stop and Remaining
// run on the UI thread, and so do the callbacks. Requests run on
// goroutines of their own and come back through Dispatch.
type TV struct {
	opts TVOptions

	// UI thread state.
	gen    uint64 // bumped by Start and Stop: answers of an older one are dropped
	state  State
	secret string
	have   bool // a code is on screen
	timer  clock.Timer
	cancel context.CancelFunc
	every  time.Duration
	wg     sync.WaitGroup
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewTV returns a stopped TV pairing.
func NewTV(o TVOptions) *TV { return &TV{opts: o} }

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Start asks for a code and begins to poll; a pairing already running is
// replaced. ctx bounds the whole pairing, like Stop.
func (t *TV) Start(ctx context.Context) {
	t.Stop()
	ctx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	t.begin(ctx, t.gen)
}

// Stop ends the pairing: the timer, the request in flight and the code
// (leaving TV02). Answers still on their way are ignored.
func (t *TV) Stop() {
	t.gen++
	t.have = false
	t.secret = ""
	if t.timer != 0 {
		t.opts.Clock.Stop(t.timer)
		t.timer = 0
	}
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}
}

// Wait blocks until the goroutines of the requests have ended; call it
// after Stop, off the UI thread, where a test or shutdown needs it.
func (t *TV) Wait() { t.wg.Wait() }

// Remaining is the time left of the code on screen ("Mã hết hạn sau
// mm:ss"), 0 when there is none or it has expired.
func (t *TV) Remaining() time.Duration {
	if !t.have {
		return 0
	}

	return max(t.state.ExpiresAt.Sub(t.opts.Clock.Now()), 0)
}

// Current returns the code on screen.
func (t *TV) Current() (State, bool) { return t.state, t.have }

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// begin sends POST /tv/pair/start on a goroutine.
func (t *TV) begin(ctx context.Context, gen uint64) {
	t.have = false
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		res, err := t.opts.Client.StartPairing(ctx, t.opts.Device)
		t.opts.Dispatch(func() { t.started(ctx, gen, res, err) })
	}()
}

// started handles the answer of start on the UI thread.
func (t *TV) started(ctx context.Context, gen uint64, res model.PairStart, err error) {
	if gen != t.gen {
		return
	}
	if err != nil {
		t.failed(gen, err, func() { t.begin(ctx, gen) })

		return
	}
	t.every = time.Duration(res.PollIntervalSec) * time.Second
	if t.every <= 0 {
		t.every = defaultPollInterval
	}
	t.secret = res.PollSecret
	t.state = State{
		Code:        res.Code,
		DisplayCode: res.DisplayCode,
		QRURL:       res.QRURL,
		ExpiresAt:   t.opts.Clock.Now().Add(time.Duration(res.ExpiresIn) * time.Second),
	}
	t.have = true
	if t.opts.OnState != nil {
		t.opts.OnState(t.state)
	}
	t.schedule(ctx, gen, t.every)
}

// schedule arms the next poll after d, but no later than the expiry.
func (t *TV) schedule(ctx context.Context, gen uint64, d time.Duration) {
	d = min(d, t.state.ExpiresAt.Sub(t.opts.Clock.Now()))
	t.timer = t.opts.Clock.After(max(d, 0), func() { t.tick(ctx, gen) })
}

// tick is the timer: restart when the code has expired, else poll.
func (t *TV) tick(ctx context.Context, gen uint64) {
	if gen != t.gen {
		return
	}
	t.timer = 0
	if !t.opts.Clock.Now().Before(t.state.ExpiresAt) {
		t.begin(ctx, gen)

		return
	}
	code, secret := t.state.Code, t.secret
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		res, err := t.opts.Client.PollPairing(ctx, code, secret)
		t.opts.Dispatch(func() { t.polled(ctx, gen, res, err) })
	}()
}

// polled handles the answer of poll on the UI thread.
func (t *TV) polled(ctx context.Context, gen uint64, res model.PairPoll, err error) {
	if gen != t.gen {
		return
	}
	var apiErr *api.Error
	switch {
	case errors.As(err, &apiErr) && apiErr.Code == codeExpired:
		t.begin(ctx, gen) // a new code at once

		return
	case err != nil:
		t.failed(gen, err, func() { t.tick(ctx, gen) })

		return
	case res.Status == model.PairConfirmed:
		t.Stop()
		if t.opts.OnConfirmed != nil {
			t.opts.OnConfirmed(res.Session())
		}

		return
	default: // pending (or a status this version does not know)
		t.schedule(ctx, gen, t.every)
	}
}

// failed reports err and calls retry after the wait the error asks for:
// retry_after on 429, else one poll interval (the next tick).
func (t *TV) failed(gen uint64, err error, retry func()) {
	if t.opts.OnError != nil {
		t.opts.OnError(err)
	}
	wait := defaultPollInterval
	if t.every > 0 {
		wait = t.every
	}
	var apiErr *api.Error
	if errors.As(err, &apiErr) && apiErr.Status == 429 {
		wait = defaultRetryAfter
		if s := apiErr.Body.RetryAfter; s > 0 {
			wait = time.Duration(s) * time.Second
		}
	}
	if t.have { // a retry does not outlive the code
		wait = max(min(wait, t.state.ExpiresAt.Sub(t.opts.Clock.Now())), 0)
	}
	t.timer = t.opts.Clock.After(wait, func() {
		if gen != t.gen {
			return
		}
		t.timer = 0
		retry()
	})
}
