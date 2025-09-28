package practical

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// Neo4jGraph - Neo4jを使ったグラフデータベース処理
type Neo4jGraph struct {
	driver neo4j.DriverWithContext
	config neo4j.Config
	mu     sync.RWMutex
}

// NewNeo4jGraph - Neo4j接続の初期化
func NewNeo4jGraph(uri, username, password string) (*Neo4jGraph, error) {
	// Neo4jドライバーの設定
	config := func(conf *neo4j.Config) {
		conf.MaxConnectionPoolSize = 50
		conf.MaxConnectionLifetime = 5 * time.Minute
		conf.ConnectionAcquisitionTimeout = 30 * time.Second
	}

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(username, password, ""), config)
	if err != nil {
		// デモモード
		fmt.Println("⚠ Neo4jに接続できません。デモモードで実行します。")
		return &Neo4jGraph{
			driver: nil,
		}, nil
	}

	// 接続テスト
	ctx := context.Background()
	err = driver.VerifyConnectivity(ctx)
	if err != nil {
		fmt.Printf("Neo4j接続エラー: %v\n", err)
		driver.Close(ctx)
		return &Neo4jGraph{driver: nil}, nil
	}

	return &Neo4jGraph{
		driver: driver,
	}, nil
}

// SocialNetworkDemo - ソーシャルネットワーク分析デモ
func (n *Neo4jGraph) SocialNetworkDemo(ctx context.Context) {
	fmt.Println("\n🌐 Neo4j ソーシャルネットワーク分析デモ")
	fmt.Println("=" + repeatString("=", 50))

	if n.driver == nil {
		n.runDemoMode(ctx)
		return
	}

	// データ初期化
	n.initializeSocialNetwork(ctx)

	// 並列データ生成
	n.parallelDataGeneration(ctx)

	// グラフアルゴリズム実行
	n.runGraphAlgorithms(ctx)

	// リアルタイム更新
	n.realtimeUpdates(ctx)

	// 分析結果表示
	n.showAnalytics(ctx)
}

// initializeSocialNetwork - ソーシャルネットワークの初期化
func (n *Neo4jGraph) initializeSocialNetwork(ctx context.Context) error {
	session := n.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	// インデックス作成
	queries := []string{
		"CREATE INDEX user_id IF NOT EXISTS FOR (u:User) ON (u.id)",
		"CREATE INDEX post_id IF NOT EXISTS FOR (p:Post) ON (p.id)",
		"CREATE INDEX tag_name IF NOT EXISTS FOR (t:Tag) ON (t.name)",
	}

	for _, query := range queries {
		_, err := session.Run(ctx, query, nil)
		if err != nil {
			fmt.Printf("インデックス作成エラー: %v\n", err)
		}
	}

	fmt.Println("  ✓ ソーシャルネットワークを初期化しました")
	return nil
}

