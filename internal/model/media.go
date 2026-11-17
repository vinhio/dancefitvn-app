package model

import "time"

// ====================================================================
// ============================ Contracts =============================
// ====================================================================

// Stream is the answer of GET /lessons/{id}/stream.
type Stream struct {
	LessonID          string    `json:"lesson_id"`
	HLSURL            string    `json:"hls_url"`
	ExpiresAt         time.Time `json:"expires_at,omitzero"`
	Aspect            string    `json:"aspect"`
	Variants          []Variant `json:"variants"`
	ResumePositionSec int       `json:"resume_position_sec"`
}

// Variant is one rendition of a stream.
type Variant struct {
	Width     int `json:"width"`
	Height    int `json:"height"`
	Bandwidth int `json:"bandwidth"`
}

// Audio is the answer of GET /lessons/{id}/audio.
type Audio struct {
	LessonID    string    `json:"lesson_id"`
	URL         string    `json:"url"`
	ExpiresAt   time.Time `json:"expires_at,omitzero"`
	Format      string    `json:"format"`
	BitrateKbps int       `json:"bitrate_kbps"`
	DurationSec int       `json:"duration_sec"`
	SizeBytes   int64     `json:"size_bytes"`
	SHA256      string    `json:"sha256"`
	Filename    string    `json:"filename"`
}
