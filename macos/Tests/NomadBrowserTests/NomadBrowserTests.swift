import Foundation
import Testing
@testable import NomadBrowser

private func builtInEnvelopes() throws -> [SignedEnvelope] {
    let url = try #require(Bundle.module.url(forResource: "demo-catalog", withExtension: "json"))
    return try JSONDecoder().decode([SignedEnvelope].self, from: Data(contentsOf: url))
}

@Test("Every bundled object is exactly committed and Ed25519 verified")
func bundledObjectsVerify() throws {
    let envelopes = try builtInEnvelopes()
    #expect(envelopes.count == 3)
    for envelope in envelopes {
        let verified = try ObjectVerifier.verify(envelope)
        #expect(verified.trustedPublisher)
        #expect(verified.id == envelope.contentHash)
    }
}

@Test("A payload mutation fails closed")
func payloadMutationFails() throws {
    let original = try #require(builtInEnvelopes().first)
    let mutated = SignedEnvelope(
        version: original.version,
        payload: original.payload + "A",
        contentHash: original.contentHash,
        publisherKey: original.publisherKey,
        signature: original.signature
    )
    #expect(throws: (any Error).self) {
        try ObjectVerifier.verify(mutated)
    }
}

@Test("Local search ranks Selection Firewall without a network callback")
func localSearchWorks() throws {
    let documents = try builtInEnvelopes().map(ObjectVerifier.verify)
    let results = LocalSearchEngine.search("privat trafik selection firewall", documents: documents)
    #expect(results.first?.document.document.title == "Selection Firewall")
}

@Test("Empty and oversized queries are bounded")
func queriesAreBounded() throws {
    let documents = try builtInEnvelopes().map(ObjectVerifier.verify)
    #expect(LocalSearchEngine.search("   ", documents: documents).isEmpty)
    let query = String(repeating: "nomad ", count: 10_000)
    let results = LocalSearchEngine.search(query, documents: documents)
    #expect(!results.isEmpty)
}
