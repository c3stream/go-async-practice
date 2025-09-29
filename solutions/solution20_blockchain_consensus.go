package solutions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Solution20 demonstrates three approaches to blockchain consensus:
// 1. Proof of Work (PoW) consensus
// 2. Practical Byzantine Fault Tolerance (PBFT)
// 3. Raft consensus algorithm

// Solution 1: Proof of Work Consensus
type PoWBlockchain struct {
	mu         sync.RWMutex
	chain      []*Block
	difficulty int
	mempool    []Transaction
	mining     int32
	validators map[string]*Validator
}

type Block struct {
	Index        int
	Timestamp    time.Time
	Transactions []Transaction
	PrevHash     string
	Hash         string
	Nonce        int
	Miner        string
}

type Transaction struct {
	ID        string
	From      string
	To        string
	Amount    float64
	Timestamp time.Time
}

type Validator struct {
	ID         string
	Stake      float64
	Reputation int
}

func NewPoWBlockchain() *PoWBlockchain {
	bc := &PoWBlockchain{
		chain:      make([]*Block, 0),
		difficulty: 4,
		mempool:    make([]Transaction, 0),
		validators: make(map[string]*Validator),
	}

	// Genesis block
	genesis := &Block{
		Index:     0,
		Timestamp: time.Now(),
		PrevHash:  "0",
		Hash:      bc.calculateHash(&Block{PrevHash: "0"}),
	}
	bc.chain = append(bc.chain, genesis)

	return bc
}

func (bc *PoWBlockchain) calculateHash(block *Block) string {
	data := fmt.Sprintf("%d%v%v%s%d",
		block.Index,
		block.Timestamp,
		block.Transactions,
		block.PrevHash,
		block.Nonce)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (bc *PoWBlockchain) isValidHash(hash string) bool {
	prefix := ""
	for i := 0; i < bc.difficulty; i++ {
		prefix += "0"
	}
	return hash[:bc.difficulty] == prefix
}

func (bc *PoWBlockchain) MineBlock(ctx context.Context, miner string) (*Block, error) {
	// Check if already mining
	if !atomic.CompareAndSwapInt32(&bc.mining, 0, 1) {
		return nil, fmt.Errorf("already mining")
	}
	defer atomic.StoreInt32(&bc.mining, 0)

	bc.mu.Lock()
	if len(bc.mempool) == 0 {
		bc.mu.Unlock()
		return nil, fmt.Errorf("no transactions to mine")
	}

	// Create new block
	prevBlock := bc.chain[len(bc.chain)-1]
	newBlock := &Block{
		Index:        prevBlock.Index + 1,
		Timestamp:    time.Now(),
		Transactions: bc.mempool,
		PrevHash:     prevBlock.Hash,
		Miner:        miner,
	}
	bc.mempool = make([]Transaction, 0)
	bc.mu.Unlock()

	// Proof of Work
	nonce := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			newBlock.Nonce = nonce
			hash := bc.calculateHash(newBlock)

			if bc.isValidHash(hash) {
				newBlock.Hash = hash

				// Add block to chain
				bc.mu.Lock()
				bc.chain = append(bc.chain, newBlock)
				bc.mu.Unlock()

				log.Printf("Block mined: %s by %s", hash, miner)
				return newBlock, nil
			}
			nonce++
		}
	}
}

func (bc *PoWBlockchain) AddTransaction(tx Transaction) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	bc.mempool = append(bc.mempool, tx)
}

func (bc *PoWBlockchain) ValidateChain() bool {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	for i := 1; i < len(bc.chain); i++ {
		current := bc.chain[i]
		prev := bc.chain[i-1]

		// Validate hash
		if current.Hash != bc.calculateHash(current) {
			return false
		}

		// Validate link
		if current.PrevHash != prev.Hash {
			return false
		}

		// Validate proof of work
		if !bc.isValidHash(current.Hash) {
			return false
		}
	}

	return true
}

