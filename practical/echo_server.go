package practical

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// EchoServerExample Echo Webサーバーの並行処理パターン
type EchoServerExample struct {
	echo     *echo.Echo
	metrics  *ServerMetrics
	limiter  *RateLimiter
	sessions *SessionStore
}

// ServerMetrics サーバーメトリクス
type ServerMetrics struct {
	requestCount    *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
	activeRequests  prometheus.Gauge
	errorRate       *prometheus.CounterVec
}

// RateLimiter レート制限
type RateLimiter struct {
	mu      sync.RWMutex
	clients map[string]*ClientLimit
}

// ClientLimit クライアント毎の制限
type ClientLimit struct {
	requests  int64
	window    time.Time
	blocked   bool
	blockTime time.Time
}

// SessionStore セッションストア
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// Session セッション情報
type Session struct {
	ID        string
	UserID    string
	Data      map[string]interface{}
	CreatedAt time.Time
	ExpiresAt time.Time
}

// NewEchoServerExample Echoサーバーの例を作成
func NewEchoServerExample() *EchoServerExample {
	e := echo.New()

	// メトリクス初期化
	metrics := &ServerMetrics{
		requestCount: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		requestDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		activeRequests: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "http_requests_active",
				Help: "Active HTTP requests",
			},
		),
		errorRate: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_errors_total",
				Help: "Total HTTP errors",
			},
			[]string{"type"},
		),
	}

	// Prometheusに登録
	prometheus.MustRegister(metrics.requestCount)
	prometheus.MustRegister(metrics.requestDuration)
	prometheus.MustRegister(metrics.activeRequests)
	prometheus.MustRegister(metrics.errorRate)

	return &EchoServerExample{
		echo:    e,
		metrics: metrics,
		limiter: &RateLimiter{
			clients: make(map[string]*ClientLimit),
		},
		sessions: &SessionStore{
			sessions: make(map[string]*Session),
		},
	}
}

// SetupMiddleware ミドルウェアを設定
func (es *EchoServerExample) SetupMiddleware() {
	// リクエストID
	es.echo.Use(middleware.RequestID())

	// ロガー
	es.echo.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${time_rfc3339} ${id} ${method} ${uri} ${status} ${latency_human}\n",
	}))

	// リカバリー
	es.echo.Use(middleware.Recover())

	// CORS
	es.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// カスタムミドルウェア: メトリクス収集
	es.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			path := c.Path()
			method := c.Request().Method

			// アクティブリクエスト増加
			es.metrics.activeRequests.Inc()
			defer es.metrics.activeRequests.Dec()

			// リクエスト処理
			err := next(c)

			// メトリクス記録
			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", c.Response().Status)

			es.metrics.requestCount.WithLabelValues(method, path, status).Inc()
			es.metrics.requestDuration.WithLabelValues(method, path).Observe(duration)

			if err != nil {
				es.metrics.errorRate.WithLabelValues("handler").Inc()
			}

			return err
		}
	})

	// カスタムミドルウェア: レート制限
	es.echo.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := c.RealIP()

			if !es.checkRateLimit(clientIP) {
				es.metrics.errorRate.WithLabelValues("rate_limit").Inc()
				return c.JSON(http.StatusTooManyRequests, map[string]string{
					"error": "Rate limit exceeded",
				})
			}

			return next(c)
		}
	})
}

// checkRateLimit レート制限チェック
func (es *EchoServerExample) checkRateLimit(clientIP string) bool {
	es.limiter.mu.Lock()
	defer es.limiter.mu.Unlock()

	now := time.Now()
	limit, exists := es.limiter.clients[clientIP]

	if !exists {
		es.limiter.clients[clientIP] = &ClientLimit{
			requests: 1,
			window:   now,
		}
		return true
	}

	// ブロック中かチェック
	if limit.blocked && now.Sub(limit.blockTime) < 1*time.Minute {
		return false
	}

	// ウィンドウリセット
	if now.Sub(limit.window) > 1*time.Minute {
		limit.requests = 1
		limit.window = now
		limit.blocked = false
		return true
	}

	// リクエスト数チェック（1分間に60リクエストまで）
	limit.requests++
	if limit.requests > 60 {
		limit.blocked = true
		limit.blockTime = now
		return false
	}

	return true
}

