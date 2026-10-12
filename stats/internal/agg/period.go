package agg

import (
	"fmt"
	"time"
)

type Period string

const (
	Daily   Period = "daily"
	Weekly  Period = "weekly"
	Monthly Period = "monthly"
	AllTime Period = "all_time"
)

func (p Period) Valid() bool {
	switch p {
	case Daily, Weekly, Monthly, AllTime:
		return true
	}
	return false
}

// KeyFor devuelve la clave del bucket del período que contiene a t (UTC).
func KeyFor(p Period, t time.Time) string {
	t = t.UTC()
	switch p {
	case Daily:
		return string(Daily) + ":" + t.Format("2006-01-02")
	case Weekly:
		y, w := t.ISOWeek()
		return string(Weekly) + ":" + fmt.Sprintf("%04d-W%02d", y, w)
	case Monthly:
		return string(Monthly) + ":" + t.Format("2006-01")
	}
	return ""
}

// StartOf devuelve el inicio del período que contiene a t (UTC).
func StartOf(p Period, t time.Time) time.Time {
	t = t.UTC()
	switch p {
	case Daily:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	case Weekly:
		monday := t.AddDate(0, 0, -(int(t.Weekday())+6)%7)
		return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
	case Monthly:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Time{}
}