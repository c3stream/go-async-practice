package interactive

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// TutorialSystem provides interactive step-by-step learning
type TutorialSystem struct {
	tutorials   map[string]*Tutorial
	progress    map[string]*TutorialProgress
	currentUser string
	mu          sync.RWMutex
}

// Tutorial represents a complete tutorial with steps
type Tutorial struct {
	ID          string
	Title       string
	Description string
	Difficulty  int // 1-10
	Topics      []string
	Steps       []TutorialStep
	Quiz        []TutorialQuizQuestion
	Exercises   []Exercise
	Resources   []Resource
}

// TutorialQuizQuestion for knowledge checking
type TutorialQuizQuestion struct {
	Question    string
	Options     []string
	Answer      int
	Explanation string
}

// TutorialStep represents a single step in a tutorial
type TutorialStep struct {
	Title       string
	Content     string
	Code        string
	Explanation string
	Interactive bool
	Checkpoint  func() bool // Validation function
	Hints       []string
}

// TutorialProgress tracks user progress
type TutorialProgress struct {
	TutorialID    string
	CurrentStep   int
	StepsComplete []bool
	QuizScore     int
	StartTime     time.Time
	LastAccessed  time.Time
	Notes         []string
}

// Exercise represents a hands-on exercise
type Exercise struct {
	Title       string
	Description string
	StartCode   string
	Solution    string
	TestCases   []TutorialTestCase
	Hints       []string
}

// TutorialTestCase for validating exercise solutions
type TutorialTestCase struct {
	Input    interface{}
	Expected interface{}
	Description string
}

// Resource for additional learning
type Resource struct {
	Title string
	URL   string
	Type  string // video, article, documentation
}

// NewTutorialSystem creates a new tutorial system
func NewTutorialSystem() *TutorialSystem {
	ts := &TutorialSystem{
		tutorials: make(map[string]*Tutorial),
		progress:  make(map[string]*TutorialProgress),
	}

	// Initialize tutorials
	ts.loadTutorials()

	return ts
}

