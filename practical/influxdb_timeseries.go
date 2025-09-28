package practical

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

// InfluxDBTimeSeries - InfluxDBを使った時系列データ処理
type InfluxDBTimeSeries struct {
	client influxdb2.Client
	org    string
	bucket string
	mu     sync.RWMutex
}

// NewInfluxDBTimeSeries - InfluxDB接続の初期化
func NewInfluxDBTimeSeries(url, token, org, bucket string) (*InfluxDBTimeSeries, error) {
	// InfluxDBクライアント作成
	client := influxdb2.NewClientWithOptions(url, token,
		influxdb2.DefaultOptions().
			SetBatchSize(100).
			SetRetryInterval(1000))

	// 接続テスト
	ctx := context.Background()
	health, err := client.Health(ctx)
	if err != nil || health.Status != "pass" {
		fmt.Println("⚠ InfluxDBに接続できません。デモモードで実行します。")
		return &InfluxDBTimeSeries{
			client: nil,
			org:    org,
			bucket: bucket,
		}, nil
	}

	return &InfluxDBTimeSeries{
		client: client,
		org:    org,
		bucket: bucket,
	}, nil
}

// IoTMonitoringDemo - IoTデバイス監視デモ
func (i *InfluxDBTimeSeries) IoTMonitoringDemo(ctx context.Context) {
	fmt.Println("\n📡 InfluxDB IoTモニタリングデモ")
	fmt.Println("=" + repeatString("=", 50))

	if i.client == nil {
		i.runDemoMode(ctx)
		return
	}

	// メトリクス
	var (
		totalPoints   int64
		errorPoints   int64
		batchesSent   int64
	)

	// IoTデバイスシミュレーター起動
	numDevices := 10
	var wg sync.WaitGroup

	for deviceID := 0; deviceID < numDevices; deviceID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			i.simulateIoTDevice(ctx, id, &totalPoints, &errorPoints, &batchesSent)
		}(deviceID)
	}

	// リアルタイム集計
	go i.realtimeAggregation(ctx)

	// 異常検知
	go i.anomalyDetection(ctx)

	// メトリクス表示
	go i.displayMetrics(ctx, &totalPoints, &errorPoints, &batchesSent)

	// 実行時間
	select {
	case <-ctx.Done():
	case <-time.After(15 * time.Second):
	}

	// 終了待機
	wg.Wait()

	// 最終レポート
	i.generateReport(ctx, totalPoints, errorPoints, batchesSent)
}

// simulateIoTDevice - IoTデバイスのシミュレーション
func (i *InfluxDBTimeSeries) simulateIoTDevice(ctx context.Context, deviceID int,
	totalPoints, errorPoints, batchesSent *int64) {

	writeAPI := i.client.WriteAPIBlocking(i.org, i.bucket)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	deviceName := fmt.Sprintf("sensor_%03d", deviceID)
	location := []string{"tokyo", "osaka", "nagoya", "fukuoka"}[deviceID%4]
	deviceType := []string{"temperature", "humidity", "pressure", "motion"}[deviceID%4]

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// センサーデータ生成
			value := i.generateSensorValue(deviceType, deviceID)

			// データポイント作成
			point := write.NewPoint("iot_metrics",
				map[string]string{
					"device":   deviceName,
					"location": location,
					"type":     deviceType,
				},
				map[string]interface{}{
					"value":   value,
					"quality": rand.Float64() * 100,
					"battery": 100 - float64(deviceID),
				},
				time.Now())

			// バッチ書き込み
			err := writeAPI.WritePoint(ctx, point)
			if err != nil {
				atomic.AddInt64(errorPoints, 1)
			} else {
				atomic.AddInt64(totalPoints, 1)
			}

			// 統計更新
			if atomic.LoadInt64(totalPoints)%100 == 0 {
				atomic.AddInt64(batchesSent, 1)
			}

			// アラート条件チェック
			if deviceType == "temperature" && value > 35 {
				i.sendAlert(deviceName, "High Temperature", value)
			}
		}
	}
}

// generateSensorValue - センサー値の生成
func (i *InfluxDBTimeSeries) generateSensorValue(sensorType string, deviceID int) float64 {
	base := float64(deviceID * 10)
	noise := (rand.Float64() - 0.5) * 10

	switch sensorType {
	case "temperature":
		// 温度: 20-30°C + 正弦波変動
		return 25 + 5*math.Sin(float64(time.Now().Unix())/10) + noise
	case "humidity":
		// 湿度: 40-60%
		return 50 + base/10 + noise
	case "pressure":
		// 気圧: 1000-1020 hPa
		return 1010 + base/100 + noise
	case "motion":
		// モーション: 0-100
		return math.Abs(base + noise)
	default:
		return rand.Float64() * 100
	}
}