// SetupRoutes ルートを設定
func (es *EchoServerExample) SetupRoutes() {
	// Health check
	es.echo.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status": "healthy",
			"time":   time.Now().Format(time.RFC3339),
		})
	})

	// Metrics endpoint
	es.echo.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	// API Routes
	api := es.echo.Group("/api/v1")

	// 並行処理デモ: バッチ処理
	api.POST("/batch", es.handleBatchProcess)

	// 並行処理デモ: パイプライン処理
	api.POST("/pipeline", es.handlePipeline)

	// WebSocketエンドポイント
	api.GET("/ws", es.handleWebSocket)

	// SSE (Server-Sent Events)
	api.GET("/events", es.handleSSE)
}

// handleBatchProcess バッチ処理エンドポイント
func (es *EchoServerExample) handleBatchProcess(c echo.Context) error {
	var request struct {
		Items []string `json:"items"`
	}

	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	results := make([]map[string]interface{}, len(request.Items))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// 並行処理でアイテムを処理
	for i, item := range request.Items {
		wg.Add(1)
		go func(index int, data string) {
			defer wg.Done()

			// 処理をシミュレート
			time.Sleep(100 * time.Millisecond)

			result := map[string]interface{}{
				"item":       data,
				"processed":  true,
				"timestamp":  time.Now(),
				"worker_id":  index % 4, // 4ワーカーをシミュレート
			}

			mu.Lock()
			results[index] = result
			mu.Unlock()
		}(i, item)
	}

	wg.Wait()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"results":     results,
		"total_items": len(request.Items),
		"status":      "completed",
	})
}

// handlePipeline パイプライン処理エンドポイント
func (es *EchoServerExample) handlePipeline(c echo.Context) error {
	var request struct {
		Data string `json:"data"`
	}

	if err := c.Bind(&request); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// パイプラインステージ
	stage1 := make(chan string, 1)
	stage2 := make(chan string, 1)
	stage3 := make(chan string, 1)

	// Stage 1: 検証
	go func() {
		fmt.Printf("Stage 1: Validating data\n")
		time.Sleep(50 * time.Millisecond)
		stage1 <- request.Data + " [validated]"
		close(stage1)
	}()

	// Stage 2: 変換
	go func() {
		data := <-stage1
		fmt.Printf("Stage 2: Transforming data\n")
		time.Sleep(50 * time.Millisecond)
		stage2 <- data + " [transformed]"
		close(stage2)
	}()

	// Stage 3: 保存
	go func() {
		data := <-stage2
		fmt.Printf("Stage 3: Saving data\n")
		time.Sleep(50 * time.Millisecond)
		stage3 <- data + " [saved]"
		close(stage3)
	}()

	// 結果を待つ
	result := <-stage3

	return c.JSON(http.StatusOK, map[string]interface{}{
		"input":  request.Data,
		"output": result,
		"stages": []string{"validation", "transformation", "storage"},
	})
}

// handleWebSocket WebSocket処理
func (es *EchoServerExample) handleWebSocket(c echo.Context) error {
	// WebSocketアップグレード
	fmt.Println("WebSocket connection requested")

	// デモ用のレスポンス
	return c.JSON(http.StatusOK, map[string]string{
		"message": "WebSocket endpoint (implementation requires gorilla/websocket)",
		"info":    "Would handle real-time bidirectional communication",
	})
}

// handleSSE Server-Sent Events処理
func (es *EchoServerExample) handleSSE(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")

	// イベント送信
	for i := 0; i < 5; i++ {
		event := fmt.Sprintf("data: {\"event\": \"update\", \"id\": %d, \"time\": \"%s\"}\n\n",
			i, time.Now().Format(time.RFC3339))

		if _, err := c.Response().Write([]byte(event)); err != nil {
			return err
		}
		c.Response().Flush()

		time.Sleep(1 * time.Second)
	}

	// 終了イベント
	c.Response().Write([]byte("event: close\ndata: {\"message\": \"stream ended\"}\n\n"))
	c.Response().Flush()

	return nil
}

