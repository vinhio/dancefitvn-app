//go:build !tvos

package screens_test

import (
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of K31 (a sign-in at the limit of devices).
const (
	limitTitle  = "Đã đạt giới hạn thiết bị"
	limitSub    = "Gỡ bớt một thiết bị để tiếp tục"
	removeLogin = "Gỡ và đăng nhập"
)

// replaceInvalid is the answer of a server that does not accept replace_device_id: the
// 400 validation_failed of docs K31, with the field named.
const replaceInvalid = `{"error":{"code":"validation_failed","message":"Thông tin chưa hợp lệ","fields":{"replace_device_id":"Thiết bị không hợp lệ"}}}`

// atLoginLimit boots to the login, signs in with the mock at its limit of devices
// (409 device_limit) and waits for the sheet of devices the screen opens (K31).
func atLoginLimit(tb testing.TB, ph kittest.Phone, f *shelltest.Fake) (*shelltest.Rig, *screens.Login, *screens.DevicesSheet) {
	tb.Helper()
	f.SetLastEmail(meEmail)
	r := shelltest.MountRig(tb, ph, false, f, shell.Options{})
	r.Boot()
	require.Equal(tb, "login", r.Route())
	lg, ok := r.Shell.Nav().Screen().(*screens.Login)
	require.True(tb, ok)
	require.NoError(tb, f.SetScenario("auth.login", "device_limit"))
	lg.Submit("secret1")
	r.Settle()
	slideIn(r)
	require.True(tb, r.Shell.Nav().Sheet().IsOpen(), "the sheet of devices opens")
	s, ok := r.Shell.Nav().Sheet().Content().(*screens.DevicesSheet)
	require.True(tb, ok, "the content is the sheet of devices")

	return r, lg, s
}

// newFake returns a Fake over the mock, with files overriding the mock's own.
func newFake(tb testing.TB, files map[string]string) *shelltest.Fake {
	tb.Helper()
	f, err := shelltest.New(overlayFS{FS: shelltest.MockFS(tb), files: files})
	require.NoError(tb, err)

	return f
}

// TestLoginDeviceLimitOpensTheSheet checks K31 on both phones: the 409 shows the banner
// and opens the sheet of devices with the phones of the error (the TV is not offered:
// the server revokes a device of the same kind), the most recent first, nothing chosen,
// the red button reading "Gỡ và đăng nhập" and not yet pressable; the email and the
// password stay in their fields.
func TestLoginDeviceLimitOpensTheSheet(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg, s := atLoginLimit(t, ph, newFake(t, nil))
		title, sub, shown := lg.Banner()
		assert.True(t, shown)
		assert.Equal(t, limitTitle, title)
		assert.Equal(t, limitSub, sub)
		assert.True(t, s.Login())
		rows := s.Rows()
		require.Len(t, rows, 2, "the TV d_201 is not a choice")
		assert.Equal(t, "devChoice:d_101", rows[0].Name)
		assert.Equal(t, "devChoice:d_102", rows[1].Name)
		for _, text := range []string{limitTitle, limitSub, "iPhone 15", "Samsung Galaxy A17",
			"Hoạt động 14/11/2026", "Hoạt động 12/11/2026", removeLogin, "Huỷ"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		assert.False(t, r.Tester.Shows("Apple TV phòng khách"))
		assert.Empty(t, s.Chosen(), "a choice revokes a device: the user makes it")
		assert.True(t, s.RemoveButton().Disabled)
		assert.Equal(t, removeLogin, s.RemoveButton().Text)
		for _, row := range rows {
			assert.False(t, row.Disabled, "no device here is this phone")
			assert.False(t, row.On)
		}
		assert.Equal(t, "secret1", lg.Password().Text())
		assert.Equal(t, meEmail, lg.Email().Text())
		assert.False(t, lg.Password().Error)
		assert.Equal(t, []string{""}, r.Fake.LoginReplaced())
		assert.Equal(t, 1, r.Fake.Calls("auth.login"))
	})
}

// TestLoginDeviceLimitPickAndSignIn checks the flow of K31 on both phones: a device is
// picked, "Gỡ và đăng nhập" sends the same login again with replace_device_id, the sheet
// closes and the sign-in goes on as any other (Home, the stack empty, the session kept).
func TestLoginDeviceLimitPickAndSignIn(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for i, id := range []string{"d_101", "d_102"} {
			t.Run(id, func(t *testing.T) {
				r, _, s := atLoginLimit(t, ph, newFake(t, nil))
				tap(r, s.Rows()[i])
				assert.Equal(t, id, s.Chosen())
				assert.True(t, s.Rows()[i].On)
				assert.False(t, s.RemoveButton().Disabled)
				tap(r, s.RemoveButton())
				r.Settle()
				slideIn(r)
				assert.Equal(t, []string{"", id}, r.Fake.LoginReplaced(), "the same login, with the device to replace")
				assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet closes")
				assert.Equal(t, "home", r.Route())
				assert.Equal(t, 0, r.Shell.Nav().Depth())
				assert.True(t, r.Fake.Signed())
				assert.Equal(t, meEmail, r.Fake.LastEmail())
			})
		}
	})
}

