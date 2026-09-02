// Package demotrust holds the publisher the alpha catalog is signed by, so the
// Go tests and the Swift client cannot drift onto different anchors.
package demotrust

// PublisherKey is the Ed25519 public key, base64, that
// macos/Sources/NomadBrowser/Models.swift anchors. It signs the demo catalog
// shipped for the alpha and nothing else; it is not a release key.
const PublisherKey = "SsX0q+oi8C1+v0yTSrltfxYkztmjrdJNE/gN7XN0jEk="
