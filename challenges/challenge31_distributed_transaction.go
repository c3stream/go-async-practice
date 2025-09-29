// Challenge 31: Distributed Transaction Coordinator
//
// Problem: The distributed transaction coordinator has several critical bugs:
// 1. Two-phase commit doesn't handle participant failures properly
// 2. Transaction logs are not persisted (crash recovery fails)
// 3. Deadlocks can occur in the lock manager
// 4. Isolation levels are not enforced correctly
// 5. Phantom reads are possible
// 6. Lost updates can occur
// 7. Cascading aborts are not handled
// 8. Network partitions cause split-brain
//
// Your task: Fix the distributed transaction coordinator to ensure ACID properties.

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

// TransactionCoordinator manages distributed transactions
type TransactionCoordinator struct {
	participants map[string]*Participant
	txnLog      *TransactionLog
	lockManager *LockManager
	txnCounter  uint64
	activeTxns  map[uint64]*Transaction
	mu          sync.RWMutex
}

// Transaction represents a distributed transaction
type Transaction struct {
	ID            uint64
	Coordinator   *TransactionCoordinator
	Participants  map[string]*Participant
	State         TxnState
	IsolationLevel IsolationLevel
	StartTime     time.Time
	Timeout       time.Duration
	Operations    []Operation
	Locks         []Lock
	WriteSet      map[string]interface{} // BUG: Not properly tracked
	ReadSet       map[string]interface{} // BUG: Not used for validation
	mu            sync.RWMutex
}

// TxnState represents transaction state
type TxnState int

const (
	TxnInit TxnState = iota
	TxnActive
	TxnPreparing
	TxnPrepared
	TxnCommitting
	TxnCommitted
	TxnAborting
	TxnAborted
)

// IsolationLevel represents transaction isolation level
type IsolationLevel int

const (
	ReadUncommitted IsolationLevel = iota
	ReadCommitted
	RepeatableRead
	Serializable
)

// Participant represents a transaction participant
type Participant struct {
	ID       string
	Endpoint string
	State    ParticipantState
	Data     map[string]interface{}
	TxnData  map[uint64]map[string]interface{} // Transaction-specific data
	Log      []LogEntry
	mu       sync.RWMutex
	available int32 // 1 if available, 0 if failed
}

// ParticipantState represents participant's state in 2PC
type ParticipantState int

const (
	ParticipantInit ParticipantState = iota
	ParticipantReady
	ParticipantPrepared
	ParticipantCommitted
	ParticipantAborted
)

// Operation represents a transaction operation
type Operation struct {
	Type      OpType
	Key       string
	Value     interface{}
	Timestamp time.Time
}

// OpType represents operation type
type OpType int

const (
	OpRead OpType = iota
	OpWrite
	OpDelete
)

// Lock represents a lock on a resource
type Lock struct {
	TxnID     uint64
	Resource  string
	Type      LockType
	Timestamp time.Time
}

// LockType represents the type of lock
type LockType int

const (
	SharedLock LockType = iota
	ExclusiveLock
)

// LockManager manages locks for transactions
type LockManager struct {
	locks     map[string][]Lock
	waitGraph map[uint64][]uint64 // For deadlock detection
	mu        sync.RWMutex
}

// TransactionLog persists transaction state
type TransactionLog struct {
	entries []LogEntry
	mu      sync.RWMutex
	// BUG: Not persisted to disk
}

// LogEntry represents a log entry
type LogEntry struct {
	TxnID     uint64
	Type      LogType
	Data      interface{}
	Timestamp time.Time
}

// LogType represents the type of log entry
type LogType int

const (
	LogBegin LogType = iota
	LogPrepare
	LogCommit
	LogAbort
	LogCheckpoint
)

// NewTransactionCoordinator creates a new coordinator
func NewTransactionCoordinator() *TransactionCoordinator {
	return &TransactionCoordinator{
		participants: make(map[string]*Participant),
		txnLog:      &TransactionLog{},
		lockManager: &LockManager{
			locks:     make(map[string][]Lock),
			waitGraph: make(map[uint64][]uint64),
		},
		activeTxns: make(map[uint64]*Transaction),
	}
}

// AddParticipant adds a participant to the coordinator
func (tc *TransactionCoordinator) AddParticipant(id string, endpoint string) {
	participant := &Participant{
		ID:       id,
		Endpoint: endpoint,
		State:    ParticipantInit,
		Data:     make(map[string]interface{}),
		TxnData:  make(map[uint64]map[string]interface{}),
		available: 1,
	}

	tc.mu.Lock()
	tc.participants[id] = participant
	tc.mu.Unlock()
}

// BeginTransaction starts a new distributed transaction
func (tc *TransactionCoordinator) BeginTransaction(isolation IsolationLevel) *Transaction {
	txnID := atomic.AddUint64(&tc.txnCounter, 1)

	txn := &Transaction{
		ID:             txnID,
		Coordinator:    tc,
		Participants:   make(map[string]*Participant),
		State:          TxnActive,
		IsolationLevel: isolation,
		StartTime:      time.Now(),
		Timeout:        30 * time.Second,
		WriteSet:       make(map[string]interface{}),
		ReadSet:        make(map[string]interface{}),
	}

	tc.mu.Lock()
	tc.activeTxns[txnID] = txn
	tc.mu.Unlock()

	// BUG: Not logging transaction begin
	return txn
}

