package solutions

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Solution06_FixedResourceLeak - リソースリーク問題の解決版
func Solution06_FixedResourceLeak() {
	fmt.Println("\n🔧 解答6: リソースリークの修正")
	fmt.Println("===================================================")

	// 解法1: HTTP クライアントの適切な管理
	solution1HTTPClient()

	// 解法2: ファイルハンドルの適切な管理
	solution2FileHandles()

	// 解法3: データベース接続の適切な管理
	solution3DatabaseConnections()

	// 解法4: ゴルーチンとチャネルのリソース管理
	solution4GoroutineResources()
}

// 解法1: HTTP クライアントの適切な管理
func solution1HTTPClient() {
	fmt.Println("\n📌 解法1: HTTP クライアントとレスポンスの適切な管理")

	// カスタム Transport でタイムアウトと接続数を制御
	transport := &http.Transport{
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     30 * time.Second,
		DisableKeepAlives:   false, // Keep-Alive を有効化して接続を再利用
	}

	// タイムアウト付きクライアント
	client := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second,
	}

	// リクエスト実行関数
	makeRequest := func(url string) error {
		// Context でタイムアウト制御
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// リクエスト作成
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return fmt.Errorf("リクエスト作成エラー: %w", err)
		}

		// リクエスト実行
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("リクエスト実行エラー: %w", err)
		}
		// 重要: レスポンスボディを必ずクローズ
		defer func() {
			// ボディを読み切ってからクローズ
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}()

		// レスポンス処理
		fmt.Printf("  ✓ リクエスト成功: Status=%d\n", resp.StatusCode)
		return nil
	}

	// 並行リクエスト処理
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 3) // 同時接続数を制限

	urls := []string{
		"https://httpbin.org/delay/1",
		"https://httpbin.org/status/200",
		"https://httpbin.org/get",
	}

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			semaphore <- struct{}{}        // セマフォ取得
			defer func() { <-semaphore }() // セマフォ解放

			if err := makeRequest(u); err != nil {
				fmt.Printf("  ⚠ エラー: %v\n", err)
			}
		}(url)
	}

	wg.Wait()

	// Transport をクリーンアップ
	transport.CloseIdleConnections()
	fmt.Println("  ✅ すべての HTTP 接続が適切にクローズされました")
}

// 解法2: ファイルハンドルの適切な管理
func solution2FileHandles() {
	fmt.Println("\n📌 解法2: ファイルハンドルの適切な管理")

	// パターン1: defer を使った確実なクローズ
	processFileWithDefer := func(filename string) error {
		file, err := os.CreateTemp("", filename)
		if err != nil {
			return err
		}
		defer func() {
			file.Close()
			os.Remove(file.Name()) // テンポラリファイルを削除
			fmt.Printf("  ✓ ファイル %s をクローズしました\n", file.Name())
		}()

		// ファイル操作
		_, err = file.WriteString("テストデータ\n")
		return err
	}

	// パターン2: 複数ファイルの一括処理
	processMultipleFiles := func() error {
		const numFiles = 5
		files := make([]*os.File, 0, numFiles)

		// クリーンアップ関数
		cleanup := func() {
			for _, f := range files {
				if f != nil {
					f.Close()
					os.Remove(f.Name())
				}
			}
		}
		defer cleanup()

		// ファイル作成
		for i := 0; i < numFiles; i++ {
			file, err := os.CreateTemp("", fmt.Sprintf("temp_%d_*.txt", i))
			if err != nil {
				return err
			}
			files = append(files, file)
		}

		fmt.Printf("  ✓ %d 個のファイルを作成\n", numFiles)

		// ファイル処理
		for i, file := range files {
			_, err := file.WriteString(fmt.Sprintf("File %d content\n", i))
			if err != nil {
				return err
			}
		}

		fmt.Printf("  ✓ すべてのファイルに書き込み完了\n")
		return nil
	}

	// パターン3: ファイルプールの実装
	type FilePool struct {
		mu       sync.Mutex
		files    map[string]*os.File
		maxFiles int
	}

	pool := &FilePool{
		files:    make(map[string]*os.File),
		maxFiles: 10,
	}

	poolOpen := func(name string) (*os.File, error) {
		pool.mu.Lock()
		defer pool.mu.Unlock()

		if f, exists := pool.files[name]; exists {
			return f, nil
		}

		if len(pool.files) >= pool.maxFiles {
			// 最も古いファイルをクローズ
			for n, f := range pool.files {
				f.Close()
				delete(pool.files, n)
				fmt.Printf("  ⚠ ファイルプール制限によりクローズ: %s\n", n)
				break
			}
		}

		file, err := os.CreateTemp("", name)
		if err != nil {
			return nil, err
		}

		pool.files[name] = file
		return file, nil
	}

	poolCloseAll := func() {
		pool.mu.Lock()
		defer pool.mu.Unlock()

		for name, f := range pool.files {
			f.Close()
			os.Remove(f.Name())
			fmt.Printf("  ✓ プールファイルをクローズ: %s\n", name)
		}
		pool.files = make(map[string]*os.File)
	}

	// 実行
	processFileWithDefer("test1.txt")
	processMultipleFiles()

	// プールを使用
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("pooled_%d", i)
		if f, err := poolOpen(name); err == nil {
			f.WriteString("pooled data\n")
		}
	}
	poolCloseAll()

	fmt.Println("  ✅ すべてのファイルハンドルが適切に管理されました")
}

