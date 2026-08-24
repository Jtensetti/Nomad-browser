// Command nomad-browser-verify decides whether a release the user already has
// may be installed over what is installed now.
//
// It downloads nothing. See docs/UPDATING.md for why an auto-updater would
// undo the guarantee the rest of the browser is built on.
package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/Jtensetti/nomad-browser/update"
)

// releaseKey is the public half of the release signing key, compiled in.
//
// It is a build constant rather than a file or a flag on purpose: a trusted
// key that an attacker can replace alongside the manifest authenticates
// nothing. -release-key exists only so the mechanism can be exercised against
// a test key, and using it prints a warning naming what the run does not
// establish.
//
// It is empty because no release key has been generated yet. See
// nomad-protocol production/EXTERNAL_BLOCKERS.md, EB-7.
const releaseKey = ""

func main() {
	manifestPath := flag.String("manifest", "", "path to the signed release manifest")
	artifactPath := flag.String("artifact", "", "path to the installer this manifest describes")
	watermarkPath := flag.String("watermark", "", "path to the installed-version watermark")
	overrideKey := flag.String("release-key", "", "hex release public key, for exercising the "+
		"mechanism against a test key; the compiled-in key is used when this is empty")
	dryRun := flag.Bool("dry-run", false, "verify without advancing the watermark")
	flag.Parse()

	if err := run(*manifestPath, *artifactPath, *watermarkPath, *overrideKey, *dryRun); err != nil {
		fmt.Fprintf(os.Stderr, "refused: %v\n", err)
		os.Exit(1)
	}
}

func run(manifestPath, artifactPath, watermarkPath, overrideKey string, dryRun bool) error {
	if manifestPath == "" || artifactPath == "" || watermarkPath == "" {
		flag.Usage()
		return errors.New("a manifest, an artifact and a watermark path are all required")
	}

	trusted, err := trustedKey(overrideKey)
	if err != nil {
		return err
	}

	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	release, err := update.Decode(encoded, trusted)
	if err != nil {
		return err
	}
	if err := update.VerifyArtifact(artifactPath, release); err != nil {
		return err
	}

	installed, err := update.InstalledVersion(watermarkPath)
	if err != nil {
		return err
	}
	switch {
	case installed == nil:
		fmt.Printf("no previous install recorded at %s\n", watermarkPath)
	default:
		fmt.Printf("installed: %s\n", installed)
	}
	fmt.Printf("offered:   %s (%s), artifact %x\n",
		release.Release, release.Channel, release.Digest[:8])

	if dryRun {
		fmt.Println("dry run: the watermark was not advanced, so this release is not recorded as installed")
		return nil
	}
	if err := update.AcceptMonotonic(watermarkPath, release); err != nil {
		return err
	}
	fmt.Printf("accepted: install %s, then the watermark records it as current\n", release.Release)
	return nil
}

func trustedKey(override string) (ed25519.PublicKey, error) {
	if override != "" {
		decoded, err := hex.DecodeString(override)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			return nil, errors.New("-release-key must be a hex ed25519 public key")
		}
		fmt.Fprintln(os.Stderr, "warning: verifying against a key supplied on the command line. "+
			"This exercises the mechanism; it establishes nothing about who signed this release, "+
			"because an attacker who can substitute the manifest can substitute this key with it.")
		return decoded, nil
	}
	if releaseKey == "" {
		return nil, errors.New("this build has no compiled-in release key, so it cannot " +
			"establish who signed anything. No release key has been generated yet " +
			"(nomad-protocol production/EXTERNAL_BLOCKERS.md, EB-7). Pass -release-key to " +
			"exercise the mechanism against a test key")
	}
	decoded, err := hex.DecodeString(releaseKey)
	if err != nil || len(decoded) != ed25519.PublicKeySize {
		return nil, errors.New("the compiled-in release key is malformed")
	}
	return decoded, nil
}
