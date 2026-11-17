//go:build !tvos

package shelltest_test

import (
	"bytes"
	"context"
	"errors"
	"image/jpeg"
	"runtime"
	"testing"

	"dancefitvn-app/internal/auth"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signedFake returns a Fake with the mock's session restored.
func signedFake(t *testing.T) *shelltest.Fake {
	t.Helper()
	f := newFake(t)
	require.NoError(t, f.SeedSession())
	_, err := f.Restore(context.Background())
	require.NoError(t, err)

	return f
}

// TestFakeAccountKeepsWrites checks that the Fake does what the overlay of the app
// does over the mock's fixed answers: a profile written is the one read next, the
// coach profile too (platforms read from the links), the avatar URL, a password
// is recorded, and the calls are logged.
func TestFakeAccountKeepsWrites(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := signedFake(t)
	a := f.Account()

	u, ok := f.SessionUser()
	require.True(t, ok)
	assert.Equal(t, "u_412", u.ID)

	res, err := f.UpdateMe(ctx, model.MeUpdateRequest{FirstName: "An", LastName: "Lê", Email: "an@example.com", Phone: "0911222333"})
	require.NoError(t, err)
	assert.Equal(t, "Lê An", res.User.FullName)
	assert.Nil(t, res.EmailChangePending)
	me, err := f.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, "0911222333", me.User.Phone, "the next read shows the write")
	assert.Len(t, me.Badges, 3)
	require.Len(t, a.Updates(), 1)

	c, err := f.UpdateCoach(ctx, model.CoachUpdateRequest{
		Bio: "Xin chào", Socials: []model.SocialURL{{URL: "https://www.Instagram.com/an"}, {URL: "https://example.com"}}, Places: []string{"CLB"},
	})
	require.NoError(t, err)
	require.Len(t, c.Socials, 2)
	assert.Equal(t, "instagram", c.Socials[0].Platform)
	assert.Equal(t, "link", c.Socials[1].Platform)
	again, err := f.Coach(ctx)
	require.NoError(t, err)
	assert.Equal(t, c, again)
	assert.Len(t, a.CoachUpdates(), 1)

	_, err = f.ChangePassword(ctx, "old-pass", "new-password")
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"old-pass", "new-password"}}, a.Passwords())

	jpg := shelltest.GeneratedPhoto(64, 64)
	av, err := f.UploadAvatar(ctx, jpg)
	require.NoError(t, err)
	assert.Equal(t, jpg, a.Avatar())
	me, err = f.Me(ctx)
	require.NoError(t, err)
	assert.Equal(t, av.AvatarURL, me.User.AvatarURL)
	assert.Equal(t, "0911222333", me.User.Phone, "the avatar does not undo the profile")

	assert.Equal(t, []string{"UpdateMe", "Me", "UpdateCoach", "Coach", "ChangePassword", "UploadAvatar", "Me"}, a.Log())
	assert.Equal(t, 2, a.Count("Me"))
	a.ClearLog()
	assert.Empty(t, a.Log())
}

// TestFakeAccountScenarios checks the failures the mock's scenarios make through
// the Fake, each as the screens will meet it.
func TestFakeAccountScenarios(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("me.get token_expired signs the session out after the refresh", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		var reasons []error
		f.OnSignedOut(func(err error) { reasons = append(reasons, err) })
		require.NoError(t, f.SetScenario("me.get", "token_expired"))
		_, err := f.Me(ctx)
		require.ErrorIs(t, err, auth.ErrSignedOut)
		assert.False(t, f.Signed())
		assert.Len(t, reasons, 1)
	})
	t.Run("the other scenarios", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		for _, tc := range []struct {
			route, scen string
			call        func() error
			want        env.FailureKind
		}{
			{"me.update", "validation_failed", func() error { _, err := f.UpdateMe(ctx, model.MeUpdateRequest{}); return err }, env.FailureValidation},
			{"me.password", "wrong_password", func() error { _, err := f.ChangePassword(ctx, "a", "b"); return err }, env.FailureWrongPassword},
			{"me.avatar", "file_too_large", func() error { _, err := f.UploadAvatar(ctx, []byte{1}); return err }, env.FailureFileTooLarge},
			{"me.coach.update", "invalid_social", func() error { _, err := f.UpdateCoach(ctx, model.CoachUpdateRequest{}); return err }, env.FailureInvalidSocial},
			{"tv.pair.confirm", "invalid_code", func() error { _, err := f.PairConfirm(ctx, "AAAAAA"); return err }, env.FailureCodeInvalid},
		} {
			require.NoError(t, f.SetScenario(tc.route, tc.scen))
			assert.Equal(t, tc.want, env.FailureOf(tc.call()).Kind, tc.route+" "+tc.scen)
		}
		assert.Len(t, f.Account().Updates(), 1, "the refused request was sent")
		me, err := f.Me(ctx)
		require.NoError(t, err)
		assert.Equal(t, "0905123456", me.User.Phone, "a failed write is not kept")
	})
}

