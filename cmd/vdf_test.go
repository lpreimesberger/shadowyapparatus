package cmd

import (
	"fmt"
	"math/big"
	"testing"
	"time"
)

func TestVDFConfigDefaults(t *testing.T) {
	config := DefaultVDFConfig()
	
	if config == nil {
		t.Fatal("DefaultVDFConfig returned nil")
	}
	
	if config.ModulusBits != 2048 {
		t.Errorf("Expected ModulusBits to be 2048, got %d", config.ModulusBits)
	}
	
	if config.TimeParameter == 0 {
		t.Error("TimeParameter should be non-zero")
	}
	
	if config.Modulus == nil {
		t.Error("Modulus should not be nil")
	}
	
	if config.SecurityParameter != 256 {
		t.Errorf("Expected SecurityParameter to be 256, got %d", config.SecurityParameter)
	}
}

func TestVDFChallengeGeneration(t *testing.T) {
	config := DefaultVDFConfig()
	config.TimeParameter = 1000 // Small value for testing
	solver := NewVDFSolver(config)
	
	inputData := []byte("test challenge data")
	challenge := solver.GenerateChallenge(inputData)
	
	if challenge == nil {
		t.Fatal("GenerateChallenge returned nil")
	}
	
	if challenge.Input == nil {
		t.Error("Challenge input should not be nil")
	}
	
	if challenge.TimeParameter != config.TimeParameter {
		t.Errorf("Expected TimeParameter %d, got %d", config.TimeParameter, challenge.TimeParameter)
	}
	
	if challenge.Modulus.Cmp(config.Modulus) != 0 {
		t.Error("Challenge modulus should match config modulus")
	}
	
	if challenge.ID == "" {
		t.Error("Challenge ID should not be empty")
	}
	
	if challenge.CreatedAt.IsZero() {
		t.Error("Challenge creation time should be set")
	}
	
	// Test that same input produces same challenge
	challenge2 := solver.GenerateChallenge(inputData)
	if challenge.ID != challenge2.ID {
		t.Error("Same input should produce same challenge ID")
	}
	
	if challenge.Input.Cmp(challenge2.Input) != 0 {
		t.Error("Same input should produce same challenge input")
	}
}

