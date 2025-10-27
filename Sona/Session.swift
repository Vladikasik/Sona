//
//  Session.swift
//  Sona
//
//  Created by Assistant on 23.10.25.
//

import Foundation
import Security
import CryptoKit

// MARK: - Keychain Helper

final class KeychainStorage {
    static let shared = KeychainStorage()
    private let service = "Sona.Keychain"

    func set(_ data: Data, for key: String) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key
        ]
        SecItemDelete(query as CFDictionary)

        var attributes = query
        attributes[kSecValueData as String] = data
        let status = SecItemAdd(attributes as CFDictionary, nil)
        guard status == errSecSuccess else { throw NSError(domain: NSOSStatusErrorDomain, code: Int(status)) }
    }

    func get(_ key: String) throws -> Data? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key,
            kSecReturnData as String: true,
            kSecMatchLimit as String: kSecMatchLimitOne
        ]
        var item: CFTypeRef?
        let status = SecItemCopyMatching(query as CFDictionary, &item)
        if status == errSecItemNotFound { return nil }
        guard status == errSecSuccess else { throw NSError(domain: NSOSStatusErrorDomain, code: Int(status)) }
        return item as? Data
    }

    func remove(_ key: String) throws {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: key
        ]
        let status = SecItemDelete(query as CFDictionary)
        guard status == errSecSuccess || status == errSecItemNotFound else {
            throw NSError(domain: NSOSStatusErrorDomain, code: Int(status))
        }
    }
}

// MARK: - Minimal User Session Models

struct UserSession: Codable {
    let uid: String?
    let name: String?
    let email: String?
    let address: String?
    let gridUserId: String?
}

// Legacy format migration helper (reads just what we need)
private struct LegacyAuthSession: Decodable {
    let serverUserUid: String?
    let data: DataPayload?
    struct DataPayload: Decodable { let address: String? }
}

// MARK: - Session Manager

final class SessionManager: ObservableObject {
    static let shared = SessionManager()
    private let key = "auth_session_json" // reuse existing key to avoid stale data
    private let gridRawKey = "grid_raw_response"
    private let gridDecryptedKeyKey = "grid_decrypted_authorization_key"
    private let gridDecryptedKeyShadowUserDefaults = "grid_decrypted_authorization_key_shadow"
    private let privyAccessTokenKey = "grid_privy_access_token"

    @Published private(set) var session: UserSession?

    private init() {
        _ = try? reload()
        // Best-effort: if decrypted key is missing in keychain but exists in shadow UserDefaults (from older builds), migrate it
        if (try? KeychainStorage.shared.get(gridDecryptedKeyKey))??.isEmpty != false {
            if let shadow = UserDefaults.standard.data(forKey: gridDecryptedKeyShadowUserDefaults), !shadow.isEmpty {
                try? KeychainStorage.shared.set(shadow, for: gridDecryptedKeyKey)
                UserDefaults.standard.removeObject(forKey: gridDecryptedKeyShadowUserDefaults)
            }
        }
    }

    // MARK: Persistence

    private func persist(_ session: UserSession) throws -> UserSession {
        let data = try JSONEncoder().encode(session)
        try KeychainStorage.shared.set(data, for: key)
        DispatchQueue.main.async { self.session = session }
        return session
    }

    @discardableResult
    func reload() throws -> UserSession? {
        guard let data = try KeychainStorage.shared.get(key) else {
            DispatchQueue.main.async { self.session = nil }
            return nil
        }
        let decoder = JSONDecoder()
        if let current = try? decoder.decode(UserSession.self, from: data) {
            DispatchQueue.main.async { self.session = current }
            return current
        }
        if let legacy = try? decoder.decode(LegacyAuthSession.self, from: data) {
            let migrated = UserSession(uid: legacy.serverUserUid, name: nil, email: nil, address: legacy.data?.address, gridUserId: nil)
            // Replace legacy payload with minimal format
            return try persist(migrated)
        }
        // Unknown format: clear
        clear()
        return nil
    }

    func clear() {
        try? KeychainStorage.shared.remove(key)
        try? KeychainStorage.shared.remove(gridRawKey)
        try? KeychainStorage.shared.remove(gridDecryptedKeyKey)
        UserDefaults.standard.removeObject(forKey: gridDecryptedKeyShadowUserDefaults)
        try? KeychainStorage.shared.remove(privyAccessTokenKey)
        DispatchQueue.main.async { self.session = nil }
    }

    // MARK: Accessors
    var uid: String? { session?.uid }
    var name: String? { session?.name }
    var email: String? { session?.email }
    var address: String? { session?.address }
    var gridUserIdValue: String? { session?.gridUserId }

    // MARK: Updaters
    @discardableResult
    func setUID(_ uid: String) throws -> UserSession {
        let updated = UserSession(uid: uid, name: session?.name, email: session?.email, address: session?.address, gridUserId: session?.gridUserId)
        return try persist(updated)
    }

    @discardableResult
    func setNameEmail(name: String, email: String) throws -> UserSession {
        let updated = UserSession(uid: session?.uid, name: name, email: email, address: session?.address, gridUserId: session?.gridUserId)
        return try persist(updated)
    }

