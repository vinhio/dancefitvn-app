//go:build !tvos

// P6 flows over the real wire: the account tab and what it leads to (15, 15B,
// 15C, the sheets of the password, the avatar and the logout, 16 and the
// sheet of devices), driven by taps on a Tester at both phones, over the
// repository's mock and its overlay, with virtual time. The flow helpers are
// those of flow_helpers_test.go (newFlow, scriptTransport, flowOpts); the
// fake picker is the one of account_mobile_test.go. Patterns adapted from
// jplayer-mobile/internal/ui/views/flow_test.go at 49de3c2 (a gate the fake
// waits on while frames run; one request at a time).

package app

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the toasts of P6 (locales/vi.json).
const (
	tInfoSaved     = "Đã cập nhật thông tin tài khoản"
	tInfoInvalid   = "Thông tin chưa hợp lệ"
	tInfoRequired  = "Tên và email là bắt buộc"
	tPwSaved       = "Đã đổi mật khẩu"
	tPwWrong       = "Mật khẩu hiện tại chưa đúng"
	tPwRequired    = "Nhập đủ ba ô mật khẩu"
	tPwShort       = "Mật khẩu mới cần ít nhất 8 ký tự"
	tPwMismatch    = "Mật khẩu nhập lại không khớp"
	tCoachSaved    = "Đã cập nhật hồ sơ HLV"
	tBadSocial     = "Chỉ hỗ trợ Facebook, Instagram, YouTube, TikTok, Zalo, Threads"
	tAvatarSaved   = "Đã cập nhật ảnh đại diện"
	tPhotoBad      = "Không đọc được ảnh này"
	tPhotoTooLarge = "Ảnh tối đa 2 MB"
	tTVLinked      = "Đã kết nối Apple TV"
	tCodeWrong     = "Mã không đúng. Kiểm tra lại mã trên TV"
	tCodeExpired   = "Mã đã hết hạn. Lấy mã mới trên TV"
	tOffline       = "Không kết nối được, thử lại sau"
	tBusy          = "Hệ thống đang bận"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// disk is a phone's vault and folders with a session of the coach.
type disk struct {
	secrets *memSecrets
	dir     string
}

// acct is a flow of a signed-in coach with the fake picker in the services.
type acct struct {
	*flow
	pick *fakePicker
}

// step is one write of P6: how to bring its screen up ready to act, the
// user's tap, the route of the request and what tells the outcome.
type step struct {
	name  string
	route string // the request the tap sends
	open  func(a *acct)
	act   func(a *acct)
	ok    func(a *acct) bool // the write went through
	stays func(a *acct) bool // the screen is as it was, for another try
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newDisk returns a phone that signed in a day ago.
func newDisk(t *testing.T) disk {
	t.Helper()
	d := disk{secrets: &memSecrets{vals: map[string][]byte{}}, dir: t.TempDir()}
	seedSession(t, d.secrets, d.dir, sys.BiometryNone)

	return d
}

// openAcct launches the app on the phone's disk, the picker answering photo.
func openAcct(t *testing.T, ph kittest.Phone, d disk, photo []byte, o flowOpts) *acct {
	t.Helper()
	fp := &fakePicker{data: photo}
	o.secrets, o.dir = d.secrets, d.dir
	prev := o.edit
	o.edit = func(dd *mobileDeps) {
		dd.PickImage = fp.pick
		if prev != nil {
			prev(dd)
		}
	}

	return &acct{flow: newFlow(t, ph, o), pick: fp}
}

// photoJPEG is a JPEG of a 2-colour picture.
func photoJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 200, G: 30, B: 60, A: 255}
			if x >= w/2 {
				c = color.RGBA{R: 20, G: 90, B: 220, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}))

	return buf.Bytes()
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestFlowAccountInfoSavePersists follows 03 sections 15 and 15B over the
// wire: the Account tab, the row to 15B, an edit saved (what is sent is
// trimmed, the phone as digits), Back with the toast, screen 15 with the new
// name, 15B showing what was saved (the overlay answers the next GET /me),
// and the same after a relaunch (the overlay file on disk).
func TestFlowAccountInfoSavePersists(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		d := newDisk(t)
		a := openAcct(t, ph, d, nil, flowOpts{})
		acc := a.toAccount()
		assert.Equal(t, "Nguyễn Thảo Vy", acc.UserName())
		a.tapName("go.info")
		a.until(env.RouteInfo)
		in := a.info()
		a.drive("the form", func() bool { return in.First().Text() == "Thảo Vy" })
		assert.Equal(t, "0905 123 456", in.Phone().Text())

		in.First().SetText("  Lan ")
		in.Last().SetText("Trần")
		in.Phone().SetText("0912 345 678")
		a.tapW(in.Submit())
		a.until(env.RouteAccount)
		a.expectToast(tInfoSaved)
		require.Equal(t, 1, a.st.calledN("me.update"))
		body := string(a.st.bodiesOf("me.update")[0])
		assert.Contains(t, body, `"first_name":"Lan"`)
		assert.Contains(t, body, `"last_name":"Trần"`)
		assert.Contains(t, body, `"phone":"0912345678"`)
		a.drive("15 with the new name", func() bool { return a.account().UserName() == "Trần Lan" })

		a.tapName("go.info")
		a.until(env.RouteInfo)
		in = a.info()
		a.drive("15B shows what was saved", func() bool {
			return in.First().Text() == "Lan" && in.Phone().Text() == "0912 345 678"
		})
		assert.Equal(t, "Trần", in.Last().Text())

		b := openAcct(t, ph, d, nil, flowOpts{})
		b.toAccount()
		b.drive("the name after a relaunch", func() bool { return b.account().UserName() == "Trần Lan" })
		b.tapName("go.info")
		b.until(env.RouteInfo)
		in = b.info()
		b.drive("15B after a relaunch", func() bool { return in.First().Text() == "Lan" })
		assert.Equal(t, "0912 345 678", in.Phone().Text())
	})
}

