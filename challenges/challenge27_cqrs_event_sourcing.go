package challenges

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge 27: CQRS and Event Sourcing
//
// Problems to fix:
// 1. Event ordering issues causing inconsistent state
// 2. Read model not syncing with write model
// 3. Event replay causing duplicate side effects
// 4. Snapshot corruption during concurrent access
// 5. Command handlers racing with queries
// 6. Event store growing unbounded
// 7. Projection rebuild failures
// 8. Lost events during async processing

type CQRSCommand interface {
	Execute() (CQRSEvent, error)
	Validate() error
}

type CQRSEvent interface {
	GetID() string
	GetAggregateID() string
	GetVersion() int64
	GetTimestamp() time.Time
	GetType() string
	GetData() interface{}
}

type CQRSAggregateRoot struct {
	ID            string
	Version       int64
	Events        []CQRSEvent
	uncommitted   []CQRSEvent
	mu            sync.RWMutex // BUG: Not used consistently
}

type CQRSEventStore struct {
	events     map[string][]CQRSEvent // AggregateID -> Events
	snapshots  map[string]*CQRSSnapshot
	streams    map[string]*CQRSEventStream
	projections map[string]*CQRSProjection
	mu         sync.RWMutex
	eventBus   chan CQRSEvent // BUG: Unbuffered channel
}

type CQRSSnapshot struct {
	AggregateID string
	Version     int64
	Data        interface{}
	Timestamp   time.Time
}

type CQRSEventStream struct {
	ID         string
	Events     []CQRSEvent
	Version    int64
	Subscribers []chan CQRSEvent // BUG: Can cause goroutine leaks
	mu         sync.RWMutex
}

type CQRSProjection struct {
	Name      string
	State     interface{}
	Position  int64 // Event position in stream
	Handlers  map[string]CQRSProjectionHandler
	mu        sync.RWMutex
}

type CQRSProjectionHandler func(event CQRSEvent, state interface{}) interface{}

type CQRSReadModel struct {
	data      map[string]interface{}
	version   int64
	mu        sync.RWMutex
	updateChan chan CQRSEvent // BUG: Can block writers
}

type CQRSCommandHandler struct {
	store      *CQRSEventStore
	aggregates map[string]*CQRSAggregateRoot
	commands   chan CQRSCommand
	mu         sync.RWMutex
}

type CQRSQueryHandler struct {
	readModel  *CQRSReadModel
	cache      map[string]CQRSCacheEntry
	mu         sync.RWMutex
}

type CQRSCacheEntry struct {
	Data      interface{}
	Version   int64
	Timestamp time.Time
	TTL       time.Duration
}

// Example domain events
type AccountCreatedEvent struct {
	ID          string
	AggregateID string
	Version     int64
	Timestamp   time.Time
	AccountName string
	Balance     float64
}

func (e AccountCreatedEvent) GetID() string           { return e.ID }
func (e AccountCreatedEvent) GetAggregateID() string  { return e.AggregateID }
func (e AccountCreatedEvent) GetVersion() int64       { return e.Version }
func (e AccountCreatedEvent) GetTimestamp() time.Time { return e.Timestamp }
func (e AccountCreatedEvent) GetType() string         { return "AccountCreated" }
func (e AccountCreatedEvent) GetData() interface{}    { return e }

type MoneyDepositedEvent struct {
	ID          string
	AggregateID string
	Version     int64
	Timestamp   time.Time
	Amount      float64
}

func (e MoneyDepositedEvent) GetID() string           { return e.ID }
func (e MoneyDepositedEvent) GetAggregateID() string  { return e.AggregateID }
func (e MoneyDepositedEvent) GetVersion() int64       { return e.Version }
func (e MoneyDepositedEvent) GetTimestamp() time.Time { return e.Timestamp }
func (e MoneyDepositedEvent) GetType() string         { return "MoneyDeposited" }
func (e MoneyDepositedEvent) GetData() interface{}    { return e }

