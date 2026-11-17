//go:build !tvos

package screens_test

import (
	"bytes"
	"errors"
	"image"
	"image/jpeg"
	"testing"
	"time"

	"dancefitvn-app/internal/avatar"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core"
	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The texts of the toasts of 15D (locales/vi.json).
const (
	toastAvatarSaved   = "Đã cập nhật ảnh đại diện"
	toastPhotoTooLarge = "Ảnh tối đa 2 MB"
	toastPhotoBad      = "Không đọc được ảnh này"
)

// atAvatar opens SH.avatar from screen 15, the way the avatar button does.
func atAvatar(tb testing.TB, ph kittest.Phone, real bool, setups ...func(*shelltest.Fake)) (*shelltest.Rig, *screens.AvatarSheet) {
	tb.Helper()
	r := atAccount(tb, ph, real, setups...)
	tapKey(tb, r, "sheet:avatar")
	slideIn(r)
	sh, ok := r.Shell.Nav().Sheet().Content().(*screens.AvatarSheet)
	require.True(tb, ok, "the sheet showing is SH.avatar")

	return r, sh
}

// pickLib taps "Chọn từ thư viện" and waits for the picture to be decoded and shown.
func pickLib(r *shelltest.Rig, sh *screens.AvatarSheet) {
	tap(r, sh.Library())
	r.Settle()
}

// touchDrag puts a finger on from, moves it through the points and lifts it on the
// last, a frame between each.
func touchDrag(r *shelltest.Rig, from geom.Point, through ...geom.Point) {
	r.Tester.TouchDown(from)
	r.Tester.Elapse(kittest.Frame)
	last := from
	for _, p := range through {
		r.Tester.TouchMove(p)
		r.Tester.Elapse(kittest.Frame)
		last = p
	}
	r.Tester.TouchUp(last)
	r.Tester.Elapse(kittest.Frame)
}

// center is the middle of the widget in the window.
func center(w ui.Widget) geom.Point { return w.Node().WindowFrame().Center() }

// trackPoint returns the point of the slider's thumb for the zoom z.
func trackPoint(sh *screens.AvatarSheet, z float64) geom.Point {
	f := sh.Zoom().WindowFrame()
	frac := (z - avatar.MinZoom) / (avatar.MaxZoom - avatar.MinZoom)

	return geom.Pt(f.X+10+float32(frac)*(f.W-20), f.Y+10)
}

// notShown asserts that none of the toasts of the sheet shows.
func notShown(t *testing.T, r *shelltest.Rig) {
	t.Helper()
	for _, m := range []string{toastAvatarSaved, toastPhotoTooLarge, toastPhotoBad, toastOffline, toastBusy} {
		assert.Falsef(t, r.Tester.Shows(m), "no toast %q", m)
	}
}

// TestAvatarSheetOpens checks the state of M15D: the prototype's view (zoom 1.2, no
// offset, the slider at 1.2), no new picture, every text of the sheet, nothing called.
func TestAvatarSheetOpens(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		assert.Equal(t, avatar.Initial, sh.Crop().View)
		assert.InDelta(t, 1.2, sh.Zoom().Value, 1e-9)
		assert.InDelta(t, 1, sh.Zoom().Min, 0)
		assert.InDelta(t, 3, sh.Zoom().Max, 0)
		assert.InDelta(t, .01, sh.Zoom().Step, 0)
		assert.False(t, sh.Picked())
		assert.False(t, sh.Picking())
		assert.False(t, sh.Busy())
		for _, text := range []string{"Chỉnh sửa ảnh đại diện", "Kéo để định vị, dùng thanh trượt để thu phóng.", "–", "+",
			"Ảnh chỉ được lưu sau khi bấm “Xác nhận”.", "Chụp ảnh mới", "Chọn từ thư viện", "Huỷ", "Xác nhận"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the sheet", text)
		}
		assert.Zero(t, r.Fake.Account().Count("PickPhoto"))
		assert.Zero(t, r.Fake.Account().Count("UploadAvatar"))
	})
}