// FilePool のメソッド定義
type FilePool struct {
	mu       sync.Mutex
	files    map[string]*os.File
	maxFiles int
	Open     func(string) (*os.File, error)
	CloseAll func()
}

// 解法3: データベース接続の適切な管理
func solution3DatabaseConnections() {
	fmt.Println("\n📌 解法3: データベース接続プールの適切な管理")

	// 模擬データベース接続
	type DBConnection struct {
		ID       int
		InUse    bool
		LastUsed time.Time
		mu       sync.Mutex
	}

	// コネクションプール
	type ConnectionPool struct {
		connections []*DBConnection
		maxConn     int
		maxIdleTime time.Duration
		mu          sync.RWMutex
	}

	// プール作成
	pool := &ConnectionPool{
		maxConn:     10,
		maxIdleTime: 30 * time.Second,
	}

	// 初期化
	poolInit := func() {
		pool.mu.Lock()
		defer pool.mu.Unlock()

		pool.connections = make([]*DBConnection, pool.maxConn)
		for i := 0; i < pool.maxConn; i++ {
			pool.connections[i] = &DBConnection{
				ID:       i,
				InUse:    false,
				LastUsed: time.Now(),
			}
		}
		fmt.Printf("  ✓ 接続プール初期化: %d 接続\n", pool.maxConn)
	}

	// 接続取得
	poolGet := func() (*DBConnection, error) {
		pool.mu.RLock()
		defer pool.mu.RUnlock()

		for _, conn := range pool.connections {
			conn.mu.Lock()
			if !conn.InUse {
				conn.InUse = true
				conn.LastUsed = time.Now()
				conn.mu.Unlock()
				return conn, nil
			}
			conn.mu.Unlock()
		}

		return nil, fmt.Errorf("利用可能な接続がありません")
	}

	// 接続返却
	poolPut := func(conn *DBConnection) {
		conn.mu.Lock()
		conn.InUse = false
		conn.LastUsed = time.Now()
		conn.mu.Unlock()
		fmt.Printf("  ✓ 接続 %d を返却\n", conn.ID)
	}

	// アイドル接続のクリーンアップ
	poolCleanupIdle := func() {
		pool.mu.RLock()
		defer pool.mu.RUnlock()

		now := time.Now()
		cleaned := 0

		for _, conn := range pool.connections {
			conn.mu.Lock()
			if !conn.InUse && now.Sub(conn.LastUsed) > pool.maxIdleTime {
				// アイドル接続をリセット（実際にはクローズと再接続）
				conn.LastUsed = now
				cleaned++
			}
			conn.mu.Unlock()
		}

		if cleaned > 0 {
			fmt.Printf("  ✓ %d 個のアイドル接続をクリーンアップ\n", cleaned)
		}
	}

	// 統計情報
	poolStats := func() {
		pool.mu.RLock()
		defer pool.mu.RUnlock()

		inUse := 0
		for _, conn := range pool.connections {
			if conn.InUse {
				inUse++
			}
		}

		fmt.Printf("  📊 プール統計: 使用中=%d, アイドル=%d, 合計=%d\n",
			inUse, pool.maxConn-inUse, pool.maxConn)
	}

	// プール初期化
	poolInit()

	// 並行処理でデータベース操作をシミュレート
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 接続取得
			conn, err := poolGet()
			if err != nil {
				fmt.Printf("  ⚠ タスク %d: %v\n", id, err)
				return
			}

			// データベース操作のシミュレーション
			time.Sleep(50 * time.Millisecond)

			// 接続返却
			poolPut(conn)
		}(i)
	}

	// 定期的なクリーンアップ
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for i := 0; i < 3; i++ {
			<-ticker.C
			poolCleanupIdle()
			poolStats()
		}
	}()

	wg.Wait()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("  ✅ データベース接続プールが適切に管理されました")
}

// ConnectionPool のメソッド定義
type ConnectionPool struct {
	connections []*DBConnection
	maxConn     int
	maxIdleTime time.Duration
	mu          sync.RWMutex
	Init        func()
	Get         func() (*DBConnection, error)
	Put         func(*DBConnection)
	CleanupIdle func()
	Stats       func()
}