// parallelDataGeneration - 並列データ生成
func (n *Neo4jGraph) parallelDataGeneration(ctx context.Context) {
	fmt.Println("\n📊 並列グラフデータ生成")

	var (
		userCount  int64
		postCount  int64
		edgeCount  int64
		errorCount int64
		wg         sync.WaitGroup
	)

	// ユーザー生成ワーカー
	numWorkers := 5
	usersPerWorker := 20

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			session := n.driver.NewSession(ctx, neo4j.SessionConfig{
				AccessMode: neo4j.AccessModeWrite,
			})
			defer session.Close(ctx)

			for i := 0; i < usersPerWorker; i++ {
				userID := fmt.Sprintf("user_%d_%d", workerID, i)
				userName := fmt.Sprintf("User %d-%d", workerID, i)

				// ユーザー作成
				result, err := session.Run(ctx, `
					MERGE (u:User {id: $id})
					SET u.name = $name,
					    u.created_at = datetime(),
					    u.influence_score = rand() * 100
					RETURN u.id as id
				`, map[string]interface{}{
					"id":   userID,
					"name": userName,
				})

				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					continue
				}

				if result.Next(ctx) {
					atomic.AddInt64(&userCount, 1)
				}

				// ポスト作成
				for p := 0; p < 3; p++ {
					postID := fmt.Sprintf("post_%s_%d", userID, p)
					_, err := session.Run(ctx, `
						MATCH (u:User {id: $user_id})
						CREATE (p:Post {
							id: $post_id,
							content: $content,
							timestamp: datetime(),
							likes: toInteger(rand() * 1000),
							shares: toInteger(rand() * 100)
						})
						CREATE (u)-[:POSTED]->(p)
						RETURN p.id
					`, map[string]interface{}{
						"user_id":  userID,
						"post_id":  postID,
						"content":  fmt.Sprintf("Post content %d", p),
					})

					if err == nil {
						atomic.AddInt64(&postCount, 1)
					}
				}

				// フォロー関係作成（ランダム）
				if i > 0 {
					targetID := fmt.Sprintf("user_%d_%d", workerID, i-1)
					_, err := session.Run(ctx, `
						MATCH (u1:User {id: $from_id})
						MATCH (u2:User {id: $to_id})
						MERGE (u1)-[:FOLLOWS]->(u2)
						RETURN u1.id
					`, map[string]interface{}{
						"from_id": userID,
						"to_id":   targetID,
					})

					if err == nil {
						atomic.AddInt64(&edgeCount, 1)
					}
				}
			}

			fmt.Printf("  Worker %d: %d ユーザー作成完了\n", workerID, usersPerWorker)
		}(w)
	}

	wg.Wait()

	fmt.Printf("\n  📈 生成結果:\n")
	fmt.Printf("    ユーザー: %d\n", userCount)
	fmt.Printf("    ポスト: %d\n", postCount)
	fmt.Printf("    関係: %d\n", edgeCount)
	fmt.Printf("    エラー: %d\n", errorCount)
}

// runGraphAlgorithms - グラフアルゴリズムの実行
func (n *Neo4jGraph) runGraphAlgorithms(ctx context.Context) {
	fmt.Println("\n🔍 グラフアルゴリズム実行")

	algorithms := []struct {
		name  string
		query string
	}{
		{
			name: "最短パス探索",
			query: `
				MATCH (start:User {id: 'user_0_0'})
				MATCH (end:User {id: 'user_2_5'})
				MATCH path = shortestPath((start)-[:FOLLOWS*]-(end))
				RETURN length(path) as distance,
				       [n in nodes(path) | n.id] as path_nodes
				LIMIT 1
			`,
		},
		{
			name: "影響力の高いユーザー（PageRank風）",
			query: `
				MATCH (u:User)
				OPTIONAL MATCH (u)<-[:FOLLOWS]-(follower)
				WITH u, COUNT(follower) as followers
				RETURN u.id, u.name, followers, u.influence_score
				ORDER BY followers DESC, u.influence_score DESC
				LIMIT 5
			`,
		},
		{
			name: "コミュニティ検出（相互フォロー）",
			query: `
				MATCH (u1:User)-[:FOLLOWS]->(u2:User)
				WHERE (u2)-[:FOLLOWS]->(u1)
				RETURN u1.id, u2.id, 'mutual' as relationship
				LIMIT 10
			`,
		},
		{
			name: "コンテンツの拡散パス",
			query: `
				MATCH (p:Post)<-[:POSTED]-(author:User)
				OPTIONAL MATCH (author)<-[:FOLLOWS*1..3]-(reached:User)
				WITH p, author, COUNT(DISTINCT reached) as potential_reach
				RETURN p.id, author.id, potential_reach
				ORDER BY potential_reach DESC
				LIMIT 5
			`,
		},
	}

	session := n.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	for _, algo := range algorithms {
		fmt.Printf("\n  🎯 %s:\n", algo.name)

		result, err := session.Run(ctx, algo.query, nil)
		if err != nil {
			fmt.Printf("    エラー: %v\n", err)
			continue
		}

		count := 0
		for result.Next(ctx) {
			record := result.Record()
			fmt.Printf("    %v\n", record.Values...)
			count++
			if count >= 3 {
				break
			}
		}
	}
}

