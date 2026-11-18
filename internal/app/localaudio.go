//go:build !tvos

package app

import (
	"encoding/json"
	"fmt"
	"sync"

	"dancefitvn-app/internal/download"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// damagedKey is the file of the prefs folder that keeps the verdicts of
// localAudio from one run to the next (K49).
const damagedKey = "damaged_audio"

// localAudio watches the files of the downloads handed to the player, for
// the file that is the right size (download.Manager.Path checks it) but that
// the system cannot play (a disk that damaged it): a file handed over twice
// without having played in between is spoilt, and the lesson is streamed
// instead, as a lesson with no file is. The file is not deleted; the screens
// mark it in red (K49) until the user deletes it. The verdict is kept in the
// prefs folder with the fingerprint of the file it was made on (size, sha256
// and time of the download), so that it holds across runs and a file
// downloaded again is judged anew. Any goroutine.
type localAudio struct {
	mu    sync.Mutex
	tried map[string]bool   // handed over, not played yet
	bad   map[string]string // lesson to the fingerprint of the file judged spoilt

	// print returns the fingerprint of the file of a lesson on the device, ""
	// when there is none; changed is told, on any goroutine, of a lesson newly
	// judged; save writes the verdicts (it runs off the UI thread). All are set
	// by the wire; nil they do nothing.
	print   func(lessonID string) string
	changed func(lessonID string)
	save    func(run func())

	saveMu sync.Mutex
	store  func(key string, data []byte) error
}

// damagedFile is the file of damagedKey.
type damagedFile struct {
	Bad map[string]string `json:"bad"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// use tells whether the file of lessonID may be handed to the player now.
func (l *localAudio) use(lessonID string) bool {
	fp := l.fingerprint(lessonID)
	l.mu.Lock()
	if have, ok := l.bad[lessonID]; ok {
		if have == fp {
			l.mu.Unlock()

			return false
		}
		delete(l.bad, lessonID) // another file under the same lesson: judged anew
		defer l.flush()
	}
	if l.tried[lessonID] {
		delete(l.tried, lessonID)
		if l.bad == nil {
			l.bad = map[string]string{}
		}
		l.bad[lessonID] = fp
		l.mu.Unlock()
		l.told(lessonID)

		return false
	}
	if l.tried == nil {
		l.tried = map[string]bool{}
	}
	l.tried[lessonID] = true
	l.mu.Unlock()

	return true
}

// played says that the file of lessonID loaded: it is sound.
func (l *localAudio) played(lessonID string) {
	l.mu.Lock()
	delete(l.tried, lessonID)
	l.mu.Unlock()
}

// damaged tells whether the file of lessonID on the device is the one judged
// spoilt. It allocates nothing while no file is.
func (l *localAudio) damaged(lessonID string) bool {
	l.mu.Lock()
	have, ok := l.bad[lessonID]
	l.mu.Unlock()
	if !ok {
		return false
	}

	return have == l.fingerprint(lessonID)
}

// forget drops what is known of lessonID: its file was deleted, or is
// downloaded again.
func (l *localAudio) forget(lessonID string) {
	l.mu.Lock()
	delete(l.tried, lessonID)
	_, had := l.bad[lessonID]
	delete(l.bad, lessonID)
	l.mu.Unlock()
	if had {
		l.flush()
	}
}

// load takes the verdicts a run before kept (data of damagedKey); a file that
// is not readable tells nothing.
func (l *localAudio) load(data []byte) {
	var f damagedFile
	if json.Unmarshal(data, &f) != nil {
		return
	}
	l.mu.Lock()
	for id, fp := range f.Bad {
		if l.bad == nil {
			l.bad = map[string]string{}
		}
		if _, ok := l.bad[id]; !ok && fp != "" {
			l.bad[id] = fp
		}
	}
	l.mu.Unlock()
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// fingerprint is print, "" without one.
func (l *localAudio) fingerprint(lessonID string) string {
	if l.print == nil {
		return ""
	}

	return l.print(lessonID)
}

// told tells the wire a lesson was judged now, and has the verdicts written.
func (l *localAudio) told(lessonID string) {
	l.flush()
	if l.changed != nil {
		l.changed(lessonID)
	}
}

// flush has the verdicts written, off the calling goroutine: what is written
// is what is known when the write runs, so that two writes in a row land in
// order.
func (l *localAudio) flush() {
	if l.save == nil || l.store == nil {
		return
	}
	l.save(l.write)
}

// write saves the verdicts as they are now.
func (l *localAudio) write() {
	l.saveMu.Lock()
	defer l.saveMu.Unlock()
	l.mu.Lock()
	data, err := json.Marshal(damagedFile{Bad: l.bad})
	l.mu.Unlock()
	if err == nil {
		_ = l.store(damagedKey, data)
	}
}

// entryPrint is the fingerprint of a file on the device.
func entryPrint(e download.Entry) string {
	return fmt.Sprintf("%d:%s:%d", e.Size, e.SHA256, e.DownloadedAt.UnixNano())
}
