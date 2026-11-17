//go:build !tvos

// Pattern from jplayer-mobile/internal/ui/sheets/a11y_test.go and
// jplayer-mobile/internal/ui/kit/a11y_test.go at 49de3c2 (what a screen reader
// reads of a screen, found by the tree Tester.Semantics returns: labels, roles,
// the layers under a sheet), over every capture state of the prototype.

package screens_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/sys"
	"dancefitvn-app/internal/ui/icon"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/shell"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// a11yState is a capture state of the prototype as the app reaches it in a test:
// the rig at it, rested.
type a11yState struct {
	// id is the state's id in local/shots (M01 to M16, M-toast).
	id string
	// sheet tells that a sheet is open over the route.
	sheet bool
	open  func(t *testing.T, ph kittest.Phone) *shelltest.Rig
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// a11yStates are the capture states of the app, one for each screen and each
// sheet: M01 to M16 (the screens and their variants), the toast and the sheets.
func a11yStates() []a11yState {
	return []a11yState{
		{id: "M01", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r := newRig(t, ph, false, shell.Options{HoldSplash: true}, signed())
			r.Boot()

			return r
		}},
		{id: "M02", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atLogin(t, ph, false, bio(sys.BiometryFace, nil), lastEmail(meEmail))

			return r
		}},
		{id: "M03", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atHome(t, ph, false) }},
		{id: "M03B", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atHomeLoading(t, ph, false) }},
		{id: "M04", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atCourses(t, ph, false) }},
		{id: "M05", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atDetail(t, ph, false, "set-35") }},
		{id: "M05B", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atDetail(t, ph, false, "set-36") }},
		{id: "M06", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _, _ := atVideo(t, ph, false)

			return r
		}},
		{id: "M07", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _, _ := atVideo(t, ph, false)
			openOptionsSheet(t, r)

			return r
		}},
		{id: "M08", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _, _ := atAudio(t, ph, false)

			return r
		}},
		{id: "M09", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return openDownloadSheet(t, ph, false) }},
		{id: "M10", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _, _ := atDoneAt(t, ph, false)

			return r
		}},
		{id: "M11", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atDownloads(t, ph, false) }},
		{id: "M12", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atDownloads(t, ph, false, emptied) }},
		{id: "M13", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atOffline(t, ph, false) }},
		{id: "M14", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return openDeleteSheet(t, ph, false) }},
		{id: "M15", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atAccount(t, ph, false) }},
		{id: "M15B", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atInfo(t, ph, false)

			return r
		}},
		{id: "M15B-password", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atPassword(t, ph, false)

			return r
		}},
		{id: "M15C", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atCoach(t, ph, false)

			return r
		}},
		{id: "M15D", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atAvatar(t, ph, false)

			return r
		}},
		{id: "M15-logout", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig { return atLogoutSheet(t, ph, false) }},
		{id: "M16", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atConnect(t, ph, false)

			return r
		}},
		{id: "M16-code", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, c := atConnect(t, ph, false)
			typeCode(r, c, "7k4-m2p")

			return r
		}},
		{id: "M16-linked", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _ := atConnectLinked(t, ph, false)

			return r
		}},
		{id: "M16-devices", sheet: true, open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r, _, _ := atLimit(t, ph, false)

			return r
		}},
		{id: "M-toast", open: func(t *testing.T, ph kittest.Phone) *shelltest.Rig {
			r := atDetail(t, ph, false, "set-35")
			r.Tester.Semantics() // a screen reader is listening
			r.Shell.Env().Act.Toast(toastText)
			r.Tester.Elapse(300 * time.Millisecond)

			return r
		}},
	}
}

