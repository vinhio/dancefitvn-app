package store

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gdeskdev/desk/foundation/core"

	"dancefitvn-app/internal/api"
	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// OverlayKey is the Backend key of the overlay: <data>/mock-overlay.json
// (Q-P1-13).
const OverlayKey = "mock-overlay"

// overlayVersion is the format version of the record.
const overlayVersion = 1

// Overlay is an api.Transport that sits in front
// of the mock one. The mock keeps no state (a PUT does not change what a
// later GET reads, docs/plan/05-data.md §4), so the Overlay keeps what the
// user changed and the demo reads as a live server: it records the changes of me.update, me.avatar,
// me.coach.update, lessons.progress, progress.batch, devices.delete and tv.pair.confirm once
// the mock answers 2xx, and applies them to the answers of me.get,
// me.coach.get, courses.show, me.home, me.courses and devices.list.
//
// It is persisted under OverlayKey, survives sign-out, and is emptied by
// Reset (the dev menu). Only the app in mock mode builds one. It is safe for
// concurrent use; saving is best effort (SaveErr tells the last failure).
//
// An overlaid GET carries no ETag and its If-None-Match is dropped: the
// body changes with the overlay, the mock's tag does not.
type Overlay struct {
	inner api.Transport
	b     Backend

	mu      sync.Mutex
	st      overlayState
	seen    map[string]seenLesson // lessons met in courses.show, by id; memory only
	seq     int64
	saveErr error
	relay   *pairRelay // nil: the pair routes answer as the mock's files
}

var _ api.Transport = (*Overlay)(nil)

// overlayState is the persisted record.
type overlayState struct {
	Version     int                    `json:"version"`
	Seq         int64                  `json:"seq,omitzero"`
	User        map[string]string      `json:"user,omitzero"`
	Coach       *coachState            `json:"coach,omitzero"`
	Lessons     map[string]lessonState `json:"lessons,omitzero"`
	BaseWatched map[string]bool        `json:"base_watched,omitzero"` // lessons the mock itself says are watched
	Continue    *continueState         `json:"continue,omitzero"`
	Removed     []string               `json:"removed_devices,omitzero"`
}

type lessonState struct {
	CourseID    string    `json:"course_id,omitzero"`
	PositionSec int       `json:"position_sec"`
	DurationSec int       `json:"duration_sec"`
	Watched     bool      `json:"watched"`
	At          time.Time `json:"at,omitzero"` // client_time of the newest save
	Seq         int64     `json:"seq"`         // order of the saves, for "the lesson to go on with"
}

type coachState struct {
	Bio       string        `json:"bio"`
	Places    []string      `json:"places"`
	Socials   []coachSocial `json:"socials"`
	UpdatedAt string        `json:"updated_at,omitzero"`
}

type coachSocial struct {
	Platform string `json:"platform"`
	URL      string `json:"url"`
}

type continueState struct {
	Course       jsontext.Value `json:"course"`
	Lesson       jsontext.Value `json:"lesson"`
	PositionSec  int            `json:"position_sec"`
	RemainingSec int            `json:"remaining_sec"`
}

// seenLesson is what courses.show told about a lesson.
type seenLesson struct {
	course map[string]any // id, name, tagline, cover_url, lesson_count
	lesson map[string]any // id, no, title, tag, duration_sec
}

