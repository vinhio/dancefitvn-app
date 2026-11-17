//go:build !tvos

// Pattern from courses_test.go (the rig, the proto comparison, the wider phone) and
// detail_test.go.

package screens_test

import (
	"testing"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/kit/kittest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/framework/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// lockedK30Skip are the prototype's elements of the footer's button that K30 takes
// out: the anchor and the globe in it.
var lockedK30Skip = map[string]bool{"a.btn.p": true, "svg.i#6": true}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestLockedBoxesMatchPrototype lays the screen of Set 36, the course the user has not
// joined, out with the iPhone's real text engine and compares every element of M05B with
// the prototype's within 1 DIP: the cover, the back button alone, the title block as wide
// as its widest line, the card with its lock, the two headings and the description, the
// four rows with the preview's chip and the dim ones' locks, the support line. The
// footer's anchor with its globe is the prototype's and not the app's (K30): it is
// the contact text now. The card's text is the user's new one (K30), so its width and
// height follow the text, not the prototype's.
func TestLockedBoxesMatchPrototype(t *testing.T) {
	t.Parallel()

	r := atDetail(t, kittest.Phones[0], true, "set-36")
	checkAgainstProto(t, "M05B", r.Dump("M05B"), 1, lockedK30Skip)
}

// TestLockedOnTheWiderPhone checks the 412 wide phone's locked course against the
// iPhone's: the same column from the top, what runs the width 19 wider (the card, the
// description, the rows with their chip and locks at the right edge), the title block
// still as wide as its text, the footer on the bottom of the taller screen.
func TestLockedOnTheWiderPhone(t *testing.T) {
	t.Parallel()

	rI := atDetail(t, kittest.Phones[0], true, "set-36")
	rA := atDetail(t, kittest.Phones[1], true, "set-36")
	a, b := rI.Dump("M05B"), rA.Dump("M05B")
	require.Len(t, b.Elements, len(a.Elements))
	for i, ea := range a.Elements {
		eb := b.Elements[i]
		require.Equal(t, ea.Key, eb.Key)
		if ea.Key == "p.muted#4" {
			assert.InDelta(t, 892-38, eb.Y+eb.H, 0.01, "the contact text ends 38 over the bottom")

			break
		}
		assert.InDelta(t, ea.Y, eb.Y, 1.5, ea.Key+": y")
	}
	for _, key := range []string{"div.card", "div.d#2", "p.muted#2", "toast", "toast#4", "p.muted#3", "p.muted#4"} {
		e, ok := b.Find(key)
		require.True(t, ok, key)
		assert.InDelta(t, 20, e.X, 0.01, key+": left")
		assert.InDelta(t, 412-40, e.W, 0.01, key+": width")
	}
	for _, key := range []string{"div.eb.acc", "div.d", "div.muted"} {
		ea, _ := a.Find(key)
		eb, _ := b.Find(key)
		assert.InDelta(t, ea.W, eb.W, 2, key+": as wide as its text")
		assert.Less(t, eb.W, 300.0, key)
	}
	for _, key := range []string{"span.chip.y", "svg.i#3", "svg.i#5"} {
		ea, _ := a.Find(key)
		eb, _ := b.Find(key)
		assert.InDelta(t, ea.X+19, eb.X, 0.5, key+": moves with the right edge")
	}
	p, _ := b.Find("p.muted")
	assert.InDelta(t, 273+19, p.W, 0.01, "the card's text takes the room")
}

// TestLockedShowsTheMockCourse checks the texts of the locked course from the mock's
// Set 36: the eyebrow, the sub line, the card's title and text, the description from
// `description`, "Nội dung · 10 bài", the preview and the three locked rows (four only,
// F-16), the support line from the app config, the contact text of K30; and that nothing
// of the detail is there.
func TestLockedShowsTheMockCourse(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-36")
		for _, s := range []string{
			"Khoá học Thường · 10 bài học", "DanceFIT Set 36", "“Khoá mới” · 10 bài học",
			"Bạn chưa tham gia khoá học này",
			"Khi tham gia khoá học, khoá học sẽ tự xuất hiện trong “Khoá học của tôi”.",
			"Giới thiệu",
			"10 bài nhảy theo nhạc, mỗi bài có video hướng dẫn và file âm thanh để huấn luyện viên dạy lại cho học viên tại lớp.",
			"Nội dung · 10 bài",
			"DanceFIT Warm Up 14", "Warm Up · Xem thử", "XEM THỬ",
			"Bài 02", "Bài 03", "Bài 04", "Mở khi tham gia khoá học",
			"Cần hỗ trợ? Gọi Cô Leyna · 036 800 8268", "Liên hệ DanceFITVN để tham gia khoá học",
		} {
			assert.True(t, r.Tester.Shows(s), s)
		}
		assert.False(t, r.Tester.Shows("Đăng ký trên website"), "K30: no enroll button")
		assert.False(t, r.Tester.Shows("Bài 05"), "four rows only")
		assert.False(t, r.Tester.Shows("Tải hết"))
		assert.False(t, r.Tester.Shows("Tiếp tục bài 01"))
		assert.True(t, r.Shell.Nav().TabBar().Hidden, "no tab bar")
		d := detailOf(t, r)
		require.NotNil(t, d.Locked())
		assert.Nil(t, d.Content())
		assert.Len(t, d.Locked().Rows(), 4)
	})
}

