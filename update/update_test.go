package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fixture struct {
	trusted   ed25519.PublicKey
	key       ed25519.PrivateKey
	artifact  []byte
	directory string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	artifact := make([]byte, 4096)
	if _, err := rand.Read(artifact); err != nil {
		t.Fatal(err)
	}
	return &fixture{trusted: public, key: private, artifact: artifact, directory: t.TempDir()}
}

func (f *fixture) release(t *testing.T, version, channel string) []byte {
	t.Helper()
	digest := sha256.Sum256(f.artifact)
	manifest, err := Sign(Manifest{
		Release: version, Channel: channel,
		ArtifactName:   "NomadBrowser-" + version + ".dmg",
		ArtifactDigest: hex.EncodeToString(digest[:]),
		ArtifactBytes:  int64(len(f.artifact)),
	}, f.key)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func (f *fixture) watermark() string { return filepath.Join(f.directory, "installed.json") }

func mustDecode(t *testing.T, f *fixture, encoded []byte) Verified {
	t.Helper()
	release, err := Decode(encoded, f.trusted)
	if err != nil {
		t.Fatal(err)
	}
	return release
}

func TestVersionOrderingPutsPreReleasesBeforeTheReleaseTheyLeadTo(t *testing.T) {
	ordered := []string{
		"0.9.9", "1.0.0-alpha.1", "1.0.0-alpha.2", "1.0.0-beta.1", "1.0.0",
		"1.0.1", "1.1.0", "2.0.0",
	}
	for index := 0; index+1 < len(ordered); index++ {
		lower, err := ParseVersion(ordered[index])
		if err != nil {
			t.Fatal(err)
		}
		higher, err := ParseVersion(ordered[index+1])
		if err != nil {
			t.Fatal(err)
		}
		if lower.Compare(higher) != -1 {
			t.Errorf("%s does not order before %s", lower, higher)
		}
		if higher.Compare(lower) != 1 {
			t.Errorf("%s does not order after %s", higher, lower)
		}
		if lower.Compare(lower) != 0 {
			t.Errorf("%s does not equal itself", lower)
		}
	}
}

// Two spellings of one version is one too many for something a rollback check
// compares, so non-canonical components are refused rather than normalised.
func TestNonCanonicalVersionsAreRefused(t *testing.T) {
	for _, text := range []string{"1.02.3", "1.2", "1.2.3.4", "1.2.x", "", "v1.2.3", "1.2.3-a b"} {
		if _, err := ParseVersion(text); err == nil {
			t.Errorf("%q parsed as a version", text)
		}
	}
}

func TestAValidReleaseVerifiesAndInstalls(t *testing.T) {
	f := newFixture(t)
	release := mustDecode(t, f, f.release(t, "1.0.0", "stable"))
	if release.Release.String() != "1.0.0" || release.Channel != "stable" {
		t.Fatalf("decoded %+v", release)
	}

	artifact := filepath.Join(f.directory, "NomadBrowser-1.0.0.dmg")
	if err := os.WriteFile(artifact, f.artifact, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifact(artifact, release); err != nil {
		t.Fatalf("a genuine artifact did not verify: %v", err)
	}
	if err := AcceptMonotonic(f.watermark(), release); err != nil {
		t.Fatalf("a first install was refused: %v", err)
	}
	installed, err := InstalledVersion(f.watermark())
	if err != nil || installed == nil || installed.String() != "1.0.0" {
		t.Fatalf("watermark records %v (%v)", installed, err)
	}
}

// The three named attacks.

func TestASignedOlderReleaseCannotRollTheInstallationBack(t *testing.T) {
	f := newFixture(t)
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.2.0", "stable"))); err != nil {
		t.Fatal(err)
	}
	older := mustDecode(t, f, f.release(t, "1.1.9", "stable"))
	err := AcceptMonotonic(f.watermark(), older)
	if !errors.Is(err, ErrRollback) {
		t.Fatalf("a validly signed older release was accepted: %v", err)
	}

	// And a pre-release of the installed version is a rollback too, which is
	// the case an ordering that ignored pre-release labels would miss.
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.2.0-alpha.1", "stable"))); !errors.Is(err, ErrRollback) {
		t.Fatalf("a pre-release of the installed version was accepted: %v", err)
	}
}

func TestTwoArtifactsClaimingOneVersionAreRefusedRatherThanResolved(t *testing.T) {
	f := newFixture(t)
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.0.0", "stable"))); err != nil {
		t.Fatal(err)
	}
	// A second, differently built artifact, validly signed, same version.
	other := newFixture(t)
	other.key, other.trusted = f.key, f.trusted
	substitute := mustDecode(t, f, other.release(t, "1.0.0", "stable"))

	err := AcceptMonotonic(f.watermark(), substitute)
	if !errors.Is(err, ErrEquivocation) {
		t.Fatalf("two artifacts for one version were resolved instead of refused: %v", err)
	}
}

func TestAValidManifestDoesNotAuthoriseADifferentArtifact(t *testing.T) {
	f := newFixture(t)
	release := mustDecode(t, f, f.release(t, "1.0.0", "stable"))

	substituted := append([]byte(nil), f.artifact...)
	substituted[len(substituted)/2] ^= 0xFF
	path := filepath.Join(f.directory, "substituted.dmg")
	if err := os.WriteFile(path, substituted, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifact(path, release); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("a substituted artifact verified: %v", err)
	}

	// Padding it to a different length must fail on the size before the hash.
	longer := append(append([]byte(nil), f.artifact...), 0x00)
	longerPath := filepath.Join(f.directory, "longer.dmg")
	if err := os.WriteFile(longerPath, longer, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArtifact(longerPath, release); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("an artifact longer than the signed length verified: %v", err)
	}
}

