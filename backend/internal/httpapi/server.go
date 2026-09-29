package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/progress"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/storage"
)

type Server struct {
	Store    *storage.Store
	Importer *data.Importer
	Plans    c.PlanService
}
type responseError struct {
	status  int
	message string
}

func (e *responseError) Error() string { return e.message }
func invalid(msg string) error         { return c.NewError("INVALID_INPUT", msg) }
func (s *Server) Handler() http.Handler {
	m := http.NewServeMux()
	m.HandleFunc("GET /api/v1/demo-datasets", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"items": s.Importer.Catalog()})
	})
	m.HandleFunc("POST /api/v1/scenarios", s.create)
	m.HandleFunc("GET /api/v1/scenarios", func(w http.ResponseWriter, r *http.Request) {
		items, err := s.Store.ListScenarios(r.Context())
		respond(w, http.StatusOK, map[string]any{"items": items}, err)
	})
	m.HandleFunc("POST /api/v1/scenarios/import", s.importCSV)
	m.HandleFunc("GET /api/v1/scenarios/{id}", s.scenario)
	m.HandleFunc("POST /api/v1/geocode", s.searchAddress)
	m.HandleFunc("POST /api/v1/scenarios/{id}/orders/{order_id}/address", s.resolveOrderAddress)
	m.HandleFunc("PATCH /api/v1/scenarios/{id}/engineers/{engineer_id}", s.patch)
	m.HandleFunc("POST /api/v1/scenarios/{id}/engineers/import", s.importEngineers)
	m.HandleFunc("POST /api/v1/scenarios/{id}/plans", s.build)
	m.HandleFunc("POST /api/v1/scenarios/{id}/proposals", s.createProposal)
	m.HandleFunc("GET /api/v1/scenarios/{id}/pending-changes", s.getPending)
	m.HandleFunc("POST /api/v1/scenarios/{id}/pending-changes", s.savePending)
	m.HandleFunc("GET /api/v1/scenarios/{id}/proposals/current", s.currentProposal)
	m.HandleFunc("POST /api/v1/proposals/{id}/accept", s.acceptProposal)
	m.HandleFunc("POST /api/v1/scenarios/{id}/plans/compare", s.compare)
	m.HandleFunc("POST /api/v1/plans/{id}/events", s.event)
	m.HandleFunc("GET /api/v1/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetRun(r.Context(), r.PathValue("id"))
		respond(w, 200, v, e)
	})
	m.HandleFunc("GET /api/v1/plans/{id}", func(w http.ResponseWriter, r *http.Request) {
		v, e := s.Store.GetPlan(r.Context(), r.PathValue("id"))
		respond(w, 200, v, e)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		limit := int64(11 * 1024 * 1024)
		if r.Method == http.MethodPost && r.URL.Path == "/api/v1/scenarios/import" {
			limit = 21 * 1024 * 1024
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
		defer cancel()
		m.ServeHTTP(w, r.WithContext(ctx))
	})
}
func decode(r *http.Request, v any) error {
	b, e := io.ReadAll(r.Body)
	if e != nil {
		return &responseError{400, "Не удалось прочитать тело запроса"}
	}
	if !json.Valid(b) {
		return &responseError{400, "Нечитаемый JSON"}
	}
	if len(bytes.TrimSpace(b)) == 0 || bytes.TrimSpace(b)[0] != '{' {
		return invalid("Ожидается объект JSON")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(v); e != nil {
		return invalid("Некорректные поля JSON: " + e.Error())
	}
	return nil
}
func write(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
func respond(w http.ResponseWriter, status int, v any, e error) {
	if e != nil {
		failure(w, e)
		return
	}
	write(w, status, v)
}
func failure(w http.ResponseWriter, e error) {
	status, ce := errorResponse(e)
	write(w, status, ce)
}

func errorResponse(e error) (int, *c.ContractError) {
	var re *responseError
	var fe *data.FormatError
	var ce *c.ContractError
	status := 500
	if errors.As(e, &re) {
		status = re.status
		ce = c.NewError("INVALID_INPUT", re.message)
	} else if errors.As(e, &fe) {
		status = 400
		ce = c.NewError("INVALID_INPUT", fe.Message)
	} else if errors.As(e, &ce) {
		switch ce.Code {
		case "INVALID_INPUT":
			status = 422
		case "NOT_FOUND":
			status = 404
		case "STALE_VERSION", "EVENT_CONFLICT", "IDEMPOTENCY_CONFLICT":
			status = 409
		case "GEO_UNAVAILABLE":
			status = 503
		}
	} else {
		slog.Error("HTTP request failed", "error", e)
		ce = c.NewError("COMPUTATION_FAILED", "Внутренняя ошибка сервера")
	}
	if ce.Details == nil {
		copy := *ce
		copy.Details = map[string]any{}
		ce = &copy
	}
	return status, ce
}
func (s *Server) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DatasetID string `json:"demo_dataset_id"`
	}
	if e := decode(r, &in); e != nil {
		failure(w, e)
		return
	}
	if in.DatasetID == "" {
		failure(w, invalid("Не указан demo_dataset_id"))
		return
	}
	respondCalculation(w, r, http.StatusCreated, 0, func(ctx context.Context) (any, error) {
		snap, meta, err := s.Importer.Demo(ctx, in.DatasetID)
		if err != nil {
			return nil, err
		}
		return s.saveImportedScenario(ctx, snap, meta)
	})
}
func (s *Server) importCSV(w http.ResponseWriter, r *http.Request) {
	if e := r.ParseMultipartForm(1024 * 1024); e != nil {
		failure(w, &responseError{400, "Некорректный multipart"})
		return
	}
	defer r.MultipartForm.RemoveAll()
	f, _, e := r.FormFile("file")
	if e != nil {
		failure(w, &responseError{400, "Отсутствует file"})
		return
	}
	defer f.Close()
	// The streaming worker can outlive a disconnected request. Give it its own
	// bytes so multipart cleanup cannot close the file while it is being read.
	raw, e := io.ReadAll(io.LimitReader(f, 10*1024*1024+1))
	if e != nil {
		failure(w, e)
		return
	}
	region, date := r.FormValue("region_id"), r.FormValue("date")
	var office *c.LocationInput
	if address := strings.TrimSpace(r.FormValue("office_address")); address != "" {
		office = &c.LocationInput{Address: address}
	}
	if pointJSON := r.FormValue("office_point"); pointJSON != "" {
		var point struct {
			Lat *float64 `json:"lat"`
			Lon *float64 `json:"lon"`
		}
		if office == nil || json.Unmarshal([]byte(pointJSON), &point) != nil || point.Lat == nil || point.Lon == nil {
			failure(w, invalid("Укажите адрес и корректные координаты офиса"))
			return
		}
		office.Point = &c.Point{Lat: *point.Lat, Lon: *point.Lon}
	}
	var engineers []c.Engineer
	roster, _, rosterErr := r.FormFile("engineers_file")
	if rosterErr == nil {
		defer roster.Close()
		engineers, e = data.ParseEngineers(roster, date, "Europe/Moscow")
		if e != nil {
			_, issue := errorResponse(e)
			failure(w, invalid("Файл бригад: "+issue.Message))
			return
		}
	} else if rosterErr != http.ErrMissingFile || strings.TrimSpace(region) == "" {
		failure(w, invalid("Добавьте CSV со списком бригад"))
		return
	}
	respondCalculation(w, r, http.StatusCreated, 0, func(ctx context.Context) (any, error) {
		snap, meta, err := s.Importer.ImportDay(ctx, bytes.NewReader(raw), region, date, office)
		if err != nil {
			return nil, err
		}
		if engineers != nil {
			snap.Engineers = engineers
			issues := make([]c.Issue, 0, len(snap.Issues))
			for _, issue := range snap.Issues {
				if issue.Code != "ENGINEERS_REQUIRED" {
					issues = append(issues, issue)
				}
			}
			snap.Issues = issues
		}
		return s.saveImportedScenario(ctx, snap, meta)
	})
}

