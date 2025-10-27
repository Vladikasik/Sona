import SwiftUI

struct ChildDetailView: View {
    let childEmail: String
    let onDismiss: () -> Void
    
    @State private var childName: String = ""
    @State private var childBalance: Double = 0.0
    @State private var isLoading: Bool = true
    @State private var errorMessage: String?
    @State private var showingSendMoney: Bool = false
    @State private var showingCreateChore: Bool = false
    
    var body: some View {
        NavigationStack {
            List {
                Section(header: Text("Child Account")) {
                    HStack {
                        Text("Name")
                        Spacer()
                        if isLoading {
                            ProgressView()
                        } else {
                            Text(childName.isEmpty ? childEmail : childName)
                                .foregroundColor(.secondary)
                        }
                    }
                    HStack {
                        Text("Email")
                        Spacer()
                        Text(childEmail)
                            .foregroundColor(.secondary)
                    }
                    HStack {
                        Text("Balance")
                        Spacer()
                        Text(formatBalance(childBalance))
                            .font(.headline)
                    }
                }
                
                Section(header: Text("Actions")) {
                    Button(action: {
                        showingSendMoney = true
                    }) {
                        HStack {
                            Image(systemName: "dollarsign.circle")
                            Text("Send Money")
                        }
                    }
                    
                    Button(action: {
                        showingCreateChore = true
                    }) {
                        HStack {
                            Image(systemName: "list.clipboard")
                            Text("Create Chore")
                        }
                    }
                }
                
                Section(header: Text("Transaction History")) {
                    Text("No transactions yet")
                        .foregroundColor(.secondary)
                        .frame(maxWidth: .infinity, alignment: .center)
                        .padding()
                }
            }
            .listStyle(.insetGrouped)
            .navigationTitle("Child Details")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .topBarLeading) {
                    Button("Close", action: onDismiss)
                }
            }
            .sheet(isPresented: $showingSendMoney) {
                SendMoneyView(
                    childEmail: childEmail,
                    childName: childName,
                    onDismiss: {
                        showingSendMoney = false
                    },
                    onSuccess: {
                        showingSendMoney = false
                        Task {
                            await loadChildDetails()
                        }
                    }
                )
            }
            .sheet(isPresented: $showingCreateChore) {
                CreateChoreView(
                    childEmail: childEmail,
                    childName: childName,
                    onDismiss: {
                        showingCreateChore = false
                    }
                )
            }
            .alert("Error", isPresented: Binding<Bool>(
                get: { errorMessage != nil },
                set: { if !$0 { errorMessage = nil } }
            ), actions: {
                Button("OK", role: .cancel) { errorMessage = nil }
            }, message: {
                Text(errorMessage ?? "")
            })
            .refreshable {
                await loadChildDetails()
            }
            .task {
                await loadChildDetails()
            }
        }
    }
    
    private func loadChildDetails() async {
        isLoading = true
        defer { isLoading = false }
        
        do {
            if let kid = try await TrackingAPI.getOrCreateChild(email: childEmail, name: nil, parentId: nil) {
                await MainActor.run {
                    childName = kid.name ?? childEmail
                    childBalance = kid.wallet
                }
            } else {
                await MainActor.run {
                    errorMessage = "Could not load child details"
                }
            }
        } catch {
            await MainActor.run {
                errorMessage = error.localizedDescription
            }
        }
    }
    
    private func formatBalance(_ balance: Double) -> String {
        String(format: "%.4f SOL", balance)
    }
}

struct SendMoneyView: View {
    let childEmail: String
    let childName: String
    let onDismiss: () -> Void
    let onSuccess: () -> Void
    
    @State private var amount: String = ""
    @State private var isSending: Bool = false
    @State private var errorMessage: String?
    
    private var amountBinding: Binding<String> {
        Binding(
            get: { amount },
            set: { newValue in
                amount = newValue.replacingOccurrences(of: ",", with: ".")
            }
        )
    }
    
