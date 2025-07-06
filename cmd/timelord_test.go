package cmd

import (
	"testing"
	"time"
)

func TestTimelordConfigDefaults(t *testing.T) {
	config := DefaultTimelordConfig()
	
	if config == nil {
		t.Fatal("DefaultTimelordConfig returned nil")
	}
	
	if config.VDFConfig == nil {
		t.Error("VDFConfig should not be nil")
	}
	
	if config.WorkerPoolSize <= 0 {
		t.Error("WorkerPoolSize should be positive")
	}
	
	if config.MaxPendingChallenges <= 0 {
		t.Error("MaxPendingChallenges should be positive")
	}
	
	if config.ChallengeTimeout <= 0 {
		t.Error("ChallengeTimeout should be positive")
	}
	
	if config.MonitoringInterval <= 0 {
		t.Error("MonitoringInterval should be positive")
	}
}

func TestNewTimelord(t *testing.T) {
	config := DefaultTimelordConfig()
	config.WorkerPoolSize = 2
	config.VDFConfig.TimeParameter = 100 // Small for testing
	
	timelord := NewTimelord(config)
	
	if timelord == nil {
		t.Fatal("NewTimelord returned nil")
	}
	
	if timelord.config != config {
		t.Error("Timelord should use provided config")
	}
	
	if len(timelord.workers) != config.WorkerPoolSize {
		t.Errorf("Expected %d workers, got %d", config.WorkerPoolSize, len(timelord.workers))
	}
	
	if timelord.solver == nil {
		t.Error("Timelord should have a solver")
	}
	
	if timelord.verifier == nil {
		t.Error("Timelord should have a verifier")
	}
	
	if timelord.jobs == nil {
		t.Error("Jobs map should be initialized")
	}
	
	if timelord.jobQueue == nil {
		t.Error("Job queue should be initialized")
	}
}

func TestTimelordJobStatus(t *testing.T) {
	statuses := []JobStatus{
		JobStatusPending,
		JobStatusRunning,
		JobStatusCompleted,
		JobStatusFailed,
		JobStatusTimeout,
	}
	
	expectedStrings := []string{
		"pending",
		"running",
		"completed",
		"failed",
		"timeout",
	}
	
	for i, status := range statuses {
		if status.String() != expectedStrings[i] {
			t.Errorf("Expected status string %s, got %s", expectedStrings[i], status.String())
		}
	}
	
	// Test unknown status
	unknownStatus := JobStatus(999)
	if unknownStatus.String() != "unknown" {
		t.Error("Unknown status should return 'unknown'")
	}
}

func TestTimelordSubmitChallenge(t *testing.T) {
	config := DefaultTimelordConfig()
	config.MaxPendingChallenges = 2
	config.VDFConfig.TimeParameter = 50 // Small for testing
	
	timelord := NewTimelord(config)
	
	inputData := []byte("test challenge")
	job, err := timelord.SubmitChallenge(inputData, 1)
	
	if err != nil {
		t.Fatalf("Failed to submit challenge: %v", err)
	}
	
	if job == nil {
		t.Fatal("SubmitChallenge returned nil job")
	}
	
	if job.ID == "" {
		t.Error("Job should have an ID")
	}
	
	if job.Challenge == nil {
		t.Error("Job should have a challenge")
	}
	
	if job.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", job.Priority)
	}
	
	if job.Status != JobStatusPending {
		t.Errorf("Initial job status should be pending, got %s", job.Status)
	}
	
	if job.SubmittedAt.IsZero() {
		t.Error("Job submission time should be set")
	}
	
	// Test retrieving the job
	retrievedJob, err := timelord.GetJob(job.ID)
	if err != nil {
		t.Fatalf("Failed to retrieve job: %v", err)
	}
	
	if retrievedJob.ID != job.ID {
		t.Error("Retrieved job should match submitted job")
	}
}

func TestTimelordMaxPendingChallenges(t *testing.T) {
	config := DefaultTimelordConfig()
	config.MaxPendingChallenges = 2
	config.VDFConfig.TimeParameter = 50
	
	timelord := NewTimelord(config)
	
	// Submit maximum number of challenges
	for i := 0; i < config.MaxPendingChallenges; i++ {
		inputData := []byte("test " + string(rune(i)))
		_, err := timelord.SubmitChallenge(inputData, 1)
		if err != nil {
			t.Fatalf("Failed to submit challenge %d: %v", i, err)
		}
	}
	
	// Try to submit one more - should fail
	_, err := timelord.SubmitChallenge([]byte("overflow"), 1)
	if err == nil {
		t.Error("Should fail when exceeding max pending challenges")
	}
}

