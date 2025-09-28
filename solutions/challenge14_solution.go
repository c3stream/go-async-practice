package solutions

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge14_SagaSolution - Sagaパターン問題の解決
func Challenge14_SagaSolution() {
	fmt.Println("\n✅ チャレンジ14: Sagaパターン問題の解決")
	fmt.Println("=" + repeatString("=", 50))

	// 解決策1: 補償付きSaga
	solution1_CompensatingSaga()

	// 解決策2: オーケストレーションベースSaga
	solution2_OrchestrationSaga()

	// 解決策3: コレオグラフィーベースSaga
	solution3_ChoreographySaga()
}

// 解決策1: 補償付きSaga
func solution1_CompensatingSaga() {
	fmt.Println("\n📝 解決策1: 補償付きSaga")

	type SagaStep struct {
		ID          string
		Name        string
		Execute     func() error
		Compensate  func() error
		Idempotent  bool
		MaxRetries  int
		RetryDelay  time.Duration
		ExecutionID string // 冪等性のための実行ID
		mu          sync.Mutex
		executed    bool
		compensated bool
	}

	type Saga struct {
		ID             string
		Steps          []*SagaStep
		Context        map[string]interface{}
		mu             sync.RWMutex
		executionLog   []string
		compensationLog []string
	}

	// Sagaマネージャー
	type SagaManager struct {
		executionHistory sync.Map // ExecutionID -> bool
	}

	manager := &SagaManager{}

	// 冪等性チェック付き実行
	executeStep := func(step *SagaStep) error {
		step.mu.Lock()
		defer step.mu.Unlock()

		if step.Idempotent {
			// 実行履歴チェック
			if _, exists := manager.executionHistory.Load(step.ExecutionID); exists {
				fmt.Printf("  ⏭️ Step %s already executed (idempotent)\n", step.Name)
				return nil
			}
		}

		// リトライ付き実行
		var lastErr error
		for attempt := 0; attempt <= step.MaxRetries; attempt++ {
			if attempt > 0 {
				fmt.Printf("  🔄 Retrying %s (attempt %d/%d)\n",
					step.Name, attempt, step.MaxRetries)
				time.Sleep(step.RetryDelay * time.Duration(attempt))
			}

			if err := step.Execute(); err != nil {
				lastErr = err
				continue
			}

			// 成功記録
			step.executed = true
			if step.Idempotent {
				manager.executionHistory.Store(step.ExecutionID, true)
			}
			fmt.Printf("  ✅ Step %s executed successfully\n", step.Name)
			return nil
		}

		return fmt.Errorf("step %s failed after %d attempts: %v",
			step.Name, step.MaxRetries+1, lastErr)
	}

	// Saga実行
	executeSaga := func(saga *Saga) error {
		saga.mu.Lock()
		saga.executionLog = append(saga.executionLog,
			fmt.Sprintf("Saga %s started at %v", saga.ID, time.Now()))
		saga.mu.Unlock()

		// 各ステップを順次実行
		for i, step := range saga.Steps {
			if err := executeStep(step); err != nil {
				fmt.Printf("  ❌ Step %s failed: %v\n", step.Name, err)

				// 補償トランザクション実行
				fmt.Println("  🔄 Starting compensation...")
				for j := i - 1; j >= 0; j-- {
					compensateStep := saga.Steps[j]
					compensateStep.mu.Lock()

					if compensateStep.executed && !compensateStep.compensated {
						fmt.Printf("    ↩ Compensating %s\n", compensateStep.Name)
						if compErr := compensateStep.Compensate(); compErr != nil {
							fmt.Printf("    ⚠️ Compensation failed for %s: %v\n",
								compensateStep.Name, compErr)
							// 補償失敗はログに記録して続行
							saga.mu.Lock()
							saga.compensationLog = append(saga.compensationLog,
								fmt.Sprintf("Compensation failed for %s: %v",
									compensateStep.Name, compErr))
							saga.mu.Unlock()
						} else {
							compensateStep.compensated = true
						}
					}

					compensateStep.mu.Unlock()
				}

				return err
			}
		}

		saga.mu.Lock()
		saga.executionLog = append(saga.executionLog,
			fmt.Sprintf("Saga %s completed at %v", saga.ID, time.Now()))
		saga.mu.Unlock()

		fmt.Printf("  🎉 Saga %s completed successfully\n", saga.ID)
		return nil
	}

	// テスト用のステップ作成
	var orderID atomic.Int32
	var inventory atomic.Int32
	inventory.Store(10)
	var balance atomic.Int32
	balance.Store(1000)

	createOrderSaga := func() *Saga {
		ordID := orderID.Add(1)
		sagaID := fmt.Sprintf("saga_%d", ordID)

		return &Saga{
			ID:      sagaID,
			Context: make(map[string]interface{}),
			Steps: []*SagaStep{
				{
					ID:          fmt.Sprintf("%s_step1", sagaID),
					Name:        "Reserve Inventory",
					ExecutionID: fmt.Sprintf("%s_reserve_%d", sagaID, time.Now().Unix()),
					Idempotent:  true,
					MaxRetries:  2,
					RetryDelay:  100 * time.Millisecond,
					Execute: func() error {
						if inventory.Load() < 2 {
							return fmt.Errorf("insufficient inventory")
						}
						inventory.Add(-2)
						fmt.Printf("    Inventory: %d → %d\n",
							inventory.Load()+2, inventory.Load())
						return nil
					},
					Compensate: func() error {
						inventory.Add(2)
						fmt.Printf("    Inventory restored: %d → %d\n",
							inventory.Load()-2, inventory.Load())
						return nil
					},
				},
				{
					ID:          fmt.Sprintf("%s_step2", sagaID),
					Name:        "Process Payment",
					ExecutionID: fmt.Sprintf("%s_payment_%d", sagaID, time.Now().Unix()),
					Idempotent:  true,
					MaxRetries:  3,
					RetryDelay:  200 * time.Millisecond,
					Execute: func() error {
						amount := int32(200)
						if balance.Load() < amount {
							return fmt.Errorf("insufficient funds")
						}
						balance.Add(-amount)
						fmt.Printf("    Balance: %d → %d\n",
							balance.Load()+amount, balance.Load())
						return nil
					},
					Compensate: func() error {
						balance.Add(200)
						fmt.Printf("    Payment refunded: %d → %d\n",
							balance.Load()-200, balance.Load())
						return nil
					},
				},
				{
					ID:          fmt.Sprintf("%s_step3", sagaID),
					Name:        "Ship Order",
					ExecutionID: fmt.Sprintf("%s_ship_%d", sagaID, time.Now().Unix()),
					Idempotent:  false,
					MaxRetries:  1,
					RetryDelay:  100 * time.Millisecond,
					Execute: func() error {
						// 50%の確率で失敗
						if time.Now().UnixNano()%2 == 0 {
							return fmt.Errorf("shipping service unavailable")
						}
						fmt.Printf("    Order shipped: #%d\n", ordID)
						return nil
					},
					Compensate: func() error {
						fmt.Printf("    Shipping cancelled for order #%d\n", ordID)
						return nil
					},
				},
			},
		}
	}

	// 実行
	fmt.Println("\n実行:")
	saga1 := createOrderSaga()
	executeSaga(saga1)

	fmt.Printf("\n最終状態: Inventory=%d, Balance=%d\n",
		inventory.Load(), balance.Load())
}

