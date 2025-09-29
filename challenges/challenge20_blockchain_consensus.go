package challenges

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Challenge 20: Blockchain-inspired Consensus Mechanism
//
// 問題点:
// 1. コンセンサスアルゴリズムのバグ
// 2. フォーク処理の不備
// 3. ネットワークパーティション対応なし
// 4. Byzantine Fault Tolerance未実装
// 5. マイニング競争の同期問題

type Block struct {
	Index        int64
	Timestamp    time.Time
	Data         []byte
	PreviousHash string
	Hash         string
	Nonce        int64
	Miner        string
	Difficulty   int
}

type Node struct {
	ID         string
	Blockchain []Block
	Peers      map[string]*Node
	Mining     bool
	mu         sync.RWMutex
	mempool    []Transaction
	consensus  *ConsensusEngine
}

type Transaction struct {
	ID        string
	From      string
	To        string
	Amount    float64
	Timestamp time.Time
	Signature string
}

type ConsensusEngine struct {
	mu              sync.RWMutex
	nodes           map[string]*Node
	difficulty      int
	blockTime       time.Duration
	validatorSet    map[string]bool
	currentLeader   string
	epoch           int64
	pendingBlocks   chan Block
	forkResolution  sync.Map // 問題: メモリリーク
}

func NewConsensusEngine() *ConsensusEngine {
	return &ConsensusEngine{
		nodes:         make(map[string]*Node),
		difficulty:    2, // 低い難易度で開始
		blockTime:     5 * time.Second,
		validatorSet:  make(map[string]bool),
		pendingBlocks: make(chan Block, 100),
	}
}

// 問題1: ブロック生成の競合
func (n *Node) MineBlock(ctx context.Context, transactions []Transaction) (*Block, error) {
	n.mu.RLock()
	lastBlock := n.Blockchain[len(n.Blockchain)-1]
	n.mu.RUnlock()

	newBlock := Block{
		Index:        lastBlock.Index + 1,
		Timestamp:    time.Now(),
		Data:         serializeTransactions(transactions),
		PreviousHash: lastBlock.Hash,
		Miner:        n.ID,
		Difficulty:   n.consensus.difficulty,
	}

	// 問題: マイニング中の状態管理
	n.Mining = true // 問題: muロックなしで書き込み
	defer func() {
		n.Mining = false // 問題: defer内でもロックなし
	}()

	// 問題: 非効率なProof of Work
	for nonce := int64(0); ; nonce++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			newBlock.Nonce = nonce
			hash := calculateHash(newBlock)

			// 問題: 難易度チェックが不正確
			if isValidHash(hash, n.consensus.difficulty) {
				newBlock.Hash = hash
				return &newBlock, nil
			}
		}
	}
}

// 問題2: ブロック検証の不備
func (n *Node) ValidateBlock(block Block) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if len(n.Blockchain) == 0 {
		return false
	}

	lastBlock := n.Blockchain[len(n.Blockchain)-1]

	// 問題: 不完全な検証
	if block.Index != lastBlock.Index+1 {
		return false
	}

	if block.PreviousHash != lastBlock.Hash {
		return false
	}

	// 問題: ハッシュ再計算の検証なし
	// 問題: タイムスタンプの検証なし
	// 問題: トランザクションの検証なし

	return true
}

// 問題3: チェーン同期のバグ
func (n *Node) SyncChain(peer *Node) error {
	peer.mu.RLock()
	peerChain := make([]Block, len(peer.Blockchain))
	copy(peerChain, peer.Blockchain)
	peer.mu.RUnlock()

	n.mu.Lock()
	defer n.mu.Unlock()

	// 問題: 単純な最長チェーンルール
	if len(peerChain) > len(n.Blockchain) {
		// 問題: 検証なしでチェーンを置き換え
		n.Blockchain = peerChain
		return nil
	}

	// 問題: フォーク解決ロジックなし

	return fmt.Errorf("peer chain is not longer")
}

// 問題4: ブロードキャストの問題
func (ce *ConsensusEngine) BroadcastBlock(block Block) {
	ce.mu.RLock()
	nodes := make([]*Node, 0, len(ce.nodes))
	for _, node := range ce.nodes {
		nodes = append(nodes, node)
	}
	ce.mu.RUnlock()

	// 問題: 同期的ブロードキャスト
	for _, node := range nodes {
		// 問題: エラーハンドリングなし
		node.AddBlock(block)
	}
}

