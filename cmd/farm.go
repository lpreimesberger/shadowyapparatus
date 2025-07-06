package cmd

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/spf13/cobra"
)

var (
	farmTimeout time.Duration
)

type PlotEntry struct {
	FilePath string
	Offset   int64
}

var farmCmd = &cobra.Command{
	Use:   "farm",
	Short: "Start farming service to respond to storage challenges",
	Long: `Start the farming service that:
1. Scans configured plot directories for umbra plot files
2. Builds a BadgerDB lookup database in scratch directory
3. Listens for storage challenges and responds with proofs
4. Runs until interrupted or timeout expires`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Starting Shadowy farming service...\n")
		
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		
		if len(config.PlotDirectories) == 0 {
			fmt.Printf("No plot directories configured. Add directories with:\n")
			fmt.Printf("  shadowy config addplotdir [directory]\n")
			os.Exit(1)
		}
		
		// Create context with timeout if specified
		ctx := context.Background()
		if farmTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, farmTimeout)
			defer cancel()
			fmt.Printf("Farm timeout set to: %v\n", farmTimeout)
		}
		
		if err := runFarm(ctx, config); err != nil {
			fmt.Printf("Farm error: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Farming service stopped.\n")
	},
}

func init() {
	rootCmd.AddCommand(farmCmd)
	farmCmd.Flags().DurationVar(&farmTimeout, "timeout", 0, 
		"Timeout for farming service (0 = run indefinitely)")
	
	// Add wallet-dir flag for config access
	farmCmd.Flags().StringVar(&walletDir, "wallet-dir", "", 
		"Directory for config files (default: $HOME/.shadowy)")
}

func runFarm(ctx context.Context, config *ShadowConfig) error {
	// Initialize BadgerDB in scratch directory
	dbPath := filepath.Join(config.ScratchDirectory, "plot-lookup")
	
	// Clean up old database
	if err := cleanupOldDatabase(dbPath); err != nil {
		return fmt.Errorf("failed to cleanup old database: %w", err)
	}
	
	// Ensure scratch directory exists
	if err := os.MkdirAll(config.ScratchDirectory, 0755); err != nil {
		return fmt.Errorf("failed to create scratch directory: %w", err)
	}
	
	// Open BadgerDB
	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable BadgerDB logging for cleaner output
	db, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to open BadgerDB: %w", err)
	}
	defer db.Close()
	
	fmt.Printf("Database opened at: %s\n", dbPath)
	
	// Scan and index all plot files
	plotCount, keyCount, err := indexPlotFiles(db, config.PlotDirectories)
	if err != nil {
		return fmt.Errorf("failed to index plot files: %w", err)
	}
	
	fmt.Printf("Indexed %d plot files with %d total keys\n", plotCount, keyCount)
	fmt.Printf("Farming service ready!\n")
	
	// Wait for context cancellation (timeout or interrupt)
	<-ctx.Done()
	
	if ctx.Err() == context.DeadlineExceeded {
		fmt.Printf("Farm timeout reached\n")
	} else {
		fmt.Printf("Farm interrupted\n")
	}
	
	return nil
}

func cleanupOldDatabase(dbPath string) error {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return nil // Nothing to clean up
	}
	
	fmt.Printf("Cleaning up old database at %s\n", dbPath)
	return os.RemoveAll(dbPath)
}

func indexPlotFiles(db *badger.DB, plotDirectories []string) (int, int, error) {
	plotCount := 0
	keyCount := 0
	
	for _, plotDir := range plotDirectories {
		fmt.Printf("Scanning plot directory: %s\n", plotDir)
		
		// Check if directory exists
		if _, err := os.Stat(plotDir); os.IsNotExist(err) {
			fmt.Printf("Warning: plot directory '%s' does not exist, skipping\n", plotDir)
			continue
		}
		
		// Find all umbra plot files
		plotFiles, err := findPlotFiles(plotDir)
		if err != nil {
			return plotCount, keyCount, fmt.Errorf("failed to find plot files in '%s': %w", plotDir, err)
		}
		
		fmt.Printf("Found %d plot files in %s\n", len(plotFiles), plotDir)
		
		// Index each plot file
		for _, plotFile := range plotFiles {
			keys, err := indexPlotFile(db, plotFile)
			if err != nil {
				fmt.Printf("Warning: failed to index plot file '%s': %v\n", plotFile, err)
				continue
			}
			
			plotCount++
			keyCount += keys
			fmt.Printf("Indexed %s (%d keys)\n", filepath.Base(plotFile), keys)
		}
	}
	
	return plotCount, keyCount, nil
}

