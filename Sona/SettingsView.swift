//
//  SettingsView.swift
//  Sona
//
//  Created by Assistant on 23.10.25.
//

import SwiftUI
import UIKit

struct SettingsView: View {
    @ObservedObject private var session = SessionManager.shared
    let onDismiss: () -> Void
    @State private var editName: String = ""
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
                print("[Settings][Parent] decrypted key length: \(keyLen)")
                loadFromCache()
            }
            .onAppear {
                session.ensureAddressFromStoredGrid()
                let keyLen = session.getDecryptedAuthorizationKey()?.count ?? 0
                print("[Settings][Parent][Appear] decrypted key length: \(keyLen)")
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
        editName = session.name ?? ""
        if let cache = BackendDataStore.shared.load(), let p = cache.parent {
            if editName.isEmpty { editName = p.name }
        }
    }

    private func saveEdits() async {
        guard let email = session.email, !email.isEmpty else { return }
        isSaving = true
        defer { isSaving = false }
        let nameOut = editName.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            let addressOut = session.address?.trimmingCharacters(in: .whitespacesAndNewlines)
            let updated = try await TrackingAPI.updateParent(email: email, name: nameOut.isEmpty ? nil : nameOut, wallet: (addressOut?.isEmpty == false ? addressOut : nil))
            BackendDataStore.shared.updateParent(updated)
            _ = try? SessionManager.shared.setNameEmail(name: updated.name, email: updated.email)
        } catch {
            alertMsg = error.localizedDescription
        }
    }
}


