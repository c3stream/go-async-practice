package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Solution21 demonstrates three approaches to graph database concurrency:
// 1. Lock-free graph traversal with atomic operations
// 2. Partitioned graph with local locks
// 3. Optimistic concurrency control with versioning

// Solution 1: Lock-free Graph Traversal
type LockFreeGraph struct {
	nodes     sync.Map // nodeID -> *Node
	edges     sync.Map // edgeID -> *Edge
	nodeCount int64
	edgeCount int64
}

type Node struct {
	ID         string
	Properties sync.Map
	OutEdges   sync.Map // edgeID -> *Edge
	InEdges    sync.Map // edgeID -> *Edge
	Version    int64
}

type Edge struct {
	ID         string
	From       string
	To         string
	Label      string
	Properties sync.Map
	Version    int64
}

type TraversalResult struct {
	Path       []string
	Properties map[string]interface{}
	Distance   int
}

func NewLockFreeGraph() *LockFreeGraph {
	return &LockFreeGraph{}
}

func (g *LockFreeGraph) AddNode(id string) *Node {
	node := &Node{
		ID:      id,
		Version: 0,
	}

	if actual, loaded := g.nodes.LoadOrStore(id, node); loaded {
		return actual.(*Node)
	}

	atomic.AddInt64(&g.nodeCount, 1)
	return node
}

func (g *LockFreeGraph) AddEdge(id, from, to, label string) *Edge {
	// Ensure nodes exist
	fromNode := g.AddNode(from)
	toNode := g.AddNode(to)

	edge := &Edge{
		ID:      id,
		From:    from,
		To:      to,
		Label:   label,
		Version: 0,
	}

	if actual, loaded := g.edges.LoadOrStore(id, edge); loaded {
		return actual.(*Edge)
	}

	// Add to node edge lists
	fromNode.OutEdges.Store(id, edge)
	toNode.InEdges.Store(id, edge)

	atomic.AddInt64(&g.edgeCount, 1)
	return edge
}

func (g *LockFreeGraph) BFS(ctx context.Context, start string, maxDepth int) ([]TraversalResult, error) {
	_, ok := g.nodes.Load(start)
	if !ok {
		return nil, fmt.Errorf("node %s not found", start)
	}

	visited := sync.Map{}
	queue := make(chan TraversalResult, 1000)
	results := make([]TraversalResult, 0)
	var wg sync.WaitGroup

	// Start traversal
	queue <- TraversalResult{
		Path:     []string{start},
		Distance: 0,
	}
	visited.Store(start, true)

	// Process queue
	for depth := 0; depth < maxDepth; depth++ {
		queueSize := len(queue)
		if queueSize == 0 {
			break
		}

		for i := 0; i < queueSize; i++ {
			select {
			case <-ctx.Done():
				return results, ctx.Err()
			case current := <-queue:
				results = append(results, current)

				// Get current node
				nodeVal, _ := g.nodes.Load(current.Path[len(current.Path)-1])
				node := nodeVal.(*Node)

				// Process neighbors in parallel
				node.OutEdges.Range(func(key, value interface{}) bool {
					edge := value.(*Edge)

					if _, exists := visited.LoadOrStore(edge.To, true); !exists {
						wg.Add(1)
						go func(e *Edge, path []string, dist int) {
							defer wg.Done()

							newPath := make([]string, len(path))
							copy(newPath, path)
							newPath = append(newPath, e.To)

							select {
							case queue <- TraversalResult{
								Path:     newPath,
								Distance: dist + 1,
							}:
							case <-ctx.Done():
							}
						}(edge, current.Path, current.Distance)
					}
					return true
				})
			}
		}

		// Wait for all goroutines to finish this level
		wg.Wait()
	}

	return results, nil
}

func (g *LockFreeGraph) DFS(ctx context.Context, start string, target string) (*TraversalResult, error) {
	visited := sync.Map{}
	resultChan := make(chan *TraversalResult, 1)

	var dfs func(nodeID string, path []string, depth int) bool
	dfs = func(nodeID string, path []string, depth int) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		if nodeID == target {
			resultChan <- &TraversalResult{
				Path:     path,
				Distance: depth,
			}
			return true
		}

		if _, exists := visited.LoadOrStore(nodeID, true); exists {
			return false
		}

		nodeVal, ok := g.nodes.Load(nodeID)
		if !ok {
			return false
		}
		node := nodeVal.(*Node)

		found := false
		node.OutEdges.Range(func(key, value interface{}) bool {
			if found {
				return false
			}

			edge := value.(*Edge)
			newPath := append(path, edge.To)
			if dfs(edge.To, newPath, depth+1) {
				found = true
				return false
			}
			return true
		})

		return found
	}

	go func() {
		dfs(start, []string{start}, 0)
		close(resultChan)
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(5 * time.Second):
		return nil, fmt.Errorf("DFS timeout")
	}
}