// Read performs a distributed read
func (txn *Transaction) Read(key string) (interface{}, error) {
	// BUG: No isolation level enforcement
	txn.mu.Lock()
	defer txn.mu.Unlock()

	// Check write set first
	if val, ok := txn.WriteSet[key]; ok {
		return val, nil
	}

	// BUG: No lock acquisition for reads in REPEATABLE READ/SERIALIZABLE
	// Try to read from any available participant
	for _, participant := range txn.Coordinator.participants {
		if atomic.LoadInt32(&participant.available) == 0 {
			continue
		}

		participant.mu.RLock()
		val, ok := participant.Data[key]
		participant.mu.RUnlock()

		if ok {
			// BUG: Not adding to read set
			return val, nil
		}
	}

	return nil, fmt.Errorf("key not found: %s", key)
}

// Write performs a distributed write
func (txn *Transaction) Write(key string, value interface{}) error {
	txn.mu.Lock()
	defer txn.mu.Unlock()

	if txn.State != TxnActive {
		return fmt.Errorf("transaction not active")
	}

	// BUG: No lock acquisition
	txn.WriteSet[key] = value

	op := Operation{
		Type:      OpWrite,
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}
	txn.Operations = append(txn.Operations, op)

	return nil
}

// Commit commits the transaction using 2PC
func (txn *Transaction) Commit() error {
	// Phase 1: Prepare
	if err := txn.prepare(); err != nil {
		// BUG: Not properly aborting on prepare failure
		return err
	}

	// Phase 2: Commit
	if err := txn.doCommit(); err != nil {
		// BUG: Partial commits possible
		return err
	}

	return nil
}

// prepare executes the prepare phase of 2PC
func (txn *Transaction) prepare() error {
	txn.mu.Lock()
	txn.State = TxnPreparing
	txn.mu.Unlock()

	// BUG: Not logging prepare
	// Send prepare to all participants
	var wg sync.WaitGroup
	errorChan := make(chan error, len(txn.Coordinator.participants))

	for _, participant := range txn.Coordinator.participants {
		if atomic.LoadInt32(&participant.available) == 0 {
			// BUG: Skipping unavailable participants
			continue
		}

		wg.Add(1)
		go func(p *Participant) {
			defer wg.Done()

			// Simulate prepare
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

			// BUG: Random failures not handled properly
			if rand.Float32() < 0.1 {
				errorChan <- fmt.Errorf("participant %s failed to prepare", p.ID)
				return
			}

			p.mu.Lock()
			p.State = ParticipantPrepared
			// BUG: Not persisting prepare state
			p.mu.Unlock()
		}(participant)
	}

	wg.Wait()
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		if err != nil {
			// BUG: Not sending abort to prepared participants
			return err
		}
	}

	txn.mu.Lock()
	txn.State = TxnPrepared
	txn.mu.Unlock()

	return nil
}

// doCommit executes the commit phase of 2PC
func (txn *Transaction) doCommit() error {
	txn.mu.Lock()
	txn.State = TxnCommitting
	txn.mu.Unlock()

	// BUG: Not logging commit decision
	// Send commit to all participants
	var wg sync.WaitGroup

	for _, participant := range txn.Coordinator.participants {
		// BUG: Committing even to non-prepared participants
		wg.Add(1)
		go func(p *Participant) {
			defer wg.Done()

			// Apply write set
			p.mu.Lock()
			for key, value := range txn.WriteSet {
				// BUG: No version checking
				p.Data[key] = value
			}
			p.State = ParticipantCommitted
			p.mu.Unlock()
		}(participant)
	}

	wg.Wait()

	txn.mu.Lock()
	txn.State = TxnCommitted
	txn.mu.Unlock()

	// BUG: Not releasing locks
	// BUG: Not removing from active transactions

	return nil
}

// AcquireLock acquires a lock for the transaction
func (lm *LockManager) AcquireLock(txnID uint64, resource string, lockType LockType) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	existingLocks := lm.locks[resource]

	// BUG: Deadlock detection not implemented
	for _, lock := range existingLocks {
		if lock.Type == ExclusiveLock || lockType == ExclusiveLock {
			if lock.TxnID != txnID {
				// BUG: Immediate failure instead of waiting
				return fmt.Errorf("lock conflict on %s", resource)
			}
		}
	}

	// Add lock
	newLock := Lock{
		TxnID:     txnID,
		Resource:  resource,
		Type:      lockType,
		Timestamp: time.Now(),
	}

	lm.locks[resource] = append(lm.locks[resource], newLock)

	// BUG: Not updating wait graph for deadlock detection
	return nil
}