// TestAvatarPickShowsThePicture checks both links ("Chụp ảnh mới" opens the library
// too, K7): the picture decoded is shown at zoom 1, offset 0, the slider at 1.
func TestAvatarPickShowsThePicture(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for name, link := range map[string]func(*screens.AvatarSheet) ui.Widget{
			"library": func(s *screens.AvatarSheet) ui.Widget { return s.Library() },
			"camera":  func(s *screens.AvatarSheet) ui.Widget { return s.Camera() },
		} {
			t.Run(name, func(t *testing.T) {
				r, sh := atAvatar(t, ph, false)
				require.Nil(t, sh.Crop().Image)
				r.Tester.ClickAt(center(link(sh)))
				r.Settle()
				assert.Equal(t, 1, r.Fake.Account().Count("PickPhoto"))
				assert.True(t, sh.Picked())
				assert.False(t, sh.Picking())
				assert.Equal(t, avatar.Picked, sh.Crop().View)
				assert.InDelta(t, 1, sh.Zoom().Value, 0)
				require.NotNil(t, sh.Crop().Image)
				assert.Equal(t, [2]int{640, 480}, [2]int{sh.Crop().Image.Width, sh.Crop().Image.Height})
				notShown(t, r)
			})
		}
	})
}

// TestAvatarLargePhotoIsReduced checks the working picture: a photo larger than the
// crop needs is brought to a shorter side of 1560 before it is shown.
func TestAvatarLargePhotoIsReduced(t *testing.T) {
	t.Parallel()

	r, sh := atAvatar(t, kittest.Phones[0], false, func(f *shelltest.Fake) { f.Account().SetPhoto(shelltest.GeneratedPhoto(2400, 1800)) })
	pickLib(r, sh)
	require.NotNil(t, sh.Crop().Image)
	assert.Equal(t, [2]int{2080, 1560}, [2]int{sh.Crop().Image.Width, sh.Crop().Image.Height})
}

// TestAvatarPickerHeldThenAnswered checks that the picture shows only once it is
// decoded, that a second tap while the picker is open does nothing, and that closing the
// sheet meanwhile leaves nothing behind.
func TestAvatarPickerHeldThenAnswered(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		release := r.Fake.Account().HoldPhoto()
		t.Cleanup(release)
		r.Tester.ClickAt(center(sh.Library()))
		r.WaitFor(sh.Picking, "the picker open")
		r.Tester.ClickAt(center(sh.Camera()))
		r.Tester.Elapse(kittest.Frame)
		assert.Nil(t, sh.Crop().Image, "nothing shows before the picture does")
		assert.False(t, sh.Picked())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Account().Count("PickPhoto"), "a second tap while it is open does nothing")
		assert.True(t, sh.Picked())
	})
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		release := r.Fake.Account().HoldPhoto()
		t.Cleanup(release)
		r.Tester.ClickAt(center(sh.Library()))
		r.WaitFor(sh.Picking, "the picker open")
		require.NoError(t, r.Tester.Click("Đóng"))
		slideIn(r)
		release()
		r.Settle()
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.False(t, sh.Picked(), "the picture of a closed sheet is dropped")
		notShown(t, r)
	})
}

