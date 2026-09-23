package baseline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/baseline"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
)

func TestBaselineConcurrentCalls(t *testing.T) {
	input := testutil.BaseRequest()
	solver := baseline.New()
	want, err := solver.Solve(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := solver.Solve(context.Background(), input)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("concurrent Solve = %+v, %v; want %+v", got, err, want)
			}
		}()
	}
	wg.Wait()
}

// Generate small synthetic, directed, non-metric problems and check the public
// contract independently of the route-construction implementation.
func FuzzBaselineFeasibility(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4})
	f.Add([]byte{255, 0, 127, 60, 90, 5})
	f.Add([]byte{0})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) == 0 {
			return
		}
		byteAt := func(i int) int { return int(data[i%len(data)]) }
		count := 1 + byteAt(0)%12
		input := testutil.BaseRequest()
		input.TimeLimitMS = 60000
		input.Orders = nil
		locations := []string{"depot"}
		for i := 0; i < count; i++ {
			id := fmt.Sprintf("job-%d", i)
			locations = append(locations, id)
			job := testutil.Order(id, id, int64(byteAt(i+1)%4), 7, byteAt(i+2)%60, 1+byteAt(i+3)%90, 0)
			job.Window.End = job.Window.Start.Add(time.Duration(byteAt(i+4)%120) * time.Minute)
			job.ReceivedAt = testutil.At(6, byteAt(i+5)%240)
			job.EquipmentRequired = map[contracts.Equipment]int64{contracts.EquipmentRouter: int64(byteAt(i+6) % 3)}
			if byteAt(i+7)%5 == 0 {
				job.RequiredSkills = []string{"repair", "installation"}
			}
			input.Orders = append(input.Orders, job)
		}
		input.TravelMatrix = testutil.Matrix(locations, contracts.TransportCar)
		for i, row := range input.TravelMatrix.Profiles[contracts.TransportCar] {
			for j := range row {
				if i == j {
					continue
				}
				x := byteAt(i*count + j + 5)
				if x%7 == 0 {
					row[j] = contracts.TravelCell{}
				} else {
					row[j] = contracts.TravelCell{Reachable: true, DurationSec: testutil.Ptr(int64(x * 20)), DistanceM: testutil.Ptr(int64(x * 100))}
				}
			}
		}
		before, _ := json.Marshal(input)
		got, err := solve(t, input)
		if err != nil {
			t.Fatal(err)
		}
		after, _ := json.Marshal(input)
		if string(before) != string(after) {
			t.Fatal("input changed")
		}
		jobs := make(map[string]contracts.Order, count)
		indices := make(map[string]int, count+1)
		for _, job := range input.Orders {
			jobs[job.ID] = job
		}
		for i, id := range locations {
			indices[id] = i
		}
		seen := map[string]int{}
		legIDs := map[string]bool{}
		for _, route := range got.Routes {
			if route.EngineerID != "e1" || len(route.Visits) != len(route.Legs) || len(route.Visits) == 0 {
				t.Fatalf("invalid route: %+v", route)
			}
			previousEnd, previousLocation := testutil.At(6, 0), "depot"
			var equipmentUsed int64
			for i, visit := range route.Visits {
				job, ok := jobs[visit.OrderID]
				if !ok {
					t.Fatalf("unknown order: %s", visit.OrderID)
				}
				seen[job.ID]++
				leg := route.Legs[i]
				if legIDs[leg.ID] || leg.ID == "" {
					t.Fatalf("duplicate/empty leg ID: %s", leg.ID)
				}
				legIDs[leg.ID] = true
				cell := input.TravelMatrix.Profiles[contracts.TransportCar][indices[previousLocation]][indices[job.LocationID]]
				if !cell.Reachable || leg.FromLocationID != previousLocation || leg.ToLocationID != job.LocationID ||
					leg.DistanceM != *cell.DistanceM || leg.EndAt.Sub(leg.StartAt) != time.Duration(*cell.DurationSec)*time.Second ||
					leg.StartAt.Before(previousEnd) || leg.StartAt.Before(job.ReceivedAt) || !visit.ArrivalAt.Equal(leg.EndAt) {
					t.Fatalf("invalid travel: %+v", leg)
				}
				if visit.StartAt.Before(visit.ArrivalAt) || visit.StartAt.Before(job.Window.Start) || visit.StartAt.After(job.Window.End) ||
					visit.EndAt.Sub(visit.StartAt) != time.Duration(job.ServiceSec)*time.Second || visit.EndAt.After(input.Engineers[0].Shift.End) {
					t.Fatalf("invalid service: %+v", visit)
				}
				if len(job.RequiredSkills) != 1 || job.RequiredSkills[0] != "repair" {
					t.Fatalf("assigned without required skills: %+v", job)
				}
				if i > 0 {
					previous := jobs[route.Visits[i-1].OrderID]
					if previous.SourceOrder > job.SourceOrder || (previous.SourceOrder == job.SourceOrder && previous.ID > job.ID) {
						t.Fatal("baseline reordered visits")
					}
				}
				equipmentUsed += job.EquipmentRequired[contracts.EquipmentRouter]
				previousEnd, previousLocation = visit.EndAt, job.LocationID
			}
			if equipmentUsed > input.EngineerStates[0].EquipmentAvailable[contracts.EquipmentRouter] {
				t.Fatal("equipment overbooked")
			}
		}
		for _, item := range got.Unassigned {
			seen[item.OrderID]++
			if item.ReasonCode == "" || item.Message == "" {
				t.Fatal("missing unassigned explanation")
			}
		}
		if len(seen) != len(jobs) {
			t.Fatalf("incomplete result: %v", seen)
		}
		for id := range jobs {
			if seen[id] != 1 {
				t.Fatalf("order %s occurs %d times", id, seen[id])
			}
		}
	})
}