// TestFlowAccountInfoRefusals checks the answers of PATCH /me over the wire:
// validation_failed marks the fields it names and says so, an email that
// waits for its confirmation goes back with the toast of the address, and the
// local checks send nothing.
func TestFlowAccountInfoRefusals(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("validation_failed", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			in := a.toInfo()
			require.NoError(t, a.w.mock.SetScenario("me.update", "validation_failed"))
			a.tapW(in.Submit())
			a.shows(tInfoInvalid)
			assert.Equal(t, env.RouteInfo, a.route(), "the screen stays")
			assert.True(t, in.First().Error, "first_name is red")
			assert.True(t, in.Email().Error, "email is red")
			assert.False(t, in.Last().Error)
			assert.False(t, in.Phone().Error)
			assert.False(t, in.Busy())
		})
		t.Run("email_change_pending", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			in := a.toInfo()
			a.st.answerNext("me.update", 200, `{"user":{"id":"u_412","first_name":"Thảo Vy","last_name":"Nguyễn",`+
				`"full_name":"Nguyễn Thảo Vy","email":"thaovy.hlv@gmail.com","phone":"0905123456","member_since":"2024-03-12","role":"coach"},`+
				`"email_change_pending":"moi@example.invalid"}`)
			in.Email().SetText("moi@example.invalid")
			a.tapW(in.Submit())
			a.until(env.RouteAccount)
			a.expectToast("Kiểm tra hộp thư moi@example.invalid để xác nhận email mới")
			a.drive("15 keeps the email", func() bool { return a.account().Email() == "thaovy.hlv@gmail.com" })
		})
		t.Run("local checks send nothing", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			in := a.toInfo()
			in.First().SetText("   ")
			a.tapW(in.Submit())
			a.shows(tInfoRequired)
			in.First().SetText("Vy")
			in.Email().SetText("no-at-sign")
			a.tt.Elapse(2300 * time.Millisecond)
			a.tapW(in.Submit())
			a.shows(tInfoRequired)
			assert.Zero(t, a.st.calledN("me.update"))
			assert.Equal(t, env.RouteInfo, a.route())
		})
	})
}

// TestFlowAccountPassword checks 03 section 3 pwSave over the wire: the three
// local checks with their toasts send nothing; a wrong current password
// leaves the sheet open with the field red; the right one closes it with
// "Đã đổi mật khẩu" and keeps the session.
func TestFlowAccountPassword(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
		sh := a.toPassword()
		type fill struct{ cur, next, again, toast string }
		for _, tc := range []fill{
			{"", "secret22", "secret22", tPwRequired},
			{"secret1", "", "", tPwRequired},
			{"secret1", "short", "short", tPwShort},
			{"secret1", "secret22", "secret23", tPwMismatch},
		} {
			sh.Current().SetText(tc.cur)
			sh.New().SetText(tc.next)
			sh.Repeat().SetText(tc.again)
			a.tapW(sh.Save())
			a.expectToast(tc.toast)
		}
		assert.Zero(t, a.st.calledN("me.password"), "nothing sent")
		assert.True(t, a.sh.Nav().Sheet().IsOpen())

		sh.Current().SetText("wrong-one")
		sh.New().SetText("secret22")
		sh.Repeat().SetText("secret22")
		require.NoError(t, a.w.mock.SetScenario("me.password", "wrong_password"))
		a.tapW(sh.Save())
		a.expectToast(tPwWrong)
		assert.True(t, sh.Current().Error, "the current password is red")
		assert.True(t, a.sh.Nav().Sheet().IsOpen(), "the sheet stays")
		assert.False(t, sh.Busy())

		require.NoError(t, a.w.mock.SetScenario("me.password", "default"))
		sh.Current().SetText("secret1")
		a.tapW(sh.Save())
		a.shows(tPwSaved)
		a.tt.Elapse(400 * time.Millisecond)
		assert.False(t, a.sh.Nav().Sheet().IsOpen(), "the sheet closes")
		bodies := a.st.bodiesOf("me.password")
		require.Len(t, bodies, 2)
		assert.Contains(t, string(bodies[1]), `"current_password":"secret1"`)
		assert.Contains(t, string(bodies[1]), `"new_password":"secret22"`)
		_, signed := a.w.mgr.Session()
		assert.True(t, signed, "the session is kept (Q-P6-23)")
		assert.Equal(t, env.RouteAccount, a.route())
	})
}

