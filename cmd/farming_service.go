package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// FarmingService manages plot file indexing and challenge responses
type FarmingService struct {
	config *ShadowConfig
	db     *badger.DB
	
	// Service state
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	isRunning  bool
	mu         sync.RWMutex
	
	// Statistics
	stats      FarmingStats
	statsMutex sync.RWMutex
	
	// Challenge handling
	challengeChan chan *StorageChallenge
	responseChan  chan *StorageProof
}

// FarmingStats contains farming service statistics
type FarmingStats struct {
	StartTime         time.Time `json:"start_time"`
	PlotFilesIndexed  int       `json:"plot_files_indexed"`
	TotalKeys         int       `json:"total_keys"`
	ChallengesHandled int64     `json:"challenges_handled"`
	LastChallengeTime time.Time `json:"last_challenge_time"`
	AverageResponseTime time.Duration `json:"average_response_time"`
	ErrorCount        int64     `json:"error_count"`
	DatabaseSize      int64     `json:"database_size"`
}

// StorageChallenge represents a proof-of-storage challenge
type StorageChallenge struct {
	ID          string    `json:"id"`
	Challenge   []byte    `json:"challenge"`
	Timestamp   time.Time `json:"timestamp"`
	Difficulty  uint32    `json:"difficulty"`
	ResponseChan chan *StorageProof `json:"-"`
}

// StorageProof represents a proof-of-storage response
type StorageProof struct {
	ChallengeID string          `json:"challenge_id"`
	PlotFile    string          `json:"plot_file"`
	Offset      int64           `json:"offset"`
	PrivateKey  string          `json:"private_key"`
	Signature   string          `json:"signature"`
	Valid       bool            `json:"valid"`
	ResponseTime time.Duration  `json:"response_time"`
	Error       string          `json:"error,omitempty"`
}

// NewFarmingService creates a new farming service
func NewFarmingService(config *ShadowConfig) *FarmingService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &FarmingService{
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		challengeChan: make(chan *StorageChallenge, 100),
		responseChan:  make(chan *StorageProof, 100),
		stats: FarmingStats{
			StartTime: time.Now().UTC(),
		},
	}
}

// Start initializes and starts the farming service
func (fs *FarmingService) Start() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if fs.isRunning {
		return fmt.Errorf("farming service is already running")
	}
	
	log.Printf("Starting farming service...")
	
	// Initialize database
	if err := fs.initializeDatabase(); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	// Index plot files
	if err := fs.indexPlotFiles(); err != nil {
		return fmt.Errorf("failed to index plot files: %w", err)
	}
	
	// Start challenge handler
	fs.wg.Add(1)
	go fs.challengeHandler()
	
	// Start stats updater
	fs.wg.Add(1)
	go fs.statsUpdater()
	
	fs.isRunning = true
	log.Printf("Farming service started successfully")
	
	return nil
}

// Stop gracefully stops the farming service
func (fs *FarmingService) Stop() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	
	if !fs.isRunning {
		return nil
	}
	
	log.Printf("Stopping farming service...")
	
	// Cancel context and wait for goroutines
	fs.cancel()
	fs.wg.Wait()
	
	// Close database
	if fs.db != nil {
		fs.db.Close()
	}
	
	fs.isRunning = false
	log.Printf("Farming service stopped")
	
	return nil
}

// IsRunning returns whether the farming service is currently running
func (fs *FarmingService) IsRunning() bool {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.isRunning
}

// GetStats returns current farming statistics
func (fs *FarmingService) GetStats() FarmingStats {
	fs.statsMutex.RLock()
	defer fs.statsMutex.RUnlock()
	return fs.stats
}

// SubmitChallenge submits a storage challenge for proof generation
func (fs *FarmingService) SubmitChallenge(challenge *StorageChallenge) *StorageProof {
	if !fs.IsRunning() {
		return &StorageProof{
			ChallengeID: challenge.ID,
			Valid:       false,
			Error:       "farming service not running",
		}
	}
	
	// Create response channel for this challenge
	challenge.ResponseChan = make(chan *StorageProof, 1)
	
	// Submit challenge
	select {
	case fs.challengeChan <- challenge:
		// Wait for response with timeout
		select {
		case proof := <-challenge.ResponseChan:
			return proof
		case <-time.After(30 * time.Second):
			return &StorageProof{
				ChallengeID: challenge.ID,
				Valid:       false,
				Error:       "challenge timeout",
			}
		}
	default:
		return &StorageProof{
			ChallengeID: challenge.ID,
			Valid:       false,
			Error:       "challenge queue full",
		}
	}
}

// ListPlotFiles returns information about indexed plot files
func (fs *FarmingService) ListPlotFiles() ([]PlotFileInfo, error) {
	if fs.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	
	var plotFiles []PlotFileInfo
	plotFileMap := make(map[string]*PlotFileInfo)
	
	err := fs.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = true
		it := txn.NewIterator(opts)
		defer it.Close()
		
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			
			err := item.Value(func(val []byte) error {
				entry, err := decodePlotEntry(val)
				if err != nil {
					return err
				}
				
				if info, exists := plotFileMap[entry.FilePath]; exists {
					info.KeyCount++
				} else {
					plotFileMap[entry.FilePath] = &PlotFileInfo{
						FilePath: entry.FilePath,
						FileName: filepath.Base(entry.FilePath),
						KeyCount: 1,
					}
				}
				return nil
			})
			
			if err != nil {
				return err
			}
		}
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Convert map to slice
	for _, info := range plotFileMap {
		// Get file stats
		if stat, err := os.Stat(info.FilePath); err == nil {
			info.FileSize = stat.Size()
			info.ModTime = stat.ModTime()
		}
		plotFiles = append(plotFiles, *info)
	}
	
	return plotFiles, nil
}