    @discardableResult
    func setAddress(_ address: String?) throws -> UserSession {
        let updated = UserSession(uid: session?.uid, name: session?.name, email: session?.email, address: address, gridUserId: session?.gridUserId)
        return try persist(updated)
    }

    @discardableResult
    func setGridUserId(_ gridUserId: String?) throws -> UserSession {
        let updated = UserSession(uid: session?.uid, name: session?.name, email: session?.email, address: session?.address, gridUserId: gridUserId)
        return try persist(updated)
    }
}

// MARK: - Grid API

enum GridAPI {
    struct BalanceDisplay { let symbol: String; let amount: Double }

    struct TokenBalance: Decodable {
        let token_address: String
        let amount: Int64
        let amount_decimal: String
        let decimals: Int
        let symbol: String?
        let name: String?
    }
    
    struct BalancesResponse: Decodable {
        let data: DataObj
        struct DataObj: Decodable {
            let tokens: [TokenBalance]?
        }
    }
    
    static func fetchTokenBalances(address: String) async throws -> [TokenBalance]? {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts/\(address)/balances") else { return nil }
        var headers: [String: String] = [ "x-grid-environment": "sandbox" ]
        let apiKey = SecureConfig.gridApiKey()
        if !apiKey.isEmpty { headers["Authorization"] = "Bearer \(apiKey)" }
        let (data, http) = try await APIService.performAbsolute(method: "GET", url: url, headers: headers)
        guard (200..<300).contains(http.statusCode) else { return nil }
        
        let response = try JSONDecoder().decode(BalancesResponse.self, from: data)
        return response.data.tokens
    }
    