func findPlotFiles(directory string) ([]string, error) {
	var plotFiles []string
	
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if info.IsDir() {
			return nil
		}
		
		// Check if file matches umbra plot naming scheme
		filename := info.Name()
		if strings.HasPrefix(filename, "umbra_v1_k") && strings.HasSuffix(filename, ".dat") {
			plotFiles = append(plotFiles, path)
		}
		
		return nil
	})
	
	return plotFiles, err
}

func indexPlotFile(db *badger.DB, plotFile string) (int, error) {
	// Read and parse plot file header
	file, err := os.Open(plotFile)
	if err != nil {
		return 0, fmt.Errorf("failed to open plot file: %w", err)
	}
	defer file.Close()
	
	// Read plot header
	var header PlotHeader
	if err := header.ReadFrom(file); err != nil {
		return 0, fmt.Errorf("failed to read plot header: %w", err)
	}
	
	// Validate header
	if header.Version != 1 {
		return 0, fmt.Errorf("unsupported plot version: %d", header.Version)
	}
	
	keyCount := 0
	batchSize := 10000 // Process in batches to avoid transaction size limits
	
	// Index entries in batches
	for i := 0; i < len(header.Entries); i += batchSize {
		end := i + batchSize
		if end > len(header.Entries) {
			end = len(header.Entries)
		}
		
		batch := header.Entries[i:end]
		
		// Process this batch in a transaction
		err = db.Update(func(txn *badger.Txn) error {
			for _, entry := range batch {
				// Use the identifier as the key (better for challenge matching)
				// and store plot file + offset as value
				plotEntry := PlotEntry{
					FilePath: plotFile,
					Offset:   int64(entry.Offset),
				}
				
				value, err := encodePlotEntry(plotEntry)
				if err != nil {
					return fmt.Errorf("failed to encode plot entry: %w", err)
				}
				
				// Store in BadgerDB (identifier -> plot entry)
				if err := txn.Set(entry.Identifier[:], value); err != nil {
					return fmt.Errorf("failed to store key in database: %w", err)
				}
				
				keyCount++
			}
			return nil
		})
		
		if err != nil {
			return keyCount, fmt.Errorf("failed to index batch starting at %d: %w", i, err)
		}
		
		// Progress indicator for large files
		if len(header.Entries) > 50000 && (i+batchSize)%50000 == 0 {
			fmt.Printf("  Indexed %d/%d keys (%.1f%%)\n", 
				keyCount, len(header.Entries), 
				float64(keyCount)/float64(len(header.Entries))*100)
		}
	}
	
	return keyCount, nil
}

func encodePlotEntry(entry PlotEntry) ([]byte, error) {
	// Simple encoding: [path_len:4][path][offset:8]
	pathBytes := []byte(entry.FilePath)
	pathLen := len(pathBytes)
	
	value := make([]byte, 4+pathLen+8)
	
	// Encode path length
	binary.LittleEndian.PutUint32(value[0:4], uint32(pathLen))
	
	// Encode path
	copy(value[4:4+pathLen], pathBytes)
	
	// Encode offset
	binary.LittleEndian.PutUint64(value[4+pathLen:4+pathLen+8], uint64(entry.Offset))
	
	return value, nil
}

func decodePlotEntry(value []byte) (PlotEntry, error) {
	if len(value) < 12 { // Minimum: 4 (path_len) + 0 (path) + 8 (offset)
		return PlotEntry{}, fmt.Errorf("invalid plot entry data")
	}
	
	// Decode path length
	pathLen := binary.LittleEndian.Uint32(value[0:4])
	
	if len(value) < int(4+pathLen+8) {
		return PlotEntry{}, fmt.Errorf("invalid plot entry data length")
	}
	
	// Decode path
	path := string(value[4 : 4+pathLen])
	
	// Decode offset
	offset := binary.LittleEndian.Uint64(value[4+pathLen : 4+pathLen+8])
	
	return PlotEntry{
		FilePath: path,
		Offset:   int64(offset),
	}, nil
}