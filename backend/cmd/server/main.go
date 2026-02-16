package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Jellman86/HarborWatch/backend/internal/httpapi"
)

func main() {
	// Fix permissions before starting
	fixPermissions()

	mux, schedSvc := httpapi.NewMuxWithScheduler()
	if schedSvc != nil {
		schedSvc.Start()
		defer schedSvc.Stop()
	}

	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("harborwatch server starting on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}
}

func fixPermissions() {
	puidStr := os.Getenv("PUID")
	pgidStr := os.Getenv("PGID")
	dbPath := os.Getenv("HARBORWATCH_DB_PATH")

	if puidStr == "" || pgidStr == "" || dbPath == "" {
		return
	}

	puid, err := strconv.Atoi(puidStr)
	if err != nil {
		return
	}
	pgid, err := strconv.Atoi(pgidStr)
	if err != nil {
		return
	}

	dataDir := filepath.Dir(dbPath)
	log.Printf("Ensuring ownership of %s is %d:%d", dataDir, puid, pgid)

	err = filepath.Walk(dataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(path, puid, pgid)
	})

	if err != nil {
		log.Printf("Warning: failed to fix permissions: %v", err)
	}
}
