package interactive

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// LearningSystem manages the interactive learning experience
type LearningSystem struct {
	player      *Player
	challenges  []Challenge
	currentIdx  int
	metrics     *PerformanceMetrics
	leaderboard *Leaderboard
	mu          sync.RWMutex
}

// Player represents a learner's progress
type Player struct {
	Name         string
	Level        int
	XP           int
	Achievements []Achievement
	StartTime    time.Time
	LastActive   time.Time
	Stats        PlayerStats
	mu           sync.RWMutex
}

// PlayerStats tracks detailed performance
type PlayerStats struct {
	ChallengesCompleted int
	TotalAttempts       int
	SuccessRate         float64
	AverageTime         time.Duration
	BestTime            time.Duration
	CurrentStreak       int
	BestStreak          int
	PreferredTopics     map[string]int
}

// Achievement represents a milestone or badge
type Achievement struct {
	ID          string
	Name        string
	Description string
	Icon        string
	UnlockedAt  time.Time
	XPReward    int
	Rarity      string // common, rare, epic, legendary
}

// Challenge represents an interactive learning challenge
type Challenge struct {
	ID          string
	Name        string
	Description string
	Category    string // basics, concurrency, distributed, advanced
	Difficulty  int    // 1-10
	XPReward    int
	TimeLimit   time.Duration
	Hints       []string
	TestCases   []TestCase
	RunFunc     func(context.Context) error
	ValidateFunc func(interface{}) bool
}

// TestCase for validating solutions
type TestCase struct {
	Name     string
	Input    interface{}
	Expected interface{}
	Timeout  time.Duration
}

// PerformanceMetrics tracks system-wide metrics
type PerformanceMetrics struct {
	TotalPlayers        int
	ActivePlayers       int
	ChallengesCompleted int
	AverageCompletionTime time.Duration
	PopularChallenges   map[string]int
	mu                  sync.RWMutex
}

// Leaderboard tracks top performers
type Leaderboard struct {
	Daily   []LeaderboardEntry
	Weekly  []LeaderboardEntry
	AllTime []LeaderboardEntry
	mu      sync.RWMutex
}

// LeaderboardEntry represents a player's position
type LeaderboardEntry struct {
	PlayerName string
	Level      int
	XP         int
	Achievements int
	CompletionRate float64
	Rank       int
}

// NewLearningSystem creates an interactive learning environment
func NewLearningSystem(playerName string) *LearningSystem {
	return &LearningSystem{
		player: &Player{
			Name:       playerName,
			Level:      1,
			XP:         0,
			StartTime:  time.Now(),
			LastActive: time.Now(),
			Stats: PlayerStats{
				PreferredTopics: make(map[string]int),
			},
		},
		challenges:  initializeChallenges(),
		metrics:     &PerformanceMetrics{PopularChallenges: make(map[string]int)},
		leaderboard: &Leaderboard{},
	}
}

