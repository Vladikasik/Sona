## Sona Backend (Go + SQLite)

Unified registration/authentication for parents and kids using Squads Grid. Parents and kids both own a Grid account (email-based); in SQLite they are stored in separate tables. Kids also have a required parent_id.

### Grid Integration
- `GRID_ENV`: `sandbox` (default) or `production`
- `GRID_API_KEY`: Bearer key for Grid
- `GRID_BASE_URL`: default `https://grid.squads.xyz/api/grid/v1/`
- Derived defaults (overridable):
  - `GRID_ACCOUNTS_URL=${GRID_BASE_URL}accounts`
  - `GRID_OTP_VERIFY_URL=${GRID_BASE_URL}accounts/verify`
  - `GRID_AUTH_INIT_URL=${GRID_BASE_URL}auth`
  - `GRID_AUTH_VERIFY_URL=${GRID_BASE_URL}auth/verify`
- HPKE keys: P-256, DER-encoded, stored per user (parent or kid) before OTP verification.

Env:
```bash
export GRID_ENV=sandbox
export GRID_API_KEY=your_squads_grid_api_key
# optional overrides
# export GRID_BASE_URL=...
# export GRID_ACCOUNTS_URL=...
# export GRID_OTP_VERIFY_URL=...
# export GRID_AUTH_INIT_URL=...
# export GRID_AUTH_VERIFY_URL=...
```

### Build
```bash
cd backend
make build
```

### Run
```bash
./server
# or
BIND_ADDR=127.0.0.1:33777 ./server
```

### Endpoints (Unified Flow)

- GET `/get_user?user_type=parent&name=&email=`
  - Parent registration/auth initiation. Creates or loads parent, ensures HPKE keys, then calls Grid Accounts. If account exists, initiates Grid Auth instead.
  - Response indicates whether the user already exists and which OTP endpoint to use next.
  - Response format (both parent and kid):
```json
{ "user_type": "new" | "existing", "otp_verify_path": "/grid/otp_verify" | "/grid/auth_otp_verify", "grid": { /* Grid raw response */ } }
```
  - Semantics:
    - `user_type: "new"` → user does not exist yet on Grid; call `POST /grid/otp_verify`.
    - `user_type: "existing"` → user already exists on Grid; call `POST /grid/auth_otp_verify`.
  - Example:
```bash
curl 'http://127.0.0.1:33777/get_user?user_type=parent&name=Alice&email=alice@example.com'
```

- GET `/get_user?user_type=kid&name=&email=&parent_id=`
  - Kid registration/auth initiation (mirrors parent flow). Requires `email` and `parent_id`. Creates or loads kid under the parent, ensures HPKE keys, then calls Grid Accounts or Grid Auth.
  - Uses the same response contract as the parent variant (see above) to indicate whether to use registration OTP or auth OTP.
  - Example:
```bash
curl 'http://127.0.0.1:33777/get_user?user_type=kid&name=Bob&email=bob.kid@example.com&parent_id=1'
```

- POST `/grid/otp_verify`
  - Completes registration by verifying OTP and providing HPKE public key. Used for both parents and kids during registration.
  - Body:
```json
{ "email": "user@example.com", "otp_code": "123456" }
```

- POST `/grid/auth_otp_verify`
  - Completes authentication for existing Grid account. Used for both parents and kids during login.
  - Body:
```json
{ "email": "user@example.com", "otp_code": "123456" }
```

- GET `/list_kids?parent_id=`
  - Lists kids for a parent.

Legacy endpoints kept for compatibility:
- GET `/get_parent?name=&email=`
- GET `/get_child?name=&parent_id=`

### Flow Summary
1) Client calls `/get_user` with `user_type` (parent or kid), `name`, and `email`. For kids also include `parent_id`.
2) Backend ensures a local record exists and generates HPKE keys if missing.
3) Backend calls Grid Accounts (registration). If the account already exists, backend switches to Grid Auth initiation.
4) Client completes OTP step via either `/grid/otp_verify` (registration) or `/grid/auth_otp_verify` (login).
5) On OTP verify, backend persists Grid identifiers.

### Data Model
- parents: `id, uid, name, email, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id`
- kids: `id, uid, parent_id, name, email, balance, hpke_priv_der, hpke_pub_der, grid_user_id, grid_wallet_id`

Notes
- Both parents and kids have `uid` generated server-side.
- Kids must include `parent_id` when registering; otherwise backend returns an error.

References
- Grid API docs: `https://grid.squads.xyz/grid/v1/api-reference/`