type DBConnection struct {
	ID       int
	InUse    bool
	LastUsed time.Time
	mu       sync.Mutex
}

// 解法4: ゴルーチンとチャネルのリソース管理
func solution4GoroutineResources() {
	fmt.Println("\n📌 解法4: ゴルーチンとチャネルのリソース管理")

	// リソースマネージャー
	type ResourceManager struct {
		ctx      context.Context
		cancel   context.CancelFunc
		wg       sync.WaitGroup
		channels []chan interface{}
		mu       sync.Mutex
	}

	manager := &ResourceManager{
		channels: make([]chan interface{}, 0),
	}
	manager.ctx, manager.cancel = context.WithCancel(context.Background())

	// チャネル登録
	managerRegisterChannel := func() chan interface{} {
		manager.mu.Lock()
		defer manager.mu.Unlock()

		ch := make(chan interface{}, 10)
		manager.channels = append(manager.channels, ch)
		return ch
	}

	// ゴルーチン起動
	managerStartGoroutine := func(f func(context.Context)) {
		manager.wg.Add(1)
		go func() {
			defer manager.wg.Done()
			f(manager.ctx)
		}()
	}

	// クリーンアップ
	managerCleanup := func() {
		fmt.Println("  クリーンアップ開始...")

		// Context をキャンセル
		manager.cancel()

		// すべてのチャネルをクローズ
		manager.mu.Lock()
		for _, ch := range manager.channels {
			close(ch)
		}
		manager.mu.Unlock()

		// ゴルーチンの終了を待つ
		done := make(chan struct{})
		go func() {
			manager.wg.Wait()
			close(done)
		}()

		// タイムアウト付き待機
		select {
		case <-done:
			fmt.Println("  ✓ すべてのゴルーチンが正常終了")
		case <-time.After(1 * time.Second):
			fmt.Println("  ⚠ タイムアウト: 一部のゴルーチンが終了していない可能性")
		}
	}

	// リソースを使用
	for i := 0; i < 3; i++ {
		ch := managerRegisterChannel()
		workerID := i

		managerStartGoroutine(func(ctx context.Context) {
			fmt.Printf("  ワーカー %d: 開始\n", workerID)
			for {
				select {
				case <-ctx.Done():
					fmt.Printf("  ワーカー %d: 終了\n", workerID)
					return
				case data := <-ch:
					if data != nil {
						// データ処理
						time.Sleep(10 * time.Millisecond)
					}
				}
			}
		})

		// データ送信
		go func(ch chan interface{}) {
			for j := 0; j < 3; j++ {
				select {
				case ch <- j:
				case <-manager.ctx.Done():
					return
				}
			}
		}(ch)
	}

	// 処理時間
	time.Sleep(100 * time.Millisecond)

	// クリーンアップ実行
	managerCleanup()

	fmt.Println("  ✅ すべてのリソースが適切に解放されました")
}

// ResourceManager のメソッド定義
type ResourceManager struct {
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	channels        []chan interface{}
	mu              sync.Mutex
	RegisterChannel func() chan interface{}
	StartGoroutine  func(func(context.Context))
	Cleanup         func()
}

// Solution06_BestPractices - リソース管理のベストプラクティス
func Solution06_BestPractices() {
	fmt.Println("\n📚 リソース管理のベストプラクティス")
	fmt.Println("===================================================")

	practices := []string{
		"1. defer を使って確実にリソースをクローズ",
		"2. Context を使ってゴルーチンのライフサイクル管理",
		"3. sync.Pool を使ってオブジェクトの再利用",
		"4. セマフォパターンで同時接続数を制限",
		"5. タイムアウトを設定してリソースの占有を防ぐ",
		"6. エラーハンドリングでリソースリークを防ぐ",
		"7. 定期的なクリーンアップでアイドルリソースを解放",
		"8. メトリクスでリソース使用状況を監視",
	}

	for _, practice := range practices {
		fmt.Printf("  ✓ %s\n", practice)
		time.Sleep(50 * time.Millisecond)
	}

	// リソースリークのチェックリスト
	fmt.Println("\n🔍 リソースリークチェックリスト:")
	checklist := []string{
		"HTTP レスポンスボディはクローズされているか？",
		"ファイルハンドルは適切にクローズされているか？",
		"データベース接続は返却されているか？",
		"ゴルーチンは適切に終了しているか？",
		"チャネルはクローズされているか？",
		"time.Ticker は Stop されているか？",
		"Context はキャンセルされているか？",
	}

	for _, item := range checklist {
		fmt.Printf("  ☐ %s\n", item)
	}
}

// SQL ダミーインポート処理
func init() {
	// sql パッケージの使用を示すダミー
	_ = sql.ErrNoRows
}