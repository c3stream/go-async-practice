package tracker

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProgressTracker 学習進捗を追跡
type ProgressTracker struct {
	mu          sync.RWMutex
	userProfile UserProfile
	lessons     map[string]*LessonProgress
	challenges  map[string]*ChallengeProgress
	achievements []Achievement
	dataFile    string
}

// UserProfile ユーザープロファイル
type UserProfile struct {
	Name         string    `json:"name"`
	StartDate    time.Time `json:"start_date"`
	LastAccess   time.Time `json:"last_access"`
	TotalTime    time.Duration `json:"total_time"`
	Level        int       `json:"level"`
	XP           int       `json:"xp"`
	CurrentStreak int      `json:"current_streak"`
	MaxStreak    int       `json:"max_streak"`
}

// LessonProgress レッスンの進捗
type LessonProgress struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Completed   bool      `json:"completed"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Attempts    int       `json:"attempts"`
	TimeSpent   time.Duration `json:"time_spent"`
	Score       int       `json:"score"`
	Notes       []string  `json:"notes"`
}

// ChallengeProgress チャレンジの進捗
type ChallengeProgress struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Status      string    `json:"status"` // "not_started", "in_progress", "completed"
	Attempts    int       `json:"attempts"`
	BestScore   int       `json:"best_score"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Solutions   []Solution `json:"solutions"`
}

// Solution 解答の記録
type Solution struct {
	Timestamp time.Time `json:"timestamp"`
	Code      string    `json:"code"`
	Score     int       `json:"score"`
	Feedback  string    `json:"feedback"`
	Passed    bool      `json:"passed"`
}

// Achievement 実績
type Achievement struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	UnlockedAt  time.Time `json:"unlocked_at"`
	XPReward    int       `json:"xp_reward"`
}

// NewProgressTracker 新しいトラッカーを作成
func NewProgressTracker(name string) *ProgressTracker {
	homeDir, _ := os.UserHomeDir()
	dataFile := filepath.Join(homeDir, ".go-async-practice", "progress.json")

	pt := &ProgressTracker{
		userProfile: UserProfile{
			Name:      name,
			StartDate: time.Now(),
			Level:     1,
		},
		lessons:    make(map[string]*LessonProgress),
		challenges: make(map[string]*ChallengeProgress),
		achievements: []Achievement{},
		dataFile:   dataFile,
	}

	// データファイルから読み込み
	pt.Load()

	// レッスンとチャレンジを初期化
	pt.initializeLessons()
	pt.initializeChallenges()

	return pt
}

// initializeLessons レッスンを初期化
func (pt *ProgressTracker) initializeLessons() {
	lessons := []struct {
		id   string
		name string
	}{
		{"01_goroutine", "Goroutine基礎"},
		{"02_race", "レース条件"},
		{"03_channel", "Channel基礎"},
		{"04_select", "Select文"},
		{"05_context", "Context"},
		{"06_timeout", "タイムアウト"},
		{"07_nonblocking", "非ブロッキング"},
		{"08_worker", "Worker Pool"},
		{"09_fanin", "Fan-In/Fan-Out"},
		{"10_pipeline", "Pipeline"},
		{"11_semaphore", "Semaphore"},
		{"12_circuit", "Circuit Breaker"},
		{"13_pubsub", "Pub/Sub"},
		{"14_bounded", "Bounded Parallelism"},
		{"15_retry", "Retry Pattern"},
		{"16_batch", "Batch Processing"},
	}

	for _, l := range lessons {
		if _, exists := pt.lessons[l.id]; !exists {
			pt.lessons[l.id] = &LessonProgress{
				ID:   l.id,
				Name: l.name,
			}
		}
	}
}

// initializeChallenges チャレンジを初期化
func (pt *ProgressTracker) initializeChallenges() {
	challenges := []struct {
		id   string
		name string
	}{
		{"deadlock", "デッドロック修正"},
		{"race", "レース条件修正"},
		{"leak", "ゴルーチンリーク修正"},
		{"ratelimit", "レート制限実装"},
	}

	for _, c := range challenges {
		if _, exists := pt.challenges[c.id]; !exists {
			pt.challenges[c.id] = &ChallengeProgress{
				ID:     c.id,
				Name:   c.name,
				Status: "not_started",
			}
		}
	}
}

// StartLesson レッスンを開始
func (pt *ProgressTracker) StartLesson(lessonID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if lesson, ok := pt.lessons[lessonID]; ok {
		lesson.Attempts++
		pt.userProfile.LastAccess = time.Now()
	}
}

