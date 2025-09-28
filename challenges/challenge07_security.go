package challenges

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Challenge07_SecurityIssue - セキュリティ問題
//
// 🎆 問題: このコードにはセキュリティ上の問題があります。並行処理における脆弱性を修正してください。
// 💡 ヒント: パスワード処理、タイミング攻撃、レース条件によるセキュリティホールを確認してください。
func Challenge07_SecurityIssue() {
	fmt.Println("\n╔════════════════════════════════════════╗")
	fmt.Println("║ 🎆 チャレンジ7: セキュリティ問題を修正  ║")
	fmt.Println("╚════════════════════════════════════════╝")
	fmt.Println("\n⚠️ 現在の状態: セキュリティホールがあります！")

	// 問題1: グローバル変数での機密情報管理（レース条件）
	var currentToken string
	var tokenMutex sync.Mutex

	// トークン生成（問題：予測可能な乱数）
	generateToken := func() string {
		// 問題: math/randは暗号学的に安全ではない
		rand.Seed(time.Now().UnixNano())
		token := fmt.Sprintf("token_%d", rand.Intn(1000000))
		return token
	}

	// 問題2: タイミング攻撃に脆弱な認証
	authenticate := func(password string) bool {
		correctPassword := "secret123"

		// 問題: 文字列比較が早期終了（タイミング攻撃可能）
		if len(password) != len(correctPassword) {
			return false
		}

		for i := 0; i < len(password); i++ {
			if password[i] != correctPassword[i] {
				return false // 早期終了でタイミングが変わる
			}
		}
		return true
	}

	// 問題3: 並行アクセスによる認証バイパス
	type Session struct {
		isAuthenticated bool
		userID         string
		// mutexがない！
	}

	session := &Session{}
	var wg sync.WaitGroup

	// 複数のゴルーチンが同時にセッションを更新（レース条件）
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// 問題: 同期なしでセッション状態を変更
			if id%2 == 0 {
				session.isAuthenticated = true
				session.userID = fmt.Sprintf("user%d", id)
				fmt.Printf("  🔓 ユーザー%d: ログイン\n", id)
			} else {
				// チェックと変更の間にレース条件
				if session.isAuthenticated {
					fmt.Printf("  🔐 ユーザー%d: アクセス許可（ID: %s）\n", id, session.userID)
					// 問題: 別のユーザーのIDでアクセスできる可能性
				}
			}
		}(i)
	}

	wg.Wait()

	// 問題4: トークンの安全でない保存
	tokenMutex.Lock()
	currentToken = generateToken()
	fmt.Printf("\n⚠️ 生成されたトークン（予測可能）: %s\n", currentToken)
	tokenMutex.Unlock()

	// 問題5: ログに機密情報が出力される
	fmt.Printf("デバッグ: パスワード=%s, トークン=%s\n", "secret123", currentToken)

	fmt.Println("\n🔧 修正が必要な箇所:")
	fmt.Println("  1. crypto/rand を使用した安全な乱数生成")
	fmt.Println("  2. 一定時間での文字列比較（subtle.ConstantTimeCompare）")
	fmt.Println("  3. セッション管理の同期")
	fmt.Println("  4. 機密情報のログ出力防止")
	fmt.Println("  5. トークンの安全な保存と管理")
}

// Challenge07_Hint - 修正のヒント
func Challenge07_Hint() {
	fmt.Println("\n💡 セキュリティ修正のヒント:")
	fmt.Println("────────────────────────")
	fmt.Println("1. crypto/rand.Read() で暗号学的に安全な乱数を生成")
	fmt.Println("2. crypto/subtle.ConstantTimeCompare() でタイミング攻撃を防ぐ")
	fmt.Println("3. sync.Mutex または sync.RWMutex でセッションを保護")
	fmt.Println("4. 環境変数や専用のシークレット管理を使用")
	fmt.Println("5. ログには機密情報を出力しない（マスキング処理）")
}