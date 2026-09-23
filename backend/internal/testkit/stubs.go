// Package testkit contains explicit development substitutes, not production planning or geocoding.
package testkit

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"os"
	"reflect"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/data"
)

// Geocoder returns deterministic artificial points for offline API development.
type Geocoder struct{}

func (Geocoder) Geocode(ctx context.Context, in c.GeocodeRequest) (c.GeocodeResult, error) {
	out := c.GeocodeResult{Items: []c.GeocodeItem{}}
	for _, l := range in.Locations {
		if e := ctx.Err(); e != nil {
			return out, e
		}
		h := fnv.New32a()
		h.Write([]byte(l.Address))
		v := h.Sum32()
		p := c.Point{Lat: 55.6 + float64(v%2000)/10000, Lon: 37.4 + float64((v/2000)%4000)/10000}
		if l.Point != nil {
			p = *l.Point
		}
		out.Items = append(out.Items, c.GeocodeItem{LocationID: l.ID, Location: &c.Location{ID: l.ID, Address: l.Address, Point: p}})
	}
	return out, nil
}

type Reader interface {
	GetSnapshot(context.Context, string, int64) (c.Snapshot, error)
	GetPlan(context.Context, string) (c.Plan, error)
}
type Fixture struct {
	Snapshot      c.Snapshot      `json:"snapshot"`
	PlanResult    c.PlanResult    `json:"plan_result"`
	ReplanRequest c.ReplanRequest `json:"replan_request"`
	ReplanResult  c.PlanResult    `json:"replan_result"`
	Execution     struct {
		Steps []struct {
			Request  c.ReplanRequest `json:"request"`
			Response c.PlanResult    `json:"response"`
		} `json:"status_steps"`
	} `json:"execution_status_flow"`
}

func Load(path string) (Fixture, error) {
	var f Fixture
	b, e := os.ReadFile(path)
	if e == nil {
		e = json.Unmarshal(b, &f)
	}
	return f, e
}

type Plans struct {
	Reader  Reader
	Fixture Fixture
}

func (p *Plans) Build(ctx context.Context, in c.BuildPlanRequest) (c.PlanResult, error) {
	s, e := p.Reader.GetSnapshot(ctx, in.ScenarioID, in.SnapshotRevision)
	if e != nil {
		return c.PlanResult{}, e
	}
	if reflect.DeepEqual(s.Orders, p.Fixture.Snapshot.Orders) && reflect.DeepEqual(s.Engineers, p.Fixture.Snapshot.Engineers) && s.Date == p.Fixture.Snapshot.Date {
		r := data.Clone(p.Fixture.PlanResult)
		r.TargetSnapshot = s
		r.Draft.ScenarioID = s.ScenarioID
		r.Draft.SnapshotRevision = s.Revision
		return r, nil
	}
	// An empty feasible result supports infrastructure development for imported datasets.
	zone, e := time.LoadLocation(s.Timezone)
	if e != nil {
		return c.PlanResult{}, e
	}
	day, e := time.ParseInLocation("2006-01-02", s.Date, zone)
	if e != nil {
		return c.PlanResult{}, e
	}
	d := c.PlanDraft{ScenarioID: s.ScenarioID, SnapshotRevision: s.Revision, AsOf: day.UTC(), Routes: []c.Route{}, Unassigned: []c.UnassignedOrder{}, CancelledOrderIDs: []string{}, CompletedOrderIDs: []string{}, EquipmentRemaining: map[string]map[c.Equipment]int64{}, Issues: append([]c.Issue{}, s.Issues...), Metrics: c.Metrics{PerEngineer: []c.EngineerDistance{}}, Changes: []c.PlanChange{}, Termination: c.TerminationCompleted}
	d.Issues = append(d.Issues, c.Issue{Code: "DEVELOPMENT_STUB", Message: "Подключена заглушка Go-4; назначения не вычислялись"})
	for _, o := range s.Orders {
		if o.Status == c.OrderStatusCancelled {
			d.CancelledOrderIDs = append(d.CancelledOrderIDs, o.ID)
		} else {
			d.Unassigned = append(d.Unassigned, c.UnassignedOrder{OrderID: o.ID, ReasonCode: c.ReasonNotAssignedBySolver, Message: "Расчёт заглушкой не выполняется"})
		}
	}
	for _, eng := range s.Engineers {
		d.EquipmentRemaining[eng.ID] = data.Clone(eng.EquipmentStock)
	}
	d.Metrics.UnassignedCount = len(d.Unassigned)
	baseline := d.Metrics
	d.BaselineMetrics = &baseline
	return c.PlanResult{Draft: d, TargetSnapshot: s}, nil
}
func (p *Plans) Replan(ctx context.Context, in c.ReplanRequest) (c.PlanResult, error) {
	s, e := p.Reader.GetSnapshot(ctx, in.ScenarioID, in.SnapshotRevision)
	if e != nil {
		return c.PlanResult{}, e
	}
	matches := func(a, b c.Event) bool {
		var x, y any
		json.Unmarshal(a.Payload, &x)
		json.Unmarshal(b.Payload, &y)
		return a.Type == b.Type && a.OccurredAt.Equal(b.OccurredAt) && reflect.DeepEqual(x, y)
	}
	var result *c.PlanResult
	if matches(in.Event, p.Fixture.ReplanRequest.Event) && reflect.DeepEqual(s.Orders, p.Fixture.Snapshot.Orders) {
		copy := p.Fixture.ReplanResult
		result = &copy
	}
	previous := p.Fixture.Snapshot
	for _, step := range p.Fixture.Execution.Steps {
		if matches(in.Event, step.Request.Event) && reflect.DeepEqual(s.Orders, previous.Orders) {
			copy := step.Response
			result = &copy
			break
		}
		previous = step.Response.TargetSnapshot
	}
	if result == nil {
		return c.PlanResult{}, c.NewError("COMPUTATION_FAILED", "Заглушка Go-4 поддерживает только события из backend_flow.json; подключите реальный PlanService")
	}
	r := data.Clone(*result)
	r.Draft.ScenarioID = s.ScenarioID
	r.Draft.SnapshotRevision = s.Revision + 1
	r.Draft.BasePlanID = &in.BasePlanID
	r.TargetSnapshot.ScenarioID = s.ScenarioID
	r.TargetSnapshot.Revision = s.Revision + 1
	r.AppliedEvent = &in.Event
	return r, nil
}
