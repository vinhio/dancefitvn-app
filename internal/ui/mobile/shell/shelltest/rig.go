//go:build !tvos

package shelltest

import (
	"runtime"
	"testing"
	"time"

	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Rig is a Shell on a test phone over a Fake: what the screens' tests drive.
type Rig struct {
	// Phone is the phone it runs on.
	Phone kittest.Phone
	// Fake is the services behind it.
	Fake *Fake
	// Shell is the root widget.
	Shell *shell.Shell
	// Tester drives it.
	Tester *ui.Tester

	tb testing.TB
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewRig mounts a Shell on the phone over a new Fake of the repository's mock
// API. real asks for the phone's real text engine (boxes), else the fake shaper
// (behaviour). The motion is pinned on, so that animations run under the race
// detector too. The Shell is as New leaves it: the splash, the boot not yet run.
func NewRig(tb testing.TB, ph kittest.Phone, real bool, o shell.Options) *Rig {
	tb.Helper()
	f, err := New(MockFS(tb))
	if err != nil {
		tb.Fatalf("shelltest: %v", err)
	}

	return mountRig(tb, ph, real, f, o)
}

// MountRig is NewRig over a Fake the test prepared (a seeded session, a gate)
// before the boot starts.
func MountRig(tb testing.TB, ph kittest.Phone, real bool, f *Fake, o shell.Options) *Rig {
	tb.Helper()

	return mountRig(tb, ph, real, f, o)
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Route returns the id of the route showing.
func (r *Rig) Route() string { return r.Shell.Nav().Current().Screen }

// Settle waits for the work off the UI thread and runs its results, with the
// sign-outs the Fake posted (PostSignOuts) and what they started.
func (r *Rig) Settle() {
	for {
		r.Tester.RunUntilIdle()
		if r.Fake.RunPosted() == 0 {
			return
		}
	}
}

// Boot runs the boot to its end with the app config answering at once: the
// results of the session and config reads, then the splash's 600 ms.
func (r *Rig) Boot() {
	r.Settle()
	r.Tester.Elapse(600 * time.Millisecond)
	r.Settle()
}

// WaitFor runs the UI thread's posted work, a nanosecond of time at a time, until
// cond holds: the way to wait for the result of work that is not all done (a
// gate holds another request), where Settle would wait for the gate too. It
// fails the test when cond does not hold after a million turns.
func (r *Rig) WaitFor(cond func() bool, what string) {
	r.Tester.View() // the tester is open
	for range 1_000_000 {
		r.Fake.RunPosted()
		if cond() {
			return
		}
		r.Tester.Elapse(time.Nanosecond)
		runtime.Gosched()
	}
	r.tb.Fatalf("shelltest: %s did not happen", what)
}

// Dump records the layout of the screen showing, the way a shot does.
func (r *Rig) Dump(id string) layoutdump.Layout {
	r.Settle()

	return layoutdump.Dump(r.Tester.View().Context(), r.Tester.View().Root(), id, "mobile", r.Phone.Size, nil)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// mountRig mounts the Shell; the Fake posts its sign-outs to the Rig's UI
// thread from then (PostSignOuts).
func mountRig(tb testing.TB, ph kittest.Phone, real bool, f *Fake, o shell.Options) *Rig {
	tb.Helper()
	f.PostSignOuts()
	sh := shell.New(f, locales.MustTexts(), o)
	var tt *ui.Tester
	if real {
		tt = kittest.MountReal(tb, ph, sh)
	} else {
		tt = kittest.Mount(tb, ph, sh)
	}
	kittest.PinMotion(tt)

	return &Rig{Phone: ph, Fake: f, Shell: sh, Tester: tt, tb: tb}
}
