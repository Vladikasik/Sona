package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"backend_mini/internal/config"
	"backend_mini/internal/db"
	"backend_mini/internal/handlers"
	"backend_mini/internal/middleware"
)

func main() {
	ctx := context.Background()

	if err := config.LoadServerWallet(); err != nil {
		log.Fatalf("failed to load server wallet: %v", err)
	}
	log.Println("✓ Server wallet loaded")

	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Fatalf("failed to create data dir: %v", err)
	}

	databasePath := dataDir + "/sona_mini.db"
	database, err := db.Open(ctx, databasePath)
	if err != nil {
		log.Fatalf("failed opening db: %v", err)
	}
	defer database.Close()

	if err := database.Migrate(ctx); err != nil {
		log.Fatalf("failed migrating db: %v", err)
	}

	api := handlers.NewAPI(database)
	mux := http.NewServeMux()

	mux.Handle("/get_parent", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.GetParent)))
	mux.Handle("/get_child", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.GetChild)))
	mux.Handle("/eurc_tx", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.EurcTx)))
	mux.Handle("/generate_merkletree", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.GenerateMerkleTree)))
	mux.Handle("/mint_nft", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.MintNFT)))
	mux.Handle("/upd_nft", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.UpdNFT)))
	mux.Handle("/accept_nft", middleware.RequireBearer("SonaBetaTestAPi", http.HandlerFunc(api.AcceptNFT)))

	// wrap with logging middleware
	handler := middleware.LogRequests(mux)

	srv := &http.Server{
		Addr:              "127.0.0.1:33777",
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("backend_mini listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
