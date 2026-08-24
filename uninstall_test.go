package nomadbrowser_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The classic uninstaller bug is not a wrong path, it is a stale one: the
// application grows a new place to write and the uninstaller keeps removing
// yesterday's list. Nothing about a shell script notices that, and nobody
// re-reads an uninstaller.
//
// So this reads both sides. Every directory name the Swift sources construct
// must appear in the uninstall script, and the script must still remove the
// sandbox container, which under com.apple.security.app-sandbox is where all
// of them actually live.
//
// It is a consistency check, not evidence that uninstalling works: the script
// has never been run against a real installed bundle by this project. See
// docs/DATA_RETENTION.md, which says so, and H-10.

const uninstallScript = "macos/scripts/uninstall.sh"
const retentionDoc = "docs/DATA_RETENTION.md"

// appendedPathComponent finds the literal directory names the Swift sources
// build with appendingPathComponent.
var appendedPathComponent = regexp.MustCompile(`appendingPathComponent\("([^"]+)"`)

func TestTheUninstallerCoversEveryDirectoryTheAppConstructs(t *testing.T) {
	sources := swiftSources(t, swiftSourceRoot)
	script := readFile(t, uninstallScript)

	constructed := map[string]string{}
	for _, source := range sources {
		text := readFile(t, source)
		for _, match := range appendedPathComponent.FindAllStringSubmatch(text, -1) {
			name := match[1]
			// A file name rather than a directory, or a name built from a
			// value, is not an install location.
			if strings.Contains(name, ".") || strings.Contains(name, "\\(") {
				continue
			}
			constructed[name] = source
		}
	}
	if len(constructed) == 0 {
		t.Fatalf("found no constructed directory names under %s, so this check "+
			"compared nothing", swiftSourceRoot)
	}

	for name, source := range constructed {
		if !strings.Contains(script, name) {
			t.Errorf("%s constructs the directory %q and %s never mentions it, so "+
				"uninstalling leaves it behind", source, name, uninstallScript)
		}
	}
}

// The container is the whole guarantee that section 1 of the retention
// document makes: under the sandbox every standard directory API resolves
// inside it, so removing it removes everything the app stored. A script that
// stopped removing it would leave that claim false.
func TestTheUninstallerRemovesTheSandboxContainer(t *testing.T) {
	script := readFile(t, uninstallScript)
	for _, required := range []string{
		"Library/Containers/$bundle_id",
		`bundle_id="io.nomad.browser"`,
	} {
		if !strings.Contains(script, required) {
			t.Errorf("%s no longer contains %q", uninstallScript, required)
		}
	}

	// The bundle identifier must agree with Info.plist, or the script removes
	// some other application's container and none of this one's.
	info := readFile(t, infoPlistPath)
	identifier := regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`).
		FindStringSubmatch(info)
	if identifier == nil {
		t.Fatalf("%s declares no CFBundleIdentifier", infoPlistPath)
	}
	if !strings.Contains(script, `bundle_id="`+identifier[1]+`"`) {
		t.Errorf("%s uses a bundle identifier that is not %s's %q",
			uninstallScript, infoPlistPath, identifier[1])
	}
}

// An uninstaller that quietly leaves things behind is worse than one that says
// what it cannot remove. The script prints that list; this keeps it from being
// trimmed to make the output tidier.
func TestTheUninstallerStatesWhatItCannotRemove(t *testing.T) {
	script := readFile(t, uninstallScript)
	for name, marker := range map[string]string{
		"crash reports":   "DiagnosticReports",
		"local snapshots": "deletelocalsnapshot",
		"Spotlight index": "mdutil",
		"unified log":     "log entries record that the app launched",
		"swap":            "hibernation image",
	} {
		if !strings.Contains(script, marker) {
			t.Errorf("%s no longer tells the user about %s", uninstallScript, name)
		}
	}

	// And the retention document must carry the same list, since the script
	// points at it for the reasoning.
	retention := readFile(t, retentionDoc)
	for _, required := range []string{
		"DiagnosticReports",
		"deletelocalsnapshot",
		"mdutil",
		"FileVault",
		"Library/Containers/io.nomad.browser",
	} {
		if !strings.Contains(retention, required) {
			t.Errorf("%s no longer covers %q", retentionDoc, required)
		}
	}

	// The document must keep saying what has not been verified. Dropping that
	// sentence would turn a consistency check into a claim of a working
	// uninstall, which nobody here has observed.
	if !strings.Contains(retention, "has not been executed on a real") {
		t.Errorf("%s no longer records that the uninstall procedure is unverified "+
			"on a real macOS install", retentionDoc)
	}
}

func TestTheUninstallScriptRefusesToRunWhileTheAppIsOpen(t *testing.T) {
	script := readFile(t, uninstallScript)
	if !strings.Contains(script, "pgrep -x NomadBrowser") {
		t.Errorf("%s no longer checks whether the app is running; removing the "+
			"container under a live process lets it recreate what was deleted",
			uninstallScript)
	}
	if !strings.Contains(script, "set -euo pipefail") {
		t.Errorf("%s does not fail on error, so a partial uninstall would report "+
			"success", uninstallScript)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	encoded, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