// socialPlatforms are the platforms a link can be told apart by its host.
var socialPlatforms = []string{"facebook", "instagram", "youtube", "tiktok", "zalo", "threads"}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// NewOverlay returns an Overlay in front of inner that keeps its record on
// b. A record that is missing or unreadable starts empty.
func NewOverlay(inner api.Transport, b Backend) *Overlay {
	o := &Overlay{inner: inner, b: b, seen: map[string]seenLesson{}}
	o.st = overlayState{Version: overlayVersion}
	if raw, err := b.Load(OverlayKey); err == nil {
		var st overlayState
		if json.Unmarshal(raw, &st) == nil && st.Version == overlayVersion {
			o.st = st
			o.seq = st.Seq
		}
	}

	return o
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Do implements api.Transport.
func (o *Overlay) Do(ctx context.Context, req *api.Request) (*api.Response, error) {
	if o.relayOn(req.RouteID) {
		return o.relayDo(ctx, req)
	}
	if isOverlaid(req.RouteID) {
		return o.get(ctx, req)
	}
	resp, err := o.inner.Do(ctx, req)
	if err != nil || resp == nil || resp.Status < 200 || resp.Status > 299 {
		return resp, err
	}
	o.record(req, resp)

	return resp, nil
}

// Reset forgets every change.
func (o *Overlay) Reset() error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.st = overlayState{Version: overlayVersion}
	o.seq = 0
	o.seen = map[string]seenLesson{}

	return o.saveLocked()
}

