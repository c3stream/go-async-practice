package challenges

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Challenge 22: Time Series Database with Aggregations
//
// 問題点:
// 1. データポイントの順序保証なし
// 2. ダウンサンプリングのバグ
// 3. ウィンドウ集計の不整合
// 4. メモリ効率の問題
// 5. 並行書き込みのデータロス

type TimeSeriesPoint struct {
	Timestamp time.Time
	Value     float64
	Tags      map[string]string
}

type TimeSeries struct {
	Name       string
	Points     []TimeSeriesPoint // 問題: 無制限に成長
	mu         sync.RWMutex
	lastWrite  time.Time
	resolution time.Duration
	retention  time.Duration
}

type TimeSeriesDB struct {
	mu               sync.RWMutex
	series           map[string]*TimeSeries
	writeBuffer      chan TimeSeriesPoint // 問題: 固定サイズ
	aggregations     map[string]*Aggregation
	downsampling     *DownsamplingEngine
	compressionRatio float64
	indexes          map[string]map[string][]string // tag -> value -> series names
	cache            sync.Map                       // 問題: 無制限キャッシュ
}

type Aggregation struct {
	Type     string        // sum, avg, max, min, count
	Window   time.Duration
	Function func([]float64) float64
	mu       sync.RWMutex
	buffer   []TimeSeriesPoint // 問題: ウィンドウ管理なし
}

type DownsamplingEngine struct {
	mu         sync.RWMutex
	rules      map[string]DownsamplingRule
	processing map[string]bool // 問題: メモリリーク
}

type DownsamplingRule struct {
	SourceResolution time.Duration
	TargetResolution time.Duration
	AggregationType  string
	RetentionPeriod  time.Duration
}

type QueryOptions struct {
	Start       time.Time
	End         time.Time
	Resolution  time.Duration
	Aggregation string
	Tags        map[string]string
	Limit       int
}

func NewTimeSeriesDB() *TimeSeriesDB {
	db := &TimeSeriesDB{
		series:      make(map[string]*TimeSeries),
		writeBuffer: make(chan TimeSeriesPoint, 1000),
		aggregations: make(map[string]*Aggregation),
		downsampling: &DownsamplingEngine{
			rules:      make(map[string]DownsamplingRule),
			processing: make(map[string]bool),
		},
		indexes: make(map[string]map[string][]string),
	}

	// 問題: goroutineリーク
	go db.processWrites()

	return db
}

// 問題1: 書き込み処理のデータロス
func (tsdb *TimeSeriesDB) Write(seriesName string, value float64, tags map[string]string) error {
	point := TimeSeriesPoint{
		Timestamp: time.Now(),
		Value:     value,
		Tags:      tags,
	}

	// 問題: ノンブロッキング送信でデータロス
	select {
	case tsdb.writeBuffer <- point:
		// OK
	default:
		// 問題: サイレントにドロップ
		return nil
	}

	// 問題: seriesNameが使われていない
	return nil
}

func (tsdb *TimeSeriesDB) processWrites() {
	for point := range tsdb.writeBuffer {
		// 問題: seriesNameがpointに含まれていない
		// どのシリーズに書き込むか不明

		// 仮にデフォルトシリーズに書き込む
		tsdb.mu.Lock()
		series, exists := tsdb.series["default"]
		if !exists {
			series = &TimeSeries{
				Name:       "default",
				Points:     []TimeSeriesPoint{},
				resolution: 1 * time.Second,
				retention:  24 * time.Hour,
			}
			tsdb.series["default"] = series
		}
		tsdb.mu.Unlock()

		// 問題2: 時系列順序の保証なし
		series.mu.Lock()
		series.Points = append(series.Points, point)
		series.lastWrite = time.Now()
		series.mu.Unlock()

		// 問題: インデックス更新なし
	}
}

// 問題3: クエリ実行
func (tsdb *TimeSeriesDB) Query(opts QueryOptions) ([]TimeSeriesPoint, error) {
	tsdb.mu.RLock()
	defer tsdb.mu.RUnlock()

	results := []TimeSeriesPoint{}

	// 問題: 全シリーズをスキャン（非効率）
	for _, series := range tsdb.series {
		series.mu.RLock()

		// 問題: タイムスタンプでソートされていない前提
		for _, point := range series.Points {
			// 問題: 時間範囲チェックが不正確
			if point.Timestamp.After(opts.Start) && point.Timestamp.Before(opts.End) {
				// 問題: タグフィルタリング未実装
				results = append(results, point)
			}
		}

		series.mu.RUnlock()
	}

	// 問題: 結果のソートなし
	// 問題: 集計処理なし
	// 問題: リミット適用なし

	return results, nil
}

