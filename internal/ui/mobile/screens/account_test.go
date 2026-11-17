//go:build !tvos

package screens_test

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/kit/kittest"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell/shelltest"

	"github.com/gdeskdev/desk/foundation/scene"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// atAccountDirect mounts screen 15 alone over a Fake with a recording Actions.
func atAccountDirect(t *testing.T, ph kittest.Phone, setups ...func(*shelltest.Fake)) (*screens.Account, *shelltest.Fake, *recorder, *shelltest.Rig) {
	t.Helper()
	f, err := shelltest.New(shelltest.MockFS(t))
	require.NoError(t, err)
	require.NoError(t, f.SeedSession())
	signed, err := f.Restore(ctx())
	require.NoError(t, err)
	require.True(t, signed)
	f.Account().SetTVLinked(false)
	for _, s := range setups {
		s(f)
	}
	rec := &recorder{}
	e := env.New(f, texts())
	e.Act = rec
	a := screens.NewAccount(e)
	tt := kittest.MountReal(t, ph, a)
	kittest.PinMotion(tt)
	tt.RunUntilIdle()

	return a, f, rec, &shelltest.Rig{Phone: ph, Fake: f, Tester: tt}
}

// TestAccountShowsTheUser checks screen 15 over GET /me of the mock, on both phones:
// the card of the user (the name as the prototype writes it, "Thành viên từ" and the
// email), the three badges with their names, the TV not linked, the switches as the
// preferences are, the size of the downloaded files.
func TestAccountShowsTheUser(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false)
		a := accountOf(t, r)
		assert.Equal(t, 3, a.Tab())
		assert.Equal(t, "Nguyễn Thảo Vy", a.UserName())
		assert.Equal(t, "Thành viên từ 12/03/2024", a.Since())
		assert.Equal(t, meEmail, a.Email())
		require.Len(t, a.Badges(), 3)
		for i, name := range []string{"HDV DanceFIT", "HDV StepX", "HDV BodyX"} {
			assert.Equal(t, name, a.Badges()[i].Label)
		}
		assert.Equal(t, "Chưa kết nối", a.TV())
		assert.True(t, a.WiFiOn(), "Chỉ tải qua Wi‑Fi is on at first (the prototype's default)")
		assert.Equal(t, r.Shell.Env().Prefs().AutoNext(), a.AutoOn())
		assert.Equal(t, format.Size(r.Fake.Dl().TotalBytes()), a.Downloaded())
		assert.Equal(t, "DanceFITVN", a.Version(), "services without a version leave it out")
		for _, text := range []string{"Tài khoản", "Huy hiệu", "Hồ sơ", "Ứng dụng", "Đăng xuất",
			"Thông tin tài khoản", "Hồ sơ huấn luyện viên", "Đổi mật khẩu", "Kết nối Apple TV",
			"Chỉ tải qua Wi‑Fi", "Tự phát bài tiếp theo", "File âm thanh đã tải"} {
			assert.Truef(t, r.Tester.Shows(text), "%q is on the screen", text)
		}
		assert.Positive(t, r.Fake.Account().Count("Me"), "GET /me was read")
		assert.Positive(t, r.Fake.Account().Count("Devices"), "GET /me/devices was read")
	})
}

// versioned is a Fake that tells the app's version.
type versioned struct {
	*shelltest.Fake
}

// AppVersion is the version of the test.
func (versioned) AppVersion() string { return "0.1.0" }

// TestAccountVersionLine checks K2: the line under the button is "DanceFITVN {version}"
// of the services that tell it.
func TestAccountVersionLine(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		e := env.New(versioned{f}, texts())
		e.Act = &recorder{}
		a := screens.NewAccount(e)
		assert.Equal(t, "DanceFITVN 0.1.0", a.Version())
		_ = ph
	})
}

// pictured is a Fake that has a picture for every URL, and notes which it was asked.
type pictured struct {
	*shelltest.Fake

	mu   sync.Mutex
	urls []string
}