// toastText is the toast of M-toast.
const toastText = "Đã tải xong bài 03"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestA11yEveryStateIsLabelled checks, in every capture state on both phones,
// what a screen reader finds: every element it can press has a label that is no
// icon key; every pressable widget of what is reachable is an element of its own
// (no button lost inside another); a sheet leaves nothing of what is under it
// (the screen, the tab bar); the screen has a heading of level 1 or its sheet has.
func TestA11yEveryStateIsLabelled(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, s := range a11yStates() {
			t.Run(s.id, func(t *testing.T) {
				t.Parallel()
				r := s.open(t, ph)
				sem := r.Tester.Semantics()
				require.NotEmpty(t, sem.Nodes)
				if os.Getenv("A11Y_DUMP") != "" {
					t.Logf("\n%s\n%s", s.id, dumpTree(sem))
				}
				checkLabels(t, sem)
				checkPressablesAreElements(t, r, s, sem)
				if s.sheet {
					checkNothingUnderTheSheet(t, r, sem)
				}
				if !strings.HasPrefix(s.id, "M01") && s.id != "M03B" {
					checkHasTitle(t, sem)
				}
			})
		}
	})
}

// TestA11yReadingOrderIsVisualOrder checks the order of the elements of M03, M05,
// M06 and M15 against the order of the screen, top to bottom: the labels as a
// screen reader walks them, on both phones (one layout).
func TestA11yReadingOrderIsVisualOrder(t *testing.T) {
	t.Parallel()

	golden := map[string][]string{
		"M03": {
			"Thứ bảy, 14/11 · Chào buổi sáng", "Xin chào, Thảo Vy", "Tài khoản", "Stink and Dutty",
			"15", "Khoá học", "10", "Bài · Set 35", "5", "Audio offline",
			"Khoá học của tôi", "Xem tất cả",
			"DanceFIT Set 35", "DanceFIT Set 34", "DanceFIT Hot Extra", "DanceFIT Set 33", "DanceFIT Set 32", "DanceFIT Set 31",
			"Mới cập nhật", "DanceFIT Set 36",
			"Trang chủ", "Khoá học", "Tải xuống", "Tài khoản",
		},
		"M05": {
			"Quay lại", "Tuỳ chọn phát", "Khoá học Thường · 10 bài học", "DanceFIT Set 35",
			"“Cùng nhảy cùng hát” · 10 bài · 39 phút", "Đang xem bài 03", "Tải hết", "Bài học · 10", "Giới thiệu",
			"DanceFIT Warm Up 13", "Đã tải", "Made You Look", "Đã tải", "Stink and Dutty", "Đã tải",
			"Em Yêu Người Ta", "Đã tải", "Hãy Hát Lên", "Tải xuống", "Nadi Nadi", "Đã tải",
			"Viva La Vida", "Huỷ tải", "We So Fly", "Tải xuống", "WET", "Tải xuống", "One Love", "Tải xuống",
			"Tiếp tục bài 03",
		},
		"M06": {
			"Stink and Dutty", // the picture, `stageTap`
			"Quay lại", "DanceFIT Set 35", "Bài 03 / 10", "Tuỳ chọn phát",
			"Tốc độ phát", "Gương", "Lặp A–B", "Đã tải",
			"Bụng", "Video dọc 9:16", "Stink and Dutty", "DanceFIT Set 35 · “Cùng nhảy cùng hát”", "Thanh đếm 8 nhịp",
			"Vị trí phát", "01:12", "03:40",
			"Bài trước", "Lùi 10 giây", "Tạm dừng", "Tới 10 giây", "Bài sau",
		},
		"M15": {
			"Tài khoản", "Chỉnh sửa ảnh đại diện", "Nguyễn Thảo Vy", "Thành viên từ 12/03/2024", "thaovy.hlv@gmail.com",
			"Huy hiệu", "HDV DanceFIT", "HDV StepX", "HDV BodyX",
			"Hồ sơ", "Thông tin tài khoản", "Hồ sơ huấn luyện viên", "Đổi mật khẩu",
			"Ứng dụng", "Kết nối Apple TV", "Chỉ tải qua Wi‑Fi", "Tự phát bài tiếp theo", "File âm thanh đã tải",
			"Đăng xuất", "DanceFITVN",
			"Trang chủ", "Khoá học", "Tải xuống", "Tài khoản",
		},
	}
	states := map[string]a11yState{}
	for _, s := range a11yStates() {
		states[s.id] = s
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for id, want := range golden {
			t.Run(id, func(t *testing.T) {
				t.Parallel()
				r := states[id].open(t, ph)
				var got []string
				for _, n := range r.Tester.Semantics().Nodes[1:] {
					if n.Role == platform.RoleScroll {
						continue // an area, not a thing to read
					}
					got = append(got, n.Label)
				}
				assert.Equal(t, want, got)
			})
		}
	})
}

