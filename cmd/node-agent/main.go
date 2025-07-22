package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mini-k8s-orchestration/internal/agent"
)

func main() {
	var (
		nodeName     = flag.String("node-name", "", "Name of the node")
		apiServerURL = flag.String("api-server", "http://localhost:8080", "URL of the API server")
		dataDir      = flag.String("data-dir", "./data", "Directory to store data")
		heartbeatInterval = flag.Duration("heartbeat-interval", 30*time.Second, "Interval between heartbeats")
	)
	flag.Parse()

	// Validate flags
	if *nodeName == "" {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("Failed to get hostname: %v", err)
		}
		*nodeName = hostname
		log.Printf("Using hostname as node name: %s", *nodeName)
	}

	// Create node agent
	config := agent.Config{
		NodeName:         *nodeName,
		APIServerURL:     *apiServerURL,
		DataDir:          *dataDir,
		HeartbeatInterval: *heartbeatInterval,
	}

	nodeAgent, err := agent.NewNodeAgent(config)
	if err != nil {
		log.Fatalf("Failed to create node agent: %v", err)
	}

	// Start node agent
	if err := nodeAgent.Start(); err != nil {
		log.Fatalf("Failed to start node agent: %v", err)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down node agent...")
	nodeAgent.Stop()
}