// Avatar answers a small picture for any URL.
func (p *pictured) Avatar(_ context.Context, url string) (*scene.Image, error) {
	p.mu.Lock()
	p.urls = append(p.urls, url)
	p.mu.Unlock()

	return scene.NewImage(4, 4)
}

// TestAccountLoadsThePictures checks that the avatar and the three medals are loaded
// behind the screen and replace the drawn figures (the person, the hexagons) when
// they are in; with none (the Fake's own Avatar fails) the figures stay.
func TestAccountLoadsThePictures(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("loaded", func(t *testing.T) {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			_, err = f.Restore(ctx())
			require.NoError(t, err)
			p := &pictured{Fake: f}
			e := env.New(p, texts())
			e.Act = &recorder{}
			a := screens.NewAccount(e)
			tt := kittest.MountReal(t, ph, a)
			kittest.PinMotion(tt)
			tt.RunUntilIdle()
			assert.NotNil(t, a.Avatar().Image, "the avatar")
			require.Len(t, a.Badges(), 3)
			for i, b := range a.Badges() {
				assert.NotNil(t, b.Image, "the medal %d", i)
			}
			assert.Len(t, p.urls, 4, "the avatar and the three medals, each once")
		})
		t.Run("none", func(t *testing.T) {
			r := atAccount(t, ph, false)
			a := accountOf(t, r)
			assert.Nil(t, a.Avatar().Image, "the person stays")
			for _, b := range a.Badges() {
				assert.Nil(t, b.Image, "the hexagons stay")
			}
		})
	})
}

// manyBadges is a Fake whose GET /me holds n badges.
type manyBadges struct {
	*shelltest.Fake
	n int
}

// Me answers the Fake's user with n badges.
func (m manyBadges) Me(ctx context.Context) (model.Me, error) {
	me, err := m.Fake.Me(ctx)
	if err != nil {
		return me, err
	}
	me.Badges = nil
	for i := range m.n {
		me.Badges = append(me.Badges, model.Badge{Code: "hdv_b" + strconv.Itoa(i), Name: "FOUNDER_CEO " + strconv.Itoa(i)})
	}

	return me, nil
}

// TestAccountManyBadgesStayInTheScreen is defect 3.1 on the phone: more than three
// badges go on further lines of three cells, none past the screen's width, none over
// another.
func TestAccountManyBadgesStayInTheScreen(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, n := range []int{1, 3, 4, 5, 8} {
			f, err := shelltest.New(shelltest.MockFS(t))
			require.NoError(t, err)
			require.NoError(t, f.SeedSession())
			_, err = f.Restore(ctx())
			require.NoError(t, err)
			e := env.New(manyBadges{Fake: f, n: n}, texts())
			e.Act = &recorder{}
			a := screens.NewAccount(e)
			tt := kittest.MountReal(t, ph, a)
			kittest.PinMotion(tt)
			tt.RunUntilIdle()
			require.Len(t, a.Badges(), n)
			for i, b := range a.Badges() {
				fr := b.Node().WindowFrame()
				assert.GreaterOrEqual(t, fr.X, float32(0), "n=%d badge %d", n, i)
				assert.LessOrEqual(t, fr.X+fr.W, ph.Size.W+.5, "n=%d badge %d inside the screen", n, i)
				assert.GreaterOrEqual(t, fr.W, float32(72), "n=%d badge %d holds its medal", n, i)
				for j := range i {
					o := a.Badges()[j].Node().WindowFrame()
					overlap := fr.X < o.X+o.W-.5 && o.X < fr.X+fr.W-.5 && fr.Y < o.Y+o.H-.5 && o.Y < fr.Y+fr.H-.5
					assert.False(t, overlap, "n=%d badges %d and %d apart", n, j, i)
				}
			}
		}
	})
}

// TestAccountTVRow checks the value of "Kết nối Apple TV": "Chưa kết nối" until the
// devices hold a TV, "Đã kết nối" with one (Q-P6-3), and that it survives a rebuild
// of the screen from the kept view.
func TestAccountTVRow(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false, func(f *shelltest.Fake) { f.Account().SetTVLinked(true) })
		a := accountOf(t, r)
		assert.Equal(t, "Đã kết nối", a.TV())
		assert.True(t, r.Tester.Shows("Đã kết nối"))

		r.Shell.Go(nav.Route{Screen: env.RouteDownloads}, true)
		r.Settle()
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		assert.Equal(t, "Đã kết nối", accountOf(t, r).TV(), "drawn at once from the kept view")
	})
}