    static func fetchPrimaryBalanceDisplay(address: String) async throws -> BalanceDisplay? {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts/\(address)/balances") else { return nil }
        var headers: [String: String] = [ "x-grid-environment": "sandbox" ]
        let apiKey = SecureConfig.gridApiKey()
        if !apiKey.isEmpty { headers["Authorization"] = "Bearer \(apiKey)" }
        let (data, http) = try await APIService.performAbsolute(method: "GET", url: url, headers: headers)
        guard (200..<300).contains(http.statusCode) else { return nil }

        if let dict = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
            if let dataObj = dict["data"] as? [String: Any], let tokens = dataObj["tokens"] as? [[String: Any]] {
                return bestBalance(from: tokens)
            }
            if let balances = dict["balances"] as? [[String: Any]] {
                return bestBalance(from: balances)
            }
            if let accounts = dict["accounts"] as? [[String: Any]] {
                return bestBalance(from: accounts)
            }
        } else if let arr = try? JSONSerialization.jsonObject(with: data) as? [[String: Any]] {
            return bestBalance(from: arr)
        }
        return nil
    }

    enum GridAPIError: Error { case accountExists(String?) }

    private static func bestBalance(from items: [[String: Any]]) -> BalanceDisplay? {
        let mapped: [BalanceDisplay] = items.compactMap { obj in
            let symbol = (obj["symbol"] as? String)
                ?? (obj["token"] as? String)
                ?? (obj["mint"] as? String)
                ?? (obj["token_address"] as? String)
                ?? "BAL"
            if let s = obj["amount_decimal"] as? String, let d = Double(s) { return BalanceDisplay(symbol: symbol, amount: d) }
            let uiAmount = (obj["uiAmount"] as? NSNumber)?.doubleValue
                ?? (obj["ui_amount"] as? NSNumber)?.doubleValue
            if let uiAmount { return BalanceDisplay(symbol: symbol, amount: uiAmount) }
            if let amountNum = obj["amount"] as? NSNumber, let decimals = obj["decimals"] as? Int {
                let val = amountNum.doubleValue / pow(10.0, Double(decimals))
                return BalanceDisplay(symbol: symbol, amount: val)
            }
            if let amountStr = obj["amount"] as? String, let amountInt = Double(amountStr), let decimals = obj["decimals"] as? Int {
                let val = amountInt / pow(10.0, Double(decimals))
                return BalanceDisplay(symbol: symbol, amount: val)
            }
            return nil
        }
        // Prefer SOL if present, else largest balance
        if let sol = mapped.first(where: { $0.symbol.uppercased().contains("SOL") }) { return sol }
        return mapped.sorted(by: { $0.amount > $1.amount }).first
    }


    // MARK: - Account/Auth models and requests with raw capture

    struct EncryptedAuthorizationKey: Decodable { let encapsulatedKey: String; let ciphertext: String }

    struct VerifyDataMinimal: Decodable { let address: String; let gridUserId: String? }

    struct VerifyEnvelope: Decodable { let data: VerifyPayload
        struct VerifyPayload: Decodable {
            let address: String
            let gridUserId: String?
            let authentication: [AuthEntry]?
            struct AuthEntry: Decodable {
                let provider: String?
                let session: SessionObj?
                struct SessionObj: Decodable {
                    let session: WrappedSession?
                    struct WrappedSession: Decodable {
                        let encryptedAuthorizationKey: EncryptedAuthorizationKey?
                    }
                }
            }
        }
    }

    struct VerifyResult { let data: VerifyDataMinimal; let raw: Data
        func firstEncryptedKey() -> EncryptedAuthorizationKey? {
            if let dict = try? JSONSerialization.jsonObject(with: raw) as? [String: Any],
               let dataObj = dict["data"] as? [String: Any],
               let authArr = dataObj["authentication"] as? [[String: Any]] {
                for entry in authArr {
                    if let session = entry["session"] as? [String: Any] {
                        // The example shows provider object key; support nested object as well
                        for (_, val) in session {
                            if let sdict = val as? [String: Any],
                               let wrapped = sdict["session"] as? [String: Any],
                               let enc = wrapped["encrypted_authorization_key"] as? [String: Any],
                               let ek = enc["encapsulated_key"] as? String,
                               let ct = enc["ciphertext"] as? String {
                                return EncryptedAuthorizationKey(encapsulatedKey: ek, ciphertext: ct)
                            }
                        }
                    }
                }
            }
            return nil
        }
    }

    private static func gridHeaders() -> [String: String] {
        var headers: [String: String] = ["x-grid-environment": "sandbox"]
        let apiKey = SecureConfig.gridApiKey()
        if !apiKey.isEmpty { headers["Authorization"] = "Bearer \(apiKey)" }
        return headers
    }

    static func createAccount(email: String, memo: String?) async throws {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts") else { throw URLError(.badURL) }
        struct Body: Encodable { let type: String = "email"; let email: String; let memo: String? }
        let body = Body(email: email, memo: memo)
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: gridHeaders(), jsonBody: body)
        guard (200..<300).contains(http.statusCode) else {
            if http.statusCode == 400 {
                if let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any],
                   let details = obj["details"] as? [[String: Any]] {
                    let hasExists = details.contains(where: { ($0["code"] as? String) == "grid_account_already_exists_for_user" })
                    if hasExists { throw GridAPIError.accountExists(obj["message"] as? String) }
                }
            }
            throw URLError(.badServerResponse)
        }
    }

    struct GridVerifyAccountRequest: Encodable { let email: String; let otpCode: String; let kmsProviderConfig: KMSConfig
        struct KMSConfig: Encodable { let encryptionPublicKey: String }
    }

    static func verifyAccount(email: String, otpCode: String) async throws -> VerifyResult {
        let pubB64 = try HPKEManager.shared.publicKeyDERBase64()
        let body = GridVerifyAccountRequest(email: email, otpCode: otpCode, kmsProviderConfig: .init(encryptionPublicKey: pubB64))
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts/verify") else { throw URLError(.badURL) }
        var headers = gridHeaders()
        headers["x-idempotency-key"] = UUID().uuidString
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: headers, jsonBody: body)
        guard (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        let decoded = try JSONDecoder().decode(VerifyEnvelope.self, from: data)
        let minimal = VerifyDataMinimal(address: decoded.data.address, gridUserId: decoded.data.gridUserId)
        return VerifyResult(data: minimal, raw: data)
    }

    static func authInitiate(email: String) async throws {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/auth") else { throw URLError(.badURL) }
        struct Body: Encodable { let email: String; let provider: String? }
        let body = Body(email: email, provider: "privy")
        _ = try await APIService.performAbsolute(method: "POST", url: url, headers: gridHeaders(), jsonBody: body)
    }

    static func authVerify(email: String, otpCode: String) async throws -> VerifyResult {
        let pubB64 = try HPKEManager.shared.publicKeyDERBase64()
        struct AuthVerifyBody: Encodable { let email: String; let otp_code: String; let kms_provider: String; let kms_provider_config: KMS
            struct KMS: Encodable { let encryption_public_key: String }
        }
        let body = AuthVerifyBody(email: email, otp_code: otpCode, kms_provider: "privy", kms_provider_config: .init(encryption_public_key: pubB64))
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/auth/verify") else { throw URLError(.badURL) }
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: gridHeaders(), jsonBody: body)
        guard (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        let decoded = try JSONDecoder().decode(VerifyEnvelope.self, from: data)
        let minimal = VerifyDataMinimal(address: decoded.data.address, gridUserId: decoded.data.gridUserId)
        return VerifyResult(data: minimal, raw: data)
    }
}

// MARK: - Session extensions for Grid raw and decrypted key

extension SessionManager {
    func setGridRawResponse(_ data: Data) throws {
        try KeychainStorage.shared.set(data, for: gridRawKey)
    }

    func setDecryptedAuthorizationKey(_ data: Data) throws {
        try KeychainStorage.shared.set(data, for: gridDecryptedKeyKey)
        // Shadow copy to UserDefaults for resilience across keychain resets (non-secure fallback, acceptable for sandbox/devnet)
        UserDefaults.standard.set(data, forKey: gridDecryptedKeyShadowUserDefaults)
        print("[GridAuthKey] Stored decrypted authorization key: \(data.count) bytes")
    }

    func getDecryptedAuthorizationKey() -> Data? {
        return try? KeychainStorage.shared.get(gridDecryptedKeyKey)
    }

    func getGridRawResponse() -> Data? {
        return try? KeychainStorage.shared.get(gridRawKey)
    }