// TestA11yVideoControlsAreHushedWhenHidden checks 06 (Q-P4.4-3): from the moment
// the auto-hide hides the controls, 3000 ms after the last touch, the tree holds
// no control and none takes a press; the picture is then one button named by the
// lesson's title, whose press shows them again.
func TestA11yVideoControlsAreHushedWhenHidden(t *testing.T) {
	t.Parallel()

	controls := []string{
		"Quay lại", "Tuỳ chọn phát", "Tốc độ phát", "Gương", "Lặp A–B", "Đã tải", "Thanh đếm 8 nhịp",
		"Vị trí phát", "Bài trước", "Lùi 10 giây", "Tạm dừng", "Tới 10 giây", "Bài sau",
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		require.True(t, v.Shown())
		sem := r.Tester.Semantics()
		for _, l := range controls {
			assert.Truef(t, hasLabel(sem, l), "%q is read while the controls show", l)
		}
		stage, ok := findNode(sem, "Stink and Dutty", platform.RoleButton)
		require.True(t, ok, "the picture is a button named by the lesson's title")
		assert.True(t, stage.Actions.Has(platform.ActPress))

		r.Tester.Elapse(2999 * time.Millisecond)
		assert.True(t, v.Shown())
		assert.True(t, hasLabel(r.Tester.Semantics(), "Tạm dừng"), "2999 ms: still read")
		r.Tester.Elapse(time.Millisecond)
		require.False(t, v.Shown(), "3000 ms")
		// from that moment, not after the fade (250 ms)
		sem = r.Tester.Semantics()
		for _, l := range controls {
			assert.Falsef(t, hasLabel(sem, l), "%q is not read once the controls hide", l)
		}
		assert.False(t, r.Tester.PerformAction("Tạm dừng", platform.ActPress), "a hidden control takes no press")
		assert.False(t, r.Tester.PerformAction("Quay lại", platform.ActPress))
		assert.Zero(t, pb.Count("Toggle"))
		assert.Equal(t, 2, len(sem.Nodes), "the window and the picture: %s", dumpTree(sem))
		assert.True(t, pb.Playing())

		changes := r.Tester.SemanticsChanges(platform.ChangeLayout)
		require.True(t, r.Tester.PerformAction("Stink and Dutty", platform.ActPress), "the picture is pressed")
		assert.True(t, v.Shown(), "StageTap shows the controls")
		sem = r.Tester.Semantics()
		for _, l := range controls {
			assert.Truef(t, hasLabel(sem, l), "%q is back", l)
		}
		r.Tester.Elapse(100 * time.Millisecond) // a layout change is told once every 100 ms
		assert.Greater(t, r.Tester.SemanticsChanges(platform.ChangeLayout), changes, "the change is told")
		r.Tester.Elapse(3000 * time.Millisecond)
		assert.False(t, v.Shown(), "the 3 s started again")
		assert.False(t, r.Tester.PerformAction("Tạm dừng", platform.ActPress))
	})
}

// TestA11yHushIsToldAtOnce checks that a screen reader that read the tree is told
// that the controls went (ChangeLayout) at the moment the auto-hide hides them,
// not after the fade.
func TestA11yHushIsToldAtOnce(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, v := atVideo(t, ph, false)
		restart(r, pb)
		r.Tester.Semantics()
		r.Tester.Elapse(2900 * time.Millisecond)
		before := r.Tester.SemanticsChanges(platform.ChangeLayout)
		r.Tester.Elapse(100 * time.Millisecond)
		require.False(t, v.Shown())
		assert.Greater(t, r.Tester.SemanticsChanges(platform.ChangeLayout), before)
	})
}

