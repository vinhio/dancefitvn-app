// The copy loop is adapted from jplayer-mobile/internal/sources/download.go
// at 49de3c2 (saveBody: chunked copy with context checks and progress),
// extended with append, Range, If-Range and a streaming sha256.

package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// State is where a job is.
type State int

// The states of a job. A finished job is gone from Jobs (it is in List).
const (
	StateUnknown State = iota
	StateQueued
	StateRunning
	StatePaused // network cut or Wi-Fi rule: ResumeAll continues it
	StateFailed // ErrNoSpace, ErrChecksum, a refusal: Start retries it
)

// outcome is how one HTTP attempt ended when it did not finish the file.
type outcome int

const (
	outcomeUnknown outcome = iota // zero value: never returned by transfer
	outcomeDone
	outcomeRefetch // the URL was refused: ask the API for a fresh one
	outcomeRestart // the part cannot be continued: start from zero
)

// Tunables.
const (
	chunkSize     = 32 << 10
	attempts      = 3
	expirySlack   = 30 * time.Second
	progressEvery = 250 * time.Millisecond // 4 events per second
)

// job is one lesson being downloaded; Manager.mu guards its fields.
type job struct {
	lessonID string
	courseID string
	meta     Meta
	seq      int
	state    State
	done     int64
	total    int64
	err      error

	cancel    context.CancelFunc
	finished  chan struct{} // closed when the goroutine of a run ends
	cancelled bool
	active    bool          // a goroutine of run is alive
	stopped   bool          // PauseAll cancelled the run; its state is set already
	after     chan struct{} // closed when the cleanup of an earlier job of the lesson is done

	audio     model.Audio // last answer of GET /lessons/{id}/audio
	hasAudio  bool
	validator string // ETag or Last-Modified of the last response
	lastEvent time.Time
	lastDone  int64 // Done of the last progress event, -1 none
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// execute downloads the lesson: fresh URL, space check, Range transfer,
// verification, rename, index. It returns nil when the file is in place.
func (m *Manager) execute(ctx context.Context, j *job) error {
	if j.after != nil { // the files of an earlier job of the lesson are still being removed
		select {
		case <-j.after:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	audio, err := m.audio(ctx, j, false)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.opts.Dir, 0o750); err != nil {
		return fmt.Errorf("download: folder: %w", err)
	}
	part := m.partPath(j.lessonID)
	have := fileSize(part)
	if m.opts.FreeSpace != nil && audio.SizeBytes > have {
		free, ferr := m.opts.FreeSpace()
		if ferr == nil && free < audio.SizeBytes-have {
			_ = removeFile(part)

			return ErrNoSpace
		}
	}
	for range attempts {
		out, err := m.transfer(ctx, j, audio)
		if err != nil {
			return err
		}
		switch out {
		case outcomeDone:
			return nil
		case outcomeRefetch:
			if audio, err = m.audio(ctx, j, true); err != nil {
				return err
			}
		case outcomeRestart:
			if err := removeFile(part); err != nil {
				return fmt.Errorf("download: restart: %w", err)
			}
		default:
			return fmt.Errorf("download: outcome %d: %w", out, core.ErrInvalid)
		}
	}

	return fmt.Errorf("download: server keeps refusing the range: %w", api.ErrServer)
}

// audio returns the audio answer of the lesson: the one in hand while its
// URL has time left, else a fresh one (force asks for one anyway).
func (m *Manager) audio(ctx context.Context, j *job, force bool) (model.Audio, error) {
	m.mu.Lock()
	a, ok := j.audio, j.hasAudio
	m.mu.Unlock()
	if ok && !force && (a.ExpiresAt.IsZero() || m.opts.Clock.Now().Add(expirySlack).Before(a.ExpiresAt)) {
		return a, nil
	}
	a, err := m.opts.Client.Audio(ctx, j.lessonID)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) { // the 404 of lessons.audio: there is no file to wait for
			return model.Audio{}, fmt.Errorf("%w: %w", ErrNoAudio, err)
		}

		return model.Audio{}, err
	}
	m.mu.Lock()
	j.audio, j.hasAudio = a, true
	m.mu.Unlock()

	return a, nil
}