// ReleaseLocks releases all locks held by a transaction
func (lm *LockManager) ReleaseLocks(txnID uint64) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// BUG: Inefficient lock release
	for resource, locks := range lm.locks {
		newLocks := make([]Lock, 0)
		for _, lock := range locks {
			if lock.TxnID != txnID {
				newLocks = append(newLocks, lock)
			}
		}
		lm.locks[resource] = newLocks
	}

	// BUG: Not waking up waiting transactions
}

// SimulatePartition simulates a network partition
func (tc *TransactionCoordinator) SimulatePartition(participantID string) {
	tc.mu.RLock()
	participant, ok := tc.participants[participantID]
	tc.mu.RUnlock()

	if ok {
		atomic.StoreInt32(&participant.available, 0)
		log.Printf("Participant %s partitioned", participantID)
	}
}

// Challenge31DistributedTransaction demonstrates the buggy distributed transaction coordinator
func Challenge31DistributedTransaction() {
	fmt.Println("Challenge 31: Distributed Transaction Coordinator")
	fmt.Println("=================================================")

	coordinator := NewTransactionCoordinator()

	// Add participants
	coordinator.AddParticipant("node1", "localhost:8001")
	coordinator.AddParticipant("node2", "localhost:8002")
	coordinator.AddParticipant("node3", "localhost:8003")

	// Initialize some data
	for _, p := range coordinator.participants {
		p.Data["balance_alice"] = 1000
		p.Data["balance_bob"] = 1000
	}

	// Test concurrent transactions
	var wg sync.WaitGroup

	// Transaction 1: Transfer from Alice to Bob
	wg.Add(1)
	go func() {
		defer wg.Done()

		txn := coordinator.BeginTransaction(ReadCommitted)

		// Read Alice's balance
		aliceBalance, err := txn.Read("balance_alice")
		if err != nil {
			log.Printf("Txn1: Read failed: %v", err)
			return
		}

		// Deduct from Alice
		newAliceBalance := aliceBalance.(int) - 100
		txn.Write("balance_alice", newAliceBalance)

		// Read Bob's balance
		bobBalance, _ := txn.Read("balance_bob")

		// Add to Bob
		newBobBalance := bobBalance.(int) + 100
		txn.Write("balance_bob", newBobBalance)

		// Commit
		if err := txn.Commit(); err != nil {
			log.Printf("Txn1: Commit failed: %v", err)
		} else {
			log.Println("Txn1: Committed successfully")
		}
	}()

	// Transaction 2: Conflicting transfer
	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(10 * time.Millisecond) // Small delay

		txn := coordinator.BeginTransaction(RepeatableRead)

		// Read Alice's balance
		aliceBalance, err := txn.Read("balance_alice")
		if err != nil {
			log.Printf("Txn2: Read failed: %v", err)
			return
		}

		// Deduct from Alice
		newAliceBalance := aliceBalance.(int) - 50
		txn.Write("balance_alice", newAliceBalance)

		// Commit
		if err := txn.Commit(); err != nil {
			log.Printf("Txn2: Commit failed: %v", err)
		} else {
			log.Println("Txn2: Committed successfully")
		}
	}()

	// Simulate network partition during transaction
	go func() {
		time.Sleep(30 * time.Millisecond)
		coordinator.SimulatePartition("node2")
	}()

	wg.Wait()

	// Check final balances
	fmt.Println("\nFinal balances:")
	for id, p := range coordinator.participants {
		if atomic.LoadInt32(&p.available) == 1 {
			p.mu.RLock()
			alice := p.Data["balance_alice"]
			bob := p.Data["balance_bob"]
			p.mu.RUnlock()
			fmt.Printf("%s: Alice=%v, Bob=%v\n", id, alice, bob)
		}
	}

	// Test deadlock scenario
	fmt.Println("\n Testing deadlock scenario:")

	txn3 := coordinator.BeginTransaction(Serializable)
	txn4 := coordinator.BeginTransaction(Serializable)

	// Create potential deadlock
	coordinator.lockManager.AcquireLock(txn3.ID, "resource_A", ExclusiveLock)
	coordinator.lockManager.AcquireLock(txn4.ID, "resource_B", ExclusiveLock)

	// Try to acquire conflicting locks
	err1 := coordinator.lockManager.AcquireLock(txn3.ID, "resource_B", ExclusiveLock)
	err2 := coordinator.lockManager.AcquireLock(txn4.ID, "resource_A", ExclusiveLock)

	if err1 != nil || err2 != nil {
		fmt.Println("Deadlock detected (or should be)")
	}

	fmt.Println("\nIssues to fix:")
	fmt.Println("1. Proper 2PC with failure handling")
	fmt.Println("2. Transaction log persistence")
	fmt.Println("3. Deadlock detection and resolution")
	fmt.Println("4. Isolation level enforcement")
	fmt.Println("5. Prevent phantom reads")
	fmt.Println("6. Prevent lost updates")
	fmt.Println("7. Handle cascading aborts")
	fmt.Println("8. Handle network partitions correctly")
}