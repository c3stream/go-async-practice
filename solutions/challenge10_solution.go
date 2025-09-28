package solutions

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge10_MessageOrderingSolution - メッセージ順序保証問題の解決
func Challenge10_MessageOrderingSolution() {
	fmt.Println("\n✅ チャレンジ10: メッセージ順序保証問題の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: ベクタークロックによる因果順序保証
	solution1_VectorClock()

	// 解決策2: ランポートタイムスタンプによる全順序
	solution2_LamportTimestamp()

	// 解決策3: バッファリングとシーケンス番号
	solution3_SequenceNumberBuffer()
}

// 解決策1: ベクタークロックによる因果順序保証
func solution1_VectorClock() {
	fmt.Println("\n📝 解決策1: ベクタークロックによる因果順序保証")

	type VectorClock map[string]int

	type Message struct {
		ID       string
		Sender   string
		Content  string
		VC       VectorClock
		Received time.Time
	}

	type Node struct {
		ID       string
		VC       VectorClock
		mu       sync.RWMutex
		inbox    chan Message
		outbox   chan Message
		delivered []Message
	}

	// ベクタークロックの初期化
	newVectorClock := func(nodeIDs []string) VectorClock {
		vc := make(VectorClock)
		for _, id := range nodeIDs {
			vc[id] = 0
		}
		return vc
	}

	// ベクタークロックのコピー
	copyVectorClock := func(vc VectorClock) VectorClock {
		copy := make(VectorClock)
		for k, v := range vc {
			copy[k] = v
		}
		return copy
	}

	// ベクタークロックの比較（a <= b）
	isLessOrEqual := func(a, b VectorClock) bool {
		for k, v := range a {
			if v > b[k] {
				return false
			}
		}
		return true
	}

	// 因果関係のあるメッセージかチェック
	isCausallyReady := func(msg Message, node *Node) bool {
		node.mu.RLock()
		defer node.mu.RUnlock()

		// 送信者のカウントが1つだけ進んでいる
		expectedVC := copyVectorClock(node.VC)
		expectedVC[msg.Sender]++

		// メッセージのVCが期待値と一致するかチェック
		if msg.VC[msg.Sender] != expectedVC[msg.Sender] {
			return false
		}

		// 他のノードのカウントは自分のVC以下である必要がある
		for k, v := range msg.VC {
			if k != msg.Sender && v > node.VC[k] {
				return false
			}
		}

		return true
	}

	// ノード作成
	createNode := func(id string, nodeIDs []string) *Node {
		return &Node{
			ID:        id,
			VC:        newVectorClock(nodeIDs),
			inbox:     make(chan Message, 100),
			outbox:    make(chan Message, 100),
			delivered: make([]Message, 0),
		}
	}

	// メッセージ送信
	sendMessage := func(node *Node, content string, broadcast chan Message) {
		node.mu.Lock()
		node.VC[node.ID]++
		vc := copyVectorClock(node.VC)
		node.mu.Unlock()

		msg := Message{
			ID:      fmt.Sprintf("%s_%d", node.ID, vc[node.ID]),
			Sender:  node.ID,
			Content: content,
			VC:      vc,
		}

		broadcast <- msg
		fmt.Printf("  📤 %s sent: %s (VC: %v)\n", node.ID, content, vc)
	}

	// メッセージ受信処理
	processMessages := func(node *Node, wg *sync.WaitGroup) {
		defer wg.Done()

		pending := make([]Message, 0)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case msg := <-node.inbox:
				if msg.Content == "STOP" {
					return
				}

				// 因果順序のチェック
				if isCausallyReady(msg, node) {
					// メッセージを配信
					node.mu.Lock()
					for k, v := range msg.VC {
						if v > node.VC[k] {
							node.VC[k] = v
						}
					}
					node.delivered = append(node.delivered, msg)
					node.mu.Unlock()

					fmt.Printf("  ✓ %s delivered: %s from %s\n",
						node.ID, msg.Content, msg.Sender)

					// ペンディングメッセージの再チェック
					newPending := make([]Message, 0)
					for _, pm := range pending {
						if isCausallyReady(pm, node) {
							node.mu.Lock()
							for k, v := range pm.VC {
								if v > node.VC[k] {
									node.VC[k] = v
								}
							}
							node.delivered = append(node.delivered, pm)
							node.mu.Unlock()

							fmt.Printf("  ✓ %s delivered (from pending): %s from %s\n",
								node.ID, pm.Content, pm.Sender)
						} else {
							newPending = append(newPending, pm)
						}
					}
					pending = newPending
				} else {
					// ペンディングに追加
					pending = append(pending, msg)
					fmt.Printf("  ⏳ %s pending: %s from %s\n",
						node.ID, msg.Content, msg.Sender)
				}

			case <-ticker.C:
				// 定期的にペンディングメッセージをチェック
				if len(pending) > 0 {
					fmt.Printf("  📋 %s has %d pending messages\n", node.ID, len(pending))
				}
			}
		}
	}

	// テスト実行
	nodeIDs := []string{"A", "B", "C"}
	nodes := make(map[string]*Node)
	broadcast := make(chan Message, 100)

	// ノード作成と起動
	var wg sync.WaitGroup
	for _, id := range nodeIDs {
		nodes[id] = createNode(id, nodeIDs)
		wg.Add(1)
		go processMessages(nodes[id], &wg)
	}

	// ブロードキャスト配信
	go func() {
		for msg := range broadcast {
			for _, node := range nodes {
				if node.ID != msg.Sender {
					node.inbox <- msg
				}
			}
		}
	}()

	// シナリオ: 因果関係のあるメッセージ送信
	time.Sleep(100 * time.Millisecond)

	// A → B → C の順序で送信
	sendMessage(nodes["A"], "Hello from A", broadcast)
	time.Sleep(50 * time.Millisecond)

	sendMessage(nodes["B"], "Reply from B", broadcast)
	time.Sleep(50 * time.Millisecond)

	sendMessage(nodes["C"], "Reply from C", broadcast)
	time.Sleep(200 * time.Millisecond)

	// 停止
	for _, node := range nodes {
		node.inbox <- Message{Content: "STOP"}
	}
	wg.Wait()

	// 配信されたメッセージの確認
	fmt.Println("\n配信結果:")
	for id, node := range nodes {
		fmt.Printf("  %s: %d messages delivered\n", id, len(node.delivered))
	}
}

