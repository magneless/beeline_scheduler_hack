//go:build vroom

package optimized_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/optimized"
)

// This oracle deliberately does not call planner validation, scoring, Worker,
// materialize, or insertion helpers. It enumerates every feasible ordered
// subset for every engineer, then combines disjoint subsets by dynamic
// programming. Only the six public objective components are compared: the
// deterministic assignment tie-break is not a claim of this oracle.
type auditCost [6]int64

func auditLess(a, b auditCost) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func auditKey(in c.SolveRequest, mask int, delay, crews, distance int64) auditCost {
	key := auditCost{0, 0, 0, delay, crews, distance}
	for i, o := range in.Orders {
		if mask&(1<<i) == 0 {
			continue
		}
		switch o.WorkType {
		case c.WorkTypeEmergency:
			key[0]--
		case c.WorkTypeConnection:
			key[1]--
		default:
			key[2]--
		}
	}
	if in.EmergencyFirst {
		key[1], key[2], key[3] = key[3], key[1], key[2]
	}
	return key
}

func auditExact(in c.SolveRequest) (auditCost, int) {
	locations := map[string]int{}
	for i, id := range in.TravelMatrix.LocationIDs {
		locations[id] = i
	}
	states := map[string]c.EngineerState{}
	for _, s := range in.EngineerStates {
		states[s.EngineerID] = s
	}
	used := map[string]bool{}
	for _, id := range in.AlreadyUsedEngineerIDs {
		used[id] = true
	}
	type cost struct{ delay, crews, distance int64 }
	less := func(a, b cost) bool {
		return a.delay < b.delay || a.delay == b.delay && (a.crews < b.crews || a.crews == b.crews && a.distance < b.distance)
	}
	dp := map[int]cost{0: {crews: int64(len(used))}}
	possible := 0
	for _, e := range in.Engineers {
		s := states[e.ID]
		skills := map[string]bool{}
		for _, skill := range e.Skills {
			skills[skill] = true
		}
		stock := [2]int64{s.EquipmentAvailable[c.EquipmentRouter], s.EquipmentAvailable[c.EquipmentTVBox]}
		routes := map[int]cost{0: {}}
		var walk func(int, int, int64, [2]int64, int64, int64)
		walk = func(mask, location int, available int64, remaining [2]int64, delay, distance int64) {
			for i, o := range in.Orders {
				if mask&(1<<i) != 0 {
					continue
				}
				eligible := true
				for _, skill := range o.RequiredSkills {
					eligible = eligible && skills[skill]
				}
				if !eligible || o.RequiredTransport != nil && *o.RequiredTransport != e.Transport {
					continue
				}
				a, b := o.EquipmentRequired[c.EquipmentRouter], o.EquipmentRequired[c.EquipmentTVBox]
				if a > remaining[0] || b > remaining[1] {
					continue
				}
				cell := in.TravelMatrix.Profiles[e.Transport][location][locations[o.LocationID]]
				if !cell.Reachable {
					continue
				}
				depart := max(available, o.ReceivedAt.Unix())
				start := max(depart+*cell.DurationSec, o.Window.Start.Unix())
				finish := start + o.ServiceSec
				if start > o.Window.End.Unix() || finish > e.Shift.End.Unix() {
					continue
				}
				newDelay := delay
				if o.WorkType == c.WorkTypeEmergency {
					newDelay += start - max(o.Window.Start.Unix(), o.ReceivedAt.Unix())
				}
				newMask := mask | (1 << i)
				extra := int64(1)
				if used[e.ID] {
					extra = 0
				}
				value := cost{newDelay, extra, distance + *cell.DistanceM}
				if old, ok := routes[newMask]; !ok || less(value, old) {
					routes[newMask] = value
				}
				possible |= newMask
				// Do not prune by subset cost: different finishing times and last
				// locations can affect subsequent feasibility.
				walk(newMask, locations[o.LocationID], finish, [2]int64{remaining[0] - a, remaining[1] - b}, newDelay, value.distance)
			}
		}
		walk(0, locations[s.StartLocationID], max(s.AvailableFrom.Unix(), e.Shift.Start.Unix()), stock, 0, 0)
		next := map[int]cost{}
		for mask, a := range dp {
			for subset, b := range routes {
				if mask&subset != 0 {
					continue
				}
				value := cost{a.delay + b.delay, a.crews + b.crews, a.distance + b.distance}
				combined := mask | subset
				if old, ok := next[combined]; !ok || less(value, old) {
					next[combined] = value
				}
			}
		}
		dp = next
	}
	best := auditKey(in, 0, 0, int64(len(used)), 0)
	for mask, value := range dp {
		key := auditKey(in, mask, value.delay, value.crews, value.distance)
		if auditLess(key, best) {
			best = key
		}
	}
	return best, possible
}