// TestA11ySheetHushesTheScreenUnderIt checks the options sheet over 06: while it is
// open nothing of the video is read (not its stage either), nothing is pressed
// under it, and once it closes the controls are read again.
func TestA11ySheetHushesTheScreenUnderIt(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, _, _ := atVideo(t, ph, false)
		sem := r.Tester.Semantics()
		require.True(t, hasLabel(sem, "Quay lại"))
		changes := r.Tester.SemanticsChanges(platform.ChangeScreen)
		openOptionsSheet(t, r)
		sem = r.Tester.Semantics()
		assert.Greater(t, r.Tester.SemanticsChanges(platform.ChangeScreen), changes, "the sheet tells a new screen")
		assert.False(t, hasLabel(sem, "Quay lại"), "the screen under the sheet is not read")
		assert.False(t, hasLabel(sem, "Stink and Dutty"), "nor its stage")
		assert.False(t, r.Tester.PerformAction("Quay lại", platform.ActPress), "nor pressed")
		assert.True(t, hasLabel(sem, "Tuỳ chọn phát"))
		assert.True(t, r.Tester.PerformAction("Xong", platform.ActPress), "the sheet is")
		r.Tester.Elapse(kittest.Frame)
		assert.False(t, r.Shell.Nav().Sheet().IsOpen())
		assert.True(t, hasLabel(r.Tester.Semantics(), "Quay lại"), "the screen is read again")
	})
}

// TestA11yToastIsAnnouncedAndHidesNothing checks the toast of M-toast: announced
// once, a status in the tree after the screen, and the screen under it still read
// and pressed.
func TestA11yToastIsAnnouncedAndHidesNothing(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		r.Tester.Semantics()
		r.Shell.Env().Act.Toast(toastText)
		r.Tester.Elapse(300 * time.Millisecond)
		assert.Equal(t, []string{toastText}, r.Tester.Announcements())
		sem := r.Tester.Semantics()
		toast, ok := findNode(sem, toastText, platform.RoleStatus)
		require.True(t, ok, "the toast is a status")
		last := sem.Nodes[len(sem.Nodes)-1]
		assert.Equal(t, toast.ID, last.ID, "after the screen, as it lies over it")
		assert.True(t, hasLabel(sem, "Quay lại"), "the screen is still read")
		assert.True(t, hasLabel(sem, "Tải hết"))
		assert.True(t, r.Tester.PerformAction("Tải hết", platform.ActPress), "and pressed")
		r.Tester.Elapse(2300 * time.Millisecond)
		assert.False(t, hasLabel(r.Tester.Semantics(), toastText), "gone with the toast")
	})
}

// TestA11yScreenChangesAreTold checks that a push and a sheet tell a new screen to
// a screen reader that is reading: it starts again from the first element.
func TestA11yScreenChangesAreTold(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atHome(t, ph, false)
		r.Tester.Semantics()
		n := r.Tester.SemanticsChanges(platform.ChangeScreen)
		r.Shell.Go(nav.Route{Screen: env.RouteCourses}, true)
		r.Tester.Elapse(kittest.Frame)
		n2 := r.Tester.SemanticsChanges(platform.ChangeScreen)
		assert.Greater(t, n2, n, "a screen pushed")
		r.Tester.Semantics()
		r.Shell.AskLogout()
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		assert.Greater(t, r.Tester.SemanticsChanges(platform.ChangeScreen), n2, "a sheet opened")
	})
}

// TestA11yMagicTapPlaysAndPauses checks that the magic tap of a screen reader
// plays or pauses on screen 06 and on screen 08, whatever holds the focus.
func TestA11yMagicTapPlaysAndPauses(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("video", func(t *testing.T) {
			t.Parallel()
			r, pb, _ := atVideo(t, ph, false)
			require.True(t, pb.Playing())
			assert.True(t, r.Tester.MagicTap())
			assert.False(t, pb.Playing(), "paused")
			assert.True(t, r.Tester.MagicTap())
			assert.True(t, pb.Playing(), "playing")
			assert.Equal(t, 2, pb.Count("Toggle"))
		})
		t.Run("audio", func(t *testing.T) {
			t.Parallel()
			r, pb, _ := atAudio(t, ph, false)
			require.False(t, pb.Playing())
			assert.True(t, r.Tester.MagicTap())
			assert.True(t, pb.Playing(), "playing")
			assert.True(t, r.Tester.MagicTap())
			assert.False(t, pb.Playing(), "paused")
		})
		t.Run("elsewhere", func(t *testing.T) {
			t.Parallel()
			r := atHome(t, ph, false)
			assert.False(t, r.Tester.MagicTap(), "no player here: the system plays or pauses what played last")
		})
	})
}

