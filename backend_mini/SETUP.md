# Server Wallet Setup

The backend now uses a Node.js script with official Metaplex/Bubblegum SDKs to create merkle trees.

## Setup Steps

1. **Install Node.js** (if not already installed):
   ```bash
   node --version  # Should be >= 20.11.1
   ```

2. **Install npm dependencies**:
   ```bash
   cd backend_mini
   npm install
   ```

3. **Generate a server wallet** (or use an existing one):
   ```bash
   # Using Solana CLI:
   solana-keygen new --outfile server-wallet.json
   
   # Get the base58 private key:
   cat server-wallet.json | jq -r '.[0:64]'  # This gives you the seed
   # OR get it in hex format from the full keypair
   ```

4. **Set environment variable** (in base58 format):
   ```bash
   export SERVER_WALLET_PRIVATE_KEY="YOUR_PRIVATE_KEY_BASE58_HERE"
   ```
   
   The private key can be in:
   - Base58 format (88 characters) - recommended
   - Hex format (128 characters)  
   - Base64 format

5. **Fund the wallet** on devnet:
   ```bash
   solana airdrop 1 YOUR_SERVER_WALLET_ADDRESS --url devnet
   ```

6. **Build and run the server**:
   ```bash
   go build -o bin/backend_mini ./cmd/server
   ./bin/backend_mini
   ```

## Generating Tree

The `/generate_merkletree` endpoint now:
- Creates the tree using the server wallet
- Signs and submits directly to Solana (bypasses Grid)
- Returns tree_id, tree_authority, and transaction signature

Request:
```json
{
  "owner_wallet": "Parent's wallet address"
}
```

Response:
```json
{
  "tree_id": "Generated tree address",
  "tree_authority": "Tree authority PDA",
  "authority": "Parent's wallet address",
  "signature": "Transaction signature",
  "message": "Merkle tree created successfully"
}
```

## Notes

- The server wallet needs SOL to pay for rent (~0.006 SOL per tree)
- Tree creation happens immediately and synchronously
- No client-side signing required for tree creation
- The requesting wallet doesn't need to have SOL