// SaveErr returns the error of the last failed save; nil when it worked.
func (o *Overlay) SaveErr() error {
	o.mu.Lock()
	defer o.mu.Unlock()

	return o.saveErr
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// isOverlaid reports whether the answer of a route is changed by the record.
func isOverlaid(id string) bool {
	switch id {
	case "me.get", "me.coach.get", "courses.show", "me.home", "me.courses", "devices.list":
		return true
	default:
		return false
	}
}

// get runs an overlaid GET: the mock answers its own body, the record is
// applied to it.
func (o *Overlay) get(ctx context.Context, req *api.Request) (*api.Response, error) {
	r := *req
	r.Header = req.Header.Clone()
	r.Header.Del("If-None-Match")
	resp, err := o.inner.Do(ctx, &r)
	if err != nil || resp == nil || resp.Status != http.StatusOK {
		return resp, err
	}
	var doc any
	if err := json.Unmarshal(resp.Body, &doc); err != nil {
		return resp, nil //nolint:nilerr // not ours to judge: the client reports an unreadable answer
	}
	root, ok := doc.(map[string]any)
	if !ok {
		return resp, nil
	}
	o.mu.Lock()
	o.apply(req.RouteID, root)
	o.mu.Unlock()
	body, err := json.Marshal(root)
	if err != nil {
		return resp, nil //nolint:nilerr // see above
	}
	h := resp.Header.Clone()
	h.Del("ETag")

	return &api.Response{Status: resp.Status, Header: h, Body: body}, nil
}

// apply changes the answer root of route id; o.mu is held.
func (o *Overlay) apply(id string, root map[string]any) {
	switch id {
	case "me.get":
		o.applyUser(asMap(root["user"]))
		if d := asMap(root["devices"]); d != nil && len(o.st.Removed) > 0 {
			if n, ok := d["active"].(float64); ok {
				d["active"] = max(0, n-float64(len(o.st.Removed)))
			}
		}
	case "me.coach.get":
		o.applyCoach(root)
	case "courses.show":
		o.applyCourse(root)
	case "me.home":
		o.applyHome(root)
	case "me.courses":
		for _, it := range asSlice(root["items"]) {
			o.applyCount(asMap(it))
		}
	case "devices.list":
		if items, ok := root["items"].([]any); ok {
			root["items"] = slices.DeleteFunc(items, func(it any) bool {
				return slices.Contains(o.st.Removed, asString(asMap(it)["id"]))
			})
		}
	default:
	}
}

func (o *Overlay) applyUser(u map[string]any) {
	if u == nil {
		return
	}
	for k, v := range o.st.User {
		u[k] = v
	}
}

func (o *Overlay) applyCoach(root map[string]any) {
	c := o.st.Coach
	if c == nil {
		return
	}
	root["bio"] = c.Bio
	places := make([]any, 0, len(c.Places))
	for _, p := range c.Places {
		places = append(places, p)
	}
	root["places"] = places
	socials := make([]any, 0, len(c.Socials))
	for _, s := range c.Socials {
		socials = append(socials, map[string]any{"platform": s.Platform, "url": s.URL})
	}
	root["socials"] = socials
	if c.UpdatedAt != "" {
		root["updated_at"] = c.UpdatedAt
	}
}

// applyCourse puts the saved lessons into a course and learns the lessons
// and the course from the mock's own answer.
func (o *Overlay) applyCourse(root map[string]any) {
	courseID := asString(root["id"])
	lessons := asSlice(root["lessons"])
	ref := map[string]any{}
	for _, k := range []string{"id", "name", "tagline", "cover_url", "lesson_count"} {
		if v, ok := root[k]; ok {
			ref[k] = v
		}
	}
	dirty := false
	for _, it := range lessons {
		l := asMap(it)
		if l == nil {
			continue
		}
		lid := asString(l["id"])
		lref := map[string]any{}
		for _, k := range []string{"id", "no", "title", "tag", "duration_sec"} {
			if v, ok := l[k]; ok {
				lref[k] = v
			}
		}
		o.seen[lid] = seenLesson{course: ref, lesson: lref}
		if w, _ := asMap(l["progress"])["watched"].(bool); w && !o.st.BaseWatched[lid] {
			if o.st.BaseWatched == nil {
				o.st.BaseWatched = map[string]bool{}
			}
			o.st.BaseWatched[lid] = true
			dirty = true
		}
		if ls, ok := o.st.Lessons[lid]; ok {
			if ls.CourseID == "" {
				ls.CourseID = courseID
				o.st.Lessons[lid] = ls
				dirty = true
			}
			p := asMap(l["progress"])
			if p == nil {
				p = map[string]any{}
				l["progress"] = p
			}
			p["position_sec"] = ls.PositionSec
			p["watched"] = ls.Watched || o.st.BaseWatched[lid]
		}
	}
	if dirty {
		_ = o.saveLocked()
	}

	watched := 0
	for _, it := range lessons {
		if w, _ := asMap(asMap(it)["progress"])["watched"].(bool); w {
			watched++
		}
	}
	p := asMap(root["progress"])
	if p == nil {
		p = map[string]any{}
		root["progress"] = p
	}
	p["watched_count"] = watched

	o.applyResume(root, courseID, lessons)
}

// applyResume points the course's resume at the lesson saved last when it is
// not finished, else at the first lesson not watched, when the mock's own
// resume lesson has been watched since.
func (o *Overlay) applyResume(root map[string]any, courseID string, lessons []any) {
	var last lessonState
	lastID := ""
	for id, ls := range o.st.Lessons {
		if ls.CourseID == courseID && ls.Seq > last.Seq {
			last, lastID = ls, id
		}
	}
	if lastID == "" {
		return
	}
	if !last.Watched {
		root["resume"] = map[string]any{"lesson_id": lastID, "position_sec": last.PositionSec}

		return
	}
	cur := asString(asMap(root["resume"])["lesson_id"])
	if cur != "" && !o.st.Lessons[cur].Watched {
		return
	}
	for _, it := range lessons {
		l := asMap(it)
		if w, _ := asMap(l["progress"])["watched"].(bool); !w {
			pos := asMap(l["progress"])["position_sec"]
			if pos == nil {
				pos = float64(0)
			}
			root["resume"] = map[string]any{"lesson_id": asString(l["id"]), "position_sec": pos}

			return
		}
	}
}

func (o *Overlay) applyHome(root map[string]any) {
	if fn, ok := o.st.User["first_name"]; ok {
		root["greeting_name"] = fn
	}
	counts := map[string]float64{}
	for _, it := range asSlice(root["my_courses"]) {
		c := asMap(it)
		o.applyCount(c)
		if c != nil {
			counts[asString(c["id"])], _ = asMap(c["progress"])["watched_count"].(float64)
		}
	}
	for _, u := range asSlice(root["updates"]) {
		o.applyCount(asMap(asMap(u)["course"]))
	}
	if c := o.st.Continue; c != nil {
		var course, lesson any
		if json.Unmarshal(c.Course, &course) == nil && json.Unmarshal(c.Lesson, &lesson) == nil {
			root["continue"] = map[string]any{
				"course": course, "lesson": lesson,
				"position_sec": c.PositionSec, "remaining_sec": c.RemainingSec,
			}
		}
	}
	cur := asMap(asMap(root["continue"])["course"])
	stats := asMap(root["stats"])
	if cur == nil || stats == nil {
		return
	}
	if n, ok := cur["lesson_count"].(float64); ok {
		stats["lessons_in_current_course"] = n
	}
	if n, ok := counts[asString(cur["id"])]; ok {
		stats["watched_in_current_course"] = n
	}
}

// applyCount adds the lessons watched here, and not in the mock's own data,
// to the progress of a course object.
func (o *Overlay) applyCount(course map[string]any) {
	if course == nil {
		return
	}
	extra := 0
	cid := asString(course["id"])
	for id, ls := range o.st.Lessons {
		if ls.CourseID == cid && ls.Watched && !o.st.BaseWatched[id] {
			extra++
		}
	}
	if extra == 0 {
		return
	}
	p := asMap(course["progress"])
	if p == nil {
		p = map[string]any{}
		course["progress"] = p
	}
	n, _ := p["watched_count"].(float64)
	p["watched_count"] = n + float64(extra)
}

// record keeps the change a 2xx answer confirms.
func (o *Overlay) record(req *api.Request, resp *api.Response) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.recordLocked(req, resp)
}

