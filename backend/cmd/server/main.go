package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Jellman86/HarborWatch/backend/internal/httpapi"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: httpapi.NewMux(),
	}

	log.Printf("harborwatch server starting on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}
