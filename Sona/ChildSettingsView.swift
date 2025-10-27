import SwiftUI
import UIKit

struct ChildSettingsView: View {
    @ObservedObject private var session = SessionManager.shared
    @AppStorage("childName") private var storedChildName: String = ""
    @AppStorage("childParentId") private var storedParentId: String = ""
    let onDismiss: () -> Void

    @State private var editName: String = ""
    @State private var editParentId: String = ""
    @State private var isSaving: Bool = false
    @State private var alertMsg: String?

    var body: some View {
        NavigationStack {
            List {
                Section(header: Text("Session")) {
                    copyRow(title: "UID", value: session.uid ?? "—")
                    HStack {
                        Text("Name")
                        Spacer()
                        TextField("—", text: $editName)
                            .multilineTextAlignment(.trailing)
                            .textInputAutocapitalization(.words)
                    }
                    copyRow(title: "Email", value: session.email ?? "—")
                    copyRow(title: "Address", value: session.address ?? "—")
                    HStack {
                        Text("Parent ID")
                        Spacer()
                        TextField("—", text: $editParentId)
                            .textInputAutocapitalization(.characters)
                            .multilineTextAlignment(.trailing)
                    }
                }
                Section {
                    Button {
                        Task { await saveEdits() }
                    } label: {
                        if isSaving { ProgressView() } else { Text("Update") }
                    }
                    .disabled(isSaving || (session.email ?? "").isEmpty)
                }
                Section(header: Text("Security")) {
                    Button(role: .destructive) {
                        session.clear()
                    } label: {
                        Text("Clear Session")
                    }
                }
            }
            .listStyle(.insetGrouped)
            .navigationTitle("Settings")
            .toolbar {
                ToolbarItem(placement: .topBarLeading) { Button("Close", action: onDismiss) }
            }
            .refreshable {
                _ = try? session.reload()
                session.ensureAddressFromStoredGrid()
                let keyLen = session.getDecryptedAuthorizationKey()?.count ?? 0
                print("[Settings][Child] decrypted key length: \(keyLen)")
                loadFromCache()
            }
            .onAppear {
                session.ensureAddressFromStoredGrid()
                let keyLen = session.getDecryptedAuthorizationKey()?.count ?? 0
                print("[Settings][Child][Appear] decrypted key length: \(keyLen)")
                loadFromCache()
            }
            .alert("Error", isPresented: Binding<Bool>(
                get: { alertMsg != nil },
                set: { newValue in if !newValue { alertMsg = nil } }
            ), actions: {
                Button("OK", role: .cancel) { alertMsg = nil }
            }, message: {
                Text(alertMsg ?? "")
            })
        }
    }

    private func copyRow(title: String, value: String) -> some View {
        HStack(alignment: .top, spacing: 8) {
            Text(title)
            Spacer()
            Text(value)
                .font(.system(.body, design: .monospaced))
                .foregroundColor(.secondary)
                .textSelection(.enabled)
                .lineLimit(3)
            Button(action: { UIPasteboard.general.string = value }) {
                Image(systemName: "doc.on.doc")
            }
            .buttonStyle(.plain)
            .disabled(value == "—")
            .accessibilityLabel("Copy \(title)")
        }
    }

    private func loadFromCache() {
        editName = session.name ?? storedChildName
        editParentId = storedParentId
        if let cache = BackendDataStore.shared.load(), let c = cache.child {
            if editName.isEmpty, let nm = c.name { editName = nm }
            if let pid = c.parentId, !pid.isEmpty { editParentId = pid }
        }
    }

    private func saveEdits() async {
        guard let email = session.email, !email.isEmpty else { return }
        isSaving = true
        defer { isSaving = false }
        let nameOut = editName.trimmingCharacters(in: .whitespacesAndNewlines)
        let parentOut = editParentId.trimmingCharacters(in: .whitespacesAndNewlines).uppercased()
        let parentParam: String? = parentOut.isEmpty ? nil : parentOut
        do {
            let updated = try await TrackingAPI.updateChild(
                email: email,
                name: nameOut.isEmpty ? nil : nameOut,
                parentId: parentParam,
                wallet: (session.address?.isEmpty == false ? session.address : nil)
            )
            BackendDataStore.shared.updateChild(updated)
            storedChildName = updated.name ?? storedChildName
            if let parentParam { storedParentId = parentParam }
            if let pid = updated.parentId, !pid.isEmpty { storedParentId = pid }
            _ = try? SessionManager.shared.setNameEmail(name: updated.name ?? nameOut, email: email)
        } catch {
            alertMsg = error.localizedDescription
        }
    }
}


