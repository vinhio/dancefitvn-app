//go:build !tvos

package env_test

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"
	"dancefitvn-app/locales"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noAccount is a Services whose own methods are hidden: only the interface's
// show through the embedded Services.
type noAccount struct{ env.Services }

// signedEnv returns an Env over a Fake with the session of the mock restored.
func signedEnv(t *testing.T) (*env.Env, *shelltest.Fake) {
	t.Helper()
	f := newFake(t)
	require.NoError(t, f.SeedSession())
	_, err := f.Restore(context.Background())
	require.NoError(t, err)

	return env.New(f, locales.MustTexts()), f
}

// waitCalls yields until n requests of the route reached the transport (the gate
// holds them there), without a sleep.
func waitCalls(t *testing.T, f *shelltest.Fake, route string, n int) {
	t.Helper()
	for range 10_000_000 {
		if f.Calls(route) >= n {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("no %d request of %s", n, route)
}

// TestAccountWithoutServicesFails checks that an Env with no services gives an
// Account that is never nil, whose every call fails with core.ErrUnsupported, and
// a view that stays empty.
func TestAccountWithoutServicesFails(t *testing.T) {
	t.Parallel()

	e := env.New(nil, locales.MustTexts())
	ctx := context.Background()
	a := e.Account()
	require.NotNil(t, a)
	_, ok := a.SessionUser()
	assert.False(t, ok)
	_, err := a.Me(ctx)
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.UpdateMe(ctx, model.MeUpdateRequest{})
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.ChangePassword(ctx, "a", "b")
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.Coach(ctx)
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.UpdateCoach(ctx, model.CoachUpdateRequest{})
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.UploadAvatar(ctx, []byte{1})
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.PickPhoto(ctx)
	require.ErrorIs(t, err, core.ErrUnsupported)
	_, err = a.Devices(ctx)
	require.ErrorIs(t, err, core.ErrUnsupported)
	require.ErrorIs(t, a.DeleteDevice(ctx, "d"), core.ErrUnsupported)
	_, err = a.PairConfirm(ctx, "7K4M2P")
	require.ErrorIs(t, err, core.ErrUnsupported)
	require.ErrorIs(t, a.Logout(ctx), core.ErrUnsupported)
	require.ErrorIs(t, a.BeginLogout()(ctx), core.ErrUnsupported)
	assert.Equal(t, env.AccountState{}, e.AccountView())
	_, linked := e.TVDevice()
	assert.False(t, linked)
	_, ok = e.AccountUser()
	assert.False(t, ok)
	assert.Same(t, a, e.Account(), "one port")
}

// TestAccountKeepsWhatItAnswers checks the view Env keeps for a screen that is
// built again: each read fills its part, each write changes it as the server
// said, a 404 on a device is success, a TV confirmed is listed.
func TestAccountKeepsWhatItAnswers(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("reads", func(t *testing.T) {
		t.Parallel()

		e, _ := signedEnv(t)
		v := e.AccountView()
		assert.False(t, v.HasMe || v.HasCoach || v.HasDevices)
		u, ok := e.AccountUser()
		require.True(t, ok, "the session's user before any /me")
		assert.NotEmpty(t, u.Email)

		_, err := e.Account().Me(ctx)
		require.NoError(t, err)
		_, err = e.Account().Coach(ctx)
		require.NoError(t, err)
		_, err = e.Account().Devices(ctx)
		require.NoError(t, err)
		v = e.AccountView()
		assert.True(t, v.HasMe && v.HasCoach && v.HasDevices)
		assert.Equal(t, "u_412", v.Me.User.ID)
		assert.Len(t, v.Me.Badges, 3)
		assert.Len(t, v.Coach.Socials, 3)
		assert.Len(t, v.Devices.Items, 3)
		tv, linked := e.TVDevice()
		require.True(t, linked)
		assert.Equal(t, "d_201", tv.ID)
		u, _ = e.AccountUser()
		assert.Equal(t, "u_412", u.ID, "GET /me replaces the session's user")
	})
	t.Run("the view is a copy", func(t *testing.T) {
		t.Parallel()

		e, _ := signedEnv(t)
		_, err := e.Account().Devices(ctx)
		require.NoError(t, err)
		v := e.AccountView()
		v.Devices.Items[0].Name = "changed"
		assert.NotEqual(t, "changed", e.AccountView().Devices.Items[0].Name)
	})
	t.Run("writes", func(t *testing.T) {
		t.Parallel()

		e, f := signedEnv(t)
		a := e.Account()
		_, err := a.Me(ctx)
		require.NoError(t, err)
		_, err = a.Coach(ctx)
		require.NoError(t, err)

		_, err = a.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "An", LastName: "Lê", Email: "an@example.com", Phone: "0911222333"})
		require.NoError(t, err)
		u, _ := e.AccountUser()
		assert.Equal(t, "An", u.FirstName)
		assert.Equal(t, "0911222333", u.Phone)

		_, err = a.UpdateCoach(ctx, model.CoachUpdateRequest{Bio: "Xin chào", Places: []string{"A"}})
		require.NoError(t, err)
		assert.Equal(t, "Xin chào", e.AccountView().Coach.Bio)

		av, err := a.UploadAvatar(ctx, shelltest.GeneratedPhoto(32, 32))
		require.NoError(t, err)
		assert.Equal(t, av.AvatarURL, e.AccountView().Me.User.AvatarURL)
		assert.Equal(t, 1, f.Account().Count("UploadAvatar"))
	})
	t.Run("failed writes change nothing", func(t *testing.T) {
		t.Parallel()

		e, f := signedEnv(t)
		a := e.Account()
		_, err := a.Me(ctx)
		require.NoError(t, err)
		require.NoError(t, f.SetScenario("me.update", "validation_failed"))
		_, err = a.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "", Email: "x"})
		require.Error(t, err)
		assert.Equal(t, env.FailureValidation, env.FailureOf(err).Kind)
		assert.Equal(t, "Thảo Vy", e.AccountView().Me.User.FirstName)
	})
	t.Run("devices", func(t *testing.T) {
		t.Parallel()

		e, f := signedEnv(t)
		a := e.Account()
		_, err := a.Devices(ctx)
		require.NoError(t, err)

		require.NoError(t, a.DeleteDevice(ctx, "d_201"))
		_, linked := e.TVDevice()
		assert.False(t, linked, "deleted from the view")

		// a device the server no longer has (404) is gone: success
		require.NoError(t, f.SetScenario("devices.delete", "not_found"))
		require.NoError(t, a.DeleteDevice(ctx, "d_102"))
		assert.Len(t, e.AccountView().Devices.Items, 1)

		// another failure stays a failure and the view is as it was
		f.Fail("devices.delete", 500, "errors/server_error.json")
		require.Error(t, a.DeleteDevice(ctx, "d_101"))
		assert.Len(t, e.AccountView().Devices.Items, 1)
		f.Fail("devices.delete", 0, "")

		res, err := a.PairConfirm(ctx, "7k4-m2p")
		require.NoError(t, err)
		assert.Equal(t, "d_201", res.Device.ID)
		tv, linked := e.TVDevice()
		require.True(t, linked, "a TV confirmed is listed")
		assert.Equal(t, model.PlatformTVOS, tv.Platform)
		_, err = a.PairConfirm(ctx, "7K4M2P")
		require.NoError(t, err)
		assert.Len(t, e.AccountView().Devices.Items, 2, "not listed twice")
	})
}

