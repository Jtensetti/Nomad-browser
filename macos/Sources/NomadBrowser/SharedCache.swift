import Foundation

enum SharedCacheError: Error, Equatable {
    case appGroupUnavailable(String)
}

enum SharedCache {
    // This identifier is intentionally a protocol/release constant rather than
    // a user preference. Both independently signed reader and materializer
    // processes must carry the same provisioned entitlement before production.
    static let appGroupIdentifier = "group.io.nomad.shared"
    static let objectDirectoryName = "objects"

    static func objectDirectory() throws -> URL {
        try objectDirectory { identifier in
            FileManager.default.containerURL(
                forSecurityApplicationGroupIdentifier: identifier
            )
        }
    }

    // The resolver is injected only so tests can prove both the exact path and
    // fail-closed behavior without requiring a provisioned Apple entitlement on
    // the test runner. Production always uses FileManager.containerURL above.
    static func objectDirectory(
        resolveContainer: (String) -> URL?
    ) throws -> URL {
        guard let container = resolveContainer(appGroupIdentifier) else {
            throw SharedCacheError.appGroupUnavailable(appGroupIdentifier)
        }
        return container.appendingPathComponent(objectDirectoryName, isDirectory: true)
    }
}