// 問題4: ダウンサンプリング
func (tsdb *TimeSeriesDB) Downsample(seriesName string, rule DownsamplingRule) error {
	tsdb.mu.RLock()
	series, exists := tsdb.series[seriesName]
	tsdb.mu.RUnlock()

	if !exists {
		return fmt.Errorf("series not found")
	}

	// 問題: 処理中フラグの管理
	tsdb.downsampling.mu.Lock()
	if tsdb.downsampling.processing[seriesName] {
		tsdb.downsampling.mu.Unlock()
		return fmt.Errorf("already processing")
	}
	tsdb.downsampling.processing[seriesName] = true
	tsdb.downsampling.mu.Unlock()

	// 問題: deferでフラグクリアを忘れている

	series.mu.Lock()
	defer series.mu.Unlock()

	// 問題: ダウンサンプリングロジックが不完全
	newPoints := []TimeSeriesPoint{}
	windowStart := series.Points[0].Timestamp
	windowPoints := []float64{}

	for _, point := range series.Points {
		// 問題: ウィンドウ境界の計算が不正確
		if point.Timestamp.Sub(windowStart) < rule.TargetResolution {
			windowPoints = append(windowPoints, point.Value)
		} else {
			// 問題: 集計タイプの処理が未実装
			if len(windowPoints) > 0 {
				avgValue := average(windowPoints)
				newPoints = append(newPoints, TimeSeriesPoint{
					Timestamp: windowStart,
					Value:     avgValue,
				})
			}
			windowStart = point.Timestamp
			windowPoints = []float64{point.Value}
		}
	}

	// 問題: 最後のウィンドウ処理忘れ

	series.Points = newPoints

	return nil
}

// 問題5: ウィンドウ集計
func (tsdb *TimeSeriesDB) CreateAggregation(name, aggType string, window time.Duration) {
	agg := &Aggregation{
		Type:   aggType,
		Window: window,
		buffer: []TimeSeriesPoint{},
	}

	// 問題: 集計関数の設定が不完全
	switch aggType {
	case "sum":
		agg.Function = sum
	case "avg":
		agg.Function = average
		// 問題: max, min, count が未実装
	}

	tsdb.mu.Lock()
	tsdb.aggregations[name] = agg
	tsdb.mu.Unlock()
}

func (tsdb *TimeSeriesDB) ApplyAggregation(seriesName, aggName string) ([]TimeSeriesPoint, error) {
	tsdb.mu.RLock()
	series := tsdb.series[seriesName]
	agg := tsdb.aggregations[aggName]
	tsdb.mu.RUnlock()

	if series == nil || agg == nil {
		return nil, fmt.Errorf("series or aggregation not found")
	}

	series.mu.RLock()
	defer series.mu.RUnlock()

	results := []TimeSeriesPoint{}

	// 問題: ウィンドウ処理が不正確
	for i := 0; i < len(series.Points); i++ {
		windowEnd := series.Points[i].Timestamp.Add(agg.Window)
		windowValues := []float64{}

		// 問題: O(n^2)の複雑さ
		for j := i; j < len(series.Points); j++ {
			if series.Points[j].Timestamp.Before(windowEnd) {
				windowValues = append(windowValues, series.Points[j].Value)
			}
		}

		if len(windowValues) > 0 {
			results = append(results, TimeSeriesPoint{
				Timestamp: series.Points[i].Timestamp,
				Value:     agg.Function(windowValues),
			})
		}
	}

	return results, nil
}

// 問題6: データ圧縮
func (tsdb *TimeSeriesDB) Compress(seriesName string) error {
	tsdb.mu.RLock()
	series := tsdb.series[seriesName]
	tsdb.mu.RUnlock()

	if series == nil {
		return fmt.Errorf("series not found")
	}

	series.mu.Lock()
	defer series.mu.Unlock()

	// 問題: 単純なデルタエンコーディングのみ
	if len(series.Points) < 2 {
		return nil
	}

	compressed := []TimeSeriesPoint{series.Points[0]}
	lastValue := series.Points[0].Value

	for i := 1; i < len(series.Points); i++ {
		delta := series.Points[i].Value - lastValue
		// 問題: 精度損失の可能性
		if math.Abs(delta) > 0.01 {
			compressed = append(compressed, TimeSeriesPoint{
				Timestamp: series.Points[i].Timestamp,
				Value:     delta, // 問題: デルタ値を保存（元の値でない）
			})
			lastValue = series.Points[i].Value
		}
	}

	// 問題: 圧縮率の計算なし
	series.Points = compressed

	return nil
}

// Utility functions
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func sum(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total
}

// Challenge: 以下の問題を修正してください
// 1. 時系列データの順序保証
// 2. 効率的なダウンサンプリング
// 3. スライディングウィンドウ集計
// 4. Gorilla圧縮アルゴリズム
// 5. タグベースのインデックス
// 6. データリテンション管理
// 7. 分散書き込み対応
// 8. 時系列予測機能

func RunChallenge22() {
	fmt.Println("Challenge 22: Time Series Database")
	fmt.Println("Fix the time series database implementation")

	tsdb := NewTimeSeriesDB()
	ctx := context.Background()
	_ = ctx

	// メトリクス書き込み
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tsdb.Write(fmt.Sprintf("cpu.usage.core%d", id),
					float64(50+j%50),
					map[string]string{
						"host": fmt.Sprintf("server%d", id),
						"dc":   "us-east-1",
					})
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()

	// クエリ実行
	results, _ := tsdb.Query(QueryOptions{
		Start: time.Now().Add(-1 * time.Hour),
		End:   time.Now(),
		Tags: map[string]string{
			"host": "server1",
		},
	})

	fmt.Printf("Query results: %d points\n", len(results))

	// ダウンサンプリング
	tsdb.Downsample("default", DownsamplingRule{
		SourceResolution: 1 * time.Second,
		TargetResolution: 1 * time.Minute,
		AggregationType:  "avg",
	})

	// ウィンドウ集計
	tsdb.CreateAggregation("1min_avg", "avg", 1*time.Minute)
	aggResults, _ := tsdb.ApplyAggregation("default", "1min_avg")
	fmt.Printf("Aggregation results: %d points\n", len(aggResults))
}