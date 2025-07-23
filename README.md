# Container Orchestration in Go (mini-k8s)

This project is a lightweight container orchestration system inspired by Kubernetes, written in Go. It provides basic functionalities for deploying and managing containerized applications across a cluster of nodes.

## Features

*   **API Server**: A central component that exposes a REST API for managing the cluster.
*   **Node Agent**: Runs on each worker node, registers with the API server, and manages container lifecycles.
*   **Scheduler**: Assigns new containers to appropriate nodes based on either a basic or a resource-aware scheduling strategy.
*   **Service Discovery**: A simple service discovery mechanism.
*   **Load Balancer**: A basic load balancer to distribute traffic among services.
*   **Database-backed State**: Uses SQLite to store the cluster's state, ensuring persistence.

## Architecture

The orchestration system consists of three main components:

1.  **API Server**: The brain of the cluster. It handles API requests, manages the cluster state, and orchestrates the other components.
2.  **Node Agent**: A lightweight agent that runs on each worker node. It communicates with the API server to receive commands and report the status of the node and its containers.
3.  **Scheduler**: A pluggable component that decides where to run new containers. It can be configured to use a simple round-robin scheduler or a more advanced resource-aware scheduler.

## How to Run

### Prerequisites

*   Go (version 1.24.1 or higher)
*   Docker

### Building the Components

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/your-username/Container-Orchestration-Go.git
    cd Container-Orchestration-Go
    ```

2.  **Build the binaries:**
    ```bash
    go build -o bin/api-server ./cmd/api-server
    go build -o bin/node-agent ./cmd/node-agent
    go build -o bin/scheduler ./cmd/scheduler
    ```

### Running the Cluster

1.  **Start the API Server:**
    ```bash
    ./bin/api-server
    ```
    By default, the API server listens on port 8080 and the load balancer on port 8081.

2.  **Start the Scheduler:**
    ```bash
    ./bin/scheduler
    ```
    The scheduler will connect to the API server and start scheduling pods.

3.  **Start the Node Agent(s):**
    On each worker node, run the following command:
    ```bash
    ./bin/node-agent --node-name=node1 --api-server=http://<api-server-ip>:8080
    ```
    Replace `<api-server-ip>` with the IP address of the machine running the API server.

## Future Work

*   **CLI:** Implement a command-line interface for interacting with the cluster.
*   **Networking:** Improve the networking model to allow for more complex application deployments.
*   **Storage:** Add support for persistent storage volumes.
*   **Security:** Implement authentication and authorization for the API server.
*   **More scheduling options:** Add more advanced scheduling features like affinity and anti-affinity.
