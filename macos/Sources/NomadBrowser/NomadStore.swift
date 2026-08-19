import Combine
import Foundation

@MainActor
final class NomadStore: ObservableObject {
    static let maximumObjects = 10_000

    @Published private(set) var documents: [VerifiedDocument] = []
    @Published private(set) var rejectedObjectCount = 0
    @Published private(set) var lastError: String?

    init() {
        reload()
    }

    func reload() {
        var envelopes: [SignedEnvelope] = []
        do {
            guard let builtInURL = Bundle.module.url(forResource: "demo-catalog", withExtension: "json") else {
                throw CocoaError(.fileNoSuchFile)
            }
            let data = try boundedData(at: builtInURL)
            envelopes.append(contentsOf: try JSONDecoder().decode([SignedEnvelope].self, from: data))
            envelopes.append(contentsOf: try diskEnvelopes())
        } catch {
            lastError = error.localizedDescription
        }

        var accepted: [String: VerifiedDocument] = [:]
        var rejected = 0
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

    private func diskEnvelopes() throws -> [SignedEnvelope] {
        let manager = FileManager.default
        let applicationSupport = try manager.url(
            for: .applicationSupportDirectory,
            in: .userDomainMask,
            appropriateFor: nil,
            create: true
        )
        let objectDirectory = applicationSupport
            .appendingPathComponent("NomadBrowser", isDirectory: true)
            .appendingPathComponent("objects", isDirectory: true)
        try manager.createDirectory(at: objectDirectory, withIntermediateDirectories: true)
        let files = try manager.contentsOfDirectory(
            at: objectDirectory,
            includingPropertiesForKeys: [.isRegularFileKey, .fileSizeKey],
            options: [.skipsHiddenFiles, .skipsPackageDescendants]
        )
        return try files
            .filter { $0.pathExtension == "nomadobject" }
            .prefix(Self.maximumObjects)
            .map { try JSONDecoder().decode(SignedEnvelope.self, from: boundedData(at: $0)) }
    }

    private func boundedData(at url: URL) throws -> Data {
        let values = try url.resourceValues(forKeys: [.isRegularFileKey, .fileSizeKey])
        guard values.isRegularFile == true else { throw CocoaError(.fileReadUnsupportedScheme) }
        guard let size = values.fileSize, size <= ObjectVerifier.maximumPayloadBytes * 2 else {
            throw ObjectVerificationError.objectTooLarge
        }
        return try Data(contentsOf: url, options: [.mappedIfSafe, .uncached])
    }
}
