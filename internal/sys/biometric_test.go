package sys

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// fakeSheet is a system sheet for Biometric.Authenticate's tests: it
// records what start and cancel got, and answers when the test says.
type fakeSheet struct {
	refuse error // start's error

	mu       sync.Mutex
	prompt   BioPrompt
	done     func(error)
	d        Dispatch
	shown    chan struct{}
	canceled atomic.Int32
	dropped  chan struct{} // closed by the first cancel
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

func TestBiometryKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		kind   BiometryKind
		name   string
		usable bool
	}{
		{BiometryUnknown, "unknown", false},
		{BiometryNone, "none", false},
		{BiometryFace, "face", true},
		{BiometryFingerprint, "fingerprint", true},
		{BiometryOther, "other", true},
		{BiometryKind(42), "BiometryKind(42)", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.name, tc.kind.String())
			assert.Equal(t, tc.usable, tc.kind.Usable())
		})
	}
}

func TestSentinelsWrap(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		err, base error
	}{
		{ErrBioCanceled, context.Canceled},
		{ErrBioFallback, context.Canceled},
		{ErrBioFailed, core.ErrInvalid},
		{ErrBioLockout, core.ErrInvalid},
		{ErrBioUnavailable, core.ErrUnsupported},
	} {
		t.Run(tc.err.Error(), func(t *testing.T) {
			t.Parallel()

			require.ErrorIs(t, tc.err, tc.base)
		})
	}
	assert.NotErrorIs(t, ErrBioFailed, ErrBioLockout)
	assert.NotErrorIs(t, ErrBioFallback, ErrBioCanceled)
}

// TestLAError checks the table of local/gdesk-proposals/G3-biometric.md
// §3 for iOS (LAError.h).
func TestLAError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		code int64
		want error // nil for none
	}{
		{"ok", 0, nil},
		{"authenticationFailed", -1, ErrBioFailed},
		{"userCancel", -2, ErrBioCanceled},
		{"userFallback", -3, ErrBioFallback},
		{"systemCancel", -4, ErrBioCanceled},
		{"passcodeNotSet", -5, ErrBioUnavailable},
		{"biometryNotAvailable", -6, ErrBioUnavailable},
		{"biometryNotEnrolled", -7, ErrBioUnavailable},
		{"biometryLockout", -8, ErrBioLockout},
		{"appCancel", -9, ErrBioCanceled},
		{"notInteractive", -1004, ErrBioCanceled},
		{"invalidContext", -10, core.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := laError(tc.code, "text")
			if tc.want == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
	assert.ErrorContains(t, laError(-10, "invalid"), "LAError -10: invalid")
}

func TestLAKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		t     int64
		want  BiometryKind
		usage bool
	}{
		{0, BiometryNone, false},
		{1, BiometryFingerprint, false},
		{2, BiometryFace, true},
		{4, BiometryOther, true},
		{8, BiometryOther, true},
	} {
		assert.Equal(t, tc.want, laKind(tc.t), "type %d", tc.t)
		assert.Equal(t, tc.usage, laNeedsUsage(laKind(tc.t)), "type %d", tc.t)
	}
}

// TestBridgeResult checks the table of local/gdesk-proposals/
// G3-biometric.md §3 for Android, through BiometricBridge's statuses.
func TestBridgeResult(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		status, code int32
		want         error // nil for none
	}{
		{"succeeded", bridgeSucceeded, 0, nil},
		{"three failures", bridgeFailedLimit, 0, ErrBioFailed},
		{"not shown", bridgeNotShown, 0, core.ErrInvalid},
		{"unknown status", 9, 0, core.ErrInvalid},
		{"HW_UNAVAILABLE", bridgeError, 1, ErrBioUnavailable},
		{"UNABLE_TO_PROCESS", bridgeError, 2, core.ErrInvalid},
		{"TIMEOUT", bridgeError, 3, ErrBioCanceled},
		{"NO_SPACE", bridgeError, 4, core.ErrInvalid},
		{"CANCELED", bridgeError, 5, ErrBioCanceled},
		{"LOCKOUT", bridgeError, 7, ErrBioLockout},
		{"VENDOR", bridgeError, 8, core.ErrInvalid},
		{"LOCKOUT_PERMANENT", bridgeError, 9, ErrBioLockout},
		{"USER_CANCELED", bridgeError, 10, ErrBioCanceled},
		{"NO_BIOMETRICS", bridgeError, 11, ErrBioUnavailable},
		{"HW_NOT_PRESENT", bridgeError, 12, ErrBioUnavailable},
		{"NEGATIVE_BUTTON", bridgeError, 13, ErrBioCanceled},
		{"NO_DEVICE_CREDENTIAL", bridgeError, 14, ErrBioUnavailable},
		{"SECURITY_UPDATE_REQUIRED", bridgeError, 15, ErrBioUnavailable},
		{"unknown code", bridgeError, 99, core.ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := bridgeResult(tc.status, tc.code, "message")
			if tc.want == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tc.want)
		})
	}
	assert.NotErrorIs(t, bridgeResult(bridgeError, 7, ""), context.Canceled)
}

func TestAndroidKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name         string
		avail, feats int32
		want         BiometryKind
	}{
		{"too old", availabilityTooOld, featureFingerprint, BiometryNone},
		{"none enrolled", 11, featureFingerprint, BiometryNone},
		{"no hardware", 12, 0, BiometryNone},
		{"fingerprint", 0, featureFingerprint, BiometryFingerprint},
		{"fingerprint first", 0, featureFingerprint | featureFace, BiometryFingerprint},
		{"face", 0, featureFace, BiometryFace},
		{"iris", 0, featureIris, BiometryOther},
		{"no feature declared", 0, 0, BiometryOther},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.want, androidKind(tc.avail, tc.feats))
		})
	}
}

func TestBiometricKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		kind      BiometryKind
		err       error
		want      BiometryKind
		available bool
	}{
		{"face", BiometryFace, nil, BiometryFace, true},
		{"none", BiometryNone, nil, BiometryNone, false},
		{"lockout hides", BiometryFace, ErrBioLockout, BiometryNone, false},
		{"unsupported", BiometryNone, core.ErrUnsupported, BiometryNone, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			b := NewBiometric(func(fn func()) { fn() })
			b.kind = func() (BiometryKind, error) { return tc.kind, tc.err }
			assert.Equal(t, tc.want, b.Kind())
			assert.Equal(t, tc.available, b.Available())
		})
	}
}

func TestBiometricAuthenticate(t *testing.T) {
	t.Parallel()

	prompt := BioPrompt{Reason: "reason", Title: "title", Cancel: "cancel"}
	for _, tc := range []struct {
		name   string
		answer error // the sheet's answer
		want   error // nil for none
	}{
		{"success", nil, nil},
		{"cancel", ErrBioCanceled, context.Canceled},
		{"fallback", ErrBioFallback, context.Canceled},
		{"lockout", ErrBioLockout, ErrBioLockout},
		{"not recognized", ErrBioFailed, ErrBioFailed},
		{"not enrolled", ErrBioUnavailable, core.ErrUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			d := uiLoop(t)
			sheet := newFakeSheet()
			b := NewBiometric(d)
			b.start = sheet.start
			res := make(chan error, 1)
			go func() { res <- b.Authenticate(t.Context(), prompt) }()
			<-sheet.shown
			assert.Equal(t, prompt, sheet.got())
			sheet.answer(tc.answer)
			err := <-res
			if tc.want == nil {
				require.NoError(t, err)

				return
			}
			require.ErrorIs(t, err, tc.want)
			assert.Zero(t, sheet.canceled.Load())
		})
	}
}

func TestBiometricAuthenticateRefused(t *testing.T) {
	t.Parallel()

	sheet := newFakeSheet()
	sheet.refuse = ErrBioUnavailable
	b := NewBiometric(uiLoop(t))
	b.start = sheet.start
	require.ErrorIs(t, b.Authenticate(t.Context(), BioPrompt{Reason: "r"}), core.ErrUnsupported)

	require.ErrorIs(t, (&Biometric{}).Authenticate(t.Context(), BioPrompt{Reason: "r"}), core.ErrInvalid, "no dispatch")
}

