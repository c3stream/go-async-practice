// Package interactive provides interactive learning and battle systems for Go concurrency
package interactive

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// BattleSystem represents the real-time concurrency battle system
type BattleSystem struct {
	players    map[string]*BattlePlayer
	battles    map[string]*Battle
	matchQueue chan *BattlePlayer
	mu         sync.RWMutex
}

// BattlePlayer represents a player in the battle system
type BattlePlayer struct {
	ID          string
	Name        string
	Level       int
	Rating      int
	Wins        int32
	Losses      int32
	CurrentHP   int32
	MaxHP       int32
	Skills      []Skill
	mu          sync.RWMutex
	battleStats *BattleStats
}

// Skill represents a concurrent programming skill in battle
type Skill struct {
	Name        string
	Type        SkillType
	Damage      int32
	Cooldown    time.Duration
	LastUsed    time.Time
	Goroutines  int // Number of goroutines spawned
	Complexity  int // Algorithm complexity factor
}

// SkillType represents the type of concurrent skill
type SkillType int

const (
	SkillParallelAttack SkillType = iota
	SkillChannelBurst
	SkillMutexShield
	SkillContextCancel
	SkillSelectDefense
	SkillWorkerPoolStrike
	SkillPipelineCombo
	SkillAtomicCounter
)

// Battle represents an active battle between two players
type Battle struct {
	ID          string
	Player1     *BattlePlayer
	Player2     *BattlePlayer
	StartTime   time.Time
	EndTime     time.Time
	Winner      *BattlePlayer
	BattleLog   []BattleEvent
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	turnChan    chan Turn
	resultChan  chan BattleResult
}

// BattleEvent represents an event in the battle
type BattleEvent struct {
	Timestamp   time.Time
	Player      string
	Action      string
	Damage      int32
	Effect      string
	Goroutines  int
	Performance float64 // Execution time in ms
}

// Turn represents a player's turn in battle
type Turn struct {
	Player    *BattlePlayer
	Skill     Skill
	Target    *BattlePlayer
	Timestamp time.Time
}

// BattleResult represents the outcome of a battle
type BattleResult struct {
	Winner       *BattlePlayer
	Loser        *BattlePlayer
	Duration     time.Duration
	TotalDamage  int32
	SkillsUsed   int
	MaxGoroutines int
	Performance  map[string]float64
}

// BattleStats tracks detailed battle statistics
type BattleStats struct {
	TotalDamageDealt   int64
	TotalDamageReceived int64
	SkillAccuracy      float64
	AverageResponseTime time.Duration
	CriticalHits       int32
	PerfectTimings     int32
	ConcurrencyScore   int32
}

// NewBattleSystem creates a new battle system
func NewBattleSystem() *BattleSystem {
	return &BattleSystem{
		players:    make(map[string]*BattlePlayer),
		battles:    make(map[string]*Battle),
		matchQueue: make(chan *BattlePlayer, 100),
	}
}

// CreatePlayer creates a new battle player
func (bs *BattleSystem) CreatePlayer(name string, level int) *BattlePlayer {
	playerID := fmt.Sprintf("player_%d", time.Now().UnixNano())
	hp := int32(100 + level*20)

	player := &BattlePlayer{
		ID:        playerID,
		Name:      name,
		Level:     level,
		Rating:    1000,
		CurrentHP: hp,
		MaxHP:     hp,
		Skills:    bs.generateSkills(level),
		battleStats: &BattleStats{},
	}

	bs.mu.Lock()
	bs.players[playerID] = player
	bs.mu.Unlock()

	return player
}

// generateSkills generates skills based on player level
func (bs *BattleSystem) generateSkills(level int) []Skill {
	skills := []Skill{
		{
			Name:       "Parallel Strike",
			Type:       SkillParallelAttack,
			Damage:     15,
			Cooldown:   2 * time.Second,
			Goroutines: 5,
			Complexity: 1,
		},
		{
			Name:       "Channel Burst",
			Type:       SkillChannelBurst,
			Damage:     20,
			Cooldown:   3 * time.Second,
			Goroutines: 3,
			Complexity: 2,
		},
	}

	if level >= 5 {
		skills = append(skills, Skill{
			Name:       "Mutex Shield",
			Type:       SkillMutexShield,
			Damage:     -10, // Healing
			Cooldown:   5 * time.Second,
			Goroutines: 1,
			Complexity: 1,
		})
	}

	if level >= 10 {
		skills = append(skills, Skill{
			Name:       "Worker Pool Strike",
			Type:       SkillWorkerPoolStrike,
			Damage:     35,
			Cooldown:   4 * time.Second,
			Goroutines: 10,
			Complexity: 3,
		})
	}

	if level >= 15 {
		skills = append(skills, Skill{
			Name:       "Pipeline Combo",
			Type:       SkillPipelineCombo,
			Damage:     50,
			Cooldown:   6 * time.Second,
			Goroutines: 20,
			Complexity: 5,
		})
	}

	return skills
}

