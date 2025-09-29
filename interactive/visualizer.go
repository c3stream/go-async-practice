package interactive

import (
	"fmt"
	"strings"
	"time"
)

// Visualizer provides visual feedback for learning progress
type Visualizer struct {
	width int
}

// NewVisualizer creates a new progress visualizer
func NewVisualizer() *Visualizer {
	return &Visualizer{width: 50}
}

// DrawProgressBar creates a visual progress bar
func (v *Visualizer) DrawProgressBar(current, total int, label string) string {
	if total == 0 {
		return ""
	}

	percentage := float64(current) / float64(total)
	filled := int(percentage * float64(v.width))
	empty := v.width - filled

	bar := fmt.Sprintf("[%s%s] %.1f%% %s",
		strings.Repeat("█", filled),
		strings.Repeat("░", empty),
		percentage*100,
		label)

	return bar
}

// DrawLevelProgress shows XP progress to next level
func (v *Visualizer) DrawLevelProgress(currentXP, requiredXP, level int) {
	fmt.Printf("\n📊 Level %d Progress:\n", level)
	bar := v.DrawProgressBar(currentXP, requiredXP, fmt.Sprintf("(%d/%d XP)", currentXP, requiredXP))
	fmt.Println(bar)
}

// DrawSkillRadar creates a skill radar chart (ASCII art)
func (v *Visualizer) DrawSkillRadar(skills map[string]int) {
	fmt.Println("\n🎯 Skills Radar:")
	fmt.Println("     Concurrency")
	fmt.Println("          |")

	maxLevel := 10
	for skill, level := range skills {
		bar := ""
		for i := 0; i < maxLevel; i++ {
			if i < level {
				bar += "▰"
			} else {
				bar += "▱"
			}
		}
		fmt.Printf("%-15s %s %d/10\n", skill+":", bar, level)
	}
}