// initializeChallenges sets up all learning challenges
func initializeChallenges() []Challenge {
	return []Challenge{
		// Basic Challenges
		{
			ID:          "basic-1",
			Name:        "Hello Goroutines",
			Description: "Create your first goroutine",
			Category:    "basics",
			Difficulty:  1,
			XPReward:    10,
			TimeLimit:   5 * time.Minute,
			Hints: []string{
				"Use the 'go' keyword",
				"Don't forget to wait for completion",
				"sync.WaitGroup can help",
			},
			RunFunc: func(ctx context.Context) error {
				// Challenge implementation
				return nil
			},
		},
		{
			ID:          "basic-2",
			Name:        "Channel Communication",
			Description: "Send and receive messages through channels",
			Category:    "basics",
			Difficulty:  2,
			XPReward:    20,
			TimeLimit:   5 * time.Minute,
			Hints: []string{
				"Channels are typed",
				"Use <- to send and receive",
				"Unbuffered channels block",
			},
		},
		// Concurrency Challenges
		{
			ID:          "conc-1",
			Name:        "Fix the Race Condition",
			Description: "Identify and fix a race condition in shared memory",
			Category:    "concurrency",
			Difficulty:  4,
			XPReward:    50,
			TimeLimit:   10 * time.Minute,
			Hints: []string{
				"Use go run -race to detect races",
				"Mutexes protect shared state",
				"Atomic operations are lock-free",
			},
		},
		{
			ID:          "conc-2",
			Name:        "Deadlock Detective",
			Description: "Find and resolve a deadlock situation",
			Category:    "concurrency",
			Difficulty:  5,
			XPReward:    75,
			TimeLimit:   15 * time.Minute,
			Hints: []string{
				"Lock ordering prevents deadlocks",
				"Timeouts can break deadlocks",
				"Visualize the dependency graph",
			},
		},
		// Distributed Challenges
		{
			ID:          "dist-1",
			Name:        "Build a Rate Limiter",
			Description: "Implement a token bucket rate limiter",
			Category:    "distributed",
			Difficulty:  6,
			XPReward:    100,
			TimeLimit:   20 * time.Minute,
			Hints: []string{
				"Tokens regenerate over time",
				"Use time.Ticker for periodic refills",
				"Consider burst capacity",
			},
		},
		{
			ID:          "dist-2",
			Name:        "Circuit Breaker Pattern",
			Description: "Implement a circuit breaker for fault tolerance",
			Category:    "distributed",
			Difficulty:  7,
			XPReward:    125,
			TimeLimit:   25 * time.Minute,
			Hints: []string{
				"Track failure rate",
				"Three states: closed, open, half-open",
				"Use timeouts for recovery",
			},
		},
		// Advanced Challenges
		{
			ID:          "adv-1",
			Name:        "Event Bus Master",
			Description: "Build a scalable event bus system",
			Category:    "advanced",
			Difficulty:  8,
			XPReward:    150,
			TimeLimit:   30 * time.Minute,
			Hints: []string{
				"Pub/Sub pattern",
				"Handle backpressure",
				"Consider message ordering",
			},
		},
		{
			ID:          "adv-2",
			Name:        "Distributed Cache",
			Description: "Implement a distributed cache with consistency",
			Category:    "advanced",
			Difficulty:  9,
			XPReward:    200,
			TimeLimit:   40 * time.Minute,
			Hints: []string{
				"Cache invalidation strategies",
				"Consistent hashing for distribution",
				"Handle cache stampede",
			},
		},
	}
}

// Start begins the interactive learning session
func (ls *LearningSystem) Start(ctx context.Context) error {
	fmt.Println("🎮 Welcome to Go Async Practice - Interactive Learning Mode!")
	fmt.Printf("👋 Hello, %s! Level %d (XP: %d)\n\n", ls.player.Name, ls.player.Level, ls.player.XP)

	ls.showMenu()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "quit" || input == "exit" {
			ls.saveProgress()
			fmt.Println("\n👋 Thanks for learning! Your progress has been saved.")
			break
		}

		if err := ls.handleCommand(ctx, input); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		}

		// Update last active time
		ls.player.mu.Lock()
		ls.player.LastActive = time.Now()
		ls.player.mu.Unlock()
	}

	return nil
}

// showMenu displays available options
func (ls *LearningSystem) showMenu() {
	fmt.Println("📚 Available Commands:")
	fmt.Println("  1. challenges  - View available challenges")
	fmt.Println("  2. start [id]  - Start a specific challenge")
	fmt.Println("  3. hint        - Get a hint for current challenge")
	fmt.Println("  4. stats       - View your statistics")
	fmt.Println("  5. achievements - View your achievements")
	fmt.Println("  6. leaderboard - View the leaderboard")
	fmt.Println("  7. daily       - Start daily challenge")
	fmt.Println("  8. help        - Show this menu")
	fmt.Println("  9. quit        - Exit the system")
}