// TestLockedTaps checks the controls: a dim row toasts "Mở khi tham gia khoá học" for
// 2200 ms, the preview does nothing yet (P4, Q-P3-21), Back goes back.
func TestLockedTaps(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-36")
		root := r.Tester.View().Root()
		detailOf(t, r).Scroll().ScrollTo(geom.Pt(0, 100), false) // the footer covers the last row from 744
		r.Tester.Elapse(kittest.Frame)
		tap(r, named(root, "toast", 1))
		r.Tester.Elapse(100 * ms)
		assert.False(t, r.Shell.Nav().Toasts().Visible(), "the preview has no toast (K2)")
		assert.Equal(t, "detail", r.Route())

		for n := 2; n <= 4; n++ {
			tap(r, named(root, "toast", n))
			r.Tester.Elapse(100 * ms)
			toast := r.Shell.Nav().Toasts()
			assert.True(t, toast.Visible(), "row %d", n)
			assert.Equal(t, "Mở khi tham gia khoá học", toast.Message(), "row %d", n)
			r.Tester.Elapse(2300 * ms)
			assert.False(t, toast.Visible(), "row %d: the toast ends after 2200 ms", n)
		}

		detailOf(t, r).Scroll().ScrollTo(geom.Point{}, false)
		r.Tester.Elapse(kittest.Frame)
		require.NoError(t, r.Tester.Click("back"))
		r.Settle()
		assert.Equal(t, "home", r.Route())
	})
}

// TestLockedHasNoEnrollAction checks K30: the footer is plain text, not focusable
// and not a link, with no globe and no button; a tap on it opens nothing, though the
// course has an `enroll_url` (a contract field the app does not open).
func TestLockedHasNoEnrollAction(t *testing.T) {
	t.Parallel()

	const contact = "Liên hệ DanceFITVN để tham gia khoá học"
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-36")
		w, rect, ok := r.Tester.Find(contact)
		require.True(t, ok, "the contact text shows")
		_, isLabel := w.(*kit.Label)
		assert.True(t, isLabel, "plain text: %T", w)
		f, ok := w.(ui.Focuser)
		require.True(t, ok)
		assert.False(t, f.AcceptsFocus(), "not focusable")
		assert.InDelta(t, ph.Size.W/2, rect.Center().X, 1, "centred")
		assert.InDelta(t, ph.Size.H-38, rect.Max().Y, 0.01, "38 over the bottom, as the prototype's footer")
		_, _, found := r.Tester.Find("Đăng ký trên website")
		assert.False(t, found, "no enroll button")
		r.Tester.ClickAt(rect.Center())
		r.Settle()
		assert.Empty(t, r.Fake.OpenedURLs(), "nothing opens")
		assert.Equal(t, "detail", r.Route())
	})
}

// TestLockedSwapsToOfflineAndBack checks 03 section 2 on the locked course: the network
// going puts the offline screen in its place, and its return the locked course again.
func TestLockedSwapsToOfflineAndBack(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atDetail(t, ph, false, "set-36")
		r.Fake.SetOnline(false)
		r.Settle()
		assert.Equal(t, "offline", r.Route())
		r.Fake.SetOnline(true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		require.Equal(t, "detail", r.Route())
		assert.NotNil(t, detailOf(t, r).Locked())
		assert.True(t, r.Tester.Shows("Bạn chưa tham gia khoá học này"))
	})
}