// TestAccountResetDropsLateAnswers checks that ResetAccount forgets the view, and
// that an answer that was on its way when it ran (the session before's request)
// is not kept for the next user.
func TestAccountResetDropsLateAnswers(t *testing.T) {
	t.Parallel()

	e, f := signedEnv(t)
	ctx := context.Background()
	_, err := e.Account().Me(ctx)
	require.NoError(t, err)
	require.True(t, e.AccountView().HasMe)
	e.ResetAccount()
	assert.Equal(t, env.AccountState{}, e.AccountView())

	release := f.Gate("devices.list")
	done := make(chan error, 1)
	go func() {
		_, err := e.Account().Devices(ctx)
		done <- err
	}()
	waitCalls(t, f, "devices.list", 1)
	e.ResetAccount() // the sign-out, while the request is in flight
	release()
	require.NoError(t, <-done)
	assert.False(t, e.AccountView().HasDevices, "the answer of the session before is dropped")

	_, err = e.Account().Devices(ctx)
	require.NoError(t, err)
	assert.True(t, e.AccountView().HasDevices, "a call begun after the reset is kept")
}

// TestFailureOf checks the classification of the errors of the account (P6 section
// 5): every scenario of the mock's me.*, devices.* and tv.pair.confirm, the
// network, the context, and the pairing's refusal of what is not a code.
func TestFailureOf(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tests := []struct {
		name  string
		route string
		scen  string
		call  func(a env.AccountServices) error
		want  env.FailureKind
		check func(t *testing.T, f env.Failure)
	}{
		{name: "validation_failed", route: "me.update", scen: "validation_failed", want: env.FailureValidation,
			call: func(a env.AccountServices) error { _, err := a.UpdateMe(ctx, model.MeUpdateRequest{}); return err },
			check: func(t *testing.T, f env.Failure) {
				assert.Contains(t, f.Fields, "first_name")
				assert.Contains(t, f.Fields, "email")
			}},
		{name: "wrong_password", route: "me.password", scen: "wrong_password", want: env.FailureWrongPassword,
			call:  func(a env.AccountServices) error { _, err := a.ChangePassword(ctx, "a", "bbbbbbbb"); return err },
			check: func(t *testing.T, f env.Failure) { assert.Contains(t, f.Fields, "current_password") }},
		{name: "file_too_large", route: "me.avatar", scen: "file_too_large", want: env.FailureFileTooLarge,
			call: func(a env.AccountServices) error { _, err := a.UploadAvatar(ctx, []byte{1, 2}); return err }},
		{name: "invalid_social", route: "me.coach.update", scen: "invalid_social", want: env.FailureInvalidSocial,
			call: func(a env.AccountServices) error {
				_, err := a.UpdateCoach(ctx, model.CoachUpdateRequest{})
				return err
			},
			check: func(t *testing.T, f env.Failure) { assert.Contains(t, f.Fields, "socials[1].url") }},
		{name: "invalid_code", route: "tv.pair.confirm", scen: "invalid_code", want: env.FailureCodeInvalid,
			call: func(a env.AccountServices) error { _, err := a.PairConfirm(ctx, "AAAAAA"); return err }},
		{name: "device_limit", route: "tv.pair.confirm", scen: "device_limit", want: env.FailureDeviceLimit,
			call: func(a env.AccountServices) error { _, err := a.PairConfirm(ctx, "AAAAAA"); return err },
			check: func(t *testing.T, f env.Failure) {
				require.Len(t, f.Devices, 3)
				assert.Equal(t, "d_201", f.Devices[2].ID)
				assert.Equal(t, model.PlatformTVOS, f.Devices[2].Platform)
			}},
		{name: "not a code", want: env.FailureCodeInvalid,
			call: func(a env.AccountServices) error { _, err := a.PairConfirm(ctx, "AB"); return err }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			e, f := signedEnv(t)
			if tc.route != "" {
				require.NoError(t, f.SetScenario(tc.route, tc.scen))
			}
			err := tc.call(e.Account())
			require.Error(t, err)
			got := env.FailureOf(err)
			assert.Equal(t, tc.want, got.Kind)
			if tc.check != nil {
				tc.check(t, got)
			}
			if tc.name == "not a code" {
				assert.Zero(t, f.Calls("tv.pair.confirm"), "nothing is sent")
			}
		})
	}

	t.Run("others", func(t *testing.T) {
		t.Parallel()

		e, f := signedEnv(t)
		f.Fail("tv.pair.confirm", 410, "errors/pair_code_expired.json")
		_, err := e.Account().PairConfirm(ctx, "AAAAAA")
		assert.Equal(t, env.FailureCodeExpired, env.FailureOf(err).Kind)
		f.Fail("tv.pair.confirm", 410, "errors/server_error.json")
		_, err = e.Account().PairConfirm(ctx, "AAAAAA")
		assert.Equal(t, env.FailureCodeExpired, env.FailureOf(err).Kind, "410 whatever the code")
		f.Fail("tv.pair.confirm", 500, "errors/server_error.json")
		_, err = e.Account().PairConfirm(ctx, "AAAAAA")
		assert.Equal(t, env.FailureBusy, env.FailureOf(err).Kind)
		f.Fail("tv.pair.confirm", 429, "errors/rate_limited.json")
		_, err = e.Account().PairConfirm(ctx, "AAAAAA")
		assert.Equal(t, env.FailureBusy, env.FailureOf(err).Kind)
		f.Fail("tv.pair.confirm", 0, "")
		f.SetOnline(false)
		_, err = e.Account().PairConfirm(ctx, "AAAAAA")
		assert.Equal(t, env.FailureOffline, env.FailureOf(err).Kind)
		assert.ErrorIs(t, err, api.ErrOffline)
	})
	t.Run("not API errors", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, env.FailureUnknown, env.FailureOf(nil).Kind)
		assert.Equal(t, env.FailureCanceled, env.FailureOf(fmt.Errorf("x: %w", context.Canceled)).Kind)
		assert.Equal(t, env.FailureCanceled, env.FailureOf(fmt.Errorf("x: %w", env.ErrPhotoCanceled)).Kind)
		assert.Equal(t, env.FailureBusy, env.FailureOf(errors.New("boom")).Kind)
	})
}