// loadTutorials initializes all available tutorials
func (ts *TutorialSystem) loadTutorials() {
	// Goroutines Basics Tutorial
	ts.tutorials["goroutines-basics"] = &Tutorial{
		ID:          "goroutines-basics",
		Title:       "Mastering Goroutines",
		Description: "Learn the fundamentals of Go's goroutines",
		Difficulty:  2,
		Topics:      []string{"goroutines", "concurrency", "parallelism"},
		Steps: []TutorialStep{
			{
				Title:   "Understanding Goroutines",
				Content: "Goroutines are lightweight threads managed by the Go runtime.",
				Code: `func main() {
    go sayHello("World")
    time.Sleep(1 * time.Second)
}

func sayHello(name string) {
    fmt.Printf("Hello, %s!\n", name)
}`,
				Explanation: "The 'go' keyword launches a function in a new goroutine.",
				Interactive: true,
				Hints: []string{
					"Goroutines run concurrently",
					"Main function must wait for goroutines to complete",
				},
			},
			{
				Title:   "Goroutine Synchronization",
				Content: "Use WaitGroups to synchronize goroutines.",
				Code: `var wg sync.WaitGroup

func main() {
    wg.Add(1)
    go worker()
    wg.Wait()
}

func worker() {
    defer wg.Done()
    // Do work
}`,
				Explanation: "WaitGroups ensure all goroutines complete before program exits.",
				Interactive: true,
			},
		},
		Quiz: []TutorialQuizQuestion{
			{
				Question: "What keyword is used to start a goroutine?",
				Options:  []string{"run", "go", "async", "spawn"},
				Answer:   1,
				Explanation: "The 'go' keyword launches a function as a goroutine.",
			},
		},
		Exercises: []Exercise{
			{
				Title:       "Create Multiple Goroutines",
				Description: "Launch 5 goroutines that print their ID",
				StartCode: `func main() {
    // Your code here
}`,
				Solution: `func main() {
    var wg sync.WaitGroup
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            fmt.Printf("Goroutine %d\n", id)
        }(i)
    }
    wg.Wait()
}`,
				Hints: []string{"Use a WaitGroup", "Pass loop variable as parameter"},
			},
		},
		Resources: []Resource{
			{
				Title: "Effective Go - Goroutines",
				URL:   "https://golang.org/doc/effective_go#goroutines",
				Type:  "documentation",
			},
		},
	}

	// Channels Tutorial
	ts.tutorials["channels-mastery"] = &Tutorial{
		ID:          "channels-mastery",
		Title:       "Channel Communication Patterns",
		Description: "Master channel-based communication in Go",
		Difficulty:  4,
		Topics:      []string{"channels", "communication", "synchronization"},
		Steps: []TutorialStep{
			{
				Title:   "Channel Basics",
				Content: "Channels are Go's way to communicate between goroutines.",
				Code: `ch := make(chan int)

// Send
go func() {
    ch <- 42
}()

// Receive
value := <-ch`,
				Explanation: "Channels provide safe communication between goroutines.",
				Interactive: true,
			},
			{
				Title:   "Buffered Channels",
				Content: "Buffered channels allow asynchronous communication.",
				Code: `ch := make(chan int, 3) // Buffer size 3

ch <- 1 // Doesn't block
ch <- 2 // Doesn't block
ch <- 3 // Doesn't block
// ch <- 4 would block`,
				Explanation: "Buffered channels can hold values without immediate receiver.",
				Interactive: true,
			},
			{
				Title:   "Select Statement",
				Content: "Select allows waiting on multiple channel operations.",
				Code: `select {
case msg1 := <-ch1:
    fmt.Println("Received from ch1:", msg1)
case msg2 := <-ch2:
    fmt.Println("Received from ch2:", msg2)
case <-timeout:
    fmt.Println("Timeout!")
}`,
				Explanation: "Select chooses from multiple channel operations.",
				Interactive: true,
			},
		},
	}

	// Advanced Patterns Tutorial
	ts.tutorials["advanced-patterns"] = &Tutorial{
		ID:          "advanced-patterns",
		Title:       "Advanced Concurrency Patterns",
		Description: "Learn worker pools, pipelines, and fan-in/fan-out",
		Difficulty:  7,
		Topics:      []string{"worker-pool", "pipeline", "fan-in", "fan-out"},
		Steps: []TutorialStep{
			{
				Title:   "Worker Pool Pattern",
				Content: "Distribute work across a fixed number of workers.",
				Code: `func workerPool(jobs <-chan int, results chan<- int) {
    for job := range jobs {
        results <- process(job)
    }
}

func main() {
    jobs := make(chan int, 100)
    results := make(chan int, 100)

    // Start workers
    for w := 1; w <= 3; w++ {
        go workerPool(jobs, results)
    }

    // Send work
    for j := 1; j <= 9; j++ {
        jobs <- j
    }
    close(jobs)
}`,
				Explanation: "Worker pools limit concurrent operations and manage resources.",
				Interactive: true,
			},
			{
				Title:   "Pipeline Pattern",
				Content: "Chain operations through channels.",
				Code: `func generate(nums ...int) <-chan int {
    out := make(chan int)
    go func() {
        for _, n := range nums {
            out <- n
        }
        close(out)
    }()
    return out
}

func square(in <-chan int) <-chan int {
    out := make(chan int)
    go func() {
        for n := range in {
            out <- n * n
        }
        close(out)
    }()
    return out
}`,
				Explanation: "Pipelines allow composing operations through channels.",
				Interactive: true,
			},
		},
	}

	// Distributed Systems Tutorial
	ts.tutorials["distributed-systems"] = &Tutorial{
		ID:          "distributed-systems",
		Title:       "Distributed Systems Patterns",
		Description: "Learn distributed locks, consensus, and coordination",
		Difficulty:  9,
		Topics:      []string{"distributed", "consensus", "coordination", "locks"},
		Steps: []TutorialStep{
			{
				Title:   "Distributed Locking",
				Content: "Implement distributed locks with Redis.",
				Code: `func acquireLock(client *redis.Client, key string, ttl time.Duration) (string, error) {
    token := generateToken()
    success, err := client.SetNX(ctx, key, token, ttl).Result()
    if !success {
        return "", ErrLockNotAcquired
    }
    return token, nil
}

func releaseLock(client *redis.Client, key, token string) error {
    // Use Lua script for atomic check-and-delete
    script := redis.NewScript(` + "`" + `
        if redis.call("get", KEYS[1]) == ARGV[1] then
            return redis.call("del", KEYS[1])
        else
            return 0
        end
    ` + "`" + `)
    return script.Run(ctx, client, []string{key}, token).Err()
}`,
				Explanation: "Distributed locks coordinate access across multiple nodes.",
				Interactive: true,
			},
		},
	}
}