// Solution 2: Partitioned Graph
type PartitionedGraph struct {
	partitions []*GraphPartition
	numParts   int
	router     *PartitionRouter
}

type GraphPartition struct {
	mu       sync.RWMutex
	id       int
	nodes    map[string]*PartitionNode
	edges    map[string]*PartitionEdge
	crossRef sync.Map // References to other partitions
}

type PartitionNode struct {
	ID         string
	Partition  int
	Properties map[string]interface{}
	Edges      []string
}

type PartitionEdge struct {
	ID           string
	From         string
	To           string
	FromPart     int
	ToPart       int
	Properties   map[string]interface{}
}

type PartitionRouter struct {
	mu         sync.RWMutex
	nodeMap    map[string]int // nodeID -> partition
	loadFactor []int64        // Load per partition
}

func NewPartitionedGraph(numPartitions int) *PartitionedGraph {
	partitions := make([]*GraphPartition, numPartitions)
	for i := 0; i < numPartitions; i++ {
		partitions[i] = &GraphPartition{
			id:    i,
			nodes: make(map[string]*PartitionNode),
			edges: make(map[string]*PartitionEdge),
		}
	}

	return &PartitionedGraph{
		partitions: partitions,
		numParts:   numPartitions,
		router: &PartitionRouter{
			nodeMap:    make(map[string]int),
			loadFactor: make([]int64, numPartitions),
		},
	}
}

func (pg *PartitionedGraph) getPartition(nodeID string) int {
	pg.router.mu.RLock()
	if part, exists := pg.router.nodeMap[nodeID]; exists {
		pg.router.mu.RUnlock()
		return part
	}
	pg.router.mu.RUnlock()

	// Assign to partition with lowest load
	minLoad := atomic.LoadInt64(&pg.router.loadFactor[0])
	minPart := 0

	for i := 1; i < pg.numParts; i++ {
		load := atomic.LoadInt64(&pg.router.loadFactor[i])
		if load < minLoad {
			minLoad = load
			minPart = i
		}
	}

	pg.router.mu.Lock()
	pg.router.nodeMap[nodeID] = minPart
	pg.router.mu.Unlock()

	atomic.AddInt64(&pg.router.loadFactor[minPart], 1)
	return minPart
}

func (pg *PartitionedGraph) AddNode(id string, properties map[string]interface{}) {
	partID := pg.getPartition(id)
	partition := pg.partitions[partID]

	partition.mu.Lock()
	defer partition.mu.Unlock()

	partition.nodes[id] = &PartitionNode{
		ID:         id,
		Partition:  partID,
		Properties: properties,
		Edges:      make([]string, 0),
	}
}

func (pg *PartitionedGraph) AddEdge(id, from, to string, properties map[string]interface{}) {
	fromPart := pg.getPartition(from)
	toPart := pg.getPartition(to)

	edge := &PartitionEdge{
		ID:         id,
		From:       from,
		To:         to,
		FromPart:   fromPart,
		ToPart:     toPart,
		Properties: properties,
	}

	// Add edge to source partition
	partition := pg.partitions[fromPart]
	partition.mu.Lock()
	partition.edges[id] = edge
	if node, exists := partition.nodes[from]; exists {
		node.Edges = append(node.Edges, id)
	}
	partition.mu.Unlock()

	// Add cross-partition reference if needed
	if fromPart != toPart {
		partition.crossRef.Store(to, toPart)
	}
}

func (pg *PartitionedGraph) ParallelTraversal(ctx context.Context, start string) ([]string, error) {
	visited := sync.Map{}
	results := make([]string, 0)
	resultMu := sync.Mutex{}

	var wg sync.WaitGroup
	var traverse func(nodeID string, partID int)

	traverse = func(nodeID string, partID int) {
		defer wg.Done()

		if _, exists := visited.LoadOrStore(nodeID, true); exists {
			return
		}

		resultMu.Lock()
		results = append(results, nodeID)
		resultMu.Unlock()

		partition := pg.partitions[partID]
		partition.mu.RLock()
		node, exists := partition.nodes[nodeID]
		if !exists {
			partition.mu.RUnlock()
			return
		}

		edges := make([]string, len(node.Edges))
		copy(edges, node.Edges)
		partition.mu.RUnlock()

		// Process edges
		for _, edgeID := range edges {
			partition.mu.RLock()
			edge, exists := partition.edges[edgeID]
			partition.mu.RUnlock()

			if exists {
				wg.Add(1)
				go traverse(edge.To, edge.ToPart)
			}
		}
	}

	startPart := pg.getPartition(start)
	wg.Add(1)
	go traverse(start, startPart)

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return results, nil
	case <-ctx.Done():
		return results, ctx.Err()
	case <-time.After(5 * time.Second):
		return results, fmt.Errorf("traversal timeout")
	}
}

