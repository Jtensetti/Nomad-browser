package nomadbrowser_test

import (
	"bufio"
	"os/exec"
	"strings"
	"testing"
)

func dependencies(t *testing.T, pkg string) map[string]bool {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", pkg)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list %s: %v", pkg, err)
	}
	deps := map[string]bool{}
	s := bufio.NewScanner(strings.NewReader(string(out)))
	for s.Scan() {
		deps[s.Text()] = true
	}
	if err := s.Err(); err != nil {
		t.Fatal(err)
	}
	return deps
}

func TestSelectorDependencyGraphHasNoNetworkPlanner(t *testing.T) {
	deps := dependencies(t, "./selector")
	for _, forbidden := range []string{
		"github.com/Jtensetti/nomad-selection-firewall/firewall",
		"github.com/Jtensetti/nomad-constant-rate-fabric/fabric",
	} {
		if deps[forbidden] {
			t.Fatalf("selector depends on network package %s", forbidden)
		}
	}
}

func TestPlannerDependencyGraphHasNoPrivateSelectionPackages(t *testing.T) {
	deps := dependencies(t, "./planner")
	for _, forbidden := range []string{
		"github.com/Jtensetti/nomad-semantic-basins/basin",
		"github.com/Jtensetti/nomad-local-reconstruction/reconstruct",
	} {
		if deps[forbidden] {
			t.Fatalf("planner depends on private-selection package %s", forbidden)
		}
	}
}

func TestVerifiedResourcePathHasNoNetworkPlannerOrSemanticQuery(t *testing.T) {
	for _, pkg := range []string{"./adapter", "./localcache"} {
		deps := dependencies(t, pkg)
		for _, forbidden := range []string{
			"github.com/Jtensetti/nomad-selection-firewall/firewall",
			"github.com/Jtensetti/nomad-constant-rate-fabric/fabric",
			"github.com/Jtensetti/nomad-semantic-basins/basin",
		} {
			if deps[forbidden] {
				t.Fatalf("%s depends on forbidden package %s", pkg, forbidden)
			}
		}
	}
}

// An updater that fetches is a network client inside the binary that is
// supposed to have none, and every guarantee downstream of "this process
// cannot open a socket" would then rest on that one component behaving.
//
// So the update package verifies bytes the user already has and downloads
// nothing. That division is enforced here rather than left as an intention:
// the package must not be able to reach a socket, launch a process, or read
// the private selection side.
func TestUpdateVerifierHasNoNetworkOrProcessCapability(t *testing.T) {
	deps := dependencies(t, "./update")
	for _, forbidden := range []string{
		"net",
		"net/http",
		"net/url",
		"os/exec",
		"github.com/Jtensetti/nomad-constant-rate-fabric/fabric",
		"github.com/Jtensetti/nomad-semantic-basins/basin",
	} {
		if deps[forbidden] {
			t.Errorf("the update verifier reaches %s, so it could fetch a release "+
				"rather than only checking one", forbidden)
		}
	}
}
