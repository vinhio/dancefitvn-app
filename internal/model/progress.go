package model

import (
	"encoding/json/jsontext"
	"time"
)

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// ProgressResult is the answer of PUT /lessons/{id}/progress. NextLesson is
// set when the lesson became watched and another follows.
type ProgressResult struct {
	LessonID             string     `json:"lesson_id"`
	PositionSec          int        `json:"position_sec"`
	DurationSec          int        `json:"duration_sec"`
	Watched              bool       `json:"watched"`
	CourseID             string     `json:"course_id"`
	WatchedCountInCourse int        `json:"watched_count_in_course"`
	NextLesson           *LessonRef `json:"next_lesson,omitzero"`
	UpdatedAt            time.Time  `json:"updated_at,omitzero"`
}

// BatchResult is the answer of POST /progress/batch. The shape of a rejected
// item is not documented, so it stays raw.
type BatchResult struct {
	Accepted   int              `json:"accepted"`
	Rejected   []jsontext.Value `json:"rejected"`
	ServerTime time.Time        `json:"server_time,omitzero"`
}
