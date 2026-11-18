//go:build !tvos

package shell

import (
	"context"
	"errors"
	"log"

	"dancefitvn-app/internal/format"
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
	"dancefitvn-app/internal/ui/kit"
	"dancefitvn-app/internal/ui/mobile/env"
	"dancefitvn-app/internal/ui/mobile/nav"
	"dancefitvn-app/internal/ui/mobile/screens"

	"github.com/gdeskdev/desk/framework/ui"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Go opens a route (`go()`, index.html:543); root clears the stack.
func (s *Shell) Go(r nav.Route, root bool) { s.nav.Go(r, root) }

// Back is `back` (03 section 1): what the Back key does.
func (s *Shell) Back() { s.nav.Back() }

// Toast shows msg for 2200 ms (`toastM`, index.html:552).
func (s *Shell) Toast(msg string) { s.nav.Toast(msg) }

// SignedIn is a sign-in that succeeded: Home on an empty stack (03 section 1).
func (s *Shell) SignedIn() { s.nav.Go(nav.Route{Screen: env.RouteHome}, true) }

// Detail is `data-go="detail"` (03 section 2): the course's detail, its offline
// screen while there is no network.
func (s *Shell) Detail(courseID string) {
	screen := env.RouteDetail
	if !s.e.Online() {
		screen = env.RouteOffline
	}
	s.nav.Go(nav.Route{Screen: screen, Params: courseID}, false)
}

// PlayVideo is `playVideo` (03 section 3): offline it toasts that the video needs
// the network; online it starts the lesson on the player and shows 06, in place
// of 06 or 10 (Replace), on the stack otherwise. The lessons of the course are
// the ones a screen read (Env.CachedCourse); a course not read yet, as the
// Home's continue card has it, is fetched first. Without a player (the services
// have none) it logs and does nothing.
func (s *Shell) PlayVideo(courseID, lessonID string, no int) {
	if !s.e.Online() {
		s.needNet(no)

		return
	}
	s.play(courseID, lessonID, player.KindVideo)
}

// PlayAudio is `playAudio` (03 section 3): the sound of the lesson starts and 08
// opens on the stack, also from 08.
func (s *Shell) PlayAudio(courseID, lessonID string, _ int) {
	s.play(courseID, lessonID, player.KindAudio)
}

// OffTap is `offTap` (03 section 2): a lesson of the offline screen plays its
// audio when the file is on the device, else toasts that the video needs the
// network.
func (s *Shell) OffTap(courseID, lessonID string, no int) {
	if st, _ := s.e.Downloads().State(lessonID); st == kit.DlDone {
		s.PlayAudio(courseID, lessonID, no)

		return
	}
	s.needNet(no)
}

// PlayNext is the countdown of 10 reaching 0 (`$P:788-790`): the next lesson
// plays from 0 and 06 takes the place of 10.
func (s *Shell) PlayNext() {
	pb := s.e.Playback()
	if pb == nil || !pb.AutoAdvance() {
		return
	}
	s.show(env.RouteVideo, true)
}

// ToDetail is `toDetail` (`$P:612`): the stack is [courses] and the course's
// detail on it.
func (s *Shell) ToDetail(courseID string) {
	s.nav.Go(nav.Route{Screen: env.RouteCourses}, true)
	s.Detail(courseID)
}

// DlTap is `dlTap` (`$P:598`): a lesson with no file opens sheet 09 on "Chỉ bài
// này", or toasts that there is no network to download with; one under way is
// forgotten, with the toast "Đã huỷ tải" (also offline); a downloaded one opens
// sheet 14.
func (s *Shell) DlTap(courseID, lessonID string, no int) {
	st, _ := s.e.Downloads().State(lessonID)
	if st == kit.DlNone && s.noAudio(courseID, lessonID) {
		st = kit.DlUnavailable
	}
	switch st {
	case kit.DlIng:
		s.e.Downloader().Forget(lessonID)
		s.nav.Toast(s.e.T("toast.dlCanceled"))
	case kit.DlDone:
		s.AskDelete(courseID, lessonID, no)
	case kit.DlUnavailable: // the server has no audio for it: no sheet to fail late in
		s.nav.Toast(s.e.T("toast.audioMissing"))
	default: // no file (kit.DlNone, or the unknown that reads as it)
		s.openDownload(courseID, lessonID, env.DownloadChoiceOne)
	}
}

// noAudio tells whether the course data, when a screen read them, give the
// lesson no audio file (has_audio false or null).
func (s *Shell) noAudio(courseID, lessonID string) bool {
	c, ok := s.e.CachedCourse(courseID)
	if !ok {
		return false
	}
	for i := range c.Lessons {
		if c.Lessons[i].ID == lessonID {
			return !c.Lessons[i].HasAudio
		}
	}

	return false
}

// DlAll is `dlAll` (`$P:602`): offline it toasts that there is no network to
// download with; online sheet 09 opens on "Cả khoá", the lesson being the first
// with no file (the first of the course when all have one).
func (s *Shell) DlAll(courseID string) {
	s.openDownload(courseID, "", env.DownloadChoiceAll)
}

// AskDelete is `askDelete` (`$P:603`): sheet 14 for the lesson's file.
func (s *Shell) AskDelete(courseID, lessonID string, no int) {
	s.nav.Sheet().Open(screens.NewDeleteSheet(s.e, env.DeleteSheetParams{CourseID: courseID, LessonID: lessonID, No: no}),
		kit.SheetOptions{})
}

// StartDownloads is `dlConfirm` (`$P:600`) with the rules of 03 section 9 the
// prototype lacks: what is not done or under way is started; when the free
// space is known and the files do not fit, "Không đủ bộ nhớ" shows and nothing
// starts; on mobile data with "Chỉ tải qua Wi‑Fi" the lessons are queued and
// "Đang chờ Wi‑Fi để tải" shows (K23). The sheet closes when a download was
// queued, and stays when none was.
func (s *Shell) StartDownloads(c model.Course, lessons []model.Lesson) {
	dm := s.e.Downloader()
	todo := make([]model.Lesson, 0, len(lessons))
	var bytes int64
	missing := false // a lesson that has no audio file to download
	for _, l := range lessons {
		st, _ := dm.State(l.ID)
		if !l.HasAudio || st == kit.DlUnavailable {
			missing = true

			continue
		}
		if st == kit.DlDone || st == kit.DlIng {
			continue
		}
		todo = append(todo, l)
		bytes += l.AudioSizeBytes
	}
	if len(todo) == 0 {
		s.nav.Sheet().Close()
		if missing {
			s.nav.Toast(s.e.T("toast.audioMissing"))
		}

		return
	}
	if free, _, ok := dm.FreeSpace(); ok && bytes > free {
		s.nav.Toast(s.e.T("toast.noSpace"))

		return
	}
	err := dm.Start(c, todo)
	switch {
	case err == nil:
		s.nav.Sheet().Close()
		s.nav.Toast(s.e.T("toast.dlStarted"))
	case env.WiFiOnlyErr(err):
		s.nav.Sheet().Close()
		s.toastWaitWiFi()
	case env.NoSpace(err):
		s.nav.Toast(s.e.T("toast.noSpace"))
	case env.NoAudio(err):
		s.nav.Sheet().Close()
		s.nav.Toast(s.e.T("toast.audioMissing"))
	case env.Offline(err):
		s.nav.Toast(s.e.T("toast.noNetDownload"))
	default:
		log.Printf("dancefitvn: start downloads of %s: %v", c.ID, err)
		s.nav.Toast(s.e.T("toast.dlFailed"))
	}
}

// DeleteDownload is `delConfirm` (`$P:604`): the file goes, sheet 14 closes and
// "Đã xoá file âm thanh" shows. A sound that plays from the file goes on from the
// stream (Playback.Reload). A file that cannot be removed is logged; the
// sheet closes all the same, with no toast (there is no text for it in 03).
func (s *Shell) DeleteDownload(lessonID string) {
	err := s.e.Downloader().Delete(lessonID)
	s.nav.Sheet().Close()
	if err != nil {
		log.Printf("dancefitvn: delete download %s: %v", lessonID, err)

		return
	}
	// K38: the sound being played came from that file (the player asks for the file
	// first): it loads again from where it is, from the stream; offline that fails
	// as any load does (the shell toasts it)
	if pb := s.e.Playback(); pb != nil && pb.Kind() == player.KindAudio && pb.Lesson().ID == lessonID {
		pb.Reload()
	}
	s.nav.Toast(s.e.T("toast.dlDeleted"))
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// withCourse calls fn with the course: the one a screen read, else the one
// fetched (the Home's continue card names a course nothing has read yet). A
// refused version is the upgrade screen, a canceled read nothing; any other
// failure is fail's.
func (s *Shell) withCourse(courseID string, fn func(model.Course), fail func(error)) {
	if c, ok := s.e.CachedCourse(courseID); ok {
		fn(c)

		return
	}
	// offline with no course read in this run: the downloaded lessons of it are
	// the course (screen 13 plays them from the device)
	if !s.e.Online() {
		if c, ok := screens.OfflineCourse(s.e.Downloader().Items(), courseID); ok {
			fn(c)

			return
		}
	}
	ui.Go(s, func(ctx context.Context) (model.Course, error) {
		return s.e.Course(ctx, courseID)
	}, func(_ *ui.Context, c model.Course, err error) {
		switch {
		case err == nil:
			fn(c)
		case s.e.Refused(err):
		case errors.Is(err, context.Canceled):
		default:
			fail(err)
		}
	})
}

// play starts the lesson of kind on the player and shows its screen. The course
// is the one a screen read; one that was not is fetched, and the lesson starts
// when it is back.
func (s *Shell) play(courseID, lessonID string, kind player.Kind) {
	if s.e.Playback() == nil {
		log.Printf("dancefitvn: play %s/%s: the services have no player", courseID, lessonID)

		return
	}
	s.withCourse(courseID, func(c model.Course) { s.start(c, lessonID, kind) },
		func(error) { s.nav.Toast(s.e.T("player.err.load")) })
}

// openDownload opens sheet 09 on the course, or toasts that there is no network
// to download with. An empty lessonID is the first lesson with no file.
func (s *Shell) openDownload(courseID, lessonID string, choice env.DownloadChoice) {
	if !s.e.Online() {
		s.nav.Toast(s.e.T("toast.noNetDownload"))

		return
	}
	s.withCourse(courseID, func(c model.Course) {
		id := lessonID
		if id == "" {
			id = env.FirstMissing(c, s.e.Downloads())
		}
		s.nav.Sheet().Open(screens.NewDownloadSheet(s.e, env.DownloadSheetParams{Course: c, LessonID: id, Choice: choice}, s.nav.Sheet().Close),
			kit.SheetOptions{})
	}, func(err error) {
		if env.Offline(err) {
			s.nav.Toast(s.e.T("toast.noNetDownload"))

			return
		}
		s.nav.Toast(s.e.T("toast.dlFailed"))
	})
}

// watchDownloads follows the downloads' events (the shell's part of them): a
// file that finished toasts "Đã tải xong bài NN" while the app is in front
// (Q-P5-4), downloads held back by the rule Wi-Fi only toast "Đang chờ Wi‑Fi để
// tải" once, one that failed toasts why ("Không đủ bộ nhớ", else "Tải lỗi, thử
// lại") and is forgotten, so that its row goes back to no file (03 section 9).
// All three toasts are for the app in front only (N5): in the background they
// would be stale when the user comes back; the lists show the state.
func (s *Shell) watchDownloads() func() {
	return s.e.Downloader().Events().Subscribe(s.onDownload)
}

// onDownload is what the shell does on an event of the downloads.
func (s *Shell) onDownload(ev env.DownloadEvent) {
	switch ev.Kind {
	case env.DownloadEventFinished:
		// in the background the toast would only be there, stale, when the user
		// comes back (Q-P5-4)
		if s.e.Foreground() {
			s.nav.Toast(s.e.T("toast.downloaded") + " " + format.No(ev.No))
		}
		s.rearmWaitWiFi()
	case env.DownloadEventPaused:
		// like the finished toast, only while the app is in front (N5): a
		// pause in the background would be a stale toast when the user comes back
		if env.WiFiOnlyErr(ev.Err) && s.e.Foreground() {
			s.toastWaitWiFi()
		}
	case env.DownloadEventChanged:
		s.rearmWaitWiFi()
	case env.DownloadEventFailed:
		// the toast is only for the user in front (N5); the row goes back to no
		// file either way
		if s.e.Foreground() {
			switch {
			case env.NoSpace(ev.Err):
				s.nav.Toast(s.e.T("toast.noSpace"))
			case env.NoAudio(ev.Err): // not a failure to try again: the file is not there
				s.nav.Toast(s.e.T("toast.audioMissing"))
			default:
				s.nav.Toast(s.e.T("toast.dlFailed"))
			}
		}
		s.e.Downloader().Forget(ev.LessonID)
	case env.DownloadEventUnknown, env.DownloadEventProgress, env.DownloadEventSpace:
	default:
	}
}

// toastWaitWiFi shows "Đang chờ Wi‑Fi để tải" once for a pause: the network went
// to mobile data (or a start was queued there) with "Chỉ tải qua Wi‑Fi" on, and
// every download under way reports it, which one toast covers.
func (s *Shell) toastWaitWiFi() {
	if s.waitToasted {
		return
	}
	s.waitToasted = true
	s.nav.Toast(s.e.T("toast.waitWiFi"))
}

// rearmWaitWiFi lets toastWaitWiFi speak again once no download is held back by
// the rule Wi-Fi only (Wi-Fi is back, or the rule was turned off).
func (s *Shell) rearmWaitWiFi() {
	if !s.waitToasted {
		return
	}
	for _, j := range s.e.Downloader().Jobs() {
		if j.Paused && env.WiFiOnlyErr(j.Err) {
			return
		}
	}
	s.waitToasted = false
}

// start hands the lesson to the player, then shows the screen, so that the
// screen is built on a lesson that is there. The one case that goes the other
// way is a video asked from 06 itself (no button does, 06 has none for another
// lesson): the 06 that is left tells the player Leave, which would pause the
// lesson just started, so it is left first.
func (s *Shell) start(c model.Course, lessonID string, kind player.Kind) {
	list := env.PlayerLessons(c)
	i := env.LessonIndex(list, lessonID)
	if i < 0 {
		s.nav.Toast(s.e.T("player.err.load"))

		return
	}
	pb := s.e.Playback()
	if kind == player.KindAudio {
		pb.PlayAudio(list[i], list)
		s.show(env.RouteAudio, false)

		return
	}
	switch s.nav.Current().Screen {
	case env.RouteVideo:
		s.show(env.RouteVideo, true)
		pb.PlayVideo(list[i], list)
	case env.RouteDone:
		pb.PlayVideo(list[i], list)
		s.show(env.RouteVideo, true)
	default:
		pb.PlayVideo(list[i], list)
		s.show(env.RouteVideo, false)
	}
}

// show opens the screen of a player route, in place of the current one when
// replace is set.
func (s *Shell) show(screen string, replace bool) {
	r := nav.Route{Screen: screen}
	if replace {
		s.nav.Replace(r)

		return
	}
	s.nav.Go(r, false)
}

// watchPlayback follows the player's changes (the shell's part of them): a video
// that ends gives way to 10, a lesson that failed to load toasts, or opens the
// course's detail when the server says the user is not in it (03 section 9).
func (s *Shell) watchPlayback() func() {
	pb := s.e.Playback()
	if pb == nil {
		return nil
	}

	return pb.Changes().Subscribe(s.onPlayback)
}

// onPlayback is what the shell does on a change of the player.
func (s *Shell) onPlayback(ch player.Change) {
	pb := s.e.Playback()
	cur := s.nav.Current().Screen
	switch ch.Kind {
	case player.ChangeEnded:
		if cur == env.RouteVideo && pb.Kind() == player.KindVideo {
			s.nav.Replace(nav.Route{Screen: env.RouteDone})
		}
	case player.ChangeError:
		if cur != env.RouteVideo && cur != env.RouteAudio {
			return
		}
		if errors.Is(ch.Err, player.ErrNotEnrolled) {
			s.nav.Replace(nav.Route{Screen: env.RouteDetail, Params: pb.Lesson().CourseID})

			return
		}
		s.nav.Toast(s.e.T("player.err.load"))
	case player.ChangeUnknown, player.ChangeLesson, player.ChangeState, player.ChangeTime,
		player.ChangeDuration, player.ChangeRate, player.ChangeOptions, player.ChangeSeek:
	default:
	}
}

// needNet toasts "Video bài NN cần có mạng để xem".
func (s *Shell) needNet(no int) {
	s.nav.Toast(s.e.Fill("toast.videoNeedsNetTpl", "no", format.No(no)))
}