    func ensureAddressFromStoredGrid() {
        if session?.address != nil { return }
        guard let raw = getGridRawResponse() else { return }
        if let dict = try? JSONSerialization.jsonObject(with: raw) as? [String: Any] {
            if let dataObj = dict["data"] as? [String: Any], let addr = dataObj["address"] as? String, !addr.isEmpty {
                _ = try? setAddress(addr)
                return
            }
        }
    }

    func refreshDecryptedAuthorizationKeyIfPossible() {
        if let key = getDecryptedAuthorizationKey(), !key.isEmpty {
            print("[GridAuthKey] Decrypted key already present: \(key.count) bytes")
            return
        }
        guard let raw = getGridRawResponse() else { return }
        if let json = try? JSONSerialization.jsonObject(with: raw) as? [String: Any] {
            if let (ek, ct) = SessionManager.findEncryptedAuthKey(in: json) {
                print("[GridAuthKey] Found encrypted key: encapsulated=\(ek.count)b, ciphertext=\(ct.count)b")
                do {
                    let dec = try HPKEManager.shared.decryptAuthorizationKey(encapsulatedKeyB64: ek, ciphertextB64: ct)
                    print("[GridAuthKey] Decryption success: \(dec.count) bytes")
                    if let raw32 = SessionManager.extractRawP256PrivateKey(fromHPKEPlaintext: dec) {
                        print("[GridAuthKey] Extracted raw P-256 key: \(raw32.count) bytes")
                        try? setDecryptedAuthorizationKey(raw32)
                    } else {
                        print("[GridAuthKey] Failed to extract raw P-256 key from plaintext")
                    }
                    return
                } catch {
                    print("[GridAuthKey] Decryption failed: \(error.localizedDescription)")
                }
            } else {
                print("[GridAuthKey] Encrypted key not found in stored JSON")
            }
        }
    }

    // MARK: Privy access token storage/extraction
    func setPrivyAccessToken(_ token: String) {
        guard let data = token.data(using: .utf8) else { return }
        try? KeychainStorage.shared.set(data, for: privyAccessTokenKey)
        print("[Privy] Stored privy_access_token: \(token.count) chars")
    }

    func getPrivyAccessToken() -> String? {
        do {
            if let data = try KeychainStorage.shared.get(privyAccessTokenKey) {
                if let s = String(data: data, encoding: .utf8), !s.isEmpty {
                    return s
                }
            }
        } catch {
            // ignore; treated as missing
        }
        return nil
    }

    func extractAndStorePrivyAccessToken(from raw: Data) {
        guard let json = try? JSONSerialization.jsonObject(with: raw) else { return }
        if let token = SessionManager.findPrivyAccessToken(in: json) {
            setPrivyAccessToken(token)
        } else {
            print("[Privy] privy_access_token not found in verify response")
        }
    }
}

// MARK: - Helpers to locate encrypted authorization key in varying JSON shapes
extension SessionManager {
    static func findEncryptedAuthKey(in root: Any) -> (String, String)? {
        if let dict = root as? [String: Any] {
            // direct match
            if let enc = dict["encrypted_authorization_key"] as? [String: Any] {
                if let ek = (enc["encapsulated_key"] as? String) ?? (enc["encapsulatedKey"] as? String),
                   let ct = enc["ciphertext"] as? String {
                    return (ek, ct)
                }
            }
            if let enc = dict["encryptedAuthorizationKey"] as? [String: Any] {
                if let ek = (enc["encapsulatedKey"] as? String) ?? (enc["encapsulated_key"] as? String),
                   let ct = enc["ciphertext"] as? String {
                    return (ek, ct)
                }
            }
            // recurse into children
            for (_, value) in dict {
                if let found = findEncryptedAuthKey(in: value) { return found }
            }
        } else if let arr = root as? [Any] {
            for item in arr { if let found = findEncryptedAuthKey(in: item) { return found } }
        }
        return nil
    }

    static func findPrivyAccessToken(in root: Any) -> String? {
        if let dict = root as? [String: Any] {
            if let token = dict["privy_access_token"] as? String, !token.isEmpty { return token }
            for (_, value) in dict { if let t = findPrivyAccessToken(in: value) { return t } }
        } else if let arr = root as? [Any] {
            for item in arr { if let t = findPrivyAccessToken(in: item) { return t } }
        }
        return nil
    }