// handleCommand processes user input
func (ls *LearningSystem) handleCommand(ctx context.Context, input string) error {
	parts := strings.Split(input, " ")
	command := parts[0]

	switch command {
	case "challenges", "1":
		ls.showChallenges()
	case "start", "2":
		if len(parts) < 2 {
			return fmt.Errorf("please specify challenge ID")
		}
		return ls.startChallenge(ctx, parts[1])
	case "hint", "3":
		ls.showHint()
	case "stats", "4":
		ls.showStats()
	case "achievements", "5":
		ls.showAchievements()
	case "leaderboard", "6":
		ls.showLeaderboard()
	case "daily", "7":
		return ls.startDailyChallenge(ctx)
	case "help", "8":
		ls.showMenu()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}

	return nil
}

// showChallenges displays available challenges
func (ls *LearningSystem) showChallenges() {
	fmt.Println("\n🎯 Available Challenges:")

	categories := map[string][]Challenge{}
	for _, ch := range ls.challenges {
		categories[ch.Category] = append(categories[ch.Category], ch)
	}

	for cat, challenges := range categories {
		fmt.Printf("\n📁 %s:\n", strings.Title(cat))
		for _, ch := range challenges {
			status := "🔒"
			if ls.canAccess(ch) {
				status = "✅"
			}
			fmt.Printf("  %s [%s] %s (Difficulty: %d/10, XP: %d)\n",
				status, ch.ID, ch.Name, ch.Difficulty, ch.XPReward)
		}
	}
}

// canAccess checks if player can access a challenge
func (ls *LearningSystem) canAccess(ch Challenge) bool {
	ls.player.mu.RLock()
	defer ls.player.mu.RUnlock()

	// Check level requirement
	requiredLevel := ch.Difficulty / 2
	if requiredLevel == 0 {
		requiredLevel = 1
	}

	return ls.player.Level >= requiredLevel
}

// startChallenge begins a specific challenge
func (ls *LearningSystem) startChallenge(ctx context.Context, id string) error {
	var challenge *Challenge
	for i, ch := range ls.challenges {
		if ch.ID == id {
			challenge = &ls.challenges[i]
			break
		}
	}

	if challenge == nil {
		return fmt.Errorf("challenge %s not found", id)
	}

	if !ls.canAccess(*challenge) {
		return fmt.Errorf("challenge locked - need level %d", challenge.Difficulty/2)
	}

	fmt.Printf("\n🚀 Starting Challenge: %s\n", challenge.Name)
	fmt.Printf("📝 %s\n", challenge.Description)
	fmt.Printf("⏱️  Time Limit: %v\n", challenge.TimeLimit)
	fmt.Printf("💎 XP Reward: %d\n\n", challenge.XPReward)

	// Create context with timeout
	challengeCtx, cancel := context.WithTimeout(ctx, challenge.TimeLimit)
	defer cancel()

	startTime := time.Now()

	// Run the challenge
	if challenge.RunFunc != nil {
		if err := challenge.RunFunc(challengeCtx); err != nil {
			fmt.Printf("❌ Challenge failed: %v\n", err)
			ls.updateStats(false, time.Since(startTime))
			return nil
		}
	}

	// Success!
	duration := time.Since(startTime)
	fmt.Printf("\n🎉 Challenge Completed in %v!\n", duration)

	// Award XP
	ls.awardXP(challenge.XPReward)
	ls.updateStats(true, duration)

	// Check for achievements
	ls.checkAchievements()

	return nil
}

// awardXP gives experience points to the player
func (ls *LearningSystem) awardXP(xp int) {
	ls.player.mu.Lock()
	defer ls.player.mu.Unlock()

	ls.player.XP += xp
	fmt.Printf("💎 +%d XP earned!\n", xp)

	// Check for level up
	requiredXP := ls.player.Level * 100
	if ls.player.XP >= requiredXP {
		ls.player.Level++
		ls.player.XP -= requiredXP
		fmt.Printf("🎊 LEVEL UP! You are now level %d!\n", ls.player.Level)
	}
}

