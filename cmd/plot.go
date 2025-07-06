package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

var (
	plotDir string
	kSize   int
)

var plotCmd = &cobra.Command{
	Use:   "plot [directory]",
	Short: "Create a proof-of-storage plot",
	Long: `Create a proof-of-storage plot in the specified directory.
The plot will be used to prove storage capacity for the Shadowy network.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plotDir = args[0]
		
		if err := createPlot(plotDir, kSize); err != nil {
			fmt.Printf("Error creating plot: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Successfully created plot in %s with k-size %d\n", plotDir, kSize)
	},
}

func init() {
	plotCmd.Flags().IntVarP(&kSize, "k-size", "k", 32, "Size parameter for the proof (default: 32)")
}

func createPlot(directory string, k int) error {
	absDir, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	
	plotFile := filepath.Join(absDir, generatePlotFilename(k))
	
	file, err := os.Create(plotFile)
	if err != nil {
		return fmt.Errorf("failed to create plot file: %w", err)
	}
	defer file.Close()
	
	keyPairs, privateKeys, err := generateCryptoKeys(k)
	if err != nil {
		return fmt.Errorf("failed to generate crypto keys: %w", err)
	}
	
	header := PlotHeader{
		Version: PlotVersion,
		K:       int32(k),
		Entries: keyPairs,
	}
	
	fmt.Printf("Creating plot with k=%d (%d keys) in %s\n", k, len(keyPairs), plotFile)
	
	if err := header.WriteTo(file); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	
	if err := writePrivateKeys(file, privateKeys); err != nil {
		return fmt.Errorf("failed to write private keys: %w", err)
	}
	
	totalSize := int64(header.Size()) + int64(len(privateKeys)*PrivateKeySize)
	fmt.Printf("Plot created successfully (total size: %d bytes)\n", totalSize)
	
	return nil
}

func generatePlotFilename(k int) string {
	// Generate timestamp
	timestamp := time.Now().UTC().Format("20060102-150405")
	
	// Generate random 4-byte suffix for uniqueness within same second
	randomBytes := make([]byte, 4)
	rand.Read(randomBytes)
	randomSuffix := hex.EncodeToString(randomBytes)
	
	return fmt.Sprintf("umbra_v1_k%d_%s_%s.dat", k, timestamp, randomSuffix)
}

func generateCryptoKeys(k int) ([]AddressOffsetPair, [][PrivateKeySize]byte, error) {
	numKeys := 1 << k
	if k > 20 {
		numKeys = 1048576
	}
	
	pairs := make([]AddressOffsetPair, numKeys)
	privateKeys := make([][PrivateKeySize]byte, numKeys)
	
	headerSize := 8 + 4 + 4 + numKeys*(AddressSize+IdentifierSize+4)
	
	fmt.Printf("Generating %d ML-DSA-87 keys...\n", numKeys)
	
	for i := 0; i < numKeys; i++ {
		keyPair, err := GenerateKeyPair()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to generate key pair %d: %w", i, err)
		}
		
		pairs[i] = AddressOffsetPair{
			Address:    keyPair.Address,
			Identifier: keyPair.Identifier,
			Offset:     int32(headerSize + i*PrivateKeySize),
		}
		
		privateKeys[i] = keyPair.PrivateKey
		
		if i%1000 == 0 && i > 0 {
			fmt.Printf("Generated %d/%d keys (%.1f%%)\n", i, numKeys, float64(i)/float64(numKeys)*100)
		}
	}
	
	return pairs, privateKeys, nil
}

func writePrivateKeys(file *os.File, privateKeys [][PrivateKeySize]byte) error {
	for i, privKey := range privateKeys {
		if _, err := file.Write(privKey[:]); err != nil {
			return fmt.Errorf("failed to write private key %d: %w", i, err)
		}
	}
	return nil
}