package data

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

var engineerHeader = []string{"id", "skills", "transport", "shift_start", "shift_end", "available", "router", "tv_box"}

// ParseEngineers parses the strict semicolon-separated roster format.
func ParseEngineers(r io.Reader, date, timezone string) ([]c.Engineer, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return nil, &FormatError{"Некорректная timezone"}
	}
	raw, readErr := io.ReadAll(io.LimitReader(r, 10*1024*1024+1))
	if readErr != nil {
		return nil, &FormatError{"Не удалось прочитать CSV"}
	}
	if len(raw) > 10*1024*1024 {
		return nil, &FormatError{"CSV превышает 10 МБ"}
	}
	cr := csv.NewReader(strings.NewReader(string(raw)))
	cr.Comma = ';'
	cr.FieldsPerRecord = -1
	h, err := cr.Read()
	if err != nil {
		return nil, &FormatError{"Нет заголовка CSV"}
	}
	if len(h) != len(engineerHeader) {
		return nil, &FormatError{"Некорректный заголовок CSV"}
	}
	for i := range h {
		if strings.TrimSpace(h[i]) != engineerHeader[i] {
			return nil, &FormatError{"Некорректный заголовок CSV"}
		}
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, &FormatError{"Некорректная дата сценария"}
	}
	var out []c.Engineer
	seen := map[string]bool{}
	for line := 2; ; line++ {
		row, e := cr.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, &FormatError{fmt.Sprintf("Строка %d: некорректный CSV", line)}
		}
		if len(row) != len(engineerHeader) {
			return nil, &FormatError{fmt.Sprintf("Строка %d: ожидается 8 полей", line)}
		}
		for i := range row {
			row[i] = strings.TrimSpace(row[i])
		}
		if row[0] == "" || seen[row[0]] {
			return nil, &FormatError{fmt.Sprintf("Строка %d: пустой или повторяющийся id", line)}
		}
		seen[row[0]] = true
		skills := make([]string, 0)
		if row[1] != "" {
			for _, s := range strings.Split(row[1], ",") {
				s = strings.TrimSpace(s)
				if s == "" {
					return nil, &FormatError{fmt.Sprintf("Строка %d: некорректные skills", line)}
				}
				skills = append(skills, s)
			}
		}
		var transport c.Transport
		switch row[2] {
		case "car":
			transport = c.TransportCar
		case "walk":
			transport = c.TransportWalk
		default:
			return nil, &FormatError{fmt.Sprintf("Строка %d: transport должен быть car или walk", line)}
		}
		start, e := parseClock(day, row[3], loc)
		if e != nil {
			return nil, &FormatError{fmt.Sprintf("Строка %d: некорректный shift_start", line)}
		}
		end, e := parseClock(day, row[4], loc)
		if e != nil || !end.After(start) {
			return nil, &FormatError{fmt.Sprintf("Строка %d: некорректный shift_end", line)}
		}
		var available bool
		switch row[5] {
		case "true":
			available = true
		case "false":
			available = false
		default:
			return nil, &FormatError{fmt.Sprintf("Строка %d: available должен быть true или false", line)}
		}
		router, e := parseNonnegative(row[6])
		if e != nil {
			return nil, &FormatError{fmt.Sprintf("Строка %d: некорректный router", line)}
		}
		tv, e := parseNonnegative(row[7])
		if e != nil {
			return nil, &FormatError{fmt.Sprintf("Строка %d: некорректный tv_box", line)}
		}
		out = append(out, c.Engineer{ID: row[0], Skills: skills, Transport: transport, Shift: c.Window{Start: start.UTC(), End: end.UTC()}, Available: available, EquipmentStock: map[c.Equipment]int64{c.EquipmentRouter: router, c.EquipmentTVBox: tv}, SourceOrder: int64(len(out) + 1)})
	}
	if len(out) == 0 {
		return nil, &FormatError{"Состав инженеров не может быть пустым"}
	}
	return out, nil
}
func parseClock(day time.Time, value string, loc *time.Location) (time.Time, error) {
	t, e := time.ParseInLocation("15:04", value, loc)
	if e != nil {
		return time.Time{}, e
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}
func parseNonnegative(v string) (int64, error) {
	n, e := strconv.ParseInt(v, 10, 64)
	if e != nil || n < 0 {
		return 0, fmt.Errorf("invalid")
	}
	return n, nil
}