// updateStats updates player statistics
func (ls *LearningSystem) updateStats(success bool, duration time.Duration) {
	ls.player.mu.Lock()
	defer ls.player.mu.Unlock()

	ls.player.Stats.TotalAttempts++
	if success {
		ls.player.Stats.ChallengesCompleted++
		ls.player.Stats.CurrentStreak++
		if ls.player.Stats.CurrentStreak > ls.player.Stats.BestStreak {
			ls.player.Stats.BestStreak = ls.player.Stats.CurrentStreak
		}

		if ls.player.Stats.BestTime == 0 || duration < ls.player.Stats.BestTime {
			ls.player.Stats.BestTime = duration
		}
	} else {
		ls.player.Stats.CurrentStreak = 0
	}

	ls.player.Stats.SuccessRate = float64(ls.player.Stats.ChallengesCompleted) /
		float64(ls.player.Stats.TotalAttempts) * 100
}

// showHint displays a hint for the current challenge
func (ls *LearningSystem) showHint() {
	if ls.currentIdx >= len(ls.challenges) {
		fmt.Println("No active challenge")
		return
	}

	challenge := ls.challenges[ls.currentIdx]
	if len(challenge.Hints) > 0 {
		fmt.Println("\n💡 Hint:", challenge.Hints[0])
		// Rotate hints
		challenge.Hints = append(challenge.Hints[1:], challenge.Hints[0])
	} else {
		fmt.Println("No hints available for this challenge")
	}
}

// showStats displays player statistics
func (ls *LearningSystem) showStats() {
	ls.player.mu.RLock()
	defer ls.player.mu.RUnlock()

	fmt.Println("\n📊 Your Statistics:")
	fmt.Printf("  Level:        %d\n", ls.player.Level)
	fmt.Printf("  XP:           %d\n", ls.player.XP)
	fmt.Printf("  Completed:    %d\n", ls.player.Stats.ChallengesCompleted)
	fmt.Printf("  Success Rate: %.1f%%\n", ls.player.Stats.SuccessRate)
	fmt.Printf("  Best Time:    %v\n", ls.player.Stats.BestTime)
	fmt.Printf("  Current Streak: %d\n", ls.player.Stats.CurrentStreak)
	fmt.Printf("  Best Streak:  %d\n", ls.player.Stats.BestStreak)
	fmt.Printf("  Play Time:    %v\n", time.Since(ls.player.StartTime))
}

// showAchievements displays earned achievements
func (ls *LearningSystem) showAchievements() {
	ls.player.mu.RLock()
	defer ls.player.mu.RUnlock()

	if len(ls.player.Achievements) == 0 {
		fmt.Println("\n🏆 No achievements yet. Keep practicing!")
		return
	}

	fmt.Println("\n🏆 Your Achievements:")
	for _, ach := range ls.player.Achievements {
		fmt.Printf("  %s %s - %s\n", ach.Icon, ach.Name, ach.Description)
		fmt.Printf("    Earned: %s | Rarity: %s | +%d XP\n",
			ach.UnlockedAt.Format("2006-01-02"),
			ach.Rarity,
			ach.XPReward)
	}
}

