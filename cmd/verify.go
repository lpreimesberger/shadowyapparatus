package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var verifyCmd = &cobra.Command{
	Use:   "verify [proof]",
	Short: "Verify a cryptographic proof",
	Long: `Verify that a proof is valid by checking:
- The public key generates the claimed address and identifier
- The identifier meets the difficulty requirement
- The signature is valid for the challenge data`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		proofStr := args[0]
		
		if err := verifyProof(proofStr); err != nil {
			fmt.Printf("Proof verification failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Proof is valid!\n")
		os.Exit(0)
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)
}

func verifyProof(proofStr string) error {
	proof, err := DecodeDifficultyProof(proofStr)
	if err != nil {
		return fmt.Errorf("failed to decode proof: %w", err)
	}
	
	fmt.Printf("Verifying proof with difficulty %d\n", proof.Difficulty)
	
	// Verify the public key generates the claimed address and identifier
	expectedAddress := generateAddress(proof.PublicKey[:])
	if expectedAddress != proof.Address {
		return fmt.Errorf("address mismatch: claimed %x, computed %x", 
			proof.Address[:], expectedAddress[:])
	}
	
	expectedIdentifier := generateIdentifier(proof.PublicKey[:])
	if expectedIdentifier != proof.Identifier {
		return fmt.Errorf("identifier mismatch: claimed %x, computed %x", 
			proof.Identifier[:], expectedIdentifier[:])
	}
	
	// Verify the identifier meets the difficulty requirement
	if !checkDifficulty(proof.Identifier, proof.Difficulty) {
		return fmt.Errorf("identifier %x does not meet difficulty %d", 
			proof.Identifier[:], proof.Difficulty)
	}
	
	// Extract challenge data for signature verification
	_, challengeData, err := parseChallenge(proof.Challenge)
	if err != nil {
		return fmt.Errorf("failed to parse challenge: %w", err)
	}
	
	// Verify the signature
	if !VerifySignature(proof.PublicKey[:], challengeData, proof.Signature) {
		return fmt.Errorf("invalid signature")
	}
	
	fmt.Printf("✓ Address verification passed\n")
	fmt.Printf("✓ Identifier verification passed\n")
	fmt.Printf("✓ Difficulty requirement met (%d zero bits)\n", proof.Difficulty)
	fmt.Printf("✓ Signature verification passed\n")
	
	return nil
}