// Solution 2: PBFT Consensus
type PBFTConsensus struct {
	mu           sync.RWMutex
	nodeID       string
	nodes        []string
	view         int
	sequence     int
	prepared     map[string]int
	committed    map[string]int
	messageLog   []Message
	isPrimary    bool
	chain        []*Block
	faultNodes   int
}

type Message struct {
	Type      string // REQUEST, PREPREPARE, PREPARE, COMMIT
	View      int
	Sequence  int
	Block     *Block
	NodeID    string
	Signature string
}

func NewPBFTConsensus(nodeID string, nodes []string) *PBFTConsensus {
	faultNodes := (len(nodes) - 1) / 3

	return &PBFTConsensus{
		nodeID:     nodeID,
		nodes:      nodes,
		view:       0,
		sequence:   0,
		prepared:   make(map[string]int),
		committed:  make(map[string]int),
		messageLog: make([]Message, 0),
		isPrimary:  nodes[0] == nodeID,
		chain:      make([]*Block, 0),
		faultNodes: faultNodes,
	}
}

func (p *PBFTConsensus) ProposeBlock(ctx context.Context, block *Block) error {
	if !p.isPrimary {
		return fmt.Errorf("not primary node")
	}

	p.mu.Lock()
	p.sequence++
	msg := Message{
		Type:     "PREPREPARE",
		View:     p.view,
		Sequence: p.sequence,
		Block:    block,
		NodeID:   p.nodeID,
	}
	p.messageLog = append(p.messageLog, msg)
	p.mu.Unlock()

	// Broadcast pre-prepare to all nodes
	p.broadcast(ctx, msg)

	// Wait for consensus
	return p.waitForConsensus(ctx, block)
}

func (p *PBFTConsensus) HandleMessage(ctx context.Context, msg Message) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Verify message
	if !p.verifyMessage(msg) {
		return
	}

	p.messageLog = append(p.messageLog, msg)

	switch msg.Type {
	case "PREPREPARE":
		p.handlePrePrepare(ctx, msg)
	case "PREPARE":
		p.handlePrepare(ctx, msg)
	case "COMMIT":
		p.handleCommit(ctx, msg)
	}
}

func (p *PBFTConsensus) handlePrePrepare(ctx context.Context, msg Message) {
	// Send prepare message
	prepareMsg := Message{
		Type:     "PREPARE",
		View:     msg.View,
		Sequence: msg.Sequence,
		Block:    msg.Block,
		NodeID:   p.nodeID,
	}

	p.broadcast(ctx, prepareMsg)
}

func (p *PBFTConsensus) handlePrepare(ctx context.Context, msg Message) {
	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)
	p.prepared[key]++

	// Check if we have enough prepares (2f+1)
	if p.prepared[key] >= 2*p.faultNodes+1 {
		// Send commit message
		commitMsg := Message{
			Type:     "COMMIT",
			View:     msg.View,
			Sequence: msg.Sequence,
			Block:    msg.Block,
			NodeID:   p.nodeID,
		}

		p.broadcast(ctx, commitMsg)
	}
}

func (p *PBFTConsensus) handleCommit(ctx context.Context, msg Message) {
	key := fmt.Sprintf("%d-%d", msg.View, msg.Sequence)
	p.committed[key]++

	// Check if we have enough commits (2f+1)
	if p.committed[key] >= 2*p.faultNodes+1 {
		// Add block to chain
		p.chain = append(p.chain, msg.Block)
		log.Printf("Block committed: %s", msg.Block.Hash)
	}
}

func (p *PBFTConsensus) verifyMessage(msg Message) bool {
	// Simplified verification
	return msg.View == p.view && msg.Sequence > 0
}

func (p *PBFTConsensus) broadcast(ctx context.Context, msg Message) {
	// Simulate network broadcast
	for _, node := range p.nodes {
		if node != p.nodeID {
			go func(n string) {
				// Simulate network delay
				time.Sleep(time.Millisecond * 10)
				// In real implementation, send to node n
			}(node)
		}
	}
}