// recordLocked is record with o.mu held.
func (o *Overlay) recordLocked(req *api.Request, resp *api.Response) {
	changed := false
	switch req.RouteID {
	case "me.update":
		changed = o.recordMe(req, resp)
	case "me.avatar":
		var a model.Avatar
		if json.Unmarshal(resp.Body, &a) == nil && a.AvatarURL != "" {
			o.setUser("avatar_url", a.AvatarURL)
			changed = true
		}
	case "me.coach.update":
		changed = o.recordCoach(req, resp)
	case "lessons.progress":
		changed = o.recordProgress(req, resp)
	case "progress.batch":
		changed = o.recordBatch(req, resp)
	case "auth.login":
		// a sign-in at the limit that replaced a device: it is revoked (K31)
		var in model.LoginRequest
		if json.Unmarshal(req.Body, &in) == nil && in.ReplaceDeviceID != "" && !slices.Contains(o.st.Removed, in.ReplaceDeviceID) {
			o.st.Removed = append(o.st.Removed, in.ReplaceDeviceID)
			changed = true
		}
	case "devices.delete":
		if id := lastSegment(req.Path); id != "" && !slices.Contains(o.st.Removed, id) {
			o.st.Removed = append(o.st.Removed, id)
			changed = true
		}
	case "tv.logout":
		// With the relay, the TV that signed out leaves the device list.
		if o.relay != nil && o.relay.device != "" && !slices.Contains(o.st.Removed, o.relay.device) {
			o.st.Removed = append(o.st.Removed, o.relay.device)
			changed = true
		}
	case "tv.pair.confirm":
		// A TV paired again shows in devices.list and me.get devices.active
		// once more, also after devices.delete took it away.
		var pc model.PairConfirm
		if json.Unmarshal(resp.Body, &pc) == nil && pc.Device.ID != "" {
			if i := slices.Index(o.st.Removed, pc.Device.ID); i >= 0 {
				o.st.Removed = slices.Delete(o.st.Removed, i, i+1)
				changed = true
			}
		}
	default:
	}
	if changed {
		_ = o.saveLocked()
	}
}

func (o *Overlay) setUser(k, v string) {
	if o.st.User == nil {
		o.st.User = map[string]string{}
	}
	o.st.User[k] = v
}