// TestAccountActions checks that every row, the avatar and the red button ask the
// shell for what the prototype's `data-go` and `data-act` do, on both phones.
func TestAccountActions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key  string
		want string
	}{
		{"go.info", "go info false"},
		{"go.coach", "go coach false"},
		{"sheet:password", "askPassword"},
		{"go.connect", "go connect false"},
		{"go.downloads", "go downloads true"},
		{"sheet:avatar", "askAvatar"},
		{"sheet:logout", "askLogout"},
	}
	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		for _, tc := range tests {
			t.Run(tc.key, func(t *testing.T) {
				a, _, rec, r := atAccountDirect(t, ph)
				scrollInto(t, r, a.Scroll(), tc.key)
				tapKey(t, r, tc.key)
				assert.Equal(t, []string{tc.want}, rec.calls)
			})
		}
	})
}

// TestAccountSwitchesPersist checks `pref:wifi` and `pref:auto`: a tap on the row
// turns the preference over, the switch shows it, the preference is the services'
// and a screen built again draws it.
func TestAccountSwitchesPersist(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false)
		a := accountOf(t, r)
		prefs := r.Fake.FakePrefs()
		wifi0, auto0 := a.WiFiOn(), a.AutoOn()
		require.True(t, wifi0)

		tapKey(t, r, "pref:wifi")
		assert.False(t, a.WiFiOn())
		assert.False(t, prefs.WiFiOnly(), "the preference changed")
		assert.False(t, r.Shell.Env().WiFiOnly())
		tapKey(t, r, "pref:auto")
		assert.Equal(t, !auto0, a.AutoOn())
		assert.Equal(t, !auto0, prefs.AutoNext(), "the preference changed")

		r.Shell.Go(nav.Route{Screen: env.RouteDownloads}, true)
		r.Settle()
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		r.Settle()
		a = accountOf(t, r)
		assert.False(t, a.WiFiOn(), "a rebuilt screen shows the kept preference")
		assert.Equal(t, !auto0, a.AutoOn())

		tapKey(t, r, "pref:wifi")
		tapKey(t, r, "pref:auto")
		assert.True(t, a.WiFiOn())
		assert.Equal(t, auto0, a.AutoOn())
		assert.True(t, prefs.WiFiOnly())
		assert.Equal(t, auto0, prefs.AutoNext())
	})
}

// TestAccountFollowsPreferences checks that a preference changed elsewhere (the
// sheet of the download has the same Wi‑Fi row) is what the switch shows when the
// screen comes back under a sheet that closes.
func TestAccountFollowsDownloads(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false)
		a := accountOf(t, r)
		before := a.Downloaded()
		r.Fake.Dl().Clear()
		r.Settle()
		assert.NotEqual(t, before, a.Downloaded(), "the size follows the files")
		assert.Equal(t, format.Size(0), a.Downloaded())
	})
}

// TestAccountOffline checks the phone with no network: the card draws from the user of
// the session with no badges and the TV not linked, nothing is toasted, and when the
// network is back the account is read again and the badges come.
func TestAccountOffline(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		f, err := shelltest.New(shelltest.MockFS(t))
		require.NoError(t, err)
		require.NoError(t, f.SeedSession())
		r := shelltest.MountRig(t, ph, false, f, shellOptions())
		r.Boot()
		f.SetOnline(false)
		r.Shell.Go(nav.Route{Screen: env.RouteAccount}, true)
		r.Tester.Elapse(kittest.Frame)
		r.Settle()
		a := accountOf(t, r)
		assert.Equal(t, "Nguyễn Thảo Vy", a.UserName(), "the session's user")
		assert.Empty(t, a.Badges())
		assert.Equal(t, meEmail, a.Email())
		assert.Empty(t, a.Since(), "the session has no join date")
		assert.False(t, shows(r, "Không kết nối được, thử lại sau"), "a read that fails says nothing")

		f.SetOnline(true)
		r.Settle()
		assert.Len(t, accountOf(t, r).Badges(), 3, "read again when the network is back")
	})
}