func Challenge27CQRSEventSourcing() {
	store := &CQRSEventStore{
		events:      make(map[string][]CQRSEvent),
		snapshots:   make(map[string]*CQRSSnapshot),
		streams:     make(map[string]*CQRSEventStream),
		projections: make(map[string]*CQRSProjection),
		eventBus:    make(chan CQRSEvent), // BUG: Unbuffered
	}

	readModel := &CQRSReadModel{
		data:       make(map[string]interface{}),
		updateChan: make(chan CQRSEvent, 100),
	}

	commandHandler := &CQRSCommandHandler{
		store:      store,
		aggregates: make(map[string]*CQRSAggregateRoot),
		commands:   make(chan CQRSCommand, 10),
	}

	queryHandler := &CQRSQueryHandler{
		readModel: readModel,
		cache:     make(map[string]CQRSCacheEntry),
	}

	// Start event processing
	go store.processEvents()
	go readModel.updateFromEvents()
	go commandHandler.processCommands()

	var wg sync.WaitGroup

	// Create multiple accounts concurrently
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			aggregateID := fmt.Sprintf("account-%d", id)

			// Create account
			event := AccountCreatedEvent{
				ID:          generateID(),
				AggregateID: aggregateID,
				Version:     1, // BUG: Not checking current version
				Timestamp:   time.Now(),
				AccountName: fmt.Sprintf("Account %d", id),
				Balance:     100.0,
			}

			// BUG: Not using command pattern properly
			store.appendEvent(event)

			// Deposit money multiple times
			for j := 0; j < 3; j++ {
				deposit := MoneyDepositedEvent{
					ID:          generateID(),
					AggregateID: aggregateID,
					Version:     int64(j + 2), // BUG: Version calculation wrong
					Timestamp:   time.Now(),
					Amount:      float64(j * 10),
				}
				store.appendEvent(deposit)
			}
		}(i)
	}

	// Query while writing (CQRS violation)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// BUG: Reading from write model instead of read model
			store.mu.RLock()
			events := store.events[fmt.Sprintf("account-%d", id)]
			store.mu.RUnlock()

			// Calculate balance from events (should use read model)
			var balance float64
			for _, e := range events {
				switch evt := e.(type) {
				case AccountCreatedEvent:
					balance = evt.Balance
				case MoneyDepositedEvent:
					balance += evt.Amount
				}
			}

			// BUG: Not updating read model properly
			queryHandler.cache[fmt.Sprintf("balance-%d", id)] = CQRSCacheEntry{
				Data:      balance,
				Version:   int64(len(events)),
				Timestamp: time.Now(),
			}
		}(i)
	}

	// Event replay scenario
	wg.Add(1)
	go func() {
		defer wg.Done()

		// BUG: Replaying events without checking idempotency
		store.replayEvents("account-0", 0)
	}()

	// Snapshot creation with race condition
	wg.Add(1)
	go func() {
		defer wg.Done()

		for i := 0; i < 3; i++ {
			aggregateID := fmt.Sprintf("account-%d", i)

			// BUG: Creating snapshot while events are being added
			snapshot := store.createSnapshot(aggregateID)

			// BUG: Not handling nil snapshot
			store.snapshots[aggregateID] = snapshot
		}
	}()

	// Projection rebuild
	wg.Add(1)
	go func() {
		defer wg.Done()

		projection := &CQRSProjection{
			Name:     "AccountBalance",
			State:    make(map[string]float64),
			Position: 0,
			Handlers: make(map[string]CQRSProjectionHandler),
		}

		// BUG: Projection handlers not thread-safe
		projection.Handlers["AccountCreated"] = func(event CQRSEvent, state interface{}) interface{} {
			s := state.(map[string]float64)
			evt := event.(AccountCreatedEvent)
			s[evt.AggregateID] = evt.Balance // BUG: Direct map access without sync
			return s
		}

		store.projections["balance"] = projection

		// BUG: Rebuilding while events are being added
		store.rebuildProjection("balance")
	}()

	wg.Wait()
}