// StartServer サーバーを起動（グレースフルシャットダウン付き）
func (es *EchoServerExample) StartServer(port string) error {
	es.SetupMiddleware()
	es.SetupRoutes()

	// グレースフルシャットダウンの設定
	go func() {
		if err := es.echo.Start(":" + port); err != nil && err != http.ErrServerClosed {
			es.echo.Logger.Fatal("shutting down the server")
		}
	}()

	fmt.Printf("🚀 Server started on port %s\n", port)
	fmt.Println("📊 Metrics available at http://localhost:" + port + "/metrics")
	fmt.Println("🔥 Try these endpoints:")
	fmt.Println("   POST /api/v1/batch    - Batch processing")
	fmt.Println("   POST /api/v1/pipeline - Pipeline processing")
	fmt.Println("   GET  /api/v1/events   - Server-Sent Events")

	return nil
}

// Shutdown サーバーをシャットダウン
func (es *EchoServerExample) Shutdown(ctx context.Context) error {
	return es.echo.Shutdown(ctx)
}

// Example_ConcurrentMiddleware 並行処理を使ったミドルウェアの例
func Example_ConcurrentMiddleware() {
	fmt.Println("\n=== Echo Server: Concurrent Middleware Pattern ===")

	// 並行処理ミドルウェア: 複数のバックエンドサービスを並行呼び出し
	aggregatorMiddleware := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			type ServiceResult struct {
				Name   string
				Data   interface{}
				Error  error
			}

			services := []string{"user-service", "product-service", "recommendation-service"}
			results := make(chan ServiceResult, len(services))

			// 各サービスを並行呼び出し
			for _, service := range services {
				go func(svc string) {
					// サービス呼び出しをシミュレート
					time.Sleep(time.Duration(50+len(svc)*10) * time.Millisecond)

					results <- ServiceResult{
						Name: svc,
						Data: map[string]interface{}{
							"service": svc,
							"status":  "ok",
							"data":    fmt.Sprintf("Data from %s", svc),
						},
					}
				}(service)
			}

			// 結果を集約
			aggregated := make(map[string]interface{})
			for i := 0; i < len(services); i++ {
				result := <-results
				if result.Error == nil {
					aggregated[result.Name] = result.Data
				}
			}

			// コンテキストに結果を設定
			c.Set("aggregated_data", aggregated)
			return next(c)
		}
	}

	// デモ実行
	e := echo.New()
	e.Use(aggregatorMiddleware)

	e.GET("/aggregate", func(c echo.Context) error {
		data := c.Get("aggregated_data")
		return c.JSON(http.StatusOK, data)
	})

	fmt.Println("Middleware example configured")
}

// CircuitBreakerMiddleware サーキットブレーカーミドルウェア
type CircuitBreakerMiddleware struct {
	failureThreshold uint32
	successThreshold uint32
	timeout          time.Duration
	failures         uint32
	successes        uint32
	lastFailureTime  int64
	state            int32 // 0: closed, 1: open, 2: half-open
}

// NewCircuitBreaker サーキットブレーカーを作成
func NewCircuitBreaker(failureThreshold, successThreshold uint32, timeout time.Duration) *CircuitBreakerMiddleware {
	return &CircuitBreakerMiddleware{
		failureThreshold: failureThreshold,
		successThreshold: successThreshold,
		timeout:          timeout,
	}
}

// Middleware Echoミドルウェア関数
func (cb *CircuitBreakerMiddleware) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		state := atomic.LoadInt32(&cb.state)

		// Circuit Open
		if state == 1 {
			lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
			if time.Since(time.Unix(0, lastFailure)) > cb.timeout {
				atomic.StoreInt32(&cb.state, 2) // Half-open
				atomic.StoreUint32(&cb.successes, 0)
			} else {
				return c.JSON(http.StatusServiceUnavailable, map[string]string{
					"error": "Circuit breaker is open",
				})
			}
		}

		// Process request
		err := next(c)

		if err != nil {
			failures := atomic.AddUint32(&cb.failures, 1)
			atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())

			if failures >= cb.failureThreshold {
				atomic.StoreInt32(&cb.state, 1) // Open
			}
			return err
		}

		// Success
		if state == 2 { // Half-open
			successes := atomic.AddUint32(&cb.successes, 1)
			if successes >= cb.successThreshold {
				atomic.StoreInt32(&cb.state, 0) // Closed
				atomic.StoreUint32(&cb.failures, 0)
			}
		}

		return nil
	}
}