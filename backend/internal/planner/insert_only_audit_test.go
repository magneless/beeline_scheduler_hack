package planner

import (
	"context"
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"time"

	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
)

// Independent arithmetic decision oracle for all three insertion positions;
// it neither calls insertAt nor Worker.AppendCandidate.
func TestInsertOnlyRandomGapsAudit(t *testing.T) {
	for seed := int64(0); seed < 2000; seed++ {
		rng := rand.New(rand.NewSource(seed))
		in := ioReq(1000)
		r := &in.FixedRoutes[0]
		r.Legs[0].StartAt = r.Visits[0].ArrivalAt.Add(-10 * time.Minute)
		if seed%2 == 0 {
			in.ProtectedLegIDs = nil
		}
		second := in.Orders[0]
		second.ID = "old-second"
		in.Orders = append(in.Orders, second)
		r.Visits = append(r.Visits, c.Visit{OrderID: second.ID, ArrivalAt: ioAt(9), StartAt: ioAt(9), EndAt: ioAt(9).Add(30 * time.Minute)})
		r.Legs = append(r.Legs, c.Leg{ID: "second-leg", FromLocationID: "old", ToLocationID: "old", StartAt: ioAt(9), EndAt: ioAt(9), GeoContextID: "g"})
		fresh := &in.Orders[1]
		fresh.ServiceSec = int64(1 + rng.Intn(7200))
		fresh.ReceivedAt = ioAt(6).Add(time.Duration(rng.Intn(18001)) * time.Second)
		fresh.Window.Start = ioAt(6).Add(time.Duration(rng.Intn(18001)) * time.Second)
		fresh.Window.End = fresh.Window.Start.Add(time.Duration(rng.Intn(7201)) * time.Second)
		in.Engineers[0].Shift.End = ioAt(10).Add(time.Duration(rng.Intn(28801)) * time.Second)
		stock, need := int64(rng.Intn(3)), int64(rng.Intn(3))
		in.Engineers[0].EquipmentStock = map[c.Equipment]int64{c.EquipmentRouter: stock}
		in.EngineerStates[0].EquipmentAvailable = map[c.Equipment]int64{c.EquipmentRouter: stock}
		fresh.EquipmentRequired = map[c.Equipment]int64{c.EquipmentRouter: need}
		matrix := in.TravelMatrix.Profiles[c.TransportCar]
		for _, pair := range [][2]int{{0, 2}, {1, 2}, {2, 1}} {
			if rng.Intn(8) == 0 {
				matrix[pair[0]][pair[1]] = c.TravelCell{}
				continue
			}
			d := int64(rng.Intn(3601))
			distance := int64(rng.Intn(10001))
			matrix[pair[0]][pair[1]] = c.TravelCell{Reachable: true, DurationSec: &d, DistanceM: &distance}
		}
		possible := false
		if need <= stock {
			for pos := 0; pos <= 2; pos++ {
				if pos == 0 && len(in.ProtectedLegIDs) > 0 {
					continue
				}
				loc, ready := 0, r.StartAt.Unix()
				if pos > 0 {
					loc = 1
					ready = r.Visits[pos-1].EndAt.Unix()
				}
				cell := matrix[loc][2]
				if !cell.Reachable {
					continue
				}
				start := max(max(ready, fresh.ReceivedAt.Unix())+*cell.DurationSec, fresh.Window.Start.Unix())
				end := start + fresh.ServiceSec
				if start > fresh.Window.End.Unix() || end > in.Engineers[0].Shift.End.Unix() {
					continue
				}
				if pos < 2 {
					back := matrix[2][1]
					if !back.Reachable {
						continue
					}
					departure := r.Visits[pos].ArrivalAt.Unix() - *back.DurationSec
					if departure < end || departure < second.ReceivedAt.Unix() {
						continue
					}
				}
				possible = true
			}
		}
		before, _ := json.Marshal(in)
		out, err := NewInserter().Solve(context.Background(), in)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		after, _ := json.Marshal(in)
		if string(before) != string(after) {
			t.Fatalf("seed %d input mutated", seed)
		}
		if (len(out.Unassigned) == 0) != possible {
			t.Fatalf("seed %d: existence differs from oracle: possible=%v result=%+v", seed, possible, out)
		}
		if len(in.ProtectedLegIDs) > 0 && !reflect.DeepEqual(out.Routes[0].Legs[0], r.Legs[0]) {
			t.Fatalf("seed %d changed protected leg", seed)
		}
		for _, old := range r.Visits {
			found := false
			for _, v := range out.Routes[0].Visits {
				if v.OrderID == old.OrderID {
					found = true
					if v != old {
						t.Fatalf("seed %d moved fixed visit", seed)
					}
				}
			}
			if !found {
				t.Fatalf("seed %d lost fixed visit", seed)
			}
		}
	}
}