// TestAvatarPickerFailures runs what the picker can end with: leaving it, a platform
// without one and a picker already open show nothing; a picture that cannot be read is
// a toast, whatever the reason (a GIF is not "unsupported" to the user).
func TestAvatarPickerFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		err   error
		photo []byte
		toast string
	}{
		{name: "left the picker", err: env.ErrPhotoCanceled},
		{name: "no picker on this platform", err: core.ErrUnsupported},
		{name: "a picker already shown", err: core.ErrDuplicate},
		{name: "a file that cannot be read", err: errors.New("the file is gone"), toast: toastPhotoBad},
		{name: "not a picture", photo: []byte("this is not a picture"), toast: toastPhotoBad},
		{name: "a GIF", photo: []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;"), toast: toastPhotoBad},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, sh := atAvatar(t, ph, false, func(f *shelltest.Fake) {
					f.Account().SetPhotoError(tc.err)
					f.Account().SetPhoto(tc.photo)
				})
				pickLib(r, sh)
				assert.False(t, sh.Picked())
				assert.False(t, sh.Picking())
				assert.Nil(t, sh.Crop().Image)
				assert.Equal(t, avatar.Initial, sh.Crop().View)
				assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
				if tc.toast == "" {
					notShown(t, r)
				} else {
					assert.True(t, shows(r, tc.toast))
				}
				// a picture picked afterwards is taken
				r.Fake.Account().SetPhotoError(nil)
				r.Fake.Account().SetPhoto(nil)
				r.Tester.Elapse(2300 * time.Millisecond)
				pickLib(r, sh)
				assert.True(t, sh.Picked())
			})
		}
	})
}

// TestAvatarDragMovesThePicture checks the finger on the crop view: the picture moves
// by what the finger moves, and past the frame's edge (no limit, Q-P6-8); the zoom
// stays; the sheet does not follow the finger down.
func TestAvatarDragMovesThePicture(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		c := center(sh.Crop())
		touchDrag(r, c, c.Add(geom.Pt(10, 5)), c.Add(geom.Pt(30, -20)))
		assert.Equal(t, avatar.View{Zoom: 1, X: 30, Y: -20}, sh.Crop().View)
		assert.False(t, sh.Crop().Dragging())
		// past the frame (353 wide, 230 tall): nothing stops it
		touchDrag(r, c, c.Add(geom.Pt(100, 0)), c.Add(geom.Pt(400, 150)))
		assert.Equal(t, avatar.View{Zoom: 1, X: 430, Y: 130}, sh.Crop().View, "no clamping")
		assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "a drag down on the crop view does not close the sheet")
		// and a mouse
		r.Tester.Drag(c, c.Add(geom.Pt(-40, 0)))
		assert.Equal(t, avatar.View{Zoom: 1, X: 390, Y: 130}, sh.Crop().View)
	})
}

// TestAvatarSheetStillFollowsAFingerElsewhere is the control of the drag test: the
// same pull on the title does close the sheet, so that the crop view's own is what keeps
// it open.
func TestAvatarSheetStillFollowsAFingerElsewhere(t *testing.T) {
	t.Parallel()

	r, sh := atAvatar(t, kittest.Phones[0], false)
	hint := element(t, r.Dump("sheet"), "p.muted#2")
	from := geom.Pt(float32(hint.X+hint.W/2), float32(hint.Y+hint.H/2))
	touchDrag(r, from, from.Add(geom.Pt(0, 60)), from.Add(geom.Pt(0, 120)), from.Add(geom.Pt(0, 200)))
	slideIn(r)
	assert.False(t, r.Shell.Nav().Sheet().IsOpen())
	assert.False(t, sh.Crop().Dragging())
}