// TestFlowAccountCoach checks 15C over the wire: the profile of the mock is
// shown, a save sends the links and places, goes back with the toast, and
// screen 15C shows the saved profile (overlay) after a relaunch; invalid_social
// stays with the toast and the red input of the link named.
func TestFlowAccountCoach(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("save and relaunch", func(t *testing.T) {
			d := newDisk(t)
			a := openAcct(t, ph, d, nil, flowOpts{})
			co := a.toCoach()
			assert.Equal(t, "Huấn luyện viên DanceFIT & StepX tại Đà Nẵng.", co.Bio().Text())
			assert.Equal(t, []string{"DANCEFIT", "STEPX", "BODYX"}, co.Subjects())
			require.Len(t, co.Socials(), 3)
			co.Bio().SetText("Bio mới")
			co.Places()[0].SetText("CLB B")
			a.tapW(co.Save())
			a.until(env.RouteAccount)
			a.expectToast(tCoachSaved)
			body := string(a.st.bodiesOf("me.coach.update")[0])
			assert.Contains(t, body, `"bio":"Bio mới"`)
			assert.Contains(t, body, `"CLB B"`)

			b := openAcct(t, ph, d, nil, flowOpts{})
			co = b.toCoach()
			b.drive("the saved bio", func() bool { return co.Bio().Text() == "Bio mới" })
			assert.Equal(t, "CLB B", co.Places()[0].Text())
		})
		t.Run("invalid_social", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			co := a.toCoach()
			require.NoError(t, a.w.mock.SetScenario("me.coach.update", "invalid_social"))
			co.Socials()[1].SetText("") // the request is [facebook, youtube]: socials[1] is youtube
			a.tapW(co.Save())
			a.shows(tBadSocial)
			assert.Equal(t, env.RouteCoach, a.route())
			assert.Equal(t, []bool{false, false, true},
				[]bool{co.Socials()[0].Error, co.Socials()[1].Error, co.Socials()[2].Error})
			assert.False(t, co.Busy())
		})
	})
}

// TestFlowAccountAvatar checks 15D over the wire with the fake picker: the
// library link picks, "Xác nhận" uploads a 512x512 JPEG as the multipart
// field "file", 15 shows the picture sent, the toast says so; a cancel does
// nothing, an unreadable picture says so; "Xác nhận" with no new photo only
// closes (K25) and sends nothing.
func TestFlowAccountAvatar(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("pick, confirm, upload", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
			acc := a.toAccount()
			before := acc.Avatar().Image
			sh := a.toAvatar()
			a.tapW(sh.Library())
			a.drive("the photo", func() bool { return sh.Picked() })
			assert.Equal(t, 1, a.pick.calls)
			a.tapW(sh.Accept())
			a.shows(tAvatarSaved)
			a.tt.Elapse(400 * time.Millisecond)
			assert.False(t, a.sh.Nav().Sheet().IsOpen())

			bodies := a.st.bodiesOf("me.avatar")
			require.Len(t, bodies, 1)
			pic := multipartFile(t, bodies[0])
			cfg, format, err := image.DecodeConfig(bytes.NewReader(pic))
			require.NoError(t, err)
			assert.Equal(t, "jpeg", format)
			assert.Equal(t, [2]int{512, 512}, [2]int{cfg.Width, cfg.Height})
			assert.LessOrEqual(t, len(pic), 2<<20, "within the 2 MB the server takes")

			a.drive("15 draws the picture sent", func() bool {
				img := acc.Avatar().Image

				return img != nil && img != before && img.Width == 512
			})
			assert.Equal(t, env.RouteAccount, a.route())
		})
		t.Run("K25: no new photo closes without upload or toast", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
			a.toAccount()
			sh := a.toAvatar()
			a.tapW(sh.Accept())
			a.tt.Elapse(400 * time.Millisecond)
			assert.False(t, a.sh.Nav().Sheet().IsOpen())
			assert.Zero(t, a.st.calledN("me.avatar"))
			assert.Zero(t, a.pick.calls)
			a.gone(tAvatarSaved)
		})
		t.Run("canceled and unreadable", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.toAccount()
			sh := a.toAvatar()
			a.pick.mu.Lock()
			a.pick.err = sys.ErrCanceled
			a.pick.mu.Unlock()
			a.tapW(sh.Library())
			a.drive("the picker's answer", func() bool { return a.pick.calls == 1 && !sh.Picking() })
			assert.False(t, sh.Picked())
			a.gone(tPhotoBad)

			a.pick.mu.Lock()
			a.pick.err, a.pick.data = nil, []byte("not a picture")
			a.pick.mu.Unlock()
			a.tapW(sh.Library())
			a.shows(tPhotoBad)
			assert.False(t, sh.Picked())
			assert.Zero(t, a.st.calledN("me.avatar"))
		})
		t.Run("file_too_large", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
			a.toAccount()
			sh := a.toAvatar()
			a.tapW(sh.Library())
			a.drive("the photo", func() bool { return sh.Picked() })
			require.NoError(t, a.w.mock.SetScenario("me.avatar", "file_too_large"))
			a.tapW(sh.Accept())
			a.shows(tPhotoTooLarge)
			assert.True(t, a.sh.Nav().Sheet().IsOpen(), "the sheet stays")
			assert.False(t, sh.Busy())
		})
	})
}

