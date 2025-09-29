package challenges

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Challenge 28: Distributed Task Scheduler with Work Stealing
//
// Problems to fix:
// 1. Work stealing causing task duplication
// 2. Priority inversion in task queue
// 3. Deadlocks in worker pool management
// 4. Task dependencies not resolved correctly
// 5. Resource starvation for low priority tasks
// 6. Memory leaks from uncompleted tasks
// 7. Scheduler not scaling with load
// 8. Lost tasks during worker failures

type Task struct {
	ID           string
	Priority     int
	Dependencies []string
	Payload      interface{}
	RetryCount   int
	MaxRetries   int
	Timeout      time.Duration
	CreatedAt    time.Time
	StartedAt    time.Time
	CompletedAt  time.Time
	Status       string // pending, running, completed, failed
	Result       interface{}
	Error        error
	mu           sync.RWMutex
}

type TaskScheduler struct {
	workers      []*Worker
	taskQueue    *PriorityQueue
	pendingTasks map[string]*Task
	runningTasks map[string]*Task
	completedTasks map[string]*Task
	dependencies map[string][]string // TaskID -> Dependencies
	mu           sync.RWMutex
	workerPool   sync.Pool
	ctx          context.Context
	cancel       context.CancelFunc
	metrics      *SchedulerMetrics
}

type Worker struct {
	ID          int
	scheduler   *TaskScheduler
	localQueue  []*Task
	stealIndex  int32 // For work stealing
	isIdle      bool
	currentTask *Task
	mu          sync.Mutex
	stopCh      chan struct{}
}

type PriorityQueue struct {
	tasks []*Task
	mu    sync.Mutex // BUG: Should be RWMutex for reads
}

type SchedulerMetrics struct {
	tasksQueued    int64
	tasksRunning   int64
	tasksCompleted int64
	tasksFailed    int64
	stealAttempts  int64
	stealSuccess   int64
	mu             sync.RWMutex
}

type TaskExecutor func(context.Context, *Task) error

func Challenge28TaskScheduler() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scheduler := &TaskScheduler{
		workers:        make([]*Worker, 4),
		taskQueue:      &PriorityQueue{},
		pendingTasks:   make(map[string]*Task),
		runningTasks:   make(map[string]*Task),
		completedTasks: make(map[string]*Task),
		dependencies:   make(map[string][]string),
		ctx:            ctx,
		cancel:         cancel,
		metrics:        &SchedulerMetrics{},
	}

	// Initialize workers
	for i := 0; i < 4; i++ {
		worker := &Worker{
			ID:         i,
			scheduler:  scheduler,
			localQueue: make([]*Task, 0, 10),
			stopCh:     make(chan struct{}),
		}
		scheduler.workers[i] = worker
		go worker.run()
	}

	// Start scheduler
	go scheduler.schedule()

	var wg sync.WaitGroup

	// Submit tasks with dependencies
	taskGraph := createTaskGraph()
	for _, task := range taskGraph {
		wg.Add(1)
		go func(t *Task) {
			defer wg.Done()
			// BUG: Not checking if dependencies exist
			scheduler.submitTask(t)
		}(task)
	}

	// Submit high priority tasks
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &Task{
				ID:       fmt.Sprintf("high-%d", id),
				Priority: 10, // High priority
				Timeout:  1 * time.Second,
				Status:   "pending",
			}
			scheduler.submitTask(task)
		}(i)
	}

	// Submit low priority tasks
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			task := &Task{
				ID:       fmt.Sprintf("low-%d", id),
				Priority: 1, // Low priority
				Timeout:  1 * time.Second,
				Status:   "pending",
			}
			// BUG: Low priority tasks might starve
			scheduler.submitTask(task)
		}(i)
	}

	// Simulate worker failures
	go func() {
		time.Sleep(500 * time.Millisecond)
		// BUG: Not handling running tasks on failed worker
		close(scheduler.workers[0].stopCh)
		scheduler.workers[0] = nil
	}()

	// Work stealing scenario
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(100 * time.Millisecond):
				// Try to steal work
				scheduler.balanceWork()
			}
		}
	}()

	wg.Wait()
	time.Sleep(2 * time.Second)

	// BUG: Not waiting for all tasks to complete
	// BUG: Not cleaning up resources
}

