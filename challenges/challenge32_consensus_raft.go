// Challenge 32: Raft Consensus Algorithm
//
// Problem: The Raft consensus implementation has critical bugs:
// 1. Leader election can result in multiple leaders (split-brain)
// 2. Log replication doesn't ensure consistency
// 3. Committed entries can be lost
// 4. Network partitions cause data inconsistency
// 5. Snapshot/compaction is broken
// 6. Configuration changes are not safe
// 7. Read consistency is not guaranteed
// 8. Follower logs can diverge from leader
//
// Your task: Fix the Raft implementation to ensure strong consistency.

package challenges

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// RaftNode represents a node in the Raft cluster
type RaftNode struct {
	ID           string
	State        NodeState
	CurrentTerm  uint64
	VotedFor     string
	Log          []LogEntryRaft
	CommitIndex  int64
	LastApplied  int64

	// Leader state
	NextIndex    map[string]int64
	MatchIndex   map[string]int64

	// Candidate state
	VotesReceived map[string]bool

	// Cluster
	Peers        []string
	LeaderID     string

	// Channels
	appendEntryCh chan AppendEntryRequest
	voteCh        chan VoteRequest
	heartbeatCh   chan bool
	commitCh      chan LogEntryRaft

	// Timers
	electionTimer  *time.Timer
	heartbeatTimer *time.Timer

	// State machine
	StateMachine map[string]interface{}

	mu           sync.RWMutex
	shutdownCh   chan struct{}
	isLeader     int32
}

// NodeState represents the state of a Raft node
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

// LogEntryRaft represents an entry in the Raft log
type LogEntryRaft struct {
	Term    uint64
	Index   int64
	Command interface{}
	// BUG: No client ID for deduplication
}

// AppendEntryRequest is sent by leaders to replicate logs
type AppendEntryRequest struct {
	Term         uint64
	LeaderID     string
	PrevLogIndex int64
	PrevLogTerm  uint64
	Entries      []LogEntryRaft
	LeaderCommit int64
	From         string
}

// AppendEntryResponse is the response to AppendEntryRequest
type AppendEntryResponse struct {
	Term    uint64
	Success bool
	// BUG: No conflict information for faster log convergence
}

// VoteRequest is sent by candidates during elections
type VoteRequest struct {
	Term         uint64
	CandidateID  string
	LastLogIndex int64
	LastLogTerm  uint64
}

// VoteResponse is the response to VoteRequest
type VoteResponse struct {
	Term        uint64
	VoteGranted bool
}

// RaftCluster manages a cluster of Raft nodes
type RaftCluster struct {
	Nodes      map[string]*RaftNode
	Network    *NetworkSimulator
	mu         sync.RWMutex
}

// NetworkSimulator simulates network conditions
type NetworkSimulator struct {
	Partitions map[string]bool
	Latency    time.Duration
	DropRate   float32
	mu         sync.RWMutex
}

// NewRaftNode creates a new Raft node
func NewRaftNode(id string, peers []string) *RaftNode {
	node := &RaftNode{
		ID:            id,
		State:         Follower,
		CurrentTerm:   0,
		Peers:         peers,
		Log:           make([]LogEntryRaft, 0),
		CommitIndex:   -1,
		LastApplied:   -1,
		NextIndex:     make(map[string]int64),
		MatchIndex:    make(map[string]int64),
		VotesReceived: make(map[string]bool),
		StateMachine:  make(map[string]interface{}),
		appendEntryCh: make(chan AppendEntryRequest, 100),
		voteCh:        make(chan VoteRequest, 100),
		heartbeatCh:   make(chan bool, 1),
		commitCh:      make(chan LogEntryRaft, 100),
		shutdownCh:    make(chan struct{}),
	}

	// Initialize next/match indices
	for _, peer := range peers {
		node.NextIndex[peer] = 0  // BUG: Should be len(log)
		node.MatchIndex[peer] = -1
	}

	return node
}

// Start starts the Raft node
func (n *RaftNode) Start() {
	n.resetElectionTimer()
	go n.run()
}

