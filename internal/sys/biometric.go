package sys

import (
	"context"
	"fmt"
	"sync"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// BiometryKind is the biometry the device offers for sign-in (03 §3 K13:
// "Đăng nhập bằng Face ID" or "… Vân tay").
type BiometryKind int

const (
	// BiometryUnknown is not known: the zero value.
	BiometryUnknown BiometryKind = iota
	// BiometryNone is no usable biometry: no sensor, nothing enrolled,
	// locked out, no passcode, an Android older than 9, or (iOS) Face ID
	// without NSFaceIDUsageDescription in Info.plist.
	BiometryNone
	// BiometryFace is Face ID, or Android's face unlock.
	BiometryFace
	// BiometryFingerprint is Touch ID, or Android's fingerprint sensor.
	BiometryFingerprint
	// BiometryOther is another biometry: Optic ID, Android's iris.
	BiometryOther
)

// BioPrompt is what the system's biometric sheet shows.
type BioPrompt struct {
	// Reason says why the app asks ("Đăng nhập DanceFITVN"): iOS's
	// localizedReason, Android's title when Title is empty. Required.
	Reason string
	// Title is Android's title, Reason then its description; "" makes
	// Reason the title alone. iOS ignores it (Face ID names itself).
	Title string
	// Cancel is the cancel button's text; "" is the system's ("Cancel"
	// in the device's language).
	Cancel string
}

// Biometrics is the device's biometric check as the app's services use
// it: *Biometric, and FakeBiometric in tests.
type Biometrics interface {
	// Kind returns the biometry usable now; BiometryNone when there is
	// none. Any goroutine; it asks the system each time (cheap).
	Kind() BiometryKind
	// Available tells Kind is Face, Fingerprint or Other.
	Available() bool
	// Authenticate shows the system's sheet and blocks until it answers:
	// nil when the user was recognized. Call it from a goroutine (a
	// ui.Go work function), never from the UI thread, which shows the
	// sheet. The errors are those of the package function Authenticate;
	// ctx's end dismisses the sheet and returns an error wrapping
	// ctx.Err().
	Authenticate(ctx context.Context, p BioPrompt) error
}

// The answers of a biometric check other than success. Each wraps a
// sentinel of core or context.Canceled, so callers test with errors.Is.
var (
	// ErrBioCanceled is the sheet dismissed: the user's Cancel, Back, the
	// system (another app came forward, a call), the app (cancel), or the
	// Android bridge's timeout.
	ErrBioCanceled = fmt.Errorf("sys: biometric: canceled: %w", context.Canceled)
	// ErrBioFallback is iOS's fallback button ("Enter Password"). The
	// sheet shows none (the login form is the fallback), but iOS may.
	ErrBioFallback = fmt.Errorf("sys: biometric: the user chose the fallback: %w", context.Canceled)
	// ErrBioFailed is the user not recognized: iOS after its own tries,
	// Android after failedLimit tries (03 §3 "sai 3 lần").
	ErrBioFailed = fmt.Errorf("sys: biometric: not recognized: %w", core.ErrInvalid)
	// ErrBioLockout is the biometry locked after too many tries; the
	// device's passcode unlocks it.
	ErrBioLockout = fmt.Errorf("sys: biometric: locked out: %w", core.ErrInvalid)
	// ErrBioUnavailable is no biometry to check with: no sensor,
	// nothing enrolled, no passcode, the usage sentence missing.
	ErrBioUnavailable = fmt.Errorf("sys: biometric: not available: %w", core.ErrUnsupported)
)

// failedLimit is how many unrecognized tries the Android bridge allows
// before it dismisses the sheet with ErrBioFailed (03 §3 "sai 3 lần";
// iOS counts by itself). BiometricBridge.FAILED_TRIES has the same value.
const failedLimit = 3 //nolint:unused // mirrors BiometricBridge.FAILED_TRIES (android/java), read by no Go file

// The LAError codes of LocalAuthentication's LAError.h (iOS 17 SDK).
const (
	laAuthenticationFailed = -1
	laUserCancel           = -2
	laUserFallback         = -3
	laSystemCancel         = -4
	laPasscodeNotSet       = -5
	laBiometryNotAvailable = -6
	laBiometryNotEnrolled  = -7
	laBiometryLockout      = -8
	laAppCancel            = -9
	laNotInteractive       = -1004
)

// The LABiometryType values of LAContext.biometryType.
const (
	laBiometryNone    = 0
	laBiometryTouchID = 1
	laBiometryFaceID  = 2
	laBiometryOpticID = 4
)

// The statuses BiometricBridge.Listener.onResult reports.
const (
	bridgeSucceeded   = 0 // recognized
	bridgeError       = 1 // onAuthenticationError: code is BIOMETRIC_ERROR_*
	bridgeFailedLimit = 2 // failedLimit tries not recognized: the bridge dismissed the sheet
	bridgeNotShown    = 3 // the sheet could not be shown: message is the exception
)

// The codes of android.hardware.biometrics.BiometricPrompt's
// BIOMETRIC_ERROR_* (javap -constants, android-35).
const (
	promptHWUnavailable  = 1
	promptUnableProcess  = 2
	promptTimeout        = 3
	promptNoSpace        = 4
	promptCanceled       = 5
	promptLockout        = 7
	promptVendor         = 8
	promptLockoutForever = 9
	promptUserCanceled   = 10
	promptNoBiometrics   = 11
	promptHWNotPresent   = 12
	promptNegativeButton = 13
	promptNoCredential   = 14
	promptSecurityUpdate = 15
)

// The answers of BiometricBridge.availability: BiometricManager's
// BIOMETRIC_SUCCESS and BIOMETRIC_ERROR_*, and the bridge's own
// availabilityTooOld below API 28.
const (
	availabilitySuccess = 0
	availabilityTooOld  = -1
)

// The bits of BiometricBridge.features: the system features the device
// declares.
const (
	featureFingerprint = 1
	featureFace        = 2
	featureIris        = 4
)

// Biometric is the device's biometric check (F6, until gDesk's G3): the
// package's Biometry and Authenticate with a blocking Authenticate for
// the app's services. Make it with NewBiometric.
type Biometric struct {
	d Dispatch
	// start shows the sheet on the UI thread: Authenticate, replaced in
	// tests.
	start func(d Dispatch, p BioPrompt, done func(error)) (cancel func(), err error)
	// kind is Biometry, replaced in tests.
	kind func() (BiometryKind, error)
}

var _ Biometrics = (*Biometric)(nil)

// FakeBiometric is a Biometrics for tests: a kind and a queue of answers
// set by the test. Safe for concurrent use.
type FakeBiometric struct {
	mu      sync.Mutex
	kind    BiometryKind
	answers []error
	calls   int
	prompt  BioPrompt
}

var _ Biometrics = (*FakeBiometric)(nil)

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewBiometric returns the device's biometric check; d is the app's
// host.App.Enqueue, which runs the sheet's calls on the UI thread.
func NewBiometric(d Dispatch) *Biometric {
	return &Biometric{d: d, start: Authenticate, kind: Biometry}
}

// NewFakeBiometric returns a FakeBiometric of kind whose Authenticate
// answers nil until Answer queues something else.
func NewFakeBiometric(kind BiometryKind) *FakeBiometric {
	return &FakeBiometric{kind: kind}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String names the kind, for logs.
func (k BiometryKind) String() string {
	switch k {
	case BiometryNone:
		return "none"
	case BiometryFace:
		return "face"
	case BiometryFingerprint:
		return "fingerprint"
	case BiometryOther:
		return "other"
	case BiometryUnknown:
		return "unknown"
	default:
		return fmt.Sprintf("BiometryKind(%d)", int(k))
	}
}

// Usable tells k is a biometry the sheet can check with.
func (k BiometryKind) Usable() bool {
	switch k {
	case BiometryFace, BiometryFingerprint, BiometryOther:
		return true
	case BiometryUnknown, BiometryNone:
		return false
	default:
		return false
	}
}

// Biometry returns the biometry usable now, from any goroutine. The
// error wraps core.ErrUnsupported where the system has no biometric
// sheet (tvOS, desktop; Android before 9 is BiometryNone); ErrBioLockout
// with the kind when the biometry is locked out. On iOS a Face ID or
// Optic ID device whose Info.plist lacks NSFaceIDUsageDescription is
// BiometryNone (the sheet would fail), so the app hides its button.
func Biometry() (BiometryKind, error) { return biometry() }

// Authenticate shows the system's biometric sheet with p and returns at
// once; done runs once, later, on the UI thread through d: nil when the
// user was recognized; an error wrapping context.Canceled when the
// sheet was dismissed (ErrBioCanceled, ErrBioFallback); ErrBioFailed,
// ErrBioLockout, ErrBioUnavailable; another error wrapping core.ErrInvalid
// for a code the package does not know. cancel dismisses the sheet (done
// then gets ErrBioCanceled); after done it does nothing. Call both on the
// UI thread. err (done then never runs) is a refused call: no Reason, no
// d or done (core.ErrInvalid), no system sheet (core.ErrUnsupported).
func Authenticate(d Dispatch, p BioPrompt, done func(error)) (cancel func(), err error) {
	switch {
	case d == nil || done == nil:
		return nil, fmt.Errorf("sys: biometric: no dispatch or done: %w", core.ErrInvalid)
	case p.Reason == "":
		return nil, fmt.Errorf("sys: biometric: no reason: %w", core.ErrInvalid)
	default:
	}

	return authenticate(d, p, done)
}

// Kind returns Biometry's kind, BiometryNone on an error.
func (b *Biometric) Kind() BiometryKind {
	k, err := b.kind()
	if err != nil {
		return BiometryNone
	}

	return k
}

// Available tells Kind is usable.
func (b *Biometric) Available() bool { return b.Kind().Usable() }

// Authenticate shows the sheet on the UI thread and waits for its
// answer (see the package's Authenticate for the errors). ctx's end
// dismisses the sheet and returns at once with an error wrapping
// ctx.Err(). Never call it from the UI thread: the sheet's answer comes
// there, and it would wait for itself.
func (b *Biometric) Authenticate(ctx context.Context, p BioPrompt) error {
	if b.d == nil {
		return fmt.Errorf("sys: biometric: no dispatch: %w", core.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sys: biometric: %w", err)
	}
	r := &bioWait{answer: make(chan error, 1)}
	b.d(func() { r.begin(ctx, b.start, b.d, p) })
	select {
	case err := <-r.answer:
		return err
	case <-ctx.Done():
		b.d(r.cancel)

		return fmt.Errorf("sys: biometric: %w", ctx.Err())
	}
}

// Kind returns the kind set.
func (f *FakeBiometric) Kind() BiometryKind {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.kind
}

// Available tells Kind is usable.
func (f *FakeBiometric) Available() bool { return f.Kind().Usable() }

// Authenticate records p and returns the next answer queued (nil when
// none is), ErrBioUnavailable when the kind is not usable, or an error
// wrapping ctx.Err() when ctx ended.
func (f *FakeBiometric) Authenticate(ctx context.Context, p BioPrompt) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.prompt = p
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("sys: biometric: %w", err)
	}
	if !f.kind.Usable() {
		return ErrBioUnavailable
	}
	if len(f.answers) == 0 {
		return nil
	}
	err := f.answers[0]
	f.answers = f.answers[1:]

	return err
}

// SetKind changes the kind.
func (f *FakeBiometric) SetKind(k BiometryKind) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kind = k
}

