//go:build tvos

package screens_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/pairing"
	"dancefitvn-app/internal/qr"
	"dancefitvn-app/internal/ui/tv/env"
	"dancefitvn-app/internal/ui/tv/screens"
	"dancefitvn-app/internal/ui/tv/tvkit"
	"dancefitvn-app/internal/ui/tv/tvtest"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/core/keys"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The state of the mock's tv/pair-start.json: the code, the address, 300 s.
const (
	mockCode    = "7K4M2P"
	mockDisplay = "7K4-M2P"
	mockQRURL   = "https://dancefitvn.com/tv?c=7K4M2P"
	mockExpires = 300 * time.Second
)

// mockState is the answer of start as pairing.TV turns it into a State.
func mockState() pairing.State {
	return pairing.State{Code: mockCode, DisplayCode: mockDisplay, QRURL: mockQRURL}
}

// onCode opens TV02 in a Shell over a fake services and returns the screen, the
// fake pairing it started and the rig.
func onCode(t testing.TB, real bool) (*rig, *screens.Code, *tvtest.Pairing) {
	t.Helper()
	svc := tvtest.NewServices()
	r := open(t, svc, env.RouteCode, real)
	code, ok := r.sh.Screen().(*screens.Code)
	require.True(t, ok)
	ps := svc.Pairings()
	require.Len(t, ps, 1, "TV02 starts the pairing when it is shown")

	return r, code, ps[0]
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// TestCodeBoxesMatchPrototype compares every element of TV02.layout.json with the
// screen's once a code is shown, but `c1` (K2: gone) and `c2`, which is focused
// here and so at its scaled box, where the capture has it at rest.
func TestCodeBoxesMatchPrototype(t *testing.T) {
	t.Parallel()
	r, _, p := onCode(t, true)
	p.Push(mockState(), 272*time.Second)
	r.tt.Elapse(tvkit.Transition)
	require.Equal(t, "c2", r.key())
	lay := r.layout()
	for _, key := range tvtest.ProtoKeys(t, "TV02") {
		if key == "c1" {
			_, ok := lay.Find(key)
			assert.False(t, ok, "c1 is K2")

			continue
		}
		e, ok := lay.Find(key)
		require.True(t, ok, "TV02 has no element %q", key)
		want := tvtest.ProtoBox(t, "TV02", key)
		if key == "c2" {
			want = scaled(want, tvkit.FocusScale)
		}
		near(t, want, geom.XYWH(float32(e.X), float32(e.Y), float32(e.W), float32(e.H)), boxTol, "TV02 "+key)
	}
}

// TestCodeExpiryTextStartsAfterTheSpace: the clock is at x 163-184 and the text of
// the expiry line starts at x 198 in the capture (TV02.png): the space between them
// is the line's own 7 DIPs, not the ~0 a measured " " gives. The ink of the first
// letter is read back from the picture.
func TestCodeExpiryTextStartsAfterTheSpace(t *testing.T) {
	t.Parallel()
	r, _, p := onCode(t, true)
	p.Push(mockState(), 272*time.Second)
	r.tt.Elapse(tvkit.Transition)
	img := r.pixels()
	// ink of the line, columns 188 on (the clock ends at 185), rows of its text
	first, last := -1, -1
	for x := 150; x < 700; x++ {
		ink := false
		for y := 838; y <= 866; y++ {
			if img.RGBAAt(x, y).R > 90 {
				ink = true

				break
			}
		}
		if ink && x < 188 {
			last = x // the clock

			continue
		}
		if ink {
			first = x

			break
		}
	}
	assert.InDelta(t, 185, last, 2, "the clock's right edge (capture 184)")
	assert.InDelta(t, 198, first, 2, "the text's first ink (capture 198)")
}

// TestCodeText: the strings of TV02 are the prototype's, the code is the
// display code and the countdown is mm:ss.
func TestCodeText(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, true)
	assert.Empty(t, s.CodeText(), "no code before the first state")
	assert.Empty(t, s.ExpiryText())
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)
	lay := r.layout()
	want := map[string]string{
		"div.d":           "ĐĂNG NHẬP APPLE TV",
		"p.muted":         "Dùng tài khoản huấn luyện viên đã đăng ký trên dancefitvn.com",
		"tv.code.step":    "Mở app DanceFITVN trên điện thoại",
		"tv.code.step#2":  "Vào Tài khoản → Kết nối Apple TV",
		"tv.code.step#3":  "Nhập mã bên dưới",
		"tv.code.value":   "7K4-M2P",
		"p.muted#2":       "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo",
		"tv.code.qrTitle": "Hoặc quét mã QR",
		"p.muted#3":       "Mở dancefitvn.com/mobile/tv trên điện thoại",
		"c2":              "Quay lại",
	}
	for key, txt := range want {
		e, ok := lay.Find(key)
		require.True(t, ok, key)
		assert.Equal(t, txt, e.Text, key)
	}
	assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText())
}