// TestA11yPositionBarsAreSliders checks the scrub of 06 and the wave of 08: a
// slider from 0 to the length at the position, spoken as a clock, that a screen
// reader steps by 10 s with increment and decrement.
func TestA11yPositionBarsAreSliders(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, screen := range []string{"video", "audio"} {
			t.Run(screen, func(t *testing.T) {
				t.Parallel()
				var r *shelltest.Rig
				var pb *shelltest.FakePlayback
				if screen == "video" {
					r, pb, _ = atVideo(t, ph, false)
				} else {
					r, pb, _ = atAudio(t, ph, false)
				}
				n, ok := findNode(r.Tester.Semantics(), "Vị trí phát", platform.RoleSlider)
				require.True(t, ok)
				assert.InDelta(t, 0, n.Min, 0)
				assert.InDelta(t, 220, n.Max, 0)
				assert.InDelta(t, 72, n.Now, 0)
				assert.Equal(t, "01:12", n.Value)
				assert.True(t, n.Actions.Has(platform.ActIncrement))
				assert.True(t, n.Actions.Has(platform.ActDecrement))
				pb.ClearCalls()
				assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActIncrement))
				assert.True(t, r.Tester.PerformAction("Vị trí phát", platform.ActDecrement))
				assert.False(t, r.Tester.PerformAction("Vị trí phát", platform.ActPress), "a slider is not pressed")
				assert.Equal(t, 2, pb.Count("SeekBy"), "+10 s and -10 s")
			})
		}
	})
}

// TestA11ySpeedAndLoopSayTheirValues checks the side buttons of 06: the speed is
// named by its prototype string with its text as the value, the mirror and the
// loop are toggles and the loop speaks its range.
func TestA11ySpeedAndLoopSayTheirValues(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r, pb, _ := atVideo(t, ph, false)
		restart(r, pb)
		sem := r.Tester.Semantics()
		speed, ok := findNode(sem, "Tốc độ phát", platform.RoleButton)
		require.True(t, ok)
		assert.Equal(t, "0.75x", speed.Value)
		mirror, _ := findNode(sem, "Gương", platform.RoleButton)
		assert.Equal(t, platform.ToggledOn, mirror.Toggle)
		loop, _ := findNode(sem, "Lặp A–B", platform.RoleButton)
		assert.Equal(t, platform.ToggledOff, loop.Toggle)
		assert.Empty(t, loop.Value)
		prev, ok := findNode(sem, "Bài trước", platform.RoleButton)
		require.True(t, ok)
		assert.Equal(t, "02", prev.Value)

		pb.SetLoop(true)
		r.Settle()
		loop, _ = findNode(r.Tester.Semantics(), "Lặp A–B", platform.RoleButton)
		assert.Equal(t, platform.ToggledOn, loop.Toggle)
		assert.Equal(t, "01:12–01:40", loop.Value, "A to B")
	})
}

// TestA11yChoicesAreSelectedButtonsNotToggles checks the tabs of the tab bar and the
// speeds of the options sheet (P8.3d, after the simulator dump): each one is a
// button that is selected or not, with no toggle state: the iOS bridge reads a
// toggle as a "toggle button" that speaks 0 or 1.
func TestA11yChoicesAreSelectedButtonsNotToggles(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		selected := func(sem *platform.Semantics, role platform.SemanticRole, labels ...string) (on []string) {
			for _, l := range labels {
				n, ok := findNode(sem, l, role)
				require.Truef(t, ok, "%q as role %d", l, role)
				assert.Equalf(t, platform.ToggledUnknown, n.Toggle, "%q is no toggle", l)
				assert.Truef(t, n.Actions.Has(platform.ActPress), "%q takes a press", l)
				if n.Flags.Has(platform.FlagSelected) {
					on = append(on, l)
				}
			}

			return on
		}
		home := atHome(t, ph, false)
		assert.Equal(t, []string{"Trang chủ"},
			selected(home.Tester.Semantics(), platform.RoleTab, "Trang chủ", "Khoá học", "Tải xuống", "Tài khoản"), "the tabs")

		var sheet *shelltest.Rig
		for _, s := range a11yStates() {
			if s.id == "M07" {
				sheet = s.open(t, ph)
			}
		}
		require.NotNil(t, sheet)
		assert.Equal(t, []string{"0.75x"},
			selected(sheet.Tester.Semantics(), platform.RoleRadio, "0.5x", "0.75x", "1x", "1.25x", "1.5x"), "the speeds")
	})
}