// Check the exported plan itself, rather than trusting the solver's score.
func auditResult(t *testing.T, in c.SolveRequest, out c.SolveResult, possible int) auditCost {
	t.Helper()
	orders := map[string]int{}
	for i, o := range in.Orders {
		orders[o.ID] = i
	}
	engineers := map[string]c.Engineer{}
	for _, e := range in.Engineers {
		engineers[e.ID] = e
	}
	states := map[string]c.EngineerState{}
	for _, s := range in.EngineerStates {
		states[s.EngineerID] = s
	}
	locations := map[string]int{}
	for i, id := range in.TravelMatrix.LocationIDs {
		locations[id] = i
	}
	seen, routed, used, legs := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, id := range in.AlreadyUsedEngineerIDs {
		used[id] = true
	}
	var distance, delay int64
	mask := 0
	if out.Termination != c.TerminationCompleted && out.Termination != c.TerminationTimeLimit {
		t.Fatalf("bad termination: %s", out.Termination)
	}
	for _, r := range out.Routes {
		e, ok := engineers[r.EngineerID]
		if !ok || routed[r.EngineerID] {
			t.Fatalf("unknown/duplicate engineer: %s", r.EngineerID)
		}
		routed[r.EngineerID] = true
		s := states[e.ID]
		if len(r.Visits) == 0 || len(r.Visits) != len(r.Legs) {
			t.Fatal("empty route or missing leg")
		}
		if r.StartLocationID != s.StartLocationID || r.StartAt.Unix() != max(s.AvailableFrom.Unix(), e.Shift.Start.Unix()) {
			t.Fatal("bad route start")
		}
		used[e.ID] = true
		stock := map[c.Equipment]int64{}
		for k, n := range s.EquipmentAvailable {
			stock[k] = n
		}
		skills := map[string]bool{}
		for _, skill := range e.Skills {
			skills[skill] = true
		}
		location, available := s.StartLocationID, r.StartAt.Unix()
		for pos, v := range r.Visits {
			i, ok := orders[v.OrderID]
			if !ok || seen[v.OrderID] {
				t.Fatalf("unknown/duplicate order: %s", v.OrderID)
			}
			seen[v.OrderID] = true
			mask |= 1 << i
			o, l := in.Orders[i], r.Legs[pos]
			if l.ID == "" || legs[l.ID] {
				t.Fatal("empty/duplicate leg ID")
			}
			legs[l.ID] = true
			for _, skill := range o.RequiredSkills {
				if !skills[skill] {
					t.Fatal("skill violated")
				}
			}
			if o.RequiredTransport != nil && *o.RequiredTransport != e.Transport {
				t.Fatal("transport violated")
			}
			for k, n := range o.EquipmentRequired {
				stock[k] -= n
				if stock[k] < 0 {
					t.Fatal("equipment violated")
				}
			}
			cell := in.TravelMatrix.Profiles[e.Transport][locations[location]][locations[o.LocationID]]
			if !cell.Reachable {
				t.Fatal("unreachable arc")
			}
			if l.FromLocationID != location || l.ToLocationID != o.LocationID || l.GeoContextID != in.TravelMatrix.GeoContextID {
				t.Fatal("leg endpoints/context violated")
			}
			if l.StartAt.Unix() < max(available, o.ReceivedAt.Unix()) || l.EndAt.Unix()-l.StartAt.Unix() != *cell.DurationSec || l.DistanceM != *cell.DistanceM {
				t.Fatal("travel/release/distance violated")
			}
			if !v.ArrivalAt.Equal(l.EndAt) || v.StartAt.Before(v.ArrivalAt) || v.StartAt.Before(o.Window.Start) || v.StartAt.After(o.Window.End) || v.EndAt.Unix()-v.StartAt.Unix() != o.ServiceSec || v.EndAt.After(e.Shift.End) {
				t.Fatal("visit times violated")
			}
			distance += l.DistanceM
			if o.WorkType == c.WorkTypeEmergency {
				delay += v.StartAt.Unix() - max(o.Window.Start.Unix(), o.ReceivedAt.Unix())
			}
			location, available = o.LocationID, v.EndAt.Unix()
		}
	}
	for _, u := range out.Unassigned {
		i, ok := orders[u.OrderID]
		if !ok || seen[u.OrderID] || u.Message == "" {
			t.Fatal("missing/duplicate order or missing explanation")
		}
		seen[u.OrderID] = true
		switch u.ReasonCode {
		case c.ReasonNoMatchingSkill, c.ReasonNoMatchingTransport, c.ReasonNoMatchingEquipment, c.ReasonNoAvailableEngineer, c.ReasonNoReachableRoute, c.ReasonNoFeasibleSlot, c.ReasonNotAssignedBySolver:
		default:
			t.Fatalf("invalid unassignment reason: %s", u.ReasonCode)
		}
		if possible&(1<<i) != 0 && u.ReasonCode != c.ReasonNotAssignedBySolver {
			t.Fatalf("false impossibility claim for %s: %s", u.OrderID, u.ReasonCode)
		}
	}
	if len(seen) != len(orders) {
		t.Fatal("lost order")
	}
	return auditKey(in, mask, delay, int64(len(used)), distance)
}