// TestCodeHintKeepsTheAddressUnbroken is defect 3.7: the hint under the QR breaks at the
// spaces only, never inside "dancefitvn.com/mobile/tv", and its box keeps the
// prototype's height.
func TestCodeHintKeepsTheAddressUnbroken(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, true)
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)
	assert.False(t, s.HintBreaksWord(), "the address is whole")
	e, ok := r.layout().Find("p.muted#3")
	require.True(t, ok)
	assert.InDelta(t, 62, e.H, 2)
	assert.LessOrEqual(t, e.X+e.W, 1290.0+340+.5)
}

// TestCodeDefaultFocusAndBack: `c2` has the focus and is alone; Select on it and
// Menu go back to TV01.
func TestCodeDefaultFocusAndBack(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewServices()
	r := open(t, svc, env.RouteWelcome, false).armed()
	require.True(t, r.tt.Key(keys.KeyEnter, 0)) // w1
	r.tt.Elapse(tvtest.Frame)
	require.Equal(t, env.RouteCode, r.sh.Stack().Screen())
	assert.Equal(t, "c2", r.key())
	for _, k := range []keys.Key{keys.KeyUp, keys.KeyDown, keys.KeyLeft, keys.KeyRight} {
		r.press(k)
		assert.Equal(t, "c2", r.key(), "nothing else to focus")
	}
	require.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen(), "Quay lại")
	assert.Equal(t, "w1", r.key())
}

// TestCodeCountdown: 05:00 on a fresh code, a second later 04:59, down to 00:01
// and then 00:00 while the new code is awaited; the line is hidden until there
// is a code.
func TestCodeCountdown(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, false)
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText())
	tick := func() {
		p.Advance(time.Second)
		r.tt.Elapse(time.Second)
	}
	tick()
	assert.Equal(t, "Mã hết hạn sau 04:59 · mã mới sẽ tự tạo", s.ExpiryText(), "K8: a 1 s tick")
	for range 58 {
		tick()
	}
	assert.Equal(t, "Mã hết hạn sau 04:01 · mã mới sẽ tự tạo", s.ExpiryText())
	for range 240 {
		tick()
	}
	assert.Equal(t, "Mã hết hạn sau 00:01 · mã mới sẽ tự tạo", s.ExpiryText())
	tick()
	assert.Equal(t, "Mã hết hạn sau 00:00 · mã mới sẽ tự tạo", s.ExpiryText())
	tick()
	assert.Equal(t, "Mã hết hạn sau 00:00 · mã mới sẽ tự tạo", s.ExpiryText(), "never below")
}

// TestCodeRoundsUp: a code with 299.4 s left reads 05:00, with 299 s 04:59.
func TestCodeRoundsUp(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, false)
	p.Push(mockState(), 299400*time.Millisecond)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText())
	p.Advance(400 * time.Millisecond)
	r.tt.Elapse(time.Second)
	assert.Equal(t, "Mã hết hạn sau 04:59 · mã mới sẽ tự tạo", s.ExpiryText())
}