// TestAvatarSlider checks the slider: a press on the track sets the zoom in steps of
// .01 from 1 to 3 and the picture scales about its centre where it is; the keys step
// it; the sheet does not follow a finger on it.
func TestAvatarSlider(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		c := center(sh.Crop())
		touchDrag(r, c, c.Add(geom.Pt(12, 8)))
		require.Equal(t, avatar.View{Zoom: 1, X: 12, Y: 8}, sh.Crop().View)

		r.Tester.Drag(trackPoint(sh, 2), trackPoint(sh, 2))
		assert.InDelta(t, 2, sh.Zoom().Value, .011)
		assert.InDelta(t, sh.Zoom().Value, sh.Crop().View.Zoom, 1e-9, "the picture follows the slider")
		assert.Equal(t, [2]float64{12, 8}, [2]float64{sh.Crop().View.X, sh.Crop().View.Y}, "where it is")

		r.Tester.Drag(trackPoint(sh, 2), geom.Pt(sh.Zoom().WindowFrame().Max().X+80, sh.Zoom().WindowFrame().Y))
		assert.InDelta(t, 3, sh.Zoom().Value, 0, "to the end of the track, no further")
		assert.InDelta(t, 3, sh.Crop().View.Zoom, 0)
		r.Tester.Drag(trackPoint(sh, 2), geom.Pt(sh.Zoom().WindowFrame().X-80, sh.Zoom().WindowFrame().Y))
		assert.InDelta(t, 1, sh.Zoom().Value, 0)
		assert.InDelta(t, 1, sh.Crop().View.Zoom, 0)

		// the keys: a step is .01
		r.Tester.View().Focus(sh.Zoom())
		require.True(t, r.Tester.Key(keys.KeyRight, 0))
		assert.InDelta(t, 1.01, sh.Crop().View.Zoom, 1e-9)
		require.True(t, r.Tester.Key(keys.KeyEnd, 0))
		assert.InDelta(t, 3, sh.Crop().View.Zoom, 0)
		require.True(t, r.Tester.Key(keys.KeyLeft, 0))
		assert.InDelta(t, 2.99, sh.Crop().View.Zoom, 1e-9)

		// a finger along the track: the sheet stays where it is
		f := sh.Zoom().WindowFrame()
		y := f.Y + 10
		touchDrag(r, geom.Pt(f.X+30, y), geom.Pt(f.X+60, y+30), geom.Pt(f.X+90, y+90), geom.Pt(f.X+120, y+200))
		assert.True(t, r.Shell.Nav().Sheet().IsOpen())
	})
}

// TestAvatarNewPickResets checks that a new picture starts at zoom 1, offset 0
// (`S.avatar.zoom=1;x=0;y=0`) wherever the last was left.
func TestAvatarNewPickResets(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		c := center(sh.Crop())
		touchDrag(r, c, c.Add(geom.Pt(50, -30)))
		r.Tester.Drag(trackPoint(sh, 2.5), trackPoint(sh, 2.5))
		require.NotEqual(t, avatar.Picked, sh.Crop().View)
		first := sh.Crop().Image
		pickLib(r, sh)
		assert.Equal(t, avatar.Picked, sh.Crop().View)
		assert.InDelta(t, 1, sh.Zoom().Value, 0)
		assert.NotSame(t, first, sh.Crop().Image, "a picture of its own")
	})
}

// TestAvatarConfirmUploads checks "Xác nhận": the 512 square under the ring, cut at the
// view the picture is left at, is what goes to the server; the sheet closes with the
// toast and screen 15 reads the account again.
func TestAvatarConfirmUploads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		photo := shelltest.GeneratedPhoto(640, 480)
		r, sh := atAvatar(t, ph, false, func(f *shelltest.Fake) { f.Account().SetPhoto(photo) })
		pickLib(r, sh)
		c := center(sh.Crop())
		touchDrag(r, c, c.Add(geom.Pt(-25, 10)))
		r.Tester.Drag(trackPoint(sh, 1.6), trackPoint(sh, 1.6))
		view := sh.Crop().View
		require.InDelta(t, 1.6, view.Zoom, .011)
		require.Equal(t, [2]float64{-25, 10}, [2]float64{view.X, view.Y})
		meBefore := r.Fake.Account().Count("Me")

		tap(r, sh.Accept())
		r.Settle()
		require.Equal(t, 1, r.Fake.Account().Count("UploadAvatar"))
		got := r.Fake.Account().Avatar()
		cfg, err := jpeg.DecodeConfig(bytes.NewReader(got))
		require.NoError(t, err)
		assert.Equal(t, image.Pt(512, 512), image.Pt(cfg.Width, cfg.Height), "512 × 512")
		assert.LessOrEqual(t, len(got), 2<<20, "at most 2 MB")
		work, err := avatar.Decode(photo)
		require.NoError(t, err)
		want, err := avatar.Export(work, view)
		require.NoError(t, err)
		assert.Equal(t, want, got, "Export(zoom, x, y) of the picture shown")

		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, shows(r, toastAvatarSaved))
		assert.Greater(t, r.Fake.Account().Count("Me"), meBefore, "screen 15 reads the account again")
		assert.False(t, sh.Busy())
		r.Tester.Elapse(2200 * time.Millisecond)
		assert.False(t, r.Tester.Shows(toastAvatarSaved), "the toast goes after 2200 ms")
	})
}