// PlotFileInfo contains information about an indexed plot file
type PlotFileInfo struct {
	FilePath string    `json:"file_path"`
	FileName string    `json:"file_name"`
	KeyCount int       `json:"key_count"`
	FileSize int64     `json:"file_size"`
	ModTime  time.Time `json:"mod_time"`
}

// initializeDatabase sets up the BadgerDB database
func (fs *FarmingService) initializeDatabase() error {
	dbPath := filepath.Join(fs.config.ScratchDirectory, "plot-lookup")
	
	// Clean up old database
	if err := cleanupOldDatabase(dbPath); err != nil {
		return fmt.Errorf("failed to cleanup old database: %w", err)
	}
	
	// Ensure scratch directory exists
	if err := os.MkdirAll(fs.config.ScratchDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create scratch directory: %w", err)
	}
	
	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable BadgerDB logging
	
	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	
	fs.db = db
	log.Printf("Database opened at: %s", dbPath)
	
	return nil
}

// indexPlotFiles indexes all plot files in configured directories
func (fs *FarmingService) indexPlotFiles() error {
	plotCount := 0
	keyCount := 0
	
	for _, plotDir := range fs.config.PlotDirectories {
		log.Printf("Scanning plot directory: %s", plotDir)
		
		// Check if directory exists
		if _, err := os.Stat(plotDir); os.IsNotExist(err) {
			log.Printf("Warning: plot directory '%s' does not exist, skipping", plotDir)
			continue
		}
		
		// Find all plot files
		plotFiles, err := findPlotFiles(plotDir)
		if err != nil {
			return fmt.Errorf("failed to find plot files in '%s': %w", plotDir, err)
		}
		
		log.Printf("Found %d plot files in %s", len(plotFiles), plotDir)
		
		// Index each plot file
		for _, plotFile := range plotFiles {
			keys, err := indexPlotFile(fs.db, plotFile)
			if err != nil {
				log.Printf("Warning: failed to index plot file '%s': %v", plotFile, err)
				continue
			}
			
			plotCount++
			keyCount += keys
			log.Printf("Indexed %s (%d keys)", filepath.Base(plotFile), keys)
		}
	}
	
	// Update stats
	fs.statsMutex.Lock()
	fs.stats.PlotFilesIndexed = plotCount
	fs.stats.TotalKeys = keyCount
	fs.statsMutex.Unlock()
	
	log.Printf("Indexed %d plot files with %d total keys", plotCount, keyCount)
	return nil
}

// challengeHandler processes incoming storage challenges
func (fs *FarmingService) challengeHandler() {
	defer fs.wg.Done()
	
	log.Printf("Challenge handler started")
	
	for {
		select {
		case <-fs.ctx.Done():
			log.Printf("Challenge handler stopping")
			return
			
		case challenge := <-fs.challengeChan:
			startTime := time.Now()
			proof := fs.processChallenge(challenge)
			proof.ResponseTime = time.Since(startTime)
			
			// Update stats
			fs.statsMutex.Lock()
			fs.stats.ChallengesHandled++
			fs.stats.LastChallengeTime = time.Now().UTC()
			if fs.stats.ChallengesHandled == 1 {
				fs.stats.AverageResponseTime = proof.ResponseTime
			} else {
				// Exponential moving average
				alpha := 0.1
				fs.stats.AverageResponseTime = time.Duration(
					float64(fs.stats.AverageResponseTime)*(1-alpha) + 
					float64(proof.ResponseTime)*alpha,
				)
			}
			if proof.Error != "" {
				fs.stats.ErrorCount++
			}
			fs.statsMutex.Unlock()
			
			// Send response
			select {
			case challenge.ResponseChan <- proof:
			default:
				log.Printf("Warning: failed to send challenge response")
			}
		}
	}
}

// processChallenge processes a single storage challenge
func (fs *FarmingService) processChallenge(challenge *StorageChallenge) *StorageProof {
	// TODO: Implement actual challenge processing
	// This would involve:
	// 1. Finding the best matching plot entry for the challenge
	// 2. Reading the private key from the plot file at the correct offset
	// 3. Generating a signature proof
	// 4. Returning the proof
	
	// For now, return a placeholder response
	return &StorageProof{
		ChallengeID:  challenge.ID,
		PlotFile:     "placeholder.dat",
		Offset:       0,
		PrivateKey:   "placeholder_key",
		Signature:    "placeholder_signature",
		Valid:        true,
		Error:        "",
	}
}

// statsUpdater periodically updates internal statistics
func (fs *FarmingService) statsUpdater() {
	defer fs.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-fs.ctx.Done():
			return
		case <-ticker.C:
			fs.updateDatabaseStats()
		}
	}
}

// updateDatabaseStats updates database-related statistics
func (fs *FarmingService) updateDatabaseStats() {
	if fs.db == nil {
		return
	}
	
	// Get database size (this is an approximation)
	lsm, vlog := fs.db.Size()
	
	fs.statsMutex.Lock()
	fs.stats.DatabaseSize = lsm + vlog
	fs.statsMutex.Unlock()
}

// Convenience function to create a test challenge
func CreateTestChallenge(data []byte) *StorageChallenge {
	return &StorageChallenge{
		ID:        fmt.Sprintf("test_%d", time.Now().UnixNano()),
		Challenge: data,
		Timestamp: time.Now().UTC(),
		Difficulty: 1,
	}
}