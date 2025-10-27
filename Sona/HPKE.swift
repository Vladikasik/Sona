import Foundation
import CryptoKit
import SwiftHPKE

final class HPKEManager {
    static let shared = HPKEManager()

    private let privateKeyKey = "hpke_private_raw"
    private let publicKeyKey = "hpke_public_der"

    private init() {}

    // Returns Base64(SPKI DER of P-256 public key). Generates and stores keypair if missing.
    func publicKeyDERBase64() throws -> String {
        if let existing = try KeychainStorage.shared.get(publicKeyKey), !existing.isEmpty {
            return existing.base64EncodedString()
        }
        try generateAndStoreKeyPair()
        guard let pub = try KeychainStorage.shared.get(publicKeyKey) else { throw URLError(.cannotCreateFile) }
        return pub.base64EncodedString()
    }

    func ensureKeysPresent() throws {
        let priv = try KeychainStorage.shared.get(privateKeyKey)
        let pub = try KeychainStorage.shared.get(publicKeyKey)
        if priv == nil || pub == nil { try generateAndStoreKeyPair() }
    }

    // Decrypts using RFC9180: DHKEM(P-256, HKDF-SHA256) + Chacha20-Poly1305, base mode, empty info/AAD
    func decryptAuthorizationKey(encapsulatedKeyB64: String, ciphertextB64: String) throws -> Data {
        guard let privRaw = try KeychainStorage.shared.get(privateKeyKey) else { throw URLError(.userAuthenticationRequired) }
        
        guard let encInput = Data(base64Encoded: encapsulatedKeyB64), let ct = Data(base64Encoded: ciphertextB64) else {
            throw URLError(.cannotDecodeContentData)
        }
        print("[HPKE] EncapsulatedKey b64Len=\(encapsulatedKeyB64.count), decoded=\(encInput.count)b, prefix=\(hexPrefix(encInput, 10))")
        print("[HPKE] Ciphertext b64Len=\(ciphertextB64.count), decoded=\(ct.count)b, suffix=\(hexSuffix(ct, 10))")

        // Parse encapsulated public key to uncompressed X9.63
        let encX963: Data
        if encInput.first == 0x04 && encInput.count == 65 {
            encX963 = encInput
        } else if let parsed = try? x963FromSPKIDER(encInput) {
            encX963 = parsed
        } else {
            throw URLError(.cannotDecodeContentData)
        }
        print("[HPKE] Parsed enc X9.63 len=\(encX963.count), startsWith04=\(encX963.first == 0x04)")
        
        // Create cipher suite: P-256 + HKDF-SHA256 + ChaCha20-Poly1305
        let suite = CipherSuite(kem: .P256, kdf: .KDF256, aead: .CHACHAPOLY)
        
        // Create private key from raw bytes
        let privateKey = try PrivateKey(kem: .P256, bytes: Array(privRaw))
        
        // Use SwiftHPKE library to decrypt
        let plaintext = try suite.open(
            privateKey: privateKey,
            info: [],  // empty info as per Privy
            ct: Array(ct),
            aad: [],   // empty AAD as per Privy
            encap: Array(encX963)
        )
        
        let utf8 = String(data: Data(plaintext), encoding: .utf8) ?? "<non-utf8>"
        let preview = utf8 == "<non-utf8>" ? hexPrefix(Data(plaintext), 16) : String(utf8.prefix(48))
        print("[HPKE] Decrypt OK. plainLen=\(plaintext.count), wallet-auth prefix=\(utf8.hasPrefix("wallet-auth:")), preview=\(preview)")
        return Data(plaintext)
    }

    // MARK: - Internals

    private func generateAndStoreKeyPair() throws {
        let priv = P256.KeyAgreement.PrivateKey()
        let pub = priv.publicKey
        let privRaw = priv.rawRepresentation // 32 bytes
        let pubDER = try spkiDERFromUncompressedX963(pub.x963Representation)
        try KeychainStorage.shared.set(privRaw, for: privateKeyKey)
        try KeychainStorage.shared.set(pubDER, for: publicKeyKey)
    }

