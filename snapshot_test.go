package nomadbrowser_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// COMPONENTS.sha256 pins the vendored snapshots. CI checks it with
// `sha256sum --check`, which answers only "does every listed file still hash
// to what it says" -- never "is every shipped file listed", because a file
// absent from the manifest is a file sha256sum never looks at.
//
// nomad-testnet had exactly this gap: seventeen of forty-six vendored files
// were unlisted, including the RLNC budget enforcement its materializer relies
// on. This repository's manifest happens to be complete today. Nothing was
// keeping it that way.
const componentManifest = "COMPONENTS.sha256"

func TestSnapshotManifestPinsEveryVendoredFile(t *testing.T) {
	pinned := readComponentManifest(t)
	shipped := walkComponents(t)

	present := map[string]struct{}{}
	var unpinned []string
	for _, path := range shipped {
		present[path] = struct{}{}
		if _, listed := pinned[path]; !listed {
			unpinned = append(unpinned, path)
		}
	}
	var missing []string
	for path := range pinned {
		if _, exists := present[path]; !exists {
			missing = append(missing, path)
		}
	}
	sort.Strings(unpinned)
	sort.Strings(missing)

	if len(unpinned) > 0 {
		t.Errorf("%d vendored file(s) ship unpinned, so an edit to them passes the "+
			"supply-chain check:\n  %s", len(unpinned), strings.Join(unpinned, "\n  "))
	}
	if len(missing) > 0 {
		t.Errorf("%d manifest entr(ies) name a file that no longer ships:\n  %s",
			len(missing), strings.Join(missing, "\n  "))
	}
}

func TestSnapshotManifestDigestsMatchWhatShips(t *testing.T) {
	pinned := readComponentManifest(t)
	for _, path := range walkComponents(t) {
		expected, listed := pinned[path]
		if !listed {
			continue // reported by the completeness test
		}
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		actual := sha256.Sum256(content)
		if hex.EncodeToString(actual[:]) != expected {
			t.Errorf("%s: ships as %x, manifest pins %s", path, actual[:8], expected[:16])
		}
	}
}

func readComponentManifest(t *testing.T) map[string]string {
	t.Helper()
	encoded, err := os.ReadFile(componentManifest)
	if err != nil {
		t.Fatal(err)
	}
	pinned := map[string]string{}
	for number, line := range strings.Split(strings.TrimSpace(string(encoded)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[0]) != 2*sha256.Size {
			t.Fatalf("%s line %d is not a sha256sum entry: %q", componentManifest, number+1, line)
		}
		if previous, repeated := pinned[fields[1]]; repeated {
			// A repeated path lets one entry satisfy the checker while the
			// other silently disagrees about what ships.
			t.Fatalf("%s pins %s twice (%s and %s)",
				componentManifest, fields[1], previous[:16], fields[0][:16])
		}
		pinned[fields[1]] = fields[0]
	}
	return pinned
}

func walkComponents(t *testing.T) []string {
	t.Helper()
	var shipped []string
	err := filepath.WalkDir("components", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			shipped = append(shipped, filepath.ToSlash(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(shipped) < 10 {
		t.Fatalf("found only %d vendored files, so the tree was not walked as expected",
			len(shipped))
	}
	sort.Strings(shipped)
	return shipped
}