// TestFakeAccountDevices checks the device list and the TV: the mock's list by
// default (the TV linked), the prototype's unlinked start, a TV confirmed listed,
// a device deleted gone also on the server's 404, the device limit that lasts
// until a device is deleted.
func TestFakeAccountDevices(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("the mock's list", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		l, err := f.Devices(ctx)
		require.NoError(t, err)
		assert.Len(t, l.Items, 3)
		assert.Equal(t, model.DeviceLimit{Phone: 2, TV: 1}, l.Limit)
	})
	t.Run("unlinked start, then a TV confirmed", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		f.Account().SetTVLinked(false)
		l, err := f.Devices(ctx)
		require.NoError(t, err)
		assert.Len(t, l.Items, 2)
		for _, d := range l.Items {
			assert.NotEqual(t, model.PlatformTVOS, d.Platform)
		}
		res, err := f.PairConfirm(ctx, "7k4-m2p")
		require.NoError(t, err)
		assert.Equal(t, "d_201", res.Device.ID)
		l, err = f.Devices(ctx)
		require.NoError(t, err)
		require.Len(t, l.Items, 3, "listed once")
		assert.Equal(t, model.PlatformTVOS, l.Items[2].Platform)
		assert.Equal(t, []string{"Devices", "PairConfirm:7k4-m2p", "Devices"}, f.Account().Log())

		require.NoError(t, f.DeleteDevice(ctx, "d_201"))
		l, err = f.Devices(ctx)
		require.NoError(t, err)
		assert.Len(t, l.Items, 2, "unlinked again")
	})
	t.Run("a refused code sends nothing", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		_, err := f.PairConfirm(ctx, "AB")
		require.ErrorIs(t, err, core.ErrInvalid)
		assert.Zero(t, f.Calls("tv.pair.confirm"))
	})
	t.Run("not found is gone", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		require.NoError(t, f.SetScenario("devices.delete", "not_found"))
		require.Error(t, f.DeleteDevice(ctx, "d_102"))
		l, err := f.Devices(ctx)
		require.NoError(t, err)
		assert.Len(t, l.Items, 2)
	})
	t.Run("the device limit lasts until a device is deleted", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		f.Account().SetDeviceLimit(true)
		_, err := f.PairConfirm(ctx, "7K4M2P")
		f2 := env.FailureOf(err)
		require.Equal(t, env.FailureDeviceLimit, f2.Kind)
		assert.Len(t, f2.Devices, 3)
		_, err = f.PairConfirm(ctx, "7K4M2P")
		assert.Equal(t, env.FailureDeviceLimit, env.FailureOf(err).Kind, "still over")
		require.NoError(t, f.DeleteDevice(ctx, "d_102"))
		res, err := f.PairConfirm(ctx, "7K4M2P")
		require.NoError(t, err, "a device is gone: the same code goes through")
		assert.Equal(t, "d_201", res.Device.ID)
	})
}

// TestFakePhoto checks the picker's knobs: a generated picture at once, a picture
// of the test's, the cancel and the other errors, the hold.
func TestFakePhoto(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("default and own", func(t *testing.T) {
		t.Parallel()

		f := newFake(t)
		b, err := f.PickPhoto(ctx)
		require.NoError(t, err)
		img, err := jpeg.Decode(bytes.NewReader(b))
		require.NoError(t, err)
		assert.Equal(t, 640, img.Bounds().Dx())
		assert.Equal(t, 480, img.Bounds().Dy())
		f.Account().SetPhoto([]byte("own"))
		b, err = f.PickPhoto(ctx)
		require.NoError(t, err)
		assert.Equal(t, []byte("own"), b)
		assert.Equal(t, 2, f.Account().Count("PickPhoto"))
	})
	t.Run("errors", func(t *testing.T) {
		t.Parallel()

		f := newFake(t)
		for _, err := range []error{env.ErrPhotoCanceled, core.ErrUnsupported, core.ErrDuplicate, errors.New("unreadable")} {
			f.Account().SetPhotoError(err)
			b, got := f.PickPhoto(ctx)
			assert.Nil(t, b)
			require.ErrorIs(t, got, err)
		}
		f.Account().SetPhotoError(nil)
		_, err := f.PickPhoto(ctx)
		require.NoError(t, err)
	})
	t.Run("hold", func(t *testing.T) {
		t.Parallel()

		f := newFake(t)
		release := f.Account().HoldPhoto()
		done := make(chan error, 1)
		go func() { _, err := f.PickPhoto(ctx); done <- err }()
		for f.Account().Count("PickPhoto") == 0 {
			runtime.Gosched()
		}
		select {
		case <-done:
			t.Fatal("the picker answered while held")
		default:
		}
		release()
		require.NoError(t, <-done)

		release2 := f.Account().HoldPhoto()
		defer release2()
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := f.PickPhoto(cctx)
		require.ErrorIs(t, err, context.Canceled)
	})
}

