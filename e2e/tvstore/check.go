// Adapted from jplayer-tv/e2e/storecheck/check.go at 781bee5: the
// settings record and the channel list give way to this app's state
// documents (prefs, progress-queue, device); -reset is gone (no ResetAll
// here), -from is new.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"time"

	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/progress"
	"dancefitvn-app/internal/store"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ExpectDefaults is the -expect value that asks for the state of a new
// install: default preferences, an empty queue, no device document.
const ExpectDefaults = "defaults"

// DeviceKey is the device document's key, the app's deviceKey.
const DeviceKey = "device"

// ScenarioKey is the entry of store.Prefs.Scenarios a -write stores its id
// under: a string field the preferences keep as they are.
const ScenarioKey = "tvstore"

// WrittenQuality is the quality a -write launch stores, a value of its
// own beside the defaults (QualityAuto).
const WrittenQuality = store.QualityP720

// WrittenPosition is the position_sec of the queue item a -write adds.
const WrittenPosition = 72

// Corruption is what -corrupt writes straight into a slot.
const Corruption = "not json"

// Keys are the documents the check reads and writes, in that order: all
// the TV keeps in its state folder (P7.5d).
var Keys = []string{store.PrefsKey, progress.QueueKey, DeviceKey}

// Config is what one launch does, from its launch arguments.
type Config struct {
	Label   string     // the first argument that is not a flag; "unlabeled" when none
	Expect  string     // "", ExpectDefaults or the id an earlier -write stored
	Write   string     // the id to store, "" for none
	Corrupt store.Slot // the slot to spoil before opening, SlotUnknown for none
	From    store.Slot // the slot every read must come from, SlotUnknown for any
}

// Slots are the two backends of the mirror.
type Slots struct {
	Primary store.Backend
	Copy    store.Backend
}