// StartBattle starts a battle between two players
func (bs *BattleSystem) StartBattle(player1, player2 *BattlePlayer) *Battle {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	battleID := fmt.Sprintf("battle_%d", time.Now().UnixNano())
	battle := &Battle{
		ID:         battleID,
		Player1:    player1,
		Player2:    player2,
		StartTime:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		turnChan:   make(chan Turn, 10),
		resultChan: make(chan BattleResult, 1),
		BattleLog:  make([]BattleEvent, 0),
	}

	bs.mu.Lock()
	bs.battles[battleID] = battle
	bs.mu.Unlock()

	// Start the battle goroutine
	go battle.run()

	return battle
}

// run executes the battle logic
func (b *Battle) run() {
	defer b.cancel()

	// Reset HP
	atomic.StoreInt32(&b.Player1.CurrentHP, b.Player1.MaxHP)
	atomic.StoreInt32(&b.Player2.CurrentHP, b.Player2.MaxHP)

	// Battle loop
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var wg sync.WaitGroup
	battleActive := true

	// Player 1 AI goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.playerAI(b.Player1, b.Player2)
	}()

	// Player 2 AI goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.playerAI(b.Player2, b.Player1)
	}()

	// Main battle loop
	for battleActive {
		select {
		case <-b.ctx.Done():
			battleActive = false

		case turn := <-b.turnChan:
			b.processTurn(turn)

			// Check for winner
			if atomic.LoadInt32(&b.Player1.CurrentHP) <= 0 {
				b.Winner = b.Player2
				battleActive = false
			} else if atomic.LoadInt32(&b.Player2.CurrentHP) <= 0 {
				b.Winner = b.Player1
				battleActive = false
			}

		case <-ticker.C:
			// Periodic status check
			if !battleActive {
				break
			}
		}
	}

	// Wait for AI goroutines to finish
	wg.Wait()

	// Calculate final result
	b.EndTime = time.Now()
	result := b.calculateResult()
	b.resultChan <- result
	close(b.resultChan)
}

// playerAI simulates player actions in battle
func (b *Battle) playerAI(player, opponent *BattlePlayer) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-b.ctx.Done():
			return

		case <-ticker.C:
			if atomic.LoadInt32(&player.CurrentHP) <= 0 {
				return
			}

			// Choose a random available skill
			skill := b.chooseSkill(player)
			if skill == nil {
				continue
			}

			// Send turn
			turn := Turn{
				Player:    player,
				Skill:     *skill,
				Target:    opponent,
				Timestamp: time.Now(),
			}

			select {
			case b.turnChan <- turn:
				// Turn sent successfully
			case <-time.After(100 * time.Millisecond):
				// Turn channel full, skip this turn
			}
		}
	}
}

// chooseSkill selects an available skill for the player
func (b *Battle) chooseSkill(player *BattlePlayer) *Skill {
	player.mu.RLock()
	defer player.mu.RUnlock()

	availableSkills := make([]Skill, 0)
	now := time.Now()

	for i := range player.Skills {
		if now.Sub(player.Skills[i].LastUsed) >= player.Skills[i].Cooldown {
			availableSkills = append(availableSkills, player.Skills[i])
		}
	}

	if len(availableSkills) == 0 {
		return nil
	}

	// Choose random skill with complexity weighting
	totalWeight := 0
	for _, skill := range availableSkills {
		totalWeight += skill.Complexity
	}

	if totalWeight == 0 {
		return &availableSkills[0]
	}

	randWeight := rand.Intn(totalWeight)
	currentWeight := 0

	for i := range availableSkills {
		currentWeight += availableSkills[i].Complexity
		if randWeight < currentWeight {
			return &availableSkills[i]
		}
	}

	return &availableSkills[len(availableSkills)-1]
}

