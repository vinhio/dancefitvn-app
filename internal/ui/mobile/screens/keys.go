//go:build !tvos

package screens

import (
	"strconv"
	"strings"
)

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// CourseKey is the id layout.json writes a course by: the API's "set-35" is the
// prototype's "set35" (K10, kitdemo.ProtoID).
func CourseKey(courseID string) string { return strings.ReplaceAll(courseID, "-", "") }

// LessonKey is the key of a lesson's controls, `data-k`: "set35-3" for lesson
// 3 of "set-35".
func LessonKey(courseID string, no int) string { return CourseKey(courseID) + "-" + strconv.Itoa(no) }
