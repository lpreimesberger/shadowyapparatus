package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
)

var challengeCmd = &cobra.Command{
	Use:   "challenge [difficulty]",
	Short: "Generate a cryptographic challenge",
	Long: `Generate a challenge that requires finding a key with a SHAKE128 identifier
that starts with a specific number of zero bits (difficulty).

Examples:
  shadowy challenge 8    # Find identifier starting with 8 zero bits (1 zero byte)
  shadowy challenge 16   # Find identifier starting with 16 zero bits (2 zero bytes)`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		difficulty, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Printf("Invalid difficulty: %v\n", err)
			return
		}
		
		challenge, err := generateChallenge(difficulty)
		if err != nil {
			fmt.Printf("Error generating challenge: %v\n", err)
			return
		}
		
		fmt.Printf("Challenge: %s\n", challenge)
		fmt.Printf("Difficulty: %d bits\n", difficulty)
		fmt.Printf("Target: identifier must start with %d zero bits\n", difficulty)
	},
}

func init() {
	rootCmd.AddCommand(challengeCmd)
}

func generateChallenge(difficulty int) (string, error) {
	if difficulty < 1 || difficulty > 64 {
		return "", fmt.Errorf("difficulty must be between 1 and 64 bits")
	}
	
	// Generate random challenge data
	challengeData := make([]byte, 32)
	if _, err := rand.Read(challengeData); err != nil {
		return "", fmt.Errorf("failed to generate random challenge: %w", err)
	}
	
	// Encode as difficulty:challenge_hex
	challenge := fmt.Sprintf("%d:%s", difficulty, hex.EncodeToString(challengeData))
	return challenge, nil
}

func parseChallenge(challengeStr string) (int, []byte, error) {
	// Find the colon separator
	colonPos := -1
	for i, char := range challengeStr {
		if char == ':' {
			colonPos = i
			break
		}
	}
	
	if colonPos == -1 {
		return 0, nil, fmt.Errorf("invalid challenge format, expected difficulty:hex")
	}
	
	difficultyStr := challengeStr[:colonPos]
	challengeHex := challengeStr[colonPos+1:]
	
	difficulty, err := strconv.Atoi(difficultyStr)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid difficulty: %w", err)
	}
	
	challengeData, err := hex.DecodeString(challengeHex)
	if err != nil {
		return 0, nil, fmt.Errorf("invalid challenge hex: %w", err)
	}
	
	return difficulty, challengeData, nil
}

func checkDifficulty(identifier [IdentifierSize]byte, difficulty int) bool {
	if difficulty <= 0 {
		return true
	}
	
	zeroBits := 0
	for _, b := range identifier {
		if b == 0 {
			zeroBits += 8
		} else {
			// Count leading zeros in this byte
			for i := 7; i >= 0; i-- {
				if (b>>i)&1 == 0 {
					zeroBits++
				} else {
					break
				}
			}
			break
		}
		
		if zeroBits >= difficulty {
			break
		}
	}
	
	return zeroBits >= difficulty
}

func findMatchingKey(plotFile string, challenge string) (*KeyPair, error) {
	difficulty, _, err := parseChallenge(challenge)
	if err != nil {
		return nil, fmt.Errorf("failed to parse challenge: %w", err)
	}
	
	file, err := os.Open(plotFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open plot file: %w", err)
	}
	defer file.Close()
	
	var header PlotHeader
	if err := header.ReadFrom(file); err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}
	
	fmt.Printf("Searching %d keys for difficulty %d challenge...\n", len(header.Entries), difficulty)
	
	for i, entry := range header.Entries {
		if i%10000 == 0 && i > 0 {
			fmt.Printf("Checked %d/%d keys (%.1f%%)\n", 
				i, len(header.Entries), float64(i)/float64(len(header.Entries))*100)
		}
		
		if checkDifficulty(entry.Identifier, difficulty) {
			privateKey, err := loadPrivateKey(file, entry.Offset)
			if err != nil {
				return nil, fmt.Errorf("failed to load private key: %w", err)
			}
			
			keyPair, err := reconstructKeyPair(privateKey)
			if err != nil {
				return nil, fmt.Errorf("failed to reconstruct key pair: %w", err)
			}
			
			fmt.Printf("Found matching key! Identifier: %s\n", keyPair.IdentifierHex())
			return keyPair, nil
		}
	}
	
	return nil, fmt.Errorf("no key found matching difficulty %d", difficulty)
}