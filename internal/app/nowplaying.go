// Adapted from jplayer-mobile/internal/app/nowplaying.go at 49de3c2
// (npSink, driverNowPlaying, nowPlaying: onChange, command, update, clear,
// close, the 10 min clear, Interrupted, sameInfo, playbackState) and
// nowplaying_art.go (the artwork's checks). Trimmed: no live channels, no
// programmes, no queue, no previous and next (Q-P4-7), no drawn cover (a
// cover that cannot be read shows none, Q-P4-8). The player is DanceFITVN's,
// which tells no interruption: the bridge hears the audio session itself.

package app

import (
	"bytes"
	"math"
	"time"

	"dancefitvn-app/internal/clock"
	"dancefitvn-app/internal/media"
	"dancefitvn-app/internal/player"

	"github.com/gdeskdev/desk/bridge/system/nowplaying"
	"github.com/gdeskdev/desk/foundation/platform"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// npSink is the system's Now Playing (bridge/system/nowplaying on the
// app's driver): the lock screen, Control Center, Android's media
// notification, headsets. A test puts the fake driver's in its place.
type npSink interface {
	// Set shows np as what the app plays.
	Set(np platform.NowPlaying) error
	// Clear takes it away; the system then routes no command to the app.
	Clear() error
	// Handle sets the function told of each command; nil removes it.
	Handle(fn func(platform.MediaCommand)) error
}

// npArt reads the cover at url for the system's media controls and hands
// its bytes to done on the UI thread, later: a PNG or a JPEG of
// artMaxBytes at most (fitArtwork reduces a larger cover); nil when it
// could not be read or decoded. It may never call done (the window
// closed).
type npArt func(url string, done func(b []byte))

// npPausedFor is how long Now Playing stays while nothing plays before it
// is taken away (jPlayer's 04 section 3.3): Android's media service then
// stops, iOS's lock screen lets the app go.
const npPausedFor = 10 * time.Minute

// npCommands are the commands the system offers on both phones (Q-P4-7,
// 04 section 2.4): play, pause, toggle, ±10 s (MediaCommandSkip, which the
// iPhone's lock screen draws in place of the track buttons) and the
// scrubber. No previous and next. Android 13 and later draw their own
// controls from the session and may leave the ±10 s out (R-P4-5).
const npCommands = platform.MediaCommands(1<<platform.MediaCommandPlay | 1<<platform.MediaCommandPause |
	1<<platform.MediaCommandToggle | 1<<platform.MediaCommandSkip | 1<<platform.MediaCommandSeekTo)

// artMaxBytes is the most a cover may weigh to be handed to the system as
// it is; a heavier one is reduced first (fitArtwork).
const artMaxBytes = 2 << 20

// driverNowPlaying is the npSink of the app's driver.
type driverNowPlaying struct {
	driver func() platform.Driver
}

// npOptions configures newNowPlaying.
type npOptions struct {
	sink   npSink
	sess   media.Session // the audio session; nil hears no interruption
	clk    clock.Clock   // the wall clock: its timers run in the background
	art    npArt         // nil shows no artwork
	labels platform.MediaLabels
	ios    bool // iOS's Now Playing, which knows no interruption
	logf   func(string, ...any)
}

// nowPlaying shows the player's lesson to the system and hands the
// system's commands to the player (04 section 2.4, 3.4): the title is the
// lesson's, the artist the course's, the artwork the course's cover. It
// shows nothing until the player first plays; from then on it sets the
// information again only when the lesson, the state, the speed, the
// length or the position (a seek, the end of a load or a stall) changes,
// never at a tick: the system moves the position on by itself. Ten
// minutes without playing (on Android, without waiting for the end of an
// interruption), a sign-out and the window's closing take it away. It
// lives on the UI thread.
type nowPlaying struct {
	o npOptions
	p *player.Player

	info    platform.NowPlaying // what was shown last
	shown   bool                // information shows: Set since the last Clear
	handled bool                // the commands' function is registered
	off     bool                // the system has no Now Playing: nothing more is asked
	closed  bool
	curID   string // the lesson shown
	// state is the player's state at the last change followed: a load or
	// a stall that ends playing shows the position again.
	state player.State
	// interrupted: the audio session took the sound from a lesson that
	// played (a call); it shows as Interrupted while paused, which keeps
	// Android's media service in the foreground so that the player may
	// play again at its end.
	interrupted bool

	// The artwork: the bytes of the cover artURL, and the cover asked for.
	artBuf          []byte
	artURL, wantURL string
	artVer          int  // counts the changes of the artwork
	shownAt         int  // artVer when it was shown
	inSet           bool // update runs: an artwork that comes now waits for it

	pauseTimer clock.Timer
	pausedFn   func()                      // pausedLong, made once
	command1   func(platform.MediaCommand) // command, made once
	unsub      func()
	unsubSess  func()
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// newNowPlaying returns the bridge between the system's Now Playing and
// the player follow gives it. It listens to the audio session from now:
// made before the player, it hears an interruption before the player
// pauses for it.
func newNowPlaying(o npOptions) *nowPlaying {
	if o.logf == nil {
		o.logf = func(string, ...any) {}
	}
	n := &nowPlaying{o: o}
	n.pausedFn = n.pausedLong
	n.command1 = n.command
	if o.sess != nil {
		n.unsubSess = o.sess.Events().Subscribe(n.onSession)
	}

	return n
}

// follow listens to p until close. Call it once.
func (n *nowPlaying) follow(p *player.Player) {
	n.p = p
	n.state = p.State()
	n.unsub = p.Changes().Subscribe(n.onChange)
}

// npLabels are the names of the buttons Android's media notification
// draws up to Android 12 (TalkBack reads them): the play and pause
// buttons' aria-labels of the prototype, from texts (vi.json).
func npLabels(texts map[string]string) platform.MediaLabels {
	return platform.MediaLabels{Play: texts["a11y.play"], Pause: texts["a11y.pause"]}
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Set shows np (nowplaying.Set).
func (s driverNowPlaying) Set(np platform.NowPlaying) error { return nowplaying.Set(s.driver(), np) }

// Clear takes it away (nowplaying.Clear).
func (s driverNowPlaying) Clear() error { return nowplaying.Clear(s.driver()) }

// Handle sets the commands' function (nowplaying.Handle).
func (s driverNowPlaying) Handle(fn func(platform.MediaCommand)) error {
	return nowplaying.Handle(s.driver(), fn)
}

// onChange follows a change of the player: the lesson, the state, the
// speed, the length and a seek show again; a tick (ChangeTime) does
// nothing and allocates nothing. A load or a stall that turns playing
// shows the position again, as a seek does: the system shows them as
// playing (Loading counts as playing), so its clock ran on while nothing
// played.
func (n *nowPlaying) onChange(c player.Change) {
	if c.Kind == player.ChangeTime || n.closed {
		return
	}
	if n.interrupted && !n.p.Interrupted() {
		n.interrupted = false // the user paused or played in the app during the call
	}
	s := n.p.State()
	resumed := n.state == player.StateLoading && s == player.StatePlaying
	n.state = s
	if s == player.StatePlaying {
		n.interrupted = false
	}
	if resumed && n.shown {
		n.update(true)

		return
	}
	switch c.Kind {
	case player.ChangeLesson:
		n.interrupted = false
		n.update(false)
	case player.ChangeState, player.ChangeRate, player.ChangeDuration:
		n.update(false)
	case player.ChangeSeek:
		n.update(true)
	case player.ChangeUnknown, player.ChangeTime, player.ChangeOptions, player.ChangeEnded, player.ChangeError:
	default:
	}
}

// command carries out a command of the system's media controls, on the
// UI thread. A play or a pause of the user ends the waiting for the end
// of an interruption.
func (n *nowPlaying) command(c platform.MediaCommand) {
	if n.closed || n.p == nil {
		return
	}
	n.o.logf("dancefitvn: media command %s", c.Kind)
	p := n.p
	was := n.interrupted
	switch c.Kind {
	case platform.MediaCommandPlay:
		n.interrupted = false
		if s := p.State(); s != player.StatePlaying && s != player.StateLoading {
			p.Play() // a failed load loads again, an ended lesson starts over
		}
	case platform.MediaCommandPause:
		n.interrupted = false
		p.Pause()
	case platform.MediaCommandToggle:
		n.interrupted = false
		p.Toggle()
	case platform.MediaCommandSeekTo:
		p.Seek(c.Position.Seconds())
	case platform.MediaCommandSkip:
		p.SeekBy(c.Offset.Seconds())
	case platform.MediaCommandUnknown, platform.MediaCommandNext, platform.MediaCommandPrevious,
		platform.MediaCommandRaise: // not offered
	default:
	}
	if was && !n.interrupted && n.shown {
		n.update(false) // no longer held, though the player was paused already
	}
}

// clear takes Now Playing away (a sign-out, ten minutes without playing);
// the next play shows it again.
func (n *nowPlaying) clear() {
	n.stopPauseTimer()
	if !n.shown {
		return
	}
	n.shown = false
	n.curID = ""
	n.info = platform.NowPlaying{}
	n.o.logf("dancefitvn: now playing cleared")
	if err := n.o.sink.Clear(); err != nil {
		n.failed("clear", err)
	}
}

// close stops listening, takes Now Playing away and drops the commands'
// function, as the window closes.
func (n *nowPlaying) close() {
	if n.closed {
		return
	}
	n.clear()
	n.closed = true
	if n.unsub != nil {
		n.unsub()
	}
	if n.unsubSess != nil {
		n.unsubSess()
	}
	if n.handled {
		n.handled = false
		if err := n.o.sink.Handle(nil); err != nil {
			n.failed("handle", err)
		}
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// onSession follows the audio session. It is told before the player (it
// subscribed first): an interruption of a lesson that plays is
// remembered, and the pause the player makes for it shows as
// Interrupted. Its end, when the player does not play again, shows the
// lesson paused, which arranges the clearing.
func (n *nowPlaying) onSession(ev media.SessionEvent) {
	if n.closed || n.p == nil {
		return
	}
	switch ev.Kind {
	case media.InterruptBegin:
		n.interrupted = n.p.Playing()
	case media.InterruptEnd:
		was := n.interrupted
		n.interrupted = false
		if was && !ev.ShouldResume && n.shown {
			n.update(false)
		}
	case media.SessionEventUnknown, media.Noisy, media.Duck, media.Unduck, media.MediaReset:
	default:
	}
}

// update shows the player's lesson as it is now, when it differs from
// what shows (force: always, for a seek or an artwork come). Nothing
// shows before the player plays; no lesson clears.
func (n *nowPlaying) update(force bool) {
	if n.closed || n.off {
		return
	}
	l := n.p.Lesson()
	if l.ID == "" {
		n.clear()

		return
	}
	state := playbackState(n.p.State())
	if !n.shown && n.p.State() != player.StatePlaying {
		return // shown from the first play on
	}
	n.inSet = true
	n.wantArt(l.CoverURL)
	n.inSet = false
	n.p.SyncPos() // in the background no tick moved it
	dur := seconds(n.p.Dur())
	pos := seconds(n.p.Pos())
	if dur > 0 {
		pos = min(pos, dur)
	}
	next := platform.NowPlaying{
		Title: l.Title, Artist: l.CourseName,
		Duration: dur, Position: pos, Rate: n.p.Rate(), State: state,
		Commands: npCommands, Labels: n.o.labels,
		Interrupted: state == platform.PlaybackPaused && n.interrupted,
	}
	if n.artURL != "" && n.artURL == l.CoverURL && len(n.artBuf) > 0 {
		next.Artwork = n.artBuf
	}
	if n.shown && !force && n.curID == l.ID && n.shownAt == n.artVer && sameInfo(next, n.info) {
		n.followPause(state)

		return
	}
	if !n.handled {
		n.handled = true
		if err := n.o.sink.Handle(n.command1); err != nil {
			n.failed("handle", err)
			if n.off {
				return
			}
		}
	}
	n.info = next
	n.curID, n.shownAt = l.ID, n.artVer
	n.o.logf("dancefitvn: now playing %s of %s: %s at %.1f of %.1f s, rate %.2f, art %d B, interrupted %t",
		l.ID, l.CourseID, state, pos.Seconds(), dur.Seconds(), next.Rate, len(next.Artwork), next.Interrupted)
	if err := n.o.sink.Set(next); err != nil {
		n.failed("set", err)
		if n.off {
			return
		}
	}
	n.shown = true
	n.followPause(state)
}

// wantArt asks for the cover at url, unless it is had or coming.
func (n *nowPlaying) wantArt(url string) {
	if n.o.art == nil || url == "" || url == n.artURL || url == n.wantURL {
		return
	}
	if n.artURL != "" {
		n.artBuf, n.artURL = n.artBuf[:0], ""
		n.artVer++
	}
	n.wantURL = url
	n.o.art(url, func(b []byte) { n.artCame(url, b) })
}

// artCame keeps the cover b of url and shows it when the lesson showing
// has it; one of a course left meanwhile is dropped. A nil b (not read)
// is asked again at the next update.
func (n *nowPlaying) artCame(url string, b []byte) {
	if n.closed || n.wantURL != url {
		return
	}
	n.wantURL = ""
	if b == nil {
		return
	}
	n.artBuf = append(n.artBuf[:0], b...)
	n.artURL = url
	n.artVer++
	if n.inSet || !n.shown {
		return // the update under way, or the next one, takes it
	}
	if n.p.Lesson().CoverURL == url {
		n.update(true)
	}
}

// followPause arranges the clearing after npPausedFor while nothing
// plays, and drops it once something does.
func (n *nowPlaying) followPause(state platform.PlaybackState) {
	if state == platform.PlaybackPlaying {
		n.stopPauseTimer()

		return
	}
	if n.pauseTimer == 0 {
		n.pauseTimer = n.o.clk.After(npPausedFor, n.pausedFn)
	}
}

// pausedLong is the end of npPausedFor without playing. On Android an
// interruption under way (a long call) keeps Now Playing: the player
// plays again at its end, which Android allows only to a media service
// kept in the foreground; its end without playing arranges the clearing
// again (onSession). iOS knows no such hold and may never tell the end of
// an interruption: it clears there as after any pause.
func (n *nowPlaying) pausedLong() {
	n.pauseTimer = 0
	if n.closed || n.p.State() == player.StatePlaying || (!n.o.ios && n.interrupted) {
		return
	}
	n.o.logf("dancefitvn: now playing cleared after %s without playing", npPausedFor)
	n.clear()
}

// stopPauseTimer drops the clearing arranged.
func (n *nowPlaying) stopPauseTimer() {
	if n.pauseTimer != 0 {
		n.o.clk.Stop(n.pauseTimer)
		n.pauseTimer = 0
	}
}

// failed logs the failure of op; a system without Now Playing turns the
// bridge off.
func (n *nowPlaying) failed(op string, err error) {
	if isUnsupported(err) {
		n.off = true

		return
	}
	n.o.logf("dancefitvn: now playing: %s: %v", op, err)
}

// playbackState is the system's state for the player's: a load counts as
// playing, as it shows only once the player played; an end and an error
// are paused (the position stays shown).
func playbackState(s player.State) platform.PlaybackState {
	switch s {
	case player.StatePlaying, player.StateLoading:
		return platform.PlaybackPlaying
	case player.StatePaused, player.StateEnded, player.StateError:
		return platform.PlaybackPaused
	case player.StateUnknown, player.StateIdle:
		return platform.PlaybackStopped
	default:
		return platform.PlaybackStopped
	}
}

// sameInfo tells whether a and b show the same, but for the position,
// which moves, and the artwork, which artVer follows.
func sameInfo(a, b platform.NowPlaying) bool {
	return a.Title == b.Title && a.Artist == b.Artist && a.Duration == b.Duration &&
		a.Rate == b.Rate && a.State == b.State && a.Interrupted == b.Interrupted && a.Commands == b.Commands
}

// seconds converts the player's seconds to a duration: 0 for what is not
// a finite, positive length.
func seconds(s float64) time.Duration {
	if math.IsNaN(s) || math.IsInf(s, 0) || s <= 0 {
		return 0
	}

	return time.Duration(s * float64(time.Second))
}

// isPNGOrJPEG tells by its first bytes whether b is a PNG or a JPEG, the
// pictures every system's media controls take.
func isPNGOrJPEG(b []byte) bool {
	return bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) || bytes.HasPrefix(b, []byte{0xff, 0xd8, 0xff})
}
