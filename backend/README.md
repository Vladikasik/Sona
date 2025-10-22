## Sona Backend (Go + SQLite)

Minimal service for parents/kids balances.

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

- GET `/get_parent?name=`
  - Returns the parent by name if exists, otherwise creates and returns it.
```bash
curl 'http://127.0.0.1:33777/get_parent?name=Alice'
```

- GET `/get_child?name=&parent_id=`
  - If `parent_id` provided: creates kid with that name for the parent (if parent exists) and returns the kid.
  - If `parent_id` omitted: returns the kid by name or `null` if none; does not create.
```bash
curl 'http://127.0.0.1:33777/get_child?name=Bob&parent_id=1'
curl 'http://127.0.0.1:33777/get_child?name=Bob'
```

- GET `/list_kids?parent_id=`
```bash
curl 'http://127.0.0.1:33777/list_kids?parent_id=1'
```

- POST `/parent_topup?parent_id=&amount=`
```bash
curl -X POST 'http://127.0.0.1:33777/parent_topup?parent_id=1&amount=500'
```

- POST `/send_kid_money?parent_id=&kid_id=&amount=`
```bash
curl -X POST 'http://127.0.0.1:33777/send_kid_money?parent_id=1&kid_id=1&amount=200'
```

### Data
- SQLite file at `backend/data/sona.db` (auto-created)