// StartTutorial begins a tutorial session
func (ts *TutorialSystem) StartTutorial(tutorialID string) error {
	ts.mu.RLock()
	tutorial, exists := ts.tutorials[tutorialID]
	ts.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tutorial %s not found", tutorialID)
	}

	// Initialize or load progress
	ts.mu.Lock()
	if _, exists := ts.progress[tutorialID]; !exists {
		ts.progress[tutorialID] = &TutorialProgress{
			TutorialID:    tutorialID,
			CurrentStep:   0,
			StepsComplete: make([]bool, len(tutorial.Steps)),
			StartTime:     time.Now(),
			LastAccessed:  time.Now(),
		}
	}
	ts.mu.Unlock()

	// Start interactive session
	ts.runInteractiveSession(tutorial)

	return nil
}

// runInteractiveSession runs the tutorial interactively
func (ts *TutorialSystem) runInteractiveSession(tutorial *Tutorial) {
	scanner := bufio.NewScanner(os.Stdin)
	progress := ts.progress[tutorial.ID]

	fmt.Printf("\n🎓 Starting Tutorial: %s\n", tutorial.Title)
	fmt.Printf("📚 %s\n", tutorial.Description)
	fmt.Printf("⭐ Difficulty: %d/10\n", tutorial.Difficulty)
	fmt.Printf("📊 Progress: %d/%d steps completed\n\n", progress.CurrentStep, len(tutorial.Steps))

	for progress.CurrentStep < len(tutorial.Steps) {
		step := tutorial.Steps[progress.CurrentStep]

		// Display step
		fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
		fmt.Printf("📍 Step %d: %s\n", progress.CurrentStep+1, step.Title)
		fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

		fmt.Println(step.Content)

		if step.Code != "" {
			fmt.Println("\n📝 Code Example:")
			fmt.Println("```go")
			fmt.Println(step.Code)
			fmt.Println("```")
		}

		if step.Explanation != "" {
			fmt.Printf("\n💡 Explanation: %s\n", step.Explanation)
		}

		if step.Interactive {
			fmt.Println("\n🔧 Try it yourself!")
			fmt.Print("Press Enter when ready to continue, or type 'hint' for a hint: ")
			scanner.Scan()
			input := strings.TrimSpace(scanner.Text())

			if input == "hint" && len(step.Hints) > 0 {
				fmt.Printf("💡 Hint: %s\n", step.Hints[0])
				fmt.Print("Press Enter to continue: ")
				scanner.Scan()
			}
		}

		// Mark step as complete
		progress.StepsComplete[progress.CurrentStep] = true
		progress.CurrentStep++
		progress.LastAccessed = time.Now()

		// Save progress
		ts.saveProgress(tutorial.ID, progress)

		// Check if user wants to continue
		if progress.CurrentStep < len(tutorial.Steps) {
			fmt.Print("\n➡️  Continue to next step? (y/n/q): ")
			scanner.Scan()
			response := strings.ToLower(strings.TrimSpace(scanner.Text()))

			if response == "n" || response == "q" {
				fmt.Println("\n📚 Tutorial paused. Your progress has been saved!")
				return
			}
		}
	}

	fmt.Println("\n🎉 Congratulations! You've completed the tutorial!")

	// Offer quiz
	if len(tutorial.Quiz) > 0 {
		fmt.Print("\n📝 Ready to take the quiz? (y/n): ")
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) == "y" {
			ts.runQuiz(tutorial)
		}
	}

	// Offer exercises
	if len(tutorial.Exercises) > 0 {
		fmt.Print("\n💪 Want to try the exercises? (y/n): ")
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) == "y" {
			ts.runExercises(tutorial)
		}
	}
}