// realtimeAggregation - リアルタイム集計
func (i *InfluxDBTimeSeries) realtimeAggregation(ctx context.Context) {
	if i.client == nil {
		return
	}

	queryAPI := i.client.QueryAPI(i.org)
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 過去1分の集計クエリ
			query := fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -1m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> group(columns: ["type"])
					|> aggregateWindow(every: 10s, fn: mean, createEmpty: false)
					|> yield(name: "mean")
			`, i.bucket)

			result, err := queryAPI.Query(ctx, query)
			if err != nil {
				continue
			}

			fmt.Println("\n📊 リアルタイム集計（過去1分）:")
			for result.Next() {
				record := result.Record()
				fmt.Printf("  %s: %.2f (時刻: %v)\n",
					record.Field(),
					record.Value(),
					record.Time())
			}

			if err := result.Err(); err != nil {
				fmt.Printf("  クエリエラー: %v\n", err)
			}
		}
	}
}

// anomalyDetection - 異常検知
func (i *InfluxDBTimeSeries) anomalyDetection(ctx context.Context) {
	if i.client == nil {
		return
	}

	queryAPI := i.client.QueryAPI(i.org)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 標準偏差を使った異常検知
			query := fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -5m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> filter(fn: (r) => r["_field"] == "value")
					|> group(columns: ["device"])
					|> stddev()
			`, i.bucket)

			result, err := queryAPI.Query(ctx, query)
			if err != nil {
				continue
			}

			anomalies := []string{}
			for result.Next() {
				record := result.Record()
				if value, ok := record.Value().(float64); ok && value > 10 {
					device := record.ValueByKey("device")
					anomalies = append(anomalies,
						fmt.Sprintf("%v (σ=%.2f)", device, value))
				}
			}

			if len(anomalies) > 0 {
				fmt.Println("\n⚠ 異常検知:")
				for _, anomaly := range anomalies {
					fmt.Printf("  %s\n", anomaly)
				}
			}
		}
	}
}

// displayMetrics - メトリクス表示
func (i *InfluxDBTimeSeries) displayMetrics(ctx context.Context,
	totalPoints, errorPoints, batchesSent *int64) {

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	lastTotal := int64(0)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := atomic.LoadInt64(totalPoints)
			errors := atomic.LoadInt64(errorPoints)
			batches := atomic.LoadInt64(batchesSent)

			// スループット計算
			throughput := float64(total-lastTotal) / 2.0
			elapsed := time.Since(startTime).Seconds()
			avgThroughput := float64(total) / elapsed

			errorRate := float64(errors) / float64(total+1) * 100

			fmt.Printf("\n📈 メトリクス: Points=%d, Errors=%d (%.1f%%), "+
				"Batches=%d, Throughput=%.1f/s (avg: %.1f/s)\n",
				total, errors, errorRate, batches, throughput, avgThroughput)

			lastTotal = total
		}
	}
}