// Solution 3: Optimistic Concurrency Control
type OptimisticGraph struct {
	mu          sync.RWMutex
	nodes       map[string]*VersionedNode
	edges       map[string]*VersionedEdge
	txManager   *GraphTransactionManager
}

type VersionedNode struct {
	ID         string
	Version    int64
	Properties map[string]interface{}
	Edges      []string
	Lock       sync.RWMutex
}

type VersionedEdge struct {
	ID         string
	Version    int64
	From       string
	To         string
	Properties map[string]interface{}
	Lock       sync.RWMutex
}

type GraphTransaction struct {
	ID        string
	StartTime time.Time
	ReadSet   map[string]int64 // entity -> version
	WriteSet  map[string]interface{}
	Status    string
}

type GraphTransactionManager struct {
	mu           sync.RWMutex
	transactions map[string]*GraphTransaction
	globalClock  int64
}

func NewOptimisticGraph() *OptimisticGraph {
	return &OptimisticGraph{
		nodes: make(map[string]*VersionedNode),
		edges: make(map[string]*VersionedEdge),
		txManager: &GraphTransactionManager{
			transactions: make(map[string]*GraphTransaction),
		},
	}
}

func (og *OptimisticGraph) BeginTransaction() *GraphTransaction {
	tx := &GraphTransaction{
		ID:        fmt.Sprintf("tx_%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		ReadSet:   make(map[string]int64),
		WriteSet:  make(map[string]interface{}),
		Status:    "active",
	}

	og.txManager.mu.Lock()
	og.txManager.transactions[tx.ID] = tx
	og.txManager.mu.Unlock()

	return tx
}

func (og *OptimisticGraph) ReadNode(tx *GraphTransaction, nodeID string) (*VersionedNode, error) {
	og.mu.RLock()
	node, exists := og.nodes[nodeID]
	og.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("node %s not found", nodeID)
	}

	node.Lock.RLock()
	version := node.Version
	node.Lock.RUnlock()

	// Record read version
	tx.ReadSet[nodeID] = version

	return node, nil
}

func (og *OptimisticGraph) UpdateNode(tx *GraphTransaction, nodeID string, updates map[string]interface{}) {
	tx.WriteSet[nodeID] = updates
}

func (og *OptimisticGraph) Commit(tx *GraphTransaction) error {
	// Validation phase
	for entityID, readVersion := range tx.ReadSet {
		og.mu.RLock()
		node, exists := og.nodes[entityID]
		og.mu.RUnlock()

		if !exists {
			return fmt.Errorf("node %s was deleted", entityID)
		}

		node.Lock.RLock()
		currentVersion := node.Version
		node.Lock.RUnlock()

		if currentVersion != readVersion {
			return fmt.Errorf("conflict on node %s: read version %d, current version %d",
				entityID, readVersion, currentVersion)
		}
	}

	// Write phase
	newVersion := atomic.AddInt64(&og.txManager.globalClock, 1)

	for entityID, updates := range tx.WriteSet {
		og.mu.Lock()
		if node, exists := og.nodes[entityID]; exists {
			node.Lock.Lock()
			// Apply updates
			if props, ok := updates.(map[string]interface{}); ok {
				for k, v := range props {
					node.Properties[k] = v
				}
			}
			node.Version = newVersion
			node.Lock.Unlock()
		}
		og.mu.Unlock()
	}

	// Mark transaction as committed
	og.txManager.mu.Lock()
	tx.Status = "committed"
	og.txManager.mu.Unlock()

	return nil
}

func (og *OptimisticGraph) CycleDetection(ctx context.Context, start string) (bool, []string) {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := make([]string, 0)
	var cycle []string
	hasCycle := false

	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		select {
		case <-ctx.Done():
			return false
		default:
		}

		visited[nodeID] = true
		recStack[nodeID] = true
		path = append(path, nodeID)

		og.mu.RLock()
		node, exists := og.nodes[nodeID]
		og.mu.RUnlock()

		if exists {
			for _, edgeID := range node.Edges {
				og.mu.RLock()
				edge, exists := og.edges[edgeID]
				og.mu.RUnlock()

				if exists {
					if !visited[edge.To] {
						if dfs(edge.To) {
							return true
						}
					} else if recStack[edge.To] {
						// Found cycle
						hasCycle = true
						// Extract cycle path
						for i, n := range path {
							if n == edge.To {
								cycle = path[i:]
								break
							}
						}
						return true
					}
				}
			}
		}

		recStack[nodeID] = false
		path = path[:len(path)-1]
		return false
	}

	dfs(start)
	return hasCycle, cycle
}

