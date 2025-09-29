package challenges

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge 26: Service Mesh Patterns
//
// Problems to fix:
// 1. Circuit breaker not resetting properly
// 2. Load balancer causing hot spots
// 3. Retry storms amplifying failures
// 4. Service discovery race conditions
// 5. Health check false positives/negatives
// 6. Traffic routing loops
// 7. Timeout cascade failures
// 8. Sidecar proxy memory leaks

type ServiceInstance struct {
	ID          string
	Address     string
	Port        int
	Health      string // healthy, unhealthy, unknown
	Load        int32  // Current connections
	LastChecked time.Time
	mu          sync.RWMutex
}

type ServiceMesh struct {
	services    map[string][]*ServiceInstance
	registry    *ServiceRegistry
	loadBalancer *LoadBalancer
	circuitBreaker *CircuitBreaker
	retryPolicy *RetryPolicy
	router      *TrafficRouter
	sidecar     *SidecarProxy
	mu          sync.RWMutex
}

type ServiceRegistry struct {
	services   map[string][]*ServiceInstance
	watchers   []chan ServiceEvent
	mu         sync.RWMutex
	updateChan chan ServiceUpdate // BUG: Unbuffered channel
}

type LoadBalancer struct {
	algorithm string // round-robin, least-conn, random
	counter   uint64
	weights   map[string]int
	mu        sync.Mutex
}

type CircuitBreaker struct {
	state           string // closed, open, half-open
	failures        int32
	successes       int32
	lastFailureTime time.Time
	timeout         time.Duration
	threshold       int32
	mu              sync.Mutex // BUG: Should be RWMutex
}

type RetryPolicy struct {
	maxAttempts int
	backoff     time.Duration
	jitter      bool
	timeout     time.Duration
}

type TrafficRouter struct {
	rules      []RoutingRule
	fallback   string
	mu         sync.RWMutex
}

type RoutingRule struct {
	Match    func(*Request) bool
	Target   string
	Weight   int
	Canary   bool
	Fallback string
}

type SidecarProxy struct {
	inbound    map[string]*Connection
	outbound   map[string]*Connection
	middleware []Middleware
	metrics    *Metrics
	mu         sync.RWMutex
	bufferPool sync.Pool // BUG: Not properly initialized
}

type Connection struct {
	ID        string
	Service   string
	StartTime time.Time
	BytesIn   int64
	BytesOut  int64
	State     string // active, idle, closed
}

type ServiceEvent struct {
	Type    string // added, removed, updated
	Service string
	Instance *ServiceInstance
}

type ServiceUpdate struct {
	Service  string
	Instance *ServiceInstance
	Action   string
}

type Request struct {
	ID      string
	Service string
	Method  string
	Headers map[string]string
	Body    []byte
	Timeout time.Duration
}

type Response struct {
	StatusCode int
	Body       []byte
	Error      error
}

type Middleware func(*Request, *Response) error

type Metrics struct {
	requests   int64
	errors     int64
	latency    []time.Duration // BUG: Unbounded growth
	mu         sync.Mutex
}

func Challenge26ServiceMesh() {
	mesh := &ServiceMesh{
		services: make(map[string][]*ServiceInstance),
		registry: &ServiceRegistry{
			services:   make(map[string][]*ServiceInstance),
			watchers:   []chan ServiceEvent{},
			updateChan: make(chan ServiceUpdate), // BUG: Unbuffered
		},
		loadBalancer: &LoadBalancer{
			algorithm: "round-robin",
			weights:   make(map[string]int),
		},
		circuitBreaker: &CircuitBreaker{
			state:     "closed",
			threshold: 5,
			timeout:   10 * time.Second,
		},
		retryPolicy: &RetryPolicy{
			maxAttempts: 3,
			backoff:     100 * time.Millisecond,
			jitter:      true,
		},
		router: &TrafficRouter{
			rules: []RoutingRule{},
		},
		sidecar: &SidecarProxy{
			inbound:  make(map[string]*Connection),
			outbound: make(map[string]*Connection),
			metrics:  &Metrics{},
		},
	}

	// Start service discovery
	go mesh.registry.watch()

	// Start health checking
	go mesh.healthCheck()

	// Simulate service instances
	var wg sync.WaitGroup

	// Add services
	for i := 0; i < 3; i++ {
		service := fmt.Sprintf("service-%d", i)
		for j := 0; j < 3; j++ {
			instance := &ServiceInstance{
				ID:      fmt.Sprintf("%s-%d", service, j),
				Address: fmt.Sprintf("10.0.%d.%d", i, j),
				Port:    8080 + j,
				Health:  "healthy",
			}

			// BUG: Race condition when adding instances
			mesh.services[service] = append(mesh.services[service], instance)
			mesh.registry.services[service] = append(mesh.registry.services[service], instance)
		}
	}

	// Client requests
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			req := &Request{
				ID:      fmt.Sprintf("req-%d", id),
				Service: fmt.Sprintf("service-%d", id%3),
				Method:  "GET",
				Headers: make(map[string]string),
				Timeout: 1 * time.Second,
			}

			// BUG: Not handling circuit breaker state
			resp := mesh.callService(req)
			if resp.Error != nil {
				fmt.Printf("Request %s failed: %v\n", req.ID, resp.Error)
			}
		}(i)
	}

	// Simulate failures
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(100 * time.Millisecond)

		// BUG: Marking all instances as unhealthy at once
		for _, instances := range mesh.services {
			for _, inst := range instances {
				inst.Health = "unhealthy"
			}
		}
	}()

	// Traffic routing updates
	wg.Add(1)
	go func() {
		defer wg.Done()

		// BUG: Adding conflicting routing rules
		rule1 := RoutingRule{
			Match: func(r *Request) bool {
				return r.Service == "service-0"
			},
			Target: "service-1",
		}

		rule2 := RoutingRule{
			Match: func(r *Request) bool {
				return r.Service == "service-1"
			},
			Target: "service-0", // BUG: Creates routing loop
		}

		mesh.router.rules = append(mesh.router.rules, rule1, rule2)
	}()

	wg.Wait()

	// BUG: Not cleaning up resources
	// Missing connection cleanup
	// Missing metric cleanup
}