// TestLoginDeviceLimitCancel checks that leaving the sheet (the cross, "Huỷ", Back) returns
// to the form as it was: the banner, the email and the password kept, nothing sent again; a
// tap on the banner asks again, which opens the sheet with a fresh list.
func TestLoginDeviceLimitCancel(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, how := range []string{"Huỷ", "cross", "back"} {
			t.Run(how, func(t *testing.T) {
				r, lg, s := atLoginLimit(t, ph, newFake(t, nil))
				switch how {
				case "Huỷ":
					require.NoError(t, r.Tester.Click("Huỷ"))
				case "cross":
					tapKey(t, r, "closeSheet")
				default:
					r.Shell.Back() // Back closes the sheet
				}
				r.Settle()
				slideIn(r)
				assert.False(t, r.Shell.Nav().Sheet().IsOpen())
				assert.Equal(t, "login", r.Route())
				_, _, shown := lg.Banner()
				assert.True(t, shown, "the banner stays")
				assert.Equal(t, "secret1", lg.Password().Text(), "the password is kept")
				assert.Equal(t, meEmail, lg.Email().Text())
				assert.Equal(t, []string{""}, r.Fake.LoginReplaced(), "nothing is sent again")
				assert.False(t, r.Fake.Signed())
				assert.False(t, s.Busy())

				// the banner is the way back to the sheet
				tapKey(t, r, "div.muted")
				r.Settle()
				slideIn(r)
				assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "a tap on the banner asks again")
				assert.Equal(t, []string{"", ""}, r.Fake.LoginReplaced())
			})
		}
	})
}

// TestLoginDeviceLimitReplaceRefused checks the 400 on replace_device_id: the sheet closes
// with the toast "Thông tin chưa hợp lệ", the banner stays for another go with a fresh list,
// the password is kept and the user is not signed in.
func TestLoginDeviceLimitReplaceRefused(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, map[string]string{"errors/replace_invalid.json": replaceInvalid})
		r, lg, s := atLoginLimit(t, ph, f)
		f.Fail("auth.login", 400, "errors/replace_invalid.json")
		tap(r, s.Rows()[0])
		tap(r, s.RemoveButton())
		r.Settle()
		slideIn(r)
		assert.Equal(t, []string{"", "d_101"}, f.LoginReplaced())
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, shows(r, "Thông tin chưa hợp lệ"), "the toast")
		assert.Equal(t, "login", r.Route())
		_, _, shown := lg.Banner()
		assert.True(t, shown, "the banner stays")
		assert.Equal(t, "secret1", lg.Password().Text())
		assert.False(t, f.Signed())

		// another go: the server lists the devices again and the sheet is back
		f.Fail("auth.login", 0, "")
		tapKey(t, r, "div.muted")
		r.Settle()
		slideIn(r)
		assert.True(t, r.Shell.Nav().Sheet().IsOpen())
	})
}

// TestLoginDeviceLimitOffline checks that no network leaves the sheet open for another try,
// with the toast "Không kết nối được, thử lại sau", and that the same press then signs in.
func TestLoginDeviceLimitOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, nil)
		r, _, s := atLoginLimit(t, ph, f)
		tap(r, s.Rows()[1])
		f.SetOnline(false)
		tap(r, s.RemoveButton())
		r.Settle()
		assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
		assert.False(t, s.Busy())
		assert.Equal(t, "d_102", s.Chosen(), "so does the choice")
		assert.True(t, shows(r, "Không kết nối được, thử lại sau"))
		f.SetOnline(true)
		r.Tester.Elapse(2300 * time.Millisecond) // the toast goes
		tap(r, s.RemoveButton())
		r.Settle()
		slideIn(r)
		assert.Equal(t, "home", r.Route())
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Equal(t, []string{"", "d_102"}, f.LoginReplaced()[:2], "the offline try never reached the server")
	})
}

// TestLoginDeviceLimitWrongPassword checks that a password the server then refuses (changed
// meanwhile) is the form's: the sheet closes, the banner says it, the field is marked.
func TestLoginDeviceLimitWrongPassword(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, nil)
		r, lg, s := atLoginLimit(t, ph, f)
		tap(r, s.Rows()[0])
		require.NoError(t, f.SetScenario("auth.login", "invalid_credentials"))
		tap(r, s.RemoveButton())
		r.Settle()
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		title, _, shown := lg.Banner()
		assert.True(t, shown)
		assert.Equal(t, "Email hoặc mật khẩu chưa đúng", title)
		assert.True(t, lg.Password().Error)
		assert.Empty(t, lg.Password().Text())
		assert.Equal(t, "login", r.Route())
	})
}