    private func spkiDERFromUncompressedX963(_ x963: Data) throws -> Data {
        // SubjectPublicKeyInfo ::= SEQUENCE { algorithm AlgorithmIdentifier, subjectPublicKey BIT STRING }
        // ecPublicKey OID 1.2.840.10045.2.1; prime256v1 OID 1.2.840.10045.3.1.7
        let algId: [UInt8] = [
            0x30, 0x13, // SEQUENCE len 19
            0x06, 0x07, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x02, 0x01, // ecPublicKey
            0x06, 0x08, 0x2A, 0x86, 0x48, 0xCE, 0x3D, 0x03, 0x01, 0x07  // prime256v1
        ]
        var spkBits = Data([0x00]) // unused bits = 0
        spkBits.append(x963)
        let spk = derBitString(spkBits)
        let alg = Data(algId)
        let spkiSeq = derSequence(alg + spk)
        return spkiSeq
    }

    private func derSequence(_ content: Data) -> Data {
        var out = Data([0x30])
        out.append(derLength(content.count))
        out.append(content)
        return out
    }

    private func derBitString(_ content: Data) -> Data {
        var out = Data([0x03])
        out.append(derLength(content.count))
        out.append(content)
        return out
    }

    private func derLength(_ length: Int) -> Data {
        if length < 0x80 {
            return Data([UInt8(length)])
        }
        var len = length
        var bytes: [UInt8] = []
        while len > 0 {
            bytes.insert(UInt8(len & 0xFF), at: 0)
            len >>= 8
        }
        var out = Data([0x80 | UInt8(bytes.count)])
        out.append(contentsOf: bytes)
        return out
    }

    // MARK: - Debug helpers (safe previews)
    private func hexPrefix(_ data: Data, _ n: Int) -> String {
        return data.prefix(n).map { String(format: "%02x", $0) }.joined()
    }

    private func hexSuffix(_ data: Data, _ n: Int) -> String {
        return data.suffix(n).map { String(format: "%02x", $0) }.joined()
    }

    // Extract uncompressed X9.63 bytes from SPKI DER
    private func x963FromSPKIDER(_ der: Data) throws -> Data {
        // Find BIT STRING tag 0x03
        var i = 0
        func readLength(_ data: Data, _ idx: inout Int) throws -> Int {
            guard idx < data.count else { throw URLError(.cannotDecodeContentData) }
            let first = data[idx]; idx += 1
            if first & 0x80 == 0 { return Int(first) }
            let count = Int(first & 0x7F)
            guard count > 0, idx + count <= data.count else { throw URLError(.cannotDecodeContentData) }
            var len = 0
            for _ in 0..<count { len = (len << 8) | Int(data[idx]); idx += 1 }
            return len
        }
        // Skip outer SEQUENCE
        guard i < der.count, der[i] == 0x30 else { throw URLError(.cannotDecodeContentData) }
        i += 1; _ = try readLength(der, &i)
        // Skip AlgorithmIdentifier (SEQUENCE)
        guard i < der.count, der[i] == 0x30 else { throw URLError(.cannotDecodeContentData) }
        i += 1; let algLen = try readLength(der, &i); i += algLen
        // Read BIT STRING
        guard i < der.count, der[i] == 0x03 else { throw URLError(.cannotDecodeContentData) }
        i += 1; let bitLen = try readLength(der, &i)
        guard i < der.count else { throw URLError(.cannotDecodeContentData) }
        let unusedBits = der[i]; i += 1
        guard unusedBits == 0 else { throw URLError(.cannotDecodeContentData) }
        let end = i + (bitLen - 1)
        guard end <= der.count else { throw URLError(.cannotDecodeContentData) }
        let x963 = der[i..<end]
        guard x963.first == 0x04, x963.count == 65 else { throw URLError(.cannotDecodeContentData) }
        return Data(x963)
    }
}


