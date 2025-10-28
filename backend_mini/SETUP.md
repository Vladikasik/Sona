# Server Wallet Setup

The backend now uses a server-side wallet to create merkle trees directly on Solana.

## Setup Steps

1. **Generate a server wallet** (or use an existing one):
   ```bash
   # Install Solana CLI if needed
   # Then generate a new keypair:
   solana-keygen new --outfile server-wallet.json
   
   # Export private key in base58 format:
   solana-keygen pubkey server-wallet.json
   ```

2. **Set environment variable**:
   ```bash
   export SERVER_WALLET_PRIVATE_KEY="YOUR_PRIVATE_KEY_BASE58_HERE"
   ```

3. **Or add to your shell profile** (`~/.zshrc` or `~/.bashrc`):
   ```bash
   echo 'export SERVER_WALLET_PRIVATE_KEY="YOUR_PRIVATE_KEY_BASE58_HERE"' >> ~/.zshrc
   source ~/.zshrc
   ```

4. **Fund the wallet** on devnet:
   ```bash
   solana airdrop 1 YOUR_SERVER_WALLET_ADDRESS --url devnet
   ```

5. **Run the server**:
   ```bash
   ./server
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