// 解決策2: ランポートタイムスタンプによる全順序
func solution2_LamportTimestamp() {
	fmt.Println("\n📝 解決策2: ランポートタイムスタンプによる全順序")

	type Message struct {
		ID        string
		Sender    string
		Content   string
		Timestamp int64
		Delivered bool
	}

	type Process struct {
		ID        string
		clock     atomic.Int64
		messages  []Message
		mu        sync.Mutex
	}

	// プロセス作成
	createProcess := func(id string) *Process {
		return &Process{
			ID:       id,
			messages: make([]Message, 0),
		}
	}

	// イベント発生時のクロック更新
	tick := func(p *Process) int64 {
		return p.clock.Add(1)
	}

	// メッセージ送信
	send := func(p *Process, content string) Message {
		ts := tick(p)
		msg := Message{
			ID:        fmt.Sprintf("%s_%d", p.ID, ts),
			Sender:    p.ID,
			Content:   content,
			Timestamp: ts,
		}

		p.mu.Lock()
		p.messages = append(p.messages, msg)
		p.mu.Unlock()

		fmt.Printf("  📤 %s sent: %s (TS: %d)\n", p.ID, content, ts)
		return msg
	}

	// メッセージ受信
	receive := func(p *Process, msg Message) {
		// Lamport's clock rule: max(local, received) + 1
		for {
			local := p.clock.Load()
			newClock := msg.Timestamp
			if local >= newClock {
				newClock = local
			}
			if p.clock.CompareAndSwap(local, newClock+1) {
				break
			}
		}

		p.mu.Lock()
		p.messages = append(p.messages, msg)
		p.mu.Unlock()

		fmt.Printf("  📥 %s received: %s from %s (TS: %d, Clock: %d)\n",
			p.ID, msg.Content, msg.Sender, msg.Timestamp, p.clock.Load())
	}

	// 全メッセージを全順序でソート
	getTotalOrder := func(processes []*Process) []Message {
		allMessages := make([]Message, 0)
		seen := make(map[string]bool)

		for _, p := range processes {
			p.mu.Lock()
			for _, msg := range p.messages {
				if !seen[msg.ID] {
					allMessages = append(allMessages, msg)
					seen[msg.ID] = true
				}
			}
			p.mu.Unlock()
		}

		// タイムスタンプでソート（同じ場合はプロセスIDで決定）
		sort.Slice(allMessages, func(i, j int) bool {
			if allMessages[i].Timestamp == allMessages[j].Timestamp {
				return allMessages[i].Sender < allMessages[j].Sender
			}
			return allMessages[i].Timestamp < allMessages[j].Timestamp
		})

		return allMessages
	}

	// テスト実行
	processes := []*Process{
		createProcess("P1"),
		createProcess("P2"),
		createProcess("P3"),
	}

	// メッセージ交換シナリオ
	var wg sync.WaitGroup

	// P1からメッセージ送信
	msg1 := send(processes[0], "Initial message")

	// P2とP3が並行して受信して返信
	wg.Add(2)
	go func() {
		defer wg.Done()
		time.Sleep(50 * time.Millisecond)
		receive(processes[1], msg1)
		msg2 := send(processes[1], "Response from P2")
		receive(processes[2], msg2)
	}()

	go func() {
		defer wg.Done()
		time.Sleep(30 * time.Millisecond)
		receive(processes[2], msg1)
		msg3 := send(processes[2], "Response from P3")
		receive(processes[0], msg3)
	}()

	wg.Wait()

	// 全順序の表示
	fmt.Println("\n全順序での配信:")
	totalOrder := getTotalOrder(processes)
	for i, msg := range totalOrder {
		fmt.Printf("  %d. %s: %s (TS: %d)\n",
			i+1, msg.Sender, msg.Content, msg.Timestamp)
	}
}

