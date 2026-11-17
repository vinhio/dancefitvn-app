//go:build !tvos

package app

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"
	"dancefitvn-app/internal/ui/mobile/shell"

	"github.com/gdeskdev/desk/foundation/core/geom"
	"github.com/gdeskdev/desk/foundation/platform"
	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockTV is the Apple TV of the mock's devices (me/devices.json): the
// prototype starts with no TV linked (`tvLinked:false`, index.html:276),
// so the states of 15 and 16 delete it through the overlay first (P6 F-3).
const mockTV = "d_201"

// pairCode is the code the prototype's M16-code shows typed (index.html:501).
const pairCode = "7K4M2P"

// pairCodeEnv names, for the state PAIR, the code the Apple TV shows:
// cmd/mockserve's relay draws it at random. pairCode when unset.
const pairCodeEnv = "DFV_PAIR_CODE"

// toastRearm is how often the toast of M-toast is shown again: before its
// 2200 ms are over, so that it stays for the shot (theme.ToastTime).
const toastRearm = 2 * time.Second

// The developer's account states (no prototype shot): what the device
// smoke of P6.1b drives on the simulator.
const (
	// infoFirst and infoLast are what INFO-SAVE types in 15B.
	infoFirst = "Lan"
	infoLast  = "Tran"
)