func (s *TaskScheduler) submitTask(task *Task) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// BUG: Not validating task
	if task.ID == "" {
		task.ID = generateID()
	}

	// Check dependencies
	if len(task.Dependencies) > 0 {
		// BUG: Not checking if dependencies are already completed
		s.dependencies[task.ID] = task.Dependencies
	}

	// BUG: Not checking for duplicate task IDs
	s.pendingTasks[task.ID] = task

	// Add to priority queue
	s.taskQueue.push(task)

	atomic.AddInt64(&s.metrics.tasksQueued, 1)
	return nil
}

func (s *TaskScheduler) schedule() {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.scheduleTasks()
		}
	}
}

func (s *TaskScheduler) scheduleTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get available workers
	availableWorkers := s.getAvailableWorkers()
	if len(availableWorkers) == 0 {
		return
	}

	// Schedule tasks
	for _, worker := range availableWorkers {
		task := s.getNextTask()
		if task == nil {
			break
		}

		// BUG: Not checking task dependencies
		if !s.dependenciesMet(task) {
			// Put back in queue
			s.taskQueue.push(task)
			continue
		}

		// Assign task to worker
		worker.assignTask(task)

		// Update task status
		task.Status = "running"
		task.StartedAt = time.Now()
		s.runningTasks[task.ID] = task
		delete(s.pendingTasks, task.ID)

		atomic.AddInt64(&s.metrics.tasksRunning, 1)
	}
}

func (s *TaskScheduler) getAvailableWorkers() []*Worker {
	available := make([]*Worker, 0)
	for _, worker := range s.workers {
		if worker != nil && worker.isIdle {
			available = append(available, worker)
		}
	}
	return available
}

func (s *TaskScheduler) getNextTask() *Task {
	// BUG: Not handling priority correctly
	return s.taskQueue.pop()
}

func (s *TaskScheduler) dependenciesMet(task *Task) bool {
	deps, exists := s.dependencies[task.ID]
	if !exists || len(deps) == 0 {
		return true
	}

	// BUG: Not checking completion status correctly
	for _, dep := range deps {
		if _, completed := s.completedTasks[dep]; !completed {
			return false
		}
	}
	return true
}

func (s *TaskScheduler) balanceWork() {
	// Work stealing algorithm
	s.mu.RLock()
	workers := make([]*Worker, len(s.workers))
	copy(workers, s.workers)
	s.mu.RUnlock()

	for _, thief := range workers {
		if thief == nil || !thief.isIdle {
			continue
		}

		// Find victim with most tasks
		var victim *Worker
		maxTasks := 0
		for _, w := range workers {
			if w == nil || w == thief {
				continue
			}
			w.mu.Lock()
			taskCount := len(w.localQueue)
			w.mu.Unlock()
			if taskCount > maxTasks {
				maxTasks = taskCount
				victim = w
			}
		}

		if victim != nil && maxTasks > 1 {
			// BUG: Work stealing can cause task duplication
			victim.mu.Lock()
			if len(victim.localQueue) > 0 {
				// Steal half of the tasks
				stealCount := len(victim.localQueue) / 2
				stolenTasks := victim.localQueue[:stealCount]
				victim.localQueue = victim.localQueue[stealCount:]
				victim.mu.Unlock()

				thief.mu.Lock()
				thief.localQueue = append(thief.localQueue, stolenTasks...)
				thief.mu.Unlock()

				atomic.AddInt64(&s.metrics.stealSuccess, 1)
			} else {
				victim.mu.Unlock()
			}
		}
		atomic.AddInt64(&s.metrics.stealAttempts, 1)
	}
}

func (w *Worker) run() {
	for {
		select {
		case <-w.stopCh:
			// BUG: Not handling cleanup of running task
			return
		default:
			w.processTask()
		}
	}
}

