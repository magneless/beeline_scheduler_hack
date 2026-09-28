package optimized

import (
	"context"
	c "github.com/magneless/beeline_scheduler_hack/backend/internal/contracts"
	"github.com/magneless/beeline_scheduler_hack/backend/internal/planner/internal/testutil"
	"strings"
	"testing"
	"time"
)

func TestUnassignedExplainsStockCommittedToOtherVisits(t *testing.T) {
	in := testutil.BaseRequest()
	first := in.Orders[0]
	first.EquipmentRequired = map[c.Equipment]int64{c.EquipmentRouter: 2}
	second := first
	second.ID = "o2"
	second.SourceOrder = 2
	in.Orders = []c.Order{first, second}
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.materialize([][]int{{0}})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.explain(context.Background(), &result, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if len(result.Unassigned) != 1 || !strings.Contains(result.Unassigned[0].Message, "оборудования после назначений — 1") {
		t.Fatalf("missing selected-plan reason: %+v", result.Unassigned)
	}
}

func TestUnassignedDoesNotInventInfeasibilityForUsableSlot(t *testing.T) {
	in := testutil.BaseRequest()
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.materialize([][]int{{}})
	if err != nil {
		t.Fatal(err)
	}
	if err = p.explain(context.Background(), &result, time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Unassigned[0].Message, "есть допустимое место") {
		t.Fatalf("invented reason: %+v", result.Unassigned)
	}
}

func TestFinalInsertionUsesGapsWithoutExceedingEquipment(t *testing.T) {
	in := testutil.BaseRequest()
	first := in.Orders[0]
	first.EquipmentRequired = map[c.Equipment]int64{c.EquipmentRouter: 1}
	second := first
	second.ID = "o2"
	second.SourceOrder = 2
	third := first
	third.ID = "o3"
	third.SourceOrder = 3
	in.Orders = []c.Order{first, second, third}
	p, err := prepareRouting(in)
	if err != nil {
		t.Fatal(err)
	}
	result, err := p.materialize([][]int{{0}})
	if err != nil {
		t.Fatal(err)
	}
	result, err = p.fillAvailableSlots(context.Background(), result, time.Now().Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Routes) != 1 || len(result.Routes[0].Visits) != 2 || len(result.Unassigned) != 1 {
		t.Fatalf("did not fill exactly the usable capacity: %+v", result)
	}
}
