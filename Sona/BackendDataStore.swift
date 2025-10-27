import Foundation

struct BackendKid: Codable, Identifiable {
    let id: String
    var email: String
    var name: String?
    var wallet: Double
    var walletAddress: String?
    var parentId: String?
}

struct BackendParent: Codable {
    let id: String
    var name: String
    var email: String
    var wallet: Double
    var walletAddress: String?
    var kids: [BackendKid]
}

struct BackendCache: Codable {
    var parent: BackendParent?
    var child: BackendKid?
}

final class BackendDataStore {
    static let shared = BackendDataStore()
    private let key = "backend_cache_json"

    private init() {}

    func load() -> BackendCache? {
        do {
            if let data = try KeychainStorage.shared.get(key) {
                return try? JSONDecoder().decode(BackendCache.self, from: data)
            }
            return nil
        } catch {
            return nil
        }
    }

    func save(_ cache: BackendCache) {
        if let data = try? JSONEncoder().encode(cache) {
            try? KeychainStorage.shared.set(data, for: key)
        }
    }

    func updateParent(_ parent: BackendParent) {
        var cache = load() ?? BackendCache(parent: nil)
        cache.parent = parent
        save(cache)
    }

    func upsertKids(_ kids: [BackendKid]) {
        guard var cache = load() else { return }
        guard var parent = cache.parent else { return }
        var map: [String: BackendKid] = [:]
        for k in kids { map[k.id] = k }
        // keep only provided kids list to reflect server truth
        parent.kids = kids
        cache.parent = parent
        save(cache)
    }

    func mergeKid(_ kid: BackendKid) {
        guard var cache = load() else { return }
        guard var parent = cache.parent else { return }
        if let idx = parent.kids.firstIndex(where: { $0.id == kid.id }) {
            parent.kids[idx] = kid
        } else {
            parent.kids.append(kid)
        }
        cache.parent = parent
        save(cache)
    }

    func updateChild(_ kid: BackendKid) {
        var cache = load() ?? BackendCache(parent: nil, child: nil)
        cache.child = kid
        save(cache)
    }
}