func (w *Worker) processTask() {
	w.mu.Lock()
	if len(w.localQueue) == 0 {
		w.isIdle = true
		w.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
		return
	}

	// Get task from local queue
	task := w.localQueue[0]
	w.localQueue = w.localQueue[1:]
	w.currentTask = task
	w.isIdle = false
	w.mu.Unlock()

	// Execute task
	ctx, cancel := context.WithTimeout(w.scheduler.ctx, task.Timeout)
	defer cancel()

	// Simulate task execution
	err := w.executeTask(ctx, task)

	// Update task status
	w.scheduler.mu.Lock()
	if err != nil {
		task.Status = "failed"
		task.Error = err
		task.RetryCount++

		// BUG: Not implementing retry logic properly
		if task.RetryCount < task.MaxRetries {
			// Should reschedule
			w.scheduler.pendingTasks[task.ID] = task
		} else {
			atomic.AddInt64(&w.scheduler.metrics.tasksFailed, 1)
		}
	} else {
		task.Status = "completed"
		task.CompletedAt = time.Now()
		w.scheduler.completedTasks[task.ID] = task
		delete(w.scheduler.runningTasks, task.ID)
		atomic.AddInt64(&w.scheduler.metrics.tasksCompleted, 1)

		// BUG: Not triggering dependent tasks
	}
	w.scheduler.mu.Unlock()

	w.mu.Lock()
	w.currentTask = nil
	w.mu.Unlock()
}

func (w *Worker) assignTask(task *Task) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.localQueue = append(w.localQueue, task)
	w.isIdle = false
}

func (w *Worker) executeTask(ctx context.Context, task *Task) error {
	// Simulate task execution with random success/failure
	select {
	case <-ctx.Done():
		return errors.New("task timeout")
	case <-time.After(time.Duration(rand.Intn(200)) * time.Millisecond):
		if rand.Float32() < 0.2 {
			return errors.New("task failed")
		}
		return nil
	}
}

func (pq *PriorityQueue) push(task *Task) {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	// BUG: Not maintaining heap property
	pq.tasks = append(pq.tasks, task)
}

func (pq *PriorityQueue) pop() *Task {
	pq.mu.Lock()
	defer pq.mu.Unlock()

	if len(pq.tasks) == 0 {
		return nil
	}

	// BUG: Not selecting highest priority task
	task := pq.tasks[0]
	pq.tasks = pq.tasks[1:]
	return task
}

func (pq *PriorityQueue) Len() int           { return len(pq.tasks) }
func (pq *PriorityQueue) Less(i, j int) bool { return pq.tasks[i].Priority > pq.tasks[j].Priority }
func (pq *PriorityQueue) Swap(i, j int)      { pq.tasks[i], pq.tasks[j] = pq.tasks[j], pq.tasks[i] }
func (pq *PriorityQueue) Push(x interface{}) { pq.tasks = append(pq.tasks, x.(*Task)) }
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.tasks
	n := len(old)
	task := old[n-1]
	pq.tasks = old[0 : n-1]
	return task
}

func createTaskGraph() []*Task {
	// Create tasks with dependencies
	tasks := []*Task{
		{ID: "task-1", Priority: 5, Dependencies: []string{}, Status: "pending"},
		{ID: "task-2", Priority: 3, Dependencies: []string{"task-1"}, Status: "pending"},
		{ID: "task-3", Priority: 4, Dependencies: []string{"task-1"}, Status: "pending"},
		{ID: "task-4", Priority: 2, Dependencies: []string{"task-2", "task-3"}, Status: "pending"},
		{ID: "task-5", Priority: 6, Dependencies: []string{"task-4"}, Status: "pending"},
	}
	return tasks
}

// Fix priority queue to use heap properly
func (pq *PriorityQueue) Init() {
	heap.Init(pq)
}

// Missing implementations:
// 1. Proper priority queue with heap
// 2. Task dependency resolution
// 3. Worker failure recovery
// 4. Dynamic worker scaling
// 5. Task retry with exponential backoff
// 6. Resource limits and quotas
// 7. Task cancellation and cleanup
// 8. Distributed coordination for multi-node scheduling