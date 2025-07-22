package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mini-k8s-orchestration/internal/api"
	"mini-k8s-orchestration/internal/storage"
)

func main() {
	var (
		port    = flag.Int("port", 8080, "Port to run the API server on")
		dataDir = flag.String("data-dir", "./data", "Directory to store database files")
	)
	flag.Parse()

	// Initialize database
	db, err := storage.NewDatabase(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize repository
	repo := storage.NewSQLRepository(db)

	// Initialize API server
	server := api.NewServer(repo, *port)

	// Handle graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down API server...")
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting mini-k8s API server on port %d", *port)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}