// TestFlowAccountConnectTV checks 16 over the wire: the mock's TV shows as
// linked, an unlink removes it (devices.delete), a code typed as "7k4-m2p"
// is sent as 7K4M2P and links ("Đã kết nối Apple TV"), unlink and relink
// work again (the overlay brings the TV back), and the refusals stay on the
// screen with the code kept.
func TestFlowAccountConnectTV(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("link, unlink, relink", func(t *testing.T) {
			d := newDisk(t)
			a := openAcct(t, ph, d, nil, flowOpts{})
			c := a.toConnect()
			a.drive("the TV of the mock is linked", func() bool { return c.Linked() })
			for round := range 2 {
				c = a.unlink(c)
				assert.Empty(t, c.Boxes().Value, "the code entry is empty")
				a.typeCode(c, "7k4-m2p")
				assert.Equal(t, "7K4M2P", c.Boxes().Value)
				a.tapW(c.Link())
				a.drive("linked", func() bool { return c.Linked() })
				a.expectToast(tTVLinked)
				a.settleFor(3)
				bodies := a.st.bodiesOf("tv.pair.confirm")
				require.Len(t, bodies, round+1)
				assert.Contains(t, string(bodies[round]), `"7K4M2P"`, "normalised")
				l, err := a.w.Devices(t.Context())
				require.NoError(t, err)
				assert.True(t, hasDevice(l, "d_201"), "the overlay brings the TV back")
			}
			assert.Equal(t, 2, a.st.calledN("devices.delete"))

			a.tapName("back")
			a.until(env.RouteAccount)
			a.drive("15 says linked", func() bool { return a.account().TV() != "" })
		})
		t.Run("devices.delete not_found is gone", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			c := a.toConnect()
			a.drive("linked", func() bool { return c.Linked() })
			require.NoError(t, a.w.mock.SetScenario("devices.delete", "not_found"))
			a.tapW(c.Unlink())
			a.drive("the code entry", func() bool { return !c.Linked() })
			assert.Equal(t, 1, a.st.calledN("devices.delete"))
			assert.False(t, c.Busy())
			a.gone(tBusy)
			a.gone(tOffline)
		})
		refusals := []struct {
			name  string
			arm   func(a *acct)
			toast string
		}{
			{"invalid_code", func(a *acct) { require.NoError(a.t, a.w.mock.SetScenario("tv.pair.confirm", "invalid_code")) }, tCodeWrong},
			{"expired", func(a *acct) {
				a.st.answerNext("tv.pair.confirm", 410, `{"error":{"code":"pair_code_expired","message":"Mã đã hết hạn."}}`)
			}, tCodeExpired},
		}
		for _, tc := range refusals {
			t.Run(tc.name, func(t *testing.T) {
				a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
				c := a.toUnlinked()
				tc.arm(a)
				a.typeCode(c, "ABCDEF")
				a.tapW(c.Link())
				a.shows(tc.toast)
				assert.False(t, c.Linked())
				assert.Equal(t, "ABCDEF", c.Boxes().Value, "the code is kept")
				assert.Equal(t, env.RouteConnect, a.route())
				assert.False(t, c.Busy())
			})
		}
	})
}

// TestFlowAccountConnectCodeEntryAfterUnlink checks that the code entry that
// "Ngắt kết nối" brings in place can be used: its parts have a frame and the
// boxes take typing.
func TestFlowAccountConnectCodeEntryAfterUnlink(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
		c := a.toConnect()
		a.drive("linked", func() bool { return c.Linked() })
		a.tapW(c.Unlink())
		a.drive("the code entry", func() bool { return !c.Linked() })
		a.settleFor(10)
		f := c.Boxes().Node().WindowFrame()
		require.Positive(t, f.W, "the boxes have a frame")
		require.Positive(t, f.H, "the boxes have a frame")
		a.typeCode(c, "7k4-m2p")
		assert.Equal(t, "7K4M2P", c.Boxes().Value)
	})
}

// TestFlowAccountDeviceLimit checks the 409 of tv.pair.confirm over the wire:
// the sheet of devices lists the devices the server names (this phone cannot
// be chosen), the choice is removed, the code is confirmed again and the TV
// is linked with the toast.
func TestFlowAccountDeviceLimit(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
		c := a.toUnlinked()
		a.st.failNext("tv.pair.confirm", 1, 409, "errors/device_limit.json")
		a.typeCode(c, "7K4M2P")
		a.tapW(c.Link())
		ds := a.devicesSheet()
		require.Len(t, ds.Rows(), 3)
		assert.Equal(t, "7K4M2P", ds.Code())
		assert.True(t, ds.Rows()[0].Disabled, "this phone cannot be removed")
		assert.Equal(t, "d_201", ds.Chosen(), "the TV is chosen first")

		a.tapName("devChoice:d_102")
		assert.Equal(t, "d_102", ds.Chosen())
		a.tapW(ds.RemoveButton())
		a.drive("linked", func() bool { return c.Linked() })
		a.tt.Elapse(400 * time.Millisecond)
		assert.False(t, a.sh.Nav().Sheet().IsOpen(), "the sheet closes")
		a.shows(tTVLinked)
		assert.Equal(t, 1, a.st.calledN("devices.delete")-1, "one removal besides the unlink")
		assert.Equal(t, 2, a.st.calledN("tv.pair.confirm"), "the code goes again")
		l, err := a.w.Devices(t.Context())
		require.NoError(t, err)
		assert.False(t, hasDevice(l, "d_102"), "the device chosen is gone")
	})
}