// CompleteLesson レッスンを完了
func (pt *ProgressTracker) CompleteLesson(lessonID string, score int) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if lesson, ok := pt.lessons[lessonID]; ok {
		if !lesson.Completed {
			lesson.Completed = true
			lesson.CompletedAt = time.Now()
			lesson.Score = score

			// XPを追加
			xpGained := score * 10
			pt.userProfile.XP += xpGained

			// レベルチェック
			pt.checkLevelUp()

			// 実績チェック
			pt.checkAchievements()

			fmt.Printf("🎉 レッスン完了！ +%dXP\n", xpGained)
		}
	}
}

// StartChallenge チャレンジを開始
func (pt *ProgressTracker) StartChallenge(challengeID string) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if challenge, ok := pt.challenges[challengeID]; ok {
		if challenge.Status == "not_started" {
			challenge.Status = "in_progress"
		}
		challenge.Attempts++
		pt.userProfile.LastAccess = time.Now()
	}
}

// SubmitChallengeSolution チャレンジの解答を提出
func (pt *ProgressTracker) SubmitChallengeSolution(challengeID string, code string, score int, passed bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	if challenge, ok := pt.challenges[challengeID]; ok {
		solution := Solution{
			Timestamp: time.Now(),
			Code:      code,
			Score:     score,
			Passed:    passed,
		}

		challenge.Solutions = append(challenge.Solutions, solution)

		if passed && challenge.Status != "completed" {
			challenge.Status = "completed"
			challenge.CompletedAt = time.Now()
			challenge.BestScore = score

			// XPを追加
			xpGained := score * 20
			pt.userProfile.XP += xpGained

			// レベルチェック
			pt.checkLevelUp()

			// 実績チェック
			pt.checkAchievements()

			fmt.Printf("🏆 チャレンジクリア！ +%dXP\n", xpGained)
		} else if score > challenge.BestScore {
			challenge.BestScore = score
		}
	}
}

// checkLevelUp レベルアップをチェック
func (pt *ProgressTracker) checkLevelUp() {
	requiredXP := pt.userProfile.Level * 100

	for pt.userProfile.XP >= requiredXP {
		pt.userProfile.XP -= requiredXP
		pt.userProfile.Level++
		fmt.Printf("🎊 レベルアップ！ Level %d\n", pt.userProfile.Level)

		// 次のレベルに必要なXPを更新
		requiredXP = pt.userProfile.Level * 100
	}
}

// checkAchievements 実績をチェック
func (pt *ProgressTracker) checkAchievements() {
	// 初心者: 最初のレッスンを完了
	pt.checkAndUnlockAchievement("beginner", "初心者", "最初のレッスンを完了", "🌱", 50)

	// 探究者: 5つのレッスンを完了
	completedLessons := 0
	for _, lesson := range pt.lessons {
		if lesson.Completed {
			completedLessons++
		}
	}
	if completedLessons >= 5 {
		pt.checkAndUnlockAchievement("explorer", "探究者", "5つのレッスンを完了", "🔍", 100)
	}

	// マスター: すべてのレッスンを完了
	if completedLessons == len(pt.lessons) {
		pt.checkAndUnlockAchievement("master", "マスター", "すべてのレッスンを完了", "👨‍🎓", 500)
	}

	// チャレンジャー: 最初のチャレンジをクリア
	for _, challenge := range pt.challenges {
		if challenge.Status == "completed" {
			pt.checkAndUnlockAchievement("challenger", "チャレンジャー", "最初のチャレンジをクリア", "💪", 100)
			break
		}
	}

	// 征服者: すべてのチャレンジをクリア
	completedChallenges := 0
	for _, challenge := range pt.challenges {
		if challenge.Status == "completed" {
			completedChallenges++
		}
	}
	if completedChallenges == len(pt.challenges) {
		pt.checkAndUnlockAchievement("conqueror", "征服者", "すべてのチャレンジをクリア", "👑", 1000)
	}
}

// checkAndUnlockAchievement 実績を確認して解除
func (pt *ProgressTracker) checkAndUnlockAchievement(id, name, description, icon string, xp int) {
	for _, achievement := range pt.achievements {
		if achievement.ID == id {
			return // すでに解除済み
		}
	}

	achievement := Achievement{
		ID:          id,
		Name:        name,
		Description: description,
		Icon:        icon,
		UnlockedAt:  time.Now(),
		XPReward:    xp,
	}

	pt.achievements = append(pt.achievements, achievement)
	pt.userProfile.XP += xp

	fmt.Printf("\n🏅 実績解除: %s %s\n", icon, name)
	fmt.Printf("   %s (+%dXP)\n", description, xp)
}

