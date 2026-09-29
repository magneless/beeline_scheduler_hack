package contracts

import "time"

// WorkingShift is the mandatory crew schedule from QA chat 4, in scenario time.
func WorkingShift(date, timezone string) (Window, error) {
	zone, err := time.LoadLocation(timezone)
	if err != nil || timezone == "" {
		return Window{}, NewError("INVALID_INPUT", "Некорректная timezone сценария")
	}
	day, err := time.ParseInLocation("2006-01-02", date, zone)
	if err != nil {
		return Window{}, NewError("INVALID_INPUT", "Некорректная дата сценария")
	}
	return WorkingShiftOn(day), nil
}

func WorkingShiftOn(day time.Time) Window {
	return Window{
		Start: time.Date(day.Year(), day.Month(), day.Day(), 10, 0, 0, 0, day.Location()).UTC(),
		End:   time.Date(day.Year(), day.Month(), day.Day(), 22, 0, 0, 0, day.Location()).UTC(),
	}
}

func ValidateEngineerShifts(snapshot Snapshot) error {
	want, err := WorkingShift(snapshot.Date, snapshot.Timezone)
	if err != nil {
		return err
	}
	for _, engineer := range snapshot.Engineers {
		if !engineer.Shift.Start.Equal(want.Start) || !engineer.Shift.End.Equal(want.End) {
			return NewError("INVALID_INPUT", "Смена бригады "+engineer.ID+" должна быть 10:00–22:00 на дату сценария в его часовом поясе")
		}
	}
	return nil
}