func TestTimelordGetJobNotFound(t *testing.T) {
	config := DefaultTimelordConfig()
	timelord := NewTimelord(config)
	
	_, err := timelord.GetJob("nonexistent_id")
	if err == nil {
		t.Error("Should return error for nonexistent job")
	}
}

func TestTimelordStats(t *testing.T) {
	config := DefaultTimelordConfig()
	config.VDFConfig.TimeParameter = 50
	
	timelord := NewTimelord(config)
	
	// Initial stats
	stats := timelord.GetStats()
	if stats.TotalJobs != 0 {
		t.Error("Initial total jobs should be 0")
	}
	
	if stats.CompletedJobs != 0 {
		t.Error("Initial completed jobs should be 0")
	}
	
	if stats.PendingJobs != 0 {
		t.Error("Initial pending jobs should be 0")
	}
	
	// Submit a job
	inputData := []byte("stats test")
	_, err := timelord.SubmitChallenge(inputData, 1)
	if err != nil {
		t.Fatalf("Failed to submit challenge: %v", err)
	}
	
	// Check updated stats
	stats = timelord.GetStats()
	if stats.TotalJobs != 1 {
		t.Errorf("Expected 1 total job, got %d", stats.TotalJobs)
	}
	
	if stats.PendingJobs != 1 {
		t.Errorf("Expected 1 pending job, got %d", stats.PendingJobs)
	}
}

func TestTimelordWorkerProcessing(t *testing.T) {
	config := DefaultTimelordConfig()
	config.WorkerPoolSize = 1
	config.VDFConfig.TimeParameter = 10 // Very small for quick test
	config.VDFConfig.ModulusBits = 256  // Small modulus for speed
	config.VDFConfig.Modulus = generateRSAModulus(256)
	config.ChallengeTimeout = 30 * time.Second
	
	timelord := NewTimelord(config)
	
	// Start the timelord
	err := timelord.Start()
	if err != nil {
		t.Fatalf("Failed to start timelord: %v", err)
	}
	defer timelord.Stop()
	
	// Submit a challenge
	inputData := []byte("worker test")
	job, err := timelord.SubmitChallenge(inputData, 1)
	if err != nil {
		t.Fatalf("Failed to submit challenge: %v", err)
	}
	
	// Wait for job to complete
	timeout := time.After(15 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Job did not complete within timeout")
		case <-ticker.C:
			retrievedJob, err := timelord.GetJob(job.ID)
			if err != nil {
				t.Fatalf("Failed to retrieve job: %v", err)
			}
			
			if retrievedJob.Status == JobStatusCompleted {
				// Job completed successfully
				if retrievedJob.Result == nil {
					t.Error("Completed job should have a result")
				}
				
				if retrievedJob.Result.Error != "" {
					t.Errorf("Job completed with error: %s", retrievedJob.Result.Error)
				}
				
				if !retrievedJob.Result.IsValid {
					t.Error("Job result should be valid")
				}
				
				if retrievedJob.StartedAt == nil {
					t.Error("Job should have start time")
				}
				
				if retrievedJob.CompletedAt == nil {
					t.Error("Job should have completion time")
				}
				
				return // Test passed
				
			} else if retrievedJob.Status == JobStatusFailed || retrievedJob.Status == JobStatusTimeout {
				t.Fatalf("Job failed with status: %s", retrievedJob.Status)
			}
		}
	}
}

func TestTimelordStartStop(t *testing.T) {
	config := DefaultTimelordConfig()
	config.WorkerPoolSize = 2
	config.VDFConfig.TimeParameter = 100
	
	timelord := NewTimelord(config)
	
	// Start timelord
	err := timelord.Start()
	if err != nil {
		t.Fatalf("Failed to start timelord: %v", err)
	}
	
	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)
	
	// Stop timelord
	err = timelord.Stop()
	if err != nil {
		t.Fatalf("Failed to stop timelord: %v", err)
	}
}

