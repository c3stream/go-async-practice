// Challenge 29: Actor Model Implementation
//
// Problem: The current actor model implementation has several critical issues:
// 1. Mailbox overflow causing message loss
// 2. No supervision tree for fault tolerance
// 3. Actors can't handle panics properly
// 4. No message prioritization
// 5. Deadletter messages are not handled
// 6. Location transparency is not implemented
//
// Your task: Fix the actor model implementation to handle these issues properly.

package challenges

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// ActorSystem manages all actors
type ActorSystem struct {
	actors     map[string]*Actor
	supervisor *Supervisor
	deadletter chan Message
	mu         sync.RWMutex
	shutdown   int32
}

// Actor represents an actor in the system
type Actor struct {
	ID          string
	mailbox     chan Message
	behavior    ActorBehavior
	children    map[string]*Actor
	parent      *Actor
	state       interface{}
	ctx         context.Context
	cancel      context.CancelFunc
	restarts    int32
	maxRestarts int32
	mu          sync.RWMutex
}

// Message represents a message sent between actors
type Message struct {
	From     string
	To       string
	Type     MessageType
	Payload  interface{}
	Priority int // BUG: Priority is not being used
	Timeout  time.Duration
}

// MessageType defines the type of message
type MessageType int

const (
	MsgRequest MessageType = iota
	MsgResponse
	MsgError
	MsgPing
	MsgPong
	MsgPoison // Shutdown message
)

// ActorBehavior defines how an actor processes messages
type ActorBehavior func(ctx context.Context, self *Actor, msg Message) error

// Supervisor manages actor lifecycle and fault tolerance
type Supervisor struct {
	strategy SupervisionStrategy
	actors   map[string]*Actor
	mu       sync.RWMutex
}

// SupervisionStrategy defines how to handle actor failures
type SupervisionStrategy int

const (
	OneForOne SupervisionStrategy = iota // Restart only the failed actor
	OneForAll                            // Restart all actors
	RestForOne                          // Restart failed actor and all created after it
)

// NewActorSystem creates a new actor system
func NewActorSystem() *ActorSystem {
	return &ActorSystem{
		actors:     make(map[string]*Actor),
		deadletter: make(chan Message, 100), // BUG: Fixed size, can overflow
		supervisor: &Supervisor{
			strategy: OneForOne,
			actors:   make(map[string]*Actor),
		},
	}
}

// SpawnActor creates a new actor
func (as *ActorSystem) SpawnActor(id string, behavior ActorBehavior) *Actor {
	// BUG: No check for duplicate IDs
	ctx, cancel := context.WithCancel(context.Background())

	actor := &Actor{
		ID:          id,
		mailbox:     make(chan Message, 10), // BUG: Small mailbox, no overflow handling
		behavior:    behavior,
		children:    make(map[string]*Actor),
		ctx:         ctx,
		cancel:      cancel,
		maxRestarts: 3,
	}

	as.mu.Lock()
	as.actors[id] = actor
	as.mu.Unlock()

	// BUG: Actor goroutine doesn't handle panics
	go actor.run()

	return actor
}

// run is the main actor loop
func (a *Actor) run() {
	for {
		select {
		case <-a.ctx.Done():
			return

		case msg := <-a.mailbox:
			// BUG: No timeout handling for message processing
			err := a.behavior(a.ctx, a, msg)
			if err != nil {
				// BUG: Error is logged but not handled properly
				log.Printf("Actor %s error: %v", a.ID, err)
			}
		}
	}
}

// Send sends a message to an actor
func (a *Actor) Send(msg Message) error {
	// BUG: No handling for full mailbox
	select {
	case a.mailbox <- msg:
		return nil
	default:
		// Message lost!
		return fmt.Errorf("mailbox full")
	}
}

// Ask sends a message and waits for response
func (a *Actor) Ask(target *Actor, msg Message, timeout time.Duration) (interface{}, error) {
	// BUG: No proper request-response correlation
	responseChan := make(chan Message, 1)

	// Send request
	err := target.Send(msg)
	if err != nil {
		return nil, err
	}

	// Wait for response
	select {
	case resp := <-responseChan: // BUG: Nothing writes to this channel
		return resp.Payload, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("ask timeout")
	}
}

// Supervise handles actor failures
func (s *Supervisor) Supervise(actor *Actor) {
	// BUG: No actual supervision implementation
	s.mu.Lock()
	s.actors[actor.ID] = actor
	s.mu.Unlock()
}