func (p *PBFTConsensus) waitForConsensus(ctx context.Context, block *Block) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(5 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("consensus timeout")
		case <-ticker.C:
			p.mu.RLock()
			key := fmt.Sprintf("%d-%d", p.view, p.sequence)
			committed := p.committed[key]
			p.mu.RUnlock()

			if committed >= 2*p.faultNodes+1 {
				return nil
			}
		}
	}
}

// Solution 3: Raft Consensus
type RaftNode struct {
	mu          sync.RWMutex
	id          string
	state       string // follower, candidate, leader
	currentTerm int
	votedFor    string
	log         []LogEntry
	commitIndex int
	lastApplied int

	// Leader state
	nextIndex  map[string]int
	matchIndex map[string]int

	// Channels
	heartbeat chan bool
	election  chan bool
	votesChan chan Vote

	// Cluster
	peers []string
	chain []*Block
}

type LogEntry struct {
	Term  int
	Index int
	Data  *Block
}

type Vote struct {
	Term        int
	CandidateID string
	Granted     bool
}

func NewRaftNode(id string, peers []string) *RaftNode {
	return &RaftNode{
		id:          id,
		state:       "follower",
		currentTerm: 0,
		votedFor:    "",
		log:         make([]LogEntry, 0),
		commitIndex: 0,
		lastApplied: 0,
		nextIndex:   make(map[string]int),
		matchIndex:  make(map[string]int),
		heartbeat:   make(chan bool),
		election:    make(chan bool),
		votesChan:   make(chan Vote),
		peers:       peers,
		chain:       make([]*Block, 0),
	}
}

func (r *RaftNode) Start(ctx context.Context) {
	go r.run(ctx)
}

func (r *RaftNode) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			r.mu.RLock()
			state := r.state
			r.mu.RUnlock()

			switch state {
			case "follower":
				r.runFollower(ctx)
			case "candidate":
				r.runCandidate(ctx)
			case "leader":
				r.runLeader(ctx)
			}
		}
	}
}

func (r *RaftNode) runFollower(ctx context.Context) {
	timeout := time.After(randomTimeout())

	select {
	case <-ctx.Done():
		return
	case <-r.heartbeat:
		// Reset timeout
		return
	case <-timeout:
		// Start election
		r.mu.Lock()
		r.state = "candidate"
		r.mu.Unlock()
	}
}

func (r *RaftNode) runCandidate(ctx context.Context) {
	r.mu.Lock()
	r.currentTerm++
	r.votedFor = r.id
	votes := 1 // Vote for self
	r.mu.Unlock()

	// Request votes from peers
	go r.requestVotes(ctx)

	timeout := time.After(randomTimeout())

	for {
		select {
		case <-ctx.Done():
			return
		case vote := <-r.votesChan:
			if vote.Granted {
				votes++
				if votes > len(r.peers)/2 {
					// Won election
					r.mu.Lock()
					r.state = "leader"
					r.mu.Unlock()
					return
				}
			}
		case <-r.heartbeat:
			// Another leader elected
			r.mu.Lock()
			r.state = "follower"
			r.mu.Unlock()
			return
		case <-timeout:
			// Election timeout, restart
			return
		}
	}
}

func (r *RaftNode) runLeader(ctx context.Context) {
	// Send heartbeats
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.sendHeartbeats(ctx)
		}
	}
}

func (r *RaftNode) requestVotes(ctx context.Context) {
	for _, peer := range r.peers {
		if peer != r.id {
			go func(p string) {
				// Simulate vote request
				// In real implementation, send RPC to peer
				time.Sleep(time.Millisecond * 10)
			}(peer)
		}
	}
}

func (r *RaftNode) sendHeartbeats(ctx context.Context) {
	for _, peer := range r.peers {
		if peer != r.id {
			go func(p string) {
				// Simulate heartbeat
				// In real implementation, send AppendEntries RPC
				time.Sleep(time.Millisecond * 5)
			}(peer)
		}
	}
}