func (es *CQRSEventStore) appendEvent(event CQRSEvent) error {
	// BUG: Not validating event version
	es.mu.Lock()
	defer es.mu.Unlock()

	aggregateID := event.GetAggregateID()
	es.events[aggregateID] = append(es.events[aggregateID], event)

	// BUG: Can block if no receiver
	es.eventBus <- event

	// Update streams
	if stream, exists := es.streams[aggregateID]; exists {
		// BUG: Not protecting stream access
		stream.Events = append(stream.Events, event)
		stream.Version++

		// BUG: Broadcasting to all subscribers without checking
		for _, sub := range stream.Subscribers {
			sub <- event // BUG: Can block
		}
	}

	return nil
}

func (es *CQRSEventStore) processEvents() {
	for event := range es.eventBus {
		// BUG: Not handling panics in projections
		for _, projection := range es.projections {
			if handler, exists := projection.Handlers[event.GetType()]; exists {
				// BUG: Not protecting projection state
				projection.State = handler(event, projection.State)
				projection.Position++
			}
		}
	}
}

func (es *CQRSEventStore) replayEvents(aggregateID string, fromVersion int64) {
	es.mu.RLock()
	events := es.events[aggregateID]
	es.mu.RUnlock()

	// BUG: Replaying all events, not from specified version
	for _, event := range events {
		// BUG: Side effects will be duplicated
		es.eventBus <- event

		// BUG: Not checking if event was already processed
	}
}

func (es *CQRSEventStore) createSnapshot(aggregateID string) *CQRSSnapshot {
	// BUG: Not locking during snapshot creation
	events := es.events[aggregateID]

	if len(events) == 0 {
		return nil
	}

	// Calculate state from events
	state := make(map[string]interface{})

	// BUG: State calculation not atomic
	for _, event := range events {
		// Apply event to state
		applyEventToState(event, state)
	}

	return &CQRSSnapshot{
		AggregateID: aggregateID,
		Version:     int64(len(events)),
		Data:        state,
		Timestamp:   time.Now(),
	}
}

func (es *CQRSEventStore) rebuildProjection(name string) error {
	projection, exists := es.projections[name]
	if !exists {
		return errors.New("projection not found")
	}

	// BUG: Not resetting projection state
	// projection.State = make(map[string]interface{})
	// projection.Position = 0

	// BUG: Iterating all events without ordering
	for _, events := range es.events {
		for _, event := range events {
			if handler, exists := projection.Handlers[event.GetType()]; exists {
				// BUG: Concurrent modification of state
				projection.State = handler(event, projection.State)
			}
		}
	}

	return nil
}

func (rm *CQRSReadModel) updateFromEvents() {
	for event := range rm.updateChan {
		// BUG: Not checking version for optimistic locking
		rm.mu.Lock()

		// Apply event to read model
		switch e := event.(type) {
		case AccountCreatedEvent:
			rm.data[e.AggregateID] = map[string]interface{}{
				"name":    e.AccountName,
				"balance": e.Balance,
			}
		case MoneyDepositedEvent:
			// BUG: Not checking if account exists
			account := rm.data[e.AggregateID].(map[string]interface{})
			account["balance"] = account["balance"].(float64) + e.Amount
		}

		atomic.AddInt64(&rm.version, 1)
		rm.mu.Unlock()
	}
}

func (ch *CQRSCommandHandler) processCommands() {
	for cmd := range ch.commands {
		// BUG: Not validating command
		// cmd.Validate()

		// BUG: Not using optimistic concurrency control
		event, err := cmd.Execute()
		if err != nil {
			continue
		}

		// BUG: Not checking aggregate version
		ch.store.appendEvent(event)
	}
}

func applyEventToState(event CQRSEvent, state map[string]interface{}) {
	// BUG: Type assertions without checking
	switch e := event.(type) {
	case AccountCreatedEvent:
		state["balance"] = e.Balance
		state["name"] = e.AccountName
	case MoneyDepositedEvent:
		state["balance"] = state["balance"].(float64) + e.Amount
	}
}

// Missing implementations:
// 1. Proper event versioning and ordering
// 2. Idempotent event processing
// 3. Optimistic concurrency control
// 4. Event store compaction
// 5. Saga orchestration
// 6. Command validation pipeline
// 7. Read model consistency guarantees
// 8. Event sourcing snapshots with versioning