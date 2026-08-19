package main

import (
	"context"
	"fmt"

	"github.com/Jtensetti/nomad-browser/browser"
	"github.com/Jtensetti/nomad-local-reconstruction/reconstruct"
	"github.com/Jtensetti/nomad-selection-firewall/firewall"
	"github.com/Jtensetti/nomad-semantic-basins/basin"
)

func main() {
	e, err := browser.New(basin.HashEmbedder{Dims: 512}, basin.Quantizer{}, firewall.NetworkConfig{CellsPerEpoch: 16, CellSize: 1200, PeerSlots: 4})
	if err != nil {
		panic(err)
	}
	candidates := []reconstruct.Candidate{{Basin: 0, Score: .1}, {Basin: ^uint64(0), Score: .9}}
	ranked, err := e.SearchLocal(context.Background(), "privacy preserving distributed web", candidates)
	if err != nil {
		panic(err)
	}
	plan, err := e.EmissionPlan(1)
	if err != nil {
		panic(err)
	}
	fmt.Printf("private basin=%016x ranked=%d observable_cells=%d\n", e.PrivateTargetBasin(), len(ranked), len(plan))
}
