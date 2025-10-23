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

	gridEnv := os.Getenv("GRID_ENV")
	if gridEnv == "" {
		gridEnv = "sandbox"
	}
	gridAPIKey := os.Getenv("GRID_API_KEY")
	// Base URL and OTP URL can be configured, defaults provided
	gridBaseURL := os.Getenv("GRID_BASE_URL")
	if gridBaseURL == "" {
		gridBaseURL = "https://grid.squads.xyz/api/grid/v1/"
	}
	gridOTPURL := os.Getenv("GRID_OTP_VERIFY_URL")
	if gridOTPURL == "" {
		gridOTPURL = gridBaseURL + "accounts/verify"
	}

	// Auth endpoints for existing users
	gridAuthInitURL := os.Getenv("GRID_AUTH_INIT_URL")
	if gridAuthInitURL == "" {
		gridAuthInitURL = gridBaseURL + "auth"
	}
	gridAuthVerifyURL := os.Getenv("GRID_AUTH_VERIFY_URL")
	if gridAuthVerifyURL == "" {
		gridAuthVerifyURL = gridBaseURL + "auth/verify"
	}

	accountsURL := os.Getenv("GRID_ACCOUNTS_URL")
	if accountsURL == "" {
		accountsURL = gridBaseURL + "accounts"
	}
	s := &handlers.Server{DB: database, GridEnv: gridEnv, GridAPIKey: gridAPIKey, GridBaseURL: gridBaseURL, GridOTPVerifyURL: gridOTPURL, GridAccountsURL: accountsURL, GridAuthInitURL: gridAuthInitURL, GridAuthVerifyURL: gridAuthVerifyURL}

	// Endpoints kept for current scope
	http.HandleFunc("/get_parent", s.GetParent)                   // triggers Grid account creation or auth initiation
	http.HandleFunc("/grid/otp_verify", s.VerifyGridOTP)          // OTP verification for registration
	http.HandleFunc("/grid/auth_otp_verify", s.VerifyGridAuthOTP) // OTP verification for authentication
	http.HandleFunc("/list_kids", s.KidsList)                     // list kid profiles for a parent
	http.HandleFunc("/get_child", s.GetChild)                     // create or fetch child record

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