// Answer queues the answers of the next calls of Authenticate, in order
// (nil for a success).
func (f *FakeBiometric) Answer(errs ...error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, errs...)
}

// Calls returns how many times Authenticate was called.
func (f *FakeBiometric) Calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

// Prompt returns the prompt of the last call.
func (f *FakeBiometric) Prompt() BioPrompt {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.prompt
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// bioWait is one blocking Authenticate: its answer and the sheet's
// cancel. begin and cancel run on the UI thread, in that order (Dispatch
// keeps the order of what it is given).
type bioWait struct {
	answer chan error // buffered: the one answer never blocks
	stop   func()     // the sheet's cancel, nil until it shows
	ended  bool       // canceled before it showed
}

// begin shows the sheet, unless ctx ended or cancel ran before.
func (r *bioWait) begin(ctx context.Context, start func(Dispatch, BioPrompt, func(error)) (func(), error), d Dispatch, p BioPrompt) {
	if r.ended || ctx.Err() != nil {
		return
	}
	stop, err := start(d, p, func(err error) { r.answer <- err })
	if err != nil {
		r.answer <- err

		return
	}
	r.stop = stop
}

// cancel dismisses the sheet; one not shown yet will not be.
func (r *bioWait) cancel() {
	r.ended = true
	if r.stop != nil {
		r.stop()
	}
}

// laKind returns the kind of an LABiometryType.
func laKind(t int64) BiometryKind {
	switch t {
	case laBiometryFaceID:
		return BiometryFace
	case laBiometryTouchID:
		return BiometryFingerprint
	case laBiometryOpticID:
		return BiometryOther
	case laBiometryNone:
		return BiometryNone
	default:
		return BiometryOther
	}
}

// laNeedsUsage tells whether the kind needs NSFaceIDUsageDescription in
// Info.plist: Face ID (and Optic ID, which reads the same key).
func laNeedsUsage(k BiometryKind) bool { return k == BiometryFace || k == BiometryOther }

// laError returns the error of an LAError code: nil for 0.
func laError(code int64, text string) error {
	switch code {
	case 0:
		return nil
	case laUserCancel, laSystemCancel, laAppCancel, laNotInteractive:
		return ErrBioCanceled
	case laUserFallback:
		return ErrBioFallback
	case laAuthenticationFailed:
		return ErrBioFailed
	case laBiometryLockout:
		return ErrBioLockout
	case laPasscodeNotSet, laBiometryNotAvailable, laBiometryNotEnrolled:
		return ErrBioUnavailable
	default:
		return fmt.Errorf("sys: biometric: LAError %d: %s: %w", code, text, core.ErrInvalid)
	}
}

// bridgeResult returns the error of BiometricBridge.Listener.onResult's
// arguments: nil for a success.
func bridgeResult(status, code int32, message string) error {
	switch status {
	case bridgeSucceeded:
		return nil
	case bridgeFailedLimit:
		return ErrBioFailed
	case bridgeError:
		return promptError(code, message)
	case bridgeNotShown:
		return fmt.Errorf("sys: biometric: the sheet was not shown: %s: %w", message, core.ErrInvalid)
	default:
		return fmt.Errorf("sys: biometric: bridge status %d: %w", status, core.ErrInvalid)
	}
}

// promptError returns the error of a BIOMETRIC_ERROR_* code.
func promptError(code int32, message string) error {
	switch code {
	case promptCanceled, promptUserCanceled, promptNegativeButton, promptTimeout:
		return ErrBioCanceled
	case promptLockout, promptLockoutForever:
		return ErrBioLockout
	case promptHWUnavailable, promptNoBiometrics, promptHWNotPresent, promptNoCredential, promptSecurityUpdate:
		return ErrBioUnavailable
	case promptUnableProcess, promptNoSpace, promptVendor:
		return fmt.Errorf("sys: biometric: BiometricPrompt error %d: %s: %w", code, message, core.ErrInvalid)
	default:
		return fmt.Errorf("sys: biometric: BiometricPrompt error %d: %s: %w", code, message, core.ErrInvalid)
	}
}

// androidKind returns the kind of BiometricBridge.availability and
// .features: none unless the check succeeds; then the fingerprint first
// (Android does not tell which biometry is enrolled), the face, the iris.
func androidKind(availability, features int32) BiometryKind {
	switch {
	case availability != availabilitySuccess: // availabilityTooOld too
		return BiometryNone
	case features&featureFingerprint != 0:
		return BiometryFingerprint
	case features&featureFace != 0:
		return BiometryFace
	case features&featureIris != 0:
		return BiometryOther
	default:
		return BiometryOther
	}
}