// TestFakeLogout checks the sign-out of the Fake: the server told and the vault
// cleared without OnSignedOut, what the player had been told recorded, the hold,
// and a Login that waits for a Logout that began.
func TestFakeLogout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	t.Run("ends the session", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		signed := 0
		f.OnSignedOut(func(error) { signed++ })
		f.FakePlayback().Leave()
		require.NoError(t, f.Logout(ctx))
		assert.False(t, f.Signed())
		assert.Zero(t, signed, "the user asked: no OnSignedOut")
		assert.Equal(t, 1, f.Calls("auth.logout"))
		assert.Equal(t, []string{"Leave"}, f.Account().LogoutSaw())
		assert.True(t, f.Account().LogoutBegan())
	})
	t.Run("error", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		boom := errors.New("boom")
		f.Account().SetLogoutError(boom)
		require.ErrorIs(t, f.Logout(ctx), boom)
		assert.False(t, f.Signed(), "signed out whatever it says")
	})
	t.Run("hold and Login", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		release := f.Account().HoldLogout()
		logoutDone := make(chan error, 1)
		go func() { logoutDone <- f.Logout(ctx) }()
		for !f.Account().LogoutBegan() {
			runtime.Gosched()
		}
		loginDone := make(chan error, 1)
		go func() { loginDone <- f.Login(ctx, "a@example.com", "x") }()
		for range 2000 {
			runtime.Gosched()
		}
		select {
		case <-loginDone:
			t.Fatal("Login did not wait")
		default:
		}
		release()
		require.NoError(t, <-logoutDone)
		require.NoError(t, <-loginDone)
		assert.True(t, f.Signed())
	})
	t.Run("marked by BeginLogout", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		assert.False(t, f.Account().LogoutMarked())
		logout := f.BeginLogout()
		assert.True(t, f.Account().LogoutMarked(), "marked before the function runs")
		assert.False(t, f.Account().LogoutBegan())
		loginDone := make(chan error, 1)
		go func() { loginDone <- f.Login(ctx, "a@example.com", "x") }()
		for range 2000 {
			runtime.Gosched()
		}
		select {
		case <-loginDone:
			t.Fatal("Login did not wait for the mark")
		default:
		}
		require.NoError(t, logout(ctx))
		require.NoError(t, <-loginDone)
		assert.False(t, f.Account().LogoutMarked())
		assert.True(t, f.Signed())
	})
	t.Run("canceled while held", func(t *testing.T) {
		t.Parallel()

		f := signedFake(t)
		release := f.Account().HoldLogout()
		defer release()
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		require.ErrorIs(t, f.Logout(cctx), context.Canceled)
	})
}

// TestFakeAvatar checks the pictures Avatar answers: none at first, the one of
// SetAvatar for any URL, the one of SetAvatarOf for its own.
func TestFakeAvatar(t *testing.T) {
	t.Parallel()

	f := newFake(t)
	ctx := context.Background()
	_, err := f.Avatar(ctx, "u")
	require.ErrorIs(t, err, core.ErrNotFound)
	anyImg, err := scene.NewImage(2, 2)
	require.NoError(t, err)
	own, err := scene.NewImage(1, 1)
	require.NoError(t, err)
	f.SetAvatar(anyImg)
	got, err := f.Avatar(ctx, "u")
	require.NoError(t, err)
	assert.Same(t, anyImg, got)
	f.SetAvatarOf("b", own)
	got, err = f.Avatar(ctx, "b")
	require.NoError(t, err)
	assert.Same(t, own, got)
	f.SetAvatarOf("b", nil)
	f.SetAvatar(nil)
	_, err = f.Avatar(ctx, "b")
	require.ErrorIs(t, err, core.ErrNotFound)
}
