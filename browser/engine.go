package browser

import (
	"context"
	"errors"

	"github.com/Jtensetti/nomad-local-reconstruction/reconstruct"
	"github.com/Jtensetti/nomad-selection-firewall/firewall"
	"github.com/Jtensetti/nomad-semantic-basins/basin"
)

// Engine deliberately separates public network scheduling state from private
// user intent. No method that accepts a query can mutate NetworkConfig.
type Engine struct {
	embedder  basin.Embedder
	quantizer basin.Quantizer
	network   firewall.NetworkConfig
	private   privateState
}

type privateState struct {
	query       string
	targetBasin uint64
	ranked      []reconstruct.Candidate
}

func New(embedder basin.Embedder, q basin.Quantizer, network firewall.NetworkConfig) (*Engine, error) {
	if embedder == nil {
		return nil, errors.New("embedder required")
	}
	if err := network.Validate(); err != nil {
		return nil, err
	}
	return &Engine{embedder: embedder, quantizer: q, network: network}, nil
}

// SearchLocal transforms intent into a basin and ranks already-known opaque
// candidates. It performs no network I/O and cannot alter emission planning.
func (e *Engine) SearchLocal(ctx context.Context, query string, candidates []reconstruct.Candidate) ([]reconstruct.Candidate, error) {
	v, err := e.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	b, err := e.quantizer.Basin(v)
	if err != nil {
		return nil, err
	}
	ranked := reconstruct.Rank(candidates, b)
	e.private.query = query
	e.private.targetBasin = b
	e.private.ranked = append([]reconstruct.Candidate(nil), ranked...)
	return append([]reconstruct.Candidate(nil), ranked...), nil
}

func (e *Engine) EmissionPlan(epoch uint64) ([]firewall.Emission, error) {
	return firewall.Plan(e.network, epoch)
}

func (e *Engine) PrivateTargetBasin() uint64 { return e.private.targetBasin }
