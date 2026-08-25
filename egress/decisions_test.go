package egress

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// PROD-16 asks that executable-content decisions are pinned by a corpus rather
// than by the tests of the code that makes them. The difference is not
// pedantry: a test describes what this implementation does, and changes when
// it does, so a rule that quietly loosens takes its own test with it. A corpus
// is data. Loosening the rule then fails against a file somebody has to edit
// deliberately, and a second implementation can check itself against the same
// file without reading any Go.
//
// testdata/url-decisions.json is that file. Every entry carries the reason the
// case matters, because a decision table nobody can read is a decision table
// nobody will maintain.
type urlDecision struct {
	URL   string `json:"url"`
	Allow bool   `json:"allow"`
	Why   string `json:"why"`
}

type decisionCorpus struct {
	Version     string        `json:"version"`
	Description string        `json:"description"`
	Cases       []urlDecision `json:"cases"`
}

const decisionCorpusVersion = "nomad-browser-url-decisions-v1"

func loadDecisions(t *testing.T) decisionCorpus {
	t.Helper()
	encoded, err := os.ReadFile("testdata/url-decisions.json")
	if err != nil {
		t.Fatal(err)
	}
	var corpus decisionCorpus
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&corpus); err != nil {
		t.Fatal(err)
	}
	if corpus.Version != decisionCorpusVersion {
		t.Fatalf("corpus version %q, want %q: an unrecognised version is refused "+
			"rather than read as if it were this one", corpus.Version, decisionCorpusVersion)
	}
	return corpus
}

func TestTheRendererGateMatchesTheFrozenDecisions(t *testing.T) {
	corpus := loadDecisions(t)
	var policy Policy
	allowed, refused := 0, 0
	for _, decision := range corpus.Cases {
		err := policy.CheckRendererURL(decision.URL)
		switch {
		case decision.Allow && err != nil:
			t.Errorf("%q was refused (%v) but the corpus allows it: %s",
				decision.URL, err, decision.Why)
		case !decision.Allow && err == nil:
			t.Errorf("%q was allowed but the corpus refuses it: %s",
				decision.URL, decision.Why)
		case !decision.Allow && !errors.Is(err, ErrDenied):
			t.Errorf("%q was refused with %v, which does not carry ErrDenied, so a "+
				"caller cannot tell a policy refusal from a bug", decision.URL, err)
		}
		if decision.Allow {
			allowed++
		} else {
			refused++
		}
	}

	// A corpus that only refuses would pass against a gate that refuses
	// everything, and a corpus that only allows would pass against one that
	// allows everything. Both directions have to be represented.
	if allowed < 10 || refused < 20 {
		t.Fatalf("the corpus has %d allowed and %d refused cases; it must exercise "+
			"both directions to mean anything", allowed, refused)
	}
	t.Logf("%d frozen decisions: %d allowed, %d refused", len(corpus.Cases), allowed, refused)
}

// The allowlist and the corpus have to agree, or one of them is stale. This
// catches a media type added to the code and not to the corpus, which is
// exactly how a scriptable type would arrive unnoticed.
func TestEveryAllowedMediaTypeIsInTheFrozenDecisions(t *testing.T) {
	corpus := loadDecisions(t)
	covered := map[string]bool{}
	for _, decision := range corpus.Cases {
		if !decision.Allow || !strings.HasPrefix(decision.URL, "data:") {
			continue
		}
		body := strings.TrimPrefix(decision.URL, "data:")
		mediaType, _, _ := strings.Cut(body, ",")
		mediaType, _, _ = strings.Cut(mediaType, ";")
		covered[strings.ToLower(mediaType)] = true
	}
	for _, mediaType := range AllowedDataMediaTypes() {
		if !covered[strings.ToLower(mediaType)] {
			t.Errorf("%s is allowed by the code and has no case in the frozen "+
				"decisions: a media type can be added without anyone deciding it "+
				"is non-scriptable", mediaType)
		}
	}
}

// And the reverse: a case in the corpus naming a type the code no longer
// allows means the corpus is describing a policy that is gone.
func TestTheFrozenDecisionsNameNoMediaTypeTheCodeDropped(t *testing.T) {
	corpus := loadDecisions(t)
	known := map[string]bool{}
	for _, mediaType := range AllowedDataMediaTypes() {
		known[strings.ToLower(mediaType)] = true
	}
	for _, decision := range corpus.Cases {
		if !decision.Allow || !strings.HasPrefix(decision.URL, "data:") {
			continue
		}
		body := strings.TrimPrefix(decision.URL, "data:")
		mediaType, _, _ := strings.Cut(body, ",")
		mediaType, _, _ = strings.Cut(mediaType, ";")
		if mediaType == "" {
			continue
		}
		if !known[strings.ToLower(mediaType)] {
			t.Errorf("the frozen decisions allow %s, which the code no longer does",
				mediaType)
		}
	}
}

// Every case must carry its reason. A decision table without them is one
// nobody can review, and reviewing it is the only way a wrong entry is caught.
func TestEveryFrozenDecisionSaysWhyItMatters(t *testing.T) {
	corpus := loadDecisions(t)
	seen := map[string]bool{}
	for _, decision := range corpus.Cases {
		if strings.TrimSpace(decision.Why) == "" {
			t.Errorf("%q has no reason recorded", decision.URL)
		}
		if seen[decision.URL] {
			t.Errorf("%q appears twice; one of the two entries is unreachable", decision.URL)
		}
		seen[decision.URL] = true
	}
}