// realtimeUpdates - リアルタイム更新シミュレーション
func (n *Neo4jGraph) realtimeUpdates(ctx context.Context) {
	fmt.Println("\n⚡ リアルタイム更新デモ")

	var (
		likeCount    int64
		commentCount int64
		shareCount   int64
		wg           sync.WaitGroup
	)

	// いいね！ワーカー
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			session := n.driver.NewSession(ctx, neo4j.SessionConfig{
				AccessMode: neo4j.AccessModeWrite,
			})
			defer session.Close(ctx)

			for j := 0; j < 10; j++ {
				userID := fmt.Sprintf("user_%d_%d", workerID, j%5)
				postID := fmt.Sprintf("post_user_%d_%d_%d", workerID, j%5, 0)

				_, err := session.Run(ctx, `
					MATCH (u:User {id: $user_id})
					MATCH (p:Post {id: $post_id})
					MERGE (u)-[:LIKED]->(p)
					SET p.likes = p.likes + 1
					RETURN p.id
				`, map[string]interface{}{
					"user_id": userID,
					"post_id": postID,
				})

				if err == nil {
					atomic.AddInt64(&likeCount, 1)
				}

				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	// コメントワーカー
	wg.Add(1)
	go func() {
		defer wg.Done()

		session := n.driver.NewSession(ctx, neo4j.SessionConfig{
			AccessMode: neo4j.AccessModeWrite,
		})
		defer session.Close(ctx)

		for i := 0; i < 5; i++ {
			userID := fmt.Sprintf("user_1_%d", i)
			postID := fmt.Sprintf("post_user_0_%d_0", i%3)
			commentID := fmt.Sprintf("comment_%d", i)

			_, err := session.Run(ctx, `
				MATCH (u:User {id: $user_id})
				MATCH (p:Post {id: $post_id})
				CREATE (c:Comment {
					id: $comment_id,
					content: $content,
					timestamp: datetime()
				})
				CREATE (u)-[:COMMENTED]->(c)
				CREATE (c)-[:ON_POST]->(p)
				RETURN c.id
			`, map[string]interface{}{
				"user_id":    userID,
				"post_id":    postID,
				"comment_id": commentID,
				"content":    fmt.Sprintf("Comment %d", i),
			})

			if err == nil {
				atomic.AddInt64(&commentCount, 1)
			}
		}
	}()

	wg.Wait()

	fmt.Printf("\n  📊 アクティビティ統計:\n")
	fmt.Printf("    いいね: %d\n", likeCount)
	fmt.Printf("    コメント: %d\n", commentCount)
	fmt.Printf("    シェア: %d\n", shareCount)
}

// showAnalytics - 分析結果表示
func (n *Neo4jGraph) showAnalytics(ctx context.Context) {
	fmt.Println("\n📊 ネットワーク分析結果")

	session := n.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	// ネットワーク統計
	queries := []struct {
		name  string
		query string
	}{
		{
			"総ノード数",
			"MATCH (n) RETURN COUNT(n) as count, labels(n)[0] as type",
		},
		{
			"総エッジ数",
			"MATCH ()-[r]->() RETURN COUNT(r) as count, type(r) as relationship",
		},
		{
			"平均次数",
			`MATCH (u:User)
			 OPTIONAL MATCH (u)-[r]-()
			 WITH u, COUNT(r) as degree
			 RETURN AVG(degree) as avg_degree`,
		},
		{
			"クラスタリング係数",
			`MATCH (u:User)-[:FOLLOWS]-(neighbor)
			 WITH u, COLLECT(DISTINCT neighbor) as neighbors
			 WHERE SIZE(neighbors) >= 2
			 MATCH (n1)-[:FOLLOWS]-(n2)
			 WHERE n1 IN neighbors AND n2 IN neighbors AND n1 <> n2
			 WITH u, SIZE(neighbors) as k, COUNT(DISTINCT [n1, n2]) as edges
			 RETURN AVG(2.0 * edges / (k * (k - 1))) as clustering_coefficient`,
		},
	}

	for _, q := range queries {
		result, err := session.Run(ctx, q.query, nil)
		if err != nil {
			continue
		}

		fmt.Printf("\n  %s:\n", q.name)
		for result.Next(ctx) {
			record := result.Record()
			for i, value := range record.Values {
				key := record.Keys[i]
				fmt.Printf("    %s: %v\n", key, value)
			}
		}
	}
}

// RecommendationEngine - 推薦エンジンデモ
func (n *Neo4jGraph) RecommendationEngine(ctx context.Context) {
	fmt.Println("\n🎯 推薦システムデモ")
	fmt.Println("=" + repeatString("=", 50))

	if n.driver == nil {
		fmt.Println("デモモード: 推薦システムシミュレーション")
		n.simulateRecommendations()
		return
	}

	session := n.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	// 協調フィルタリング（ユーザーベース）
	fmt.Println("\n📍 協調フィルタリング推薦:")
	result, err := session.Run(ctx, `
		MATCH (u:User {id: 'user_0_0'})-[:LIKED]->(p:Post)<-[:LIKED]-(other:User)
		MATCH (other)-[:LIKED]->(rec:Post)
		WHERE NOT (u)-[:LIKED]->(rec)
		RETURN rec.id, rec.content, COUNT(*) as score
		ORDER BY score DESC
		LIMIT 5
	`, nil)

	if err == nil {
		for result.Next(ctx) {
			record := result.Record()
			fmt.Printf("  推薦: %v (スコア: %v)\n", record.Values[0], record.Values[2])
		}
	}

	// グラフベース推薦
	fmt.Println("\n📍 グラフベース推薦:")
	result, err = session.Run(ctx, `
		MATCH (u:User {id: 'user_0_0'})
		MATCH (u)-[:FOLLOWS*2..3]-(friend:User)
		MATCH (friend)-[:POSTED]->(p:Post)
		WHERE NOT (u)-[:LIKED]->(p)
		RETURN DISTINCT p.id, p.content, COUNT(friend) as relevance
		ORDER BY relevance DESC
		LIMIT 5
	`, nil)

	if err == nil {
		for result.Next(ctx) {
			record := result.Record()
			fmt.Printf("  推薦: %v (関連度: %v)\n", record.Values[0], record.Values[2])
		}
	}
}

// runDemoMode - デモモード実行
func (n *Neo4jGraph) runDemoMode(ctx context.Context) {
	fmt.Println("\n🎭 デモモード: Neo4jグラフ処理シミュレーション")

	// 仮想グラフ構造
	type VirtualGraph struct {
		mu    sync.RWMutex
		nodes map[string]interface{}
		edges map[string][]string
	}

	graph := &VirtualGraph{
		nodes: make(map[string]interface{}),
		edges: make(map[string][]string),
	}

	// 並列ノード生成
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				nodeID := fmt.Sprintf("node_%d_%d", id, j)

				graph.mu.Lock()
				graph.nodes[nodeID] = map[string]interface{}{
					"type":  "user",
					"score": float64(id*10 + j),
				}

				// エッジ作成
				if j > 0 {
					prevID := fmt.Sprintf("node_%d_%d", id, j-1)
					graph.edges[nodeID] = append(graph.edges[nodeID], prevID)
				}
				graph.mu.Unlock()
			}

			fmt.Printf("  Worker %d: 10ノード作成\n", id)
		}(i)
	}

	wg.Wait()

	graph.mu.RLock()
	fmt.Printf("\n  ✓ デモ完了: %d ノード, %d エッジ\n",
		len(graph.nodes), len(graph.edges))
	graph.mu.RUnlock()
}

// simulateRecommendations - 推薦シミュレーション
func (n *Neo4jGraph) simulateRecommendations() {
	items := []string{"Item A", "Item B", "Item C", "Item D", "Item E"}
	scores := []float64{0.95, 0.87, 0.82, 0.76, 0.71}

	fmt.Println("\n  推薦アイテム:")
	for i, item := range items {
		fmt.Printf("    %d. %s (信頼度: %.2f)\n", i+1, item, scores[i])
	}
}

// Close - リソースクリーンアップ
func (n *Neo4jGraph) Close(ctx context.Context) error {
	if n.driver != nil {
		return n.driver.Close(ctx)
	}
	return nil
}