func TestBiometricAuthenticateContext(t *testing.T) {
	t.Parallel()

	t.Run("canceled while showing", func(t *testing.T) {
		t.Parallel()

		sheet := newFakeSheet()
		b := NewBiometric(uiLoop(t))
		b.start = sheet.start
		ctx, cancel := context.WithCancel(t.Context())
		res := make(chan error, 1)
		go func() { res <- b.Authenticate(ctx, BioPrompt{Reason: "r"}) }()
		<-sheet.shown
		cancel()
		require.ErrorIs(t, <-res, context.Canceled)
		<-sheet.dropped // the sheet is dismissed
		assert.Equal(t, int32(1), sheet.canceled.Load())
	})
	t.Run("canceled before", func(t *testing.T) {
		t.Parallel()

		sheet := newFakeSheet()
		b := NewBiometric(uiLoop(t))
		b.start = sheet.start
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, b.Authenticate(ctx, BioPrompt{Reason: "r"}), context.Canceled)
		select {
		case <-sheet.shown:
			t.Fatal("a sheet shown for an ended context")
		default:
		}
	})
	t.Run("canceled before it shows", func(t *testing.T) {
		t.Parallel()

		var r bioWait
		r.answer = make(chan error, 1)
		r.cancel()
		sheet := newFakeSheet()
		r.begin(t.Context(), sheet.start, func(fn func()) { fn() }, BioPrompt{Reason: "r"})
		select {
		case <-sheet.shown:
			t.Fatal("a sheet shown after its cancel")
		default:
		}
	})
}

func TestAuthenticateChecksItsArguments(t *testing.T) {
	t.Parallel()

	d := func(fn func()) { fn() }
	for _, tc := range []struct {
		name string
		d    Dispatch
		p    BioPrompt
		done func(error)
	}{
		{"no dispatch", nil, BioPrompt{Reason: "r"}, func(error) {}},
		{"no done", d, BioPrompt{Reason: "r"}, nil},
		{"no reason", d, BioPrompt{}, func(error) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cancel, err := Authenticate(tc.d, tc.p, tc.done)
			require.ErrorIs(t, err, core.ErrInvalid)
			assert.Nil(t, cancel)
		})
	}
}

func TestFakeBiometric(t *testing.T) {
	t.Parallel()

	f := NewFakeBiometric(BiometryFace)
	assert.Equal(t, BiometryFace, f.Kind())
	assert.True(t, f.Available())
	p := BioPrompt{Reason: "r"}
	require.NoError(t, f.Authenticate(t.Context(), p))
	assert.Equal(t, p, f.Prompt())

	f.Answer(ErrBioCanceled, ErrBioLockout)
	require.ErrorIs(t, f.Authenticate(t.Context(), p), context.Canceled)
	require.ErrorIs(t, f.Authenticate(t.Context(), p), ErrBioLockout)
	require.NoError(t, f.Authenticate(t.Context(), p))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, f.Authenticate(ctx, p), context.Canceled)

	f.SetKind(BiometryNone)
	assert.False(t, f.Available())
	require.ErrorIs(t, f.Authenticate(t.Context(), p), ErrBioUnavailable)
	assert.Equal(t, 6, f.Calls())
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// uiLoop returns a Dispatch running its functions in order on one
// goroutine, the test's UI thread, stopped at the test's end.
func uiLoop(t *testing.T) Dispatch {
	t.Helper()
	fns := make(chan func(), 16)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case fn := <-fns:
				fn()
			case <-stop:
				return
			}
		}
	}()
	t.Cleanup(func() {
		close(stop)
		<-done
	})

	return func(fn func()) {
		select {
		case fns <- fn:
		case <-stop:
		}
	}
}

func newFakeSheet() *fakeSheet {
	return &fakeSheet{shown: make(chan struct{}), dropped: make(chan struct{})}
}

// start is Authenticate's stand-in.
func (s *fakeSheet) start(d Dispatch, p BioPrompt, done func(error)) (func(), error) {
	if s.refuse != nil {
		return nil, s.refuse
	}
	s.mu.Lock()
	s.prompt, s.done, s.d = p, done, d
	s.mu.Unlock()
	close(s.shown)

	return func() {
		if s.canceled.Add(1) == 1 {
			close(s.dropped)
		}
		s.answer(ErrBioCanceled)
	}, nil
}

func (s *fakeSheet) got() BioPrompt {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.prompt
}

// answer hands err to done on the UI thread, once.
func (s *fakeSheet) answer(err error) {
	s.mu.Lock()
	done, d := s.done, s.d
	s.done = nil
	s.mu.Unlock()
	if done != nil {
		d(func() { done(err) })
	}
}
