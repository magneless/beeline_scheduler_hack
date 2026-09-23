//go:build ortools

package optimized

import (
	cs "github.com/airspacetechnologies/or-tools/go/ortools/constraintsolver"
	"google.golang.org/protobuf/proto"
	"testing"
)

func TestRoutingParameterAdapterPreservesNativeDefaults(t *testing.T) {
	search := cs.DefaultRoutingSearchParameters()
	freshSearch := routingSearchParameters(&search)
	if !proto.Equal(&search, &freshSearch) {
		t.Fatal("parameter adapter discarded native search defaults")
	}
	model := cs.DefaultRoutingModelParameters()
	freshModel := routingModelParameters(&model)
	if !proto.Equal(&model, &freshModel) {
		t.Fatal("parameter adapter discarded native model defaults")
	}
}
