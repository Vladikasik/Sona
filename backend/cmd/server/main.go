package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"sona-backend/internal/db"
	"sona-backend/internal/handlers"
)

func main() {
	dataPath := filepath.Join("data", "sona.db")
	if err := os.MkdirAll(filepath.Dir(dataPath), 0o755); err != nil {
		log.Fatalf("data dir create error: %v", err)
	}
	database, err := db.Open(dataPath)
	if err != nil {
		log.Fatalf("db open error: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("db close error: %v", err)
		}
	}()

	s := &handlers.Server{DB: database}

	http.HandleFunc("/register_parent", s.RegisterParent)
	http.HandleFunc("/register_kid", s.RegisterKid)
	http.HandleFunc("/kids_list", s.KidsList)
	http.HandleFunc("/parent_topup", s.ParentTopUp)
	http.HandleFunc("/send_kid_money", s.SendKidMoney)
	http.HandleFunc("/get_parent", s.GetParent)
	http.HandleFunc("/get_child", s.GetChild)

	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		bindAddr = "0.0.0.0:33777"
	}
	ln, err := net.Listen("tcp", bindAddr)
	if err != nil {
		log.Fatalf("listen error: %v", err)
	}
	log.Printf("server listening on %s", ln.Addr().String())
	if err := http.Serve(ln, nil); err != nil {
		log.Fatalf("serve error: %v", err)
	}
}