// TestAvatarConfirmWithoutPictureCloses is K25: with no new picture "Xác nhận" only
// closes the sheet, with no upload and no toast.
func TestAvatarConfirmWithoutPictureCloses(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		tap(r, sh.Accept())
		r.Settle()
		slideIn(r)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.Zero(t, r.Fake.Account().Count("UploadAvatar"))
		assert.Zero(t, r.Fake.Calls("me.avatar"))
		notShown(t, r)
	})
}

// TestAvatarCancelAndCross check that "Huỷ" and the cross close the sheet with nothing
// uploaded, a picture picked or not.
func TestAvatarCancelAndCross(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, closeIt := range []func(*shelltest.Rig, *screens.AvatarSheet){
			func(r *shelltest.Rig, sh *screens.AvatarSheet) { tap(r, sh.Cancel()) },
			func(r *shelltest.Rig, _ *screens.AvatarSheet) { require.NoError(t, r.Tester.Click("Đóng")) },
		} {
			r, sh := atAvatar(t, ph, false)
			pickLib(r, sh)
			closeIt(r, sh)
			r.Settle()
			slideIn(r)
			assert.False(t, r.Shell.Nav().Sheet().IsOpen())
			assert.Zero(t, r.Fake.Account().Count("UploadAvatar"))
			notShown(t, r)
			// the next time the sheet opens it is the prototype's start, the picture dropped
			tapKey(t, r, "sheet:avatar")
			slideIn(r)
			again, ok := r.Shell.Nav().Sheet().Content().(*screens.AvatarSheet)
			require.True(t, ok)
			assert.Equal(t, avatar.Initial, again.Crop().View)
			assert.False(t, again.Picked())
		}
	})
}

// TestAvatarUploadErrors runs the failures of the upload: the sheet stays open with what
// was cut, a toast tells what is wrong, and a second try goes through.
func TestAvatarUploadErrors(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		tests := []struct {
			name  string
			setup func(*shelltest.Rig)
			undo  func(*shelltest.Rig)
			toast string
		}{
			{"file too large", func(r *shelltest.Rig) { require.NoError(t, r.Fake.SetScenario("me.avatar", "file_too_large")) },
				func(r *shelltest.Rig) { require.NoError(t, r.Fake.SetScenario("me.avatar", "default")) }, toastPhotoTooLarge},
			{"offline", func(r *shelltest.Rig) { r.Fake.SetOnline(false) }, func(r *shelltest.Rig) { r.Fake.SetOnline(true) }, toastOffline},
			{"server error", func(r *shelltest.Rig) { r.Fake.Fail("me.avatar", 500, "errors/server_error.json") },
				func(r *shelltest.Rig) { r.Fake.Fail("me.avatar", 0, "") }, toastBusy},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				r, sh := atAvatar(t, ph, false)
				pickLib(r, sh)
				c := center(sh.Crop())
				touchDrag(r, c, c.Add(geom.Pt(7, 7)))
				view := sh.Crop().View
				tc.setup(r)
				tap(r, sh.Accept())
				r.Settle()
				assert.True(t, shows(r, tc.toast))
				assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "the sheet stays")
				assert.False(t, sh.Busy())
				assert.True(t, sh.Picked())
				assert.Equal(t, view, sh.Crop().View, "the picture is where it was left")
				assert.Equal(t, 1, r.Fake.Account().Count("UploadAvatar"))

				tc.undo(r)
				r.Tester.Elapse(2300 * time.Millisecond)
				tap(r, sh.Accept())
				r.Settle()
				slideIn(r)
				assert.False(t, r.Shell.Nav().Sheet().IsOpen(), "a second try goes through")
			})
		}
		t.Run("upgrade required", func(t *testing.T) {
			r, sh := atAvatar(t, ph, false)
			pickLib(r, sh)
			r.Fake.Fail("me.avatar", 426, "errors/upgrade_required.json")
			tap(r, sh.Accept())
			r.Settle()
			assert.Equal(t, env.RouteUpgrade, r.Route())
		})
	})
}