// TestCodeNewCodeAfterExpiry: when the pairing gives a new state (the code
// expired) the screen shows the new code, makes the QR of the new address and
// restarts the countdown; the same address does not make a new symbol.
func TestCodeNewCodeAfterExpiry(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, false)
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)
	first := s.QR().Code()
	require.NotNil(t, first)
	for range 10 {
		p.Advance(time.Second)
		r.tt.Elapse(time.Second)
	}
	assert.Equal(t, "Mã hết hạn sau 04:50 · mã mới sẽ tự tạo", s.ExpiryText())

	p.Push(mockState(), mockExpires) // the same address: the symbol stays
	r.tt.Elapse(tvtest.Frame)
	assert.Same(t, first, s.QR().Code(), "re-encoded only when the address changes")
	assert.Equal(t, "Mã hết hạn sau 05:00 · mã mới sẽ tự tạo", s.ExpiryText())

	next := pairing.State{Code: "9Q8W7E", DisplayCode: "9Q8-W7E", QRURL: "https://dancefitvn.com/tv?c=9Q8W7E"}
	p.Push(next, mockExpires)
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, "9Q8-W7E", s.CodeText())
	assert.NotSame(t, first, s.QR().Code())
	assert.False(t, p.Stopped)
}

// TestCodeConfirmedGoesHome: the phone confirming is TV04 as the root of the
// stack, a toast "Đã đăng nhập bằng {email}", and the pairing stopped.
func TestCodeConfirmedGoesHome(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewServices()
	r := open(t, svc, env.RouteWelcome, false).armed()
	require.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	p := svc.Pairings()[0]
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)
	require.Equal(t, 1, r.sh.Stack().Depth())

	p.SignIn("thaovy.hlv@gmail.com")
	r.tt.Elapse(tvtest.Frame)
	assert.Equal(t, env.RouteHome, r.sh.Stack().Screen())
	assert.Zero(t, r.sh.Stack().Depth(), "home is a root")
	assert.Equal(t, "Đã đăng nhập bằng thaovy.hlv@gmail.com", r.sh.ToastHost().Message())
	assert.True(t, p.Stopped, "the screen is gone: the polls with it")
	assert.Empty(t, r.tt.Leaks())
}

// TestCodeLeavingStopsPairing: Back stops the pairing, and a state or a
// confirmation that arrives after it does nothing; entering again pairs again.
func TestCodeLeavingStopsPairing(t *testing.T) {
	t.Parallel()
	svc := tvtest.NewServices()
	r := open(t, svc, env.RouteWelcome, false).armed()
	require.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	p1 := svc.Pairings()[0]
	assert.False(t, p1.Stopped)
	require.True(t, r.press(keys.KeyEscape))
	assert.True(t, p1.Stopped)
	p1.Push(mockState(), mockExpires)
	p1.SignIn("late@example.com")
	r.tt.Elapse(time.Second)
	assert.Equal(t, env.RouteWelcome, r.sh.Stack().Screen(), "a late confirmation does nothing")
	assert.False(t, r.sh.ToastHost().Visible())

	require.True(t, r.tt.Key(keys.KeyEnter, 0))
	r.tt.Elapse(tvtest.Frame)
	ps := svc.Pairings()
	require.Len(t, ps, 2)
	assert.False(t, ps[1].Stopped)
	assert.Empty(t, r.tt.Leaks())
}

// TestCodeErrors: a failed request the pairing retries is TV11 over TV02 (Q-P7-15),
// offline or not, except a 429, which the pairing waits out without a word; Back
// from TV11 pairs again.
func TestCodeErrors(t *testing.T) {
	t.Parallel()
	t.Run("online", func(t *testing.T) {
		t.Parallel()
		svc := tvtest.NewServices()
		r := open(t, svc, env.RouteWelcome, false).armed()
		require.True(t, r.tt.Key(keys.KeyEnter, 0))
		r.tt.Elapse(tvtest.Frame)
		p := svc.Pairings()[0]
		p.Fail(errors.New("server error"))
		r.tt.Elapse(tvtest.Frame)
		assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
		assert.False(t, r.sh.ToastHost().Visible(), "no toast: TV11 says it")
		assert.True(t, p.Stopped, "TV02 left: the polls stop")
		require.True(t, r.press(keys.KeyEscape))
		assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
		assert.Len(t, svc.Pairings(), 2, "back on TV02 pairs again")
	})
	t.Run("rate limited", func(t *testing.T) {
		t.Parallel()
		r, _, p := onCode(t, false)
		p.Fail(&api.Error{Status: http.StatusTooManyRequests, Code: "rate_limited"})
		r.tt.Elapse(tvtest.Frame)
		assert.Equal(t, env.RouteCode, r.sh.Stack().Screen(), "the pairing waits retry_after by itself")
		assert.False(t, r.sh.ToastHost().Visible())
		assert.False(t, p.Stopped)
	})
	t.Run("offline", func(t *testing.T) {
		t.Parallel()
		svc := tvtest.NewServices()
		r := open(t, svc, env.RouteWelcome, false).armed()
		require.True(t, r.tt.Key(keys.KeyEnter, 0))
		r.tt.Elapse(tvtest.Frame)
		svc.SetOnline(false)
		p := svc.Pairings()[0]
		p.Fail(errors.New("offline"))
		r.tt.Elapse(tvtest.Frame)
		assert.Equal(t, env.RouteError, r.sh.Stack().Screen())
		assert.True(t, p.Stopped, "TV02 left: the polls stop")
		svc.SetOnline(true)
		require.True(t, r.press(keys.KeyEscape))
		assert.Equal(t, env.RouteCode, r.sh.Stack().Screen())
		assert.Len(t, svc.Pairings(), 2, "back on TV02 pairs again")
	})
}

