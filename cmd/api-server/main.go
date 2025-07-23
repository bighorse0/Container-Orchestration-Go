package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mini-k8s-orchestration/internal/api"
	"mini-k8s-orchestration/internal/controller"
	"mini-k8s-orchestration/internal/loadbalancer"
	"mini-k8s-orchestration/internal/storage"
)

func main() {
	var (
		port           = flag.Int("port", 8080, "Port to run the API server on")
		lbPort         = flag.Int("lb-port", 8081, "Port to run the load balancer on")
		dataDir        = flag.String("data-dir", "./data", "Directory to store database files")
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

	// Initialize controllers
	nodeMonitor := controller.NewNodeMonitor(repo, 2*time.Minute)
	serviceController := controller.NewServiceController(repo)
	
	// Initialize load balancer
	lb := loadbalancer.NewLoadBalancer(repo)

	// Start controllers
	nodeMonitor.Start()
	serviceController.Start()
	
	// Start load balancer
	if err := lb.Start(*lbPort); err != nil {
		log.Fatalf("Failed to start load balancer: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		log.Println("Shutting down services...")
		nodeMonitor.Stop()
		serviceController.Stop()
		lb.Stop()
		os.Exit(0)
	}()

	// Start server
	log.Printf("Starting mini-k8s API server on port %d", *port)
	log.Printf("Starting load balancer on port %d", *lbPort)
	if err := server.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}