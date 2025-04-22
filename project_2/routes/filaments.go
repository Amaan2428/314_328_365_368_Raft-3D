package routes

import (
	"encoding/json"
	"net/http"
	"time"

	"project_2/raftnode"
	"github.com/hashicorp/raft"
)

type Filament = raftnode.Filament

func FilamentsHandler(w http.ResponseWriter, r *http.Request) {
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
		var filament Filament
		if err := json.NewDecoder(r.Body).Decode(&filament); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		entry := raftnode.LogEntry{Type: "CreateFilament", Data: filament}
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
		list := make([]Filament, 0, len(fsm.Filaments))
		for _, f := range fsm.Filaments {
			list = append(list, f)
		}
		fsm.Mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
