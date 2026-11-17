package download

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"dancefitvn-app/internal/api"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// indexName is the file in the audio folder that lists the downloads.
const indexName = "index.json"

// Meta is what the downloads screens show about a lesson (11 and 13) and
// must keep while offline and after the API cache is cleared at sign-out.
// It is written at Start; an index written before it existed reads as zero.
type Meta struct {
	No            int    `json:"no,omitzero"`
	Title         string `json:"title,omitzero"`
	Tag           string `json:"tag,omitzero"`
	DurationSec   int    `json:"duration_sec,omitzero"`
	CatalogBytes  int64  `json:"catalog_bytes,omitzero"` // lesson.audio_size_bytes
	CourseName    string `json:"course_name,omitzero"`
	CourseLessons int    `json:"course_lessons,omitzero"`
	// CoverURL is the course's cover_url: the offline list draws the real
	// cover from the disk cache with it (K14). Entries written before have none.
	CoverURL string `json:"course_cover_url,omitzero"`
}

// Entry is one finished download (05 §6).
type Entry struct {
	LessonID     string    `json:"lesson_id"`
	CourseID     string    `json:"course_id"`
	UserID       string    `json:"user_id"`
	Size         int64     `json:"size"`
	SHA256       string    `json:"sha256"`
	DownloadedAt time.Time `json:"downloaded_at"`
	Meta
}

// Pending is a download that has not finished; it survives a restart and
// is loaded as a paused job (the part file holds the bytes so far).
type Pending struct {
	LessonID string `json:"lesson_id"`
	CourseID string `json:"course_id"`
	UserID   string `json:"user_id"`
	Seq      int    `json:"seq"`
	// Validator is the ETag or Last-Modified of the part file's server
	// answer: a resume after a restart sends it as If-Range.
	Validator string `json:"validator,omitempty"`
	Meta
}

// index is the list of downloads on disk. It is safe for concurrent use.
type index struct {
	path string

	mu      sync.Mutex
	entries []Entry
	pending []Pending
}

// indexDoc is the file format.
type indexDoc struct {
	Items   []Entry   `json:"items"`
	Pending []Pending `json:"pending,omitempty"`
}

// indexes is the index of each folder opened in this process.
var indexes = struct {
	sync.Mutex
	byPath map[string]*index
}{byPath: map[string]*index{}}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// openIndex reads dir/index.json; a missing or unreadable file is an
// empty index (the files stay on disk, the list is rebuilt by downloading
// again).
//
// Managers of different users on one folder share one index in memory, so
// that neither overwrites the other's entries when it writes the file.
func openIndex(dir string) *index {
	path := filepath.Join(dir, indexName)
	indexes.Lock()
	defer indexes.Unlock()
	if ix, ok := indexes.byPath[path]; ok {
		return ix
	}
	ix := &index{path: path}
	indexes.byPath[path] = ix
	data, err := os.ReadFile(ix.path)
	if err != nil {
		return ix
	}
	var doc indexDoc
	// api.Decode: the titles an older build stored as the API sent them
	// come out composed (NFC).
	if api.Decode(data, &doc) == nil {
		ix.entries, ix.pending = doc.Items, doc.Pending
	}

	return ix
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// finish records the entry of a finished download and drops the pending
// record of the same lesson and user, in one write. The entry is keyed by
// (user, lesson): another user's entry of the lesson stays, unless its
// sha256 differs (the shared file was replaced by another version).
func (ix *index) finish(e Entry) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.pending = slices.DeleteFunc(ix.pending, func(p Pending) bool { return p.LessonID == e.LessonID && p.UserID == e.UserID })
	ix.entries = slices.DeleteFunc(ix.entries, func(x Entry) bool {
		return x.LessonID == e.LessonID && x.UserID != e.UserID && !strings.EqualFold(x.SHA256, e.SHA256)
	})
	if i := slices.IndexFunc(ix.entries, func(x Entry) bool { return x.LessonID == e.LessonID && x.UserID == e.UserID }); i >= 0 {
		ix.entries[i] = e
	} else {
		ix.entries = append(ix.entries, e)
	}

	return ix.writeLocked()
}

// putPending adds or replaces the pending record of a lesson and user.
func (ix *index) putPending(p Pending) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if i := slices.IndexFunc(ix.pending, func(x Pending) bool { return x.LessonID == p.LessonID && x.UserID == p.UserID }); i >= 0 {
		ix.pending[i] = p
	} else {
		ix.pending = append(ix.pending, p)
	}

	return ix.writeLocked()
}

// removePending drops the pending record of a lesson and user.
func (ix *index) removePending(lessonID, userID string) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	n := len(ix.pending)
	ix.pending = slices.DeleteFunc(ix.pending, func(x Pending) bool { return x.LessonID == lessonID && x.UserID == userID })
	if len(ix.pending) == n {
		return nil
	}

	return ix.writeLocked()
}

// pendingOf returns the pending records of a user in start order.
func (ix *index) pendingOf(userID string) []Pending {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	out := slices.DeleteFunc(slices.Clone(ix.pending), func(p Pending) bool { return p.UserID != userID })
	slices.SortStableFunc(out, func(a, b Pending) int { return a.Seq - b.Seq })

	return out
}

// remove drops the entry of a lesson of a user and writes the file.
func (ix *index) remove(lessonID, userID string) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	n := len(ix.entries)
	ix.entries = slices.DeleteFunc(ix.entries, func(x Entry) bool { return x.LessonID == lessonID && x.UserID == userID })
	if len(ix.entries) == n {
		return nil
	}

	return ix.writeLocked()
}

// othersHold reports whether a user other than userID has a finished
// entry of the lesson (the file is shared, so it must stay).
func (ix *index) othersHold(lessonID, userID string) bool {
	ix.mu.Lock()
	defer ix.mu.Unlock()

	return slices.ContainsFunc(ix.entries, func(x Entry) bool { return x.LessonID == lessonID && x.UserID != userID })
}

// othersPending reports whether a user other than userID has a pending
// record of the lesson (the part file is shared, so it must stay).
func (ix *index) othersPending(lessonID, userID string) bool {
	ix.mu.Lock()
	defer ix.mu.Unlock()

	return slices.ContainsFunc(ix.pending, func(x Pending) bool { return x.LessonID == lessonID && x.UserID != userID })
}

// get returns the entry of a lesson of a user.
func (ix *index) get(lessonID, userID string) (Entry, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	i := slices.IndexFunc(ix.entries, func(x Entry) bool { return x.LessonID == lessonID && x.UserID == userID })
	if i < 0 {
		return Entry{}, false
	}

	return ix.entries[i], true
}

// all returns a copy of the entries.
func (ix *index) all() []Entry {
	ix.mu.Lock()
	defer ix.mu.Unlock()

	return slices.Clone(ix.entries)
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// writeLocked writes the index through a temporary file and a rename, so
// that a crash leaves the old or the new one.
func (ix *index) writeLocked() error {
	data, err := json.Marshal(indexDoc{Items: ix.entries, Pending: ix.pending}, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("download: encode index: %w", err)
	}
	if err := writeAtomic(ix.path, data); err != nil {
		return fmt.Errorf("download: write index: %w", err)
	}

	return nil
}

// writeAtomic writes data to path: temp file, fsync, rename.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())

		return err
	}

	return nil
}

// removeFile deletes a file; a missing file is not an error.
func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	return nil
}