// 解決策2: オーケストレーションベースSaga
func solution2_OrchestrationSaga() {
	fmt.Println("\n📝 解決策2: オーケストレーションベースSaga")

	// コマンドとイベント
	type Command struct {
		ID      string
		Type    string
		Payload interface{}
	}

	type Event struct {
		ID        string
		Type      string
		Success   bool
		Error     error
		Payload   interface{}
		Timestamp time.Time
	}

	// オーケストレーター
	type Orchestrator struct {
		mu         sync.RWMutex
		sagas      map[string]*SagaState
		commandBus chan Command
		eventBus   chan Event
	}

	type SagaState struct {
		ID           string
		CurrentStep  int
		Steps        []string
		Status       string // "running", "completed", "compensating", "failed"
		Context      map[string]interface{}
		CompletedSteps []string
	}

	orchestrator := &Orchestrator{
		sagas:      make(map[string]*SagaState),
		commandBus: make(chan Command, 100),
		eventBus:   make(chan Event, 100),
	}

	// サービスシミュレーション
	services := map[string]func(Command) Event{
		"inventory": func(cmd Command) Event {
			fmt.Printf("  📦 Inventory service processing: %s\n", cmd.Type)
			time.Sleep(100 * time.Millisecond)
			return Event{
				ID:        cmd.ID,
				Type:      cmd.Type + "_completed",
				Success:   true,
				Timestamp: time.Now(),
			}
		},
		"payment": func(cmd Command) Event {
			fmt.Printf("  💳 Payment service processing: %s\n", cmd.Type)
			time.Sleep(150 * time.Millisecond)
			return Event{
				ID:        cmd.ID,
				Type:      cmd.Type + "_completed",
				Success:   true,
				Timestamp: time.Now(),
			}
		},
		"shipping": func(cmd Command) Event {
			fmt.Printf("  🚚 Shipping service processing: %s\n", cmd.Type)
			time.Sleep(200 * time.Millisecond)
			// 30%の確率で失敗
			if time.Now().UnixNano()%10 < 3 {
				return Event{
					ID:        cmd.ID,
					Type:      cmd.Type + "_failed",
					Success:   false,
					Error:     fmt.Errorf("shipping unavailable"),
					Timestamp: time.Now(),
				}
			}
			return Event{
				ID:        cmd.ID,
				Type:      cmd.Type + "_completed",
				Success:   true,
				Timestamp: time.Now(),
			}
		},
	}

	// オーケストレーター実行
	runOrchestrator := func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case cmd := <-orchestrator.commandBus:
				// コマンドルーティング
				go func(c Command) {
					var event Event
					switch c.Type {
					case "reserve_inventory", "release_inventory":
						event = services["inventory"](c)
					case "process_payment", "refund_payment":
						event = services["payment"](c)
					case "ship_order", "cancel_shipping":
						event = services["shipping"](c)
					default:
						event = Event{
							ID:      c.ID,
							Type:    "unknown_command",
							Success: false,
							Error:   fmt.Errorf("unknown command type: %s", c.Type),
						}
					}
					orchestrator.eventBus <- event
				}(cmd)

			case event := <-orchestrator.eventBus:
				// イベント処理
				orchestrator.mu.Lock()
				saga, exists := orchestrator.sagas[event.ID]
				if !exists {
					orchestrator.mu.Unlock()
					continue
				}

				if event.Success {
					saga.CompletedSteps = append(saga.CompletedSteps, event.Type)
					saga.CurrentStep++

					if saga.CurrentStep >= len(saga.Steps) {
						saga.Status = "completed"
						fmt.Printf("  ✅ Saga %s completed successfully\n", saga.ID)
					} else {
						// 次のステップ実行
						nextStep := saga.Steps[saga.CurrentStep]
						orchestrator.commandBus <- Command{
							ID:   saga.ID,
							Type: nextStep,
						}
					}
				} else {
					// 補償開始
					saga.Status = "compensating"
					fmt.Printf("  ⚠️ Saga %s failed at step %d, starting compensation\n",
						saga.ID, saga.CurrentStep)

					// 補償コマンド送信
					go func(s *SagaState) {
						compensationSteps := map[string]string{
							"reserve_inventory_completed": "release_inventory",
							"process_payment_completed":   "refund_payment",
							"ship_order_completed":        "cancel_shipping",
						}

						for i := len(s.CompletedSteps) - 1; i >= 0; i-- {
							if compCmd, exists := compensationSteps[s.CompletedSteps[i]]; exists {
								fmt.Printf("    ↩ Compensating: %s\n", compCmd)
								orchestrator.commandBus <- Command{
									ID:   s.ID,
									Type: compCmd,
								}
								time.Sleep(100 * time.Millisecond)
							}
						}
						s.Status = "compensated"
					}(saga)
				}
				orchestrator.mu.Unlock()
			}
		}
	}

	// Saga開始
	startSaga := func(sagaID string, steps []string) {
		orchestrator.mu.Lock()
		orchestrator.sagas[sagaID] = &SagaState{
			ID:           sagaID,
			CurrentStep:  0,
			Steps:        steps,
			Status:       "running",
			Context:      make(map[string]interface{}),
			CompletedSteps: []string{},
		}
		orchestrator.mu.Unlock()

		// 最初のコマンド送信
		orchestrator.commandBus <- Command{
			ID:   sagaID,
			Type: steps[0],
		}
	}

	// 実行
	fmt.Println("\n実行:")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go runOrchestrator(ctx)

	// Saga定義と実行
	sagaSteps := []string{
		"reserve_inventory",
		"process_payment",
		"ship_order",
	}

	startSaga("order_saga_1", sagaSteps)
	time.Sleep(2 * time.Second)

	startSaga("order_saga_2", sagaSteps)
	time.Sleep(2 * time.Second)
}