func TestTimelordJobTimeout(t *testing.T) {
	config := DefaultTimelordConfig()
	config.WorkerPoolSize = 1
	config.VDFConfig.TimeParameter = 1000000 // Very large to force timeout
	config.ChallengeTimeout = 500 * time.Millisecond // Short timeout
	
	timelord := NewTimelord(config)
	
	err := timelord.Start()
	if err != nil {
		t.Fatalf("Failed to start timelord: %v", err)
	}
	defer timelord.Stop()
	
	// Submit a challenge that will timeout
	inputData := []byte("timeout test")
	job, err := timelord.SubmitChallenge(inputData, 1)
	if err != nil {
		t.Fatalf("Failed to submit challenge: %v", err)
	}
	
	// Wait for job to timeout
	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	
	for {
		select {
		case <-timeout:
			t.Fatal("Job did not timeout within expected time")
		case <-ticker.C:
			retrievedJob, err := timelord.GetJob(job.ID)
			if err != nil {
				t.Fatalf("Failed to retrieve job: %v", err)
			}
			
			if retrievedJob.Status == JobStatusTimeout {
				// Job timed out as expected
				if retrievedJob.Result == nil {
					t.Error("Timeout job should have a result")
				}
				
				if retrievedJob.Result.Error == "" {
					t.Error("Timeout job should have an error message")
				}
				
				return // Test passed
			}
		}
	}
}

func TestTimelordDifficultyAdjustment(t *testing.T) {
	config := DefaultTimelordConfig()
	config.AutoAdjustDifficulty = true
	config.TargetProofTime = 5 * time.Second
	config.VDFConfig.TimeParameter = 2000 // Higher initial difficulty
	
	timelord := NewTimelord(config)
	
	initialDifficulty := config.VDFConfig.TimeParameter
	
	// Test decreasing difficulty (proof too slow)
	slowProofTime := 15 * time.Second
	timelord.adjustDifficulty(slowProofTime)
	
	if timelord.config.VDFConfig.TimeParameter >= initialDifficulty {
		t.Errorf("Difficulty should decrease when proof is too slow: was %d, now %d", 
			initialDifficulty, timelord.config.VDFConfig.TimeParameter)
	}
	
	// Reset difficulty
	timelord.config.VDFConfig.TimeParameter = initialDifficulty
	
	// Test increasing difficulty (proof too fast)
	fastProofTime := 1 * time.Second
	timelord.adjustDifficulty(fastProofTime)
	
	if timelord.config.VDFConfig.TimeParameter <= initialDifficulty {
		t.Error("Difficulty should increase when proof is too fast")
	}
	
	// Test no change (proof time in acceptable range)
	timelord.config.VDFConfig.TimeParameter = initialDifficulty
	acceptableProofTime := 5 * time.Second
	timelord.adjustDifficulty(acceptableProofTime)
	
	if timelord.config.VDFConfig.TimeParameter != initialDifficulty {
		t.Error("Difficulty should not change when proof time is acceptable")
	}
}

func TestTimelordUpdateStats(t *testing.T) {
	config := DefaultTimelordConfig()
	timelord := NewTimelord(config)
	
	// Test concurrent stats updates
	done := make(chan bool, 10)
	
	for i := 0; i < 10; i++ {
		go func(val int64) {
			timelord.updateStats(func(stats *TimelordStats) {
				stats.TotalJobs += val
				stats.CompletedJobs += val
			})
			done <- true
		}(int64(i + 1))
	}
	
	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
	
	stats := timelord.GetStats()
	
	// Sum of 1+2+...+10 = 55
	expectedTotal := int64(55)
	if stats.TotalJobs != expectedTotal {
		t.Errorf("Expected total jobs %d, got %d", expectedTotal, stats.TotalJobs)
	}
	
	if stats.CompletedJobs != expectedTotal {
		t.Errorf("Expected completed jobs %d, got %d", expectedTotal, stats.CompletedJobs)
	}
}

func BenchmarkTimelordSubmitChallenge(b *testing.B) {
	config := DefaultTimelordConfig()
	config.MaxPendingChallenges = 10000 // Large enough for benchmark
	config.VDFConfig.TimeParameter = 100
	
	timelord := NewTimelord(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		inputData := []byte("benchmark " + string(rune(i)))
		_, err := timelord.SubmitChallenge(inputData, 1)
		if err != nil {
			b.Fatalf("Submit challenge failed: %v", err)
		}
	}
}

func BenchmarkTimelordGetJob(b *testing.B) {
	config := DefaultTimelordConfig()
	timelord := NewTimelord(config)
	
	// Submit a job first
	inputData := []byte("benchmark get job")
	job, err := timelord.SubmitChallenge(inputData, 1)
	if err != nil {
		b.Fatalf("Failed to submit job: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := timelord.GetJob(job.ID)
		if err != nil {
			b.Fatalf("Get job failed: %v", err)
		}
	}
}