// TestFlowAccountLogout checks 03 section 3 logout from screen 15 over the
// wire: the sheet, the login at once with no banner, the save of the position
// that Leave started waited for before the server is told (and the saves
// returned), the queue of progress empty, the downloaded file and its record
// kept (Q-P5-8), and the same user signing in again finds the download.
func TestFlowAccountLogout(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		secrets, dir := seedDownloads(t, 5)
		df := newDlFlow(t, ph, dlOpts{secrets: secrets, dir: dir})
		df.startLesson3()
		df.play(3 * time.Second)
		release := df.st.holdRequest("lessons.progress")
		t.Cleanup(release)
		saves := df.saves()
		require.True(t, df.back(), "Back leaves the video")
		df.await("the save of the leave", func() bool { return df.saves() > saves })

		df.sh.Go(nav.Route{Screen: env.RouteAccount}, true)
		df.until(env.RouteAccount)
		df.drive("the badges", func() bool { return len(df.account().Badges()) > 0 })
		df.tt.Elapse(2300 * time.Millisecond) // a toast over the button would take the tap
		ac := &acct{flow: df.flow}            // taps that scroll the page into view
		ac.tapName("sheet:logout")
		df.tt.Elapse(400 * time.Millisecond)
		_, ok := df.sh.Nav().Sheet().Content().(*screens.LogoutSheet)
		require.True(t, ok, "the sheet of the logout")
		ac.tapName("logout")
		assert.Equal(t, env.RouteLogin, df.route(), "the login at once")
		for range 50 {
			df.q.drain()
			runtime.Gosched()
		}
		assert.Zero(t, df.st.calledN("auth.logout"), "the server is told after the save")
		_, signed := df.w.mgr.Session()
		assert.True(t, signed, "the session lasts until the save returned")

		release()
		df.drive("the sign-out", func() bool {
			_, signed := df.w.mgr.Session()
			df.w.mu.Lock()
			going := df.w.loggingOut != nil
			df.w.mu.Unlock()

			return !signed && !going
		})
		assert.Equal(t, df.st.calledN("lessons.progress"), df.st.doneN("lessons.progress"), "the save returned first")
		assert.Equal(t, 1, df.st.calledN("auth.logout"))
		assert.Zero(t, df.w.queue.Load().Len(), "nothing queued")
		title, _, banner := df.login().Banner()
		assert.False(t, banner, "no banner: %q", title)
		assert.FileExists(t, filepath.Join(df.audio, "l_3505.mp3"), "the downloads are kept")
		index, err := os.ReadFile(filepath.Join(df.audio, "index.json"))
		require.NoError(t, err)
		assert.Contains(t, string(index), "l_3505")

		df.signIn("coach@example.invalid", "secret1")
		df.homeLoaded()
		st, _ := df.sh.Env().Downloads().State("l_3505")
		assert.Equal(t, kit.DlDone, st, "the same user finds the download")
		assert.Zero(t, df.st.calledN("progress.batch"), "nothing of the user before was sent")
	})
}

