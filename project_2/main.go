package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"project_2/raftnode"
	"project_2/routes"

	"github.com/hashicorp/raft"
)

func main() {
	// Command-line flags for node configuration
	id := flag.String("id", "node1", "Node ID (e.g., node1, node2, node3)")
	dataDir := flag.String("data-dir", "./raft_data_node1", "Path to data directory")
	raftPort := flag.String("raft-port", "65362", "Raft bind port")
	httpPort := flag.String("http-port", "8080", "HTTP server port")
	bootstrap := flag.Bool("bootstrap", false, "Whether to bootstrap the cluster (only true for the first node)")
	flag.Parse()

	// Ensure all raft_data_nodeX directories exist
	dirs := []string{"./raft_data_node1", "./raft_data_node2", "./raft_data_node3"}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Fatalf("Failed to create %s directory: %v", dir, err)
		}
	}

	// Define Raft peers (all 3 nodes)
	peers := []raft.Server{
		{ID: raft.ServerID("node1"), Address: raft.ServerAddress("127.0.0.1:65362")},
		{ID: raft.ServerID("node2"), Address: raft.ServerAddress("127.0.0.1:65363")},
		{ID: raft.ServerID("node3"), Address: raft.ServerAddress("127.0.0.1:65364")},
	}

	// Compute bind address for this node
	bindAddr := fmt.Sprintf("127.0.0.1:%s", *raftPort)

	// Initialize Raft Node
	err := raftnode.InitializeRaftNode(*id, *dataDir, bindAddr, peers, *bootstrap)
	if err != nil {
		log.Fatalf("Failed to initialize Raft node: %v", err)
	}

	http.HandleFunc("/admin/leader", func(w http.ResponseWriter, r *http.Request) {
		leader := raftnode.RaftNode.Leader()
		if leader == "" {
			http.Error(w, "No leader elected", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Current leader: %s", leader)))
	})

	http.HandleFunc("/admin/status", func(w http.ResponseWriter, r *http.Request) {
		status := raftnode.RaftNode.Stats()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if jsonBytes, err := json.Marshal(status); err == nil {
			w.Write(jsonBytes)
		} else {
			http.Error(w, "Failed to marshal status", http.StatusInternalServerError)
		}
	})
	http.HandleFunc("/api/v1/printers", routes.PrintersHandler)
	http.HandleFunc("/api/v1/filaments", routes.FilamentsHandler)
	http.HandleFunc("/api/v1/print_jobs", routes.PrintJobsHandler)
	http.HandleFunc("/api/v1/print_jobs/", routes.PrintJobsHandler) // for subroutes like /api/v1/print_jobs/j1/status

	// Admin endpoint to add Raft voters dynamically
	http.HandleFunc("/admin/add_voter", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		nodeID := r.URL.Query().Get("id")
		addr := r.URL.Query().Get("addr")
		if nodeID == "" || addr == "" {
			http.Error(w, "Missing id or addr", http.StatusBadRequest)
			return
		}
		future := raftnode.RaftNode.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
		if err := future.Error(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Voter added"))
	})

	// Start HTTP server on the specified port
	log.Printf("Starting server on :%s...", *httpPort)
	log.Fatal(http.ListenAndServe(":"+*httpPort, nil))
}