    static func extractRawP256PrivateKey(fromHPKEPlaintext plaintext: Data) -> Data? {
        // Strip optional "wallet-auth:" prefix if present
        var data = plaintext
        if let s = String(data: data, encoding: .utf8), s.hasPrefix("wallet-auth:") {
            let stripped = String(s.dropFirst("wallet-auth:".count))
            if let decoded = Data(base64Encoded: stripped) { data = decoded } else { data = Data(stripped.utf8) }
        }
        // If still base64 text, decode
        if let asString = String(data: data, encoding: .utf8), let b64 = Data(base64Encoded: asString) {
            data = b64
        }
        print("[GridAuthKey] Plain candidate len=\(data.count), prefix=\(hexPrefix(data, 10))")
        
        // Robust DER scan: find OCTET STRING of length 32 (pattern 0x04 0x20)
        // In PKCS#8 ECPrivateKey, the private key is at depth 3: SEQUENCE { INTEGER, OCTET STRING(key)}
        let bytes = [UInt8](data)
        
        // Look for pattern [0x04, 0x20] indicating OCTET STRING of 32 bytes
        var candidateIndices: [(index: Int, depth: Int)] = []
        var depth = 0
        var i = 0
        
        while i + 1 < bytes.count {
            if bytes[i] == 0x30 { // SEQUENCE
                depth += 1
            } else if bytes[i] == 0x04 && bytes[i + 1] == 0x20 { // OCTET STRING length 32
                candidateIndices.append((index: i, depth: depth))
            }
            i += 1
        }
        
        // Prefer the deepest OCTET STRING of length 32 (most likely the private key)
        if let bestCandidate = candidateIndices.max(by: { $0.depth < $1.depth }) {
            let idx = bestCandidate.index
            if idx + 2 + 32 <= bytes.count {
                let slice = bytes[(idx + 2)..<(idx + 2 + 32)]
                print("[GridAuthKey] Found OCTET STRING at idx=\(idx), depth=\(bestCandidate.depth), extracted 32 bytes: \(hexPrefix(Data(slice), 8))")
                return Data(slice)
            }
        }
        
        print("[GridAuthKey] OCTET STRING marker 0x04 0x20 not found or too short")
        print("[GridAuthKey] Debug: data bytes=\(data.count), hex=\(hexPrefix(data, 32))")
        return nil
    }
    
    private static func hexPrefix(_ data: Data, _ n: Int) -> String {
        return data.prefix(n).map { String(format: "%02x", $0) }.joined()
    }
}

// MARK: - Solana minimal utils and Grid transfer

enum SolanaRPC {
    struct AnyEncodable: Encodable {
        private let _encode: (Encoder) throws -> Void
        init<T: Encodable>(_ wrapped: T) { _encode = wrapped.encode }
        func encode(to encoder: Encoder) throws { try _encode(encoder) }
    }

    struct GetLatestBlockhashRequest: Encodable {
        let jsonrpc: String = "2.0"
        let id: Int = 1
        let method: String = "getLatestBlockhash"
        let params: [AnyEncodable] = [AnyEncodable(["commitment": "confirmed"])]
    }

    struct GetLatestBlockhashResponse: Decodable {
        struct Result: Decodable {
            struct Value: Decodable { let blockhash: String }
            let value: Value
        }
        let result: Result?
    }

    static func getLatestBlockhash(devnet: Bool = true) async throws -> String {
        let url = URL(string: devnet ? "https://api.devnet.solana.com" : "https://api.mainnet-beta.solana.com")!
        var req = URLRequest(url: url)
        req.httpMethod = "POST"
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        let body = GetLatestBlockhashRequest()
        req.httpBody = try JSONEncoder().encode(body)
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse, (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        let decoded = try JSONDecoder().decode(GetLatestBlockhashResponse.self, from: data)
        guard let hash = decoded.result?.value.blockhash else { throw URLError(.cannotDecodeContentData) }
        return hash
    }
}

enum Base58 {
    private static let alphabet = Array("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz")
    private static let alphabetMap: [Character: Int] = {
        var m: [Character: Int] = [:]
        for (i, c) in alphabet.enumerated() { m[c] = i }
        return m
    }()

    static func decode(_ s: String) throws -> Data {
        var bytes = [UInt8](repeating: 0, count: 0)
        var zeros = 0
        for ch in s { if ch == "1" { zeros += 1 } else { break } }
        var b256: [UInt8] = []
        for ch in s {
            guard let val = alphabetMap[ch] else { throw URLError(.cannotDecodeContentData) }
            var carry = val
            for j in 0..<b256.count {
                let idx = b256.count - 1 - j
                let x = Int(b256[idx]) * 58 + carry
                b256[idx] = UInt8(x & 0xFF)
                carry = x >> 8
            }
            while carry > 0 { b256.insert(UInt8(carry & 0xFF), at: 0); carry >>= 8 }
        }
        let tail = Array(b256.drop(while: { $0 == 0 }))
        bytes = [UInt8](repeating: 0, count: zeros) + tail
        return Data(bytes)
    }
    
    static func encode(_ data: Data) -> String {
        var num = [UInt8](data)
        var zeros = 0
        while zeros < num.count && num[zeros] == 0 { zeros += 1 }
        num = Array(num[zeros...])
        var digits: [Character] = []
        for _ in 0..<zeros { digits.append("1") }
        var idx = 0
        while idx < num.count {
            var carry = Int(num[idx])
            for digit_idx in 0..<digits.count {
                let val = Int(alphabetMap[digits[digit_idx]]!) * 256 + carry
                carry = val / 58
                digits[digit_idx] = alphabet[val % 58]
            }
            while carry > 0 {
                digits.append(alphabet[carry % 58])
                carry /= 58
            }
            idx += 1
        }
        return String(digits.reversed())
    }
}

struct SolanaLegacyInstruction { let programId: Data; let accounts: [UInt8]; let data: Data }

struct SolanaLegacyTransaction {
    let accountKeys: [Data]
    let recentBlockhash: Data
    let instructions: [SolanaLegacyInstruction]
    let numRequiredSignatures: UInt8
    let numReadonlySigned: UInt8
    let numReadonlyUnsigned: UInt8