// read is what one launch found under one key.
type read struct {
	key  string
	info store.LoadInfo
	err  error // the document's own error (NotFound: none stored)
}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// ParseArgs reads the launch arguments: a label, then -expect=<id|defaults>,
// -write=<id>, -corrupt=<primary|copy> and -from=<primary|copy>, in any
// order. A flag it does not know, a second label or an empty value wrap
// core.ErrInvalid.
func ParseArgs(args []string) (Config, error) {
	var c Config
	for _, a := range args {
		name, value, hasValue := strings.Cut(a, "=")
		switch {
		case !strings.HasPrefix(a, "-"):
			if c.Label != "" {
				return Config{}, fmt.Errorf("tvstore: second label %q: %w", a, core.ErrInvalid)
			}
			c.Label = a
		case !hasValue || value == "":
			return Config{}, fmt.Errorf("tvstore: argument %q: %w", a, core.ErrInvalid)
		case name == "-expect":
			c.Expect = value
		case name == "-write":
			c.Write = value
		case name == "-corrupt", name == "-from":
			s, err := parseSlot(name, value)
			if err != nil {
				return Config{}, err
			}
			if name == "-corrupt" {
				c.Corrupt = s
			} else {
				c.From = s
			}
		default:
			return Config{}, fmt.Errorf("tvstore: unknown argument %q: %w", a, core.ErrInvalid)
		}
	}
	if c.Label == "" {
		c.Label = "unlabeled"
	}

	return c, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Check runs one launch over the slots and prints its "tvstore:" lines on
// out: spoil a slot (-corrupt, every key), open a Mirror as the app does,
// print the install id's first 8 characters (sys.InstallID: it lives in
// Library/Caches, so a purge changes it, local/plans/P7.md R-P7-7), read
// the preferences, the progress queue and the device document and print
// what each gave and from which slot, compare with -expect and -from (PASS
// or FAIL), then write -write's values, synchronize, and read each key
// back. It tells whether every expectation held (true when there is
// none).
func Check(c Config, slots Slots, installID func() (string, error), out io.Writer) bool {
	say := func(format string, args ...any) {
		_, _ = fmt.Fprintf(out, "tvstore: "+format+"\n", args...)
	}
	for _, k := range Keys {
		switch c.Corrupt {
		case store.SlotPrimary:
			say("corrupt label=%s slot=primary key=%s err=%s", c.Label, k, errText(slots.Primary.Save(k, []byte(Corruption))))
		case store.SlotCopy:
			say("corrupt label=%s slot=copy key=%s err=%s", c.Label, k, errText(slots.Copy.Save(k, []byte(Corruption))))
		default:
		}
	}
	m := store.NewMirror(slots.Primary, slots.Copy, store.WithSlotError(func(s store.Slot, err error) {
		say("slot error label=%s slot=%s err=%v", c.Label, SlotName(s), err)
	}))
	id, err := installID()
	say("install label=%s id=%s err=%s", c.Label, short(id), errText(err))

	prefs := store.OpenPrefs(m, true)
	reads := []read{{key: store.PrefsKey, info: m.LastLoad(), err: prefsErr(m.LastLoad(), prefs.LoadErr())}}
	q, qerr := progress.NewQueue(m, nopSender{})
	reads = append(reads, read{key: progress.QueueKey, info: m.LastLoad(), err: queueErr(m.LastLoad(), qerr)})
	dev, derr := m.Load(DeviceKey)
	reads = append(reads, read{key: DeviceKey, info: m.LastLoad(), err: derr})
	p := prefs.Get()
	say("read label=%s key=%s auto_next=%t quality=%s scenario=%s err=%s", c.Label, store.PrefsKey,
		p.AutoNext, p.Quality, p.Scenarios[ScenarioKey], errText(reads[0].err))
	say("read label=%s key=%s lessons=%s err=%s", c.Label, progress.QueueKey, strings.Join(lessons(q), ","), errText(reads[1].err))
	say("read label=%s key=%s doc=%s err=%s", c.Label, DeviceKey, dev, errText(derr))
	for _, r := range reads {
		say("slots label=%s key=%s seq=%d from=%s primaryErr=%s copyErr=%s",
			c.Label, r.key, r.info.Seq, SlotName(r.info.From), errText(r.info.PrimaryErr), errText(r.info.CopyErr))
	}

	ok := true
	why := mismatch(c, p, prefs.LoadErr(), q, qerr, dev, derr, reads)
	switch {
	case c.Expect == "" && c.From == store.SlotUnknown:
		say("SKIP %s no -expect", c.Label)
	case why != "":
		ok = false
		say("FAIL %s expect=%s from=%s: %s", c.Label, c.Expect, SlotName(c.From), why)
	default:
		say("PASS %s expect=%s from=%s", c.Label, c.Expect, SlotName(c.From))
	}
	if c.Write != "" {
		write(c, m, prefs, q, say)
	}

	return ok
}

// SlotName names a slot as the log lines do.
func SlotName(s store.Slot) string {
	switch s {
	case store.SlotPrimary:
		return "primary"
	case store.SlotCopy:
		return "copy"
	case store.SlotUnknown:
		return "none"
	default:
		return "slot" + strconv.Itoa(int(s))
	}
}

// ====================================================================
// ========================= Helper Functions =========================
// ====================================================================

// write stores -write's values under every key, synchronizes, and reads
// each key back.
func write(c Config, m *store.Mirror, prefs *store.PrefsStore, q *progress.Queue, say func(string, ...any)) {
	errs := map[string]error{
		store.PrefsKey: prefs.Update(func(p *store.Prefs) {
			p.AutoNext, p.Quality = false, WrittenQuality
			p.Scenarios = map[string]string{ScenarioKey: c.Write}
		}),
		progress.QueueKey: fmt.Errorf("tvstore: no queue: %w", core.ErrInvalid),
	}
	if q != nil {
		errs[progress.QueueKey] = q.Add(progress.Item{
			LessonID: c.Write, PositionSec: WrittenPosition, DurationSec: 600, ClientTime: time.Now().UTC(),
		})
	}
	raw, err := json.Marshal(deviceDoc{LastEmail: Email(c.Write)})
	if err == nil {
		err = m.Save(DeviceKey, raw)
	}
	errs[DeviceKey] = err
	say("sync label=%s err=%s", c.Label, errText(m.Sync()))
	for _, k := range Keys {
		_, lerr := m.Load(k)
		back := m.LastLoad()
		say("wrote label=%s key=%s id=%s err=%s seq=%d from=%s readback=%s primaryErr=%s copyErr=%s",
			c.Label, k, c.Write, errText(errs[k]), back.Seq, SlotName(back.From), errText(lerr),
			errText(back.PrimaryErr), errText(back.CopyErr))
	}
}

// Email is the device document's last_email of a -write=<id>: a reserved
// domain, no real address.
func Email(id string) string { return id + "@example.invalid" }

// deviceDoc is the device document (the app's deviceDoc).
type deviceDoc struct {
	LastEmail string `json:"last_email"`
}

// mismatch tells how what was read differs from what c asks for, "" when
// it does not. ExpectDefaults asks for a new install's state; an id asks
// for what a -write of it stores; -from asks every key's read to come from
// that slot.
func mismatch(c Config, p store.Prefs, perr error, q *progress.Queue, qerr error, dev []byte, derr error, reads []read) string {
	var why []string
	switch c.Expect {
	case "":
	case ExpectDefaults:
		if want := store.DefaultPrefs(true); perr != nil || !reflect.DeepEqual(p, want) {
			why = append(why, fmt.Sprintf("prefs %+v (err %s), want %+v", p, errText(perr), want))
		}
		if qerr != nil || q == nil || q.Len() != 0 {
			why = append(why, "queue lessons="+strings.Join(lessons(q), ",")+" err="+errText(qerr))
		}
		if !errors.Is(derr, core.ErrNotFound) {
			why = append(why, "device="+string(dev)+" err="+errText(derr))
		}
	default:
		if perr != nil || p.AutoNext || p.Quality != WrittenQuality || p.Scenarios[ScenarioKey] != c.Expect {
			why = append(why, fmt.Sprintf("prefs auto_next=%t quality=%s scenario=%s err=%s", p.AutoNext, p.Quality, p.Scenarios[ScenarioKey], errText(perr)))
		}
		if !hasLesson(q, c.Expect) {
			why = append(why, "queue lessons="+strings.Join(lessons(q), ",")+" err="+errText(qerr))
		}
		var d deviceDoc
		if derr != nil || json.Unmarshal(dev, &d) != nil || d.LastEmail != Email(c.Expect) {
			why = append(why, "device="+string(dev)+" err="+errText(derr))
		}
	}
	if c.From != store.SlotUnknown {
		for _, r := range reads {
			if r.info.From != c.From {
				why = append(why, r.key+" from="+SlotName(r.info.From))
			}
		}
	}

	return strings.Join(why, "; ")
}

// prefsErr is the preferences' error: the Mirror's when it found nothing
// (NotFound), else what OpenPrefs kept.
func prefsErr(info store.LoadInfo, loadErr error) error {
	if loadErr == nil && info.From == store.SlotUnknown {
		return fmt.Errorf("tvstore: prefs: %w", core.ErrNotFound)
	}

	return loadErr
}

// queueErr is the queue's error, NotFound when the Mirror found nothing.
func queueErr(info store.LoadInfo, err error) error {
	if err == nil && info.From == store.SlotUnknown {
		return fmt.Errorf("tvstore: queue: %w", core.ErrNotFound)
	}

	return err
}

// lessons lists the lesson ids of q's items; none for a nil q.
func lessons(q *progress.Queue) []string {
	if q == nil {
		return nil
	}
	items := q.Items()
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.LessonID)
	}

	return out
}

