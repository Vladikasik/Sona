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

- POST `/register_parent?name=`
```bash
curl -X POST 'http://127.0.0.1:33777/register_parent?name=Alice'
```

- POST `/register_kid?name=&parent_id=`
```bash
curl -X POST 'http://127.0.0.1:33777/register_kid?name=Bob&parent_id=1'
```

- GET `/kids_list?parent_id=`
```bash
curl 'http://127.0.0.1:33777/kids_list?parent_id=1'
```

- POST `/parent_topup?parent_id=&amount=`
```bash
curl -X POST 'http://127.0.0.1:33777/parent_topup?parent_id=1&amount=500'
```

- POST `/send_kid_money?parent_id=&kid_id=&amount=`
- GET `/get_parent?id=` or `/get_parent?name=` (one required)
```bash
curl 'http://127.0.0.1:33777/get_parent?id=1'
curl 'http://127.0.0.1:33777/get_parent?name=Alice'
```

- GET `/get_child?id=` or `/get_child?name=` (one required)
```bash
curl 'http://127.0.0.1:33777/get_child?id=1'
curl 'http://127.0.0.1:33777/get_child?name=Bob'
```
```bash
curl -X POST 'http://127.0.0.1:33777/send_kid_money?parent_id=1&kid_id=1&amount=200'
```

### Data
- SQLite file at `backend/data/sona.db` (auto-created)