// runQuiz runs the tutorial quiz
func (ts *TutorialSystem) runQuiz(tutorial *Tutorial) {
	scanner := bufio.NewScanner(os.Stdin)
	score := 0

	fmt.Println("\n📝 Quiz Time!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, q := range tutorial.Quiz {
		fmt.Printf("\nQuestion %d: %s\n", i+1, q.Question)

		for j, opt := range q.Options {
			fmt.Printf("%d) %s\n", j+1, opt)
		}

		fmt.Print("Your answer (1-4): ")
		scanner.Scan()
		var answer int
		fmt.Sscanf(scanner.Text(), "%d", &answer)

		if answer-1 == q.Answer {
			fmt.Println("✅ Correct!")
			score++
		} else {
			fmt.Printf("❌ Incorrect. %s\n", q.Explanation)
		}
	}

	fmt.Printf("\n🏆 Quiz Complete! Score: %d/%d\n", score, len(tutorial.Quiz))

	// Update progress
	ts.mu.Lock()
	if progress, exists := ts.progress[tutorial.ID]; exists {
		progress.QuizScore = score
	}
	ts.mu.Unlock()
}

// runExercises runs tutorial exercises
func (ts *TutorialSystem) runExercises(tutorial *Tutorial) {
	fmt.Println("\n💪 Exercise Time!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for i, ex := range tutorial.Exercises {
		fmt.Printf("\nExercise %d: %s\n", i+1, ex.Title)
		fmt.Println(ex.Description)

		fmt.Println("\nStarter Code:")
		fmt.Println("```go")
		fmt.Println(ex.StartCode)
		fmt.Println("```")

		fmt.Println("\n📝 Implement your solution and test it!")

		if len(ex.Hints) > 0 {
			fmt.Printf("💡 Hint: %s\n", ex.Hints[0])
		}

		fmt.Println("\nWhen ready, compare with the solution:")
		fmt.Print("Show solution? (y/n): ")

		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()
		if strings.ToLower(strings.TrimSpace(scanner.Text())) == "y" {
			fmt.Println("\n✨ Solution:")
			fmt.Println("```go")
			fmt.Println(ex.Solution)
			fmt.Println("```")
		}
	}
}

// saveProgress saves user progress
func (ts *TutorialSystem) saveProgress(tutorialID string, progress *TutorialProgress) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.progress[tutorialID] = progress
}

// GetProgress returns user progress for a tutorial
func (ts *TutorialSystem) GetProgress(tutorialID string) *TutorialProgress {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return ts.progress[tutorialID]
}

// ListTutorials lists all available tutorials
func (ts *TutorialSystem) ListTutorials() {
	fmt.Println("\n📚 Available Tutorials:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	for _, tutorial := range ts.tutorials {
		progress := ts.GetProgress(tutorial.ID)

		progressStr := "Not Started"
		if progress != nil {
			completed := 0
			for _, done := range progress.StepsComplete {
				if done {
					completed++
				}
			}
			progressStr = fmt.Sprintf("%d/%d steps", completed, len(tutorial.Steps))
		}

		difficulty := strings.Repeat("⭐", tutorial.Difficulty/2)
		if tutorial.Difficulty%2 == 1 {
			difficulty += "☆"
		}

		fmt.Printf("\n🎯 %s (%s)\n", tutorial.Title, tutorial.ID)
		fmt.Printf("   %s\n", tutorial.Description)
		fmt.Printf("   Difficulty: %s (%d/10)\n", difficulty, tutorial.Difficulty)
		fmt.Printf("   Progress: %s\n", progressStr)
		fmt.Printf("   Topics: %s\n", strings.Join(tutorial.Topics, ", "))
	}

	fmt.Println("\nTo start a tutorial, use: go run cmd/runner/main.go -mode=tutorial -tutorial=<id>")
}