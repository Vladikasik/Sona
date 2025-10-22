//
//  ContentView.swift
//  Sona
//
//  Created by Vladislav Ainshtein on 21.10.25.
//

import SwiftUI

struct ContentView: View {
    private enum FlowState {
        case selection
        case parentOnboarding
        case childOnboarding
    }

    private enum UserRole: String {
        case parent
        case child
    }

    @AppStorage("userRole") private var storedRole: String?
    @AppStorage("parentName") private var parentName: String = ""
    @AppStorage("parentId") private var parentId: String = ""
    @AppStorage("parentServerId") private var parentServerId: Int = 0
    @AppStorage("parentUID") private var parentUID: String = ""
    @AppStorage("childName") private var childName: String = ""
    @AppStorage("childParentId") private var childParentId: String = ""
    @AppStorage("childParentIdInt") private var childParentIdInt: Int = 0
    @AppStorage("kidId") private var kidId: Int = 0

    @State private var flow: FlowState = .selection
    @State private var alertMessage: String?

    var body: some View {
        Group {
            if let roleString = storedRole, let role = UserRole(rawValue: roleString) {
                switch role {
                case .parent:
                    ParentMainView(name: parentName, parentId: parentServerId, parentUID: parentUID, onSignOut: signOut, onError: { alertMessage = $0 })
                case .child:
                    ChildMainView(onSignOut: signOut, onError: { alertMessage = $0 })
                }
            } else {
                switch flow {
                case .selection:
                    RoleSelectionView(
                        onSelectParent: { flow = .parentOnboarding },
                        onSelectChild: { flow = .childOnboarding }
                    )
                case .parentOnboarding:
                    ParentOnboardingView(onComplete: { name in
                        let trimmed = name.trimmingCharacters(in: .whitespacesAndNewlines)
                        guard !trimmed.isEmpty else { return }
                        Task {
                            do {
                                let res = try await APIService.registerParent(name: trimmed)
                                parentName = trimmed
                                parentServerId = Int(res.parentId)
                                parentUID = res.parentUID
                                storedRole = UserRole.parent.rawValue
                            } catch {
                                alertMessage = error.localizedDescription
                            }
                        }
                    })
                case .childOnboarding:
                    ChildOnboardingView(onComplete: { name, parent in
                        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
                        let trimmedParent = parent.trimmingCharacters(in: .whitespacesAndNewlines)
                        guard let parentInt = Int(trimmedParent), parentInt > 0 else {
                            alertMessage = "Invalid parent ID"
                            return
                        }
                        Task {
                            do {
                                let res = try await APIService.registerKid(name: trimmedName, parentId: Int64(parentInt))
                                childName = trimmedName
                                childParentIdInt = parentInt
                                kidId = Int(res.kidId)
                                storedRole = UserRole.child.rawValue
                            } catch {
                                alertMessage = error.localizedDescription
                            }
                        }
                    })
                }
            }
        }
        .onAppear {
            if storedRole == nil {
                flow = .selection
            }
        }
        .alert(item: $alertMessage.asIdentifiable) { msg in
            Alert(title: Text("Error"), message: Text(msg.value), dismissButton: .default(Text("OK")))
        }
    }

    private static func generateParentId() -> String {
        let raw = UUID().uuidString.replacingOccurrences(of: "-", with: "")
        return String(raw.prefix(8)).uppercased()
    }

    private func signOut() {
        storedRole = nil
        parentName = ""
        parentId = ""
        parentServerId = 0
        parentUID = ""
        childName = ""
        childParentId = ""
        childParentIdInt = 0
        kidId = 0
        flow = .selection
    }
}

private struct RoleSelectionView: View {
    let onSelectParent: () -> Void
    let onSelectChild: () -> Void

    var body: some View {
        VStack(spacing: 24) {
            Text("Welcome to Sona").font(.title).fontWeight(.semibold)
            VStack(spacing: 12) {
                Button(action: onSelectParent) {
                    Text("I'm a parent")
                        .frame(maxWidth: .infinity)
                        .padding()
                        .background(Color.accentColor.opacity(0.15))
                        .foregroundColor(.accentColor)
                        .cornerRadius(12)
                }
                Button(action: onSelectChild) {
                    Text("I'm a child")
                        .frame(maxWidth: .infinity)
                        .padding()
                        .background(Color.accentColor)
                        .foregroundColor(.white)
                        .cornerRadius(12)
                }
            }
            .padding(.horizontal)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
        .padding()
    }
}

private struct ParentOnboardingView: View {
    @State private var name: String = ""
    let onComplete: (String) -> Void

