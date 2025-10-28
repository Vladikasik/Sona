# Chores API Documentation

## Overview
The Chores API provides endpoints for managing chores between parents and children, including bounty tracking and automated EURC transfer when chores are completed.

## Database Schema

### Chores Table
```sql
CREATE TABLE IF NOT EXISTS chores (
    chore_id TEXT PRIMARY KEY,
    parent_wallet TEXT NOT NULL,
    child_wallet TEXT NOT NULL,
    chore_name TEXT NOT NULL,
    chore_description TEXT NOT NULL,
    bounty_amount INTEGER NOT NULL,
    chore_status INTEGER NOT NULL DEFAULT 0
);
```

### Chore Status Values
- `0`: Assigned
- `1`: Pending
- `3`: Completed
- `4`: Rejected

## Endpoints

All endpoints require bearer authentication header:
```
Authorization: Bearer SonaBetaTestAPi
```

### 1. Create Chore

**Endpoint:** `POST /create_chore`

**Request Body:**
```json
{
  "parent_wallet": "string",
  "child_wallet": "string",
  "chore_name": "string",
  "chore_description": "string",
  "bounty_amount": "string"
}
```

**Response:** Chore object with status 0 (assigned)
```json
{
  "chore_id": "ABC123",
  "parent_wallet": "...",
  "child_wallet": "...",
  "chore_name": "Take out trash",
  "chore_description": "Empty all bins and take to curb",
  "bounty_amount": 1000000,
  "chore_status": 0
}
```

### 2. Update Chore

**Endpoint:** `POST /update_chore`

**Request Body:**
```json
{
  "chore_id": "string",
  "new_status": 3
}
```

**Response for completed chores (status 3):**
```json
{
  "chore": {
    "chore_id": "ABC123",
    "parent_wallet": "...",
    "child_wallet": "...",
    "chore_name": "Take out trash",
    "chore_description": "Empty all bins and take to curb",
    "bounty_amount": 1000000,
    "chore_status": 3
  },
  "transaction": {
    "serialized": "...",
    "instructions": [...],
    "recent_blockhash": "...",
    "fee_payer": "...",
    "required_signatures": [...]
  }
}
```

**Response for other status updates:**
```json
{
  "chore_id": "ABC123",
  "parent_wallet": "...",
  "child_wallet": "...",
  "chore_name": "Take out trash",
  "chore_description": "Empty all bins and take to curb",
  "bounty_amount": 1000000,
  "chore_status": 1
}
```

### 3. Get Chores

**Endpoint:** `POST /get_chores`

**Request Body:**
```json
{
  "wallet": "string"
}
```

**Response:** Array of chores
```json
[
  {
    "chore_id": "ABC123",
    "parent_wallet": "...",
    "child_wallet": "...",
    "chore_name": "Take out trash",
    "chore_description": "Empty all bins and take to curb",
    "bounty_amount": 1000000,
    "chore_status": 0
  },
  {
    "chore_id": "DEF456",
    "parent_wallet": "...",
    "child_wallet": "...",
    "chore_name": "Clean room",
    "chore_description": "Organize toys and make bed",
    "bounty_amount": 2000000,
    "chore_status": 1
  }
]
```

**Query Logic:**
Returns all chores where the provided wallet matches either:
- `parent_wallet` (chores created by this parent)
- `child_wallet` (chores assigned to this child)

## Example Usage

### Create a chore
```bash
curl -X POST http://127.0.0.1:33777/create_chore \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{
    "parent_wallet": "ParentWalletAddress123",
    "child_wallet": "ChildWalletAddress456",
    "chore_name": "Wash dishes",
    "chore_description": "Load dishwasher and run it",
    "bounty_amount": "1500000"
  }'
```

### Update chore to completed
```bash
curl -X POST http://127.0.0.1:33777/update_chore \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{
    "chore_id": "ABC123",
    "new_status": 3
  }'
```

### Get all chores for a wallet
```bash
curl -X POST http://127.0.0.1:33777/get_chores \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{
    "wallet": "ParentWalletAddress123"
  }'
```

## Transaction Details

When a chore is marked as completed (status 3), the API automatically constructs a EURC transfer transaction:
- **From:** Parent wallet
- **To:** Child wallet
- **Amount:** Bounty amount specified when creating the chore
- **Currency:** EURC (Euro Coin)

The transaction includes all necessary instructions for:
1. Creating associated token accounts if needed
2. Transferring EURC tokens

This transaction must be signed and submitted by the client application.

## Error Responses

All endpoints return standard error responses:
```json
{
  "error": "error message"
}
```

Common status codes:
- `400`: Bad Request (invalid input)
- `401`: Unauthorized (missing or invalid bearer token)
- `404`: Not Found (chore not found)
- `500`: Internal Server Error