func (s *Server) saveImportedScenario(ctx context.Context, snap c.Snapshot, meta any) (any, error) {
	total := len(snap.Orders) + len(snap.UnlocatedOrders)
	progress.Report(progress.WithState(ctx, progress.State{Completed: total, Total: total}), "saving", "Сохраняем сценарий")
	return s.Store.CreateScenario(ctx, snap, meta)
}
func (s *Server) scenario(w http.ResponseWriter, r *http.Request) {
	var rev int64
	if values, ok := r.URL.Query()["revision"]; ok {
		if len(values) != 1 {
			failure(w, invalid("Одна revision обязательна"))
			return
		}
		var e error
		rev, e = strconv.ParseInt(values[0], 10, 64)
		if e != nil || rev < 1 {
			failure(w, invalid("revision должна быть положительной"))
			return
		}
	}
	v, e := s.Store.GetScenario(r.Context(), r.PathValue("id"), rev)
	respond(w, 200, v, e)
}
func (s *Server) build(w http.ResponseWriter, r *http.Request) {
	if _, enabled := s.Plans.(optionService); enabled {
		failure(w, c.NewError("EVENT_CONFLICT", "Для нового плана сначала рассчитайте варианты через proposals и примите один из них"))
		return
	}
	var in struct {
		SolveMode c.SolveMode     `json:"solve_mode"`
		RequestID string          `json:"request_id"`
		Revision  int64           `json:"snapshot_revision"`
		Expected  json.RawMessage `json:"expected_current_plan_id"`
	}
	if e := decode(r, &in); e != nil {
		failure(w, e)
		return
	}
	if in.SolveMode != "" && in.SolveMode != c.SolveModeBaseline && in.SolveMode != c.SolveModeOptimized {
		failure(w, invalid("solve_mode должен быть baseline или optimized"))
		return
	}
	if strings.TrimSpace(in.RequestID) == "" || in.Revision < 1 || len(in.Expected) == 0 {
		failure(w, invalid("Требуются request_id, snapshot_revision и expected_current_plan_id"))
		return
	}
	var expected *string
	if e := json.Unmarshal(in.Expected, &expected); e != nil || (expected != nil && strings.TrimSpace(*expected) == "") {
		failure(w, invalid("Некорректный expected_current_plan_id"))
		return
	}
	cmd := storage.Command{Kind: "build", Build: &c.BuildPlanRequest{SolveMode: in.SolveMode, RequestID: in.RequestID, ScenarioID: r.PathValue("id"), SnapshotRevision: in.Revision, ExpectedCurrentPlanID: expected}}
	id, e := s.Store.Register(r.Context(), cmd)
	respond(w, 202, map[string]string{"run_id": id}, e)
}
func (s *Server) event(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SolveMode c.SolveMode `json:"solve_mode"`
		RequestID string      `json:"request_id"`
		Revision  int64       `json:"snapshot_revision"`
		Event     c.Event     `json:"event"`
	}
	if e := decode(r, &in); e != nil {
		failure(w, e)
		return
	}
	if in.SolveMode != "" && in.SolveMode != c.SolveModeBaseline && in.SolveMode != c.SolveModeOptimized {
		failure(w, invalid("solve_mode должен быть baseline или optimized"))
		return
	}
	if strings.TrimSpace(in.RequestID) == "" || in.Revision < 1 {
		failure(w, invalid("Некорректные request_id или snapshot_revision"))
		return
	}
	if e := validateEvent(in.Event); e != nil {
		failure(w, e)
		return
	}
	p, e := s.Store.GetPlan(r.Context(), r.PathValue("id"))
	if e != nil {
		failure(w, e)
		return
	}
	if _, enabled := s.Plans.(optionService); enabled {
		unassignedCancellation := false
		if in.Event.Type == "order_cancelled" {
			var cancellation c.OrderCancelled
			if e := payload(in.Event.Payload, &cancellation); e != nil {
				failure(w, e)
				return
			}
			ids, err := cancellation.IDs()
			if err != nil {
				failure(w, err)
				return
			}
			unassigned := make(map[string]bool, len(p.Unassigned))
			for _, item := range p.Unassigned {
				unassigned[item.OrderID] = true
			}
			unassignedCancellation = true
			for _, id := range ids {
				unassignedCancellation = unassignedCancellation && unassigned[id]
			}
			if !unassignedCancellation && cancellation.OrderIDs != nil {
				failure(w, c.NewError("EVENT_CONFLICT", "Для общей отмены выберите только неназначенные заявки. Обновите список."))
				return
			}
		}
		if in.Event.Type != "order_status_changed" && !unassignedCancellation {
			failure(w, c.NewError("EVENT_CONFLICT", "Для изменения расписания сначала рассчитайте варианты через proposals и примите один из них"))
			return
		}
		if in.Event.Type == "order_status_changed" {
			var change c.OrderStatusChanged
			if e := payload(in.Event.Payload, &change); e != nil {
				failure(w, e)
				return
			}
			if change.Status != c.OrderStatusInProgress && change.Status != c.OrderStatusCompleted {
				failure(w, invalid("Инженер сообщает только о начале и завершении работы; статус «В пути» определяется автоматически"))
				return
			}
			if change.Status == c.OrderStatusCompleted {
				for _, route := range p.Routes {
					for _, visit := range route.Visits {
						if visit.OrderID == change.OrderID && in.Event.OccurredAt.After(visit.EndAt) {
							failure(w, c.NewError("EVENT_CONFLICT", "Позднее завершение меняет маршрут. Сохраните событие в очередь и запустите пересчёт кнопкой."))
							return
						}
					}
				}
			}
		}
	}
	in.Event.OccurredAt = in.Event.OccurredAt.UTC()
	in.Event = normalizeEvent(in.Event)
	cmd := storage.Command{Kind: "replan", Replan: &c.ReplanRequest{SolveMode: in.SolveMode, RequestID: in.RequestID, ScenarioID: p.ScenarioID, SnapshotRevision: in.Revision, BasePlanID: p.ID, Event: in.Event}}
	id, e := s.Store.Register(r.Context(), cmd)
	respond(w, 202, map[string]string{"run_id": id}, e)
}
func payload(raw json.RawMessage, v any) error {
	if len(raw) == 0 || bytes.TrimSpace(raw)[0] != '{' {
		return invalid("payload должен быть объектом")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		return invalid("Некорректный payload: " + e.Error())
	}
	return nil
}
func validateEvent(e c.Event) error {
	if strings.TrimSpace(e.ID) == "" || e.OccurredAt.IsZero() || e.OccurredAt.Nanosecond() != 0 {
		return invalid("Требуются ID и время события с точностью до секунды")
	}
	switch e.Type {
	case "urgent_order_added", "ordinary_order_added":
		var p c.UrgentOrderAdded
		if err := payload(e.Payload, &p); err != nil {
			return err
		}
		return validateNewOrder(e, p)
	case "order_cancelled":
		var p c.OrderCancelled
		if err := payload(e.Payload, &p); err != nil {
			return err
		}
		_, err := p.IDs()
		return err
	case "engineer_unavailable":
		var p c.EngineerUnavailable
		if err := payload(e.Payload, &p); err != nil {
			return err
		}
		if p.EngineerID == "" {
			return invalid("Требуется engineer_id")
		}
	case "order_status_changed":
		var p c.OrderStatusChanged
		if err := payload(e.Payload, &p); err != nil {
			return err
		}
		if p.OrderID == "" || p.EngineerID == "" {
			return invalid("Требуются order_id и engineer_id")
		}
		switch p.Status {
		case c.OrderStatusSent, c.OrderStatusEnRoute, c.OrderStatusInProgress, c.OrderStatusCompleted:
		default:
			return invalid("Недопустимый статус")
		}
		if p.ExpectedEndAt != nil && (p.Status != c.OrderStatusInProgress || !p.ExpectedEndAt.After(e.OccurredAt) || p.ExpectedEndAt.Nanosecond() != 0) {
			return invalid("Некорректная оценка окончания")
		}
	default:
		return invalid("Неизвестный тип события")
	}
	return nil
}
func equipment(m map[c.Equipment]int64) error {
	for k, v := range m {
		if (k != c.EquipmentRouter && k != c.EquipmentTVBox) || v < 0 {
			return invalid("Некорректное оборудование")
		}
	}
	return nil
}
func (s *Server) patch(w http.ResponseWriter, r *http.Request) {
	var raw map[string]json.RawMessage
	if e := decode(r, &raw); e != nil {
		failure(w, e)
		return
	}
	var rev int64
	if e := json.Unmarshal(raw["expected_revision"], &rev); e != nil || rev < 1 {
		failure(w, invalid("Требуется expected_revision"))
		return
	}
	delete(raw, "expected_revision")
	if len(raw) == 0 {
		failure(w, invalid("Нет изменяемых полей"))
		return
	}
	for key, value := range raw {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			failure(w, invalid("null запрещён: "+key))
			return
		}
		switch key {
		case "reserve":
			failure(w, invalid("Резерв определяется по первому принятому плану и не задаётся вручную"))
			return
		case "skills", "transport", "shift", "available", "equipment_stock":
		default:
			failure(w, invalid("Неизвестное поле: "+key))
			return
		}
	}
	v, e := s.Store.PatchEngineer(r.Context(), r.PathValue("id"), r.PathValue("engineer_id"), rev, func(eng *c.Engineer) error {
		for key, value := range raw {
			var target any
			switch key {
			case "skills":
				target = &eng.Skills
			case "transport":
				target = &eng.Transport
			case "shift":
				eng.Shift = c.Window{}
				target = &eng.Shift
			case "available":
				target = &eng.Available
			case "reserve":
				target = &eng.Reserve
			case "equipment_stock":
				eng.EquipmentStock = map[c.Equipment]int64{}
				target = &eng.EquipmentStock
			}
			decoder := json.NewDecoder(bytes.NewReader(value))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(target); err != nil {
				return invalid("Некорректное поле " + key)
			}
		}
		if _, reserveChanged := raw["reserve"]; reserveChanged && eng.Reserve {
			eng.Available = false
		}
		if eng.Transport != c.TransportCar && eng.Transport != c.TransportWalk {
			return invalid("Некорректный транспорт")
		}
		if eng.Shift.Start.IsZero() || !eng.Shift.End.After(eng.Shift.Start) || eng.Shift.Start.Nanosecond() != 0 || eng.Shift.End.Nanosecond() != 0 {
			return invalid("Некорректная смена")
		}
		for _, skill := range eng.Skills {
			if strings.TrimSpace(skill) == "" {
				return invalid("Пустой навык")
			}
		}
		eng.Shift.Start = eng.Shift.Start.UTC()
		eng.Shift.End = eng.Shift.End.UTC()
		return equipment(eng.EquipmentStock)
	})
	respond(w, 200, v, e)
}