    var body: some View {
        Form {
            Section(header: Text("Parent")) {
                TextField("Your name", text: $name)
                    .textInputAutocapitalization(.words)
            }
            Section {
                Button(action: { onComplete(name.trimmingCharacters(in: .whitespacesAndNewlines)) }) {
                    Text("Continue")
                        .frame(maxWidth: .infinity)
                }
                .disabled(!isValid)
            }
        }
    }

    private var isValid: Bool {
        !name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty
    }
}

private struct ChildOnboardingView: View {
    @State private var name: String = ""
    @State private var parentId: String = ""
    let onComplete: (String, String) -> Void

    var body: some View {
        Form {
            Section(header: Text("Child")) {
                TextField("Your name", text: $name)
                    .textInputAutocapitalization(.words)
                TextField("Parent ID", text: $parentId)
                    .textInputAutocapitalization(.characters)
                    .autocorrectionDisabled(true)
            }
            Section {
                Button(action: {
                    onComplete(
                        name.trimmingCharacters(in: .whitespacesAndNewlines),
                        parentId.trimmingCharacters(in: .whitespacesAndNewlines).uppercased()
                    )
                }) {
                    Text("Continue")
                        .frame(maxWidth: .infinity)
                }
                .disabled(!isValid)
            }
        }
    }

    private var isValid: Bool {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedId = parentId.trimmingCharacters(in: .whitespacesAndNewlines)
        return !trimmedName.isEmpty && trimmedId.count >= 4
    }
}

private struct ParentMainView: View {
    let name: String
    let parentId: Int
    let parentUID: String
    let onSignOut: () -> Void
    let onError: (String) -> Void

    @State private var parentBalanceCents: Int64 = 0
    @State private var kids: [APIService.Kid] = []