    func serializeUnsigned() -> Data {
        var out = Data()
        // Signatures length with placeholders (64 zero-bytes each) matching numRequiredSignatures
        out.append(encodeShortVec(Int(numRequiredSignatures)))
        for _ in 0..<numRequiredSignatures { out.append(Data(repeating: 0, count: 64)) }
        // Header
        out.append(numRequiredSignatures)
        out.append(numReadonlySigned)
        out.append(numReadonlyUnsigned)
        // Accounts
        out.append(encodeShortVec(accountKeys.count))
        for key in accountKeys { out.append(key) }
        // Recent blockhash
        out.append(recentBlockhash)
        // Instructions
        out.append(encodeShortVec(instructions.count))
        for ix in instructions {
            // Program id index
            guard let progIndex = accountKeys.firstIndex(of: ix.programId) else {
                print("[TXSerialize] ERROR: Program ID not found in accountKeys")
                print("[TXSerialize] Program ID: \(Base58.encode(ix.programId))")
                print("[TXSerialize] Account keys count: \(accountKeys.count)")
                continue
            }
            print("[TXSerialize] Found program at index \(progIndex)")
            out.append(UInt8(progIndex))
            // Accounts
            out.append(encodeShortVec(ix.accounts.count))
            out.append(contentsOf: ix.accounts)
            // Data
            out.append(encodeShortVec(ix.data.count))
            out.append(ix.data)
        }
        return out
    }
}

private func encodeShortVec(_ len: Int) -> Data { // Solana shortvec
    var rem = len
    var out = Data()
    while true {
        var elem = UInt8(rem & 0x7F)
        rem >>= 7
        if rem == 0 { out.append(elem); break } else { elem |= 0x80; out.append(elem) }
    }
    return out
}

enum SystemProgramBuilder {
    static let programId = try! Base58.decode("11111111111111111111111111111111")

    static func transferInstruction(fromIndex: Int, toIndex: Int, lamports: UInt64) -> SolanaLegacyInstruction {
        var data = Data()
        // Transfer instruction index = 2 (u32 little-endian)
        data.append(contentsOf: [0x02, 0x00, 0x00, 0x00])
        // lamports u64 little-endian
        var amount = lamports
        for _ in 0..<8 { data.append(UInt8(amount & 0xFF)); amount >>= 8 }
        return SolanaLegacyInstruction(programId: programId, accounts: [UInt8(fromIndex), UInt8(toIndex)], data: data)
    }
}

enum TokenProgramBuilder {
    static let programId = try! Base58.decode("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")
    static let associatedTokenProgram = try! Base58.decode("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
    
    static func deriveAssociatedTokenAddress(walletPubkey: Data, mintPubkey: Data) -> (address: Data, bump: UInt8) {
        var bump: UInt8 = 255
        
        print("[ATADerivation] Deriving ATA for wallet: \(Base58.encode(walletPubkey))")
        print("[ATADerivation] Mint: \(Base58.encode(mintPubkey))")
        print("[ATADerivation] Token Program: \(Base58.encode(programId))")
        
        while bump > 0 {
            var digest = SHA256()
            print("[ATADerivation] Hash inputs (in order):")
            print("[ATADerivation]   1. ATA Program: \(Base58.encode(associatedTokenProgram))")
            print("[ATADerivation]   2. Owner: \(Base58.encode(walletPubkey))")
            print("[ATADerivation]   3. Token Program: \(Base58.encode(programId))")
            print("[ATADerivation]   4. Mint: \(Base58.encode(mintPubkey))")
            print("[ATADerivation]   5. Bump: \(bump)")
            
            digest.update(data: associatedTokenProgram)
            digest.update(data: walletPubkey)
            digest.update(data: programId)
            digest.update(data: mintPubkey)
            digest.update(data: Data([bump]))
            
            let address = Data(digest.finalize())
            
            if isOffCurveEd25519(address) {
                print("[ATADerivation] Found off-curve PDA with bump \(bump)")
                print("[ATADerivation] Derived ATA: \(Base58.encode(address))")
                return (address, bump)
            }
            bump -= 1
        }
        
        print("[ATADerivation] ERROR: No valid bump found!")
        return (Data(), 0)
    }
    
    private static func isOffCurveEd25519(_ pubkey: Data) -> Bool {
        guard pubkey.count == 32 else { return false }
        let bytes = Array(pubkey)
        return (bytes[31] & 0x80) == 0
    }
    
    static func createAssociatedTokenAccountInstruction(
        fundingAccountIndex: Int,
        walletAccountIndex: Int,
        mintAccountIndex: Int,
        tokenAccountIndex: Int,
        systemProgramIndex: Int,
        tokenProgramIndex: Int
    ) -> SolanaLegacyInstruction {
        var data = Data()
        data.append(0x00)
        let accountsArray = [
            UInt8(fundingAccountIndex),
            UInt8(tokenAccountIndex),
            UInt8(walletAccountIndex),
            UInt8(mintAccountIndex),
            UInt8(systemProgramIndex),
            UInt8(tokenProgramIndex)
        ]
        print("[CreateATA] Account order: [payer=\(fundingAccountIndex), ATA=\(tokenAccountIndex), owner=\(walletAccountIndex), mint=\(mintAccountIndex), system=\(systemProgramIndex), token=\(tokenProgramIndex)]")
        print("[CreateATA] Using CreateIdempotent (0x00) instead of Create (0x01)")
        return SolanaLegacyInstruction(
            programId: associatedTokenProgram,
            accounts: accountsArray,
            data: data
        )
    }
    
    static func transferInstruction(sourceIndex: Int, destinationIndex: Int, ownerIndex: Int, amount: UInt64) -> SolanaLegacyInstruction {
        var data = Data()
        data.append(0x03)
        var amt = amount
        for _ in 0..<8 { data.append(UInt8(amt & 0xFF)); amt >>= 8 }
        return SolanaLegacyInstruction(programId: programId, accounts: [UInt8(sourceIndex), UInt8(destinationIndex), UInt8(ownerIndex)], data: data)
    }
}

enum SolanaTransferBuilder {
    struct TransferRequest {
        let fromAddress: String
        let toAddress: String
        let amountSOL: Double
    }
    
