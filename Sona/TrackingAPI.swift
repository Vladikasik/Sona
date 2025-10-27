import Foundation

enum TrackingAPI {
    private static let baseURL = URL(string: "https://api.aynshteyn.dev")!
    private static let authHeader = "Bearer SonaBetaTestAPi"

    struct APIError: Decodable, Error { let error: String }

    struct ParentResponse: Decodable {
        let id: String
        let name: String
        let email: String
        let wallet: Double
        let wallet_address: String?
        let kids_list: [ParentKid]

        private enum CodingKeys: String, CodingKey { case id, name, email, wallet, kids_list }

        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            id = try c.decode(String.self, forKey: .id)
            name = try c.decode(String.self, forKey: .name)
            email = try c.decode(String.self, forKey: .email)
            if let w = try? c.decode(Double.self, forKey: .wallet) {
                wallet = w
                wallet_address = nil
            } else if let ws = try? c.decode(String.self, forKey: .wallet) {
                if let wd = Double(ws) {
                    wallet = wd
                    wallet_address = nil
                } else {
                    wallet = 0
                    wallet_address = ws.isEmpty ? nil : ws
                }
            } else {
                wallet = 0
                wallet_address = nil
            }
            kids_list = try c.decode([ParentKid].self, forKey: .kids_list)
        }
    }

    struct ParentKid: Decodable {
        let email: String
        let wallet: Double
        let walletAddress: String?

        private enum CodingKeys: String, CodingKey { case email, wallet }
        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            email = try c.decode(String.self, forKey: .email)
            if let w = try? c.decode(Double.self, forKey: .wallet) {
                wallet = w
                walletAddress = nil
            } else if let ws = try? c.decode(String.self, forKey: .wallet) {
                if let wd = Double(ws) {
                    wallet = wd
                    walletAddress = nil
                } else {
                    wallet = 0
                    walletAddress = ws.isEmpty ? nil : ws
                }
            } else {
                wallet = 0
                walletAddress = nil
            }
        }
    }

    struct ChildResponse: Decodable {
        let id: String
        let name: String
        let email: String
        let parent_id: String
        let wallet: Double
        let wallet_address: String?

        private enum CodingKeys: String, CodingKey { case id, name, email, parent_id, wallet }
        init(from decoder: Decoder) throws {
            let c = try decoder.container(keyedBy: CodingKeys.self)
            id = try c.decode(String.self, forKey: .id)
            name = try c.decode(String.self, forKey: .name)
            email = try c.decode(String.self, forKey: .email)
            parent_id = (try? c.decode(String.self, forKey: .parent_id)) ?? ""
            if let w = try? c.decode(Double.self, forKey: .wallet) {
                wallet = w
                wallet_address = nil
            } else if let ws = try? c.decode(String.self, forKey: .wallet) {
                if let wd = Double(ws) {
                    wallet = wd
                    wallet_address = nil
                } else {
                    wallet = 0
                    wallet_address = ws.isEmpty ? nil : ws
                }
            } else {
                wallet = 0
                wallet_address = nil
            }
        }
    }

    // Centralized network helpers that leverage APIService logging
    private static func performGET<T: Decodable>(_ path: String, query: [String: String]? = nil) async throws -> T {
        var comps = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if let query, !query.isEmpty {
            comps.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) }
        }
        guard let url = comps.url else { throw URLError(.badURL) }
        let (data, http) = try await APIService.performAbsolute(method: "GET", url: url, headers: [
            "Authorization": authHeader
        ])
        let decoder = JSONDecoder()
        if (200..<300).contains(http.statusCode) {
            return try decoder.decode(T.self, from: data)
        }
        if let api = try? decoder.decode(APIError.self, from: data) { throw api }
        throw URLError(.badServerResponse)
    }

    private static func performPOST<T: Decodable, B: Encodable>(_ path: String, body: B) async throws -> T {
        let url = baseURL.appendingPathComponent(path)
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: [
            "Authorization": authHeader
        ], jsonBody: body)
        let decoder = JSONDecoder()
        if (200..<300).contains(http.statusCode) {
            return try decoder.decode(T.self, from: data)
        }
        if let api = try? decoder.decode(APIError.self, from: data) { throw api }
        throw URLError(.badServerResponse)
    }

    // MARK: - Public

    struct GetUserResponse: Decodable {
        let userUid: String
        private enum CodingKeys: String, CodingKey { case userUid = "user_uid" }
    }

    static func getUser(email: String?) async throws -> String {
        let q: [String: String]? = (email != nil) ? ["email": email!] : nil
        let res: GetUserResponse = try await performGET("/get_user", query: q)
        return res.userUid
    }

    static func getOrCreateParent(email: String, name: String?) async throws -> BackendParent {
        struct Body: Encodable { let email: String; let name: String? }
        let body = Body(email: email, name: (name?.isEmpty == false) ? name : nil)
        let res: ParentResponse = try await performPOST("/get_parent", body: body)
        let kids: [BackendKid] = res.kids_list.enumerated().map { idx, k in
            BackendKid(id: "kid_\(idx)_\(k.email)", email: k.email, name: nil, wallet: k.wallet, walletAddress: k.walletAddress, parentId: nil)
        }
        return BackendParent(id: res.id, name: res.name, email: res.email, wallet: res.wallet, walletAddress: res.wallet_address, kids: kids)
    }

    static func updateParent(email: String, name: String?, wallet: String?) async throws -> BackendParent {
        struct Body: Encodable { let email: String; let upd: Bool; let name: String?; let wallet: String? }
        let body = Body(email: email, upd: true, name: name, wallet: wallet)
        let res: ParentResponse = try await performPOST("/get_parent", body: body)
        let kids: [BackendKid] = res.kids_list.enumerated().map { idx, k in
            BackendKid(id: "kid_\(idx)_\(k.email)", email: k.email, name: nil, wallet: k.wallet, walletAddress: k.walletAddress, parentId: nil)
        }
        return BackendParent(id: res.id, name: res.name, email: res.email, wallet: res.wallet, walletAddress: res.wallet_address, kids: kids)
    }

    static func getOrCreateChild(email: String, name: String?, parentId: String?) async throws -> BackendKid? {
        struct Body: Encodable { let email: String; let name: String?; let parent_id: String? }
        let body = Body(email: email, name: name, parent_id: parentId)
        let res: ChildResponse = try await performPOST("/get_child", body: body)
        return BackendKid(id: res.id, email: res.email, name: res.name, wallet: res.wallet, walletAddress: res.wallet_address, parentId: res.parent_id.isEmpty ? nil : res.parent_id)
    }

    static func updateChild(email: String, name: String?, parentId: String?, wallet: String?) async throws -> BackendKid {
        struct Body: Encodable { let email: String; let upd: Bool; let name: String?; let parent_id: String?; let wallet: String? }
        let body = Body(email: email, upd: true, name: name, parent_id: parentId, wallet: wallet)
        let res: ChildResponse = try await performPOST("/get_child", body: body)
        return BackendKid(id: res.id, email: res.email, name: res.name, wallet: res.wallet, walletAddress: res.wallet_address, parentId: res.parent_id.isEmpty ? nil : res.parent_id)
    }
}