// Everything about verification fails closed. None of these may become a
// warning, a prompt, or an install that proceeds.
func TestEveryMalformedManifestIsRefused(t *testing.T) {
	f := newFixture(t)
	valid := f.release(t, "1.0.0", "stable")

	tamper := func(mutate func(map[string]any)) []byte {
		var fields map[string]any
		if err := json.Unmarshal(valid, &fields); err != nil {
			t.Fatal(err)
		}
		mutate(fields)
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}

	for name, encoded := range map[string][]byte{
		"empty":                    {},
		"not json":                 []byte("this is not a manifest"),
		"trailing data":            append(append([]byte(nil), valid...), []byte(`{"extra":1}`)...),
		"unknown field":            tamper(func(m map[string]any) { m["surprise"] = true }),
		"unknown manifest version": tamper(func(m map[string]any) { m["version"] = "nomad-browser-release-v2" }),
		"broken signature":         tamper(func(m map[string]any) { m["signature"] = "00" + m["signature"].(string)[2:] }),
		"release moved":            tamper(func(m map[string]any) { m["release"] = "9.9.9" }),
		"channel moved":            tamper(func(m map[string]any) { m["channel"] = "canary" }),
		"digest moved":             tamper(func(m map[string]any) { m["artifact_digest"] = hex.EncodeToString(make([]byte, 32)) }),
		"size moved":               tamper(func(m map[string]any) { m["artifact_bytes"] = 4097 }),
		"artifact name is a path":  tamper(func(m map[string]any) { m["artifact_name"] = "../elsewhere.dmg" }),
		"zero size":                tamper(func(m map[string]any) { m["artifact_bytes"] = 0 }),
		"malformed key":            tamper(func(m map[string]any) { m["public_key"] = "zz" }),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Decode(encoded, f.trusted); !errors.Is(err, ErrUnverified) {
				t.Fatalf("accepted: %v", err)
			}
		})
	}
}

// A manifest that names its own key authenticates nothing. Trust must come
// from the build, and a manifest signed by anyone else must be refused even
// though its own signature is internally consistent.
func TestAManifestSignedByAnotherKeyIsRefused(t *testing.T) {
	f := newFixture(t)
	attacker := newFixture(t)
	forged := attacker.release(t, "9.9.9", "stable")

	if _, err := Decode(forged, f.trusted); !errors.Is(err, ErrUnverified) {
		t.Fatalf("a manifest signed by an untrusted key was accepted: %v", err)
	}
	// It must verify under its own key, so the case above really is about
	// trust rather than about a broken signature.
	if _, err := Decode(forged, attacker.trusted); err != nil {
		t.Fatalf("the forged manifest is not internally valid, so this case proves nothing: %v", err)
	}
}

// Corrupting the watermark must not be a way to switch rollback protection
// off. This is the failure mode a "treat unreadable as missing" default would
// hand an attacker who can write one file.
func TestAnUnreadableWatermarkRefusesTheInstallRatherThanIgnoringIt(t *testing.T) {
	f := newFixture(t)
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "2.0.0", "stable"))); err != nil {
		t.Fatal(err)
	}

	for name, content := range map[string]string{
		"truncated":      `{"version":"2.0.`,
		"empty object":   `{}`,
		"missing digest": `{"version":"2.0.0","channel":"stable"}`,
		"unknown field":  `{"version":"2.0.0","artifact_digest":"aa","channel":"stable","extra":1}`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(f.watermark(), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			older := mustDecode(t, f, f.release(t, "1.0.0", "stable"))
			err := AcceptMonotonic(f.watermark(), older)
			if err == nil {
				t.Fatal("a corrupt watermark let an older release install")
			}
			if errors.Is(err, ErrRollback) {
				t.Fatalf("the watermark was read successfully, so this case proves nothing: %v", err)
			}
		})
	}
}

// An install from a different channel is not an upgrade decision this code
// gets to make silently: a stable user must not be moved onto canary by a
// canary manifest with a higher number.
func TestAReleaseFromAnotherChannelDoesNotInstallOverThisOne(t *testing.T) {
	f := newFixture(t)
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.0.0", "stable"))); err != nil {
		t.Fatal(err)
	}
	newer := mustDecode(t, f, f.release(t, "2.0.0", "canary"))
	if err := AcceptMonotonic(f.watermark(), newer); !errors.Is(err, ErrRollback) {
		t.Fatalf("a canary release installed over a stable one: %v", err)
	}
}

func TestReinstallingTheSameVersionIsRefused(t *testing.T) {
	f := newFixture(t)
	encoded := f.release(t, "1.0.0", "stable")
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, encoded)); err != nil {
		t.Fatal(err)
	}
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, encoded)); !errors.Is(err, ErrRollback) {
		t.Fatalf("the installed version reinstalled over itself: %v", err)
	}
}

// A failed install must leave the previous watermark intact, or one refused
// update would block every future one.
func TestARefusedInstallLeavesTheWatermarkUnchanged(t *testing.T) {
	f := newFixture(t)
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.5.0", "stable"))); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(f.watermark())
	if err != nil {
		t.Fatal(err)
	}
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.0.0", "stable"))); !errors.Is(err, ErrRollback) {
		t.Fatal(err)
	}
	after, err := os.ReadFile(f.watermark())
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("a refused install rewrote the watermark:\n%s\n%s", before, after)
	}

	// And a genuine newer release still installs afterwards, so refusing did
	// not wedge the mechanism.
	if err := AcceptMonotonic(f.watermark(), mustDecode(t, f, f.release(t, "1.6.0", "stable"))); err != nil {
		t.Fatalf("a valid update was refused after an earlier refusal: %v", err)
	}
}
