package challenges

import (
	"fmt"
	"sync"
	"time"
)

// Challenge 21: Graph Database with Concurrent Traversal
//
// 問題点:
// 1. グラフトラバーサルのデッドロック
// 2. サイクル検出の不備
// 3. 並行更新時のデータ不整合
// 4. メモリリークとなる参照
// 5. インデックス管理の問題

type GraphNode struct {
	ID         string
	Properties map[string]interface{}
	mu         sync.RWMutex
	edges      map[string]*Edge // 問題: 循環参照によるメモリリーク
	version    int64
}

type Edge struct {
	ID         string
	From       *GraphNode
	To         *GraphNode
	Type       string
	Properties map[string]interface{}
	Weight     float64
	mu         sync.RWMutex
}

type GraphDatabase struct {
	mu          sync.RWMutex
	nodes       map[string]*GraphNode
	edges       map[string]*Edge
	indexes     map[string]map[string][]string // property -> value -> node IDs
	constraints map[string]func(*GraphNode) bool
	cache       sync.Map // 問題: 無制限キャッシュ
	txManager   *TransactionManager
}

type TransactionManager struct {
	mu           sync.Mutex
	transactions map[string]*GraphTransaction
	locks        map[string]string // node/edge ID -> transaction ID
}

type GraphTransaction struct {
	ID        string
	StartTime time.Time
	Changes   []Change
	State     string // active, committed, aborted
	mu        sync.Mutex
}

type Change struct {
	Type   string // create, update, delete
	Target string // node ID or edge ID
	Old    interface{}
	New    interface{}
}

type QueryResult struct {
	Nodes []GraphNode
	Edges []Edge
	Paths [][]string
	Error error
}

func NewGraphDatabase() *GraphDatabase {
	return &GraphDatabase{
		nodes:       make(map[string]*GraphNode),
		edges:       make(map[string]*Edge),
		indexes:     make(map[string]map[string][]string),
		constraints: make(map[string]func(*GraphNode) bool),
		txManager: &TransactionManager{
			transactions: make(map[string]*GraphTransaction),
			locks:        make(map[string]string),
		},
	}
}

// 問題1: ノード作成の競合状態
func (gdb *GraphDatabase) CreateNode(id string, props map[string]interface{}) (*GraphNode, error) {
	// 問題: 存在チェックと作成が別々の操作
	if _, exists := gdb.nodes[id]; exists {
		return nil, fmt.Errorf("node %s already exists", id)
	}

	node := &GraphNode{
		ID:         id,
		Properties: props,
		edges:      make(map[string]*Edge),
		version:    1,
	}

	// 問題: ロックなしで書き込み
	gdb.nodes[id] = node

	// 問題: インデックス更新が不完全
	for key, value := range props {
		gdb.updateIndex(key, fmt.Sprintf("%v", value), id)
	}

	return node, nil
}

// 問題2: エッジ作成時のデッドロック
func (gdb *GraphDatabase) CreateEdge(fromID, toID, edgeType string) (*Edge, error) {
	gdb.mu.Lock()
	fromNode := gdb.nodes[fromID]
	toNode := gdb.nodes[toID]
	gdb.mu.Unlock()

	if fromNode == nil || toNode == nil {
		return nil, fmt.Errorf("nodes not found")
	}

	// 問題: ノードロックの順序が不定でデッドロックの可能性
	fromNode.mu.Lock()
	toNode.mu.Lock()
	defer fromNode.mu.Unlock()
	defer toNode.mu.Unlock()

	edge := &Edge{
		ID:         fmt.Sprintf("%s-%s-%s", fromID, toID, edgeType),
		From:       fromNode,
		To:         toNode,
		Type:       edgeType,
		Properties: make(map[string]interface{}),
	}

	// 問題: 循環参照でメモリリーク
	fromNode.edges[edge.ID] = edge
	toNode.edges[edge.ID] = edge

	gdb.mu.Lock()
	gdb.edges[edge.ID] = edge
	gdb.mu.Unlock()

	return edge, nil
}

// 問題3: グラフトラバーサル
func (gdb *GraphDatabase) TraverseBFS(startID string, maxDepth int) QueryResult {
	gdb.mu.RLock()
	startNode := gdb.nodes[startID]
	gdb.mu.RUnlock()

	if startNode == nil {
		return QueryResult{Error: fmt.Errorf("start node not found")}
	}

	visited := make(map[string]bool) // 問題: 並行アクセス時に安全でない
	queue := []string{startID}
	depth := 0
	result := QueryResult{}

	for len(queue) > 0 && depth < maxDepth {
		levelSize := len(queue)

		for i := 0; i < levelSize; i++ {
			nodeID := queue[0]
			queue = queue[1:]

			// 問題: visited チェックが不完全
			if visited[nodeID] {
				continue
			}
			visited[nodeID] = true

			gdb.mu.RLock()
			node := gdb.nodes[nodeID]
			gdb.mu.RUnlock()

			if node == nil {
				continue
			}

			// 問題: ノードのコピーが深いコピーでない
			result.Nodes = append(result.Nodes, *node)

			// 問題: エッジ探索中のロック
			node.mu.RLock()
			for _, edge := range node.edges {
				// 問題: nilチェックなし
				if edge.To.ID != nodeID {
					queue = append(queue, edge.To.ID)
				}
			}
			node.mu.RUnlock()
		}
		depth++
	}

	return result
}