// TestAvatarConfirmOnce checks that a second tap on "Xác nhận" while the upload is out
// does nothing, and that the button looks off meanwhile.
func TestAvatarConfirmOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		release := r.Fake.Gate("me.avatar")
		t.Cleanup(release)
		tap(r, sh.Accept())
		r.WaitFor(sh.Busy, "the upload under way")
		assert.True(t, sh.Accept().Disabled)
		tap(r, sh.Accept())
		release()
		r.Settle()
		assert.Equal(t, 1, r.Fake.Calls("me.avatar"))
		assert.False(t, sh.Accept().Disabled)
	})
}

// TestAvatarCloseDropsTheUpload checks that closing the sheet while the upload is out
// shows nothing of its answer.
func TestAvatarCloseDropsTheUpload(t *testing.T) {
	t.Parallel()

	r, sh := atAvatar(t, kittest.Phones[0], false)
	pickLib(r, sh)
	release := r.Fake.Gate("me.avatar")
	t.Cleanup(release)
	tap(r, sh.Accept())
	r.WaitFor(sh.Busy, "the upload under way")
	require.NoError(t, r.Tester.Click("Đóng"))
	slideIn(r)
	release()
	r.Settle()
	assert.False(t, shows(r, toastAvatarSaved), "the answer of a closed sheet shows nothing")
}

// TestAvatarBoxesMatchPrototype lays SH.avatar over screen 15 as M15D (393 wide) within
// 1 DIP; the title's box is text Core Text sets narrower than Chrome.
func TestAvatarBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r, _ := atAvatar(t, kittest.Phones[0], true)
	lay := r.Dump("M15D")
	checkAgainstProto(t, "M15D", lay, 1, map[string]bool{"span.d": true})
	w, _ := shelltest.ProtoLayout(t, "M15D").Find("span.d")
	g := element(t, lay, "span.d")
	assert.InDelta(t, w.Y, g.Y, 1)
	assert.InDelta(t, w.W, g.W, 1.5)
	// the crop view and the slider's touch area are not in the prototype's capture:
	// the frame the sheet gives them
	crop := element(t, lay, "crop")
	assert.InDeltaSlice(t, []float64{20, 393, 353, 230}, []float64{crop.X, crop.Y, crop.W, crop.H}, 1)
	zoom := element(t, lay, "zoom")
	assert.InDeltaSlice(t, []float64{40, 660, 310.13, 20}, []float64{zoom.X, zoom.Y, zoom.W, zoom.H}, 1)
}