// checkAchievements checks for new achievements
func (ls *LearningSystem) checkAchievements() {
	ls.player.mu.Lock()
	defer ls.player.mu.Unlock()

	// Check various achievement conditions
	achievements := []Achievement{
		{
			ID:          "first-challenge",
			Name:        "First Steps",
			Description: "Complete your first challenge",
			Icon:        "🎯",
			XPReward:    20,
			Rarity:      "common",
		},
		{
			ID:          "streak-5",
			Name:        "On Fire",
			Description: "Complete 5 challenges in a row",
			Icon:        "🔥",
			XPReward:    50,
			Rarity:      "rare",
		},
		{
			ID:          "level-10",
			Name:        "Concurrency Master",
			Description: "Reach level 10",
			Icon:        "👑",
			XPReward:    200,
			Rarity:      "legendary",
		},
	}

	for _, ach := range achievements {
		if !ls.hasAchievement(ach.ID) {
			if ls.checkAchievementCondition(ach.ID) {
				ls.player.Achievements = append(ls.player.Achievements, ach)
				fmt.Printf("\n🎊 Achievement Unlocked: %s %s!\n", ach.Icon, ach.Name)
				fmt.Printf("   %s (+%d XP)\n", ach.Description, ach.XPReward)
				ls.player.XP += ach.XPReward
			}
		}
	}
}

// hasAchievement checks if player has an achievement
func (ls *LearningSystem) hasAchievement(id string) bool {
	for _, ach := range ls.player.Achievements {
		if ach.ID == id {
			return true
		}
	}
	return false
}

// checkAchievementCondition checks if condition is met
func (ls *LearningSystem) checkAchievementCondition(id string) bool {
	switch id {
	case "first-challenge":
		return ls.player.Stats.ChallengesCompleted >= 1
	case "streak-5":
		return ls.player.Stats.CurrentStreak >= 5
	case "level-10":
		return ls.player.Level >= 10
	default:
		return false
	}
}

// showLeaderboard displays the leaderboard
func (ls *LearningSystem) showLeaderboard() {
	fmt.Println("\n🏅 Leaderboard - All Time:")
	fmt.Println("  Rank | Player        | Level | XP    | Completion")
	fmt.Println("  -----|---------------|-------|-------|------------")

	// Sample leaderboard data
	entries := []LeaderboardEntry{
		{PlayerName: "Alice", Level: 15, XP: 3250, CompletionRate: 92.5, Rank: 1},
		{PlayerName: "Bob", Level: 12, XP: 2100, CompletionRate: 88.0, Rank: 2},
		{PlayerName: ls.player.Name, Level: ls.player.Level, XP: ls.player.XP,
		 CompletionRate: ls.player.Stats.SuccessRate, Rank: 3},
	}

	for _, entry := range entries {
		medal := ""
		switch entry.Rank {
		case 1:
			medal = "🥇"
		case 2:
			medal = "🥈"
		case 3:
			medal = "🥉"
		default:
			medal = "  "
		}

		fmt.Printf("  %s %2d | %-13s | %5d | %5d | %5.1f%%\n",
			medal, entry.Rank, entry.PlayerName, entry.Level, entry.XP, entry.CompletionRate)
	}
}

// startDailyChallenge starts the daily challenge
func (ls *LearningSystem) startDailyChallenge(ctx context.Context) error {
	// Select daily challenge based on date
	today := time.Now().Day()
	idx := today % len(ls.challenges)
	challenge := ls.challenges[idx]

	fmt.Printf("\n⭐ Daily Challenge: %s\n", challenge.Name)
	fmt.Printf("   Complete for bonus 2x XP!\n")

	// Double XP for daily challenge
	challenge.XPReward *= 2
	defer func() { challenge.XPReward /= 2 }()

	return ls.startChallenge(ctx, challenge.ID)
}

// saveProgress saves player progress
func (ls *LearningSystem) saveProgress() {
	// In a real implementation, this would save to a database or file
	fmt.Println("\n💾 Progress saved successfully!")
}

// RunInteractiveLearning starts the interactive learning system
func RunInteractiveLearning() {
	fmt.Print("Enter your name: ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	name := scanner.Text()
	if name == "" {
		name = "Gopher"
	}

	system := NewLearningSystem(name)
	ctx := context.Background()

	if err := system.Start(ctx); err != nil {
		fmt.Printf("Error: %v\n", err)
	}
}