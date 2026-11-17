//go:build !tvos

package screens_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/require"
)

// atAccount boots a signed-in app, takes the prototype's start of M15 (the Apple TV
// not linked yet: `tvLinked:false`) and opens screen 15, the root of its tab. setups
// run on the Fake before the screen opens; real asks for the phone's own text
// engine.
func atAccount(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)

	return openAccount(tb, ph, real, f, setups...)
}

// openAccount is atAccount over a Fake that is already made.
func openAccount(tb testing.TB, ph kittest.Phone, real bool, f *shelltest.Fake, setups ...func(*shelltest.Fake)) *shelltest.Rig {
	tb.Helper()
	require.NoError(tb, f.SeedSession())
	f.Account().SetTVLinked(false)
	f.SetConfigEdit(noPrivacy)
	for _, s := range setups {
		s(f)
	}
	r := shelltest.MountRig(tb, ph, real, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteAccount, r.Route())

	return r
}

// atAccountLate is atAccount with a step that runs after the boot, before screen 15
// opens: a scenario of a route that the boot also reads.
func atAccountLate(tb testing.TB, ph kittest.Phone, late func(*shelltest.Fake)) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(shelltest.MockFS(tb))
	require.NoError(tb, err)
	require.NoError(tb, f.SeedSession())
	f.Account().SetTVLinked(false)
	f.SetConfigEdit(noPrivacy)
	r := shelltest.MountRig(tb, ph, false, f, shellOptions())
	r.Boot()
	require.Equal(tb, "home", r.Route())
	late(f)
	r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
	r.Tester.Elapse(kittest.Frame)
	r.Settle()

	return r
}

// scrollInto scrolls the page of the screen showing until the element key is over the
// tab bar (above the last 100 DIPs of the window), then returns.
func scrollInto(tb testing.TB, r *shelltest.Rig, sc *ui.Scroll, key string) {
	tb.Helper()
	e := element(tb, r.Dump("reveal"), key)
	if over := float32(e.Y+e.H) - (r.Phone.Size.H - 100); over > 0 {
		sc.ScrollBy(geom.Pt(0, over))
		r.Tester.Elapse(kittest.Frame)
	}
}

// noPrivacy empties links.privacy and links.terms of the app config: the account then has
// the rows of the prototype's M15 and nothing else (K32, K34), which the box tests compare
// with it.
func noPrivacy(c *model.AppConfig) { c.Links.Privacy, c.Links.Terms = "", "" }

// accountOf returns screen 15 showing.
func accountOf(tb testing.TB, r *shelltest.Rig) *screens.Account {
	tb.Helper()
	a, ok := r.Shell.Nav().Screen().(*screens.Account)
	require.True(tb, ok, "the screen showing is Account")

	return a
}

// atInfo opens screen 15B from screen 15, as a tap on its row does.
func atInfo(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) (*shelltest.Rig, *screens.Info) {
	tb.Helper()
	r := atAccount(tb, ph, real, setups...)

	return toInfo(tb, r)
}

// toInfo taps the row "Thông tin tài khoản" of screen 15.
func toInfo(tb testing.TB, r *shelltest.Rig) (*shelltest.Rig, *screens.Info) {
	tb.Helper()
	tapKey(tb, r, "go.info")
	r.Tester.Elapse(kittest.Frame)
	r.Settle()
	require.Equal(tb, env.RouteInfo, r.Route())
	in, ok := r.Shell.Nav().Screen().(*screens.Info)
	require.True(tb, ok, "the screen showing is Info")

	return r, in
}

// overlayFS answers the files of files instead of the mock's own: the answer the
// mock has no scenario for (`email_change_pending` set).
type overlayFS struct {
	fs.FS

	files map[string]string
}

// Open serves an overridden file from memory.
func (o overlayFS) Open(name string) (fs.File, error) {
	if body, ok := o.files[name]; ok {
		return fstest.MapFS{name: {Data: []byte(body)}}.Open(name)
	}

	return o.FS.Open(name)
}

// atAccountOver is atAccount over a mock whose files are overridden.
func atAccountOver(tb testing.TB, ph kittest.Phone, files map[string]string, setups ...func(*shelltest.Fake)) *shelltest.Rig {
	tb.Helper()
	f, err := shelltest.New(overlayFS{FS: shelltest.MockFS(tb), files: files})
	require.NoError(tb, err)

	return openAccount(tb, ph, false, f, setups...)
}

// texts is locales/vi.json, for tests that build an Env of their own.
func texts() map[string]string { return locales.MustTexts() }

// ctx is a context for the calls a test makes on the Fake directly.
func ctx() context.Context { return context.Background() }