// run is the main loop for the Raft node
func (n *RaftNode) run() {
	for {
		select {
		case <-n.shutdownCh:
			return

		case <-n.electionTimer.C:
			n.startElection()

		case <-n.heartbeatTimer.C:
			if atomic.LoadInt32(&n.isLeader) == 1 {
				n.sendHeartbeats()
			}

		case req := <-n.appendEntryCh:
			n.handleAppendEntry(req)

		case req := <-n.voteCh:
			n.handleVoteRequest(req)

		case entry := <-n.commitCh:
			n.applyEntry(entry)
		}
	}
}

// startElection starts a leader election
func (n *RaftNode) startElection() {
	n.mu.Lock()
	n.State = Candidate
	n.CurrentTerm++
	n.VotedFor = n.ID
	n.VotesReceived = map[string]bool{n.ID: true}
	currentTerm := n.CurrentTerm
	n.mu.Unlock()

	log.Printf("Node %s starting election for term %d", n.ID, currentTerm)

	// BUG: Not resetting election timer

	// Request votes from peers
	for _, peer := range n.Peers {
		go n.requestVote(peer, currentTerm)
	}

	// BUG: Not waiting for majority before becoming leader
	time.Sleep(50 * time.Millisecond)

	n.mu.Lock()
	if len(n.VotesReceived) > len(n.Peers)/2 {
		// BUG: Not checking if still candidate
		n.becomeLeader()
	}
	n.mu.Unlock()
}

// requestVote sends a vote request to a peer
func (n *RaftNode) requestVote(peer string, term uint64) {
	n.mu.RLock()
	lastLogIndex := int64(len(n.Log) - 1)
	lastLogTerm := uint64(0)
	if lastLogIndex >= 0 {
		lastLogTerm = n.Log[lastLogIndex].Term
	}
	n.mu.RUnlock()

	req := VoteRequest{
		Term:         term,
		CandidateID:  n.ID,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	// Simulate sending vote request
	// BUG: No actual network communication
	resp := VoteResponse{
		Term:        term,
		VoteGranted: rand.Float32() > 0.3, // BUG: Random voting
	}

	if resp.VoteGranted {
		n.mu.Lock()
		n.VotesReceived[peer] = true
		n.mu.Unlock()
	}
}

// handleVoteRequest handles incoming vote requests
func (n *RaftNode) handleVoteRequest(req VoteRequest) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// BUG: Not checking if log is up-to-date
	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.State = Follower
		n.VotedFor = ""
		// BUG: Not resetting leader
	}

	voteGranted := false
	if req.Term >= n.CurrentTerm && (n.VotedFor == "" || n.VotedFor == req.CandidateID) {
		// BUG: Not checking log completeness
		voteGranted = true
		n.VotedFor = req.CandidateID
		n.resetElectionTimer()
	}

	// Send response (simulated)
	_ = VoteResponse{
		Term:        n.CurrentTerm,
		VoteGranted: voteGranted,
	}
}

// becomeLeader transitions to leader state
func (n *RaftNode) becomeLeader() {
	log.Printf("Node %s became leader for term %d", n.ID, n.CurrentTerm)

	n.State = Leader
	n.LeaderID = n.ID
	atomic.StoreInt32(&n.isLeader, 1)

	// BUG: Not reinitializing NextIndex and MatchIndex

	// Start heartbeat timer
	n.heartbeatTimer = time.NewTimer(50 * time.Millisecond)

	// Send initial heartbeats
	n.sendHeartbeats()
}

// sendHeartbeats sends heartbeats to all peers
func (n *RaftNode) sendHeartbeats() {
	n.mu.RLock()
	currentTerm := n.CurrentTerm
	commitIndex := n.CommitIndex
	n.mu.RUnlock()

	for _, peer := range n.Peers {
		go n.sendAppendEntry(peer, currentTerm, commitIndex)
	}

	// Reset heartbeat timer
	n.heartbeatTimer.Reset(50 * time.Millisecond)
}

