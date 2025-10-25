Backend Mini (Go + SQLite)

Auth: Bearer token required on all requests.
Token value: SonaBetaTestAPi

Build/Run (Linux-friendly, no CGO)
- go build ./cmd/server
- ./server

Endpoints
- POST /get_parent
  - Body: {"email":"e@example.com", "name":"Optional Name", "wallet":123.45, "upd":true}
  - Behavior:
    - Returns full parent row by email.
    - If name provided and email not found: creates parent.
    - If email exists and upd=true: updates name and/or wallet if provided.

- POST /get_child
  - Body: {"email":"c@example.com", "name":"Optional", "parent_id":"A1B2C3", "wallet":50, "upd":true}
  - Behavior:
    - Returns full child row by email.
    - If email not found and both name and parent_id provided: creates child.
    - If email not found and missing name or parent_id: 400 with message "for user creation you need all: email, name, parent_id".
    - If email exists and upd=true: updates name and/or parent_id and/or wallet if provided.

Notes
- parent_id in children is the parent's 6-character id.
- parents.kids_list is a JSON array of child ids and is kept in sync.

Auth Header
- Authorization: Bearer SonaBetaTestAPi

Examples
curl -X POST http://localhost:8080/get_parent \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"email":"p@example.com","name":"Parent One"}'

curl -X POST http://localhost:8080/get_child \
  -H "Authorization: Bearer SonaBetaTestAPi" \
  -H "Content-Type: application/json" \
  -d '{"email":"c@example.com","name":"Child One","parent_id":"A1B2C3"}'