// 問題4: サイクル検出
func (gdb *GraphDatabase) DetectCycle(startID string) bool {
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	// 問題: DFSでのサイクル検出が不完全
	var dfs func(nodeID string) bool
	dfs = func(nodeID string) bool {
		visited[nodeID] = true
		recStack[nodeID] = true

		gdb.mu.RLock()
		node := gdb.nodes[nodeID]
		gdb.mu.RUnlock()

		if node == nil {
			return false
		}

		// 問題: ロック順序
		node.mu.RLock()
		defer node.mu.RUnlock()

		for _, edge := range node.edges {
			nextID := edge.To.ID

			// 問題: 有向グラフと無向グラフの区別なし
			if !visited[nextID] {
				if dfs(nextID) {
					return true
				}
			} else if recStack[nextID] {
				return true
			}
		}

		recStack[nodeID] = false
		return false
	}

	return dfs(startID)
}

// 問題5: トランザクション処理
func (gdb *GraphDatabase) BeginTransaction() *GraphTransaction {
	tx := &GraphTransaction{
		ID:        fmt.Sprintf("tx-%d", time.Now().UnixNano()),
		StartTime: time.Now(),
		Changes:   []Change{},
		State:     "active",
	}

	// 問題: トランザクション登録なし
	// gdb.txManager.transactions[tx.ID] = tx を忘れている

	return tx
}

func (gdb *GraphDatabase) UpdateNode(txID, nodeID string, updates map[string]interface{}) error {
	// 問題: トランザクション検証なし

	gdb.mu.Lock()
	node := gdb.nodes[nodeID]
	gdb.mu.Unlock()

	if node == nil {
		return fmt.Errorf("node not found")
	}

	// 問題: 楽観的ロックなし（バージョンチェックなし）
	node.mu.Lock()
	defer node.mu.Unlock()

	// 問題: 変更履歴の記録なし
	for key, value := range updates {
		node.Properties[key] = value
	}

	node.version++ // 問題: アトミックでない

	return nil
}

// 問題6: インデックス管理
func (gdb *GraphDatabase) updateIndex(property, value, nodeID string) {
	// 問題: 並行アクセス保護なし
	if gdb.indexes[property] == nil {
		gdb.indexes[property] = make(map[string][]string)
	}

	// 問題: 重複チェックなし
	gdb.indexes[property][value] = append(gdb.indexes[property][value], nodeID)
}

func (gdb *GraphDatabase) QueryByProperty(property, value string) []string {
	gdb.mu.RLock()
	defer gdb.mu.RUnlock()

	// 問題: nil チェックなし
	return gdb.indexes[property][value]
}

// 問題7: 最短経路探索
func (gdb *GraphDatabase) ShortestPath(fromID, toID string) []string {
	// 問題: 実装が不完全（BFSのみで重み考慮なし）
	visited := make(map[string]bool)
	parent := make(map[string]string)
	queue := []string{fromID}

	visited[fromID] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == toID {
			// パス復元
			path := []string{}
			for current != "" {
				path = append([]string{current}, path...)
				current = parent[current]
			}
			return path
		}

		// 問題: エッジの重み無視
		gdb.mu.RLock()
		node := gdb.nodes[current]
		gdb.mu.RUnlock()

		if node != nil {
			node.mu.RLock()
			for _, edge := range node.edges {
				next := edge.To.ID
				if !visited[next] {
					visited[next] = true
					parent[next] = current
					queue = append(queue, next)
				}
			}
			node.mu.RUnlock()
		}
	}

	return nil
}

// Challenge: 以下の問題を修正してください
// 1. デッドロックフリーなロック戦略
// 2. メモリリークの修正
// 3. ACID特性の実装
// 4. 効率的なインデックス管理
// 5. 重み付き最短経路（Dijkstra）
// 6. グラフパーティショニング
// 7. 分散グラフ処理
// 8. グラフ分析アルゴリズム（PageRank風）

func RunChallenge21() {
	fmt.Println("Challenge 21: Graph Database Patterns")
	fmt.Println("Fix the graph database implementation")

	gdb := NewGraphDatabase()

	// ノード作成
	users := []string{"Alice", "Bob", "Charlie", "David", "Eve"}
	for _, user := range users {
		gdb.CreateNode(user, map[string]interface{}{
			"type": "user",
			"age":  25 + len(user),
		})
	}

	// エッジ作成（友人関係）
	gdb.CreateEdge("Alice", "Bob", "friend")
	gdb.CreateEdge("Bob", "Charlie", "friend")
	gdb.CreateEdge("Charlie", "David", "friend")
	gdb.CreateEdge("David", "Eve", "friend")
	gdb.CreateEdge("Eve", "Alice", "friend") // サイクル作成

	// グラフトラバーサル
	result := gdb.TraverseBFS("Alice", 3)
	fmt.Printf("BFS from Alice: %d nodes found\n", len(result.Nodes))

	// サイクル検出
	hasCycle := gdb.DetectCycle("Alice")
	fmt.Printf("Cycle detected: %v\n", hasCycle)

	// 最短経路
	path := gdb.ShortestPath("Alice", "David")
	fmt.Printf("Shortest path from Alice to David: %v\n", path)

	// 並行更新テスト
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tx := gdb.BeginTransaction()
			gdb.UpdateNode(tx.ID, "Alice", map[string]interface{}{
				fmt.Sprintf("prop%d", id): id,
			})
		}(i)
	}
	wg.Wait()
}