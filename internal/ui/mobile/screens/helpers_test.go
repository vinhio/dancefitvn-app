//go:build !tvos

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/kit/layoutdump"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// meEmail is the email of the mock user (me/me.json), which M02 shows in the
// field.
const meEmail = "thaovy.hlv@gmail.com"

// setup prepares the Fake before the Shell mounts.
type setup func(t *testing.T, f *shelltest.Fake)

// newRig mounts the Shell on the phone over a new Fake, the setups run first;
// real asks for the phone's own text engine.
func newRig(t *testing.T, ph kittest.Phone, real bool, o shell.Options, setups ...setup) *shelltest.Rig {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	for _, s := range setups {
		s(t, f)
	}

	return shelltest.MountRig(t, ph, real, f, o)
}

// bio makes the device offer a biometry and a slot, ending as result says.
func bio(kind sys.BiometryKind, result error) setup {
	return func(_ *testing.T, f *shelltest.Fake) { f.SetBiometry(kind, result) }
}

// lastEmail makes the device remember an email.
func lastEmail(s string) setup {
	return func(_ *testing.T, f *shelltest.Fake) { f.SetLastEmail(s) }
}

// signed puts a session in the vault: a launch that finds the user in.
func signed() setup {
	return func(t *testing.T, f *shelltest.Fake) { require.NoError(t, f.SeedSession()) }
}

// atLogin boots an unsigned app to the login screen.
func atLogin(t *testing.T, ph kittest.Phone, real bool, setups ...setup) (*shelltest.Rig, *screens.Login) {
	t.Helper()
	r := newRig(t, ph, real, shell.Options{}, setups...)
	r.Boot()
	require.Equal(t, "login", r.Route())
	lg, ok := r.Shell.Nav().Screen().(*screens.Login)
	require.True(t, ok)

	return r, lg
}

// focused returns what holds the focus.
func focused(r *shelltest.Rig) ui.Widget { return r.Tester.View().Focused() }

// checkAgainstProto asserts that every element of the prototype's capture of id
// is in the dump within tol DIPs (the keys in skip left out), listing the ones
// that are not.
func checkAgainstProto(t *testing.T, id string, lay layoutdump.Layout, tol float64, skip map[string]bool) {
	t.Helper()
	want := shelltest.ProtoLayout(t, id)
	for _, d := range shelltest.Compare(want, lay, skip) {
		if d.Missing {
			t.Errorf("%s: %s is missing", id, d.Key)

			continue
		}
		assert.LessOrEqualf(t, d.Max(), tol, "%s: %s is off by dx %+.2f dy %+.2f dw %+.2f dh %+.2f", id, d.Key, d.DX, d.DY, d.DW, d.DH)
	}
}

// checkAgainstProtoWithout is checkAgainstProto for a screen that has lost elements
// of the prototype on purpose: drop names their keys in the capture, and the rest
// is compared under the numbers the app gives them (shelltest.Without).
func checkAgainstProtoWithout(t *testing.T, id string, lay layoutdump.Layout, tol float64, skip map[string]bool, drop ...string) {
	t.Helper()
	want := shelltest.Without(shelltest.ProtoLayout(t, id), drop...)
	for _, d := range shelltest.Compare(want, lay, skip) {
		if d.Missing {
			t.Errorf("%s: %s is missing", id, d.Key)

			continue
		}
		assert.LessOrEqualf(t, d.Max(), tol, "%s: %s is off by dx %+.2f dy %+.2f dw %+.2f dh %+.2f", id, d.Key, d.DX, d.DY, d.DW, d.DH)
	}
}

// videoSideDrop are the keys of the capture of screen 06 (and of 07, which lies
// over it) that K38 took away: the speed, mirror and A-B loop buttons of the side
// column, their icons, and "sheet:options#2", the speed button.
var videoSideDrop = []string{"sheet:options#2", "mirror", "loop", "svg.i#3", "svg.i#4", "svg.i#5"}

// heroDrop are the keys of the prototype's captures of the course detail (M05, and
// M09 and M-toast, which lie over it) that the owner took away: the hero's "…" button
// and its icon (it has no action in the prototype, index.html:398).
var heroDrop = []string{"button.rb.dk", "svg.i#2"}

// tap taps the middle of w, as a finger does.
func tap(r *shelltest.Rig, w ui.Widget) {
	r.Tester.ClickAt(w.Node().WindowFrame().Center())
}

// shellOptions is the Shell's default options.
func shellOptions() shell.Options { return shell.Options{} }
