package model

import (
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCourseShort(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, want string
	}{
		{"DanceFIT Set 35", "SET 35"},
		{"DanceFIT Hot Extra", "HOT EXTRA"},
		{"DanceFIT Hot Extra 2", "HOT EXTRA 2"},
		{"Giới thiệu DanceFIT", "GIỚI THIỆU"},
		{"Other", "OTHER"},
		{"", ""},
		{"DanceFIT", "DANCEFIT"},
		// production names: the registered mark goes, the subtitle after the first colon too
		{"DanceFIT® Set 35: \"Cùng nhảy cùng hát\"", "SET 35"},
		{"DanceFIT® Set 35 : subtitle: more", "SET 35"},
		{"DanceFIT® Hot Extra", "HOT EXTRA"},
		{"Giới thiệu DanceFIT®", "GIỚI THIỆU"},
		{"Giới thiệu DanceFIT®: bắt đầu", "GIỚI THIỆU"},
		{"DanceFIT Set 36:", "SET 36"},
		{": only a subtitle", ""},
		{"  DanceFIT Set 7  ", "SET 7"},
	} {
		assert.Equal(t, tc.want, Course{Name: tc.name}.Short(), tc.name)
	}
}

func TestCourseShortOfMockCourses(t *testing.T) {
	t.Parallel()

	want := map[string]string{
		"hot-extra-2": "HOT EXTRA 2", "hot-extra": "HOT EXTRA", "intro": "GIỚI THIỆU",
	}
	for n := 24; n <= 36; n++ {
		want[courseFile(n)] = "SET " + string(rune('0'+n/10)) + string(rune('0'+n%10))
	}
	assert.Len(t, want, 16)
	for id, short := range want {
		var c Course
		require.NoError(t, json.Unmarshal(readMock(t, "courses/"+id+".json"), &c))
		assert.Equal(t, short, c.Short(), id)
	}
}

func courseFile(n int) string { return "set-" + string(rune('0'+n/10)) + string(rune('0'+n%10)) }
