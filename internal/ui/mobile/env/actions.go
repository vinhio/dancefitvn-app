//go:build !tvos

package env

import (
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/ui/mobile/nav"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The ids of the routes (nav.Route.Screen), the prototype's `S.m.scr`.
const (
	// RouteSplash is MS.splash, the first screen.
	RouteSplash = "splash"
	// RouteLogin is MS.login; Params is LoginParams.
	RouteLogin = "login"
	// RouteUpgrade is the screen of a version the server refuses (03 section 9).
	RouteUpgrade = "upgrade"
	// RouteHome is MS.home, with the loading state (MS.loading) in it.
	RouteHome = "home"
	// RouteCourses is MS.courses.
	RouteCourses = "courses"
	// RouteDetail is MS.detail, and MS.locked for a course not enrolled; Params
	// is the course id (a string).
	RouteDetail = "detail"
	// RouteOffline is MS.offline of a course; Params is the course id.
	RouteOffline = "offline"
	// RouteDownloads, RouteAccount are the tabs P5 and P6 fill in; until then
	// they are stubs.
	RouteDownloads = "downloads"
	RouteAccount   = "account"
	// RouteVideo is MS.video (06), RouteAudio MS.audio (08, "Nghe") and RouteDone
	// MS.done (10, "Hết bài"). They have no Params: the lesson is the player's
	// (Playback). Open them with the actions PlayVideo, PlayAudio and PlayNext,
	// never with Go, so that the lesson starts first.
	RouteVideo = "video"
	RouteAudio = "audio"
	RouteDone  = "done"
	// RouteInfo, RouteCoach, RouteConnect are the screens of P5 and P6.
	RouteInfo    = "info"
	RouteCoach   = "coach"
	RouteConnect = "connect"
)

// LoginBanner is the banner on top of the login form.
type LoginBanner uint8

// The banners. LoginBannerUnknown is the zero value and is shown as no banner,
// like LoginBannerNone.
const (
	LoginBannerUnknown LoginBanner = iota
	// LoginBannerNone is no banner, said explicitly.
	LoginBannerNone
	// LoginBannerExpired is "Phiên đăng nhập đã hết hạn" (03 section 9): the
	// session was ended by the server, or the biometric slot was refused.
	LoginBannerExpired
)

// LoginParams is the Params of the login route.
type LoginParams struct {
	// Banner is shown over the form from the start.
	Banner LoginBanner
}

// UpgradeParams is the Params of the upgrade route.
type UpgradeParams struct {
	// StoreURL is the store page by platform ("ios", "android") of the 426's
	// body; empty when the app config only said force_update.
	StoreURL map[string]string
}

// Actions are the actions of 03 section 3 the screens call, named as the
// prototype's `data-act`. The Shell implements them; a screen never opens a
// route itself.
type Actions interface {
	// Go opens a route (`go()`); root clears the stack (a tab, `data-root`).
	Go(r nav.Route, root bool)
	// Back is `back`.
	Back()
	// Toast shows a toast for 2200 ms (`toast`).
	Toast(msg string)
	// SignedIn is what a login that succeeded does: Home on an empty stack
	// (03 section 1).
	SignedIn()
	// Detail is `data-go="detail"`: the course's detail, or the offline screen
	// of the course while offline (03 section 2).
	Detail(courseID string)
	// PlayVideo is `playVideo` (03 section 3): offline it toasts "Video bài {NN}
	// cần có mạng để xem"; online it starts the lesson on the player and shows
	// screen 06, in place of 06 or 10 when one of them is showing (Replace), on
	// top of the stack otherwise.
	PlayVideo(courseID, lessonID string, no int)
	// PlayAudio is `playAudio`: it starts the sound of the lesson and shows
	// screen 08 on top of the stack.
	PlayAudio(courseID, lessonID string, no int)
	// OffTap is `offTap`: a lesson of the offline screen; a downloaded one is
	// PlayAudio, else the toast of PlayVideo.
	OffTap(courseID, lessonID string, no int)
	// PlayNext is the end-of-lesson countdown reaching 0 (`$P:788-790`): the
	// player plays the next lesson, at 0, and screen 06 takes the place of
	// screen 10, its controls shown. It does nothing when the preference
	// "Tự phát bài tiếp theo" is off or the course has no next lesson.
	PlayNext()
	// ToDetail is `toDetail` of screen 10: the course's detail with the stack
	// [courses] under it.
	ToDetail(courseID string)
	// DlTap is `dlTap` (`$P:594-604`): a lesson with no file opens sheet 09 on
	// "Chỉ bài này", or toasts "Không có mạng để tải" offline; one under way is
	// canceled with the toast "Đã huỷ tải"; a downloaded one opens sheet 14.
	DlTap(courseID, lessonID string, no int)
	// DlAll is `dlAll`: sheet 09 on "Cả khoá", on the first lesson with no
	// file; offline it toasts "Không có mạng để tải".
	DlAll(courseID string)
	// AskDelete is `askDelete`: sheet 14 for the lesson's file.
	AskDelete(courseID, lessonID string, no int)
	// StartDownloads is `dlConfirm` of sheet 09: it starts the download of the
	// lessons (those not done or under way), shows "Đang tải file âm thanh…"
	// and closes the sheet. A device too full to hold them toasts "Không đủ
	// bộ nhớ" and leaves the sheet open; Wi-Fi only on mobile data closes the
	// sheet with "Đang chờ Wi‑Fi để tải" (K23, the lessons start on Wi-Fi).
	StartDownloads(c model.Course, lessons []model.Lesson)
	// DeleteDownload is `delConfirm` of sheet 14: the file goes, the sheet
	// closes and "Đã xoá file âm thanh" shows.
	DeleteDownload(lessonID string)

	// AskPassword is `data-act="sheet" data-v="password"` of screens 15 and 15B:
	// SH.password opens.
	AskPassword()
	// AskLogout is `sheet` v=logout of screen 15: SH.logout, "Đăng xuất?".
	AskLogout()
	// AskAvatar is `sheet` v=avatar of screen 15: SH.avatar (15D), at the
	// prototype's zoom 1.2 and no new picture.
	AskAvatar()
	// AskDevices opens the sheet of devices of screen 16 for a TV code the server
	// refused for the limit (03 section 9).
	AskDevices(p DevicesSheetParams)
	// Logout is `logout` of SH.logout (`$P:530`): the sheet closes, the sound
	// stops and the player Leaves (its last position is saved), the login shows at
	// once with no banner, and the sign-out itself (Account.Logout: the queue sent,
	// the server told, the vault cleared) runs off the UI thread. The kept account
	// view is forgotten.
	Logout()
}

// OnlineAware is a screen that wants to know when the network goes and comes
// back (the offline bar of Home and Courses): the Shell calls SetOnline on the
// screen showing, on the UI thread.
type OnlineAware interface {
	SetOnline(online bool)
}

// ForegroundAware is a screen that wants to know when the app comes back to the
// front (Home and Courses, whose first read the system may have cut): the Shell
// calls Foregrounded on the screen showing, on the UI thread.
type ForegroundAware interface {
	Foregrounded()
}