// transfer makes one HTTP attempt: it continues the part with a Range
// request, appends the body, and on EOF verifies and installs the file.
func (m *Manager) transfer(ctx context.Context, j *job, a model.Audio) (outcome, error) {
	part := m.partPath(j.lessonID)
	have := fileSize(part)
	if a.SizeBytes > 0 && have > a.SizeBytes {
		if err := removeFile(part); err != nil {
			return outcomeDone, err
		}
		have = 0
	}
	h := sha256.New()
	if have > 0 {
		if err := hashFile(h, part, have); err != nil {
			have = 0
			h.Reset()
			if rerr := removeFile(part); rerr != nil {
				return outcomeDone, rerr
			}
		}
	}
	if a.SizeBytes > 0 && have == a.SizeBytes { // only the check and rename are left
		return outcomeDone, m.install(j, a, part, h, have)
	}

	m.mu.Lock()
	validator := j.validator
	m.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, http.NoBody)
	if err != nil {
		return outcomeDone, fmt.Errorf("download: request: %w", core.ErrInvalid)
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
		if validator != "" {
			req.Header.Set("If-Range", validator)
		}
	}
	resp, err := m.opts.HTTP.Do(req)
	if err != nil {
		return outcomeDone, netError(ctx, err)
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
	switch resp.StatusCode {
	case http.StatusPartialContent:
		if start, ok := rangeStart(resp.Header.Get("Content-Range")); !ok || start != have {
			return outcomeRestart, nil
		}
		m.logf("download: %s resume at %d (206)", j.lessonID, have)
	case http.StatusOK: // a server that ignores Range: start again
		if have > 0 {
			m.logf("download: %s restart (200)", j.lessonID)
		}
		have = 0
		h.Reset()
		flags = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	case http.StatusRequestedRangeNotSatisfiable:
		return outcomeRestart, nil
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusGone:
		return outcomeRefetch, nil
	case http.StatusNotFound:
		return outcomeDone, fmt.Errorf("download: media not found: %w", ErrNoAudio)
	default:
		if resp.StatusCode >= 500 {
			return outcomeDone, fmt.Errorf("download: media status %d: %w", resp.StatusCode, api.ErrServer)
		}

		return outcomeDone, fmt.Errorf("download: media status %d: %w", resp.StatusCode, core.ErrInvalid)
	}
	if v := resp.Header.Get("ETag"); v != "" {
		validator = v
	} else if v := resp.Header.Get("Last-Modified"); v != "" {
		validator = v
	}
	m.mu.Lock()
	if validator != j.validator {
		j.validator = validator
		if m.jobs[j.lessonID] == j && !j.cancelled { // persisted: a resume after a restart sends If-Range
			m.savePendingLocked(j)
		}
	}
	m.mu.Unlock()

	total := a.SizeBytes
	if total <= 0 && resp.ContentLength >= 0 {
		total = have + resp.ContentLength
	}
	got, err := m.copyBody(ctx, j, resp.Body, part, flags, h, have, total)
	if err != nil {
		return outcomeDone, err
	}
	if a.SizeBytes > 0 && got < a.SizeBytes { // closed early, without an error
		return outcomeDone, fmt.Errorf("download: body ended at %d of %d: %w", got, a.SizeBytes, api.ErrOffline)
	}

	return outcomeDone, m.install(j, a, part, h, got)
}

// copyBody appends r to the part file, hashing and reporting progress, and
// returns the size of the file.
func (m *Manager) copyBody(ctx context.Context, j *job, r io.Reader, part string, flags int, h hash.Hash, have, total int64) (int64, error) {
	f, err := os.OpenFile(part, flags, 0o600)
	if err != nil {
		return 0, fmt.Errorf("download: open part: %w", err)
	}
	got := have
	m.progress(j, got, total, true)
	buf := make([]byte, chunkSize)
	for {
		if err := ctx.Err(); err != nil {
			_ = f.Close()

			return got, err
		}
		n, rerr := r.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				_ = f.Close()
				if full := diskFull(werr); full != nil {
					_ = removeFile(part) // the bytes kept would only keep the disk full

					return got, full
				}

				return got, fmt.Errorf("download: write part: %w", werr)
			}
			h.Write(buf[:n])
			got += int64(n)
			m.progress(j, got, total, false)
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			_ = f.Close()

			return got, netError(ctx, rerr)
		}
	}
	if err := errors.Join(f.Sync(), f.Close()); err != nil {
		return got, fmt.Errorf("download: sync part: %w", err)
	}
	m.progress(j, got, total, true)

	return got, nil
}