// TestFailureToast checks the text of each failure (P6 section 5, locales/vi.json).
func TestFailureToast(t *testing.T) {
	t.Parallel()

	e := env.New(noAccount{newFake(t)}, locales.MustTexts())
	tests := []struct {
		kind env.FailureKind
		want string
	}{
		{env.FailureOffline, "Không kết nối được, thử lại sau"},
		{env.FailureValidation, "Thông tin chưa hợp lệ"},
		{env.FailureWrongPassword, "Mật khẩu hiện tại chưa đúng"},
		{env.FailureFileTooLarge, "Ảnh tối đa 2 MB"},
		{env.FailureInvalidSocial, "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo, Threads"},
		{env.FailureCodeInvalid, "Mã không đúng. Kiểm tra lại mã trên TV"},
		{env.FailureCodeExpired, "Mã đã hết hạn. Lấy mã mới trên TV"},
		{env.FailureBusy, "Hệ thống đang bận"},
		{env.FailureDeviceLimit, ""},
		{env.FailureCanceled, ""},
		{env.FailureUnknown, ""},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, e.FailureToast(env.Failure{Kind: tc.kind}), "kind %d", tc.kind)
	}
}

// TestPhotoToast checks which failures of the picker are silent.
func TestPhotoToast(t *testing.T) {
	t.Parallel()

	e := env.New(noAccount{newFake(t)}, locales.MustTexts())
	assert.Empty(t, e.PhotoToast(nil))
	assert.Empty(t, e.PhotoToast(fmt.Errorf("x: %w", env.ErrPhotoCanceled)))
	assert.Empty(t, e.PhotoToast(context.Canceled))
	assert.Empty(t, e.PhotoToast(core.ErrUnsupported))
	assert.Empty(t, e.PhotoToast(core.ErrDuplicate))
	assert.Equal(t, "Không đọc được ảnh này", e.PhotoToast(errors.New("not an image")))
	assert.True(t, env.PhotoSilent(env.ErrPhotoCanceled))
	assert.False(t, env.PhotoSilent(errors.New("x")))
}