// TestA11yDownloadButtonsSayWhichLesson checks M05 (P8.3d): the download button of
// every lesson keeps its label ("Tải xuống", "Huỷ tải", "Đã tải") and is valued by
// the lesson, "Bài 03 · Stink and Dutty", so that a list of them tells them apart.
func TestA11yDownloadButtonsSayWhichLesson(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-35")
		sem := r.Tester.Semantics()
		values := map[string]int{}
		buttons := 0
		for _, n := range sem.Nodes {
			if n.Role != platform.RoleButton || (n.Label != "Tải xuống" && n.Label != "Huỷ tải" && n.Label != "Đã tải") {
				continue
			}
			buttons++
			assert.Regexpf(t, `^Bài \d\d · .+`, n.Value, "the %q button says which lesson", n.Label)
			values[n.Value]++
		}
		assert.Equal(t, 10, buttons, "one for each lesson of the course")
		assert.Len(t, values, 10, "ten different lessons")
		assert.Contains(t, values, "Bài 03 · Stink and Dutty")
	})
}

// TestA11yFileRowsHoldTheirButtons checks screen 11 and the detail: a row with a
// button of its own is a list item, whose button is an element under it that a
// screen reader presses (a button holds no elements).
func TestA11yFileRowsHoldTheirButtons(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDownloads(t, ph, false)
		sem := r.Tester.Semantics()
		row, ok := findNode(sem, "03 · Stink and Dutty", platform.RoleListItem)
		require.True(t, ok)
		assert.Equal(t, "Bụng · 03:40, 8,8 MB", row.Value)
		child := childOf(sem, row, "Tuỳ chọn file")
		require.NotNil(t, child, "the button of options is an element under the row")
		assert.True(t, child.Actions.Has(platform.ActPress))
		assert.True(t, r.Tester.PerformAction("Tuỳ chọn file", platform.ActPress))
		r.Settle()
		r.Tester.Elapse(kittest.Frame)
		assert.True(t, r.Shell.Nav().Sheet().IsOpen(), "its press opens the sheet to delete the file")
	})
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// dumpTree writes the elements of a reading, one a line.
func dumpTree(sem *platform.Semantics) string {
	var b strings.Builder
	for i, n := range sem.Nodes {
		fmt.Fprintf(&b, "%3d p%-3d role=%d L%d %q val=%q act=%b fl=%b tg=%d [%.0f,%.0f %.0fx%.0f]\n",
			i, n.Parent, n.Role, n.Level, n.Label, n.Value, n.Actions, n.Flags, n.Toggle,
			n.Bounds.X, n.Bounds.Y, n.Bounds.W, n.Bounds.H)
	}

	return b.String()
}

// hasLabel tells whether an element of the reading is called label.
func hasLabel(sem *platform.Semantics, label string) bool {
	for _, n := range sem.Nodes {
		if n.Label == label {
			return true
		}
	}

	return false
}

// findNode returns the first element called label of that role.
func findNode(sem *platform.Semantics, label string, role platform.SemanticRole) (platform.SemanticNode, bool) {
	for _, n := range sem.Nodes {
		if n.Label == label && n.Role == role {
			return n, true
		}
	}

	return platform.SemanticNode{}, false
}

// childOf returns the element called label under parent, nil when it has none.
func childOf(sem *platform.Semantics, parent platform.SemanticNode, label string) *platform.SemanticNode {
	for i := range sem.Nodes {
		n := &sem.Nodes[i]
		if n.Label == label && n.Parent >= 0 && sem.Nodes[n.Parent].ID == parent.ID {
			return n
		}
	}

	return nil
}

// iconKeys are the keys of the icons, which a label must not be.
func iconKeys() map[string]bool {
	m := map[string]bool{}
	for _, n := range icon.Names() {
		m[n.String()] = true
	}

	return m
}