// 問題5: ブロック追加の競合状態
func (n *Node) AddBlock(block Block) error {
	// 問題: 二重追加チェックなし
	if !n.ValidateBlock(block) {
		return fmt.Errorf("invalid block")
	}

	n.mu.Lock()
	n.Blockchain = append(n.Blockchain, block)
	n.mu.Unlock()

	// 問題: mempool更新なし
	// トランザクションがmempoolから削除されない

	return nil
}

// 問題6: フォーク解決
func (ce *ConsensusEngine) ResolveForks() error {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	// 問題: 全ノードをチェック（O(n^2)の複雑さ）
	for _, node1 := range ce.nodes {
		for _, node2 := range ce.nodes {
			if node1.ID == node2.ID {
				continue
			}

			// 問題: フォーク検出ロジックが不完全
			node1.mu.RLock()
			node2.mu.RLock()

			if len(node1.Blockchain) != len(node2.Blockchain) {
				// 問題: 単純に長いチェーンを採用
				if len(node1.Blockchain) > len(node2.Blockchain) {
					// 問題: デッドロックの可能性
					node2.Blockchain = node1.Blockchain
				} else {
					node1.Blockchain = node2.Blockchain
				}
			}

			node2.mu.RUnlock()
			node1.mu.RUnlock()
		}
	}

	return nil
}

// 問題7: リーダー選出
func (ce *ConsensusEngine) ElectLeader() string {
	ce.mu.Lock()
	defer ce.mu.Unlock()

	// 問題: 単純すぎるリーダー選出
	for id := range ce.validatorSet {
		ce.currentLeader = id
		return id
	}

	// 問題: リーダーなしの状態
	return ""
}

// Utility functions
func calculateHash(block Block) string {
	record := fmt.Sprintf("%d%s%s%s%d%d",
		block.Index, block.Timestamp, block.Data,
		block.PreviousHash, block.Nonce, block.Difficulty)
	h := sha256.New()
	h.Write([]byte(record))
	return hex.EncodeToString(h.Sum(nil))
}

func isValidHash(hash string, difficulty int) bool {
	prefix := ""
	for i := 0; i < difficulty; i++ {
		prefix += "0"
	}
	return len(hash) >= difficulty && hash[:difficulty] == prefix
}

func serializeTransactions(transactions []Transaction) []byte {
	// 簡単なシリアライゼーション
	data := ""
	for _, tx := range transactions {
		data += fmt.Sprintf("%s:%s:%s:%f;", tx.ID, tx.From, tx.To, tx.Amount)
	}
	return []byte(data)
}

// Challenge: 以下の問題を修正してください
// 1. 適切なコンセンサスアルゴリズム (PBFT/Raft風)
// 2. フォーク解決メカニズム
// 3. ネットワークパーティション対応
// 4. 二重支払い防止
// 5. マークルツリー実装
// 6. ブロックファイナリティ
// 7. インセンティブメカニズム
// 8. 状態同期の最適化

func RunChallenge20() {
	fmt.Println("Challenge 20: Blockchain Consensus")
	fmt.Println("Fix the consensus mechanism issues")

	ce := NewConsensusEngine()
	ctx := context.Background()

	// ノード作成
	nodes := make([]*Node, 3)
	for i := 0; i < 3; i++ {
		node := &Node{
			ID:        fmt.Sprintf("node-%d", i),
			Blockchain: []Block{{
				Index:     0,
				Timestamp: time.Now(),
				Data:      []byte("Genesis Block"),
				Hash:      "0",
			}},
			Peers:     make(map[string]*Node),
			mempool:   []Transaction{},
			consensus: ce,
		}
		nodes[i] = node
		ce.nodes[node.ID] = node
		ce.validatorSet[node.ID] = true
	}

	// ピア接続
	for i, node := range nodes {
		for j, peer := range nodes {
			if i != j {
				node.Peers[peer.ID] = peer
			}
		}
	}

	// マイニング競争
	var wg sync.WaitGroup
	for _, node := range nodes {
		wg.Add(1)
		go func(n *Node) {
			defer wg.Done()

			transactions := []Transaction{
				{ID: "tx1", From: "Alice", To: "Bob", Amount: 100},
			}

			block, err := n.MineBlock(ctx, transactions)
			if err == nil && block != nil {
				fmt.Printf("Node %s mined block %d\n", n.ID, block.Index)
				ce.BroadcastBlock(*block)
			}
		}(node)
	}

	wg.Wait()

	// フォーク解決
	ce.ResolveForks()

	// 最終状態表示
	for _, node := range nodes {
		fmt.Printf("Node %s chain length: %d\n", node.ID, len(node.Blockchain))
	}
}