    struct TransferResult {
        let unsignedTransactionB64: String
    }
    
    static func buildTransfer(_ request: TransferRequest) async throws -> TransferResult {
        let lamports = UInt64(request.amountSOL * 1_000_000_000)
        
        let recentBlockhash = try await SolanaRPC.getLatestBlockhash(devnet: true)
        let recentBlockhashData = try Base58.decode(recentBlockhash)
        
        let fromKey = try Base58.decode(request.fromAddress)
        let toKey = try Base58.decode(request.toAddress)
        let systemProgramId = SystemProgramBuilder.programId
        
        let transferIx = SystemProgramBuilder.transferInstruction(fromIndex: 0, toIndex: 1, lamports: lamports)
        
        let tx = SolanaLegacyTransaction(
            accountKeys: [fromKey, toKey, systemProgramId],
            recentBlockhash: recentBlockhashData,
            instructions: [transferIx],
            numRequiredSignatures: 1,
            numReadonlySigned: 0,
            numReadonlyUnsigned: 1
        )
        
        let serialized = tx.serializeUnsigned()
        let b64 = serialized.base64EncodedString()
        
        return TransferResult(unsignedTransactionB64: b64)
    }
}

enum GridTransfers {
    struct PrepareResponse: Decodable { struct DataObj: Decodable { let transaction: String; let kms_payloads: [AnyCodable]?; let transaction_signers: [String]? }; let data: DataObj }
    struct AnyCodable: Codable {}
    struct SubmitResponse: Decodable { struct DataObj: Decodable { let transaction_signature: String? }; let data: DataObj }
    struct PrepareOutput { let decoded: PrepareResponse; let raw: Data }

    private static func headers() -> [String: String] { var h = ["x-grid-environment": "sandbox"]; let key = SecureConfig.gridApiKey(); if !key.isEmpty { h["Authorization"] = "Bearer \(key)" }; return h }

    static func prepare(accountAddress: String, transactionB64: String, signers: [String]?, payerAddress: String) async throws -> PrepareOutput {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts/\(accountAddress)/transactions") else { throw URLError(.badURL) }
        struct Body: Encodable {
            let transaction: String
            let transaction_signers: [String]?
            let fee_config: Fee
            struct Fee: Encodable { let currency: String; let payer_address: String }
        }
        let body = Body(transaction: transactionB64, transaction_signers: signers, fee_config: .init(currency: "sol", payer_address: payerAddress))
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: headers(), jsonBody: body)
        guard (200..<300).contains(http.statusCode) else { throw URLError(.badServerResponse) }
        let decoded = try JSONDecoder().decode(PrepareResponse.self, from: data)
        return PrepareOutput(decoded: decoded, raw: data)
    }

    static func submit(accountAddress: String, preparedTransactionB64: String, kmsPayloadSignatures: [KMSPayloadSignature]) async throws -> SubmitResponse {
        guard let url = URL(string: "https://grid.squads.xyz/api/grid/v1/accounts/\(accountAddress)/submit") else { throw URLError(.badURL) }
        struct KMSSig: Encodable { let provider: String; let signature: String }
        struct Body: Encodable { let transaction: String; let kms_payloads: [KMSSig] }
        let body = Body(transaction: preparedTransactionB64, kms_payloads: kmsPayloadSignatures.map { KMSSig(provider: $0.provider, signature: $0.signatureBase64) })
        let (data, http) = try await APIService.performAbsolute(method: "POST", url: url, headers: headers(), jsonBody: body)
        guard (200..<300).contains(http.statusCode) else {
            let bodyStr = String(data: data, encoding: .utf8) ?? "<non-utf8>"
            let err = NSError(domain: "GridSubmit", code: http.statusCode, userInfo: [NSLocalizedDescriptionKey: "Grid submit failed (status=\(http.statusCode)): \n\(bodyStr)"])
            throw err
        }
        return try JSONDecoder().decode(SubmitResponse.self, from: data)
    }

