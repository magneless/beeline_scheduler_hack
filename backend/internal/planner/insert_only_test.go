package planner

import (
	"context"
	"errors"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"reflect"
	"testing"
	"time"
)

func ioAt(h int) time.Time { return time.Date(2026, 9, 17, h, 0, 0, 0, time.UTC) }
func ioPtr(v int64) *int64 { return &v }
func ioReq(limit int64) contracts.SolveRequest {
	cells := make([][]contracts.TravelCell, 3)
	for i := range cells {
		cells[i] = make([]contracts.TravelCell, 3)
		for j := range cells[i] {
			d := int64(0)
			if i != j {
				d = 600
			}
			x := int64(0)
			if i != j {
				x = 1000
			}
			cells[i][j] = contracts.TravelCell{Reachable: true, DurationSec: &d, DistanceM: &x}
		}
	}
	old := contracts.Order{ID: "old", LocationID: "old", WorkType: contracts.WorkTypeRepair, RequiredSkills: []string{"repair"}, Window: contracts.Window{Start: ioAt(7), End: ioAt(17)}, ReceivedAt: ioAt(6), ServiceSec: 1800, Priority: contracts.PriorityNormal, Status: contracts.OrderStatusActive}
	fresh := old
	fresh.ID = "new"
	fresh.LocationID = "new"
	fresh.Window = contracts.Window{Start: ioAt(8), End: ioAt(12)}
	fresh.ServiceSec = 600
	route := contracts.Route{EngineerID: "e", StartLocationID: "depot", StartAt: ioAt(6), Visits: []contracts.Visit{{OrderID: "old", ArrivalAt: ioAt(7), StartAt: ioAt(7), EndAt: ioAt(7).Add(30 * time.Minute)}}, Legs: []contracts.Leg{{ID: "keep", FromLocationID: "depot", ToLocationID: "old", StartAt: ioAt(6), EndAt: ioAt(7), DistanceM: 1000, GeoContextID: "g", Geometry: []contracts.Point{{Lat: 1, Lon: 2}}}}}
	_ = route
	return contracts.SolveRequest{Mode: contracts.SolveModeInsertOnly, Orders: []contracts.Order{old, fresh}, Engineers: []contracts.Engineer{{ID: "e", Skills: []string{"repair"}, Transport: contracts.TransportCar, Shift: contracts.Window{Start: ioAt(6), End: ioAt(18)}, Available: true}}, EngineerStates: []contracts.EngineerState{{EngineerID: "e", StartLocationID: "depot", AvailableFrom: ioAt(6)}}, TravelMatrix: contracts.TravelMatrix{ID: "m", GeoContextID: "g", LocationIDs: []string{"depot", "old", "new"}, Profiles: map[contracts.Transport][][]contracts.TravelCell{contracts.TransportCar: cells}}, FixedRoutes: []contracts.Route{route}, ProtectedLegIDs: []string{"keep"}, TimeLimitMS: limit}
}
func TestInsertOnlyPreservesProtectedLeg(t *testing.T) {
	in := ioReq(1000)
	before := in.FixedRoutes[0].Legs[0]
	got, err := NewInserter().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes[0].Visits) != 2 {
		t.Fatalf("route=%+v", got.Routes)
	}
	if !reflect.DeepEqual(got.Routes[0].Legs[0], before) {
		t.Fatalf("protected leg changed: %#v", got.Routes[0].Legs[0])
	}
}
func TestInsertOnlyMalformedFixedRoutes(t *testing.T) {
	in := ioReq(1000)
	in.FixedRoutes[0].Legs = nil
	_, err := NewInserter().Solve(context.Background(), in)
	var ce *contracts.ContractError
	if !errors.As(err, &ce) || ce.Code != "INVALID_INPUT" {
		t.Fatalf("err=%v", err)
	}
}
func TestInsertOnlyTimeoutKeepsFallback(t *testing.T) {
	in := ioReq(1)
	clock := ioAt(6)
	ins := &Inserter{now: func() time.Time { x := clock; clock = clock.Add(time.Millisecond); return x }}
	got, err := ins.Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Termination != contracts.TerminationTimeLimit || len(got.Routes) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNotAssignedBySolver {
		t.Fatalf("result=%+v", got)
	}
}

func TestInsertOnlyGapKeepsArrivalAndWaitsBeforeOldLeg(t *testing.T) {
	in := ioReq(1000)
	in.ProtectedLegIDs = nil
	in.FixedRoutes[0].Legs[0].StartAt = ioAt(7).Add(-10 * time.Minute)
	in.FixedRoutes[0].Legs[0].ID = "insert-leg-1"
	in.Orders[1].Window = contracts.Window{Start: ioAt(6), End: ioAt(6).Add(40 * time.Minute)}
	old := in.FixedRoutes[0].Visits[0]
	got, err := NewInserter().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 0 || len(got.Routes) != 1 || len(got.Routes[0].Visits) != 2 {
		t.Fatalf("gap insertion: %+v", got)
	}
	route := got.Routes[0]
	if !reflect.DeepEqual(route.Visits[1], old) || !route.Legs[1].EndAt.Equal(old.ArrivalAt) || !route.Legs[1].StartAt.Equal(old.ArrivalAt.Add(-10*time.Minute)) {
		t.Fatalf("old arrival was retimed: %+v", route)
	}
	if route.Legs[0].ID == route.Legs[1].ID || route.Legs[0].ID == "insert-leg-1" || route.Legs[1].ID == "insert-leg-1" {
		t.Fatalf("colliding leg IDs: %+v", route.Legs)
	}
}