// sendAppendEntry sends append entry to a peer
func (n *RaftNode) sendAppendEntry(peer string, term uint64, commitIndex int64) {
	n.mu.RLock()
	nextIndex := n.NextIndex[peer]

	// BUG: Not checking bounds
	prevLogIndex := nextIndex - 1
	prevLogTerm := uint64(0)
	if prevLogIndex >= 0 && prevLogIndex < int64(len(n.Log)) {
		prevLogTerm = n.Log[prevLogIndex].Term
	}

	// Get entries to send
	entries := make([]LogEntryRaft, 0)
	if nextIndex < int64(len(n.Log)) {
		// BUG: Sending entire log suffix
		entries = n.Log[nextIndex:]
	}
	n.mu.RUnlock()

	req := AppendEntryRequest{
		Term:         term,
		LeaderID:     n.ID,
		PrevLogIndex: prevLogIndex,
		PrevLogTerm:  prevLogTerm,
		Entries:      entries,
		LeaderCommit: commitIndex,
		From:         n.ID,
	}

	// Simulate sending (no actual network)
	// BUG: Always succeeds
	resp := AppendEntryResponse{
		Term:    term,
		Success: true,
	}

	if resp.Success {
		n.mu.Lock()
		// BUG: Not updating NextIndex and MatchIndex correctly
		n.NextIndex[peer] = nextIndex + int64(len(entries))
		n.MatchIndex[peer] = n.NextIndex[peer] - 1
		n.mu.Unlock()

		// BUG: Not checking for majority to update commit index
	}
}

// handleAppendEntry handles incoming append entries
func (n *RaftNode) handleAppendEntry(req AppendEntryRequest) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Reset election timer
	n.resetElectionTimer()

	// BUG: Not checking term properly
	if req.Term < n.CurrentTerm {
		// Reject
		return
	}

	if req.Term > n.CurrentTerm {
		n.CurrentTerm = req.Term
		n.State = Follower
		n.VotedFor = ""
		atomic.StoreInt32(&n.isLeader, 0)
	}

	n.LeaderID = req.LeaderID

	// BUG: Not checking log consistency
	// Append entries
	if len(req.Entries) > 0 {
		// BUG: Just appending without checking conflicts
		n.Log = append(n.Log, req.Entries...)
	}

	// Update commit index
	if req.LeaderCommit > n.CommitIndex {
		// BUG: Not taking minimum of LeaderCommit and last log index
		n.CommitIndex = req.LeaderCommit

		// Apply committed entries
		n.applyCommittedEntries()
	}
}

// applyCommittedEntries applies committed entries to state machine
func (n *RaftNode) applyCommittedEntries() {
	// BUG: Not applying in order
	for n.LastApplied < n.CommitIndex {
		n.LastApplied++
		if n.LastApplied < int64(len(n.Log)) {
			entry := n.Log[n.LastApplied]
			// BUG: Not using a separate goroutine/channel
			n.applyEntry(entry)
		}
	}
}

// applyEntry applies a log entry to the state machine
func (n *RaftNode) applyEntry(entry LogEntryRaft) {
	// BUG: No deduplication
	// Apply to state machine
	if cmd, ok := entry.Command.(map[string]interface{}); ok {
		if key, ok := cmd["key"].(string); ok {
			if value, ok := cmd["value"]; ok {
				n.StateMachine[key] = value
			}
		}
	}
}

// ProposeValue proposes a value to the cluster
func (n *RaftNode) ProposeValue(key string, value interface{}) error {
	if atomic.LoadInt32(&n.isLeader) != 1 {
		return fmt.Errorf("not leader")
	}

	n.mu.Lock()

	// Create log entry
	entry := LogEntryRaft{
		Term:    n.CurrentTerm,
		Index:   int64(len(n.Log)),
		Command: map[string]interface{}{
			"key":   key,
			"value": value,
		},
	}

	// BUG: Appending to log before replication
	n.Log = append(n.Log, entry)

	n.mu.Unlock()

	// BUG: Not waiting for replication before returning
	return nil
}