func (o *Overlay) recordMe(req *api.Request, resp *api.Response) bool {
	var in model.MeUpdateRequest
	if json.Unmarshal(req.Body, &in) != nil {
		return false
	}
	var out model.MeUpdate
	pending := json.Unmarshal(resp.Body, &out) == nil && out.EmailChangePending != nil
	o.setUser("first_name", in.FirstName)
	o.setUser("last_name", in.LastName)
	o.setUser("full_name", strings.TrimSpace(in.LastName+" "+in.FirstName))
	o.setUser("phone", in.Phone)
	if !pending {
		o.setUser("email", in.Email)
	}

	return true
}

func (o *Overlay) recordCoach(req *api.Request, resp *api.Response) bool {
	var in model.CoachUpdateRequest
	if json.Unmarshal(req.Body, &in) != nil {
		return false
	}
	var out model.CoachProfile
	_ = json.Unmarshal(resp.Body, &out)
	known := map[string]string{}
	for _, s := range out.Socials {
		known[s.URL] = s.Platform
	}
	c := &coachState{Bio: in.Bio, Places: slices.Clone(in.Places), Socials: []coachSocial{}}
	if c.Places == nil {
		c.Places = []string{}
	}
	for _, s := range in.Socials {
		p := platformOf(s.URL)
		if p == "" {
			p = known[s.URL]
		}
		c.Socials = append(c.Socials, coachSocial{Platform: p, URL: s.URL})
	}
	if !out.UpdatedAt.IsZero() {
		c.UpdatedAt = out.UpdatedAt.Format(time.RFC3339)
	}
	o.st.Coach = c

	return true
}

func (o *Overlay) recordProgress(req *api.Request, resp *api.Response) bool {
	id := segmentAfter(req.Path, "lessons")
	var in model.ProgressRequest
	if id == "" || json.Unmarshal(req.Body, &in) != nil {
		return false
	}
	var out model.ProgressResult
	_ = json.Unmarshal(resp.Body, &out)
	course := out.CourseID
	if s, ok := o.seen[id]; ok && course == "" {
		course = asString(s.course["id"])
	}
	prev := o.st.Lessons[id]
	watched := prev.Watched || out.Watched || watchedAt(in.PositionSec, in.DurationSec)
	o.put(id, lessonState{
		CourseID: course, PositionSec: in.PositionSec, DurationSec: in.DurationSec,
		Watched: watched, At: in.ClientTime,
	})
	o.setContinue(id, course, in, out, watched)

	return true
}

// setContinue makes the lesson the user goes on with: the one just saved,
// or the one that follows when it was watched.
func (o *Overlay) setContinue(id, course string, in model.ProgressRequest, out model.ProgressResult, watched bool) {
	seen, haveSeen := o.seen[id]
	courseRef := seen.course
	if courseRef == nil {
		courseRef = map[string]any{"id": course}
	}
	switch {
	case watched && out.NextLesson != nil:
		raw, err := json.Marshal(out.NextLesson)
		cr, err2 := json.Marshal(courseRef)
		if err != nil || err2 != nil {
			return
		}
		o.st.Continue = &continueState{Course: cr, Lesson: raw, RemainingSec: out.NextLesson.DurationSec}
	case watched:
	case haveSeen:
		cr, err := json.Marshal(courseRef)
		lr, err2 := json.Marshal(seen.lesson)
		if err != nil || err2 != nil {
			return
		}
		o.st.Continue = &continueState{
			Course: cr, Lesson: lr,
			PositionSec: in.PositionSec, RemainingSec: max(0, in.DurationSec-in.PositionSec),
		}
	case o.st.Continue != nil:
		var l struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(o.st.Continue.Lesson, &l) == nil && l.ID == id {
			o.st.Continue.PositionSec = in.PositionSec
			o.st.Continue.RemainingSec = max(0, in.DurationSec-in.PositionSec)
		}
	default:
	}
}

