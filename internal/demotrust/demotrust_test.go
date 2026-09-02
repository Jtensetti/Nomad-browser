package demotrust

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Swift client anchors its publisher as a literal. If someone rotates it
// there, every Go test would keep verifying against the retired key and stay
// green while the two implementations trusted different publishers.
func TestTheAnchorMatchesTheOneTheSwiftClientCompilesIn(t *testing.T) {
	path := filepath.Join("..", "..", "macos", "Sources", "NomadBrowser", "Models.swift")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the Swift anchor: %v", err)
	}
	const marker = "trustedDemoPublisher"
	line := ""
	for _, candidate := range strings.Split(string(source), "\n") {
		if strings.Contains(candidate, marker) {
			line = candidate
			break
		}
	}
	if line == "" {
		t.Fatalf("%s no longer declares %s, so this check proves nothing", path, marker)
	}
	if !strings.Contains(line, PublisherKey) {
		t.Fatalf("the Swift client anchors a different publisher:\n\t%s\nGo anchors %s",
			strings.TrimSpace(line), PublisherKey)
	}
}