// hasLesson reports whether q holds an item of id at WrittenPosition.
func hasLesson(q *progress.Queue, id string) bool {
	if q == nil {
		return false
	}
	for _, it := range q.Items() {
		if it.LessonID == id && it.PositionSec == WrittenPosition {
			return true
		}
	}

	return false
}

// parseSlot reads a -corrupt or -from value.
func parseSlot(flag, v string) (store.Slot, error) {
	switch v {
	case "primary":
		return store.SlotPrimary, nil
	case "copy":
		return store.SlotCopy, nil
	default:
		return store.SlotUnknown, fmt.Errorf("tvstore: %s=%s: want primary or copy: %w", flag, v, core.ErrInvalid)
	}
}

// short is the first 8 characters of an install id, enough to tell two
// apart in a log.
func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	if id == "" {
		return "none"
	}

	return id
}

// errText formats an error for a log line: "nil" for none, and the
// sentinel it wraps first, so that a reader sees notfound or invalid at a
// glance.
func errText(err error) string {
	switch {
	case err == nil:
		return "nil"
	case errors.Is(err, core.ErrNotFound):
		return "notfound(" + err.Error() + ")"
	case errors.Is(err, core.ErrInvalid):
		return "invalid(" + err.Error() + ")"
	default:
		return err.Error()
	}
}

// nopSender is the queue's sender: never called, nothing flushes here.
type nopSender struct{}

func (nopSender) SendProgressBatch(context.Context, model.BatchRequest) (model.BatchResult, error) {
	return model.BatchResult{}, nil
}