// resetElectionTimer resets the election timeout
func (n *RaftNode) resetElectionTimer() {
	// BUG: Fixed timeout, should be randomized
	if n.electionTimer != nil {
		n.electionTimer.Stop()
	}
	n.electionTimer = time.NewTimer(150 * time.Millisecond)
}

// GetValue reads a value from the state machine
func (n *RaftNode) GetValue(key string) (interface{}, error) {
	// BUG: Not ensuring linearizable reads
	n.mu.RLock()
	defer n.mu.RUnlock()

	value, ok := n.StateMachine[key]
	if !ok {
		return nil, fmt.Errorf("key not found")
	}

	return value, nil
}

// Challenge32ConsensusRaft demonstrates the buggy Raft implementation
func Challenge32ConsensusRaft() {
	fmt.Println("Challenge 32: Raft Consensus Algorithm")
	fmt.Println("======================================")

	// Create a 5-node cluster
	nodes := make([]*RaftNode, 5)
	nodeIDs := []string{"node1", "node2", "node3", "node4", "node5"}

	for i := 0; i < 5; i++ {
		peers := make([]string, 0)
		for j := 0; j < 5; j++ {
			if i != j {
				peers = append(peers, nodeIDs[j])
			}
		}
		nodes[i] = NewRaftNode(nodeIDs[i], peers)
	}

	// Start all nodes
	for _, node := range nodes {
		node.Start()
	}

	// Wait for leader election
	time.Sleep(500 * time.Millisecond)

	// Find the leader
	var leader *RaftNode
	for _, node := range nodes {
		if atomic.LoadInt32(&node.isLeader) == 1 {
			leader = node
			fmt.Printf("Leader elected: %s (term %d)\n", node.ID, node.CurrentTerm)
			break
		}
	}

	if leader == nil {
		fmt.Println("No leader elected!")
		return
	}

	// Propose some values
	fmt.Println("\nProposing values...")
	leader.ProposeValue("counter", 0)
	leader.ProposeValue("name", "raft")
	leader.ProposeValue("counter", 1) // Update

	time.Sleep(200 * time.Millisecond)

	// Check consistency across nodes
	fmt.Println("\nChecking consistency:")
	for _, node := range nodes {
		val, err := node.GetValue("counter")
		if err != nil {
			fmt.Printf("%s: counter = error(%v)\n", node.ID, err)
		} else {
			fmt.Printf("%s: counter = %v\n", node.ID, val)
		}
	}

	// Simulate network partition
	fmt.Println("\nSimulating network partition...")
	// Partition nodes 3,4,5 from nodes 1,2
	// This should prevent consensus

	// Force new election in minority partition
	if atomic.LoadInt32(&nodes[0].isLeader) == 1 {
		nodes[0].State = Follower
		atomic.StoreInt32(&nodes[0].isLeader, 0)
	}

	time.Sleep(300 * time.Millisecond)

	// Check for split-brain
	leaderCount := 0
	for _, node := range nodes {
		if atomic.LoadInt32(&node.isLeader) == 1 {
			leaderCount++
			fmt.Printf("Leader: %s\n", node.ID)
		}
	}

	if leaderCount > 1 {
		fmt.Printf("\n⚠️  SPLIT BRAIN DETECTED: %d leaders!\n", leaderCount)
	}

	// Try to propose in minority partition
	for _, node := range nodes[:2] {
		if atomic.LoadInt32(&node.isLeader) == 1 {
			err := node.ProposeValue("split", "brain")
			fmt.Printf("Minority leader proposal: %v\n", err)
		}
	}

	fmt.Println("\nIssues to fix:")
	fmt.Println("1. Proper leader election with randomized timeouts")
	fmt.Println("2. Log replication with consistency checks")
	fmt.Println("3. Commit index advancement only with majority")
	fmt.Println("4. Handle network partitions correctly")
	fmt.Println("5. Log compaction/snapshots")
	fmt.Println("6. Configuration changes (add/remove nodes)")
	fmt.Println("7. Linearizable reads")
	fmt.Println("8. Log convergence after conflicts")
}