import Foundation

enum SecureConfig {
    private static let gridApiKeyObfuscated: [UInt8] = [106, 98, 108, 111, 106, 102, 80, 83, 79, 85, 2, 85, 7, 74, 92, 92, 14, 95, 65, 84, 13, 10, 68, 92, 70, 22, 21, 66, 71, 67, 27, 77, 75, 25, 78, 30]
    private static let mask: UInt8 = 0x5A

    static func gridApiKey() -> String {
        let bytes = gridApiKeyObfuscated.enumerated().map { i, b in b ^ (mask &+ UInt8(i)) }
        return String(data: Data(bytes), encoding: .utf8) ?? ""
    }
}


