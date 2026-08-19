package browser

import (
	"context"
	"reflect"
	"testing"

	"github.com/Jtensetti/nomad-local-reconstruction/reconstruct"
	"github.com/Jtensetti/nomad-selection-firewall/firewall"
	"github.com/Jtensetti/nomad-semantic-basins/basin"
)

func TestPrivateSearchCannotChangeNetworkPlan(t *testing.T) {
	cfg := firewall.NetworkConfig{CellsPerEpoch: 16, CellSize: 1200, PeerSlots: 4}
	e, err := New(basin.HashEmbedder{Dims: 512}, basin.Quantizer{}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.EmissionPlan(42)
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.SearchLocal(context.Background(), "vapensystem i irans militär", []reconstruct.Candidate{{Basin: 1, Score: .5}, {Basin: 2, Score: .8}})
	if err != nil {
		t.Fatal(err)
	}
	after, err := e.EmissionPlan(42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("private search changed externally observable plan")
	}
}

func TestDifferentQueriesSamePlan(t *testing.T) {
	cfg := firewall.NetworkConfig{CellsPerEpoch: 8, CellSize: 1200, PeerSlots: 3}
	a, _ := New(basin.HashEmbedder{Dims: 256}, basin.Quantizer{}, cfg)
	b, _ := New(basin.HashEmbedder{Dims: 256}, basin.Quantizer{}, cfg)
	_, _ = a.SearchLocal(context.Background(), "Iran military", nil)
	_, _ = b.SearchLocal(context.Background(), "sourdough pizza", nil)
	pa, _ := a.EmissionPlan(999)
	pb, _ := b.EmissionPlan(999)
	if !reflect.DeepEqual(pa, pb) {
		t.Fatal("query influenced plan")
	}
}