// WindowedAnalytics - ウィンドウ分析
func (i *InfluxDBTimeSeries) WindowedAnalytics(ctx context.Context) {
	fmt.Println("\n📊 ウィンドウ分析デモ")
	fmt.Println("=" + repeatString("=", 50))

	if i.client == nil {
		fmt.Println("デモモード: ウィンドウ分析シミュレーション")
		i.simulateWindowAnalytics()
		return
	}

	queryAPI := i.client.QueryAPI(i.org)

	// 各種ウィンドウ関数
	windowFunctions := []struct {
		name  string
		query string
	}{
		{
			name: "移動平均（5分）",
			query: fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -30m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> filter(fn: (r) => r["_field"] == "value")
					|> movingAverage(n: 5)
					|> yield(name: "moving_avg")
			`, i.bucket),
		},
		{
			name: "累積合計",
			query: fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -10m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> filter(fn: (r) => r["_field"] == "value")
					|> cumulativeSum()
					|> yield(name: "cumulative")
			`, i.bucket),
		},
		{
			name: "変化率",
			query: fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -10m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> filter(fn: (r) => r["_field"] == "value")
					|> derivative(unit: 1m, nonNegative: false)
					|> yield(name: "rate_of_change")
			`, i.bucket),
		},
		{
			name: "パーセンタイル（P95）",
			query: fmt.Sprintf(`
				from(bucket: "%s")
					|> range(start: -10m)
					|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
					|> filter(fn: (r) => r["_field"] == "value")
					|> quantile(q: 0.95)
					|> yield(name: "p95")
			`, i.bucket),
		},
	}

	for _, wf := range windowFunctions {
		fmt.Printf("\n📍 %s:\n", wf.name)

		result, err := queryAPI.Query(ctx, wf.query)
		if err != nil {
			fmt.Printf("  エラー: %v\n", err)
			continue
		}

		count := 0
		for result.Next() {
			record := result.Record()
			fmt.Printf("  %v: %.2f\n", record.Time(), record.Value())
			count++
			if count >= 5 {
				fmt.Println("  ...")
				break
			}
		}

		if count == 0 {
			fmt.Println("  データなし")
		}
	}
}

// sendAlert - アラート送信
func (i *InfluxDBTimeSeries) sendAlert(device, alertType string, value float64) {
	fmt.Printf("\n🚨 アラート: %s - %s (値: %.2f)\n", device, alertType, value)
}

// generateReport - レポート生成
func (i *InfluxDBTimeSeries) generateReport(ctx context.Context,
	totalPoints, errorPoints, batchesSent int64) {

	fmt.Println("\n📋 最終レポート")
	fmt.Println("=" + repeatString("=", 50))

	if i.client == nil {
		fmt.Println("デモモード完了")
		return
	}

	// サマリー統計
	fmt.Printf("\n📊 データ統計:\n")
	fmt.Printf("  総データポイント: %d\n", totalPoints)
	fmt.Printf("  エラー: %d\n", errorPoints)
	fmt.Printf("  バッチ送信: %d\n", batchesSent)
	fmt.Printf("  成功率: %.2f%%\n", float64(totalPoints-errorPoints)/float64(totalPoints)*100)

	// データベースからの統計取得
	queryAPI := i.client.QueryAPI(i.org)

	// デバイス別統計
	query := fmt.Sprintf(`
		from(bucket: "%s")
			|> range(start: -5m)
			|> filter(fn: (r) => r["_measurement"] == "iot_metrics")
			|> filter(fn: (r) => r["_field"] == "value")
			|> group(columns: ["device"])
			|> count()
	`, i.bucket)

	result, err := queryAPI.Query(ctx, query)
	if err == nil {
		fmt.Println("\n📊 デバイス別データ数:")
		for result.Next() {
			record := result.Record()
			device := record.ValueByKey("device")
			count := record.Value()
			fmt.Printf("  %v: %v\n", device, count)
		}
	}
}

// runDemoMode - デモモード実行
func (i *InfluxDBTimeSeries) runDemoMode(ctx context.Context) {
	fmt.Println("\n🎭 デモモード: InfluxDB時系列処理シミュレーション")

	// 仮想時系列データ
	type TimeSeriesPoint struct {
		Timestamp time.Time
		Value     float64
		Tags      map[string]string
	}

	var (
		points []TimeSeriesPoint
		mu     sync.Mutex
		wg     sync.WaitGroup
	)

	// データ生成
	numGenerators := 5
	pointsPerGenerator := 100

	for g := 0; g < numGenerators; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < pointsPerGenerator; i++ {
				point := TimeSeriesPoint{
					Timestamp: time.Now().Add(time.Duration(i) * time.Second),
					Value:     rand.Float64() * 100,
					Tags: map[string]string{
						"generator": fmt.Sprintf("gen_%d", id),
						"metric":    "demo_metric",
					},
				}

				mu.Lock()
				points = append(points, point)
				mu.Unlock()

				if i%20 == 0 {
					fmt.Printf("  Generator %d: %d points生成\n", id, i)
				}
			}
		}(g)
	}

	wg.Wait()

	fmt.Printf("\n  ✓ デモ完了: %d 時系列データポイント生成\n", len(points))

	// 簡単な集計
	mu.Lock()
	defer mu.Unlock()

	if len(points) > 0 {
		sum := 0.0
		min, max := points[0].Value, points[0].Value

		for _, p := range points {
			sum += p.Value
			if p.Value < min {
				min = p.Value
			}
			if p.Value > max {
				max = p.Value
			}
		}

		fmt.Printf("\n  📊 統計:\n")
		fmt.Printf("    平均: %.2f\n", sum/float64(len(points)))
		fmt.Printf("    最小: %.2f\n", min)
		fmt.Printf("    最大: %.2f\n", max)
	}
}

// simulateWindowAnalytics - ウィンドウ分析のシミュレーション
func (i *InfluxDBTimeSeries) simulateWindowAnalytics() {
	// 仮想時系列データ生成
	data := make([]float64, 100)
	for i := range data {
		data[i] = 50 + 30*math.Sin(float64(i)/10) + rand.Float64()*10
	}

	// 移動平均計算
	windowSize := 5
	movingAvg := make([]float64, len(data)-windowSize+1)
	for i := 0; i < len(movingAvg); i++ {
		sum := 0.0
		for j := 0; j < windowSize; j++ {
			sum += data[i+j]
		}
		movingAvg[i] = sum / float64(windowSize)
	}

	fmt.Println("\n  📊 ウィンドウ分析結果（シミュレーション）:")
	fmt.Printf("    データ数: %d\n", len(data))
	fmt.Printf("    移動平均ウィンドウ: %d\n", windowSize)
	fmt.Printf("    最初の5つの移動平均: %.2f, %.2f, %.2f, %.2f, %.2f\n",
		movingAvg[0], movingAvg[1], movingAvg[2], movingAvg[3], movingAvg[4])
}

// Close - リソースクリーンアップ
func (i *InfluxDBTimeSeries) Close() {
	if i.client != nil {
		i.client.Close()
	}
}