func TestVDFSmallExample(t *testing.T) {
	// Create a small test configuration for faster computation
	config := &VDFConfig{
		ModulusBits:       512, // Smaller for testing
		TimeParameter:     10,  // Very small T for quick test
		Modulus:           generateRSAModulus(512),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	solver := NewVDFSolver(config)
	verifier := NewVDFVerifier(config)
	
	inputData := []byte("small test")
	challenge := solver.GenerateChallenge(inputData)
	
	// Solve VDF
	proof, err := solver.Solve(challenge)
	if err != nil {
		t.Fatalf("Failed to solve VDF: %v", err)
	}
	
	if proof == nil {
		t.Fatal("Solve returned nil proof")
	}
	
	if proof.Challenge != challenge {
		t.Error("Proof should reference the original challenge")
	}
	
	if proof.Output == nil {
		t.Error("Proof output should not be nil")
	}
	
	if proof.Proof == nil {
		t.Error("Proof value should not be nil")
	}
	
	if proof.ComputeTime <= 0 {
		t.Error("Compute time should be positive")
	}
	
	// Verify proof
	isValid, err := verifier.Verify(proof)
	if err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
	
	if !isValid {
		t.Error("Valid proof was marked as invalid")
	}
}

func TestVDFVerificationFailure(t *testing.T) {
	config := &VDFConfig{
		ModulusBits:       512,
		TimeParameter:     10,
		Modulus:           generateRSAModulus(512),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	solver := NewVDFSolver(config)
	verifier := NewVDFVerifier(config)
	
	inputData := []byte("test for invalid proof")
	challenge := solver.GenerateChallenge(inputData)
	
	proof, err := solver.Solve(challenge)
	if err != nil {
		t.Fatalf("Failed to solve VDF: %v", err)
	}
	
	// Tamper with the proof to make it invalid
	proof.Output = big.NewInt(12345) // Wrong output
	
	isValid, err := verifier.Verify(proof)
	if err != nil {
		t.Fatalf("Verification failed with error: %v", err)
	}
	
	if isValid {
		t.Error("Tampered proof should be invalid")
	}
}

func TestVDFNilInputs(t *testing.T) {
	verifier := NewVDFVerifier(nil) // Should use default config
	
	// Test with nil proof
	isValid, err := verifier.Verify(nil)
	if err == nil {
		t.Error("Verifying nil proof should return error")
	}
	if isValid {
		t.Error("Nil proof should not be valid")
	}
	
	// Test with proof containing nil challenge
	proof := &VDFProof{
		Challenge: nil,
		Output:    big.NewInt(123),
		Proof:     big.NewInt(456),
	}
	
	isValid, err = verifier.Verify(proof)
	if err == nil {
		t.Error("Verifying proof with nil challenge should return error")
	}
	if isValid {
		t.Error("Proof with nil challenge should not be valid")
	}
}

func TestRepeatedSquaring(t *testing.T) {
	config := &VDFConfig{
		ModulusBits:   512,
		TimeParameter: 5,
		Modulus:       generateRSAModulus(512),
	}
	
	solver := NewVDFSolver(config)
	
	x := big.NewInt(3)
	N := config.Modulus
	T := uint64(3)
	
	result := solver.repeatedSquaring(x, T, N)
	
	// Manually compute x^(2^3) = x^8 mod N
	expected := new(big.Int).Exp(x, big.NewInt(8), N)
	
	if result.Cmp(expected) != 0 {
		t.Errorf("Repeated squaring incorrect: expected %s, got %s", expected.String(), result.String())
	}
}

func TestFiatShamirChallenge(t *testing.T) {
	config := DefaultVDFConfig()
	solver := NewVDFSolver(config)
	verifier := NewVDFVerifier(config)
	
	x := big.NewInt(123)
	y := big.NewInt(456)
	N := config.Modulus
	
	// Both solver and verifier should generate the same challenge
	solverChallenge := solver.generateFiatShamirChallenge(x, y, N)
	verifierChallenge := verifier.generateFiatShamirChallenge(x, y, N)
	
	if solverChallenge.Cmp(verifierChallenge) != 0 {
		t.Error("Solver and verifier should generate the same Fiat-Shamir challenge")
	}
	
	// Challenge should be odd
	if solverChallenge.Bit(0) == 0 {
		t.Error("Fiat-Shamir challenge should be odd")
	}
	
	// Different inputs should produce different challenges
	differentChallenge := solver.generateFiatShamirChallenge(big.NewInt(789), y, N)
	if solverChallenge.Cmp(differentChallenge) == 0 {
		t.Error("Different inputs should produce different challenges")
	}
}

func TestComputeRemainder(t *testing.T) {
	config := DefaultVDFConfig()
	solver := NewVDFSolver(config)
	
	T := uint64(10)
	l := big.NewInt(7)
	
	remainder := solver.computeRemainder(T, l)
	
	// 2^10 = 1024, 1024 mod 7 = 2
	expected := big.NewInt(2)
	
	if remainder.Cmp(expected) != 0 {
		t.Errorf("Compute remainder incorrect: expected %s, got %s", expected.String(), remainder.String())
	}
}

func TestComputeQuotient(t *testing.T) {
	config := DefaultVDFConfig()
	solver := NewVDFSolver(config)
	
	T := uint64(10)
	l := big.NewInt(7)
	
	quotient := solver.computeQuotient(T, l)
	
	// floor(2^10 / 7) = floor(1024 / 7) = 146
	expected := big.NewInt(146)
	
	if quotient.Cmp(expected) != 0 {
		t.Errorf("Compute quotient incorrect: expected %s, got %s", expected.String(), quotient.String())
	}
}

func TestComputeAndVerifyVDF(t *testing.T) {
	config := &VDFConfig{
		ModulusBits:       512,
		TimeParameter:     8,
		Modulus:           generateRSAModulus(512),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	inputData := []byte("end-to-end test")
	result := ComputeAndVerifyVDF(config, inputData)
	
	if result == nil {
		t.Fatal("ComputeAndVerifyVDF returned nil")
	}
	
	if result.Error != "" {
		t.Fatalf("ComputeAndVerifyVDF failed: %s", result.Error)
	}
	
	if result.Proof == nil {
		t.Error("Result should contain a proof")
	}
	
	if !result.IsValid {
		t.Error("Result should be valid")
	}
	
	if result.VerifyTime <= 0 {
		t.Error("Verify time should be positive")
	}
}

func TestVDFWithDifferentModuli(t *testing.T) {
	// Test with different modulus sizes
	sizes := []int{256, 512} // Keep sizes small for testing
	
	for _, size := range sizes {
		t.Run(fmt.Sprintf("Modulus%dBits", size), func(t *testing.T) {
			config := &VDFConfig{
				ModulusBits:       size,
				TimeParameter:     5,
				Modulus:           generateRSAModulus(size),
				SecurityParameter: 128,
				TargetSolvingTime: time.Second,
			}
			
			result := ComputeAndVerifyVDF(config, []byte("test data"))
			
			if result.Error != "" {
				t.Errorf("VDF failed with %d-bit modulus: %s", size, result.Error)
			}
			
			if !result.IsValid {
				t.Errorf("VDF proof invalid with %d-bit modulus", size)
			}
		})
	}
}

func TestVDFConsistency(t *testing.T) {
	config := &VDFConfig{
		ModulusBits:       512,
		TimeParameter:     6,
		Modulus:           generateRSAModulus(512),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	inputData := []byte("consistency test")
	
	// Run VDF computation multiple times with same input
	results := make([]*VDFResult, 3)
	for i := 0; i < 3; i++ {
		results[i] = ComputeAndVerifyVDF(config, inputData)
		if results[i].Error != "" {
			t.Fatalf("VDF computation %d failed: %s", i, results[i].Error)
		}
	}
	
	// All results should have the same output
	for i := 1; i < len(results); i++ {
		if results[0].Proof.Output.Cmp(results[i].Proof.Output) != 0 {
			t.Error("VDF should produce consistent outputs for same input")
		}
		
		if results[0].Proof.Challenge.ID != results[i].Proof.Challenge.ID {
			t.Error("VDF should produce consistent challenge IDs for same input")
		}
	}
}

func BenchmarkVDFSolveSmall(b *testing.B) {
	config := &VDFConfig{
		ModulusBits:       256, // Very small for benchmarking
		TimeParameter:     50,
		Modulus:           generateRSAModulus(256),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	solver := NewVDFSolver(config)
	inputData := []byte("benchmark test")
	challenge := solver.GenerateChallenge(inputData)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := solver.Solve(challenge)
		if err != nil {
			b.Fatalf("Solve failed: %v", err)
		}
	}
}

func BenchmarkVDFVerify(b *testing.B) {
	config := &VDFConfig{
		ModulusBits:       256,
		TimeParameter:     50,
		Modulus:           generateRSAModulus(256),
		SecurityParameter: 128,
		TargetSolvingTime: time.Second,
	}
	
	solver := NewVDFSolver(config)
	verifier := NewVDFVerifier(config)
	
	inputData := []byte("benchmark verify")
	challenge := solver.GenerateChallenge(inputData)
	proof, err := solver.Solve(challenge)
	if err != nil {
		b.Fatalf("Failed to create proof for benchmark: %v", err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := verifier.Verify(proof)
		if err != nil {
			b.Fatalf("Verify failed: %v", err)
		}
	}
}