    var body: some View {
        List {
            Section(header: Text("Account")) {
                HStack {
                    Text("Name")
                    Spacer()
                    Text(name).foregroundColor(.secondary)
                }
                HStack {
                    Text("ID")
                    Spacer()
                    Text(String(parentId)).font(.system(.body, design: .monospaced)).foregroundColor(.secondary)
                }
                HStack {
                    Text("UID")
                    Spacer()
                    Text(parentUID).font(.system(.body, design: .monospaced)).foregroundColor(.secondary)
                }
            }
            Section(header: Text("Balance")) {
                Text(formatCents(parentBalanceCents)).font(.title2)
                Button("Top up $100") { Task { await topUp(amountCents: 10_000) } }
            }
            Section(header: Text("Kids")) {
                if kids.isEmpty {
                    Text("No kids yet")
                } else {
                    ForEach(kids, id: \.id) { kid in
                        HStack {
                            VStack(alignment: .leading) {
                                Text(kid.name)
                                Text(formatCents(kid.balance)).font(.footnote).foregroundColor(.secondary)
                            }
                            Spacer()
                            Button("Send $50") { Task { await send(to: kid, amountCents: 5_000) } }
                                .buttonStyle(.borderedProminent)
                        }
                    }
                }
            }
            Section {
                Button("Sign out", role: .destructive, action: onSignOut)
                    .frame(maxWidth: .infinity)
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Parent")
        .refreshable { await reload() }
        .task { await reload() }
    }

    private func reload() async {
        guard parentId > 0 else { return }
        do {
            async let parent = APIService.getParent(id: Int64(parentId), name: nil)
            async let kidsRes = APIService.kidsList(parentId: Int64(parentId))
            let (p, kl) = try await (parent, kidsRes)
            parentBalanceCents = p.balance
            kids = kl.kids
        } catch {
            onError(error.localizedDescription)
        }
    }

    private func topUp(amountCents: Int64) async {
        guard parentId > 0 else { return }
        do {
            let res = try await APIService.parentTopUp(parentId: Int64(parentId), amount: amountCents)
            parentBalanceCents = res.parentBalance
            await reload()
        } catch {
            onError(error.localizedDescription)
        }
    }

    private func send(to kid: APIService.Kid, amountCents: Int64) async {
        guard parentId > 0 else { return }
        do {
            let res = try await APIService.sendKidMoney(parentId: Int64(parentId), kidId: kid.id, amount: amountCents)
            parentBalanceCents = res.parentBalance
            if let idx = kids.firstIndex(where: { $0.id == kid.id }) {
                kids[idx].balance = res.kidBalance
            }
        } catch {
            onError(error.localizedDescription)
        }
    }
}

private struct ChildMainView: View {
    @AppStorage("childName") private var childName: String = ""
    @AppStorage("childParentId") private var parentId: String = ""
    @AppStorage("kidId") private var kidId: Int = 0
    let onSignOut: () -> Void
    let onError: (String) -> Void

    @State private var childBalanceCents: Int64 = 0

    var body: some View {
        List {
            Section(header: Text("Account")) {
                HStack {
                    Text("Name")
                    Spacer()
                    Text(childName).foregroundColor(.secondary)
                }
                HStack {
                    Text("Parent ID")
                    Spacer()
                    Text(parentId).font(.system(.body, design: .monospaced)).foregroundColor(.secondary)
                }
            }
            Section(header: Text("Balance")) {
                Text(formatCents(childBalanceCents)).font(.title2)
            }
            Section {
                Button("Sign out", role: .destructive, action: onSignOut)
                    .frame(maxWidth: .infinity)
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Child")
        .refreshable { await reload() }
        .task { await reload() }
    }

    private func reload() async {
        do {
            if kidId > 0 {
                let kid = try await APIService.getChild(id: Int64(kidId), name: nil)
                childBalanceCents = kid.balance
            } else if !childName.isEmpty {
                let kid = try await APIService.getChild(id: nil, name: childName)
                childBalanceCents = kid.balance
            }
        } catch {
            onError(error.localizedDescription)
        }
    }
}

#Preview {
    ContentView()
}

// MARK: - API Layer

private enum APIService {
    static let baseURL = URL(string: "http://95.163.223.236:33777")!

    struct APIErrorResponse: Decodable, Error { let error: String }

    struct RegisterParentResponse: Decodable { let parentId: Int64; let parentUID: String }
    struct RegisterKidResponse: Decodable { let kidId: Int64 }
    struct ParentBalanceResponse: Decodable { let parentBalance: Int64 }
    struct SendKidMoneyResponse: Decodable { let parentBalance: Int64; let kidBalance: Int64 }

    struct Parent: Decodable { let id: Int64; let uid: String; let name: String; let balance: Int64 }
    struct Kid: Decodable, Identifiable { let id: Int64; let name: String; var balance: Int64 }
    struct KidsListResponse: Decodable { var kids: [Kid] }

    static func registerParent(name: String) async throws -> RegisterParentResponse {
        try await request(path: "/register_parent", method: "POST", query: ["name": name])
    }

    static func registerKid(name: String, parentId: Int64) async throws -> RegisterKidResponse {
        try await request(path: "/register_kid", method: "POST", query: ["name": name, "parent_id": String(parentId)])
    }

    static func kidsList(parentId: Int64) async throws -> KidsListResponse {
        try await request(path: "/kids_list", method: "GET", query: ["parent_id": String(parentId)])
    }

    static func parentTopUp(parentId: Int64, amount: Int64) async throws -> ParentBalanceResponse {
        try await request(path: "/parent_topup", method: "POST", query: ["parent_id": String(parentId), "amount": String(amount)])
    }

    static func sendKidMoney(parentId: Int64, kidId: Int64, amount: Int64) async throws -> SendKidMoneyResponse {
        try await request(path: "/send_kid_money", method: "POST", query: ["parent_id": String(parentId), "kid_id": String(kidId), "amount": String(amount)])
    }

    static func getParent(id: Int64?, name: String?) async throws -> Parent {
        var q: [String: String] = [:]
        if let id { q["id"] = String(id) }
        if let name { q["name"] = name }
        return try await request(path: "/get_parent", method: "GET", query: q)
    }

    static func getChild(id: Int64?, name: String?) async throws -> Kid {
        var q: [String: String] = [:]
        if let id { q["id"] = String(id) }
        if let name { q["name"] = name }
        return try await request(path: "/get_child", method: "GET", query: q)
    }

    private static func request<T: Decodable>(path: String, method: String, query: [String: String]) async throws -> T {
        var comps = URLComponents(url: baseURL.appendingPathComponent(path), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { comps.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = comps.url else { throw URLError(.badURL) }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        if http.statusCode >= 200 && http.statusCode < 300 {
            return try decoder.decode(T.self, from: data)
        } else {
            if let apiErr = try? decoder.decode(APIErrorResponse.self, from: data) {
                throw apiErr
            }
            throw URLError(.badServerResponse)
        }
    }
}

// MARK: - Utils

private func formatCents(_ cents: Int64) -> String {
    let sign = cents < 0 ? "-" : ""
    let absCents = Int64(abs(Int(cents)))
    let dollars = absCents / 100
    let remainder = absCents % 100
    return "\(sign)$\(dollars).\(String(format: "%02d", remainder))"
}

private struct IdentifiableString: Identifiable { let id = UUID(); let value: String }
private extension Optional where Wrapped == String {
    var asIdentifiable: IdentifiableString? {
        switch self {
        case .none: return nil
        case .some(let s): return IdentifiableString(value: s)
        }
    }
}