// TestAvatarOnTheWiderPhone checks SH.avatar on the 412 wide phone: the panel rests on
// the bottom as tall as on the iPhone, the crop view, the slider row and the buttons
// are 372 wide, the buttons share it, and the order of the parts from the top of the
// panel is the same.
func TestAvatarOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	ra, _ := atAvatar(t, kittest.Phones[0], true)
	rb, _ := atAvatar(t, kittest.Phones[1], true)
	a, b := ra.Dump("M15D"), rb.Dump("M15D")
	pa, pb := element(t, a, "div.sheet"), element(t, b, "div.sheet")
	assert.InDelta(t, 412, pb.W, 0.01)
	assert.InDelta(t, 892, pb.Y+pb.H, 0.01, "the panel rests on the bottom")
	assert.InDelta(t, pa.H, pb.H, 1.5, "as tall as on the iPhone")
	for _, key := range []string{"div.sh", "crop", "p.muted#2", "p.muted#3"} {
		e := element(t, b, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 372, e.W, 0.01, key+": width")
	}
	for _, key := range []string{"div.grab", "div.sh", "crop", "p.muted#2", "b.muted", "input", "p.muted#3", "pickPhoto", "pickPhoto#2", "closeSheet#3", "avatarOk"} {
		ea, eb := element(t, a, key), element(t, b, key)
		assert.InDelta(t, ea.Y-pa.Y, eb.Y-pb.Y, 1.5, key+": distance from the top of the panel")
		assert.InDelta(t, ea.H, eb.H, 1.5, key+": h")
	}
	// "+" and the second link at the right edge, the slider between its signs
	assert.InDelta(t, 412-20, element(t, b, "pickPhoto#2").X+element(t, b, "pickPhoto#2").W, 0.5)
	assert.InDelta(t, 412-20, element(t, b, "b.muted#2").X+element(t, b, "b.muted#2").W, 0.5)
	in := element(t, b, "input")
	assert.InDelta(t, 40, in.X, 0.01)
	assert.InDelta(t, 372-8-12-12-element(t, b, "b.muted#2").W, in.W, 0.02)
	// the buttons share 372 as flex-shrink does: 372 + 12 over, by their inner widths
	l, rr := element(t, b, "closeSheet#3"), element(t, b, "avatarOk")
	assert.InDelta(t, 372, l.W+rr.W+12, 0.01)
	assert.InDelta(t, 20, l.X, 0.01)
	assert.InDelta(t, 412-20, rr.X+rr.W, 0.01)
	assert.Greater(t, l.W, rr.W, "the outlined button, whose border is not counted, gives up less")
	assert.InDelta(t, 54, l.H, 0.01)
}

// TestAvatarSteadyFrameDoesNotAllocate checks that a frame of the sheet standing still
// allocates nothing with the picture shown, on both phones.
func TestAvatarSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, sh := atAvatar(t, ph, false)
		for range 40 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": the figure")
		pickLib(r, sh)
		for range 40 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name+": the picture")
	}
}

// TestCropDragDoesNotAllocate checks that a move of the finger on the crop view, with
// the frame it repaints, allocates nothing.
func TestCropDragDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		c := center(sh.Crop())
		r.Tester.TouchDown(c)
		r.Tester.Elapse(kittest.Frame)
		flip := float32(1)
		move := func() {
			flip = -flip
			r.Tester.TouchMove(c.Add(geom.Pt(40*flip, 25*flip)))
			r.Tester.Frame()
		}
		for range 10 {
			move()
		}
		require.True(t, sh.Crop().Dragging(), "the finger is moving the picture")
		assert.Zero(t, testing.AllocsPerRun(50, move), ph.Name)
		r.Tester.TouchUp(c)
	}
}

// TestSliderDragDoesNotAllocate checks the same for the zoom slider.
func TestSliderDragDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r, sh := atAvatar(t, ph, false)
		pickLib(r, sh)
		r.Tester.Press(trackPoint(sh, 1.5))
		r.Tester.Elapse(kittest.Frame)
		flip := 1.0
		move := func() {
			flip = -flip
			r.Tester.Hover(trackPoint(sh, 2+.5*flip))
			r.Tester.Frame()
		}
		for range 10 {
			move()
		}
		require.True(t, sh.Zoom().Pressed(), "the press follows the pointer")
		assert.Zero(t, testing.AllocsPerRun(50, move), ph.Name)
		r.Tester.Release(trackPoint(sh, 2))
	}
}