// DrawTimeline shows learning journey timeline
func (v *Visualizer) DrawTimeline(events []TimelineEvent) {
	fmt.Println("\n📅 Learning Timeline:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, event := range events {
		marker := "○"
		if event.Type == "achievement" {
			marker = "🏆"
		} else if event.Type == "levelup" {
			marker = "⭐"
		} else if event.Type == "challenge" {
			marker = "✅"
		}

		indent := strings.Repeat(" ", i%3*2)
		fmt.Printf("%s%s %s - %s\n",
			indent,
			marker,
			event.Time.Format("Jan 02"),
			event.Description)

		if i < len(events)-1 {
			fmt.Printf("%s│\n", indent)
		}
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// TimelineEvent represents an event in learning journey
type TimelineEvent struct {
	Time        time.Time
	Type        string // challenge, achievement, levelup
	Description string
}

// DrawChallengeMap shows challenge completion map
func (v *Visualizer) DrawChallengeMap(challenges []ChallengeStatus) {
	fmt.Println("\n🗺️  Challenge Map:")
	fmt.Println("┌─────────────────────────────────────────────────┐")

	// Group by category
	categories := make(map[string][]ChallengeStatus)
	for _, ch := range challenges {
		categories[ch.Category] = append(categories[ch.Category], ch)
	}

	for cat, chs := range categories {
		fmt.Printf("│ %s:\n", strings.ToUpper(cat))
		fmt.Printf("│ ")

		for i, ch := range chs {
			icon := "⬜"
			if ch.Completed {
				icon = "✅"
			} else if ch.InProgress {
				icon = "🔄"
			} else if ch.Locked {
				icon = "🔒"
			}

			fmt.Printf("%s ", icon)

			if (i+1)%10 == 0 {
				fmt.Printf("\n│ ")
			}
		}
		fmt.Println()
	}

	fmt.Println("└─────────────────────────────────────────────────┘")
	fmt.Println("  ✅ Completed  🔄 In Progress  ⬜ Available  🔒 Locked")
}

// ChallengeStatus represents a challenge's completion status
type ChallengeStatus struct {
	ID         string
	Category   string
	Completed  bool
	InProgress bool
	Locked     bool
	Stars      int // 0-3 stars based on performance
}

// DrawStreakCalendar shows activity streak calendar
func (v *Visualizer) DrawStreakCalendar(activities map[time.Time]int) {
	fmt.Println("\n📆 Activity Calendar (Last 30 Days):")
	fmt.Println("  Mon Tue Wed Thu Fri Sat Sun")

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Find the Monday of the week containing the 1st
	daysFromMonday := int(startOfMonth.Weekday() - time.Monday)
	if daysFromMonday < 0 {
		daysFromMonday += 7
	}
	startDate := startOfMonth.AddDate(0, 0, -daysFromMonday)

	// Print calendar
	fmt.Print("  ")
	for d := startDate; d.Month() <= now.Month(); d = d.AddDate(0, 0, 1) {
		if d.Day() == 1 || d.Equal(startDate) {
			// Skip days from previous month
			if d.Month() < now.Month() {
				fmt.Print("    ")
				continue
			}
		}

		// Get activity level for this day
		activity := activities[d]
		cell := "  "

		if activity > 0 {
			switch {
			case activity >= 5:
				cell = "🟩"
			case activity >= 3:
				cell = "🟨"
			case activity >= 1:
				cell = "🟥"
			}
		} else if d.Before(now) || d.Equal(now) {
			cell = "⬜"
		}

		fmt.Printf("%s  ", cell)

		if d.Weekday() == time.Sunday {
			fmt.Print("\n  ")
		}

		if d.Day() == now.Day() && d.Month() == now.Month() {
			break
		}
	}
	fmt.Println("\n  🟩 5+ challenges  🟨 3-4 challenges  🟥 1-2 challenges")
}

// DrawPerformanceGraph shows performance over time
func (v *Visualizer) DrawPerformanceGraph(data []float64, label string) {
	fmt.Printf("\n📈 %s:\n", label)

	if len(data) == 0 {
		fmt.Println("No data available")
		return
	}

	// Find max value for scaling
	maxVal := 0.0
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}

	if maxVal == 0 {
		maxVal = 1
	}

	// Draw graph
	height := 10
	for h := height; h >= 0; h-- {
		threshold := float64(h) / float64(height) * maxVal

		if h == height {
			fmt.Printf("%6.1f │", maxVal)
		} else if h == height/2 {
			fmt.Printf("%6.1f │", maxVal/2)
		} else if h == 0 {
			fmt.Printf("%6.1f │", 0.0)
		} else {
			fmt.Print("       │")
		}

		for _, val := range data {
			if val >= threshold {
				fmt.Print("█")
			} else {
				fmt.Print(" ")
			}
		}
		fmt.Println()
	}

	// X-axis
	fmt.Print("       └")
	fmt.Println(strings.Repeat("─", len(data)))
}

// AnimateSuccess shows success animation
func (v *Visualizer) AnimateSuccess() {
	frames := []string{
		"    ⭐    ",
		"   ⭐⭐   ",
		"  ⭐⭐⭐  ",
		" ⭐⭐⭐⭐ ",
		"⭐⭐⭐⭐⭐",
		"🎉🎊🎉🎊🎉",
	}

	for _, frame := range frames {
		fmt.Print("\r" + frame)
		time.Sleep(100 * time.Millisecond)
	}
	fmt.Println("\r🎉 Success! 🎉")
}

// AnimateLoading shows loading animation
func (v *Visualizer) AnimateLoading(done chan bool) {
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0

	for {
		select {
		case <-done:
			fmt.Print("\r✅ Complete!\n")
			return
		default:
			fmt.Printf("\r%s Loading...", spinners[i%len(spinners)])
			i++
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// DrawLeaderboardPodium creates ASCII podium for top 3
func (v *Visualizer) DrawLeaderboardPodium(top3 []LeaderboardEntry) {
	if len(top3) == 0 {
		return
	}

	fmt.Println("\n🏆 Top Players:")
	fmt.Println()

	// Podium visualization
	fmt.Println("         🥇")
	fmt.Printf("      %s\n", top3[0].PlayerName)
	fmt.Println("    ┌─────────┐")
	fmt.Println("    │    1    │")

	if len(top3) > 1 {
		fmt.Println(" 🥈 ├─────────┤ 🥉")
		fmt.Printf("%s│         │%s\n",
			padCenter(top3[1].PlayerName, 4),
			padCenter(getPlayerName(top3, 2), 4))
		fmt.Println("┌───┼─────────┼───┐")
		fmt.Println("│ 2 │         │ 3 │")
		fmt.Println("└───┴─────────┴───┘")
	} else {
		fmt.Println("    └─────────┘")
	}
}

func padCenter(s string, minWidth int) string {
	if len(s) >= minWidth {
		return s
	}
	padding := minWidth - len(s)
	left := padding / 2
	right := padding - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func getPlayerName(entries []LeaderboardEntry, index int) string {
	if index < len(entries) {
		return entries[index].PlayerName
	}
	return "---"
}

// DrawDifficultyMeter shows challenge difficulty
func (v *Visualizer) DrawDifficultyMeter(difficulty int) {
	fmt.Print("Difficulty: ")

	for i := 1; i <= 10; i++ {
		if i <= difficulty {
			if difficulty <= 3 {
				fmt.Print("🟢")
			} else if difficulty <= 6 {
				fmt.Print("🟡")
			} else if difficulty <= 8 {
				fmt.Print("🟠")
			} else {
				fmt.Print("🔴")
			}
		} else {
			fmt.Print("⚪")
		}
	}

	labels := map[int]string{
		1:  " (Beginner)",
		3:  " (Easy)",
		5:  " (Medium)",
		7:  " (Hard)",
		9:  " (Expert)",
		10: " (Master)",
	}

	if label, ok := labels[difficulty]; ok {
		fmt.Print(label)
	}
	fmt.Println()
}

// ShowWelcomeBanner displays welcome screen
func (v *Visualizer) ShowWelcomeBanner() {
	banner := `
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║     ██████╗  ██████╗      █████╗ ███████╗██╗   ██╗███╗   ██╗║
║    ██╔════╝ ██╔═══██╗    ██╔══██╗██╔════╝╚██╗ ██╔╝████╗  ██║║
║    ██║  ███╗██║   ██║    ███████║███████╗ ╚████╔╝ ██╔██╗ ██║║
║    ██║   ██║██║   ██║    ██╔══██║╚════██║  ╚██╔╝  ██║╚██╗██║║
║    ╚██████╔╝╚██████╔╝    ██║  ██║███████║   ██║   ██║ ╚████║║
║     ╚═════╝  ╚═════╝     ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═══╝║
║                                                              ║
║            🎮 Interactive Concurrency Learning 🎮            ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
`
	fmt.Println(banner)
}