func (o *Overlay) recordBatch(req *api.Request, resp *api.Response) bool {
	var in model.BatchRequest
	if json.Unmarshal(req.Body, &in) != nil {
		return false
	}
	var out model.BatchResult
	_ = json.Unmarshal(resp.Body, &out)
	rejected := map[string]bool{}
	for _, r := range out.Rejected {
		var it struct {
			LessonID string `json:"lesson_id"`
		}
		if json.Unmarshal(r, &it) == nil {
			rejected[it.LessonID] = true
		}
	}
	changed := false
	for _, it := range in.Items {
		prev, had := o.st.Lessons[it.LessonID]
		if rejected[it.LessonID] || (had && !it.ClientTime.After(prev.At)) {
			continue
		}
		course := prev.CourseID
		if s, ok := o.seen[it.LessonID]; ok && course == "" {
			course = asString(s.course["id"])
		}
		o.put(it.LessonID, lessonState{
			CourseID: course, PositionSec: it.PositionSec, DurationSec: it.DurationSec,
			Watched: prev.Watched || watchedAt(it.PositionSec, it.DurationSec), At: it.ClientTime,
		})
		changed = true
	}

	return changed
}

// put stores the state of a lesson as the newest save.
func (o *Overlay) put(id string, ls lessonState) {
	o.seq++
	ls.Seq = o.seq
	if o.st.Lessons == nil {
		o.st.Lessons = map[string]lessonState{}
	}
	o.st.Lessons[id] = ls
}

// saveLocked writes the record; o.mu is held.
func (o *Overlay) saveLocked() error {
	o.st.Seq = o.seq
	raw, err := json.Marshal(o.st)
	if err == nil {
		err = o.b.Save(OverlayKey, raw)
	}
	o.saveErr = err

	return err
}

// watchedAt is the prototype's 90 % rule (03-behavior-spec.md): 198 of 220
// is watched, 197 is not.
func watchedAt(pos, dur int) bool { return dur > 0 && pos*10 >= dur*9 }

// platformOf names the platform of a link by its host; "" when it is none of
// the allowed ones.
func platformOf(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "youtu.be" {
		return "youtube"
	}
	for _, p := range socialPlatforms {
		if strings.Contains(host, p) {
			return p
		}
	}

	return ""
}

// segmentAfter returns the path segment that follows name.
func segmentAfter(p, name string) string {
	parts := strings.Split(strings.Trim(p, "/"), "/")
	for i, s := range parts[:max(0, len(parts)-1)] {
		if s == name {
			return parts[i+1]
		}
	}

	return ""
}

func lastSegment(p string) string {
	p = strings.TrimRight(p, "/")

	return p[strings.LastIndex(p, "/")+1:]
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)

	return m
}

func asSlice(v any) []any {
	s, _ := v.([]any)

	return s
}

func asString(v any) string {
	s, _ := v.(string)

	return s
}

// --- Pair relay (P7.2b) ---

// Codes of the pair relay: the alphabet leaves out 0, 1, I, L and O, which
// are read wrong off a TV.
const (
	pairAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	pairCodeLen  = 6
	pairTTL      = 300 * time.Second
	pairMaxLive  = 256
)

// Fallback bodies (code only, ASCII message) when PairRelayOptions leaves
// the mock's own out; callers pass the bodies of the mock's error files.
const (
	pairExpiredBody = `{"error":{"code":"pair_code_expired","message":"pair code expired"}}`
	pairInvalidBody = `{"error":{"code":"pair_code_invalid","message":"pair code invalid"}}`
)

// PairRelayOptions configures EnablePairRelay.
type PairRelayOptions struct {
	// Confirmed is the body of tv/pair-poll.confirmed.json: the session a
	// confirmed poll answers with. Required.
	Confirmed []byte
	// Expired and Invalid are the bodies of errors/pair_code_expired.json
	// and errors/pair_code_invalid.json; nil takes the built-in copy.
	Expired, Invalid []byte
	// Now is the clock; nil is time.Now.
	Now func() time.Time
	// Rand is the source of codes and secrets; nil is crypto/rand.
	Rand io.Reader
	// TTL is the life of a code; zero is 300 s.
	TTL time.Duration
}

