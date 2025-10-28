Backend Mini (Go + SQLite)

Auth: Bearer token required on all requests.
Token value: SonaBetaTestAPi

Build/Run (Linux-friendly, no CGO)
- go build ./cmd/server
- ./server

Endpoints
- POST /get_parent
  - Body: {"email":"e@example.com", "name":"Optional Name", "wallet":"2vjz4bEwmLo2VMdVv7gwnuvMUsPnWUDSXicVUefRrgTT", "upd":true}
  - Behavior:
    - Returns full parent row by email.
    - If name provided and email not found: creates parent.
    - If email exists and upd=true: updates name and/or wallet if provided.
    - wallet field accepts Solana wallet address as a string.

- POST /get_child
  - Body: {"email":"c@example.com", "name":"Optional", "parent_id":"A1B2C3", "wallet":"3vjz4bEwmLo2VMdVv7gwnuvMUsPnWUDSXicVUefRrgTT", "upd":true}
  - Behavior:
    - Returns full child row by email.
    - If email not found and both name and parent_id provided: creates child.
    - If email not found and missing name or parent_id: 400 with message "for user creation you need all: email, name, parent_id".
    - If email exists and upd=true: updates name and/or parent_id and/or wallet if provided.

- POST /eurc_tx
  - Body: {"wallet_from":"Fz..." , "wallet_to":"ABC...", "amount":"1000000"}
  - Behavior: Constructs EURC token transfer transaction with `transfer_checked` instruction
  - Creates recipient ATA if it doesn't exist (compatible with wallets that have no EURC balance)
  - Compatible with MPC wallets - transaction is signed on device
  - Returns: Unserialized transaction data for client-side signing

- POST /generate_merkletree
  - Body: {"owner_wallet":"Fz..."}
  - Behavior: Creates merkle tree using Node.js script with official Metaplex SDK
  - Returns: Tree ID, tree authority PDA, and transaction signature
  - Note: Requires SERVER_WALLET_PRIVATE_KEY environment variable and Node.js installed

- POST /mint_nft
  - Body: {"owner_wallet":"Fz...", "name":"Chore #1", "price":"100", "description":"Task", "send_to":"ABC...", "tree_id":"9GgFXzL5H6Yai7A2TNaEdU5cNqAvZM3Hpw3fQcqGGpAx"}
  - Behavior: Constructs compressed NFT mint transaction using Bubblegum program
  - Requires tree_id parameter for the merkle tree to mint into
  - Returns: Unserialized transaction data for client-side signing

- POST /upd_nft
  - Body: {"nft_address":"XYZ...", "new_status":"completed", "send_to":"ABC..."}
  - Behavior: Constructs NFT metadata update transaction + transfers NFT to recipient
  - Returns: Unserialized transaction data for client-side signing

- POST /accept_nft
  - Body: {"nft_address":"XYZ...", "sender_wallet":"ABC...", "payment_amount":"1000000"}
  - Behavior: Constructs transaction to burn NFT and pay sender with EURC
  - Returns: Unserialized transaction data for client-side signing

Notes
- parent_id in children is the parent's 6-character id.
- parents.kids_list is a JSON array of child ids and is kept in sync.
- All Light Protocol endpoints return unserialized transaction data.
- Transactions must be signed and serialized on device before submission.

Auth Header
- Authorization: Bearer SonaBetaTestAPi

Examples
curl -X POST http://localhost:33777/get_parent \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"email":"p@example.com","name":"Parent One"}'

curl -X POST http://localhost:33777/get_child \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"email":"c@example.com","name":"Child One","parent_id":"A1B2C3"}'

curl -X POST http://localhost:33777/eurc_tx \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"wallet_from":"Fz...","wallet_to":"ABC...","amount":"1000000"}'

curl -X POST http://localhost:33777/mint_nft \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"owner_wallet":"Fz...","name":"Chore #1","price":"100","description":"Task","send_to":"ABC...","tree_id":"9GgFXzL5H6Yai7A2TNaEdU5cNqAvZM3Hpw3fQcqGGpAx"}'


