//go:build tvos

// Adapted from dancefitvn-app/internal/ui/mobile/env/playback.go (PlayerLessons
// and LessonIndex): the TV cannot import the mobile packages.

package env

import (
	"dancefitvn-app/internal/model"
	"dancefitvn-app/internal/player"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// PlayerLessons returns the lessons of c as the player wants them (the list
// Prev, Next and the end-of-lesson screen walk): the number, the length the API
// gives, the resume position and whether the lesson is watched, with the names
// of the course on each (the player's course line, the up-next card).
func PlayerLessons(c model.Course) []player.Lesson {
	out := make([]player.Lesson, len(c.Lessons))
	count := c.LessonCount
	if count == 0 {
		count = len(c.Lessons)
	}
	for i, l := range c.Lessons {
		out[i] = player.Lesson{
			CourseID: c.ID, CourseName: c.Name, CourseShort: c.Short(), Tagline: c.Tagline,
			ID: l.ID, Title: l.Title, Tag: l.Tag, CoverURL: c.CoverURL,
			No: l.No, Count: count, DurationSec: l.DurationSec,
			ResumeSec: l.Progress.PositionSec, Watched: l.Progress.Watched,
		}
	}

	return out
}

// LessonIndex returns where lesson id is in the list PlayerLessons made, -1 when
// the course has no such lesson.
func LessonIndex(list []player.Lesson, id string) int {
	for i := range list {
		if list[i].ID == id {
			return i
		}
	}

	return -1
}
