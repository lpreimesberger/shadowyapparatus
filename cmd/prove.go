package cmd

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var proveCmd = &cobra.Command{
	Use:   "prove [plot-file] [challenge]",
	Short: "Generate a proof by finding a key that meets the challenge difficulty",
	Long: `Generate a cryptographic proof by finding a key with a SHAKE128 identifier
that starts with the required number of zero bits, then signing the challenge.

The challenge format is: difficulty:challenge_hex
Example: 8:deadbeef1234567890abcdef...`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		plotFile := args[0]
		challenge := args[1]
		
		proof, err := generateProofForChallenge(plotFile, challenge)
		if err != nil {
			fmt.Printf("Error generating proof: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Proof: %s\n", proof)
	},
}

func init() {
	rootCmd.AddCommand(proveCmd)
}

func generateProofForChallenge(plotFilePath, challenge string) (string, error) {
	keyPair, err := findMatchingKey(plotFilePath, challenge)
	if err != nil {
		return "", fmt.Errorf("failed to find matching key: %w", err)
	}
	
	difficulty, challengeData, err := parseChallenge(challenge)
	if err != nil {
		return "", fmt.Errorf("failed to parse challenge: %w", err)
	}
	
	signature, err := keyPair.Sign(challengeData)
	if err != nil {
		return "", fmt.Errorf("failed to sign challenge: %w", err)
	}
	
	proof := DifficultyProof{
		Challenge:   challenge,
		PublicKey:   keyPair.PublicKey,
		Address:     keyPair.Address,
		Identifier:  keyPair.Identifier,
		Signature:   signature,
		Difficulty:  difficulty,
	}
	
	return proof.Encode(), nil
}

func loadPrivateKey(file *os.File, offset int32) ([PrivateKeySize]byte, error) {
	var privateKey [PrivateKeySize]byte
	
	if _, err := file.Seek(int64(offset), 0); err != nil {
		return privateKey, fmt.Errorf("failed to seek to offset %d: %w", offset, err)
	}
	
	if _, err := file.Read(privateKey[:]); err != nil {
		return privateKey, fmt.Errorf("failed to read private key: %w", err)
	}
	
	return privateKey, nil
}

type DifficultyProof struct {
	Challenge  string
	PublicKey  [PublicKeySize]byte
	Address    [AddressSize]byte
	Identifier [IdentifierSize]byte
	Signature  []byte
	Difficulty int
}

func (dp *DifficultyProof) Encode() string {
	result := dp.Challenge
	result += "|" + hex.EncodeToString(dp.PublicKey[:])
	result += "|" + hex.EncodeToString(dp.Address[:])
	result += "|" + hex.EncodeToString(dp.Identifier[:])
	result += "|" + hex.EncodeToString(dp.Signature)
	return result
}

func DecodeDifficultyProof(encoded string) (*DifficultyProof, error) {
	parts := []string{}
	current := ""
	for _, char := range encoded {
		if char == '|' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(char)
		}
	}
	parts = append(parts, current)
	
	if len(parts) != 5 {
		return nil, fmt.Errorf("proof must have 5 parts, got %d", len(parts))
	}
	
	dp := &DifficultyProof{}
	
	dp.Challenge = parts[0]
	
	difficulty, _, err := parseChallenge(dp.Challenge)
	if err != nil {
		return nil, fmt.Errorf("invalid challenge format: %w", err)
	}
	dp.Difficulty = difficulty
	
	pubKeyBytes, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid public key hex: %w", err)
	}
	copy(dp.PublicKey[:], pubKeyBytes)
	
	addressBytes, err := hex.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid address hex: %w", err)
	}
	copy(dp.Address[:], addressBytes)
	
	identifierBytes, err := hex.DecodeString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid identifier hex: %w", err)
	}
	copy(dp.Identifier[:], identifierBytes)
	
	dp.Signature, err = hex.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid signature hex: %w", err)
	}
	
	return dp, nil
}