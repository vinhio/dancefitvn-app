package model

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Lesson is a lesson of a course. A locked lesson has no id, title, tag and
// duration (null in the JSON, "" and 0 here).
type Lesson struct {
	ID             string         `json:"id"`
	No             int            `json:"no"`
	Title          string         `json:"title"`
	Tag            string         `json:"tag"`
	DurationSec    int            `json:"duration_sec"`
	HasVideo       bool           `json:"has_video,omitzero"`
	HasAudio       bool           `json:"has_audio,omitzero"`
	AudioSizeBytes int64          `json:"audio_size_bytes,omitzero"`
	VideoAspect    string         `json:"video_aspect,omitzero"`
	Progress       LessonProgress `json:"progress,omitzero"`
	Preview        bool           `json:"preview,omitzero"`
	Locked         bool           `json:"locked,omitzero"`
}

// LessonProgress is the progress of the user in a lesson.
type LessonProgress struct {
	PositionSec int  `json:"position_sec"`
	Watched     bool `json:"watched"`
}

// LessonRef is the short form of a lesson (Home.Continue, next lesson).
type LessonRef struct {
	ID          string `json:"id"`
	No          int    `json:"no"`
	Title       string `json:"title"`
	Tag         string `json:"tag"`
	DurationSec int    `json:"duration_sec"`
}