func auditRequest(seed int64) c.SolveRequest {
	rng := rand.New(rand.NewSource(seed))
	base := time.Date(2026, 9, 28, 6, 0, 0, 0, time.FixedZone("audit", (rng.Intn(25)-12)*3600))
	at := func(seconds int64) time.Time { return base.Add(time.Duration(seconds) * time.Second) }
	n, m := 4+rng.Intn(4), 1+rng.Intn(3)
	in := c.SolveRequest{Mode: c.SolveModeOptimized, TimeLimitMS: 20, EmergencyFirst: seed%2 == 0, TravelMatrix: c.TravelMatrix{ID: "audit", GeoContextID: "audit", Profiles: map[c.Transport][][]c.TravelCell{}}}
	for i := 0; i <= n; i++ {
		in.TravelMatrix.LocationIDs = append(in.TravelMatrix.LocationIDs, fmt.Sprintf("p%d", i))
	}
	for _, mode := range []c.Transport{c.TransportCar, c.TransportWalk} {
		matrix := make([][]c.TravelCell, n+1)
		for i := range matrix {
			matrix[i] = make([]c.TravelCell, n+1)
			for j := range matrix[i] {
				if i != j && rng.Intn(12) == 0 {
					continue
				}
				duration, distance := int64(rng.Intn(2401)), int64(rng.Intn(12001))
				if i == j {
					duration, distance = 0, 0
				}
				matrix[i][j] = c.TravelCell{Reachable: true, DurationSec: &duration, DistanceM: &distance}
			}
		}
		in.TravelMatrix.Profiles[mode] = matrix
	}
	for i := 0; i < m; i++ {
		e := c.Engineer{ID: fmt.Sprintf("e%d", i), SourceOrder: int64(i), Available: true, Transport: c.TransportCar, Shift: c.Window{Start: at(0), End: at(int64(10800 + rng.Intn(18001)))}, EquipmentStock: map[c.Equipment]int64{c.EquipmentRouter: int64(rng.Intn(5)), c.EquipmentTVBox: int64(rng.Intn(4))}}
		if rng.Intn(3) == 0 {
			e.Transport = c.TransportWalk
		}
		for _, skill := range []string{"a", "b", "c"} {
			if rng.Intn(4) != 0 {
				e.Skills = append(e.Skills, skill)
			}
		}
		in.Engineers = append(in.Engineers, e)
		stock := map[c.Equipment]int64{}
		for k, v := range e.EquipmentStock {
			stock[k] = int64(rng.Intn(int(v) + 1))
		}
		in.EngineerStates = append(in.EngineerStates, c.EngineerState{EngineerID: e.ID, StartLocationID: fmt.Sprintf("p%d", rng.Intn(n+1)), AvailableFrom: at(int64(rng.Intn(7201))), EquipmentAvailable: stock})
		if rng.Intn(2) == 0 {
			in.AlreadyUsedEngineerIDs = append(in.AlreadyUsedEngineerIDs, e.ID)
		}
	}
	if seed%3 == 0 {
		in.AlreadyUsedEngineerIDs = append(in.AlreadyUsedEngineerIDs, "finished-crew")
	}
	for i := 0; i < n; i++ {
		start := int64(rng.Intn(18001))
		o := c.Order{ID: fmt.Sprintf("o%d", i), SourceOrder: int64(i), LocationID: fmt.Sprintf("p%d", 1+rng.Intn(n)), Status: c.OrderStatusActive, WorkType: []c.WorkType{c.WorkTypeEmergency, c.WorkTypeConnection, c.WorkTypeRepair, c.WorkTypeAdditional}[rng.Intn(4)], Priority: c.PriorityNormal, ReceivedAt: at(int64(rng.Intn(14401))), Window: c.Window{Start: at(start), End: at(start + int64(rng.Intn(14401)))}, ServiceSec: int64(1 + rng.Intn(1800)), EquipmentRequired: map[c.Equipment]int64{c.EquipmentRouter: int64(rng.Intn(3)), c.EquipmentTVBox: int64(rng.Intn(2))}}
		if o.WorkType == c.WorkTypeEmergency {
			o.Priority, o.ServiceSec = c.PriorityUrgent, 4800
		}
		for _, skill := range []string{"a", "b", "c"} {
			if rng.Intn(4) == 0 {
				o.RequiredSkills = append(o.RequiredSkills, skill)
			}
		}
		if rng.Intn(3) == 0 {
			mode := []c.Transport{c.TransportCar, c.TransportWalk}[rng.Intn(2)]
			o.RequiredTransport = &mode
		}
		in.Orders = append(in.Orders, o)
	}
	// A quarter of instances have abundant resources and broad windows. They
	// exercise route ordering and crew minimisation, rather than mostly testing
	// the rejection of incompatible jobs in the scarce-resource instances.
	if seed%4 == 0 {
		in.EmergencyFirst = seed%8 == 0
		for i := range in.Engineers {
			in.Engineers[i].Skills = []string{"a", "b", "c"}
			in.Engineers[i].EquipmentStock = map[c.Equipment]int64{c.EquipmentRouter: 100, c.EquipmentTVBox: 100}
			in.Engineers[i].Shift = c.Window{Start: at(0), End: at(36000)}
			in.EngineerStates[i].AvailableFrom = at(0)
			in.EngineerStates[i].EquipmentAvailable = map[c.Equipment]int64{c.EquipmentRouter: 100, c.EquipmentTVBox: 100}
		}
		for i := range in.Orders {
			in.Orders[i].RequiredTransport = nil
			in.Orders[i].ReceivedAt = at(0)
			in.Orders[i].Window = c.Window{Start: at(0), End: at(30000)}
		}
	}
	return in
}