// RunSolution21 demonstrates the three graph database solutions
func RunSolution21() {
	fmt.Println("=== Solution 21: Graph Database Patterns ===")

	ctx := context.Background()

	// Solution 1: Lock-free Graph
	fmt.Println("\n1. Lock-free Graph Traversal:")
	lfGraph := NewLockFreeGraph()

	// Create graph
	nodes := []string{"A", "B", "C", "D", "E"}
	for _, n := range nodes {
		lfGraph.AddNode(n)
	}

	lfGraph.AddEdge("e1", "A", "B", "follows")
	lfGraph.AddEdge("e2", "A", "C", "follows")
	lfGraph.AddEdge("e3", "B", "D", "follows")
	lfGraph.AddEdge("e4", "C", "E", "follows")
	lfGraph.AddEdge("e5", "D", "E", "follows")

	// BFS traversal
	results, err := lfGraph.BFS(ctx, "A", 3)
	if err != nil {
		fmt.Printf("  BFS error: %v\n", err)
	} else {
		fmt.Printf("  BFS found %d nodes\n", len(results))
		for _, r := range results[:min(5, len(results))] {
			fmt.Printf("    Path: %v, Distance: %d\n", r.Path, r.Distance)
		}
	}

	// DFS search
	result, err := lfGraph.DFS(ctx, "A", "E")
	if err != nil {
		fmt.Printf("  DFS error: %v\n", err)
	} else {
		fmt.Printf("  DFS path from A to E: %v\n", result.Path)
	}

	// Solution 2: Partitioned Graph
	fmt.Println("\n2. Partitioned Graph:")
	pGraph := NewPartitionedGraph(4)

	// Add nodes and edges
	for i := 1; i <= 10; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		pGraph.AddNode(nodeID, map[string]interface{}{"value": i})
	}

	for i := 1; i < 10; i++ {
		edgeID := fmt.Sprintf("edge%d", i)
		from := fmt.Sprintf("node%d", i)
		to := fmt.Sprintf("node%d", i+1)
		pGraph.AddEdge(edgeID, from, to, nil)
	}

	// Parallel traversal
	visited, err := pGraph.ParallelTraversal(ctx, "node1")
	if err != nil {
		fmt.Printf("  Parallel traversal error: %v\n", err)
	} else {
		fmt.Printf("  Parallel traversal visited %d nodes\n", len(visited))
	}

	// Solution 3: Optimistic Concurrency Control
	fmt.Println("\n3. Optimistic Concurrency Control:")
	ocGraph := NewOptimisticGraph()

	// Initialize graph
	for _, n := range []string{"X", "Y", "Z"} {
		ocGraph.nodes[n] = &VersionedNode{
			ID:         n,
			Properties: make(map[string]interface{}),
			Edges:      make([]string, 0),
		}
	}

	// Transaction 1
	tx1 := ocGraph.BeginTransaction()
	node, _ := ocGraph.ReadNode(tx1, "X")
	fmt.Printf("  Read node X, version: %d\n", node.Version)

	ocGraph.UpdateNode(tx1, "X", map[string]interface{}{"color": "red"})

	err = ocGraph.Commit(tx1)
	if err != nil {
		fmt.Printf("  Transaction 1 failed: %v\n", err)
	} else {
		fmt.Printf("  Transaction 1 committed successfully\n")
	}

	// Cycle detection
	ocGraph.nodes["X"].Edges = append(ocGraph.nodes["X"].Edges, "e1")
	ocGraph.edges = map[string]*VersionedEdge{
		"e1": {From: "X", To: "Y"},
		"e2": {From: "Y", To: "Z"},
		"e3": {From: "Z", To: "X"},
	}
	ocGraph.nodes["Y"].Edges = append(ocGraph.nodes["Y"].Edges, "e2")
	ocGraph.nodes["Z"].Edges = append(ocGraph.nodes["Z"].Edges, "e3")

	hasCycle, cyclePath := ocGraph.CycleDetection(ctx, "X")
	if hasCycle {
		fmt.Printf("  Cycle detected: %v\n", cyclePath)
	} else {
		fmt.Printf("  No cycle detected\n")
	}

	fmt.Println("\nAll graph database patterns demonstrated successfully!")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}