// GetStatistics 統計情報を取得
func (pt *ProgressTracker) GetStatistics() Statistics {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	stats := Statistics{
		TotalLessons:        len(pt.lessons),
		CompletedLessons:    0,
		TotalChallenges:     len(pt.challenges),
		CompletedChallenges: 0,
		TotalAchievements:   len(pt.achievements),
		TotalXP:             pt.userProfile.XP,
		Level:               pt.userProfile.Level,
		StudyDays:           int(time.Since(pt.userProfile.StartDate).Hours() / 24),
	}

	for _, lesson := range pt.lessons {
		if lesson.Completed {
			stats.CompletedLessons++
		}
		stats.TotalAttempts += lesson.Attempts
		stats.TotalTimeSpent += lesson.TimeSpent
	}

	for _, challenge := range pt.challenges {
		if challenge.Status == "completed" {
			stats.CompletedChallenges++
		}
	}

	if stats.CompletedLessons > 0 {
		stats.CompletionRate = float64(stats.CompletedLessons) / float64(stats.TotalLessons) * 100
	}

	return stats
}

// Statistics 統計情報
type Statistics struct {
	TotalLessons        int           `json:"total_lessons"`
	CompletedLessons    int           `json:"completed_lessons"`
	TotalChallenges     int           `json:"total_challenges"`
	CompletedChallenges int           `json:"completed_challenges"`
	TotalAchievements   int           `json:"total_achievements"`
	TotalAttempts       int           `json:"total_attempts"`
	TotalTimeSpent      time.Duration `json:"total_time_spent"`
	CompletionRate      float64       `json:"completion_rate"`
	TotalXP             int           `json:"total_xp"`
	Level               int           `json:"level"`
	StudyDays           int           `json:"study_days"`
}

// PrintReport 進捗レポートを表示
func (pt *ProgressTracker) PrintReport() {
	stats := pt.GetStatistics()

	fmt.Println("\n╔══════════════════════════════════════════════════════╗")
	fmt.Println("║                  📈 学習進捗レポート                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════╝")

	fmt.Printf("\n👤 %s (Level %d)\n", pt.userProfile.Name, pt.userProfile.Level)
	fmt.Printf("📅 学習期間: %d日\n", stats.StudyDays)
	fmt.Printf("⭐ XP: %d\n", stats.TotalXP)

	fmt.Println("\n📚 レッスン進捗:")
	progressBar := makeProgressBar(stats.CompletedLessons, stats.TotalLessons)
	fmt.Printf("  %s %d/%d (%.1f%%)\n", progressBar,
		stats.CompletedLessons, stats.TotalLessons, stats.CompletionRate)

	fmt.Println("\n🎯 チャレンジ進捗:")
	challengeBar := makeProgressBar(stats.CompletedChallenges, stats.TotalChallenges)
	fmt.Printf("  %s %d/%d\n", challengeBar,
		stats.CompletedChallenges, stats.TotalChallenges)

	if len(pt.achievements) > 0 {
		fmt.Println("\n🏅 獲得実績:")
		for _, a := range pt.achievements {
			fmt.Printf("  %s %s - %s\n", a.Icon, a.Name, a.Description)
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 56))
}

// makeProgressBar プログレスバーを生成
func makeProgressBar(current, total int) string {
	if total == 0 {
		return "[----------]"
	}

	barLength := 20
	filled := (current * barLength) / total

	bar := "["
	for i := 0; i < barLength; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"

	return bar
}

// Save 進捗をファイルに保存
func (pt *ProgressTracker) Save() error {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	data := map[string]interface{}{
		"profile":      pt.userProfile,
		"lessons":      pt.lessons,
		"challenges":   pt.challenges,
		"achievements": pt.achievements,
	}

	dir := filepath.Dir(pt.dataFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(pt.dataFile)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// Load 進捗をファイルから読み込み
func (pt *ProgressTracker) Load() error {
	file, err := os.Open(pt.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // ファイルがなければスキップ
		}
		return err
	}
	defer file.Close()

	var data map[string]json.RawMessage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return err
	}

	if profile, ok := data["profile"]; ok {
		json.Unmarshal(profile, &pt.userProfile)
	}
	if lessons, ok := data["lessons"]; ok {
		json.Unmarshal(lessons, &pt.lessons)
	}
	if challenges, ok := data["challenges"]; ok {
		json.Unmarshal(challenges, &pt.challenges)
	}
	if achievements, ok := data["achievements"]; ok {
		json.Unmarshal(achievements, &pt.achievements)
	}

	return nil
}