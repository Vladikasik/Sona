## Sona Backend (Go + SQLite)

Minimal service for parents/kids balances.

### Grid Integration (Squads Grid)

- Environment is controlled by `GRID_ENV` with default `sandbox`. Set to `production` for prod.
- Auth via `GRID_API_KEY` using `Authorization: Bearer <KEY>` header.
- Base URL defaults to `https://grid.squads.xyz/api/grid/v1/` and can be overridden via `GRID_BASE_URL`.
- Accounts endpoint defaults to `${GRID_BASE_URL}accounts` and can be overridden via `GRID_ACCOUNTS_URL`.
- OTP verification endpoint defaults to `${GRID_BASE_URL}accounts/verify` and can be overridden via `GRID_OTP_VERIFY_URL`.
- We generate HPKE keys using P-256 (ECDSA curve) and store DER-encoded private/public keys in DB.

Environment variables:

```bash
export GRID_ENV=sandbox
export GRID_API_KEY=your_squads_grid_api_key
# optional overrides
# export GRID_BASE_URL=https://grid.squads.xyz/api/grid/v1/
# export GRID_ACCOUNTS_URL=https://grid.squads.xyz/api/grid/v1/accounts
# export GRID_OTP_VERIFY_URL=https://grid.squads.xyz/api/grid/v1/accounts/verify
```

### Build (Linux default)
```bash
cd backend
make build
# outputs bin/server (linux amd64)
```

### Run
- Default bind: `0.0.0.0:33777`
- Override: `BIND_ADDR=127.0.0.1:33777`
```bash
./server
# or
BIND_ADDR=127.0.0.1:33777 ./server
```

### Endpoints

- GET `/get_parent?name=&email=`
  - Initiates Grid "Create Account" (type=email) with provided email.
  - If account already exists, the Grid conflict response is returned unchanged.
  - Query params:
    - `name` (string, required)
    - `email` (email, required)
  - Response: direct passthrough of Grid response
    - Success: 201/202 with pending verification payload
    - Conflict: 409 with
      {
        "message": "Resource conflict",
        "details": [{"field":"account","code":"RESOURCE_EXISTS","message":"Account already exists"}],
        "timestamp": "..."
      }
  - Example:
```bash
curl 'http://127.0.0.1:33777/get_parent?name=Alice&email=alice@example.com'
```

- POST `/grid/otp_verify`
  - Completes account creation by verifying OTP and supplying HPKE public key
  - Request body (JSON):
    - `email` (string, required)
    - `otp_code` (string length 6, required)
  - Behavior:
    - Backend injects Grid headers: `Authorization` and `x-grid-environment` and adds `x-idempotency-key`
    - Backend will include `kms_provider_config.encryption_public_key` (base64 of stored DER P-256 public key) to Grid
  - Response: direct passthrough of Grid response (200 on success)
```bash
curl -X POST 'http://127.0.0.1:33777/grid/otp_verify' \
     -H 'Content-Type: application/json' \
     -d '{"email":"alice@example.com","otp_code":"123456"}'
```

- GET `/list_kids?parent_id=`
  - Lists kid profiles for a parent (local DB helper for UI)
  - Query params:
    - `parent_id` (integer, required)
  - Response: `{ "kids": [{"id":number, "name":string, "balance":number}, ...] }`
  - Example:
```bash
curl 'http://127.0.0.1:33777/list_kids?parent_id=1'
```

Notes:
- `GET /get_child` is enabled.

References:
- Create Account: [grid.squads.xyz API](https://grid.squads.xyz/grid/v1/api-reference/endpoint/account-management/post)
- Verify Account OTP: [grid.squads.xyz API](https://grid.squads.xyz/grid/v1/api-reference/endpoint/account-management/verify)

### Data
- SQLite file at `backend/data/sona.db` (auto-created)