// install checks the finished part against the answer of the API, renames
// it to <lesson>.mp3 and records it in the index. A file that fails the
// check is deleted.
func (m *Manager) install(j *job, a model.Audio, part string, h hash.Hash, size int64) error {
	sum := hex.EncodeToString(h.Sum(nil))
	if (a.SizeBytes > 0 && size != a.SizeBytes) || (a.SHA256 != "" && !strings.EqualFold(sum, a.SHA256)) {
		_ = removeFile(part)

		return ErrChecksum
	}
	final := m.finalPath(j.lessonID)
	if err := os.Rename(part, final); err != nil {
		return fmt.Errorf("download: install: %w", err)
	}
	m.mu.Lock()
	meta, courseID := j.meta, j.courseID
	m.mu.Unlock()
	e := Entry{LessonID: j.lessonID, CourseID: courseID, UserID: m.opts.UserID, Size: size, SHA256: sum, DownloadedAt: m.opts.Clock.Now(), Meta: meta}
	if err := m.idx.finish(e); err != nil {
		_ = removeFile(final)

		return err
	}

	return nil
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// partPath and finalPath name the files of a lesson.
func (m *Manager) partPath(id string) string  { return filepath.Join(m.opts.Dir, id+".mp3.part") }
func (m *Manager) finalPath(id string) string { return filepath.Join(m.opts.Dir, id+".mp3") }

// progress records the position of j and, at most every progressEvery
// (or when force is set), tells the UI.
func (m *Manager) progress(j *job, done, total int64, force bool) {
	now := m.opts.Clock.Now()
	m.mu.Lock()
	j.done, j.total = done, total
	if (!force && now.Sub(j.lastEvent) < progressEvery) || (force && done == j.lastDone) {
		m.mu.Unlock()

		return
	}
	j.lastEvent, j.lastDone = now, done
	m.mu.Unlock()
	m.emit(Event{Kind: EventProgress, LessonID: j.lessonID, Done: done, Total: total})
}

// netError says what a failed request or read was: the context's own error
// when it ended, else a lost network (ErrOffline).
func netError(ctx context.Context, err error) error {
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}

	return fmt.Errorf("%w: %w", api.ErrOffline, withoutURL(err))
}

// withoutURL drops the request URL (signed: sig, exp) that net/http puts in
// the text of its errors.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}

	return err
}

// logf writes a line to Options.Logf; the arguments never hold a URL.
func (m *Manager) logf(format string, args ...any) {
	if m.opts.Logf != nil {
		m.opts.Logf(format, args...)
	}
}

// fileSize returns the size of a file, 0 when it is absent.
func fileSize(path string) int64 {
	st, err := os.Stat(path)
	if err != nil || !st.Mode().IsRegular() {
		return 0
	}

	return st.Size()
}

// hashFile feeds the first n bytes of path to h.
func hashFile(h hash.Hash, path string, n int64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	got, err := io.Copy(h, io.LimitReader(f, n))
	if err != nil {
		return err
	}
	if got != n {
		return io.ErrUnexpectedEOF
	}

	return nil
}

// rangeStart returns the first byte of a Content-Range "bytes a-b/c".
func rangeStart(v string) (int64, bool) {
	rest, ok := strings.CutPrefix(v, "bytes ")
	if !ok {
		return 0, false
	}
	first, _, ok := strings.Cut(rest, "-")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(first, 10, 64)

	return n, err == nil && n >= 0
}

// diskFull returns ErrNoSpace (wrapping err) when err is a write that found
// the disk full or over its quota, else nil: the user is told "Không đủ bộ
// nhớ" (Q13) rather than "Tải lỗi".
func diskFull(err error) error {
	if errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return fmt.Errorf("%w: %w", ErrNoSpace, err)
	}

	return nil
}