// accountStates returns the states of the account (local/plans/P6.md section
// 7): screens 15, 15B, 15C, 16 and their sheets, over the mock with the TV
// deleted (but M16-linked), and M-toast (the detail of Set 35 with "Đã tải
// xong bài 03"). AVATAR-PICK, INFO-SAVE, LOGOUT, PAIR and PAIR-CHECK are a
// developer's.
func accountStates() map[string]shotState {
	return map[string]shotState{
		"M15": {knobs: protoAccount, prepare: signedInNoTV, steps: accountSteps()},
		// M15-privacy is M15 with the row "Chính sách bảo mật" (K32) scrolled into view.
		"M15-privacy":   {knobs: demo, prepare: signedInNoTV, steps: append(accountSteps(), privacyShown())},
		"M15B":          {knobs: demo, prepare: signedInNoTV, steps: append(accountSteps(), tapNamed("go.info"), routeIs(env.RouteInfo))},
		"M15B-password": {knobs: protoAccount, prepare: signedInNoTV, steps: append(accountSteps(), tapNamed("sheet:password"), sheetOpen())},
		"M15C":          {knobs: demo, prepare: signedInNoTV, steps: append(accountSteps(), tapNamed("go.coach"), routeIs(env.RouteCoach), coachLoaded())},
		"M15D":          {knobs: protoAccount, prepare: signedInNoTV, steps: append(accountSteps(), tapNamed("sheet:avatar"), sheetOpen())},
		"M15-logout": {knobs: protoAccount, prepare: signedInNoTV, steps: append(accountSteps(),
			stateStep{what: "ask logout", act: func(sh *shell.Shell, _ *mobileWire) { sh.AskLogout() }}, sheetOpen())},
		"M16":        {knobs: demo, prepare: signedInNoTV, steps: connectSteps()},
		"M16-code":   {knobs: demo, prepare: signedInNoTV, steps: append(connectSteps(), typeCode(pairCode))},
		"M16-linked": {knobs: demo, prepare: signedIn(true), steps: append(accountLinkedSteps(), pushRoute(env.RouteConnect), routeIs(env.RouteConnect))},
		// PAIR and PAIR-CHECK are the phone's half of the pairing with an Apple
		// TV over cmd/mockserve (DFV_API; local/plans/P7.md section 8.2, P7.5d):
		// PAIR types the code the TV shows (DFV_PAIR_CODE) in 16 and taps "Liên
		// kết"; PAIR-CHECK opens 16 again as the server tells it, linked or not
		// (after the TV signed out: not).
		"PAIR": {knobs: demo, prepare: signedInNoTV, steps: slices.Concat(accountSteps(), pushConnectRead(),
			[]stateStep{typeCodeOf(envPairCode), tapLink(), connectIs(true), logConnect()})},
		"PAIR-CHECK": {knobs: demo, prepare: signedIn(true), steps: []stateStep{homeLoaded(), goRoute(env.RouteAccount, nil),
			routeIs(env.RouteAccount), accountRead(), pushRoute(env.RouteConnect), routeIs(env.RouteConnect), connectFollows(), logConnect()}},
		"M-toast": {knobs: demo, prepare: signedIn(true), steps: []stateStep{
			homeLoaded(), openDetail(demoCourse), detailLoaded(), holdFinishedToast(demoLessonID, demoLessonNo),
		}},
		"AVATAR-PICK": {knobs: demo, prepare: signedInNoTV, steps: append(accountSteps(), tapNamed("sheet:avatar"), sheetOpen(),
			tapAvatarSheet("library", (*screens.AvatarSheet).Library, nil), logStep("avatar picker asked"),
			avatarPicked(), tapAvatarSheet("accept", func(a *screens.AvatarSheet) ui.Widget { return a.Accept() }, (*screens.AvatarSheet).Busy),
			avatarUploaded())},
		"INFO-SAVE": {knobs: demo, prepare: signedInNoTV, diskOverlay: true, steps: append(accountSteps(),
			tapNamed("go.info"), infoShown(), typeInfo(), tapSubmit(), routeIs(env.RouteAccount), logStep("info saved"),
			tapNamed("go.info"), infoShown(), logInfo())},
		// ACCOUNT is M15 over the overlay of the disk: what INFO-SAVE saved, read by
		// another launch; OVERLAY-RESET is the dev menu's "Reset overlay" (05
		// section 4), then M15 again.
		"ACCOUNT": {knobs: demo, prepare: signedIn(true), diskOverlay: true, steps: []stateStep{
			homeLoaded(), goRoute(env.RouteAccount, nil), routeIs(env.RouteAccount), meShown(), logAccount(),
		}},
		"OVERLAY-RESET": {knobs: demo, prepare: resetOverlay, diskOverlay: true, steps: []stateStep{
			homeLoaded(), goRoute(env.RouteAccount, nil), routeIs(env.RouteAccount), meShown(), logAccount(),
		}},
		"LOGOUT": {knobs: demo, prepare: signedInNoTV, steps: append(accountSteps(),
			stateStep{what: "ask logout", act: func(sh *shell.Shell, _ *mobileWire) { sh.AskLogout() }}, sheetOpen(),
			tapNamed("logout"), routeIs(env.RouteLogin), signedOutDone(), logStep("logged out"))},
	}
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

func init() { //nolint:gochecknoinits // the account's states join the table of states_mobile.go
	if !devParts { // the shipped live build has no states
		return
	}
	for id, st := range accountStates() {
		shotStates[id] = st
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// protoAccount is demo with the app config's links.privacy and links.terms emptied:
// the account of the states that are compared with the prototype's (M15,
// M15B-password, M15D, M15-logout) has no row for the privacy policy or the terms of
// use, which the prototype lacks (K32, K34).
func protoAccount(k *wireKnobs) {
	demo(k)
	k.noPrivacy = true
}

// privacyShown scrolls screen 15 to its end and waits for the row "Chính sách bảo
// mật" to be there.
func privacyShown() stateStep {
	return stateStep{what: "privacy row", when: func(sh *shell.Shell) bool {
		a, ok := sh.Nav().Screen().(*screens.Account)

		return ok && a.PrivacyShown()
	}, act: func(sh *shell.Shell, _ *mobileWire) {
		if a, ok := sh.Nav().Screen().(*screens.Account); ok {
			a.Scroll().ScrollBy(geom.Pt(0, 10000))
		}
	}}
}

// signedInNoTV is signedIn(true) with the mock's Apple TV deleted through the
// overlay: the prototype's account with no TV linked.
func signedInNoTV(ctx context.Context, w *mobileWire) error {
	if err := signedIn(true)(ctx, w); err != nil {
		return err
	}
	// the client's own: prepare runs inside setup, which the wire's calls wait for
	if err := w.client.DeleteDevice(ctx, mockTV); err != nil {
		return fmt.Errorf("unlink the TV: %w", err)
	}

	return nil
}

// resetOverlay is signedIn(true) after the overlay is emptied, as the dev
// menu's "Reset overlay" does.
func resetOverlay(ctx context.Context, w *mobileWire) error {
	if w.overlay == nil {
		return fmt.Errorf("reset the overlay: %w", errNoMock)
	}
	if err := w.overlay.Reset(); err != nil {
		return fmt.Errorf("reset the overlay: %w", err)
	}
	w.cache.Clear()

	return signedIn(true)(ctx, w)
}

// meShown waits for screen 15 with the user's name.
func meShown() stateStep {
	return stateStep{what: "15 shown", when: func(sh *shell.Shell) bool {
		a, ok := sh.Nav().Screen().(*screens.Account)

		return ok && sh.Env().AccountView().HasMe && a.UserName() != ""
	}}
}

// logAccount logs what screen 15 shows.
func logAccount() stateStep {
	return stateStep{what: "log 15", act: func(sh *shell.Shell, w *mobileWire) {
		if a, ok := sh.Nav().Screen().(*screens.Account); ok {
			w.deps.Logf("dancefitvn: state: 15 shows %q %q TV %q", a.UserName(), a.Email(), a.TV())
		}
	}}
}

// accountSteps opens the account tab from Home and waits until GET /me and
// GET /me/devices answered and screen 15 shows them: the user's name, the
// three badges, "Chưa kết nối" (the prototype's M15).
func accountSteps() []stateStep {
	return []stateStep{homeLoaded(), goRoute(env.RouteAccount, nil), routeIs(env.RouteAccount), meLoaded(false)}
}

// accountLinkedSteps is accountSteps with the mock's TV linked.
func accountLinkedSteps() []stateStep {
	return []stateStep{homeLoaded(), goRoute(env.RouteAccount, nil), routeIs(env.RouteAccount), meLoaded(true)}
}

// connectSteps is M15 then screen 16 pushed, as the row "Kết nối Apple TV"
// does.
func connectSteps() []stateStep {
	return append(accountSteps(), pushRoute(env.RouteConnect), routeIs(env.RouteConnect))
}

// accountBadges is how many badges the mock's user has (me/me.json).
const accountBadges = 3

// meLoaded waits until the account's reads answered and screen 15 shows
// them: the name, the badges, the TV row as linked says.
func meLoaded(linked bool) stateStep {
	return stateStep{what: "me loaded", when: func(sh *shell.Shell) bool {
		v := sh.Env().AccountView()
		a, ok := sh.Nav().Screen().(*screens.Account)
		if !ok || !v.HasMe || !v.HasDevices {
			return false
		}

		return a.UserName() != "" && len(a.Badges()) == accountBadges &&
			(a.TV() == sh.Env().T("account.notLinked")) != linked
	}}
}

// coachLoaded waits until GET /me/coach-profile answered; a stub of 15C
// (before P6.2) reads nothing and passes.
func coachLoaded() stateStep {
	return stateStep{what: "coach loaded", when: func(sh *shell.Shell) bool {
		if _, stub := sh.Nav().Screen().(*screens.Stub); stub {
			return true
		}
		_, ok := sh.Nav().Screen().(*screens.Coach)

		return ok && sh.Env().AccountView().HasCoach
	}}
}

// pushRoute opens a route over the one showing, as a row with data-go does.
func pushRoute(route string) stateStep {
	return stateStep{what: "push " + route, act: func(sh *shell.Shell, _ *mobileWire) {
		sh.Go(nav.Route{Screen: route}, false)
	}}
}

// typeCode types the code into the boxes of 16 as a keyboard does (the
// view's text event on the focused field), then takes the focus away so
// that no keyboard shows. A stub of 16 (before P6.4) has no boxes: nothing
// is typed.
func typeCode(code string) stateStep {
	st := typeCodeOf(func() string { return code })
	st.what = "type " + code

	return st
}

// envPairCode is DFV_PAIR_CODE, read when PAIR types it (on Android the
// launch's extras are set after init), or pairCode.
func envPairCode() string {
	if c := devEnv(pairCodeEnv); c != "" {
		return c
	}

	return pairCode
}

// typeCodeOf is typeCode with the code code returns as the step runs.
func typeCodeOf(code func() string) stateStep {
	return stateStep{what: "type the code", target: func(sh *shell.Shell) ui.Widget {
		if b := findCodeBoxes(sh.Nav().Screen()); b != nil {
			return b.Field() // typed only once the boxes have a frame, in the view
		}

		return nil
	}, act: func(sh *shell.Shell, w *mobileWire) {
		boxes := findCodeBoxes(sh.Nav().Screen())
		w.mu.Lock()
		v := w.view
		w.mu.Unlock()
		if boxes == nil || v == nil {
			w.deps.Logf("dancefitvn: state: no code boxes on %T", sh.Nav().Screen())

			return
		}
		v.Focus(boxes.Field())
		v.HandleEvent(platform.Event{Kind: platform.EventText, Text: code()})
		v.Focus(nil)
	}}
}

// connectOf returns screen 16 when it shows.
func connectOf(sh *shell.Shell) *screens.Connect {
	c, _ := sh.Nav().Screen().(*screens.Connect)

	return c
}

// tapLink taps "Liên kết" of 16, as a finger does, once the code typed
// enabled it.
func tapLink() stateStep {
	target := func(sh *shell.Shell) ui.Widget {
		if c := connectOf(sh); c != nil && !c.Link().Hidden && !c.Link().Disabled {
			return c.Link()
		}

		return nil
	}

	return stateStep{what: "tap link", target: target,
		act: func(sh *shell.Shell, w *mobileWire) {
			if wd := target(sh); wd != nil {
				w.tap(wd)
			}
		},
		// the code is out, or answered
		settled: func(sh *shell.Shell) bool {
			c := connectOf(sh)

			return c == nil || c.Busy() || c.Linked()
		},
	}
}

// devicesRoute is the route id of GET /me/devices.
const devicesRoute = "devices.list"

// devicesMarks holds, by wire, how many reads of the devices were answered
// when pushConnectRead opened 16 (the states of both phones run side by
// side in the tests).
var devicesMarks sync.Map // *mobileWire -> int

// pushConnectRead opens 16 over 15 and waits until its read of the devices
// answered and was kept: a code confirmed while that read is out loses the
// TV the confirm kept to the read's late answer (accountPort.Devices keeps
// an answer whose context was cancelled), which a user who waits does not
// see.
func pushConnectRead() []stateStep {
	return []stateStep{
		{what: "push 16", act: func(sh *shell.Shell, w *mobileWire) {
			devicesMarks.Store(w, w.tr.answered(devicesRoute))
			sh.Go(nav.Route{Screen: env.RouteConnect}, false)
		}},
		routeIs(env.RouteConnect),
		{what: "16 read the devices", until: func(_ *shell.Shell, w *mobileWire) bool {
			v, _ := devicesMarks.Load(w)
			mark, _ := v.(int)
			if w.tr.answered(devicesRoute) <= mark {
				return false
			}
			devicesMarks.Delete(w)

			return true
		}},
	}
}

// connectIs waits until 16 shows the state linked tells, with nothing out.
func connectIs(linked bool) stateStep {
	return stateStep{what: fmt.Sprintf("16 linked %t", linked), when: func(sh *shell.Shell) bool {
		c := connectOf(sh)

		return c != nil && !c.Busy() && c.Linked() == linked
	}}
}

// accountRead waits until GET /me and GET /me/devices answered, whatever
// they hold.
func accountRead() stateStep {
	return stateStep{what: "account read", when: func(sh *shell.Shell) bool {
		v := sh.Env().AccountView()

		return v.HasMe && v.HasDevices
	}}
}

// connectFollows waits until 16 shows what the devices read tells.
func connectFollows() stateStep {
	return stateStep{what: "16 follows the devices", when: func(sh *shell.Shell) bool {
		c := connectOf(sh)
		_, linked := sh.Env().TVDevice()

		return c != nil && !c.Busy() && c.Linked() == linked
	}}
}

// logConnect logs the state 16 shows and the TV the account holds.
func logConnect() stateStep {
	return stateStep{what: "log 16", act: func(sh *shell.Shell, w *mobileWire) {
		c := connectOf(sh)
		tv, linked := sh.Env().TVDevice()
		w.deps.Logf("dancefitvn: state: 16 linked=%t, devices TV=%t %q", c != nil && c.Linked(), linked, tv.ID)
	}}
}

// findCodeBoxes returns the first visible code boxes under root.
func findCodeBoxes(root ui.Widget) *kit.CodeBoxes {
	if root == nil || root.Node().Hidden {
		return nil
	}
	if b, ok := root.(*kit.CodeBoxes); ok {
		return b
	}
	for ch := range root.Node().Children() {
		if b := findCodeBoxes(ch); b != nil {
			return b
		}
	}

	return nil
}

// holdFinishedToast tells the shell that the audio of the lesson finished, as
// the downloads do (DownloadEventFinished: "Đã tải xong bài NN", the prototype's
// M-toast), and tells it again every toastRearm with the motion reduced (no
// fade), so that the toast stays for the shot.
func holdFinishedToast(lessonID string, no int) stateStep {
	return stateStep{what: "toast", act: func(sh *shell.Shell, w *mobileWire) {
		w.mu.Lock()
		v := w.view
		w.mu.Unlock()
		if v == nil {
			return
		}
		v.Context().Theme().ReduceMotion = true
		ev := env.DownloadEvent{Kind: env.DownloadEventFinished, LessonID: lessonID, No: no}
		var again func()
		again = func() {
			sh.Env().Downloader().Events().Publish(ev)
			v.Context().AfterFunc(toastRearm, again)
		}
		again()
	}}
}

// avatarSheet returns sheet 15D when it is the open sheet's content.
func avatarSheet(sh *shell.Shell) *screens.AvatarSheet {
	if !sh.Nav().Sheet().IsOpen() {
		return nil
	}
	a, _ := sh.Nav().Sheet().Content().(*screens.AvatarSheet)

	return a
}

// tapAvatarSheet taps a widget of sheet 15D, as a finger does: "Chọn từ thư
// viện" (the system's picker, which local/p6/_phototap taps on the device), or
// "Xác nhận"; busy, when set, tells that the tap had its effect.
func tapAvatarSheet(what string, of func(*screens.AvatarSheet) ui.Widget, busy func(*screens.AvatarSheet) bool) stateStep {
	target := func(sh *shell.Shell) ui.Widget {
		if a := avatarSheet(sh); a != nil {
			return of(a)
		}

		return nil
	}
	st := stateStep{what: "tap " + what, target: target, act: func(sh *shell.Shell, w *mobileWire) {
		if wd := target(sh); wd != nil {
			w.tap(wd)
		}
	}}
	if busy != nil {
		st.settled = func(sh *shell.Shell) bool {
			a := avatarSheet(sh)

			return a == nil || busy(a)
		}
	}

	return st
}

// avatarPicked waits for the picture the user chose in the system's picker,
// in sheet 15D.
func avatarPicked() stateStep {
	return stateStep{what: "avatar picked", when: func(sh *shell.Shell) bool {
		a := avatarSheet(sh)

		return a != nil && a.Picked() && !a.Picking()
	}, act: func(sh *shell.Shell, w *mobileWire) { w.deps.Logf("dancefitvn: state: avatar picked") }}
}

// avatarUploaded waits for sheet 15D to close on PUT /me/avatar's answer: the
// kept user has the URL it answered, not the session's, and logs it. The
// picture sent is DFV_AVATAR_DUMP's.
func avatarUploaded() stateStep {
	return stateStep{what: "avatar uploaded", when: func(sh *shell.Shell) bool {
		if sh.Nav().Sheet().IsOpen() {
			return false
		}
		v := sh.Env().AccountView()
		u, _ := sh.Env().Account().SessionUser()

		return v.HasMe && v.Me.User.AvatarURL != "" && v.Me.User.AvatarURL != u.AvatarURL
	}, act: func(sh *shell.Shell, w *mobileWire) {
		w.deps.Logf("dancefitvn: state: avatar uploaded %s", sh.Env().AccountView().Me.User.AvatarURL)
	}}
}

// infoShown waits for screen 15B with the user in its fields.
func infoShown() stateStep {
	return stateStep{what: "15B", when: func(sh *shell.Shell) bool {
		f, ok := sh.Nav().Screen().(*screens.Info)

		return ok && f.First().Text() != ""
	}}
}

// typeInfo types a new first and last name in 15B.
func typeInfo() stateStep {
	return stateStep{what: "type the name", act: func(sh *shell.Shell, w *mobileWire) {
		if f, ok := sh.Nav().Screen().(*screens.Info); ok {
			w.deps.Logf("dancefitvn: state: 15B before: %q %q", f.Last().Text(), f.First().Text())
			f.First().SetText(infoFirst)
			f.Last().SetText(infoLast)
		}
	}}
}

// tapSubmit taps "Cập nhật" of 15B, as a finger does.
func tapSubmit() stateStep {
	info := func(sh *shell.Shell) *screens.Info {
		f, _ := sh.Nav().Screen().(*screens.Info)

		return f
	}

	return stateStep{what: "tap submit",
		target: func(sh *shell.Shell) ui.Widget {
			if f := info(sh); f != nil {
				return f.Submit()
			}

			return nil
		},
		act: func(sh *shell.Shell, w *mobileWire) {
			if f := info(sh); f != nil {
				w.tap(f.Submit())
			}
		},
		// the save is under way (taps are ignored from then), or over
		settled: func(sh *shell.Shell) bool {
			f := info(sh)

			return f == nil || f.Busy()
		},
	}
}

// logInfo logs what 15B shows when it is opened again.
func logInfo() stateStep {
	return stateStep{what: "log 15B", act: func(sh *shell.Shell, w *mobileWire) {
		if f, ok := sh.Nav().Screen().(*screens.Info); ok {
			w.deps.Logf("dancefitvn: state: 15B again: %q %q", f.Last().Text(), f.First().Text())
		}
	}}
}

// signedOutDone waits until the sign-out the user asked for ended.
func signedOutDone() stateStep {
	return stateStep{what: "signed out", until: func(_ *shell.Shell, w *mobileWire) bool {
		w.mu.Lock()
		going := w.loggingOut != nil
		w.mu.Unlock()
		_, signed := w.mgr.Session()

		return !going && !signed
	}}
}

// logStep logs a line of the state's progress.
func logStep(what string) stateStep {
	return stateStep{what: "log " + what, act: func(_ *shell.Shell, w *mobileWire) {
		w.deps.Logf("dancefitvn: state: %s", what)
	}}
}