func TestInsertOnlyCannotDepartToOldOrderBeforeReceived(t *testing.T) {
	in := ioReq(1000)
	in.ProtectedLegIDs = nil
	in.FixedRoutes[0].Legs[0].StartAt = ioAt(7).Add(-10 * time.Minute)
	in.Orders[0].ReceivedAt = in.FixedRoutes[0].Legs[0].StartAt
	in.Orders[1].Window = contracts.Window{Start: ioAt(6), End: ioAt(6).Add(40 * time.Minute)}
	in.TravelMatrix.Profiles[contracts.TransportCar][2][1].DurationSec = ioPtr(1200)
	got, err := NewInserter().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 1 || got.Unassigned[0].ReasonCode != contracts.ReasonNoFeasibleInsertion || !reflect.DeepEqual(got.Routes, in.FixedRoutes) {
		t.Fatalf("pre-receipt old leg accepted: %+v", got)
	}
}

func TestInsertOnlyProtectsRemainingTripFromCurrentPosition(t *testing.T) {
	in := ioReq(1000)
	in.EngineerStates[0].StartLocationID = "new"
	in.EngineerStates[0].AvailableFrom = ioAt(6).Add(30 * time.Minute)
	in.FixedRoutes[0].StartLocationID = "new"
	in.FixedRoutes[0].StartAt = in.EngineerStates[0].AvailableFrom
	in.Orders[1].Window = contracts.Window{Start: ioAt(6), End: ioAt(6).Add(40 * time.Minute)}
	// The fresh matrix disagrees with the actual trip. The saved remaining leg
	// is authoritative, and the worker cannot detour to a new job at this point.
	in.TravelMatrix.Profiles[contracts.TransportCar][2][1] = contracts.TravelCell{}
	before := cloneRoutes(in.FixedRoutes)
	got, err := NewInserter().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Unassigned) != 1 || !reflect.DeepEqual(got.Routes, before) {
		t.Fatalf("protected residual trip changed: %+v", got)
	}
	got.Routes[0].Legs[0].Geometry[0].Lat = 20
	if in.FixedRoutes[0].Legs[0].Geometry[0].Lat != 1 {
		t.Fatal("result aliases input geometry")
	}
}

func TestInsertOnlyUsesFreeEngineerAndShiftStart(t *testing.T) {
	in := ioReq(1000)
	in.FixedRoutes = nil
	in.ProtectedLegIDs = nil
	in.Orders = in.Orders[1:]
	in.Engineers[0].Shift.Start = ioAt(8)
	in.EngineerStates[0].AvailableFrom = ioAt(6)
	got, err := NewInserter().Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Routes) != 1 || len(got.Unassigned) != 0 || got.Routes[0].Legs[0].StartAt.Before(ioAt(8)) {
		t.Fatalf("free engineer not scheduled correctly: %+v", got)
	}
}

func TestInsertOnlyTimeoutInsideGapScanKeepsFallback(t *testing.T) {
	in := ioReq(1000)
	calls := 0
	ins := &Inserter{now: func() time.Time {
		calls++
		if calls >= 4 {
			return ioAt(6).Add(time.Second)
		}
		return ioAt(6)
	}}
	got, err := ins.Solve(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if got.Termination != contracts.TerminationTimeLimit || got.Unassigned[0].ReasonCode != contracts.ReasonNotAssignedBySolver || !reflect.DeepEqual(got.Routes, in.FixedRoutes) {
		t.Fatalf("inner timeout lost fallback: %+v", got)
	}
}

func TestInsertOnlyRejectsInvalidFixedSchedule(t *testing.T) {
	cases := map[string]func(*contracts.SolveRequest){
		"service": func(in *contracts.SolveRequest) { in.FixedRoutes[0].Visits[0].EndAt = ioAt(7) },
		"stock": func(in *contracts.SolveRequest) {
			in.Orders[0].EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: 1}
		},
		"duplicate engineer":     func(in *contracts.SolveRequest) { in.FixedRoutes = append(in.FixedRoutes, in.FixedRoutes[0]) },
		"duplicate protected ID": func(in *contracts.SolveRequest) { in.ProtectedLegIDs = append(in.ProtectedLegIDs, "keep") },
		"state":                  func(in *contracts.SolveRequest) { in.FixedRoutes[0].StartAt = ioAt(5) },
		"matrix":                 func(in *contracts.SolveRequest) { in.ProtectedLegIDs = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := ioReq(1000)
			mutate(&in)
			_, err := NewInserter().Solve(context.Background(), in)
			var ce *contracts.ContractError
			if !errors.As(err, &ce) || ce.Code != "INVALID_INPUT" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
