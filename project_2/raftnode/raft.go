package raftnode

import (
	"os"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

var RaftNode *raft.Raft
var FSMInstance *FSM

func InitializeRaftNode(nodeID, dataDir string, bindAddr string, peers []raft.Server, bootstrap bool) error {
	// Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	// Log store
	logStore, err := raftboltdb.NewBoltStore(dataDir + "/raft-log.bolt")
	if err != nil {
		return err
	}

	// Stable store
	stableStore, err := raftboltdb.NewBoltStore(dataDir + "/raft-stable.bolt")
	if err != nil {
		return err
	}

	// Snapshot store
	snapshotStore, err := raft.NewFileSnapshotStore(dataDir, 2, os.Stdout)
	if err != nil {
		return err
	}

	// Transport
	transport, err := raft.NewTCPTransport(bindAddr, nil, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return err
	}

	fsm := NewFSM()
	FSMInstance = fsm
	RaftNode, err = raft.NewRaft(config, fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	// Bootstrap cluster only if requested
	if bootstrap {
		configuration := raft.Configuration{Servers: peers}
		RaftNode.BootstrapCluster(configuration)
	}

	return nil
}