    var body: some View {
        NavigationStack {
            Form {
                Section(header: Text("Amount")) {
                    HStack {
                        Text("SOL")
                        TextField("0.00", text: amountBinding)
                            .keyboardType(.decimalPad)
                    }
                }
                
                Section {
                    Button(action: {
                        Task { await sendMoney() }
                    }) {
                        HStack {
                            Spacer()
                            if isSending {
                                ProgressView()
                            } else {
                                Text("Send Money")
                            }
                            Spacer()
                        }
                    }
                    .disabled(isSending || amount.isEmpty || Double(amount) == nil || (Double(amount) ?? 0) <= 0)
                }
            }
            .navigationTitle("Send to \(childName)")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: onDismiss)
                }
            }
            .alert("Error", isPresented: Binding<Bool>(
                get: { errorMessage != nil },
                set: { if !$0 { errorMessage = nil } }
            ), actions: {
                Button("OK", role: .cancel) { errorMessage = nil }
            }, message: {
                Text(errorMessage ?? "")
            })
        }
    }
    
    private func sendMoney() async {
        print("[SendMoney] Starting send money: amount='\(amount)'")
        guard let amountValue = Double(amount), amountValue > 0 else {
            print("[SendMoney] Invalid amount: '\(amount)'")
            errorMessage = "Invalid amount"
            return
        }
        print("[SendMoney] Parsed amount: \(amountValue) SOL")
        
        isSending = true
        defer { isSending = false }
        
        do {
            var childWalletAddress: String?
            if let cache = BackendDataStore.shared.load(), let p = cache.parent, let kid = p.kids.first(where: { $0.email == childEmail }) {
                childWalletAddress = kid.walletAddress
            }
            if childWalletAddress == nil {
                if let kid = try await TrackingAPI.getOrCreateChild(email: childEmail, name: nil, parentId: nil) {
                    BackendDataStore.shared.mergeKid(kid)
                    childWalletAddress = kid.walletAddress
                }
            }
            guard let toAddress = childWalletAddress, !toAddress.isEmpty else {
                throw NSError(domain: "SendMoney", code: 1, userInfo: [NSLocalizedDescriptionKey: "Child wallet address missing. Ask child to share address in settings."])
            }
            print("[SendMoney] To address: \(toAddress)")

            if SessionManager.shared.address == nil || SessionManager.shared.address?.isEmpty == true {
                SessionManager.shared.ensureAddressFromStoredGrid()
            }
            var fromAddress: String? = SessionManager.shared.address
            if fromAddress == nil || fromAddress?.isEmpty == true {
                if let cache = BackendDataStore.shared.load(), let p = cache.parent, let addr = p.walletAddress, !addr.isEmpty {
                    fromAddress = addr
                }
            }
            guard let fromAddress, !fromAddress.isEmpty else {
                throw NSError(domain: "SendMoney", code: 2, userInfo: [NSLocalizedDescriptionKey: "Parent wallet address missing. Link Grid account in settings."])
            }
            print("[SendMoney] From address: \(fromAddress)")

            let request = SolanaTransferBuilder.TransferRequest(
                fromAddress: fromAddress,
                toAddress: toAddress,
                amountSOL: amountValue
            )
            
            let result = try await SolanaTransferBuilder.buildTransfer(request)
            print("[SendMoney] Built SOL transfer transaction")
            
            print("[SendMoney] Preparing transaction with Grid...")
            let prepare = try await GridTransfers.prepare(accountAddress: fromAddress, transactionB64: result.unsignedTransactionB64, signers: nil, payerAddress: fromAddress)
            print("[SendMoney] Grid prepare successful")
            
            SessionManager.shared.refreshDecryptedAuthorizationKeyIfPossible()
            let kmsSignatures = try GridKMSSigner.signKMSpayloadsIfNeeded(prepareData: prepare.raw)
            print("[SendMoney] Signed KMS payloads count: \(kmsSignatures.count)")
            
            let submit = try await GridTransfers.submit(accountAddress: fromAddress, preparedTransactionB64: prepare.decoded.data.transaction, kmsPayloadSignatures: kmsSignatures)
            print("[SendMoney] ✅ Transaction submitted: \(submit.data.transaction_signature ?? "pending")")
            
            await MainActor.run { onSuccess() }
        } catch {
            print("sendMoney error: \(error.localizedDescription)")
            if let urlErr = error as? URLError {
                print("[SendMoney][URLError] code=\(urlErr.code.rawValue) domain=NSURLErrorDomain")
            } else if let nsErr = error as NSError? {
                print("[SendMoney][NSError] domain=\(nsErr.domain) code=\(nsErr.code) userInfo=\(nsErr.userInfo)")
            }
            await MainActor.run {
                errorMessage = error.localizedDescription
            }
        }
    }
}

struct CreateChoreView: View {
    let childEmail: String
    let childName: String
    let onDismiss: () -> Void
    
    @State private var choreName: String = ""
    @State private var choreAmount: String = ""
    @State private var choreDescription: String = ""
    
    private var choreAmountBinding: Binding<String> {
        Binding(
            get: { choreAmount },
            set: { newValue in
                choreAmount = newValue.replacingOccurrences(of: ",", with: ".")
            }
        )
    }
    
    var body: some View {
        NavigationStack {
            Form {
                Section(header: Text("Chore Details")) {
                    TextField("Chore name", text: $choreName)
                        .textInputAutocapitalization(.words)
                    
                    HStack {
                        Text("SOL")
                        TextField("0.00", text: choreAmountBinding)
                            .keyboardType(.decimalPad)
                    }
                }
                
                Section(header: Text("Description (Optional)")) {
                    TextField("Add details about the chore", text: $choreDescription, axis: .vertical)
                        .lineLimit(3...6)
                }
                
                Section {
                    Button(action: {
                        onDismiss()
                    }) {
                        HStack {
                            Spacer()
                            Text("Create Chore")
                            Spacer()
                        }
                    }
                    .disabled(choreName.isEmpty || choreAmount.isEmpty || Double(choreAmount) == nil)
                }
            }
            .navigationTitle("New Chore for \(childName)")
            .navigationBarTitleDisplayMode(.inline)
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Cancel", action: onDismiss)
                }
            }
        }
    }
}

