package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"project_2/raftnode"

	"github.com/hashicorp/raft"
)

type PrintJob = raftnode.PrintJob

func PrintJobsHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/print_jobs/") && strings.HasSuffix(r.URL.Path, "/status") {
		// Handle status update
		jobID := strings.TrimPrefix(r.URL.Path, "/api/v1/print_jobs/")
		jobID = strings.TrimSuffix(jobID, "/status")
		status := r.URL.Query().Get("status")
		if status == "" {
			http.Error(w, "Missing status query parameter", http.StatusBadRequest)
			return
		}
		if raftnode.RaftNode.State() != raft.Leader {
			leader := raftnode.RaftNode.Leader()
			if leader == "" {
				http.Error(w, "No leader elected", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("X-Raft-Leader", string(leader))
			http.Error(w, "Not leader", http.StatusTemporaryRedirect)
			return
		}
		entry := raftnode.LogEntry{Type: "UpdatePrintJobStatus", Data: map[string]interface{}{"job_id": jobID, "status": status}}
		b, _ := json.Marshal(entry)
		applyF := raftnode.RaftNode.Apply(b, 5*time.Second)
		if err := applyF.Error(); err != nil {
			http.Error(w, "Raft apply failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	switch r.Method {
	case http.MethodPost:
		if raftnode.RaftNode.State() != raft.Leader {
			leader := raftnode.RaftNode.Leader()
			if leader == "" {
				http.Error(w, "No leader elected", http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("X-Raft-Leader", string(leader))
			http.Error(w, "Not leader", http.StatusTemporaryRedirect)
			return
		}
		var job PrintJob
		if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		entry := raftnode.LogEntry{Type: "CreatePrintJob", Data: job}
		b, _ := json.Marshal(entry)
		applyF := raftnode.RaftNode.Apply(b, 5*time.Second)
		if err := applyF.Error(); err != nil {
			http.Error(w, "Raft apply failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	case http.MethodGet:
		fsm := raftnode.FSMInstance
		if fsm == nil {
			http.Error(w, "FSM unavailable", http.StatusInternalServerError)
			return
		}
		fsm.Mu.Lock()
		list := make([]raftnode.PrintJob, 0, len(fsm.PrintJobs))
		for _, j := range fsm.PrintJobs {
			list = append(list, j)
		}
		fsm.Mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
