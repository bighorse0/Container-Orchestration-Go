package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mini-k8s-orchestration/internal/scheduler"
	"mini-k8s-orchestration/internal/storage"
)

func main() {
	var (
		dataDir = flag.String("data-dir", "./data", "Directory to store data")
		useResourceScheduler = flag.Bool("resource-scheduler", true, "Use resource-aware scheduler")
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

	// Create base scheduler
	baseScheduler := scheduler.NewScheduler(repo)

	// Create scheduler instance
	var sched interface {
		Start()
		Stop()
	}

	if *useResourceScheduler {
		log.Println("Using resource-aware scheduler")
		sched = scheduler.NewResourceScheduler(baseScheduler)
	} else {
		log.Println("Using basic scheduler")
		sched = baseScheduler
	}

	// Start scheduler
	sched.Start()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down scheduler...")
	sched.Stop()
}