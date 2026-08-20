import CryptoKit
import Foundation

struct NomadDocument: Codable, Sendable, Hashable {
    let title: String
    let summary: String
    let body: String
    let tags: [String]
    let publishedAt: String
    let publisherName: String
    let mediaType: String
}

struct SignedEnvelope: Codable, Sendable {
    let version: Int
    let payload: String
    let contentHash: String
    let publisherKey: String
    let signature: String
}

struct VerifiedDocument: Identifiable, Sendable, Hashable {
    let id: String
    let document: NomadDocument
    let publisherFingerprint: String
    let trustedPublisher: Bool
}

enum ObjectVerificationError: LocalizedError, Equatable {
    case unsupportedVersion
    case malformedEncoding
    case objectTooLarge
    case commitmentMismatch
    case invalidSignature
    case untrustedPublisher
    case unsupportedMediaType
    case invalidDocument

    var errorDescription: String? {
        switch self {
        case .unsupportedVersion: return "Objektversionen stöds inte."
        case .malformedEncoding: return "Objektets kodning är ogiltig."
        case .objectTooLarge: return "Objektet överskrider säkerhetsgränsen."
        case .commitmentMismatch: return "Objektets SHA-256-åtagande stämmer inte."
        case .invalidSignature: return "Objektets Ed25519-signatur är ogiltig."
        case .untrustedPublisher: return "Objektets publiceringsnyckel är inte betrodd av denna klient."
        case .unsupportedMediaType: return "Objektets medietyp stöds inte av den säkra renderaren."
        case .invalidDocument: return "Dokumentets innehåll är ogiltigt."
        }
    }
}

enum ObjectVerifier {
    static let maximumPayloadBytes = 262_144
    static let maximumBodyCharacters = 200_000
    static let maximumTitleCharacters = 300
    static let maximumSummaryCharacters = 2_000
    static let maximumPublisherNameCharacters = 200
    static let maximumPublishedAtCharacters = 64
    static let maximumTags = 64
    static let objectDomain = Data("nomad-object-v1".utf8)
    static let trustedDemoPublisher = Data(base64Encoded: "SsX0q+oi8C1+v0yTSrltfxYkztmjrdJNE/gN7XN0jEk=")!
    static let trustedPublisherKeys: Set<Data> = [trustedDemoPublisher]

    static func verify(_ envelope: SignedEnvelope) throws -> VerifiedDocument {
        guard envelope.version == 1 else {
            throw ObjectVerificationError.unsupportedVersion
        }
        guard
            let payload = Data(base64Encoded: envelope.payload),
            let publisherKey = Data(base64Encoded: envelope.publisherKey),
            let signature = Data(base64Encoded: envelope.signature),
            payload.count <= maximumPayloadBytes,
            publisherKey.count == 32,
            signature.count == 64
        else {
            throw ObjectVerificationError.malformedEncoding
        }

        let digest = Data(SHA256.hash(data: payload))
        guard digest.hexString == envelope.contentHash.lowercased() else {
            throw ObjectVerificationError.commitmentMismatch
        }
        var signingMessage = objectDomain
        signingMessage.append(digest)
        let key: Curve25519.Signing.PublicKey
        do {
            key = try Curve25519.Signing.PublicKey(rawRepresentation: publisherKey)
        } catch {
            throw ObjectVerificationError.malformedEncoding
        }
        guard key.isValidSignature(signature, for: signingMessage) else {
            throw ObjectVerificationError.invalidSignature
        }
        guard trustedPublisherKeys.contains(publisherKey) else {
            throw ObjectVerificationError.untrustedPublisher
        }

        let document: NomadDocument
        do {
            document = try JSONDecoder().decode(NomadDocument.self, from: payload)
        } catch {
            throw ObjectVerificationError.invalidDocument
        }
        guard document.mediaType == "text/plain; charset=utf-8" else {
            throw ObjectVerificationError.unsupportedMediaType
        }
        guard
            !document.title.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty,
            document.title.count <= maximumTitleCharacters,
            document.summary.count <= maximumSummaryCharacters,
            document.body.count <= maximumBodyCharacters,
            document.publisherName.count <= maximumPublisherNameCharacters,
            document.publishedAt.count <= maximumPublishedAtCharacters,
            document.tags.count <= maximumTags,
            document.tags.allSatisfy({ $0.count <= 100 })
        else {
            throw ObjectVerificationError.invalidDocument
        }

        return VerifiedDocument(
            id: digest.hexString,
            document: document,
            publisherFingerprint: String(Data(SHA256.hash(data: publisherKey)).hexString.prefix(16)),
            trustedPublisher: true
        )
    }
}

extension Data {
    var hexString: String {
        map { String(format: "%02x", $0) }.joined()
    }
}
