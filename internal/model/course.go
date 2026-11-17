package model

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Category is the kind of a course.
type Category int

// The categories of a course. CategoryUnknown is any text the app does not
// know.
const (
	CategoryUnknown Category = iota
	CategoryRegular
	CategorySpecial
	CategoryIntro
)

// Course is a course, in a list (no Lessons) or in full (courses/<id>.json).
// A null tagline is "", a null total_duration_sec is 0 (unknown).
type Course struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Tagline          string    `json:"tagline"`
	Category         Category  `json:"category,omitzero"`
	CategoryLabel    string    `json:"category_label"`
	LessonCount      int       `json:"lesson_count"`
	TotalDurationSec int       `json:"total_duration_sec,omitzero"`
	CoverURL         string    `json:"cover_url"`
	IsNew            bool      `json:"is_new,omitzero"`
	Enrolled         bool      `json:"enrolled"`
	Progress         Progress  `json:"progress,omitzero"`
	UpdatedAt        time.Time `json:"updated_at,omitzero"`
	Description      string    `json:"description,omitzero"`
	Lessons          []Lesson  `json:"lessons,omitzero"`
	Resume           *Resume   `json:"resume,omitzero"`
	EnrollURL        string    `json:"enroll_url,omitzero"`
}

// Progress is the progress of the user in a course.
type Progress struct {
	WatchedCount int `json:"watched_count"`
}

// Resume is where the user stopped in a course; nil when nowhere.
type Resume struct {
	LessonID    string `json:"lesson_id"`
	PositionSec int    `json:"position_sec"`
}

// CourseRef is the short form of a course in Home.Continue.
type CourseRef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Tagline     string `json:"tagline"`
	CoverURL    string `json:"cover_url"`
	LessonCount int    `json:"lesson_count"`
}

// CoursePage is the answer of GET /me/courses.
type CoursePage struct {
	Items  []Course `json:"items"`
	Counts struct {
		All     int `json:"all"`
		Regular int `json:"regular"`
		Special int `json:"special"`
		Intro   int `json:"intro"`
	} `json:"counts"`
	Page struct {
		NextCursor *string `json:"next_cursor"`
		Limit      int     `json:"limit"`
		Total      int     `json:"total"`
	} `json:"page"`
}

// Home is the answer of GET /me/home. Continue is nil for a new user.
type Home struct {
	GreetingName string    `json:"greeting_name"`
	Continue     *Continue `json:"continue"`
	Stats        struct {
		CoursesTotal           int `json:"courses_total"`
		LessonsInCurrentCourse int `json:"lessons_in_current_course"`
		WatchedInCurrentCourse int `json:"watched_in_current_course"`
	} `json:"stats"`
	MyCourses []Course `json:"my_courses"`
	Updates   []Update `json:"updates"`
}

// Continue is the lesson the user can go on with.
type Continue struct {
	Course       CourseRef `json:"course"`
	Lesson       LessonRef `json:"lesson"`
	PositionSec  int       `json:"position_sec"`
	RemainingSec int       `json:"remaining_sec"`
}

// Update is one news item of the home screen.
type Update struct {
	Type   string `json:"type"`
	Course Course `json:"course"`
}

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// String returns the JSON text of c ("regular", "special", "intro"); "" for
// CategoryUnknown.
func (c Category) String() string {
	switch c {
	case CategoryRegular:
		return "regular"
	case CategorySpecial:
		return "special"
	case CategoryIntro:
		return "intro"
	case CategoryUnknown:
		return ""
	default:
		return ""
	}
}

// MarshalText implements encoding.TextMarshaler.
func (c Category) MarshalText() ([]byte, error) { return []byte(c.String()), nil }

// UnmarshalText implements encoding.TextUnmarshaler; a text the app does not
// know is CategoryUnknown, not an error.
func (c *Category) UnmarshalText(b []byte) error {
	switch string(b) {
	case "regular":
		*c = CategoryRegular
	case "special":
		*c = CategorySpecial
	case "intro":
		*c = CategoryIntro
	default:
		*c = CategoryUnknown
	}

	return nil
}