// processTurn processes a player's turn
func (b *Battle) processTurn(turn Turn) {
	start := time.Now()

	// Update skill cooldown
	for i := range turn.Player.Skills {
		if turn.Player.Skills[i].Name == turn.Skill.Name {
			turn.Player.Skills[i].LastUsed = time.Now()
			break
		}
	}

	// Simulate concurrent execution with goroutines
	damage := b.executeSkill(turn.Skill)

	// Apply damage or healing
	if damage > 0 {
		// Damage to opponent
		newHP := atomic.AddInt32(&turn.Target.CurrentHP, -damage)
		if newHP < 0 {
			atomic.StoreInt32(&turn.Target.CurrentHP, 0)
		}

		// Update battle stats
		atomic.AddInt64(&turn.Player.battleStats.TotalDamageDealt, int64(damage))
		atomic.AddInt64(&turn.Target.battleStats.TotalDamageReceived, int64(damage))
	} else if damage < 0 {
		// Healing to self
		healing := -damage
		currentHP := atomic.LoadInt32(&turn.Player.CurrentHP)
		newHP := currentHP + healing
		if newHP > turn.Player.MaxHP {
			newHP = turn.Player.MaxHP
		}
		atomic.StoreInt32(&turn.Player.CurrentHP, newHP)
	}

	// Log the event
	event := BattleEvent{
		Timestamp:   turn.Timestamp,
		Player:      turn.Player.Name,
		Action:      turn.Skill.Name,
		Damage:      damage,
		Effect:      b.getSkillEffect(turn.Skill.Type),
		Goroutines:  turn.Skill.Goroutines,
		Performance: float64(time.Since(start).Microseconds()) / 1000.0,
	}

	b.mu.Lock()
	b.BattleLog = append(b.BattleLog, event)
	b.mu.Unlock()
}

// executeSkill simulates concurrent skill execution
func (b *Battle) executeSkill(skill Skill) int32 {
	baseDamage := skill.Damage

	// Simulate concurrent processing with goroutines
	results := make(chan int32, skill.Goroutines)
	var wg sync.WaitGroup

	for i := 0; i < skill.Goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Simulate processing with complexity
			time.Sleep(time.Duration(skill.Complexity) * time.Millisecond)

			// Random damage variation based on concurrency
			variation := rand.Int31n(5) - 2
			results <- variation
		}(i)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Aggregate results
	totalVariation := int32(0)
	for variation := range results {
		totalVariation += variation
	}

	finalDamage := baseDamage + totalVariation

	// Critical hit chance based on goroutines
	if rand.Float32() < float32(skill.Goroutines)*0.02 {
		finalDamage = int32(float64(finalDamage) * 1.5)
		atomic.AddInt32(&b.Player1.battleStats.CriticalHits, 1)
	}

	return finalDamage
}

// getSkillEffect returns the effect description for a skill type
func (b *Battle) getSkillEffect(skillType SkillType) string {
	effects := map[SkillType]string{
		SkillParallelAttack:    "Multiple concurrent strikes",
		SkillChannelBurst:      "Channel-based damage burst",
		SkillMutexShield:       "Mutex-protected defense",
		SkillContextCancel:     "Context cancellation attack",
		SkillSelectDefense:     "Select-based reactive defense",
		SkillWorkerPoolStrike:  "Worker pool coordinated attack",
		SkillPipelineCombo:     "Pipeline pattern combo",
		SkillAtomicCounter:     "Atomic operation burst",
	}
	return effects[skillType]
}

// calculateResult calculates the final battle result
func (b *Battle) calculateResult() BattleResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	maxGoroutines := 0
	totalDamage := int32(0)
	performanceMap := make(map[string]float64)

	for _, event := range b.BattleLog {
		if event.Goroutines > maxGoroutines {
			maxGoroutines = event.Goroutines
		}
		totalDamage += event.Damage

		if _, exists := performanceMap[event.Player]; !exists {
			performanceMap[event.Player] = 0
		}
		performanceMap[event.Player] += event.Performance
	}

	var loser *BattlePlayer
	if b.Winner == b.Player1 {
		loser = b.Player2
	} else {
		loser = b.Player1
	}

	return BattleResult{
		Winner:        b.Winner,
		Loser:         loser,
		Duration:      b.EndTime.Sub(b.StartTime),
		TotalDamage:   totalDamage,
		SkillsUsed:    len(b.BattleLog),
		MaxGoroutines: maxGoroutines,
		Performance:   performanceMap,
	}
}