// 解決策3: バッファリングとシーケンス番号
func solution3_SequenceNumberBuffer() {
	fmt.Println("\n📝 解決策3: バッファリングとシーケンス番号")

	type Message struct {
		PartitionKey string
		SequenceNum  int64
		Content      string
		Timestamp    time.Time
	}

	type OrderedBuffer struct {
		mu            sync.RWMutex
		buffers       map[string]*PartitionBuffer
		deliveryChan  chan Message
	}

	type PartitionBuffer struct {
		nextSeq      int64
		pending      map[int64]Message
		maxBufferSize int
		lastDelivery time.Time
		timeout      time.Duration
	}

	// バッファ作成
	createOrderedBuffer := func() *OrderedBuffer {
		return &OrderedBuffer{
			buffers:      make(map[string]*PartitionBuffer),
			deliveryChan: make(chan Message, 100),
		}
	}

	// メッセージ追加
	addMessage := func(ob *OrderedBuffer, msg Message) {
		ob.mu.Lock()
		defer ob.mu.Unlock()

		// パーティションバッファの取得または作成
		if _, exists := ob.buffers[msg.PartitionKey]; !exists {
			ob.buffers[msg.PartitionKey] = &PartitionBuffer{
				nextSeq:       1,
				pending:       make(map[int64]Message),
				maxBufferSize: 100,
				lastDelivery:  time.Now(),
				timeout:       500 * time.Millisecond,
			}
		}

		pb := ob.buffers[msg.PartitionKey]

		// 期待するシーケンス番号の場合はすぐに配信
		if msg.SequenceNum == pb.nextSeq {
			ob.deliveryChan <- msg
			pb.nextSeq++
			pb.lastDelivery = time.Now()

			fmt.Printf("  ✓ Delivered immediately: Key=%s, Seq=%d, Content=%s\n",
				msg.PartitionKey, msg.SequenceNum, msg.Content)

			// ペンディングから連続するメッセージを配信
			for {
				if pendingMsg, exists := pb.pending[pb.nextSeq]; exists {
					ob.deliveryChan <- pendingMsg
					delete(pb.pending, pb.nextSeq)
					pb.nextSeq++
					pb.lastDelivery = time.Now()

					fmt.Printf("  ✓ Delivered from buffer: Key=%s, Seq=%d, Content=%s\n",
						pendingMsg.PartitionKey, pendingMsg.SequenceNum, pendingMsg.Content)
				} else {
					break
				}
			}
		} else if msg.SequenceNum > pb.nextSeq {
			// 将来のメッセージはバッファに保存
			pb.pending[msg.SequenceNum] = msg
			fmt.Printf("  ⏳ Buffered: Key=%s, Seq=%d (expecting %d), Content=%s\n",
				msg.PartitionKey, msg.SequenceNum, pb.nextSeq, msg.Content)
		} else {
			// 既に配信済みのシーケンス番号
			fmt.Printf("  ⚠️ Duplicate or old: Key=%s, Seq=%d, Content=%s\n",
				msg.PartitionKey, msg.SequenceNum, msg.Content)
		}
	}

	// タイムアウトチェック
	checkTimeouts := func(ob *OrderedBuffer, ctx context.Context) {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				ob.mu.Lock()
				now := time.Now()

				for key, pb := range ob.buffers {
					// タイムアウトチェック
					if now.Sub(pb.lastDelivery) > pb.timeout && len(pb.pending) > 0 {
						// 欠落メッセージをスキップ
						fmt.Printf("  ⏱️ Timeout for partition %s, skipping to next available\n", key)

						// 最小のペンディングシーケンス番号を探す
						minSeq := int64(-1)
						for seq := range pb.pending {
							if minSeq == -1 || seq < minSeq {
								minSeq = seq
							}
						}

						if minSeq != -1 {
							// 欠落をスキップ
							pb.nextSeq = minSeq
							msg := pb.pending[minSeq]
							ob.deliveryChan <- msg
							delete(pb.pending, minSeq)
							pb.nextSeq++
							pb.lastDelivery = now

							fmt.Printf("  ⏩ Skipped to: Key=%s, Seq=%d\n", key, minSeq)
						}
					}

					// バッファサイズチェック
					if len(pb.pending) > pb.maxBufferSize {
						fmt.Printf("  ⚠️ Buffer overflow for partition %s, dropping old messages\n", key)
						// 古いメッセージを削除
						for seq := range pb.pending {
							if seq < pb.nextSeq + int64(pb.maxBufferSize/2) {
								delete(pb.pending, seq)
							}
						}
					}
				}

				ob.mu.Unlock()

			case <-ctx.Done():
				return
			}
		}
	}

	// テスト実行
	buffer := createOrderedBuffer()
	ctx, cancel := context.WithCancel(context.Background())

	// タイムアウトチェッカー起動
	go checkTimeouts(buffer, ctx)

	// 配信されたメッセージの受信
	go func() {
		for msg := range buffer.deliveryChan {
			_ = msg // 実際の処理
		}
	}()

	// シミュレーション: 順序が入れ替わったメッセージ
	messages := []Message{
		{PartitionKey: "user1", SequenceNum: 1, Content: "First"},
		{PartitionKey: "user1", SequenceNum: 3, Content: "Third"},  // 順序逆転
		{PartitionKey: "user1", SequenceNum: 2, Content: "Second"},
		{PartitionKey: "user2", SequenceNum: 1, Content: "User2-First"},
		{PartitionKey: "user1", SequenceNum: 4, Content: "Fourth"},
		{PartitionKey: "user2", SequenceNum: 3, Content: "User2-Third"}, // 欠落あり
		{PartitionKey: "user1", SequenceNum: 5, Content: "Fifth"},
	}

	// メッセージ送信
	for _, msg := range messages {
		addMessage(buffer, msg)
		time.Sleep(50 * time.Millisecond)
	}

	// タイムアウトテスト
	time.Sleep(600 * time.Millisecond)

	// 最終状態の確認
	buffer.mu.RLock()
	fmt.Println("\n最終バッファ状態:")
	for key, pb := range buffer.buffers {
		fmt.Printf("  Partition %s: NextSeq=%d, Pending=%d messages\n",
			key, pb.nextSeq, len(pb.pending))
	}
	buffer.mu.RUnlock()

	cancel()
}