// TestCodeQRIsTheSymbolOfTheAddress paints TV02 and reads the modules back from
// the picture: they are the symbol libqrencode makes for the mock's `qr_url`
// (internal/qr/testdata/pairing.M.txt), which every reader decodes to that
// address. The modules are whole DIPs, 10 of them for a 29 by 29 symbol, centered
// in the 300 square of the white box.
func TestCodeQRIsTheSymbolOfTheAddress(t *testing.T) {
	t.Parallel()
	r, s, p := onCode(t, false)
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(tvtest.Frame)

	data, err := os.ReadFile(filepath.Join("..", "..", "..", "qr", "testdata", "pairing.M.txt"))
	if err != nil {
		t.Skipf("no reference symbol: %v", err)
	}
	ref := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	q := s.QR()
	require.NotNil(t, q.Code())
	n := q.Code().Size()
	require.Equal(t, 29, n)
	require.Len(t, ref, n)
	assert.InDelta(t, 10, q.Module(), 0, "floor(300 / 29)")
	assert.Equal(t, geom.Pt(25, 25), q.Origin(), "centered in the 300 square, 20 in")
	frame := q.WindowFrame()
	assert.Equal(t, geom.XYWH(1290, 250, 340, 344), frame)

	img := r.pixels()
	for y := range n {
		for x := range n {
			px := int(frame.X + q.Origin().X + (float32(x)+.5)*q.Module())
			py := int(frame.Y + q.Origin().Y + (float32(y)+.5)*q.Module())
			dark := img.RGBAAt(px, py).R < 128
			require.Equal(t, ref[y][x] == '#', dark, "module %d,%d", x, y)
		}
	}
	// the quiet zone and the box's corners: white
	assert.Equal(t, uint8(255), img.RGBAAt(int(frame.X)+30, int(frame.Y)+12).R)
	assert.Equal(t, uint8(255), img.RGBAAt(int(frame.X)+170, int(frame.Y)+342).R)
	// and the symbol is the encoder's
	enc, err := qr.Encode([]byte(mockQRURL), qr.LevelM)
	require.NoError(t, err)
	for y := range n {
		for x := range n {
			assert.Equal(t, enc.Black(x, y), q.Code().Black(x, y))
		}
	}
}

// TestCodeCountdownDoesNotAllocate: a tick of the countdown (the timer, the
// text, the layout and the frame) and a steady frame allocate nothing, once the
// seconds of a code have been shown.
func TestCodeCountdownDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	skipUnderRace(t)
	r, _, p := onCode(t, false)
	p.Push(mockState(), mockExpires)
	r.tt.Elapse(time.Second)
	step := func() {
		p.Advance(time.Second)
		r.tt.Elapse(time.Second)
	}
	for range 260 { // every second's text is made and shaped once
		step()
	}
	p.Push(mockState(), mockExpires) // back to 05:00: the texts are those seen
	r.tt.Elapse(tvtest.Frame)
	assert.Zero(t, testing.AllocsPerRun(100, step), "a tick")
	r.tt.Elapse(time.Second)
	r.tt.Repaint()
	assert.Zero(t, testing.AllocsPerRun(50, r.tt.Repaint), "a steady frame")
}