// RestartActor restarts a failed actor
func (s *Supervisor) RestartActor(actor *Actor) error {
	// BUG: No proper restart logic
	atomic.AddInt32(&actor.restarts, 1)

	if atomic.LoadInt32(&actor.restarts) > actor.maxRestarts {
		// BUG: Actor is not properly removed from system
		return fmt.Errorf("max restarts exceeded")
	}

	// BUG: Old goroutine is not properly stopped
	go actor.run()

	return nil
}

// HandleDeadletter processes dead letters
func (as *ActorSystem) HandleDeadletter() {
	// BUG: Deadletter handler is never started
	for msg := range as.deadletter {
		log.Printf("Deadletter: %+v", msg)
		// BUG: No actual handling, just logging
	}
}

// Shutdown gracefully shuts down the actor system
func (as *ActorSystem) Shutdown(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&as.shutdown, 0, 1) {
		return fmt.Errorf("already shutting down")
	}

	// BUG: No graceful shutdown of actors
	as.mu.RLock()
	for _, actor := range as.actors {
		actor.cancel() // BUG: Abrupt cancellation, messages in mailbox are lost
	}
	as.mu.RUnlock()

	return nil
}

// Example behaviors with bugs
func EchoBehavior(ctx context.Context, self *Actor, msg Message) error {
	// BUG: Can panic on type assertion
	text := msg.Payload.(string)

	// Echo back
	response := Message{
		From:    self.ID,
		To:      msg.From,
		Type:    MsgResponse,
		Payload: text,
	}

	// BUG: No way to send response back to original sender
	_ = response

	return nil
}

func CounterBehavior(ctx context.Context, self *Actor, msg Message) error {
	// BUG: State is not thread-safe
	if self.state == nil {
		self.state = 0
	}

	counter := self.state.(int)

	switch msg.Type {
	case MsgRequest:
		// BUG: Can panic if payload is not string
		action := msg.Payload.(string)

		if action == "increment" {
			counter++
		} else if action == "decrement" {
			counter--
		}

		// BUG: Race condition when updating state
		self.state = counter

	case MsgPing:
		// BUG: Response is not sent back
		fmt.Printf("Counter value: %d\n", counter)
	}

	return nil
}

// RouterActor demonstrates routing with bugs
type RouterActor struct {
	*Actor
	workers []*Actor
	next    int32
}

func (r *RouterActor) Route(msg Message) error {
	// BUG: No bounds checking
	idx := atomic.AddInt32(&r.next, 1) % int32(len(r.workers))

	// BUG: Can panic if workers is empty
	return r.workers[idx].Send(msg)
}

// Challenge29ActorModel demonstrates the buggy actor model
func Challenge29ActorModel() {
	fmt.Println("Challenge 29: Actor Model Implementation")
	fmt.Println("========================================")

	system := NewActorSystem()

	// Create actors
	echo := system.SpawnActor("echo", EchoBehavior)
	counter := system.SpawnActor("counter", CounterBehavior)

	// Send messages
	echo.Send(Message{
		From:    "main",
		To:      "echo",
		Type:    MsgRequest,
		Payload: "Hello, Actor!",
	})

	// This will cause issues
	counter.Send(Message{
		From:    "main",
		To:      "counter",
		Type:    MsgRequest,
		Payload: "increment",
	})

	// Try to ask (will timeout)
	resp, err := echo.Ask(counter, Message{
		Type:    MsgPing,
		Payload: nil,
	}, 1*time.Second)

	if err != nil {
		fmt.Printf("Ask failed: %v\n", err)
	} else {
		fmt.Printf("Response: %v\n", resp)
	}

	// Simulate high load (will cause mailbox overflow)
	for i := 0; i < 100; i++ {
		err := counter.Send(Message{
			Type:    MsgRequest,
			Payload: "increment",
			Priority: i % 3, // Priority is ignored
		})
		if err != nil {
			fmt.Printf("Send failed at message %d: %v\n", i, err)
		}
	}

	// Shutdown (not graceful)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := system.Shutdown(ctx); err != nil {
		fmt.Printf("Shutdown error: %v\n", err)
	}

	fmt.Println("\nIssues to fix:")
	fmt.Println("1. Mailbox overflow handling")
	fmt.Println("2. Supervision tree implementation")
	fmt.Println("3. Panic recovery in actors")
	fmt.Println("4. Message priority queue")
	fmt.Println("5. Deadletter handling")
	fmt.Println("6. Location transparency")
	fmt.Println("7. Graceful shutdown")
	fmt.Println("8. Request-response correlation")
}