// checkLabels checks the first assertion: each element that takes a press, a step
// or the focus has a label, and no label is the key of an icon.
func checkLabels(t *testing.T, sem *platform.Semantics) {
	t.Helper()
	keys := iconKeys()
	for i, n := range sem.Nodes {
		if i == 0 {
			continue
		}
		pressable := n.Actions.Has(platform.ActPress) || n.Actions.Has(platform.ActIncrement) || n.Actions.Has(platform.ActDecrement)
		if pressable {
			assert.NotEmptyf(t, n.Label, "element %d (role %d at %v) takes an action and has no label", i, n.Role, n.Bounds)
		}
		assert.Falsef(t, keys[n.Label], "element %d is labelled with the icon key %q", i, n.Label)
		if n.Role == platform.RoleSlider {
			assert.NotEmptyf(t, n.Label, "slider %d", i)
			assert.Greaterf(t, n.Max, n.Min, "slider %q has a range", n.Label)
			assert.GreaterOrEqualf(t, n.Now, n.Min, "slider %q is in its range", n.Label)
			assert.LessOrEqualf(t, n.Now, n.Max, "slider %q is in its range", n.Label)
			assert.Truef(t, n.Actions.Has(platform.ActIncrement|platform.ActDecrement), "slider %q steps", n.Label)
		}
	}
}

// checkHasTitle checks that the reading has a heading of level 1: the title of
// the screen, or of the sheet over it.
func checkHasTitle(t *testing.T, sem *platform.Semantics) {
	t.Helper()
	for _, n := range sem.Nodes {
		if n.Role == platform.RoleHeading && n.Level == 1 {
			return
		}
	}
	t.Errorf("no heading of level 1:\n%s", dumpTree(sem))
}

// checkNothingUnderTheSheet checks the second assertion: with a sheet open every
// element but the window lies in the sheet's panel, and none is a tab.
func checkNothingUnderTheSheet(t *testing.T, r *shelltest.Rig, sem *platform.Semantics) {
	t.Helper()
	require.True(t, r.Shell.Nav().Sheet().IsOpen())
	panel, ok := r.Dump("sheet").Find("div.sheet")
	require.True(t, ok, "the panel of the sheet")
	frame := geom.XYWH(float32(panel.X)-1, float32(panel.Y)-1, float32(panel.W)+2, float32(panel.H)+2)
	for i, n := range sem.Nodes {
		if i == 0 {
			continue
		}
		assert.NotEqualf(t, platform.RoleTab, n.Role, "the tab bar is read under the sheet: %q", n.Label)
		assert.Truef(t, frame.Contains(n.Bounds.Min()) && frame.Contains(n.Bounds.Max()),
			"element %d %q at %v is not in the sheet %v: the screen under it is read", i, n.Label, n.Bounds, frame)
	}
}

// checkPressablesAreElements checks that each pressable widget that a screen
// reader can reach (the sheet's content with a sheet open, else the whole
// screen) is an element with a press and the widget's own box: a button inside
// another is no element unless the other says so (DescribeChildren).
func checkPressablesAreElements(t *testing.T, r *shelltest.Rig, s a11yState, sem *platform.Semantics) {
	t.Helper()
	var root ui.Widget = r.Tester.View().Root()
	if s.sheet {
		root = r.Shell.Nav().Sheet().Content()
		require.NotNil(t, root)
	}
	var visit func(w ui.Widget)
	visit = func(w ui.Widget) {
		b := w.Node()
		if b.Hidden {
			return
		}
		if p, ok := w.(interface{ Press() }); ok && !b.Disabled {
			_ = p
			f := b.WindowFrame()
			if f.W > 0 && f.H > 0 {
				found := false
				for _, n := range sem.Nodes {
					if n.Actions.Has(platform.ActPress) && near(n.Bounds, f) {
						found = true

						break
					}
				}
				assert.Truef(t, found, "%T %q at %v takes a press and no element of a reading is it", w, b.Name, f)
			}
		}
		for c := range b.Children() {
			visit(c)
		}
	}
	visit(root)
}

// near tells whether two boxes are the same, within a DIP and a half.
func near(a, b geom.Rect) bool {
	const tol = 1.5
	d := func(x, y float32) bool {
		if x < y {
			x, y = y, x
		}

		return x-y <= tol
	}

	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.W, b.W) && d(a.H, b.H)
}
