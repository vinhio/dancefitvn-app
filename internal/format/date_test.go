package format

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDate(t *testing.T) {
	t.Parallel()

	zone := time.FixedZone("ICT", 7*3600)
	for _, tc := range []struct {
		name         string
		t            time.Time
		date, dayMon string
		weekday      int
	}{
		{"member since", time.Date(2024, 3, 12, 0, 0, 0, 0, time.UTC), "12/03/2024", "12/03", 2},
		{"today", time.Date(2026, 11, 14, 9, 41, 0, 0, zone), "14/11/2026", "14/11", 6},
		{"sunday", time.Date(2026, 11, 15, 0, 0, 0, 0, zone), "15/11/2026", "15/11", 0},
		{"single digits", time.Date(2025, 1, 2, 23, 59, 0, 0, zone), "02/01/2025", "02/01", 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.date, Date(tc.t))
			assert.Equal(t, tc.dayMon, DayMonth(tc.t))
			assert.Equal(t, tc.weekday, Weekday(tc.t))
		})
	}
}
