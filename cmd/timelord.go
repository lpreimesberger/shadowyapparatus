package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// TimelordConfig contains configuration for the timelord service
type TimelordConfig struct {
	// VDF configuration
	VDFConfig *VDFConfig `json:"vdf_config"`
	
	// Worker pool size for parallel VDF computation
	WorkerPoolSize int `json:"worker_pool_size"`
	
	// Maximum number of pending challenges
	MaxPendingChallenges int `json:"max_pending_challenges"`
	
	// Challenge timeout duration
	ChallengeTimeout time.Duration `json:"challenge_timeout"`
	
	// Performance monitoring interval
	MonitoringInterval time.Duration `json:"monitoring_interval"`
	
	// Enable automatic difficulty adjustment
	AutoAdjustDifficulty bool `json:"auto_adjust_difficulty"`
	
	// Target proof generation time
	TargetProofTime time.Duration `json:"target_proof_time"`
}

// DefaultTimelordConfig returns default timelord configuration
func DefaultTimelordConfig() *TimelordConfig {
	return &TimelordConfig{
		VDFConfig:            DefaultVDFConfig(),
		WorkerPoolSize:       2, // Conservative default
		MaxPendingChallenges: 100,
		ChallengeTimeout:     5 * time.Minute,
		MonitoringInterval:   30 * time.Second,
		AutoAdjustDifficulty: true,
		TargetProofTime:      30 * time.Second,
	}
}

