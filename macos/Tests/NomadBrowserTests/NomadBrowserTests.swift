import Foundation
import CryptoKit
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

@Test("A valid signature from an untrusted publisher fails closed")
func untrustedPublisherFails() throws {
    let original = try #require(builtInEnvelopes().first)
    let payload = try #require(Data(base64Encoded: original.payload))
    let digest = Data(SHA256.hash(data: payload))
    let privateKey = Curve25519.Signing.PrivateKey()
    var message = ObjectVerifier.objectDomain
    message.append(digest)
    let signature = try privateKey.signature(for: message)
    let envelope = SignedEnvelope(
        version: 1,
        payload: original.payload,
        contentHash: digest.hexString,
        publisherKey: privateKey.publicKey.rawRepresentation.base64EncodedString(),
        signature: signature.base64EncodedString()
    )
    #expect(throws: (any Error).self) {
        try ObjectVerifier.verify(envelope)
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

@Test("Shared cache uses the exact provisioned App Group container")
func sharedCacheUsesExactAppGroup() throws {
    let container = URL(fileURLWithPath: "/tmp/nomad-shared-test", isDirectory: true)
    var requestedIdentifier: String?
    let directory = try SharedCache.objectDirectory { identifier in
        requestedIdentifier = identifier
        return container
    }

    #expect(requestedIdentifier == "group.io.nomad.shared")
    #expect(SharedCache.appGroupIdentifier == "group.io.nomad.shared")
    #expect(directory == container.appendingPathComponent("objects", isDirectory: true))
}

@Test("Missing App Group fails closed instead of falling back to private storage")
func missingSharedCacheFailsClosed() {
    #expect(throws: SharedCacheError.appGroupUnavailable("group.io.nomad.shared")) {
        try SharedCache.objectDirectory { _ in nil }
    }
}

@Test("Periodic local cache reload isolates malformed objects")
@MainActor
func liveCacheReloadIsFailClosedPerObject() throws {
    let directory = FileManager.default.temporaryDirectory
        .appendingPathComponent(UUID().uuidString, isDirectory: true)
    try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
    defer { try? FileManager.default.removeItem(at: directory) }

    let envelopes = try builtInEnvelopes()
    let store = NomadStore(objectDirectory: directory, includeBuiltIn: false)
    #expect(store.documents.isEmpty)

    let encoder = JSONEncoder()
    try encoder.encode(envelopes[0]).write(
        to: directory.appendingPathComponent("01.nomadobject"),
        options: .atomic
    )
    try Data("not-json".utf8).write(
        to: directory.appendingPathComponent("02.nomadobject"),
        options: .atomic
    )
    try encoder.encode(envelopes[1]).write(
        to: directory.appendingPathComponent("03.nomadobject"),
        options: .atomic
    )

    store.reload()
    #expect(store.documents.count == 2)
    #expect(store.rejectedObjectCount == 1)
    #expect(store.lastError == nil)
}