// TestAccountScenarios runs the scenarios of GET /me through the screen: a session
// that the server refuses to renew (token_expired, refresh invalid) is the login's
// banner; the other errors leave the card as it is.
func TestAccountScenarios(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		t.Run("server error", func(t *testing.T) {
			r := atAccountLate(t, ph, func(f *shelltest.Fake) { f.Fail("me.get", 500, "errors/server_error.json") })
			a := accountOf(t, r)
			assert.Equal(t, "Nguyễn Thảo Vy", a.UserName())
			assert.Empty(t, a.Badges())
			assert.False(t, shows(r, "Hệ thống đang bận"))
		})
		t.Run("upgrade required", func(t *testing.T) {
			r := atAccountLate(t, ph, func(f *shelltest.Fake) { f.Fail("me.get", 426, "errors/upgrade_required.json") })
			assert.Equal(t, env.RouteUpgrade, r.Route(), "the upgrade screen replaces it (03 section 9)")
		})
		t.Run("token_expired", func(t *testing.T) {
			// the scenario answers 401 to every request, the retry after the refresh as
			// well: the second 401 ends the session (auth.Manager.Handle) and the login
			// shows its banner. The Fake posts the sign-out to the UI thread, as the app's
			// wire does (Rig.Settle runs it); the next run makes the same call from the UI
			// thread.
			r := atAccountLate(t, ph, func(f *shelltest.Fake) { require.NoError(t, f.SetScenario("me.get", "token_expired")) })
			r.Settle()
			assert.Equal(t, env.RouteLogin, r.Route())
			assert.False(t, r.Fake.Signed())
			assert.Equal(t, env.AccountState{}, r.Shell.Env().AccountView(), "nothing of the user is kept")
		})
		t.Run("token_expired from the UI thread", func(t *testing.T) {
			r := atAccountLate(t, ph, func(*shelltest.Fake) {})
			require.NoError(t, r.Fake.SetScenario("me.get", "token_expired"))
			_, err := r.Shell.Env().Account().Me(ctx())
			require.Error(t, err)
			r.Settle()
			assert.Equal(t, env.RouteLogin, r.Route())
			assert.Equal(t, env.AccountState{}, r.Shell.Env().AccountView())
		})
	})
}

// TestAccountSheetsFromTheShell checks the sheets that the rows open, through the
// shell, on both phones: the password sheet, the logout sheet.
func TestAccountSheetsFromTheShell(t *testing.T) {
	t.Parallel()

	kittest.ForEachPhone(t, func(t *testing.T, ph kittest.Phone) {
		r := atAccount(t, ph, false)
		tapKey(t, r, "sheet:password")
		slideIn(r)
		_, ok := r.Shell.Nav().Sheet().Content().(*screens.PasswordSheet)
		assert.True(t, ok, "SH.password")
		r.Shell.Nav().Sheet().Close()
		slideIn(r)
		scrollInto(t, r, accountOf(t, r).Scroll(), "sheet:logout")
		tapKey(t, r, "sheet:logout")
		slideIn(r)
		_, ok = r.Shell.Nav().Sheet().Content().(*screens.LogoutSheet)
		assert.True(t, ok, "SH.logout")
	})
}

// TestAccountSteadyFrameDoesNotAllocate checks that a frame of screen 15 standing
// still allocates nothing, on both phones.
func TestAccountSteadyFrameDoesNotAllocate(t *testing.T) { //nolint:paralleltest // AllocsPerRun needs the process to itself
	if kittest.Race {
		t.Skip("allocation counts are not stable under the race detector")
	}
	for _, ph := range kittest.Phones {
		r := atAccount(t, ph, false)
		for range 5 {
			r.Tester.Elapse(kittest.Frame)
		}
		assert.Zero(t, testing.AllocsPerRun(50, r.Tester.Repaint), ph.Name)
	}
}