    struct KMSPayloadSignature { let provider: String; let signatureBase64: String }
}

enum CanonicalJSON {
    static func canonicalData(from value: Any) throws -> Data {
        let s = try canonicalString(from: value)
        guard let d = s.data(using: .utf8) else { throw NSError(domain: "CanonicalJSON", code: -1, userInfo: [NSLocalizedDescriptionKey: "Cannot encode UTF-8 data"]) }
        return d
    }

    private static func canonicalString(from value: Any) throws -> String {
        if let dict = value as? [String: Any] {
            let keys = dict.keys.sorted()
            let parts: [String] = try keys.map { key in
                let v = dict[key]!
                return "\(try canonicalString(from: key)):\(try canonicalString(from: v))"
            }
            return "{\(parts.joined(separator: ","))}"
        } else if let arr = value as? [Any] {
            let parts: [String] = try arr.map { try canonicalString(from: $0) }
            return "[\(parts.joined(separator: ","))]"
        } else if let s = value as? String {
            return jsonEscapeString(s)
        } else if let n = value as? NSNumber {
            if CFGetTypeID(n) == CFBooleanGetTypeID() {
                return n.boolValue ? "true" : "false"
            }
            return try jsonNumberString(n)
        } else if value is NSNull { return "null" }
        else if let b = value as? Bool { return b ? "true" : "false" }
        else { throw NSError(domain: "CanonicalJSON", code: -1, userInfo: [NSLocalizedDescriptionKey: "Unsupported JSON value type"]) }
    }

    private static func jsonEscapeString(_ s: String) -> String {
        var out = "\""
        for ch in s.unicodeScalars {
            switch ch.value {
            case 0x22: out += "\\\""
            case 0x5C: out += "\\\\"
            case 0x08: out += "\\b"
            case 0x0C: out += "\\f"
            case 0x0A: out += "\\n"
            case 0x0D: out += "\\r"
            case 0x09: out += "\\t"
            case 0x00...0x1F:
                let hex = String(format: "%04X", ch.value)
                out += "\\u\(hex)"
            default:
                out.unicodeScalars.append(ch)
            }
        }
        out += "\""
        return out
    }

    private static func jsonNumberString(_ n: NSNumber) throws -> String {
        if n.doubleValue.isNaN || n.doubleValue.isInfinite { throw NSError(domain: "CanonicalJSON", code: -1, userInfo: [NSLocalizedDescriptionKey: "Invalid number value"]) }
        let formatter = NumberFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.numberStyle = .decimal
        formatter.maximumFractionDigits = 340
        formatter.usesGroupingSeparator = false
        guard let s = formatter.string(from: n) else { throw NSError(domain: "CanonicalJSON", code: -1, userInfo: [NSLocalizedDescriptionKey: "Failed to format number"]) }
        return s
    }
}

enum GridKMSSigner {
    // Local-only signing for Grid email-based accounts (no Privy RPC)
    static func signKMSpayloadsIfNeeded(prepareData: Data) throws -> [GridTransfers.KMSPayloadSignature] {
        guard let dict = try JSONSerialization.jsonObject(with: prepareData) as? [String: Any], let dataObj = dict["data"] as? [String: Any] else { return [] }
        let payloads = (dataObj["kms_payloads"] as? [Any]) ?? []
        guard !payloads.isEmpty else { return [] }
        // Local raw key only
        var keyData = SessionManager.shared.getDecryptedAuthorizationKey()
        if keyData == nil || keyData?.isEmpty == true { SessionManager.shared.refreshDecryptedAuthorizationKeyIfPossible(); keyData = SessionManager.shared.getDecryptedAuthorizationKey() }
        guard let keyData, !keyData.isEmpty else { throw NSError(domain: "KMS", code: URLError.Code.userAuthenticationRequired.rawValue, userInfo: [NSLocalizedDescriptionKey: "Missing decrypted authorization key. Re-authenticate and ensure OTP decrypts key."]) }
        let priv = try P256.Signing.PrivateKey(rawRepresentation: keyData)
        var out: [GridTransfers.KMSPayloadSignature] = []
        for entry in payloads {
            var canonicalBytes: Data
            if let obj = entry as? [String: Any], let payloadField = obj["payload"] {
                if let payloadB64 = payloadField as? String, let payloadData = Data(base64Encoded: payloadB64) {
                    // Payload is base64-encoded JSON
                    if let json = try? JSONSerialization.jsonObject(with: payloadData) {
                        canonicalBytes = try CanonicalJSON.canonicalData(from: json)
                    } else {
                        // If not JSON, sign raw bytes
                        canonicalBytes = payloadData
                    }
                } else if let payloadObj = payloadField as? [String: Any] {
                    canonicalBytes = try CanonicalJSON.canonicalData(from: payloadObj)
                } else {
                    canonicalBytes = try CanonicalJSON.canonicalData(from: obj)
                }
            } else {
                canonicalBytes = try CanonicalJSON.canonicalData(from: entry)
            }
            let digest = SHA256.hash(data: canonicalBytes)
            let signature = try priv.signature(for: digest)
            // Grid KMS expects ECDSA P-256 signature; use DER encoding as commonly required
            let b64 = Data(signature.derRepresentation).base64EncodedString()
            out.append(.init(provider: "privy", signatureBase64: b64))
        }
        return out
    }
}