// GetBattleResult waits for and returns the battle result
func (b *Battle) GetBattleResult() BattleResult {
	return <-b.resultChan
}

// PrintBattleSummary prints a summary of the battle
func (b *Battle) PrintBattleSummary() {
	fmt.Println("\n===== BATTLE SUMMARY =====")
	fmt.Printf("Battle ID: %s\n", b.ID)
	fmt.Printf("Duration: %v\n", b.EndTime.Sub(b.StartTime))

	if b.Winner != nil {
		fmt.Printf("\n🏆 WINNER: %s (Level %d)\n", b.Winner.Name, b.Winner.Level)
		fmt.Printf("Remaining HP: %d/%d\n", b.Winner.CurrentHP, b.Winner.MaxHP)
	}

	fmt.Println("\n📊 Battle Statistics:")
	fmt.Printf("Total Events: %d\n", len(b.BattleLog))

	// Count skills used by each player
	player1Skills := 0
	player2Skills := 0
	var totalPerf float64

	for _, event := range b.BattleLog {
		if event.Player == b.Player1.Name {
			player1Skills++
		} else {
			player2Skills++
		}
		totalPerf += event.Performance
	}

	fmt.Printf("\n%s: %d skills used\n", b.Player1.Name, player1Skills)
	fmt.Printf("%s: %d skills used\n", b.Player2.Name, player2Skills)

	if len(b.BattleLog) > 0 {
		fmt.Printf("\nAverage skill execution time: %.2f ms\n", totalPerf/float64(len(b.BattleLog)))
	}

	// Top 5 events
	fmt.Println("\n🔥 Key Battle Moments:")
	displayCount := 5
	if len(b.BattleLog) < displayCount {
		displayCount = len(b.BattleLog)
	}

	for i := 0; i < displayCount && i < len(b.BattleLog); i++ {
		event := b.BattleLog[i]
		fmt.Printf("  [%s] %s used %s: %d damage (%d goroutines, %.2fms)\n",
			event.Timestamp.Format("15:04:05"),
			event.Player,
			event.Action,
			event.Damage,
			event.Goroutines,
			event.Performance)
	}

	fmt.Println("\n========================")
}

// RunBattleDemo runs a demonstration battle
func RunBattleDemo() {
	fmt.Println("🎮 Welcome to Go Concurrency Battle Arena!")
	fmt.Println("=========================================")

	bs := NewBattleSystem()

	// Create two players
	player1 := bs.CreatePlayer("AsyncMaster", 10)
	player2 := bs.CreatePlayer("ParallelKnight", 10)

	fmt.Printf("\n⚔️ Battle: %s vs %s\n", player1.Name, player2.Name)
	fmt.Println("Starting in 3...")
	time.Sleep(1 * time.Second)
	fmt.Println("2...")
	time.Sleep(1 * time.Second)
	fmt.Println("1...")
	time.Sleep(1 * time.Second)
	fmt.Println("FIGHT!")

	// Start the battle
	battle := bs.StartBattle(player1, player2)

	// Wait for battle to complete
	result := battle.GetBattleResult()

	// Print results
	battle.PrintBattleSummary()

	if result.Winner != nil {
		// Update player stats
		atomic.AddInt32(&result.Winner.Wins, 1)
		atomic.AddInt32(&result.Loser.Losses, 1)

		// Update ratings (simple ELO)
		winnerNewRating := result.Winner.Rating + 25
		loserNewRating := result.Loser.Rating - 25
		if loserNewRating < 0 {
			loserNewRating = 0
		}

		result.Winner.Rating = winnerNewRating
		result.Loser.Rating = loserNewRating

		fmt.Printf("\n📈 Rating Updates:\n")
		fmt.Printf("%s: %d (+25)\n", result.Winner.Name, winnerNewRating)
		fmt.Printf("%s: %d (-25)\n", result.Loser.Name, loserNewRating)
	}

	fmt.Println("\n🎯 Battle Complete! Thanks for playing!")
}