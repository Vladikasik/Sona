//
//  ContentView.swift
//  Sona
//
//  Created by Vladislav Ainshtein on 21.10.25.
//

import SwiftUI
import Foundation
import UIKit

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
    @AppStorage("parentEmail") private var parentEmail: String = ""
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
                    ParentMainView(name: parentName, parentId: parentServerId, onSignOut: signOut, onError: { alertMessage = $0 })
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
                    ParentOnboardingView(onComplete: { name, email, gridUserId in
                        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
                        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
                        guard !trimmedName.isEmpty, !trimmedEmail.isEmpty else { return }
                        parentName = trimmedName
                        parentEmail = trimmedEmail
                        parentServerId = 0
                        parentUID = SessionManager.shared.uid ?? ""
                        storedRole = UserRole.parent.rawValue
                    })
                case .childOnboarding:
                    ChildOnboardingView(onComplete: { name, parent in
                        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
                        let trimmedParent = parent.trimmingCharacters(in: .whitespacesAndNewlines)
                        Task {
                            do {
                                guard let email = SessionManager.shared.email, !email.isEmpty else {
                                    alertMessage = "Missing email in session"
                                    return
                                }
                                let parentIdParam: String? = trimmedParent.isEmpty ? nil : trimmedParent
                                if let kid = try await TrackingAPI.getOrCreateChild(email: email, name: trimmedName, parentId: parentIdParam) {
                                    BackendDataStore.shared.mergeKid(kid)
                                    BackendDataStore.shared.updateChild(kid)
                                    childName = trimmedName
                                    if let parentIdParam { childParentId = parentIdParam }
                                    storedRole = UserRole.child.rawValue
                                } else {
                                    alertMessage = parentIdParam == nil
                                        ? "Child not found. Enter Parent ID to create a new profile."
                                        : "Failed to create child"
                                }
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
        .alert(item: Binding<IdentifiableString?>(
            get: { alertMessage.map { IdentifiableString(value: $0) } },
            set: { newValue in alertMessage = newValue?.value }
        )) { msg in
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
        SessionManager.shared.clear()
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
    @State private var email: String = ""
    @State private var otpCode: String = ""
    @State private var isSubmitting: Bool = false
    @State private var step: Step = .enterDetails
    private enum OTPMode { case account, auth }
    @State private var otpMode: OTPMode = .account
    @State private var localAlert: String?
    let onComplete: (String, String, String?) -> Void

    private enum Step {
        case enterDetails
        case enterOTP
    }

    var body: some View {
        Form {
            if step == .enterDetails {
                Section(header: Text("Parent")) {
                    TextField("Your name", text: $name)
                        .textInputAutocapitalization(.words)
                    TextField("Email", text: $email)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled(true)
                }
                Section {
                    Button(action: initiateRegistration) {
                        if isSubmitting { ProgressView() } else { Text("Continue") }
                    }
                    .disabled(!isDetailsValid || isSubmitting)
                }
            } else {
                Section(header: Text("Verify Email")) {
                    Text("Enter the 6-digit code sent to \(email)")
                        .font(.footnote)
                        .foregroundColor(.secondary)
                    TextField("OTP Code", text: $otpCode)
                        .keyboardType(.numberPad)
                        .autocorrectionDisabled(true)
                }
                Section {
                    Button(action: submitOTP) {
                        if isSubmitting { ProgressView() } else { Text("Verify & Continue") }
                    }
                    .disabled(!isOTPValid || isSubmitting)
                }
            }
        }
        .alert(item: Binding<IdentifiableString?>(
            get: { localAlert.map { IdentifiableString(value: $0) } },
            set: { newValue in localAlert = newValue?.value }
        )) { msg in
            Alert(title: Text("Error"), message: Text(msg.value), dismissButton: .default(Text("OK")))
        }
    }

    private var isDetailsValid: Bool {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        return !trimmedName.isEmpty && trimmedEmail.contains("@") && trimmedEmail.contains(".")
    }

    private var isOTPValid: Bool {
        let trimmed = otpCode.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.count == 6 && trimmed.allSatisfy { $0.isNumber }
    }

    private func initiateRegistration() {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !isSubmitting else { return }
        isSubmitting = true
        Task {
            do {
                // First: register/fetch parent in tracking backend
                do {
                    let parent = try await TrackingAPI.getOrCreateParent(email: trimmedEmail, name: trimmedName)
                    BackendDataStore.shared.updateParent(parent)
                    _ = try? SessionManager.shared.setNameEmail(name: parent.name, email: parent.email)
                    _ = try? SessionManager.shared.setUID(parent.id)
                } catch {
                    localAlert = error.localizedDescription
                }
                try HPKEManager.shared.ensureKeysPresent()
                do {
                    try await GridAPI.createAccount(email: trimmedEmail, memo: "Parent: \(trimmedName)")
                    otpMode = .account
                    step = .enterOTP
                } catch GridAPI.GridAPIError.accountExists {
                    try await GridAPI.authInitiate(email: trimmedEmail)
                    otpMode = .auth
                    step = .enterOTP
                } catch {
                    throw error
                }
            } catch {
                localAlert = error.localizedDescription
            }
            isSubmitting = false
        }
    }

    private func submitOTP() {
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedOTP = otpCode.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !isSubmitting else { return }
        isSubmitting = true
        Task {
            do {
                try HPKEManager.shared.ensureKeysPresent()
                if otpMode == .account {
                    let result = try await GridAPI.verifyAccount(email: trimmedEmail, otpCode: trimmedOTP)
                    _ = try? SessionManager.shared.setAddress(result.data.address)
                    _ = try? SessionManager.shared.setGridUserId(result.data.gridUserId)
                    try? SessionManager.shared.setGridRawResponse(result.raw)
                    SessionManager.shared.extractAndStorePrivyAccessToken(from: result.raw)
                    if let enc = result.firstEncryptedKey() {
                        if let key = try? HPKEManager.shared.decryptAuthorizationKey(encapsulatedKeyB64: enc.encapsulatedKey, ciphertextB64: enc.ciphertext), let raw32 = SessionManager.extractRawP256PrivateKey(fromHPKEPlaintext: key) {
                            try? SessionManager.shared.setDecryptedAuthorizationKey(raw32)
                            print("[OTP][Parent] saved raw P-256 key: \(raw32.count) bytes")
                        } else { print("[OTP][Parent] failed to extract raw P-256 key") }
                    }
                } else {
                    let result = try await GridAPI.authVerify(email: trimmedEmail, otpCode: trimmedOTP)
                    _ = try? SessionManager.shared.setAddress(result.data.address)
                    _ = try? SessionManager.shared.setGridUserId(result.data.gridUserId)
                    try? SessionManager.shared.setGridRawResponse(result.raw)
                    SessionManager.shared.extractAndStorePrivyAccessToken(from: result.raw)
                    if let enc = result.firstEncryptedKey() {
                        if let key = try? HPKEManager.shared.decryptAuthorizationKey(encapsulatedKeyB64: enc.encapsulatedKey, ciphertextB64: enc.ciphertext), let raw32 = SessionManager.extractRawP256PrivateKey(fromHPKEPlaintext: key) {
                            try? SessionManager.shared.setDecryptedAuthorizationKey(raw32)
                            print("[OTP][Parent][Auth] saved raw P-256 key: \(raw32.count) bytes")
                        } else { print("[OTP][Parent][Auth] failed to extract raw P-256 key") }
                    }
                }
                _ = try? SessionManager.shared.setNameEmail(name: name.trimmingCharacters(in: .whitespacesAndNewlines), email: trimmedEmail)
                // After verification, optionally refresh parent record silently (non-blocking)
                Task.detached {
                    if let p = try? await TrackingAPI.getOrCreateParent(email: trimmedEmail, name: name.trimmingCharacters(in: .whitespacesAndNewlines)) {
                        BackendDataStore.shared.updateParent(p)
                        _ = try? SessionManager.shared.setUID(p.id)
                    }
                }
                onComplete(name.trimmingCharacters(in: .whitespacesAndNewlines), trimmedEmail, SessionManager.shared.gridUserIdValue)
            } catch {
                localAlert = error.localizedDescription
            }
            isSubmitting = false
        }
    }
}

private struct ChildOnboardingView: View {
    @State private var name: String = ""
    @State private var email: String = ""
    @State private var parentId: String = ""
    @State private var otpCode: String = ""
    @State private var isSubmitting: Bool = false
    @State private var step: Step = .enterDetails
    private enum OTPMode { case account, auth }
    @State private var otpMode: OTPMode = .account
    @State private var localAlert: String?
    let onComplete: (String, String) -> Void

    private enum Step { case enterDetails, enterOTP }

    var body: some View {
        Form {
            if step == .enterDetails {
                Section(header: Text("Child")) {
                    TextField("Your name", text: $name)
                        .textInputAutocapitalization(.words)
                    TextField("Email", text: $email)
                        .keyboardType(.emailAddress)
                        .textInputAutocapitalization(.never)
                        .autocorrectionDisabled(true)
                    TextField("Parent ID (optional, 6-char code)", text: $parentId)
                        .textInputAutocapitalization(.characters)
                        .autocorrectionDisabled(true)
                }
                Section {
                    Button(action: initiateRegistration) {
                        if isSubmitting { ProgressView() } else { Text("Continue") }
                    }
                    .disabled(!isDetailsValid || isSubmitting)
                }
            } else {
                Section(header: Text("Verify Email")) {
                    Text("Enter the 6-digit code sent to \(email)")
                        .font(.footnote)
                        .foregroundColor(.secondary)
                    TextField("OTP Code", text: $otpCode)
                        .keyboardType(.numberPad)
                        .autocorrectionDisabled(true)
                }
                Section {
                    Button(action: submitOTP) {
                        if isSubmitting { ProgressView() } else { Text("Verify & Continue") }
                    }
                    .disabled(!isOTPValid || isSubmitting)
                }
            }
        }
        .alert(item: Binding<IdentifiableString?>(
            get: { localAlert.map { IdentifiableString(value: $0) } },
            set: { newValue in localAlert = newValue?.value }
        )) { msg in
            Alert(title: Text("Error"), message: Text(msg.value), dismissButton: .default(Text("OK")))
        }
    }

    private var isDetailsValid: Bool {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        let parentInput = parentId.trimmingCharacters(in: .whitespacesAndNewlines).uppercased()
        let isParentEmpty = parentInput.isEmpty
        let isValidCode = parentInput.count == 6 && parentInput.allSatisfy { $0.isLetter || $0.isNumber }
        return !trimmedName.isEmpty && trimmedEmail.contains("@") && trimmedEmail.contains(".") && (isParentEmpty || isValidCode)
    }

    private var isOTPValid: Bool {
        let trimmed = otpCode.trimmingCharacters(in: .whitespacesAndNewlines)
        return trimmed.count == 6 && trimmed.allSatisfy { $0.isNumber }
    }

    private func initiateRegistration() {
        let trimmedName = name.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        let parentInput = parentId.trimmingCharacters(in: .whitespacesAndNewlines).uppercased()
        guard !isSubmitting else { return }
        isSubmitting = true
        Task {
            do {
                // First: register/fetch child in tracking backend
                do {
                    if let kid = try await TrackingAPI.getOrCreateChild(email: trimmedEmail, name: trimmedName, parentId: parentInput.isEmpty ? nil : parentInput) {
                        BackendDataStore.shared.mergeKid(kid)
                        BackendDataStore.shared.updateChild(kid)
                        _ = try? SessionManager.shared.setUID(kid.id)
                    }
                } catch {
                    localAlert = error.localizedDescription
                }
                try HPKEManager.shared.ensureKeysPresent()
                do {
                    let memoParent = parentInput.isEmpty ? nil : "parent=\(parentInput)"
                    let memo = memoParent != nil ? "Kid: \(trimmedName) \(memoParent!)" : "Kid: \(trimmedName)"
                    try await GridAPI.createAccount(email: trimmedEmail, memo: memo)
                    otpMode = .account
                    step = .enterOTP
                } catch GridAPI.GridAPIError.accountExists {
                    try await GridAPI.authInitiate(email: trimmedEmail)
                    otpMode = .auth
                    step = .enterOTP
                } catch {
                    throw error
                }
            } catch {
                localAlert = error.localizedDescription
            }
            isSubmitting = false
        }
    }

    private func submitOTP() {
        let trimmedEmail = email.trimmingCharacters(in: .whitespacesAndNewlines)
        let trimmedOTP = otpCode.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !isSubmitting else { return }
        isSubmitting = true
        Task {
            do {
                try HPKEManager.shared.ensureKeysPresent()
                if otpMode == .account {
                    let result = try await GridAPI.verifyAccount(email: trimmedEmail, otpCode: trimmedOTP)
                    _ = try? SessionManager.shared.setAddress(result.data.address)
                    _ = try? SessionManager.shared.setGridUserId(result.data.gridUserId)
                    try? SessionManager.shared.setGridRawResponse(result.raw)
                    SessionManager.shared.extractAndStorePrivyAccessToken(from: result.raw)
                    if let enc = result.firstEncryptedKey() {
                        if let key = try? HPKEManager.shared.decryptAuthorizationKey(encapsulatedKeyB64: enc.encapsulatedKey, ciphertextB64: enc.ciphertext), let raw32 = SessionManager.extractRawP256PrivateKey(fromHPKEPlaintext: key) {
                            try? SessionManager.shared.setDecryptedAuthorizationKey(raw32)
                            print("[OTP][Child] saved raw P-256 key: \(raw32.count) bytes")
                        } else { print("[OTP][Child] failed to extract raw P-256 key") }
                    }
                } else {
                    let result = try await GridAPI.authVerify(email: trimmedEmail, otpCode: trimmedOTP)
                    _ = try? SessionManager.shared.setAddress(result.data.address)
                    _ = try? SessionManager.shared.setGridUserId(result.data.gridUserId)
                    try? SessionManager.shared.setGridRawResponse(result.raw)
                    SessionManager.shared.extractAndStorePrivyAccessToken(from: result.raw)
                    if let enc = result.firstEncryptedKey() {
                        if let key = try? HPKEManager.shared.decryptAuthorizationKey(encapsulatedKeyB64: enc.encapsulatedKey, ciphertextB64: enc.ciphertext), let raw32 = SessionManager.extractRawP256PrivateKey(fromHPKEPlaintext: key) {
                            try? SessionManager.shared.setDecryptedAuthorizationKey(raw32)
                            print("[OTP][Child][Auth] saved raw P-256 key: \(raw32.count) bytes")
                        } else { print("[OTP][Child][Auth] failed to extract raw P-256 key") }
                    }
                }
                _ = try? SessionManager.shared.setNameEmail(name: name.trimmingCharacters(in: .whitespacesAndNewlines), email: trimmedEmail)
                // Also register or fetch child in tracking backend
                do {
                    if let kid = try await TrackingAPI.getOrCreateChild(email: trimmedEmail, name: name.trimmingCharacters(in: .whitespacesAndNewlines), parentId: nil) {
                        BackendDataStore.shared.mergeKid(kid)
                        BackendDataStore.shared.updateChild(kid)
                        _ = try? SessionManager.shared.setUID(kid.id)
                    }
                } catch {
                    // continue even if tracking backend fails
                }
                onComplete(name.trimmingCharacters(in: .whitespacesAndNewlines), parentId.trimmingCharacters(in: .whitespacesAndNewlines))
            } catch {
                localAlert = error.localizedDescription
            }
            isSubmitting = false
        }
    }
}

private struct ParentMainView: View {
    let name: String
    let parentId: Int
    let onSignOut: () -> Void
    let onError: (String) -> Void

    @State private var balanceText: String = "0.0000 SOL"
    @State private var kids: [BackendKid] = []
    @State private var parentDisplayName: String = ""
    @State private var parentWallet: Double = 0
    @State private var showingSettings: Bool = false
    @State private var isLoadingBalance: Bool = false
    @State private var selectedChildEmail: String?
    @ObservedObject private var session = SessionManager.shared

    var body: some View {
        NavigationStack {
            List {
                Section(header: Text("Overview")) {
                    HStack {
                        Text("Name")
                        Spacer()
                        Text(parentDisplayName.isEmpty ? name : parentDisplayName).foregroundColor(.secondary)
                    }
                    HStack {
                        Text("Balance")
                        Spacer()
                        if isLoadingBalance { ProgressView() } else { Text(balanceText).font(.headline) }
                    }
                }
                if !kids.isEmpty {
                    Section(header: Text("Kids")) {
                        ForEach(kids) { kid in
                            Button(action: {
                                selectedChildEmail = kid.email
                            }) {
                                HStack {
                                    Text(kid.name ?? kid.email)
                                        .foregroundColor(.primary)
                                    Spacer()
                                    Text(formatSOL(kid.wallet))
                                        .font(.system(.body, design: .monospaced))
                                        .foregroundColor(.secondary)
                                }
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
            .toolbar {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(action: { showingSettings = true }) { Image(systemName: "gearshape") }
                }
            }
            .sheet(isPresented: $showingSettings) {
                SettingsView(onDismiss: { showingSettings = false })
            }
            .sheet(item: Binding<IdentifiableString?>(
                get: { selectedChildEmail.map { IdentifiableString(value: $0) } },
                set: { newValue in selectedChildEmail = newValue?.value }
            )) { emailWrapper in
                ChildDetailView(childEmail: emailWrapper.value, onDismiss: {
                    selectedChildEmail = nil
                })
            }
            .refreshable {
                await refreshParent()
                await fetchBalance()
            }
            .task {
                loadFromCache()
                await fetchBalance()
                await refreshParent()
            }
        }
    }

    private func fetchBalance() async {
        guard let address = session.address else { balanceText = "0.0000 SOL"; return }
        isLoadingBalance = true
        defer { isLoadingBalance = false }
        do {
            if let display = try await GridAPI.fetchPrimaryBalanceDisplay(address: address) {
                balanceText = "\(String(format: "%.4f", display.amount)) \(display.symbol)"
            } else {
                balanceText = "0.0000 SOL"
            }
        } catch {
            balanceText = "0.0000 SOL"
        }
    }

    private func loadFromCache() {
        if let cache = BackendDataStore.shared.load(), let p = cache.parent {
            parentDisplayName = p.name
            parentWallet = p.wallet
            kids = p.kids
        }
    }

    private func refreshParent() async {
        guard let email = SessionManager.shared.email else { return }
        do {
            let parent = try await TrackingAPI.getOrCreateParent(email: email, name: nil)
            BackendDataStore.shared.updateParent(parent)
            _ = try? SessionManager.shared.setUID(parent.id)
            await MainActor.run {
                parentDisplayName = parent.name
                parentWallet = parent.wallet
                kids = parent.kids
            }
        } catch {
            // ignore silently for now; UI shows last cached
        }
    }
}

private struct ChildMainView: View {
    @AppStorage("childName") private var childName: String = ""
    @AppStorage("childParentId") private var parentId: String = ""
    @AppStorage("kidId") private var kidId: Int = 0
    @ObservedObject private var session = SessionManager.shared
    let onSignOut: () -> Void
    let onError: (String) -> Void

    @State private var childBalanceCents: Int64 = 0

    @State private var showingSettings: Bool = false

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
            Section(header: Text("Address")) {
                Text(session.address ?? "—").font(.system(.body, design: .monospaced)).foregroundColor(.secondary)
            }
            Section {
                Button("Sign out", role: .destructive, action: onSignOut)
                    .frame(maxWidth: .infinity)
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Child")
        .toolbar {
            ToolbarItem(placement: .topBarTrailing) {
                Button(action: { showingSettings = true }) { Image(systemName: "gearshape") }
            }
        }
        .sheet(isPresented: $showingSettings) {
            ChildSettingsView(onDismiss: { showingSettings = false })
        }
        .refreshable {
            await refreshChild()
        }
        .task {
            await refreshChild()
        }
    }

    private func refreshChild() async {
        guard let email = session.email, !email.isEmpty else { return }
        do {
            if let kid = try await TrackingAPI.getOrCreateChild(email: email, name: nil, parentId: nil) {
                BackendDataStore.shared.mergeKid(kid)
                BackendDataStore.shared.updateChild(kid)
                await MainActor.run {
                    // Update any derived UI state if needed in future
                }
            }
        } catch {
            // Silent fail: keep last cached values
        }
    }
}

#Preview {
    ContentView()
}

// MARK: - API Layer

enum APIService {
    static var baseURLString: String = "https://api.aynshteyn.dev"
    static var baseURL: URL { URL(string: baseURLString)! }
    static var enableLogging: Bool = true
    private static let trackingHost = "api.aynshteyn.dev"
    private static let trackingAuthHeader = "Bearer SonaBetaTestAPi"

    struct APIErrorResponse: Decodable, Error { let error: String }
    struct ParentBalanceResponse: Decodable { let parentBalance: Int64 }
    struct SendKidMoneyResponse: Decodable { let parentBalance: Int64; let kidBalance: Int64 }

    struct Parent: Decodable { let id: Int64; let uid: String; let name: String; let balance: Int64
        private enum CodingKeys: String, CodingKey { case id = "ID", uid = "UID", name = "Name", balance = "Balance" }
    }
    struct Kid: Decodable, Identifiable { let id: Int64; let name: String; var balance: Int64
        private enum CodingKeys: String, CodingKey { case id = "ID", name = "Name", balance = "Balance" }
    }
    struct KidsListResponse: Decodable { var kids: [Kid] }

    static func kidsList(parentId: Int64) async throws -> KidsListResponse {
        try await request(path: "/list_kids", method: "GET", query: ["parent_id": String(parentId)])
    }

    static func parentTopUp(parentId: Int64, amount: Int64) async throws -> ParentBalanceResponse {
        try await request(path: "/parent_topup", method: "POST", query: ["parent_id": String(parentId), "amount": String(amount)])
    }

    static func sendKidMoney(parentId: Int64, kidId: Int64, amount: Int64) async throws -> SendKidMoneyResponse {
        try await request(path: "/send_kid_money", method: "POST", query: ["parent_id": String(parentId), "kid_id": String(kidId), "amount": String(amount)])
    }

    static func getChild(name: String, parentId: Int64?) async throws -> Kid? {
        var q: [String: String] = ["name": name]
        if let parentId { q["parent_id"] = String(parentId) }
        return try await requestAllowingNull(path: "/get_child", method: "GET", query: q)
    }

    private static func request<T: Decodable>(path: String, method: String, query: [String: String]) async throws -> T {
        let (data, http) = try await perform(method: method, path: path, query: query)
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        if http.statusCode >= 200 && http.statusCode < 300 {
            return try decoder.decode(T.self, from: data)
        } else {
            if let apiErr = try? decoder.decode(APIErrorResponse.self, from: data) { throw apiErr }
            throw URLError(.badServerResponse)
        }
    }

    // Centralized request executor and logger
    private static func perform(method: String, path: String, query: [String: String] = [:], headers: [String: String] = [:]) async throws -> (Data, HTTPURLResponse) {
        let pathComponent = path.hasPrefix("/") ? String(path.dropFirst()) : path
        var comps = URLComponents(url: baseURL.appendingPathComponent(pathComponent), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { comps.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = comps.url else { throw URLError(.badURL) }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        var combinedHeaders = headers
        if url.host == trackingHost && combinedHeaders["Authorization"] == nil {
            combinedHeaders["Authorization"] = trackingAuthHeader
        }
        for (k, v) in combinedHeaders { req.setValue(v, forHTTPHeaderField: k) }
        if enableLogging {
            print("➡️ \(method) \(url.absoluteString)")
            if !combinedHeaders.isEmpty {
                var redacted = combinedHeaders
                if redacted["Authorization"] != nil { redacted["Authorization"] = "Bearer <redacted>" }
                print("➡️ Headers: \(redacted)")
            }
        }
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        if enableLogging {
            print("⬅️ [\(http.statusCode)] \(url.absoluteString)")
            if let all = http.allHeaderFields as? [String: Any] { print("⬅️ Headers: \(all)") }
            let body = String(data: data, encoding: .utf8) ?? "<non-utf8>"
            print("⬅️ Body: \(body)")
        }
        return (data, http)
    }

    private static func perform<B: Encodable>(method: String, path: String, query: [String: String] = [:], headers: [String: String] = [:], jsonBody: B) async throws -> (Data, HTTPURLResponse) {
        let pathComponent = path.hasPrefix("/") ? String(path.dropFirst()) : path
        var comps = URLComponents(url: baseURL.appendingPathComponent(pathComponent), resolvingAgainstBaseURL: false)!
        if !query.isEmpty { comps.queryItems = query.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = comps.url else { throw URLError(.badURL) }
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        var combined = headers
        if combined["Content-Type"] == nil { combined["Content-Type"] = "application/json" }
        if url.host == trackingHost && combined["Authorization"] == nil {
            combined["Authorization"] = trackingAuthHeader
        }
        for (k, v) in combined { req.setValue(v, forHTTPHeaderField: k) }
        req.httpBody = try JSONEncoder().encode(jsonBody)
        if enableLogging {
            print("➡️ \(method) \(url.absoluteString)")
            var redacted = combined
            if redacted["Authorization"] != nil { redacted["Authorization"] = "Bearer <redacted>" }
            print("➡️ Headers: \(redacted)")
            let bodyOut = String(data: req.httpBody ?? Data(), encoding: .utf8) ?? "<non-utf8>"
            print("➡️ Body: \(bodyOut)")
        }
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        if enableLogging {
            print("⬅️ [\(http.statusCode)] \(url.absoluteString)")
            if let all = http.allHeaderFields as? [String: Any] { print("⬅️ Headers: \(all)") }
            let body = String(data: data, encoding: .utf8) ?? "<non-utf8>"
            print("⬅️ Body: \(body)")
        }
        return (data, http)
    }

    private static func requestAllowingNull<T: Decodable>(path: String, method: String, query: [String: String]) async throws -> T? {
        let (data, http) = try await perform(method: method, path: path, query: query)
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        if http.statusCode >= 200 && http.statusCode < 300 {
            if data.count == 0 { return nil }
            if String(data: data, encoding: .utf8)?.trimmingCharacters(in: .whitespacesAndNewlines) == "null" { return nil }
            return try decoder.decode(T.self, from: data)
        } else {
            if let apiErr = try? decoder.decode(APIErrorResponse.self, from: data) { throw apiErr }
            throw URLError(.badServerResponse)
        }
    }

    private static func requestStatusCode(path: String, method: String, query: [String: String]) async throws -> Int {
        let (_, http) = try await perform(method: method, path: path, query: query)
        return http.statusCode
    }

    private static func requestJSONBody<T: Decodable, B: Encodable>(path: String, method: String, body: B) async throws -> T {
        let (data, http) = try await perform(method: method, path: path, query: [:], headers: ["Content-Type": "application/json"], jsonBody: body)
        let decoder = JSONDecoder()
        decoder.keyDecodingStrategy = .convertFromSnakeCase
        if http.statusCode >= 200 && http.statusCode < 300 {
            return try decoder.decode(T.self, from: data)
        } else {
            if let apiErr = try? decoder.decode(APIErrorResponse.self, from: data) { throw apiErr }
            throw URLError(.badServerResponse)
        }
    }

    static func performAbsolute<B: Encodable>(method: String, url: URL, headers: [String: String] = [:], jsonBody: B) async throws -> (Data, HTTPURLResponse) {
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        var combinedHeaders = headers
        if combinedHeaders["Content-Type"] == nil { combinedHeaders["Content-Type"] = "application/json" }
        if url.host == trackingHost && combinedHeaders["Authorization"] == nil {
            combinedHeaders["Authorization"] = trackingAuthHeader
        }
        for (k, v) in combinedHeaders { req.setValue(v, forHTTPHeaderField: k) }
        req.httpBody = try JSONEncoder().encode(jsonBody)
        if enableLogging {
            print("➡️ \(method) \(url.absoluteString)")
            var redacted = combinedHeaders
            if redacted["Authorization"] != nil { redacted["Authorization"] = "Bearer <redacted>" }
            print("➡️ Headers: \(redacted)")
            let bodyOut = String(data: req.httpBody ?? Data(), encoding: .utf8) ?? "<non-utf8>"
            print("➡️ Body: \(bodyOut)")
        }
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        if enableLogging {
            print("⬅️ [\(http.statusCode)] \(url.absoluteString)")
            if let all = http.allHeaderFields as? [String: Any] { print("⬅️ Headers: \(all)") }
            let body = String(data: data, encoding: .utf8) ?? "<non-utf8>"
            print("⬅️ Body: \(body)")
        }
        return (data, http)
    }

    static func performAbsolute(method: String, url: URL, headers: [String: String] = [:]) async throws -> (Data, HTTPURLResponse) {
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Accept")
        var combinedHeaders = headers
        if url.host == trackingHost && combinedHeaders["Authorization"] == nil {
            combinedHeaders["Authorization"] = trackingAuthHeader
        }
        for (k, v) in combinedHeaders { req.setValue(v, forHTTPHeaderField: k) }
        if enableLogging {
            print("➡️ \(method) \(url.absoluteString)")
            if !combinedHeaders.isEmpty {
                var redacted = combinedHeaders
                if redacted["Authorization"] != nil { redacted["Authorization"] = "Bearer <redacted>" }
                print("➡️ Headers: \(redacted)")
            }
        }
        let (data, resp) = try await URLSession.shared.data(for: req)
        guard let http = resp as? HTTPURLResponse else { throw URLError(.badServerResponse) }
        if enableLogging {
            print("⬅️ [\(http.statusCode)] \(url.absoluteString)")
            if let all = http.allHeaderFields as? [String: Any] { print("⬅️ Headers: \(all)") }
            let body = String(data: data, encoding: .utf8) ?? "<non-utf8>"
            print("⬅️ Body: \(body)")
        }
        return (data, http)
    }
}

// MARK: - Utils

private func formatSOL(_ sol: Double) -> String {
    String(format: "%.4f SOL", sol)
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