// TestLoginDeviceLimitAgain checks that a limit the server answers again (the list changed
// under the user) rebuilds the sheet with the new list, open, with no choice.
func TestLoginDeviceLimitAgain(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, nil)
		r, _, s := atLoginLimit(t, ph, f)
		f.Fail("auth.login", 409, "errors/device_limit.json") // every login is refused, whatever it replaces
		tap(r, s.Rows()[0])
		tap(r, s.RemoveButton())
		r.Settle()
		assert.True(t, r.Shell.Nav().Sheet().IsOpen())
		assert.False(t, s.Busy())
		assert.Len(t, s.Rows(), 2)
		assert.Empty(t, s.Chosen())
		assert.True(t, s.RemoveButton().Disabled)
		assert.False(t, f.Signed())
	})
}

// TestLoginDeviceLimitOnlyTVs checks that a limit whose list has no phone leaves the banner
// alone (nothing to choose), and the password kept.
func TestLoginDeviceLimitOnlyTVs(t *testing.T) {
	t.Parallel()

	const onlyTVs = `{"error":{"code":"device_limit","message":"m","devices":[{"id":"d_201","platform":"tvos","name":"Apple TV phòng khách"}]}}`
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, map[string]string{"errors/only_tvs.json": onlyTVs})
		f.SetLastEmail(meEmail)
		f.Fail("auth.login", 409, "errors/only_tvs.json")
		r := shelltest.MountRig(t, ph, false, f, shell.Options{})
		r.Boot()
		lg := r.Shell.Nav().Screen().(*screens.Login)
		lg.Submit("secret1")
		r.Settle()
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		title, _, shown := lg.Banner()
		assert.True(t, shown)
		assert.Equal(t, limitTitle, title)
		assert.Equal(t, "secret1", lg.Password().Text())
	})
}

// TestLoginBioDeviceLimit checks the biometric sign-in meeting the limit (the refresh holds no
// password to send again with a device to replace): the banner says it, the password takes the
// focus, and no sheet opens; the password's own sign-in then opens it.
func TestLoginBioDeviceLimit(t *testing.T) {
	t.Parallel()

	limit := &api.Error{Status: 409, Code: "device_limit", Body: model.APIError{Code: "device_limit", Devices: []model.Device{{ID: "d_101", Name: "iPhone 15"}}}}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, lg := atLogin(t, ph, false, bio(sys.BiometryFace, limit), lastEmail(meEmail))
		tap(r, lg.BioButton())
		r.Settle()
		title, sub, shown := lg.Banner()
		assert.True(t, shown)
		assert.Equal(t, limitTitle, title)
		assert.Equal(t, limitSub, sub)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Same(t, lg.Password().Field(), focused(r))
		assert.Equal(t, "login", r.Route())
		assert.False(t, r.Fake.Signed())

		require.NoError(t, r.Fake.SetScenario("auth.login", "device_limit"))
		lg.Submit("secret1")
		r.Settle()
		slideIn(r)
		assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "with the password the sheet opens")
	})
}

// TestLoginDeviceLimitCloseWhileBusy checks that the retry's request is the sheet's: a request
// that the close of the sheet cuts short does not sign in.
func TestLoginDeviceLimitCloseWhileBusy(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f := newFake(t, nil)
		r, _, s := atLoginLimit(t, ph, f)
		tap(r, s.Rows()[0])
		release := f.Gate("auth.login")
		tap(r, s.RemoveButton())
		assert.True(t, s.Busy(), "the request is out")
		r.Shell.Nav().Sheet().Close()
		release()
		r.Settle()
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.NotPanics(t, func() { r.Tester.Elapse(time.Second) })
		assert.Equal(t, "login", r.Route(), "a sign-in cut short does not go on")
	})
}

// TestReplaceableDevices checks the list a sign-in offers: phones only, the most recently seen
// first, devices with no day last, the order of the answer among equals.
func TestReplaceableDevices(t *testing.T) {
	t.Parallel()

	day := func(d int) time.Time { return time.Date(2026, 11, d, 9, 0, 0, 0, time.UTC) }
	in := []model.Device{
		{ID: "d_1", Platform: model.PlatformAndroid, LastSeenAt: day(10)},
		{ID: "d_2", Platform: model.PlatformTVOS, LastSeenAt: day(14)},
		{ID: "d_3", Platform: model.PlatformIOS},
		{ID: "d_4", Platform: model.PlatformIOS, LastSeenAt: day(12)},
		{ID: "d_5", Platform: model.PlatformUnknown, LastSeenAt: day(12)},
		{ID: "", Platform: model.PlatformIOS, LastSeenAt: day(13)},
	}
	var ids []string
	for _, d := range env.ReplaceableDevices(in) {
		ids = append(ids, d.ID)
	}
	assert.Equal(t, []string{"d_4", "d_5", "d_1", "d_3"}, ids)
	assert.Empty(t, env.ReplaceableDevices(nil))
}

// TestLoginDeviceSheetSteadyFrameDoesNotAllocate checks that a frame of the login with its sheet
// standing still allocates nothing, on both phones.
func TestLoginDeviceSheetSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, _, s := atLoginLimit(t, ph, newFake(t, nil))
		tap(r, s.Rows()[0])
		for range 5 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