// TestSetAutoNext checks `pref:auto`: the services' Prefs when they can set it,
// else the memory of Env.
func TestSetAutoNext(t *testing.T) {
	t.Parallel()

	t.Run("services prefs", func(t *testing.T) {
		t.Parallel()

		f := newFake(t)
		e := env.New(f, locales.MustTexts())
		require.True(t, e.Prefs().AutoNext())
		e.SetAutoNext(false)
		assert.False(t, f.FakePrefs().AutoNext())
		e.SetAutoNext(true)
		assert.True(t, f.FakePrefs().AutoNext())
	})
	t.Run("memory", func(t *testing.T) {
		t.Parallel()

		e := env.New(noPrefs{newFake(t)}, locales.MustTexts())
		require.True(t, e.Prefs().AutoNext())
		e.SetAutoNext(false)
		assert.False(t, e.Prefs().AutoNext())
		e.SetAutoNext(true)
		assert.True(t, e.Prefs().AutoNext())
	})
}

// lateDevices is a Fake whose Devices reads the list at once, as the server
// answers, then holds the answer until release: a slow read.
type lateDevices struct {
	*shelltest.Fake

	hold    bool          // the next read is the slow one
	started chan struct{} // closed when a read has the list
	release chan struct{} // closed to let the read answer
}

// Devices reads the list, says so, then waits to hand it over.
func (l *lateDevices) Devices(ctx context.Context) (model.DeviceList, error) {
	d, err := l.Fake.Devices(ctx)
	if !l.hold {
		return d, err
	}
	close(l.started)
	<-l.release

	return d, err
}

// TestAccountStaleDevicesReadKeepsTheWrite checks the ordering of the Connect
// screen: a read of the devices begun, a confirm that lists the TV, then the read
// answering with the list from before. Whether its context was cancelled (what
// Connect does before it confirms) or not, the kept list keeps the TV.
func TestAccountStaleDevicesReadKeepsTheWrite(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		cancel bool
	}{
		{"cancelled read", true},
		{"read begun before a write", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t)
			require.NoError(t, f.SeedSession())
			_, err := f.Restore(context.Background())
			require.NoError(t, err)
			f.Account().SetTVLinked(false) // the TV is not paired yet
			late := &lateDevices{Fake: f, started: make(chan struct{}), release: make(chan struct{})}
			e := env.New(late, locales.MustTexts())
			a := e.Account()
			ctx := context.Background()

			_, err = a.Devices(ctx) // the screen of the account read the list: no TV in it
			require.NoError(t, err)
			_, linked := e.TVDevice()
			require.False(t, linked)
			late.hold = true // the read of the Connect screen is the slow one

			readCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := a.Devices(readCtx)
				done <- err
			}()
			<-late.started // the server answered: the list without the TV
			if tc.cancel {
				cancel()
			}
			_, err = a.PairConfirm(ctx, "7K4M2P") // the confirm lists the TV
			require.NoError(t, err)
			close(late.release)
			<-done // the stale read answers now
			tv, linked := e.TVDevice()
			require.True(t, linked, "the TV the confirm listed is still listed")
			assert.Equal(t, model.PlatformTVOS, tv.Platform)
		})
	}
}