// pairRelay links the TV's start and poll to the phone's confirm: the mock
// alone answers them from files and so cannot (the TV and the phone are
// two processes, 05-data.md). It lives in memory only: a restart leaves
// nothing pending.
type pairRelay struct {
	opts    PairRelayOptions
	entries map[string]*pairEntry // by normalised code
	device  string                // id of the TV device last confirmed
}

type pairEntry struct {
	secret    string
	expires   time.Time
	confirmed bool
	device    jsontext.Value // device of the confirm answer; may be empty
}

// EnablePairRelay makes the Overlay answer tv.pair.start, tv.pair.poll and
// tv.pair.confirm from one shared state: start gives a random code and
// poll secret; poll answers pending, then once the confirmed session after
// a phone confirmed the code; a code that is unknown, expired or polled
// with another secret is 410 pair_code_expired; confirming a code that is
// unknown, expired or already confirmed is 404 pair_code_invalid. The
// mock still decides the other outcomes of start and confirm (its
// scenarios: rate_limited, device_limit, 401), and tv.logout takes the TV
// out of the device list. Without it (the app in its own process) the
// three routes answer as the mock's files do.
func (o *Overlay) EnablePairRelay(opts PairRelayOptions) error {
	var probe map[string]any
	if err := json.Unmarshal(opts.Confirmed, &probe); err != nil || probe["status"] != "confirmed" {
		return fmt.Errorf("store: pair relay: confirmed body is not a confirmed session: %w", core.ErrInvalid)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.Rand == nil {
		opts.Rand = rand.Reader
	}
	if opts.TTL <= 0 {
		opts.TTL = pairTTL
	}
	if len(opts.Expired) == 0 {
		opts.Expired = []byte(pairExpiredBody)
	}
	if len(opts.Invalid) == 0 {
		opts.Invalid = []byte(pairInvalidBody)
	}
	o.mu.Lock()
	o.relay = &pairRelay{opts: opts, entries: map[string]*pairEntry{}}
	o.mu.Unlock()

	return nil
}

// relayOn reports whether the relay answers the route.
func (o *Overlay) relayOn(id string) bool {
	switch id {
	case "tv.pair.start", "tv.pair.poll", "tv.pair.confirm":
		o.mu.Lock()
		defer o.mu.Unlock()

		return o.relay != nil
	default:
		return false
	}
}

// relayDo answers a pair route.
func (o *Overlay) relayDo(ctx context.Context, req *api.Request) (*api.Response, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if req.RouteID == "tv.pair.poll" {
		o.mu.Lock()
		defer o.mu.Unlock()

		return o.relay.poll(req), nil
	}
	resp, err := o.inner.Do(ctx, req) // the mock's say: 401, 429, 409 ...
	if err != nil || resp == nil || resp.Status < 200 || resp.Status > 299 {
		return resp, err
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	var out *api.Response
	if req.RouteID == "tv.pair.start" {
		out = o.relay.start(resp)
	} else {
		out = o.relay.confirm(req, resp)
	}
	if out.Status >= 200 && out.Status <= 299 && req.RouteID == "tv.pair.confirm" {
		o.recordLocked(req, out)
	}

	return out, nil
}

func (r *pairRelay) prune() {
	now := r.opts.Now()
	for c, e := range r.entries {
		if !now.Before(e.expires) {
			delete(r.entries, c)
		}
	}
}

func (r *pairRelay) random(alphabet string, n int) string {
	out := make([]byte, 0, n)
	buf := make([]byte, 1)
	limit := 256 - 256%len(alphabet)
	for len(out) < n {
		if _, err := io.ReadFull(r.opts.Rand, buf); err != nil {
			buf[0] = byte(r.opts.Now().UnixNano()) // a dead source: the mock still answers
		}
		if int(buf[0]) < limit {
			out = append(out, alphabet[int(buf[0])%len(alphabet)])
		}
	}

	return string(out)
}

func relayJSON(status int, body []byte) *api.Response {
	h := http.Header{}
	h.Set("Content-Type", "application/json; charset=utf-8")

	return &api.Response{Status: status, Header: h, Body: body}
}

func (r *pairRelay) start(resp *api.Response) *api.Response {
	var doc map[string]any
	if json.Unmarshal(resp.Body, &doc) != nil {
		return resp
	}
	r.prune()
	if len(r.entries) >= pairMaxLive { // a runaway client: the oldest goes
		oldest := ""
		for c, e := range r.entries {
			if oldest == "" || e.expires.Before(r.entries[oldest].expires) {
				oldest = c
			}
		}
		delete(r.entries, oldest)
	}
	var code string
	for {
		code = r.random(pairAlphabet, pairCodeLen)
		if r.entries[code] == nil {
			break
		}
	}
	e := &pairEntry{secret: "ps_" + r.random("0123456789abcdef", 16), expires: r.opts.Now().Add(r.opts.TTL)}
	r.entries[code] = e
	old, _ := doc["code"].(string)
	doc["code"] = code
	doc["display_code"] = code[:3] + "-" + code[3:]
	doc["poll_secret"] = e.secret
	doc["expires_in"] = int(r.opts.TTL / time.Second)
	doc["expires_at"] = e.expires.Format(time.RFC3339)
	if q, ok := doc["qr_url"].(string); ok && old != "" {
		doc["qr_url"] = strings.ReplaceAll(q, old, code)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		return resp
	}

	return relayJSON(resp.Status, body)
}

func (r *pairRelay) poll(req *api.Request) *api.Response {
	var in model.PairPollRequest
	_ = json.Unmarshal(req.Body, &in)
	r.prune()
	code := normPairCode(in.Code)
	e := r.entries[code]
	if e == nil || subtle.ConstantTimeCompare([]byte(e.secret), []byte(in.PollSecret)) != 1 {
		return relayJSON(http.StatusGone, r.opts.Expired)
	}
	if !e.confirmed {
		left := max(int(e.expires.Sub(r.opts.Now())/time.Second), 0)
		body, _ := json.Marshal(map[string]any{"status": "pending", "expires_in": left})

		return relayJSON(http.StatusOK, body)
	}
	delete(r.entries, code) // the session goes out once
	var doc map[string]any
	_ = json.Unmarshal(r.opts.Confirmed, &doc)
	if len(e.device) > 0 {
		var dev any
		if json.Unmarshal(e.device, &dev) == nil {
			doc["device"] = dev
		}
	}
	body, _ := json.Marshal(doc)

	return relayJSON(http.StatusOK, body)
}

func (r *pairRelay) confirm(req *api.Request, resp *api.Response) *api.Response {
	var in model.PairConfirmRequest
	_ = json.Unmarshal(req.Body, &in)
	r.prune()
	e := r.entries[normPairCode(in.Code)]
	if e == nil || e.confirmed {
		return relayJSON(http.StatusNotFound, r.opts.Invalid)
	}
	e.confirmed = true
	var pc struct {
		Device jsontext.Value `json:"device"`
	}
	if json.Unmarshal(resp.Body, &pc) == nil {
		e.device = pc.Device
	}
	var ref model.PairConfirm
	if json.Unmarshal(resp.Body, &ref) == nil && ref.Device.ID != "" {
		r.device = ref.Device.ID
	}

	return resp
}

// normPairCode upper-cases a code and drops dashes and spaces.
func normPairCode(s string) string {
	s = strings.ToUpper(s)

	return strings.NewReplacer("-", "", " ", "").Replace(s)
}
