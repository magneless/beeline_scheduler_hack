package contracts

import (
	"testing"
	"time"
)

func TestWorkingShiftUsesScenarioCivilTime(t *testing.T) {
	for _, tc := range []struct{ date, zone, start, end string }{
		{"2026-08-17", "Europe/Moscow", "2026-08-17T07:00:00Z", "2026-08-17T19:00:00Z"},
		{"2026-08-17", "Asia/Yekaterinburg", "2026-08-17T05:00:00Z", "2026-08-17T17:00:00Z"},
		// A DST transition must not move either local clock boundary.
		{"2026-03-29", "Europe/Berlin", "2026-03-29T08:00:00Z", "2026-03-29T20:00:00Z"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			got, err := WorkingShift(tc.date, tc.zone)
			if err != nil || got.Start.Format(time.RFC3339) != tc.start || got.End.Format(time.RFC3339) != tc.end {
				t.Fatalf("shift = %+v, err = %v", got, err)
			}
		})
	}
}

func TestValidateEngineerShiftsRejectsNonMandatorySchedule(t *testing.T) {
	shift, _ := WorkingShift("2026-08-17", "Europe/Moscow")
	for _, tc := range []struct {
		name       string
		start, end time.Duration
	}{
		{"starts early", -time.Hour, 0}, {"starts late", time.Hour, 0},
		{"ends early", 0, -time.Hour}, {"ends late", 0, time.Hour},
		{"wrong date", 24 * time.Hour, 24 * time.Hour}, {"fractional seconds", time.Nanosecond, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := Snapshot{Date: "2026-08-17", Timezone: "Europe/Moscow", Engineers: []Engineer{
				{ID: "reserve", Reserve: true, Shift: Window{Start: shift.Start.Add(tc.start), End: shift.End.Add(tc.end)}},
			}}
			if err := ValidateEngineerShifts(snapshot); err == nil {
				t.Fatal("invalid shift accepted")
			}
		})
	}
}
