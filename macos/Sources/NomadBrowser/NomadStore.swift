import Combine
import Foundation

@MainActor
final class NomadStore: ObservableObject {
    static let maximumObjects = 256
    static let maximumEncodedEnvelopeBytes = 400_000
    static let publicCacheRefreshInterval: TimeInterval = 5

    @Published private(set) var documents: [VerifiedDocument] = []
    @Published private(set) var rejectedObjectCount = 0
    @Published private(set) var lastError: String?

    private let objectDirectoryOverride: URL?
    private let includeBuiltIn: Bool

    init(objectDirectory: URL? = nil, includeBuiltIn: Bool = true) {
        objectDirectoryOverride = objectDirectory
        self.includeBuiltIn = includeBuiltIn
        reload()
    }

    func reload() {
        var envelopes: [SignedEnvelope] = []
        var rejected = 0
        lastError = nil

        if includeBuiltIn {
            do {
                guard let builtInURL = Bundle.module.url(forResource: "demo-catalog", withExtension: "json") else {
                    throw CocoaError(.fileNoSuchFile)
                }
                let data = try Self.boundedData(at: builtInURL)
                envelopes.append(contentsOf: try JSONDecoder().decode([SignedEnvelope].self, from: data))
            } catch {
                lastError = error.localizedDescription
            }
        }

        do {
            let disk = try diskEnvelopes()
            envelopes.append(contentsOf: disk.envelopes)
            rejected += disk.rejected
        } catch {
            lastError = error.localizedDescription
        }

        var accepted: [String: VerifiedDocument] = [:]
        for envelope in envelopes.prefix(Self.maximumObjects) {
            do {
                let verified = try ObjectVerifier.verify(envelope)
                accepted[verified.id] = verified
            } catch {
                rejected += 1
            }
        }
        documents = accepted.values.sorted {
            $0.document.title.localizedStandardCompare($1.document.title) == .orderedAscending
        }
        rejectedObjectCount = rejected
    }

    private func diskEnvelopes() throws -> (envelopes: [SignedEnvelope], rejected: Int) {
        let manager = FileManager.default
        let objectDirectory: URL
        if let objectDirectoryOverride {
            objectDirectory = objectDirectoryOverride
        } else {
            let applicationSupport = try manager.url(
                for: .applicationSupportDirectory,
                in: .userDomainMask,
                appropriateFor: nil,
                create: true
            )
            objectDirectory = applicationSupport
                .appendingPathComponent("NomadBrowser", isDirectory: true)
                .appendingPathComponent("objects", isDirectory: true)
        }
        try manager.createDirectory(at: objectDirectory, withIntermediateDirectories: true)
        let files = try manager.contentsOfDirectory(
            at: objectDirectory,
            includingPropertiesForKeys: [.isRegularFileKey, .isSymbolicLinkKey, .fileSizeKey],
            options: [.skipsHiddenFiles, .skipsPackageDescendants]
        )
        let candidates = files
            .filter { $0.pathExtension == "nomadobject" }
            .sorted { $0.lastPathComponent < $1.lastPathComponent }
            .prefix(Self.maximumObjects)
        var envelopes: [SignedEnvelope] = []
        var rejected = 0
        for file in candidates {
            do {
                envelopes.append(try JSONDecoder().decode(SignedEnvelope.self, from: Self.boundedData(at: file)))
            } catch {
                // One hostile or partially written object must not suppress the
                // other immutable cache entries.
                rejected += 1
            }
        }
        return (envelopes, rejected)
    }

    private static func boundedData(at url: URL) throws -> Data {
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .isSymbolicLinkKey, .fileSizeKey])
        guard values.isRegularFile == true, values.isSymbolicLink != true else {
            throw CocoaError(.fileReadUnsupportedScheme)
        }
        guard let size = values.fileSize, size <= Self.maximumEncodedEnvelopeBytes else {
            throw ObjectVerificationError.objectTooLarge
        }
        let data = try Data(contentsOf: url, options: [.mappedIfSafe, .uncached])
        guard data.count <= Self.maximumEncodedEnvelopeBytes else {
            throw ObjectVerificationError.objectTooLarge
        }
        return data
    }
}