// TestFlowAccountWriteFailures checks, for every write of P6, the answers
// every route shares (03 section 9): offline says so and a retry goes
// through; 426 goes to the upgrade screen; a token that expired is refreshed
// once and the write sent again; a server that is busy says so.
func TestFlowAccountWriteFailures(t *testing.T) {
	t.Parallel()

	steps := []step{
		{
			name: "info", route: "me.update",
			open: func(a *acct) { in := a.toInfo(); in.First().SetText("Lan") },
			act:  func(a *acct) { a.tapW(a.info().Submit()) },
			ok:   func(a *acct) bool { return a.route() == env.RouteAccount },
			stays: func(a *acct) bool {
				return a.route() == env.RouteInfo && !a.info().Busy()
			},
		},
		{
			name: "password", route: "me.password",
			open: func(a *acct) {
				sh := a.toPassword()
				sh.Current().SetText("secret1")
				sh.New().SetText("secret22")
				sh.Repeat().SetText("secret22")
			},
			act: func(a *acct) { a.tapW(a.passwordSheet().Save()) },
			ok: func(a *acct) bool {
				a.tt.Elapse(400 * time.Millisecond)

				return !a.sh.Nav().Sheet().IsOpen()
			},
			stays: func(a *acct) bool { return a.sh.Nav().Sheet().IsOpen() && !a.passwordSheet().Busy() },
		},
		{
			name: "coach", route: "me.coach.update",
			open: func(a *acct) { a.toCoach().Bio().SetText("Mới") },
			act:  func(a *acct) { a.tapW(a.coach().Save()) },
			ok:   func(a *acct) bool { return a.route() == env.RouteAccount },
			stays: func(a *acct) bool {
				return a.route() == env.RouteCoach && !a.coach().Busy()
			},
		},
		{
			name: "avatar", route: "me.avatar",
			open: func(a *acct) {
				a.toAccount()
				sh := a.toAvatar()
				a.tapW(sh.Library())
				a.drive("the photo", func() bool { return sh.Picked() })
			},
			act: func(a *acct) { a.tapW(a.avatarSheet().Accept()) },
			ok: func(a *acct) bool {
				a.tt.Elapse(400 * time.Millisecond)

				return !a.sh.Nav().Sheet().IsOpen()
			},
			stays: func(a *acct) bool { return a.sh.Nav().Sheet().IsOpen() && !a.avatarSheet().Busy() },
		},
		{
			name: "connect", route: "tv.pair.confirm",
			open: func(a *acct) { a.typeCode(a.toUnlinked(), "7K4M2P") },
			act:  func(a *acct) { a.tapW(a.connect().Link()) },
			ok:   func(a *acct) bool { return a.connect().Linked() },
			stays: func(a *acct) bool {
				return a.route() == env.RouteConnect && !a.connect().Linked() && !a.connect().Busy()
			},
		},
		{
			name: "unlink", route: "devices.delete",
			open: func(a *acct) {
				c := a.toConnect()
				a.drive("linked", func() bool { return c.Linked() })
			},
			act: func(a *acct) { a.tapW(a.connect().Unlink()) },
			ok:  func(a *acct) bool { return !a.connect().Linked() },
			stays: func(a *acct) bool {
				return a.route() == env.RouteConnect && a.connect().Linked() && !a.connect().Busy()
			},
		},
		{
			name: "devices sheet", route: "devices.delete",
			open: func(a *acct) {
				c := a.toUnlinked()
				a.st.failNext("tv.pair.confirm", 1, 409, "errors/device_limit.json")
				a.typeCode(c, "7K4M2P")
				a.tapW(c.Link())
				a.devicesSheet()
			},
			act: func(a *acct) { a.tapW(a.devicesSheet().RemoveButton()) },
			ok: func(a *acct) bool {
				a.tt.Elapse(400 * time.Millisecond)

				return a.connect().Linked() && !a.sh.Nav().Sheet().IsOpen()
			},
			stays: func(a *acct) bool { return a.sh.Nav().Sheet().IsOpen() && !a.devicesSheet().Busy() },
		},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, s := range steps {
			t.Run(s.name+"/offline then retry", func(t *testing.T) {
				a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
				s.open(a)
				a.w.tr.setOffline(true)
				s.act(a)
				a.shows(tOffline)
				a.drive("the screen as it was", func() bool { return s.stays(a) })
				a.tt.Elapse(2300 * time.Millisecond)
				a.w.tr.setOffline(false)
				s.act(a)
				a.drive("the write goes through", func() bool { return s.ok(a) })
			})
			t.Run(s.name+"/426", func(t *testing.T) {
				a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
				s.open(a)
				a.st.failNext(s.route, 1, 426, "errors/upgrade_required.json")
				s.act(a)
				a.until(env.RouteUpgrade)
				assert.Equal(t, 0, a.sh.Nav().Depth())
			})
			t.Run(s.name+"/token_expired refreshes once", func(t *testing.T) {
				a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
				s.open(a)
				before := a.st.calledN(s.route)
				a.st.failNext(s.route, 1, 401, "errors/token_expired.json")
				s.act(a)
				a.drive("the write goes through", func() bool { return s.ok(a) })
				assert.Equal(t, 1, a.st.calledN("auth.refresh"), "one refresh")
				assert.Equal(t, before+2, a.st.calledN(s.route), "sent again once")
			})
			t.Run(s.name+"/busy", func(t *testing.T) {
				a := openAcct(t, ph, newDisk(t), photoJPEG(t, 640, 480), flowOpts{})
				s.open(a)
				a.st.failNext(s.route, 1, 500, "errors/server_error.json")
				s.act(a)
				a.shows(tBusy)
				a.drive("the screen as it was", func() bool { return s.stays(a) })
			})
		}
	})
}