func (m *ServiceMesh) callService(req *Request) *Response {
	// BUG: Not checking circuit breaker first
	instance := m.selectInstance(req.Service)
	if instance == nil {
		return &Response{Error: errors.New("no healthy instances")}
	}

	// BUG: Not implementing timeout properly
	ctx, cancel := context.WithTimeout(context.Background(), req.Timeout)
	defer cancel()

	// Try with retries
	var lastError error
	for attempt := 0; attempt < m.retryPolicy.maxAttempts; attempt++ {
		if attempt > 0 {
			// BUG: Exponential backoff not implemented correctly
			time.Sleep(m.retryPolicy.backoff)
		}

		resp := m.executeRequest(ctx, instance, req)
		if resp.Error == nil {
			// BUG: Not updating circuit breaker on success
			return resp
		}

		lastError = resp.Error
		// BUG: Not checking if error is retryable
	}

	// BUG: Incrementing failures without limit
	atomic.AddInt32(&m.circuitBreaker.failures, 1)

	return &Response{Error: lastError}
}

func (m *ServiceMesh) selectInstance(service string) *ServiceInstance {
	m.mu.RLock()
	instances := m.services[service]
	m.mu.RUnlock()

	if len(instances) == 0 {
		return nil
	}

	// BUG: Load balancer not considering instance health
	switch m.loadBalancer.algorithm {
	case "round-robin":
		// BUG: Counter can overflow
		idx := atomic.AddUint64(&m.loadBalancer.counter, 1) % uint64(len(instances))
		return instances[idx]

	case "least-conn":
		// BUG: Not actually implementing least connections
		return instances[rand.Intn(len(instances))]

	default:
		return instances[rand.Intn(len(instances))]
	}
}

func (m *ServiceMesh) executeRequest(ctx context.Context, instance *ServiceInstance, req *Request) *Response {
	// Create sidecar connection
	conn := &Connection{
		ID:        generateID(),
		Service:   req.Service,
		StartTime: time.Now(),
		State:     "active",
	}

	// BUG: Memory leak - connections not cleaned up
	m.sidecar.mu.Lock()
	m.sidecar.outbound[conn.ID] = conn
	m.sidecar.mu.Unlock()

	// Simulate request
	select {
	case <-ctx.Done():
		return &Response{Error: errors.New("timeout")}
	case <-time.After(time.Duration(rand.Intn(200)) * time.Millisecond):
		if instance.Health != "healthy" {
			return &Response{Error: errors.New("unhealthy instance")}
		}
		return &Response{StatusCode: 200}
	}
}

func (m *ServiceMesh) healthCheck() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// BUG: Checking all services sequentially
		for service, instances := range m.services {
			for _, instance := range instances {
				// BUG: Not using timeout for health check
				go func(inst *ServiceInstance) {
					// Simulate health check
					time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

					// BUG: Race condition updating health
					if rand.Float32() > 0.8 {
						inst.Health = "unhealthy"
					} else {
						inst.Health = "healthy"
					}
					inst.LastChecked = time.Now()
				}(instance)
			}
			_ = service
		}
	}
}

func (r *ServiceRegistry) watch() {
	for update := range r.updateChan {
		// BUG: Can panic if watchers are modified concurrently
		for _, watcher := range r.watchers {
			event := ServiceEvent{
				Type:     update.Action,
				Service:  update.Service,
				Instance: update.Instance,
			}

			// BUG: Can block if watcher is slow
			watcher <- event
		}
	}
}

// Missing implementations:
// 1. Circuit breaker state transitions
// 2. Proper load balancing algorithms
// 3. Retry with jitter and backoff
// 4. Service discovery with TTL
// 5. Health check with proper timeout
// 6. Traffic routing without loops
// 7. Sidecar proxy connection pooling
// 8. Metrics aggregation and cleanup