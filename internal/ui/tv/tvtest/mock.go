//go:build tvos

package tvtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"dancefitvn-app/internal/model"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// mockDir is the mock API's files, from a test's package directory under
// internal/ui/tv/<pkg>.
var mockDir = filepath.Join("..", "..", "..", "..", "mock", "dancefitvn-mock-api")

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Mock decodes the mock API's file rel ("me/home.json") into v: the answers the
// screens' tests feed the fake services, so that what they show is what the
// contract says. A file that is not there fails the test.
func Mock(t testing.TB, rel string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(mockDir, rel))
	if err != nil {
		t.Fatalf("mock %s: %v", rel, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("mock %s: %v", rel, err)
	}
}

// NewSignedServices returns the fake services of a signed-in user with the mock
// answers of the home (me/home.json), the courses (me/courses.json), the course
// set-35 and no player: what TV04, TV05 and TV06 show in the prototype's states.
func NewSignedServices(t testing.TB) *Services {
	t.Helper()
	s := NewServices()
	Mock(t, "me/home.json", &s.HomeData)
	Mock(t, "me/courses.json", &s.CoursesData)
	Mock(t, "courses/set-35.json", &s.CourseData)
	s.User = model.User{FirstName: "Vy"}

	return s
}