// TestFlowAccountReadsOfflineAndRefused checks the reads of the account
// screens: with the network gone after one visit each screen draws what it
// read (the cache), no toast of failure; a 426 on a read goes to the
// upgrade screen; a token that expired on GET /me is refreshed once and the
// screen draws.
func TestFlowAccountReadsOfflineAndRefused(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("offline after a visit", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.toAccount()
			a.drive("badges", func() bool { return len(a.account().Badges()) == 3 })
			co := a.toCoachFromAccount()
			a.drive("coach", func() bool { return len(co.Socials()) == 3 })
			a.tapName("back")
			a.until(env.RouteAccount)
			c := a.toConnect()
			a.drive("linked", func() bool { return c.Linked() })
			a.tapName("back")
			a.until(env.RouteAccount)

			a.w.tr.setOffline(true)
			a.tapName("go.info")
			a.until(env.RouteInfo)
			in := a.info()
			a.settleFor(10)
			assert.Equal(t, "Thảo Vy", in.First().Text())
			a.gone(tOffline)
			a.tapName("back")
			a.until(env.RouteAccount)
			a.settleFor(10)
			assert.Equal(t, "Nguyễn Thảo Vy", a.account().UserName())
			assert.Len(t, a.account().Badges(), 3)
			a.tapName("go.coach")
			a.until(env.RouteCoach)
			co = a.coach()
			a.settleFor(10)
			assert.Equal(t, "Huấn luyện viên DanceFIT & StepX tại Đà Nẵng.", co.Bio().Text())
			a.tapName("back")
			a.until(env.RouteAccount)
			c = a.toConnect()
			a.settleFor(10)
			assert.True(t, c.Linked(), "the devices read before")
		})
		t.Run("426 on a read", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.homeLoaded()
			a.st.failNext("me.get", 1, 426, "errors/upgrade_required.json")
			a.tab(3)
			a.until(env.RouteUpgrade)
		})
		t.Run("426 on the devices", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.toAccount()
			a.st.failNext("devices.list", 1, 426, "errors/upgrade_required.json")
			a.tapName("go.connect")
			a.until(env.RouteUpgrade)
		})
		t.Run("token_expired on GET /me", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.homeLoaded()
			a.st.failNext("me.get", 1, 401, "errors/token_expired.json")
			a.tab(3)
			a.until(env.RouteAccount)
			a.drive("the badges of the retry", func() bool { return len(a.account().Badges()) == 3 })
			assert.Equal(t, 1, a.st.calledN("auth.refresh"))
			assert.Equal(t, 2, a.st.calledN("me.get"))
			assert.Equal(t, env.RouteAccount, a.route())
		})
		t.Run("token_expired on the refresh: the login", func(t *testing.T) {
			a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
			a.homeLoaded()
			require.NoError(t, a.w.mock.SetScenario("auth.refresh", "refresh_invalid"))
			a.st.failNext("me.coach.get", 1, 401, "errors/token_expired.json")
			a.toAccount()
			a.tapName("go.coach")
			a.until(env.RouteLogin)
			title, _, shown := a.login().Banner()
			assert.True(t, shown)
			assert.Equal(t, "Phiên đăng nhập đã hết hạn", title)
		})
	})
}