func TestExhaustiveIndependentAudit(t *testing.T) {
	cases, budget := 40, int64(20)
	if s := os.Getenv("PLANNER_AUDIT_CASES"); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil || n < 1 {
			t.Fatal("invalid PLANNER_AUDIT_CASES")
		}
		cases = n
	}
	if s := os.Getenv("PLANNER_AUDIT_MS"); s != "" {
		n, e := strconv.ParseInt(s, 10, 64)
		if e != nil || n < 1 {
			t.Fatal("invalid PLANNER_AUDIT_MS")
		}
		budget = n
	}
	exact, gaps, over := 0, 0, 0
	var worst time.Duration
	for seed := int64(0); seed < int64(cases); seed++ {
		t.Run(fmt.Sprintf("seed-%d", seed), func(t *testing.T) {
			in := auditRequest(seed)
			in.TimeLimitMS = budget
			want, possible := auditExact(in)
			before, _ := json.Marshal(in)
			started := time.Now()
			out, err := optimized.New().Solve(context.Background(), in)
			elapsed := time.Since(started)
			worst = max(worst, elapsed)
			if elapsed > time.Duration(budget)*time.Millisecond+10*time.Millisecond {
				over++
			}
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(in)
			if string(before) != string(after) {
				t.Fatal("input mutated")
			}
			got := auditResult(t, in, out, possible)
			if os.Getenv("PLANNER_AUDIT_DUMP") == "1" {
				debug, _ := json.Marshal(map[string]any{"seed": seed, "input": in, "output": out, "optimum": want})
				t.Logf("CASE %s", debug)
			}
			if auditLess(got, want) {
				t.Fatalf("result beats exact oracle, invalid proof: got %v optimum %v", got, want)
			}
			if got == want {
				exact++
			} else {
				gaps++
				t.Logf("quality gap: got %v optimum %v", got, want)
			}
		})
	}
	t.Logf("AUDIT cases=%d requested=%d budget_ms=%d exact=%d gaps=%d over_budget_plus_10ms=%d max_ms=%d", exact+gaps, cases, budget, exact, gaps, over, worst.Milliseconds())
}

