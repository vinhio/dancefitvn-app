//go:build !tvos

package kitdemo

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"dancefitvn-app/internal/model"

	"github.com/gdeskdev/desk/foundation/core"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// The mock files the pages are drawn from (docs/plan/05-data.md): Set 35 in
// full, the home, the user and the list of the user's courses.
const (
	fileSet35   = "courses/set-35.json"
	fileHome    = "me/home.json"
	fileMe      = "me/me.json"
	fileCourses = "me/courses.json"
)

// Data is what the pages show: the mock API's answers decoded into the model's
// types (D7: data comes from the API, never from the prototype's samples) and
// the texts of locales/vi.json.
type Data struct {
	// Set35 is Set 35 with its ten lessons.
	Set35 model.Course
	// Home is the answer of GET /me/home.
	Home model.Home
	// Me is the answer of GET /me.
	Me model.Me
	// Courses is the answer of GET /me/courses.
	Courses model.CoursePage

	texts map[string]string
}

// The demo state the prototype starts with and the mock does not hold: which
// lessons of Set 35 have their audio on the device and how far the one under
// way is (`S.dl` of index.html: lessons 1, 2, 3, 4 and 6 done, 7 at 64 %).
const (
	// downloadingNo is the lesson being downloaded.
	downloadingNo = 7
	// downloadingProgress is how far it is.
	downloadingProgress float32 = .64
)

// downloadedNos are the lessons of Set 35 with their audio on the device.
var downloadedNos = [...]int{1, 2, 3, 4, 6}

// ====================================================================
// ============================= Creation =============================
// ====================================================================

// Load decodes the mock files from fsys (the mock API's root: routes.json,
// courses/, me/ ...) and keeps texts, the locale's strings.
func Load(fsys fs.FS, texts map[string]string) (*Data, error) {
	if fsys == nil {
		return nil, fmt.Errorf("kitdemo: no mock API folder: %w", core.ErrInvalid)
	}
	d := &Data{texts: texts}
	for _, f := range []struct {
		name string
		into any
	}{
		{fileSet35, &d.Set35}, {fileHome, &d.Home}, {fileMe, &d.Me}, {fileCourses, &d.Courses},
	} {
		raw, err := fs.ReadFile(fsys, f.name)
		if err != nil {
			return nil, fmt.Errorf("kitdemo: read %s: %w", f.name, err)
		}
		if err := json.Unmarshal(raw, f.into); err != nil {
			return nil, fmt.Errorf("kitdemo: decode %s: %w: %w", f.name, core.ErrInvalid, err)
		}
	}

	return d, nil
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// T returns the text of key; a key the locale lacks shows as the key itself,
// where a missing string cannot be overlooked.
func (d *Data) T(key string) string {
	if s, ok := d.texts[key]; ok {
		return s
	}

	return key
}

// Downloaded tells whether lesson no of Set 35 has its audio on the device.
func Downloaded(no int) bool {
	for _, n := range downloadedNos {
		if n == no {
			return true
		}
	}

	return false
}

// ProtoID returns the id the prototype's layout.json keys use for a course:
// the API's "set-35" is its "set35", "hot-extra-2" is "hotextra2".
func ProtoID(id string) string { return strings.ReplaceAll(id, "-", "") }
