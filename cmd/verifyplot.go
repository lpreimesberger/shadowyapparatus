package cmd

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"github.com/spf13/cobra"
)

var verifyplotCmd = &cobra.Command{
	Use:   "verifyplot [plot-file]",
	Short: "Verify the integrity of a plot file",
	Long: `Verify that a plot file is valid by checking:
- File format and header integrity
- Key consistency (addresses match public keys)
- Private key validity
- Identifier correctness (SHAKE128 of public keys)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		plotFile := args[0]
		
		if err := verifyPlotFile(plotFile); err != nil {
			fmt.Printf("Plot verification failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Plot file %s is valid\n", plotFile)
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(verifyplotCmd)
}

func verifyPlotFile(plotFilePath string) error {
	file, err := os.Open(plotFilePath)
	if err != nil {
		return fmt.Errorf("failed to open plot file: %w", err)
	}
	defer file.Close()
	
	var header PlotHeader
	if err := header.ReadFrom(file); err != nil {
		return fmt.Errorf("failed to read header: %w", err)
	}
	
	if err := validateHeader(&header); err != nil {
		return fmt.Errorf("invalid header: %w", err)
	}
	
	fmt.Printf("Header validation passed: version=%d, k=%d, entries=%d\n", 
		header.Version, header.K, len(header.Entries))
	
	if err := validateKeyConsistency(file, &header); err != nil {
		return fmt.Errorf("key consistency check failed: %w", err)
	}
	
	fmt.Printf("Key consistency validation passed\n")
	
	if err := validateFileSize(file, &header); err != nil {
		return fmt.Errorf("file size validation failed: %w", err)
	}
	
	fmt.Printf("File size validation passed\n")
	
	return nil
}

func validateHeader(header *PlotHeader) error {
	if header.Version != PlotVersion {
		return fmt.Errorf("unsupported version %d, expected %d", header.Version, PlotVersion)
	}
	
	if header.K < 1 || header.K > 32 {
		return fmt.Errorf("invalid k value %d, must be between 1 and 32", header.K)
	}
	
	expectedEntries := 1 << header.K
	if header.K > 20 {
		expectedEntries = 1048576
	}
	
	if len(header.Entries) != expectedEntries {
		return fmt.Errorf("wrong number of entries: got %d, expected %d for k=%d", 
			len(header.Entries), expectedEntries, header.K)
	}
	
	headerSize := 8 + 4 + 4 + len(header.Entries)*(AddressSize+IdentifierSize+4)
	for i, entry := range header.Entries {
		expectedOffset := int32(headerSize + i*PrivateKeySize)
		if entry.Offset != expectedOffset {
			return fmt.Errorf("entry %d has wrong offset: got %d, expected %d", 
				i, entry.Offset, expectedOffset)
		}
	}
	
	return nil
}

func validateKeyConsistency(file *os.File, header *PlotHeader) error {
	for i, entry := range header.Entries {
		if i%1000 == 0 && i > 0 {
			fmt.Printf("Validated %d/%d keys (%.1f%%)\n", 
				i, len(header.Entries), float64(i)/float64(len(header.Entries))*100)
		}
		
		privateKey, err := loadPrivateKey(file, entry.Offset)
		if err != nil {
			return fmt.Errorf("failed to load private key %d: %w", i, err)
		}
		
		keyPair, err := reconstructKeyPair(privateKey)
		if err != nil {
			return fmt.Errorf("failed to reconstruct key pair %d: %w", i, err)
		}
		
		if keyPair.Address != entry.Address {
			return fmt.Errorf("entry %d: address mismatch\nexpected: %x\ngot:      %x", 
				i, entry.Address[:], keyPair.Address[:])
		}
		
		if keyPair.Identifier != entry.Identifier {
			return fmt.Errorf("entry %d: identifier mismatch\nexpected: %x\ngot:      %x", 
				i, entry.Identifier[:], keyPair.Identifier[:])
		}
	}
	
	return nil
}

func validateFileSize(file *os.File, header *PlotHeader) error {
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file stats: %w", err)
	}
	
	expectedSize := int64(header.Size()) + int64(len(header.Entries)*PrivateKeySize)
	actualSize := stat.Size()
	
	if actualSize != expectedSize {
		return fmt.Errorf("file size mismatch: got %d bytes, expected %d bytes", 
			actualSize, expectedSize)
	}
	
	return nil
}

func reconstructKeyPair(privateKeyBytes [PrivateKeySize]byte) (*KeyPair, error) {
	privKey := (*mldsa87.PrivateKey)(unsafe.Pointer(&privateKeyBytes[0]))
	pubKey := privKey.Public().(*mldsa87.PublicKey)
	pubKeyBytes := (*[PublicKeySize]byte)(unsafe.Pointer(pubKey))
	
	kp := &KeyPair{}
	copy(kp.PrivateKey[:], privateKeyBytes[:])
	copy(kp.PublicKey[:], pubKeyBytes[:])
	
	kp.Address = generateAddress(pubKeyBytes[:])
	kp.Identifier = generateIdentifier(kp.PublicKey[:])
	
	return kp, nil
}