// A concrete witness for a local-search trap: a repair brings the crew to the
// location of a future emergency before that emergency is released. Travelling
// to that emergency directly after release misses its window. Choosing the
// other emergency first also consumes the equipment needed for the repair.
// The witness is feasible and globally optimal on the six primary metrics.
// This test checks the witness, not that a time-bounded heuristic must find it.
func TestAuditWitnessForReleasedEmergency(t *testing.T) {
	in := auditRequest(778)
	at := func(s string) time.Time {
		v, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	route := c.Route{
		EngineerID: "e0", StartLocationID: "p2", StartAt: in.EngineerStates[0].AvailableFrom,
		Visits: []c.Visit{
			{OrderID: "o0", ArrivalAt: at("2026-09-28T07:10:42+02:00"), StartAt: at("2026-09-28T07:34:51+02:00"), EndAt: at("2026-09-28T07:46:32+02:00")},
			{OrderID: "o4", ArrivalAt: at("2026-09-28T09:42:25+02:00"), StartAt: at("2026-09-28T09:42:25+02:00"), EndAt: at("2026-09-28T11:02:25+02:00")},
		},
		Legs: []c.Leg{
			{ID: "witness-1", FromLocationID: "p2", ToLocationID: "p1", StartAt: at("2026-09-28T06:58:07+02:00"), EndAt: at("2026-09-28T07:10:42+02:00"), DistanceM: 285, GeoContextID: "audit"},
			{ID: "witness-2", FromLocationID: "p1", ToLocationID: "p1", StartAt: at("2026-09-28T09:42:25+02:00"), EndAt: at("2026-09-28T09:42:25+02:00"), GeoContextID: "audit"},
		},
	}
	out := c.SolveResult{Routes: []c.Route{route}, Termination: c.TerminationCompleted}
	for _, id := range []string{"o1", "o2", "o3"} {
		out.Unassigned = append(out.Unassigned, c.UnassignedOrder{OrderID: id, ReasonCode: c.ReasonNotAssignedBySolver, Message: "Witness comparison"})
	}
	exact, possible := auditExact(in)
	got := auditResult(t, in, out, possible)
	if got != (auditCost{-1, 0, 0, -1, 1, 285}) || got != exact {
		t.Fatalf("witness is not optimal: witness=%v exact=%v", got, exact)
	}
}
