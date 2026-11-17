package format

import "time"

// ====================================================================
// ========================== Main functions ==========================
// ====================================================================

// Date formats t as day/month/year, "12/03/2024", in t's own zone.
func Date(t time.Time) string { return t.Format("02/01/2006") }

// DayMonth formats t as day/month, "14/11", in t's own zone.
func DayMonth(t time.Time) string { return t.Format("02/01") }

// Weekday returns the day of the week of t in t's own zone, 0 for Sunday.
// The names belong to the locale.
func Weekday(t time.Time) int { return int(t.Weekday()) }
