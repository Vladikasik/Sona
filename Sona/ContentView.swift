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
    @AppStorage("childName") private var childName: String = ""
    @AppStorage("childParentId") private var childParentId: String = ""

    @State private var flow: FlowState = .selection

    var body: some View {
        Group {
            if let roleString = storedRole, let role = UserRole(rawValue: roleString) {
                switch role {
                case .parent:
                    ParentMainView(name: parentName, id: parentId, onSignOut: signOut)
                case .child:
                    ChildMainView(onSignOut: signOut)
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
                        let generatedId = Self.generateParentId()
                        parentName = name
                        parentId = generatedId
                        storedRole = UserRole.parent.rawValue
                    })
                case .childOnboarding:
                    ChildOnboardingView(onComplete: { name, parent in
                        childName = name
                        childParentId = parent
                        storedRole = UserRole.child.rawValue
                    })
                }
            }
        }
        .onAppear {
            if storedRole == nil {
                flow = .selection
            }
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
        childName = ""
        childParentId = ""
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
    let id: String
    let onSignOut: () -> Void

    private var balanceText: String {
        let value = Self.makeDeterministicAmount(from: id)
        return String(format: "$%.2f", value)
    }

    private var kids: [String] {
        Self.makeDeterministicKids(from: id)
    }

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
                    Text(id).font(.system(.body, design: .monospaced)).foregroundColor(.secondary)
                }
            }
            Section(header: Text("Balance")) {
                Text(balanceText).font(.title2)
            }
            Section(header: Text("Kids")) {
                if kids.isEmpty {
                    Text("No kids yet")
                } else {
                    ForEach(kids, id: \.self) { kid in
                        Text(kid)
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
    }

    private static func makeDeterministicAmount(from seed: String) -> Double {
        var accumulator: UInt64 = 0
        for u in seed.utf8 { accumulator = accumulator &* 1099511628211 &+ UInt64(u) }
        let base = Double(accumulator % 90_000) / 100.0
        return base + 10.0
    }

    private static func makeDeterministicKids(from seed: String) -> [String] {
        let pool = ["Alex", "Sam", "Taylor", "Jordan", "Casey", "Riley", "Morgan", "Quinn", "Avery", "Jamie"]
        var result: [String] = []
        var acc: UInt64 = 1469598103934665603
        for u in seed.utf8 { acc = acc &* 1099511628211 &+ UInt64(u) }
        let count = Int((acc % 3) + 1)
        var used = Set<Int>()
        var temp = acc
        while result.count < count {
            temp = temp &* 2862933555777941757 &+ 3037000493
            let idx = Int(temp % UInt64(pool.count))
            if !used.contains(idx) {
                used.insert(idx)
                result.append(pool[idx])
            }
        }
        return result
    }
}

private struct ChildMainView: View {
    @AppStorage("childName") private var childName: String = ""
    @AppStorage("childParentId") private var parentId: String = ""
    let onSignOut: () -> Void

    private var balanceText: String {
        let value = Self.makeDeterministicAmount(from: childName)
        return String(format: "$%.2f", value)
    }

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
                Text(balanceText).font(.title2)
            }
            Section {
                Button("Sign out", role: .destructive, action: onSignOut)
                    .frame(maxWidth: .infinity)
            }
        }
        .listStyle(.insetGrouped)
        .navigationTitle("Child")
    }

    private static func makeDeterministicAmount(from seed: String) -> Double {
        var accumulator: UInt64 = 0
        for u in seed.utf8 { accumulator = accumulator &* 1099511628211 &+ UInt64(u) }
        let base = Double(accumulator % 40_000) / 100.0
        return base + 5.0
    }
}

#Preview {
    ContentView()
}
