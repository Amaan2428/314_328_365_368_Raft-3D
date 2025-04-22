package raftnode

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/hashicorp/raft"
)

// FSM implements raft.FSM for printers, filaments, and print jobs.
type FSM struct {
	Mu        sync.Mutex
	Printers  map[string]Printer
	Filaments map[string]Filament
	PrintJobs map[string]PrintJob
	Changelog []LogEntry // For event log/audit
}

type Printer struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Model   string `json:"model"`
}

type Filament struct {
	ID                     string `json:"id"`
	Type                   string `json:"type"`
	Color                  string `json:"color"`
	TotalWeightInGrams     int    `json:"total_weight_in_grams"`
	RemainingWeightInGrams int    `json:"remaining_weight_in_grams"`
}

type PrintJob struct {
	ID                 string `json:"id"`
	PrinterID          string `json:"printer_id"`
	FilamentID         string `json:"filament_id"`
	Filepath           string `json:"filepath"`
	PrintWeightInGrams int    `json:"print_weight_in_grams"`
	Status             string `json:"status"`
}

type LogEntry struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewFSM() *FSM {
	return &FSM{
		Printers:  make(map[string]Printer),
		Filaments: make(map[string]Filament),
		PrintJobs: make(map[string]PrintJob),
		Changelog: make([]LogEntry, 0),
	}
}

// Apply applies a Raft log entry to the FSM.
func (f *FSM) Apply(log *raft.Log) interface{} {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	var entry LogEntry
	if err := json.Unmarshal(log.Data, &entry); err != nil {
		return err
	}
	f.Changelog = append(f.Changelog, entry)

	switch entry.Type {
	case "CreatePrinter":
		var p Printer
		b, _ := json.Marshal(entry.Data)
		if err := json.Unmarshal(b, &p); err != nil {
			return "invalid printer data"
		}
		if p.ID == "" || p.Company == "" || p.Model == "" {
			return "missing printer fields"
		}
		f.Printers[p.ID] = p
	case "CreateFilament":
		var fil Filament
		b, _ := json.Marshal(entry.Data)
		if err := json.Unmarshal(b, &fil); err != nil {
			return "invalid filament data"
		}
		if fil.ID == "" || fil.Type == "" || fil.Color == "" || fil.TotalWeightInGrams <= 0 {
			return "missing or invalid filament fields"
		}
		fil.RemainingWeightInGrams = fil.TotalWeightInGrams
		f.Filaments[fil.ID] = fil
	case "CreatePrintJob":
		var job PrintJob
		b, _ := json.Marshal(entry.Data)
		if err := json.Unmarshal(b, &job); err != nil {
			return "invalid print job data"
		}
		// Validate printer and filament existence
		if _, ok := f.Printers[job.PrinterID]; !ok {
			return "printer not found"
		}
		fil, ok := f.Filaments[job.FilamentID]
		if !ok {
			return "filament not found"
		}
		if job.PrintWeightInGrams <= 0 || job.PrintWeightInGrams > fil.RemainingWeightInGrams {
			return "invalid or insufficient filament weight"
		}
		job.Status = "Queued"
		f.PrintJobs[job.ID] = job
	case "UpdatePrintJobStatus":
		var upd struct {
			JobID  string `json:"job_id"`
			Status string `json:"status"`
		}
		b, _ := json.Marshal(entry.Data)
		if err := json.Unmarshal(b, &upd); err != nil {
			return "invalid status update data"
		}
		pj, ok := f.PrintJobs[upd.JobID]
		if !ok {
			return "print job not found"
		}
		// Allowed transitions
		valid := false
		switch pj.Status {
		case "Queued":
			if upd.Status == "Running" || upd.Status == "Cancelled" {
				valid = true
			}
			if upd.Status == "Done" {
				valid = false
				// Cannot mark job as done from queued
				return "cannot mark job as done from queued"
			}
		case "Running":
			if upd.Status == "Done" || upd.Status == "Cancelled" {
				valid = true
			}
		}
		if !valid {
			return "invalid status transition"
		}
		// On Done, deduct filament weight
		if pj.Status == "Running" && upd.Status == "Done" {
			fil, ok := f.Filaments[pj.FilamentID]
			if ok {
				fil.RemainingWeightInGrams -= pj.PrintWeightInGrams
				if fil.RemainingWeightInGrams < 0 {
					fil.RemainingWeightInGrams = 0
				}
				f.Filaments[pj.FilamentID] = fil
			}
		}
		pj.Status = upd.Status
		f.PrintJobs[upd.JobID] = pj
	default:
		return "unknown log entry type"
	}
	return nil
}

// Snapshot returns a snapshot of the FSM.
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	// Deep copy state for snapshot
	printers := make(map[string]Printer)
	for k, v := range f.Printers {
		printers[k] = v
	}
	filaments := make(map[string]Filament)
	for k, v := range f.Filaments {
		filaments[k] = v
	}
	printJobs := make(map[string]PrintJob)
	for k, v := range f.PrintJobs {
		printJobs[k] = v
	}
	changelog := append([]LogEntry(nil), f.Changelog...)
	return &fsmSnapshot{
		Printers:  printers,
		Filaments: filaments,
		PrintJobs: printJobs,
		Changelog: changelog,
	}, nil
}

// Restore restores the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	decoder := json.NewDecoder(rc)
	var snap fsmSnapshot
	if err := decoder.Decode(&snap); err != nil {
		return err
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Printers = snap.Printers
	f.Filaments = snap.Filaments
	f.PrintJobs = snap.PrintJobs
	f.Changelog = snap.Changelog
	return nil
}

type fsmSnapshot struct {
	Printers  map[string]Printer
	Filaments map[string]Filament
	PrintJobs map[string]PrintJob
	Changelog []LogEntry
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