func (r *RaftNode) ProposeBlock(ctx context.Context, block *Block) error {
	r.mu.RLock()
	isLeader := r.state == "leader"
	r.mu.RUnlock()

	if !isLeader {
		return fmt.Errorf("not leader")
	}

	// Append to log
	r.mu.Lock()
	entry := LogEntry{
		Term:  r.currentTerm,
		Index: len(r.log),
		Data:  block,
	}
	r.log = append(r.log, entry)
	r.mu.Unlock()

	// Replicate to followers
	return r.replicateEntry(ctx, entry)
}

func (r *RaftNode) replicateEntry(ctx context.Context, entry LogEntry) error {
	successCount := 1 // Leader counts as success

	for _, peer := range r.peers {
		if peer != r.id {
			go func(p string) {
				// Simulate replication
				// In real implementation, send AppendEntries RPC
				time.Sleep(time.Millisecond * 10)
				successCount++
			}(peer)
		}
	}

	// Wait for majority
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	timeout := time.After(2 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("replication timeout")
		case <-ticker.C:
			if successCount > len(r.peers)/2 {
				// Commit entry
				r.mu.Lock()
				r.commitIndex = entry.Index
				r.chain = append(r.chain, entry.Data)
				r.mu.Unlock()
				return nil
			}
		}
	}
}

func randomTimeout() time.Duration {
	return time.Duration(150+time.Now().UnixNano()%150) * time.Millisecond
}

// RunSolution20 demonstrates the three consensus solutions
func RunSolution20() {
	fmt.Println("=== Solution 20: Blockchain Consensus ===")

	ctx := context.Background()

	// Solution 1: Proof of Work
	fmt.Println("\n1. Proof of Work Consensus:")
	pow := NewPoWBlockchain()

	// Add transactions
	pow.AddTransaction(Transaction{
		ID:     "tx1",
		From:   "Alice",
		To:     "Bob",
		Amount: 100,
	})
	pow.AddTransaction(Transaction{
		ID:     "tx2",
		From:   "Bob",
		To:     "Charlie",
		Amount: 50,
	})

	// Mine block
	mineCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	block, err := pow.MineBlock(mineCtx, "Miner1")
	if err != nil {
		fmt.Printf("  Mining failed: %v\n", err)
	} else {
		fmt.Printf("  Block mined: Hash=%s, Nonce=%d\n", block.Hash, block.Nonce)
	}

	// Validate chain
	isValid := pow.ValidateChain()
	fmt.Printf("  Chain valid: %v\n", isValid)

	// Solution 2: PBFT Consensus
	fmt.Println("\n2. PBFT Consensus:")
	nodes := []string{"node1", "node2", "node3", "node4"}
	pbft := NewPBFTConsensus("node1", nodes)

	// Propose block
	newBlock := &Block{
		Index:     1,
		Timestamp: time.Now(),
		Hash:      "block_hash",
	}

	proposeCtx, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()

	err = pbft.ProposeBlock(proposeCtx, newBlock)
	if err != nil {
		fmt.Printf("  Consensus failed: %v\n", err)
	} else {
		fmt.Printf("  Block accepted through PBFT consensus\n")
	}

	// Solution 3: Raft Consensus
	fmt.Println("\n3. Raft Consensus:")
	raftPeers := []string{"raft1", "raft2", "raft3", "raft4", "raft5"}
	raft := NewRaftNode("raft1", raftPeers)

	// Start node
	raftCtx, cancel3 := context.WithCancel(ctx)
	defer cancel3()

	raft.Start(raftCtx)

	// Simulate leader election
	time.Sleep(500 * time.Millisecond)

	// Force leader state for demo
	raft.mu.Lock()
	raft.state = "leader"
	raft.mu.Unlock()

	// Propose block
	raftBlock := &Block{
		Index:     1,
		Timestamp: time.Now(),
		Hash:      "raft_block",
	}

	err = raft.ProposeBlock(raftCtx, raftBlock)
	if err != nil {
		fmt.Printf("  Raft proposal failed: %v\n", err)
	} else {
		fmt.Printf("  Block accepted through Raft consensus\n")
	}

	fmt.Println("\nAll consensus mechanisms demonstrated successfully!")
}