// 解決策3: コレオグラフィーベースSaga
func solution3_ChoreographySaga() {
	fmt.Println("\n📝 解決策3: コレオグラフィーベースSaga")

	// イベントバス
	type EventBus struct {
		mu          sync.RWMutex
		subscribers map[string][]chan Event
	}

	type Event struct {
		ID          string
		Type        string
		SagaID      string
		Payload     interface{}
		Timestamp   time.Time
		Success     bool
		Error       error
	}

	eventBus := &EventBus{
		subscribers: make(map[string][]chan Event),
	}

	// イベント購読
	subscribe := func(eventType string) <-chan Event {
		eventBus.mu.Lock()
		defer eventBus.mu.Unlock()

		ch := make(chan Event, 10)
		eventBus.subscribers[eventType] = append(eventBus.subscribers[eventType], ch)
		return ch
	}

	// イベント発行
	publish := func(event Event) {
		eventBus.mu.RLock()
		defer eventBus.mu.RUnlock()

		fmt.Printf("  📢 Published: %s for saga %s\n", event.Type, event.SagaID)

		if subscribers, exists := eventBus.subscribers[event.Type]; exists {
			for _, ch := range subscribers {
				select {
				case ch <- event:
				default:
					// チャネルがフルの場合はスキップ
				}
			}
		}
	}

	// サービス実装
	// Inventoryサービス
	go func() {
		orderCreated := subscribe("OrderCreated")
		paymentFailed := subscribe("PaymentFailed")

		for {
			select {
			case event := <-orderCreated:
				fmt.Printf("  📦 Inventory: Reserving for saga %s\n", event.SagaID)
				time.Sleep(100 * time.Millisecond)

				// 在庫予約成功
				publish(Event{
					ID:        fmt.Sprintf("inv_%d", time.Now().UnixNano()),
					Type:      "InventoryReserved",
					SagaID:    event.SagaID,
					Success:   true,
					Timestamp: time.Now(),
				})

			case event := <-paymentFailed:
				fmt.Printf("  📦 Inventory: Releasing for saga %s\n", event.SagaID)
				time.Sleep(100 * time.Millisecond)

				publish(Event{
					ID:        fmt.Sprintf("inv_%d", time.Now().UnixNano()),
					Type:      "InventoryReleased",
					SagaID:    event.SagaID,
					Success:   true,
					Timestamp: time.Now(),
				})
			}
		}
	}()

	// Paymentサービス
	go func() {
		inventoryReserved := subscribe("InventoryReserved")
		shippingFailed := subscribe("ShippingFailed")

		for {
			select {
			case event := <-inventoryReserved:
				fmt.Printf("  💳 Payment: Processing for saga %s\n", event.SagaID)
				time.Sleep(150 * time.Millisecond)

				// 支払い処理
				publish(Event{
					ID:        fmt.Sprintf("pay_%d", time.Now().UnixNano()),
					Type:      "PaymentProcessed",
					SagaID:    event.SagaID,
					Success:   true,
					Timestamp: time.Now(),
				})

			case event := <-shippingFailed:
				fmt.Printf("  💳 Payment: Refunding for saga %s\n", event.SagaID)
				time.Sleep(150 * time.Millisecond)

				publish(Event{
					ID:        fmt.Sprintf("pay_%d", time.Now().UnixNano()),
					Type:      "PaymentRefunded",
					SagaID:    event.SagaID,
					Success:   true,
					Timestamp: time.Now(),
				})

				// 在庫解放もトリガー
				publish(Event{
					ID:        fmt.Sprintf("pay_%d", time.Now().UnixNano()),
					Type:      "PaymentFailed",
					SagaID:    event.SagaID,
					Success:   false,
					Timestamp: time.Now(),
				})
			}
		}
	}()

	// Shippingサービス
	go func() {
		paymentProcessed := subscribe("PaymentProcessed")

		for event := range paymentProcessed {
			fmt.Printf("  🚚 Shipping: Processing for saga %s\n", event.SagaID)
			time.Sleep(200 * time.Millisecond)

			// 30%の確率で失敗
			if time.Now().UnixNano()%10 < 3 {
				fmt.Printf("  ❌ Shipping failed for saga %s\n", event.SagaID)
				publish(Event{
					ID:        fmt.Sprintf("ship_%d", time.Now().UnixNano()),
					Type:      "ShippingFailed",
					SagaID:    event.SagaID,
					Success:   false,
					Error:     fmt.Errorf("shipping unavailable"),
					Timestamp: time.Now(),
				})
			} else {
				publish(Event{
					ID:        fmt.Sprintf("ship_%d", time.Now().UnixNano()),
					Type:      "OrderShipped",
					SagaID:    event.SagaID,
					Success:   true,
					Timestamp: time.Now(),
				})
			}
		}
	}()

	// Saga完了監視
	go func() {
		orderShipped := subscribe("OrderShipped")
		paymentRefunded := subscribe("PaymentRefunded")

		for {
			select {
			case event := <-orderShipped:
				fmt.Printf("  🎉 Saga %s completed successfully!\n", event.SagaID)

			case event := <-paymentRefunded:
				fmt.Printf("  ⚠️ Saga %s compensated successfully\n", event.SagaID)
			}
		}
	}()

	// テスト実行
	fmt.Println("\n実行:")
	time.Sleep(100 * time.Millisecond) // サービス起動待ち

	// Saga開始
	for i := 0; i < 3; i++ {
		sagaID := fmt.Sprintf("saga_%d", i+1)
		fmt.Printf("\n▶ Starting %s\n", sagaID)

		publish(Event{
			ID:        fmt.Sprintf("order_%d", i+1),
			Type:      "OrderCreated",
			SagaID:    sagaID,
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"order_id": fmt.Sprintf("order_%d", i+1),
				"amount":   float64((i+1) * 100),
			},
		})

		time.Sleep(1 * time.Second)
	}

	time.Sleep(1 * time.Second)
}