// TestFlowAccountReloginSameUser checks that the user who signs out and in
// again finds the profile as saved (the overlay is the phone's, not the
// session's) and that the first sign-in did not leave a banner.
func TestFlowAccountReloginSameUser(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		a := openAcct(t, ph, newDisk(t), nil, flowOpts{})
		in := a.toInfo()
		in.First().SetText("Lan")
		a.tapW(in.Submit())
		a.until(env.RouteAccount)
		a.expectToast(tInfoSaved) // the toast stands over the logout button, as in the prototype
		a.drive("the name", func() bool { return a.account().UserName() == "Nguyễn Lan" })

		a.tapName("sheet:logout")
		a.tt.Elapse(400 * time.Millisecond)
		a.tapName("logout")
		a.until(env.RouteLogin)
		a.drive("the sign-out", func() bool { _, s := a.w.mgr.Session(); return !s })
		_, _, banner := a.login().Banner()
		assert.False(t, banner)
		assert.Equal(t, "coach@example.invalid", a.login().Email().Text())

		a.signIn("coach@example.invalid", "secret1")
		a.homeLoaded()
		a.tab(3)
		a.until(env.RouteAccount)
		a.drive("the saved name", func() bool { return a.account().UserName() == "Nguyễn Lan" })
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// toAccount waits for Home, opens the Account tab and waits for its content.
func (a *acct) toAccount() *screens.Account {
	a.t.Helper()
	if a.route() == env.RouteAccount {
		return a.account()
	}
	a.homeLoaded()
	a.tab(3)
	a.until(env.RouteAccount)
	a.drive("the badges", func() bool { return len(a.account().Badges()) > 0 })

	return a.account()
}

// toInfo opens 15B from the Account tab.
func (a *acct) toInfo() *screens.Info {
	a.t.Helper()
	a.toAccount()
	a.tapName("go.info")
	a.until(env.RouteInfo)
	in := a.info()
	a.drive("the form", func() bool { return in.First().Text() != "" })

	return in
}

// toCoach opens 15C from the Account tab and waits for its profile.
func (a *acct) toCoach() *screens.Coach {
	a.t.Helper()
	a.toAccount()

	return a.toCoachFromAccount()
}

// toCoachFromAccount taps the row of 15C on screen 15 showing.
func (a *acct) toCoachFromAccount() *screens.Coach {
	a.t.Helper()
	a.tapName("go.coach")
	a.until(env.RouteCoach)
	co := a.coach()
	a.drive("the profile", func() bool { return len(co.Socials()) == 3 })

	return co
}

// toConnect opens 16 from the Account tab.
func (a *acct) toConnect() *screens.Connect {
	a.t.Helper()
	a.toAccount()
	a.tapName("go.connect")
	a.until(env.RouteConnect)

	return a.connect()
}

// toUnlinked opens 16 and takes the mock's TV away, so that the code entry
// shows.
func (a *acct) toUnlinked() *screens.Connect {
	a.t.Helper()
	c := a.toConnect()
	a.drive("linked", func() bool { return c.Linked() })

	return a.unlink(c)
}

// unlink taps "Ngắt kết nối": the code entry comes in place and takes the code at
// once.
func (a *acct) unlink(c *screens.Connect) *screens.Connect {
	a.t.Helper()
	a.tapW(c.Unlink())
	a.drive("the code entry", func() bool { return !c.Linked() })
	a.settleFor(3)
	require.False(a.t, c.Linked())

	return c
}

// toPassword opens SH.password from the Account tab.
func (a *acct) toPassword() *screens.PasswordSheet {
	a.t.Helper()
	a.toAccount()
	a.tapName("sheet:password")
	a.tt.Elapse(400 * time.Millisecond)

	return a.passwordSheet()
}

// toAvatar opens SH.avatar from screen 15.
func (a *acct) toAvatar() *screens.AvatarSheet {
	a.t.Helper()
	a.tapName("sheet:avatar")
	a.tt.Elapse(400 * time.Millisecond)

	return a.avatarSheet()
}

// account returns screen 15 showing.
func (a *flow) account() *screens.Account {
	a.t.Helper()
	s, ok := a.sh.Nav().Screen().(*screens.Account)
	require.True(a.t, ok, "the screen is 15, not %T", a.sh.Nav().Screen())

	return s
}

// info returns 15B showing.
func (a *acct) info() *screens.Info {
	a.t.Helper()
	s, ok := a.sh.Nav().Screen().(*screens.Info)
	require.True(a.t, ok, "the screen is 15B, not %T", a.sh.Nav().Screen())

	return s
}

// coach returns 15C showing.
func (a *acct) coach() *screens.Coach {
	a.t.Helper()
	s, ok := a.sh.Nav().Screen().(*screens.Coach)
	require.True(a.t, ok, "the screen is 15C, not %T", a.sh.Nav().Screen())

	return s
}

// connect returns 16 showing.
func (a *acct) connect() *screens.Connect {
	a.t.Helper()
	s, ok := a.sh.Nav().Screen().(*screens.Connect)
	require.True(a.t, ok, "the screen is 16, not %T", a.sh.Nav().Screen())

	return s
}

// passwordSheet returns the sheet of the password open.
func (a *acct) passwordSheet() *screens.PasswordSheet {
	a.t.Helper()
	s, ok := a.sh.Nav().Sheet().Content().(*screens.PasswordSheet)
	require.True(a.t, ok, "the sheet is SH.password, not %T", a.sh.Nav().Sheet().Content())

	return s
}

// avatarSheet returns the sheet of the avatar open.
func (a *acct) avatarSheet() *screens.AvatarSheet {
	a.t.Helper()
	s, ok := a.sh.Nav().Sheet().Content().(*screens.AvatarSheet)
	require.True(a.t, ok, "the sheet is SH.avatar, not %T", a.sh.Nav().Sheet().Content())

	return s
}

// devicesSheet waits for the sheet of devices and returns it.
func (a *acct) devicesSheet() *screens.DevicesSheet {
	a.t.Helper()
	var s *screens.DevicesSheet
	a.drive("the sheet of devices", func() bool {
		if !a.sh.Nav().Sheet().IsOpen() {
			return false
		}
		var ok bool
		s, ok = a.sh.Nav().Sheet().Content().(*screens.DevicesSheet)

		return ok
	})
	a.tt.Elapse(400 * time.Millisecond)

	return s
}

// typeCode types s into the boxes of 16, as a finger and a keyboard do.
func (a *acct) typeCode(c *screens.Connect, s string) {
	a.t.Helper()
	a.tt.View().Focus(c.Boxes().Field())
	a.tt.Type(s)
}

// tapName taps the middle of the first visible widget named name.
func (a *acct) tapName(name string) {
	a.t.Helper()
	w := widgetNth(a.sh, name, 1)
	require.NotNil(a.t, w, "no visible widget %q", name)
	a.tapW(w)
}

// tapW taps the middle of the widget, the page scrolled until it shows over
// the tab bar, then runs a few frames.
func (a *acct) tapW(w ui.Widget) {
	a.t.Helper()
	if sc, ok := a.sh.Nav().Screen().(nav.Scroller); ok && !a.sh.Nav().Sheet().IsOpen() {
		if f := w.Node().WindowFrame(); f.Y+f.H > a.ph.Size.H-100 {
			sc.Scroll().ScrollBy(geom.Pt(0, f.Y+f.H-(a.ph.Size.H-100)))
			a.tt.Elapse(kittest.Frame)
		}
	}
	a.tt.ClickAt(w.Node().WindowFrame().Center())
	a.settleFor(3)
}

// expectToast waits for the toast, then lets its 2200 ms pass and checks
// that it went.
func (a *acct) expectToast(text string) {
	a.t.Helper()
	a.shows(text)
	a.tt.Elapse(2300 * time.Millisecond)
	a.gone(text)
}

// multipartFile returns the bytes of the part "file" of a multipart body
// whose boundary is the first line.
func multipartFile(t *testing.T, body []byte) []byte {
	t.Helper()
	line, _, ok := bytes.Cut(body, []byte("\r\n"))
	require.True(t, ok)
	boundary := strings.TrimPrefix(string(line), "--")
	r := multipart.NewReader(bytes.NewReader(body), boundary)
	for {
		p, err := r.NextPart()
		require.NoError(t, err, "a part named file")
		if p.FormName() == "file" {
			b, err := io.ReadAll(p)
			require.NoError(t, err)

			return b
		}
	}
}
