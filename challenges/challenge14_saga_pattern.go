package challenges

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Challenge 14: Sagaパターン（分散トランザクション）の問題
//
// 問題点:
// 1. 補償トランザクションが正しく実行されない
// 2. タイムアウト処理が不適切
// 3. 状態管理が不完全
// 4. デッドロックの可能性

type SagaStep struct {
	Name        string
	Execute     func(context.Context) error
	Compensate  func(context.Context) error
}

type SagaOrchestrator struct {
	mu              sync.Mutex
	steps           []SagaStep
	executedSteps   []int
	compensating    bool
	completedCompensations map[int]bool
}

// BUG: 補償トランザクションが不完全
func (s *SagaOrchestrator) Execute(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, step := range s.steps {
		fmt.Printf("実行中: %s\n", step.Name)

		// 問題: タイムアウトなし
		err := step.Execute(ctx)
		if err != nil {
			fmt.Printf("エラー発生: %s - %v\n", step.Name, err)

			// BUG: 補償が逆順で実行されない
			for j := 0; j <= i; j++ {
				if s.steps[j].Compensate != nil {
					// 問題: エラーハンドリングなし
					s.steps[j].Compensate(ctx)
				}
			}
			return err
		}

		s.executedSteps = append(s.executedSteps, i)
	}

	return nil
}

// BUG: 並行補償処理に問題
func (s *SagaOrchestrator) CompensateAsync(ctx context.Context) error {
	s.mu.Lock()
	s.compensating = true
	steps := make([]int, len(s.executedSteps))
	copy(steps, s.executedSteps)
	s.mu.Unlock()

	s.completedCompensations = make(map[int]bool)
	var wg sync.WaitGroup

	// 問題: 全ての補償を同時実行（依存関係無視）
	for _, stepIdx := range steps {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			step := s.steps[idx]
			if step.Compensate != nil {
				// 問題: パニックハンドリングなし
				err := step.Compensate(ctx)
				if err == nil {
					s.mu.Lock()
					s.completedCompensations[idx] = true
					s.mu.Unlock()
				}
			}
		}(stepIdx)
	}

	wg.Wait()
	return nil
}

// BUG: 状態管理が不完全
func (s *SagaOrchestrator) GetStatus() string {
	// 問題: ロックなし
	if s.compensating {
		completed := len(s.completedCompensations)
		total := len(s.executedSteps)
		return fmt.Sprintf("補償中: %d/%d", completed, total)
	}

	return fmt.Sprintf("実行済み: %d/%d", len(s.executedSteps), len(s.steps))
}

// BUG: デッドロックの可能性
func (s *SagaOrchestrator) AddStep(step SagaStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.compensating {
		// 問題: 補償中に新しいステップを追加しようとして無限待機
		s.mu.Lock() // デッドロック！
		defer s.mu.Unlock()
	}

	s.steps = append(s.steps, step)
	return nil
}

func Challenge14_SagaPatternProblem() {
	fmt.Println("Challenge 14: Sagaパターン（分散トランザクション）の問題")
	fmt.Println("問題:")
	fmt.Println("1. 補償トランザクションが正しく実行されない")
	fmt.Println("2. タイムアウト処理が不適切")
	fmt.Println("3. 状態管理が不完全")
	fmt.Println("4. デッドロックの可能性")

	saga := &SagaOrchestrator{
		steps: []SagaStep{
			{
				Name: "注文作成",
				Execute: func(ctx context.Context) error {
					time.Sleep(100 * time.Millisecond)
					return nil
				},
				Compensate: func(ctx context.Context) error {
					fmt.Println("注文キャンセル")
					return nil
				},
			},
			{
				Name: "在庫確保",
				Execute: func(ctx context.Context) error {
					time.Sleep(100 * time.Millisecond)
					if rand.Float32() < 0.3 {
						return errors.New("在庫不足")
					}
					return nil
				},
				Compensate: func(ctx context.Context) error {
					fmt.Println("在庫解放")
					return nil
				},
			},
			{
				Name: "決済処理",
				Execute: func(ctx context.Context) error {
					time.Sleep(100 * time.Millisecond)
					if rand.Float32() < 0.2 {
						return errors.New("決済失敗")
					}
					return nil
				},
				Compensate: func(ctx context.Context) error {
					fmt.Println("返金処理")
					time.Sleep(200 * time.Millisecond) // 時間のかかる補償
					return nil
				},
			},
		},
	}

	ctx := context.Background()

	// Saga実行
	err := saga.Execute(ctx)
	if err != nil {
		fmt.Printf("Saga失敗: %v\n", err)
		// 非同期補償
		saga.CompensateAsync(ctx)
	}

	// ステータス確認（レース条件の可能性）
	for i := 0; i < 3; i++ {
		fmt.Printf("ステータス: %s\n", saga.GetStatus())
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("\n修正が必要な箇所を特定してください")
}