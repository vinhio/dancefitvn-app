//go:build !tvos

package shell

import (
	"context"
	"errors"
	"log"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// AskPassword is `sheet` v=password (`$P:620`): SH.password opens.
func (s *Shell) AskPassword() {
	s.nav.Sheet().Open(screens.NewPasswordSheet(s.e), kit.SheetOptions{})
}

// AskLogout is `sheet` v=logout: SH.logout, "Đăng xuất?".
func (s *Shell) AskLogout() {
	s.nav.Sheet().Open(screens.NewLogoutSheet(s.e), kit.SheetOptions{})
}

// AskAvatar is `sheet` v=avatar: SH.avatar (15D).
func (s *Shell) AskAvatar() {
	s.nav.Sheet().Open(screens.NewAvatarSheet(s.e), kit.SheetOptions{})
}

// AskDevices opens the sheet of devices for a TV code the server refused for the
// limit of devices.
func (s *Shell) AskDevices(p env.DevicesSheetParams) {
	s.nav.Sheet().Open(screens.NewDevicesSheet(s.e, p), kit.SheetOptions{})
}

// Logout is `logout` of SH.logout (`$P:530`, 03 section 3). In this order, on the
// UI thread: the sheet closes; the sound stops and the player Leaves, so that the
// position it saves reaches the queue before the sign-out clears it (a sound plays
// on after Leave); the kept account view is forgotten; the login shows, with no
// banner, the stack empty (the prototype does not wait for the server). Then the
// sign-out itself runs off the UI thread (Account.Logout: the saves and the queue
// are waited for and sent, the server told, the vault cleared), owned by the
// Shell so that leaving a screen does not cancel it. Its error is for the log:
// the user is signed out whatever it says (the session of the vault that could
// not be cleared is the next launch's to refuse).
func (s *Shell) Logout() {
	s.nav.Sheet().Close()
	if pb := s.e.Playback(); pb != nil {
		pb.Pause()
		pb.Leave()
	}
	s.e.ResetAccount()
	// the sign-out is marked here, on the UI thread and before the login shows: a
	// Login begun from it waits, whenever the goroutine below gets to run
	logout := s.e.Account().BeginLogout()
	s.nav.Go(nav.Route{Screen: env.RouteLogin, Params: env.LoginParams{Banner: env.LoginBannerNone}}, true)
	ui.Go(s, func(ctx context.Context) (struct{}, error) {
		return struct{}{}, logout(ctx)
	}, func(_ *ui.Context, _ struct{}, err error) {
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("dancefitvn: logout: %v", err)
		}
	})
}
