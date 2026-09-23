package data

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	c "github.com/magneless/beeline_scheduler_hack/internal/contracts"
	"golang.org/x/text/encoding/charmap"
)

const MappingVersion = "synthetic-v1"

type Dataset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	RegionID string `json:"region_id"`
	Date     string `json:"date"`
	Timezone string `json:"timezone"`
	File     string `json:"-"`
}

var Catalog = []Dataset{
	{"east", "Восток", "east", "2026-08-17", "Europe/Moscow", "Восток Синтетические данные.csv"},
	{"southeast", "Юго-восток", "southeast", "2026-08-17", "Europe/Moscow", "Юго-восток Синтетические данные.csv"},
	{"southcentral", "Югоцентр", "southcentral", "2026-08-17", "Europe/Moscow", "Югоцентр Синтетические данные.csv"},
}

type Importer struct {
	Geo      c.Geocoder
	Root     string
	Prepared map[string]c.Snapshot
}

func (i *Importer) Catalog() []Dataset {
	out := append([]Dataset{}, Catalog...)
	keys := make([]string, 0, len(i.Prepared))
	for id := range i.Prepared {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	for _, id := range keys {
		s := i.Prepared[id]
		out = append(out, Dataset{ID: id, Name: "Пример контрактов (заглушка)", RegionID: s.RegionID, Date: s.Date, Timezone: s.Timezone})
	}
	return out
}

func Lookup(id string) (Dataset, bool) {
	for _, d := range Catalog {
		if d.ID == id {
			return d, true
		}
	}
	return Dataset{}, false
}
func (i *Importer) Demo(ctx context.Context, id string) (c.Snapshot, any, error) {
	if prepared, ok := i.Prepared[id]; ok {
		snap := Clone(prepared)
		snap.ScenarioID = ""
		return snap, map[string]any{"source": "contract fixture", "development_stub": true}, nil
	}
	d, ok := Lookup(id)
	if !ok {
		return c.Snapshot{}, nil, c.NewError("NOT_FOUND", "Набор не найден")
	}
	f, e := os.Open(filepath.Join(i.Root, d.File))
	if e != nil {
		return c.Snapshot{}, nil, e
	}
	defer f.Close()
	return i.Import(ctx, f, d.RegionID, d.Date)
}

// FormatError separates unreadable CSV (400) from invalid input values (422).
type FormatError struct{ Message string }

func (e *FormatError) Error() string { return e.Message }

type ImportMetadata struct {
	MappingVersion string                   `json:"mapping_version"`
	RegionID       string                   `json:"region_id"`
	SourceRows     [][]string               `json:"source_rows"`
	Assumptions    []string                 `json:"assumptions"`
	Mappings       map[string]Normalization `json:"mappings"`
}

type Normalization struct {
	WorkType   c.WorkType            `json:"work_type"`
	Skill      string                `json:"skill"`
	ServiceSec int64                 `json:"service_sec"`
	Equipment  map[c.Equipment]int64 `json:"equipment_required"`
	Recognized bool                  `json:"recognized"`
}

func (i *Importer) Import(ctx context.Context, r io.Reader, region, date string) (c.Snapshot, any, error) {
	d, ok := Lookup(region)
	if !ok {
		return c.Snapshot{}, nil, c.NewError("INVALID_INPUT", "Неизвестный region_id")
	}
	zone, e := time.LoadLocation(d.Timezone)
	if e != nil {
		return c.Snapshot{}, nil, e
	}
	day, e := time.ParseInLocation("2006-01-02", date, zone)
	if e != nil {
		return c.Snapshot{}, nil, c.NewError("INVALID_INPUT", "Некорректная дата")
	}
	snap := c.Snapshot{Revision: 1, RegionID: region, Date: date, Timezone: d.Timezone, OfficeLocationID: region + "-office", Locations: []c.Location{}, Orders: []c.Order{}, Engineers: engineers(region, day), Issues: []c.Issue{}}
	raw, e := io.ReadAll(io.LimitReader(r, 10*1024*1024+1))
	if e != nil {
		return snap, nil, e
	}
	if len(raw) > 10*1024*1024 {
		return snap, nil, &FormatError{"CSV превышает 10 МБ"}
	}
	if !utf8.Valid(raw) {
		raw, e = charmap.Windows1251.NewDecoder().Bytes(raw)
		if e != nil {
			return snap, nil, &FormatError{"Нечитаемая кодировка"}
		}
	}
	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	header, e := reader.Read()
	if e != nil {
		return snap, nil, &FormatError{"Нет заголовка CSV"}
	}
	columns := map[string]int{}
	for n, h := range header {
		h = strings.TrimSpace(h)
		if _, exists := columns[h]; exists {
			return snap, nil, &FormatError{"Повторный заголовок CSV"}
		}
		columns[h] = n
	}
	for _, name := range []string{"Заявка", "Тип заявки BK", "Тип заявки HD", "Начало", "Окончание", "Адрес"} {
		if _, ok := columns[name]; !ok {
			return snap, nil, &FormatError{"Отсутствует колонка " + name}
		}
	}
	meta := ImportMetadata{MappingVersion: MappingVersion, RegionID: region, SourceRows: [][]string{}, Assumptions: []string{"Время источника московское; дата должна совпадать с выбранной", "Поступление исходных заявок без времени — начало местного дня", "Инженеры, навыки, транспорт и оборудование — фиксированная синтетическая модель v1"}}
	meta.Mappings = map[string]Normalization{}
	locations := []c.LocationInput{}
	seen := map[string]bool{}
	office := ""
	rows := map[string]int{}
	addIssue := func(row int, id, field, msg string) {
		snap.Issues = append(snap.Issues, c.Issue{SourceRow: &row, EntityID: &id, Field: &field, Code: "INVALID_INPUT", Message: msg})
	}
	for {
		row, e := reader.Read()
		if e == io.EOF {
			break
		}
		if e != nil {
			return snap, nil, &FormatError{"Нечитаемый CSV: " + e.Error()}
		}
		line, _ := reader.FieldPos(0)
		empty := true
		for j := range row {
			row[j] = strings.TrimSpace(row[j])
			if row[j] != "" {
				empty = false
			}
		}
		if empty {
			continue
		}
		meta.SourceRows = append(meta.SourceRows, row)
		if strings.EqualFold(row[0], "Адрес офиса") {
			if len(row) < 2 || row[1] == "" || office != "" {
				return snap, nil, c.NewError("INVALID_INPUT", "Некорректная или повторная строка офиса")
			}
			office = row[1]
			continue
		}
		if len(row) != len(header) {
			addIssue(line, "", "row", "Число колонок не совпадает с заголовком")
			continue
		}
		get := func(k string) string { return row[columns[k]] }
		id := get("Заявка")
		if id == "" || seen[id] {
			addIssue(line, id, "id", "Пустой или повторный ID заявки")
			continue
		}
		seen[id] = true
		start, e1 := time.ParseInLocation("02.01.2006 15:04", get("Начало"), zone)
		end, e2 := time.ParseInLocation("02.01.2006 15:04", get("Окончание"), zone)
		if e1 != nil || e2 != nil || end.Before(start) {
			addIssue(line, id, "window", "Некорректное окно")
			continue
		}
		if start.Format("2006-01-02") != date || end.Format("2006-01-02") != date {
			return snap, nil, c.NewError("INVALID_INPUT", "Дата CSV не совпадает с выбранной датой")
		}
		work, skill, sec, eq, ok := classify(get("Тип заявки BK"), get("Тип заявки HD"))
		meta.Mappings[get("Тип заявки BK")+" / "+get("Тип заявки HD")] = Normalization{work, skill, sec, eq, ok}
		if !ok {
			addIssue(line, id, "work_type", "Неоднозначное или неизвестное сочетание ВК/HD")
			continue
		}
		if get("Адрес") == "" {
			addIssue(line, id, "address", "Адрес отсутствует")
			continue
		}
		priority := c.PriorityNormal
		if work == c.WorkTypeEmergency {
			priority = c.PriorityUrgent
		}
		lid := region + "-location-" + id
		rows[lid] = line
		locations = append(locations, c.LocationInput{ID: lid, Address: get("Адрес")})
		snap.Orders = append(snap.Orders, c.Order{ID: id, LocationID: lid, WorkType: work, RequiredSkills: []string{skill}, Window: c.Window{Start: start.UTC(), End: end.UTC()}, ReceivedAt: day.UTC(), ServiceSec: sec, Priority: priority, EquipmentRequired: eq, SourceOrder: int64(line), Status: c.OrderStatusActive})
	}
	if office == "" {
		return snap, nil, c.NewError("INVALID_INPUT", "Отсутствует адрес офиса")
	}
	locations = append(locations, c.LocationInput{ID: snap.OfficeLocationID, Address: office})
	result, e := i.Geo.Geocode(ctx, c.GeocodeRequest{RegionID: region, Locations: locations})
	if e != nil {
		return snap, nil, e
	}
	expected := map[string]bool{}
	for _, l := range locations {
		expected[l.ID] = true
	}
	resolved := map[string]c.Location{}
	answered := map[string]bool{}
	for _, item := range result.Items {
		if !expected[item.LocationID] || answered[item.LocationID] || (item.Location == nil) == (item.Issue == nil) {
			return snap, nil, c.NewError("GEO_UNAVAILABLE", "Некорректный ответ геокодера")
		}
		answered[item.LocationID] = true
		if item.Location != nil {
			l := *item.Location
			if math.IsNaN(l.Point.Lat) || math.IsNaN(l.Point.Lon) || math.IsInf(l.Point.Lat, 0) || math.IsInf(l.Point.Lon, 0) {
				return snap, nil, c.NewError("GEO_UNAVAILABLE", "Некорректные координаты геокодера")
			}
			if l.ID != item.LocationID || l.Point.Lat < -90 || l.Point.Lat > 90 || l.Point.Lon < -180 || l.Point.Lon > 180 {
				return snap, nil, c.NewError("GEO_UNAVAILABLE", "Некорректные координаты геокодера")
			}
			resolved[l.ID] = l
		} else {
			issue := *item.Issue
			line := rows[item.LocationID]
			issue.SourceRow = &line
			snap.Issues = append(snap.Issues, issue)
		}
	}
	if len(answered) != len(expected) {
		return snap, nil, c.NewError("GEO_UNAVAILABLE", "Геокодер пропустил адреса")
	}
	if _, ok := resolved[snap.OfficeLocationID]; !ok {
		return snap, nil, c.NewError("INVALID_INPUT", "Не удалось определить офис")
	}
	for _, l := range locations {
		if resolvedLoc, ok := resolved[l.ID]; ok {
			snap.Locations = append(snap.Locations, resolvedLoc)
		}
	}
	orders := make([]c.Order, 0, len(snap.Orders))
	for _, o := range snap.Orders {
		if _, ok := resolved[o.LocationID]; ok {
			orders = append(orders, o)
		}
	}
	snap.Orders = orders
	return snap, meta, nil
}
func engineers(region string, day time.Time) []c.Engineer {
	result := []c.Engineer{}
	for n := 0; n < 8; n++ {
		skills := []string{"connection"}
		if n%3 == 0 {
			skills = []string{"connection", "repair", "emergency"}
		} else if n%3 == 1 {
			skills = []string{"repair", "emergency"}
		}
		transport := c.TransportCar
		if n%4 == 3 {
			transport = c.TransportWalk
		}
		result = append(result, c.Engineer{ID: fmt.Sprintf("%s-eng-%02d", region, n+1), Skills: skills, Transport: transport, Shift: c.Window{Start: day.Add(8 * time.Hour).UTC(), End: day.Add(23 * time.Hour).UTC()}, Available: true, EquipmentStock: map[c.Equipment]int64{c.EquipmentRouter: 4, c.EquipmentTVBox: 2}, SourceOrder: int64(n + 1)})
	}
	return result
}
func classify(bk, hd string) (c.WorkType, string, int64, map[c.Equipment]int64, bool) {
	eq := map[c.Equipment]int64{}
	switch bk {
	case "Подключение":
		if hd == "Конвергенция абонента" || hd == "Заявка на подключение" || hd == "Заказ подключения/Дозаказ оборудования" {
			eq[c.EquipmentRouter] = 1
			return c.WorkTypeConnection, "connection", 4200, eq, true
		}
	case "Дозаказ":
		if hd == "Дозаказ оборудования" || hd == "Заказ подключения/Дозаказ оборудования" {
			eq[c.EquipmentRouter] = 1
			return c.WorkTypeAdditional, "connection", 1200, eq, true
		}
	case "Глобальная проблема":
		if hd == "Авария" {
			return c.WorkTypeEmergency, "emergency", 4800, eq, true
		}
	case "Локальная заявка":
		switch hd {
		case "Мониторинг", "Нет линка", "Переключение на Гбит/с", "Работа с кабелем", "Разрывы", "Низкая скорость", "Рост ошибок на порту", "IP-адрес 169...", "TVE/ENT. Другие ошибки":
			return c.WorkTypeRepair, "repair", 1800, eq, true
		case "Роутер. Замена техническим специалистом":
			eq[c.EquipmentRouter] = 1
			return c.WorkTypeRepair, "repair", 1800, eq, true
		case "TVE/ENT. Замена приставки техником", "ТВ. Замена приставки техником":
			eq[c.EquipmentTVBox] = 1
			return c.WorkTypeRepair, "repair", 1800, eq, true
		}
	}
	return "", "", 0, eq, false
}
func Clone[T any](v T) T { b, _ := json.Marshal(v); var out T; json.Unmarshal(b, &out); return out }