// TimelordJob represents a VDF computation job
type TimelordJob struct {
	ID          string        `json:"id"`
	Challenge   *VDFChallenge `json:"challenge"`
	Priority    int           `json:"priority"`
	SubmittedAt time.Time     `json:"submitted_at"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Result      *VDFResult    `json:"result,omitempty"`
	Status      JobStatus     `json:"status"`
}

// JobStatus represents the status of a timelord job
type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusRunning
	JobStatusCompleted
	JobStatusFailed
	JobStatusTimeout
)

func (js JobStatus) String() string {
	switch js {
	case JobStatusPending:
		return "pending"
	case JobStatusRunning:
		return "running"
	case JobStatusCompleted:
		return "completed"
	case JobStatusFailed:
		return "failed"
	case JobStatusTimeout:
		return "timeout"
	default:
		return "unknown"
	}
}

// TimelordStats contains performance statistics
type TimelordStats struct {
	TotalJobs         int64         `json:"total_jobs"`
	CompletedJobs     int64         `json:"completed_jobs"`
	FailedJobs        int64         `json:"failed_jobs"`
	TimeoutJobs       int64         `json:"timeout_jobs"`
	AverageProofTime  time.Duration `json:"average_proof_time"`
	CurrentDifficulty uint64        `json:"current_difficulty"`
	ActiveWorkers     int           `json:"active_workers"`
	PendingJobs       int           `json:"pending_jobs"`
	LastProofTime     time.Time     `json:"last_proof_time"`
}

// Timelord manages VDF computation and verification
type Timelord struct {
	config *TimelordConfig
	solver *VDFSolver
	verifier *VDFVerifier
	
	// Job management
	jobs        map[string]*TimelordJob
	jobQueue    chan *TimelordJob
	jobsMutex   sync.RWMutex
	
	// Worker management
	workers     []*TimelordWorker
	workerGroup sync.WaitGroup
	
	// Statistics
	stats      TimelordStats
	statsMutex sync.RWMutex
	
	// Control
	ctx    context.Context
	cancel context.CancelFunc
	
	// Monitoring
	monitorTicker *time.Ticker
}

// TimelordWorker handles individual VDF computations
type TimelordWorker struct {
	id       int
	timelord *Timelord
	solver   *VDFSolver
	verifier *VDFVerifier
}

// NewTimelord creates a new timelord service
func NewTimelord(config *TimelordConfig) *Timelord {
	if config == nil {
		config = DefaultTimelordConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	tl := &Timelord{
		config:    config,
		solver:    NewVDFSolver(config.VDFConfig),
		verifier:  NewVDFVerifier(config.VDFConfig),
		jobs:      make(map[string]*TimelordJob),
		jobQueue:  make(chan *TimelordJob, config.MaxPendingChallenges),
		workers:   make([]*TimelordWorker, config.WorkerPoolSize),
		ctx:       ctx,
		cancel:    cancel,
		stats: TimelordStats{
			CurrentDifficulty: config.VDFConfig.TimeParameter,
		},
	}
	
	// Create workers
	for i := 0; i < config.WorkerPoolSize; i++ {
		tl.workers[i] = &TimelordWorker{
			id:       i,
			timelord: tl,
			solver:   NewVDFSolver(config.VDFConfig),
			verifier: NewVDFVerifier(config.VDFConfig),
		}
	}
	
	return tl
}

// Start begins the timelord service
func (tl *Timelord) Start() error {
	log.Printf("Starting timelord service with %d workers", tl.config.WorkerPoolSize)
	
	// Start workers
	for _, worker := range tl.workers {
		tl.workerGroup.Add(1)
		go worker.run(tl.ctx, &tl.workerGroup)
	}
	
	// Start monitoring
	tl.monitorTicker = time.NewTicker(tl.config.MonitoringInterval)
	go tl.monitorPerformance()
	
	// Start cleanup routine
	go tl.cleanupExpiredJobs()
	
	log.Printf("Timelord service started successfully")
	return nil
}

// Stop gracefully shuts down the timelord service
func (tl *Timelord) Stop() error {
	log.Printf("Stopping timelord service...")
	
	// Cancel context to signal workers to stop
	tl.cancel()
	
	// Stop monitoring
	if tl.monitorTicker != nil {
		tl.monitorTicker.Stop()
	}
	
	// Wait for workers to complete
	tl.workerGroup.Wait()
	
	log.Printf("Timelord service stopped")
	return nil
}

// SubmitChallenge submits a new VDF challenge for computation
func (tl *Timelord) SubmitChallenge(inputData []byte, priority int) (*TimelordJob, error) {
	// Generate challenge
	challenge := tl.solver.GenerateChallenge(inputData)
	
	// Create job
	job := &TimelordJob{
		ID:          challenge.ID,
		Challenge:   challenge,
		Priority:    priority,
		SubmittedAt: time.Now().UTC(),
		Status:      JobStatusPending,
	}
	
	// Store job
	tl.jobsMutex.Lock()
	if len(tl.jobs) >= tl.config.MaxPendingChallenges {
		tl.jobsMutex.Unlock()
		return nil, fmt.Errorf("maximum pending challenges reached (%d)", tl.config.MaxPendingChallenges)
	}
	tl.jobs[job.ID] = job
	tl.jobsMutex.Unlock()
	
	// Queue job for processing
	select {
	case tl.jobQueue <- job:
		tl.updateStats(func(stats *TimelordStats) {
			stats.TotalJobs++
			stats.PendingJobs++
		})
		return job, nil
	default:
		// Queue is full, remove job
		tl.jobsMutex.Lock()
		delete(tl.jobs, job.ID)
		tl.jobsMutex.Unlock()
		return nil, fmt.Errorf("job queue is full")
	}
}

// GetJob retrieves a job by ID
func (tl *Timelord) GetJob(jobID string) (*TimelordJob, error) {
	tl.jobsMutex.RLock()
	defer tl.jobsMutex.RUnlock()
	
	job, exists := tl.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	
	return job, nil
}

// GetStats returns current timelord statistics
func (tl *Timelord) GetStats() TimelordStats {
	tl.statsMutex.RLock()
	defer tl.statsMutex.RUnlock()
	
	// Update pending jobs count
	tl.jobsMutex.RLock()
	pendingCount := 0
	for _, job := range tl.jobs {
		if job.Status == JobStatusPending || job.Status == JobStatusRunning {
			pendingCount++
		}
	}
	tl.jobsMutex.RUnlock()
	
	stats := tl.stats
	stats.PendingJobs = pendingCount
	return stats
}

// Worker implementation
func (tw *TimelordWorker) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	
	log.Printf("Timelord worker %d started", tw.id)
	defer log.Printf("Timelord worker %d stopped", tw.id)
	
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-tw.timelord.jobQueue:
			tw.processJob(job)
		}
	}
}

func (tw *TimelordWorker) processJob(job *TimelordJob) {
	log.Printf("Worker %d processing job %s", tw.id, job.ID)
	
	// Update job status
	now := time.Now().UTC()
	job.StartedAt = &now
	job.Status = JobStatusRunning
	
	tw.timelord.updateStats(func(stats *TimelordStats) {
		stats.ActiveWorkers++
		stats.PendingJobs--
	})
	
	// Create timeout context
	jobCtx, cancel := context.WithTimeout(context.Background(), tw.timelord.config.ChallengeTimeout)
	defer cancel()
	
	// Solve VDF with timeout
	resultChan := make(chan *VDFResult, 1)
	go func() {
		proof, err := tw.solver.Solve(job.Challenge)
		if err != nil {
			resultChan <- &VDFResult{
				Error: fmt.Sprintf("solve error: %v", err),
			}
			return
		}
		
		// Verify the proof
		verifyStart := time.Now()
		isValid, err := tw.verifier.Verify(proof)
		verifyTime := time.Since(verifyStart)
		
		if err != nil {
			resultChan <- &VDFResult{
				Proof: proof,
				Error: fmt.Sprintf("verify error: %v", err),
			}
			return
		}
		
		resultChan <- &VDFResult{
			Proof:      proof,
			IsValid:    isValid,
			VerifyTime: verifyTime,
		}
	}()
	
	// Wait for result or timeout
	select {
	case <-jobCtx.Done():
		job.Status = JobStatusTimeout
		job.Result = &VDFResult{
			Error: "job timed out",
		}
		tw.timelord.updateStats(func(stats *TimelordStats) {
			stats.TimeoutJobs++
			stats.ActiveWorkers--
		})
		
	case result := <-resultChan:
		completedAt := time.Now().UTC()
		job.CompletedAt = &completedAt
		job.Result = result
		
		if result.Error != "" {
			job.Status = JobStatusFailed
			tw.timelord.updateStats(func(stats *TimelordStats) {
				stats.FailedJobs++
				stats.ActiveWorkers--
			})
		} else {
			job.Status = JobStatusCompleted
			proofTime := result.Proof.ComputeTime
			
			tw.timelord.updateStats(func(stats *TimelordStats) {
				stats.CompletedJobs++
				stats.ActiveWorkers--
				stats.LastProofTime = completedAt
				
				// Update average proof time
				if stats.CompletedJobs == 1 {
					stats.AverageProofTime = proofTime
				} else {
					// Exponential moving average
					alpha := 0.1
					stats.AverageProofTime = time.Duration(
						float64(stats.AverageProofTime)*(1-alpha) + float64(proofTime)*alpha,
					)
				}
			})
			
			// Adjust difficulty if enabled
			if tw.timelord.config.AutoAdjustDifficulty {
				tw.timelord.adjustDifficulty(proofTime)
			}
		}
	}
	
	log.Printf("Worker %d completed job %s (status: %s)", tw.id, job.ID, job.Status)
}

// updateStats safely updates timelord statistics
func (tl *Timelord) updateStats(updateFunc func(*TimelordStats)) {
	tl.statsMutex.Lock()
	defer tl.statsMutex.Unlock()
	updateFunc(&tl.stats)
}

// adjustDifficulty adjusts VDF difficulty based on proof time
func (tl *Timelord) adjustDifficulty(proofTime time.Duration) {
	targetTime := tl.config.TargetProofTime
	currentDifficulty := tl.config.VDFConfig.TimeParameter
	
	// Simple difficulty adjustment algorithm
	if proofTime > targetTime*2 {
		// Too slow, decrease difficulty by 10%
		newDifficulty := uint64(float64(currentDifficulty) * 0.9)
		if newDifficulty < 1000 {
			newDifficulty = 1000 // Minimum difficulty
		}
		tl.config.VDFConfig.TimeParameter = newDifficulty
		log.Printf("Decreased VDF difficulty to %d (proof time: %v)", newDifficulty, proofTime)
		
	} else if proofTime < targetTime/2 {
		// Too fast, increase difficulty by 10%
		newDifficulty := uint64(float64(currentDifficulty) * 1.1)
		if newDifficulty > 10000000 {
			newDifficulty = 10000000 // Maximum difficulty
		}
		tl.config.VDFConfig.TimeParameter = newDifficulty
		log.Printf("Increased VDF difficulty to %d (proof time: %v)", newDifficulty, proofTime)
	}
	
	tl.updateStats(func(stats *TimelordStats) {
		stats.CurrentDifficulty = tl.config.VDFConfig.TimeParameter
	})
}

// monitorPerformance logs performance statistics
func (tl *Timelord) monitorPerformance() {
	for {
		select {
		case <-tl.ctx.Done():
			return
		case <-tl.monitorTicker.C:
			stats := tl.GetStats()
			log.Printf("Timelord Stats - Total: %d, Completed: %d, Failed: %d, Timeout: %d, Pending: %d, Avg Time: %v, Difficulty: %d",
				stats.TotalJobs, stats.CompletedJobs, stats.FailedJobs, stats.TimeoutJobs,
				stats.PendingJobs, stats.AverageProofTime, stats.CurrentDifficulty)
		}
	}
}

// cleanupExpiredJobs removes old completed jobs to prevent memory leaks
func (tl *Timelord) cleanupExpiredJobs() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-tl.ctx.Done():
			return
		case <-ticker.C:
			cutoff := time.Now().UTC().Add(-24 * time.Hour)
			
			tl.jobsMutex.Lock()
			cleaned := 0
			for id, job := range tl.jobs {
				if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusTimeout {
					if job.CompletedAt != nil && job.CompletedAt.Before(cutoff) {
						delete(tl.jobs, id)
						cleaned++
					}
				}
			}
			tl.jobsMutex.Unlock()
			
			if cleaned > 0 {
				log.Printf("Cleaned up %d expired jobs", cleaned)
			}
		}
	}
}

// Timelord CLI command
var timelordCmd = &cobra.Command{
	Use:   "timelord",
	Short: "Start the VDF timelord service",
	Long:  "Starts the Verifiable Delay Function (VDF) timelord service for Shadowy blockchain",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			return
		}
		
		// Get timelord configuration
		timelordConfig := DefaultTimelordConfig()
		if config.TimelordConfig != nil {
			configData, _ := json.Marshal(config.TimelordConfig)
			json.Unmarshal(configData, timelordConfig)
		}
		
		// Create and start timelord
		timelord := NewTimelord(timelordConfig)
		
		if err := timelord.Start(); err != nil {
			fmt.Printf("Error starting timelord: %v\n", err)
			return
		}
		
		// Wait for interrupt signal (placeholder - would use signal handling in production)
		select {}
		
		// Graceful shutdown
		if err := timelord.Stop(); err != nil {
			fmt.Printf("Error stopping timelord: %v\n", err)
		}
	},
}

func init() {
	rootCmd.AddCommand(timelordCmd)
	
	timelordCmd.Flags().Int("workers", 2, "Number of worker threads")
	timelordCmd.Flags().Duration("timeout", 5*time.Minute, "Job timeout duration")
	timelordCmd.Flags().Bool("auto-adjust", true, "Enable automatic difficulty adjustment")
}