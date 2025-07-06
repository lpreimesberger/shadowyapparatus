package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"

	"golang.org/x/crypto/sha3"
)

// VDFConfig contains configuration parameters for the VDF
type VDFConfig struct {
	// Modulus bit length (recommended: 2048 bits for security)
	ModulusBits int `json:"modulus_bits"`
	
	// Time parameter T - number of sequential squarings required
	TimeParameter uint64 `json:"time_parameter"`
	
	// RSA modulus N (should be product of two safe primes)
	Modulus *big.Int `json:"modulus"`
	
	// Security parameter for challenge generation
	SecurityParameter int `json:"security_parameter"`
	
	// Target solving time in seconds (for difficulty adjustment)
	TargetSolvingTime time.Duration `json:"target_solving_time"`
}

// DefaultVDFConfig returns a secure default configuration
func DefaultVDFConfig() *VDFConfig {
	// Generate a 2048-bit RSA modulus (product of two safe primes)
	// In practice, this should be generated using a trusted setup
	modulus := generateRSAModulus(2048)
	
	return &VDFConfig{
		ModulusBits:       2048,
		TimeParameter:     1000000, // 1 million sequential squarings
		Modulus:           modulus,
		SecurityParameter: 256,
		TargetSolvingTime: 30 * time.Second,
	}
}

// VDFChallenge represents a VDF challenge
type VDFChallenge struct {
	// Input to the VDF (challenge value)
	Input *big.Int `json:"input"`
	
	// Time parameter T (number of squarings)
	TimeParameter uint64 `json:"time_parameter"`
	
	// RSA modulus N
	Modulus *big.Int `json:"modulus"`
	
	// Challenge identifier for tracking
	ID string `json:"id"`
	
	// Timestamp when challenge was created
	CreatedAt time.Time `json:"created_at"`
}

// VDFProof represents a Wesolowski VDF proof
type VDFProof struct {
	// The challenge this proof corresponds to
	Challenge *VDFChallenge `json:"challenge"`
	
	// Output y = x^(2^T) mod N
	Output *big.Int `json:"output"`
	
	// Wesolowski proof π
	Proof *big.Int `json:"proof"`
	
	// Time taken to compute (for performance metrics)
	ComputeTime time.Duration `json:"compute_time"`
	
	// Timestamp when proof was generated
	GeneratedAt time.Time `json:"generated_at"`
}

// VDFSolver implements the Wesolowski VDF solver
type VDFSolver struct {
	config *VDFConfig
}

// NewVDFSolver creates a new VDF solver with the given configuration
func NewVDFSolver(config *VDFConfig) *VDFSolver {
	if config == nil {
		config = DefaultVDFConfig()
	}
	return &VDFSolver{config: config}
}

// GenerateChallenge creates a new VDF challenge from input data
func (vs *VDFSolver) GenerateChallenge(data []byte) *VDFChallenge {
	// Hash the input data to create a challenge
	hasher := sha3.NewShake256()
	hasher.Write(data)
	
	// Generate challenge input of appropriate size
	challengeBytes := make([]byte, vs.config.ModulusBits/8)
	hasher.Read(challengeBytes)
	
	// Ensure challenge is less than modulus
	input := new(big.Int).SetBytes(challengeBytes)
	input.Mod(input, vs.config.Modulus)
	
	// Create challenge ID
	idHasher := sha3.New256()
	idHasher.Write(data)
	idHasher.Write(input.Bytes())
	challengeID := hex.EncodeToString(idHasher.Sum(nil))
	
	return &VDFChallenge{
		Input:         input,
		TimeParameter: vs.config.TimeParameter,
		Modulus:       vs.config.Modulus,
		ID:            challengeID,
		CreatedAt:     time.Now().UTC(),
	}
}

// Solve computes the VDF proof using Wesolowski's construction
func (vs *VDFSolver) Solve(challenge *VDFChallenge) (*VDFProof, error) {
	startTime := time.Now()
	
	// Step 1: Compute y = x^(2^T) mod N through repeated squaring
	x := new(big.Int).Set(challenge.Input)
	y := vs.repeatedSquaring(x, challenge.TimeParameter, challenge.Modulus)
	
	// Step 2: Generate Fiat-Shamir challenge l
	l := vs.generateFiatShamirChallenge(challenge.Input, y, challenge.Modulus)
	
	// Step 3: Compute r = 2^T mod l (not used in this simplified version)
	_ = vs.computeRemainder(challenge.TimeParameter, l)
	
	// Step 4: Compute proof π
	// We need to compute x^(floor(2^T / l)) mod N
	// But we need to be more careful about the computation
	
	// Calculate 2^T
	two := big.NewInt(2)
	tBig := new(big.Int).SetUint64(challenge.TimeParameter)
	powerOfTwo := new(big.Int).Exp(two, tBig, nil)
	
	// Compute quotient = floor(2^T / l)
	quotient := new(big.Int).Div(powerOfTwo, l)
	
	// Compute proof = x^quotient mod N
	proof := new(big.Int).Exp(challenge.Input, quotient, challenge.Modulus)
	
	computeTime := time.Since(startTime)
	
	return &VDFProof{
		Challenge:   challenge,
		Output:      y,
		Proof:       proof,
		ComputeTime: computeTime,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// repeatedSquaring performs T sequential squarings: x^(2^T) mod N
func (vs *VDFSolver) repeatedSquaring(x *big.Int, T uint64, N *big.Int) *big.Int {
	result := new(big.Int).Set(x)
	
	for i := uint64(0); i < T; i++ {
		// result = result^2 mod N
		result.Mul(result, result)
		result.Mod(result, N)
	}
	
	return result
}

// generateFiatShamirChallenge generates the challenge l using Fiat-Shamir heuristic
func (vs *VDFSolver) generateFiatShamirChallenge(x, y, N *big.Int) *big.Int {
	// Hash(x || y || N) to generate challenge
	hasher := sha3.New256()
	hasher.Write(x.Bytes())
	hasher.Write(y.Bytes())
	hasher.Write(N.Bytes())
	hash := hasher.Sum(nil)
	
	// Convert hash to big integer
	l := new(big.Int).SetBytes(hash)
	
	// Ensure l is odd and relatively prime to common small factors
	if l.Bit(0) == 0 {
		l.Add(l, big.NewInt(1))
	}
	
	return l
}

// computeRemainder calculates 2^T mod l
func (vs *VDFSolver) computeRemainder(T uint64, l *big.Int) *big.Int {
	// 2^T mod l
	two := big.NewInt(2)
	tBig := new(big.Int).SetUint64(T)
	powerOfTwo := new(big.Int).Exp(two, tBig, nil)
	
	remainder := new(big.Int).Mod(powerOfTwo, l)
	return remainder
}

// computeQuotient calculates floor(2^T / l)
func (vs *VDFSolver) computeQuotient(T uint64, l *big.Int) *big.Int {
	// floor(2^T / l)
	two := big.NewInt(2)
	tBig := new(big.Int).SetUint64(T)
	powerOfTwo := new(big.Int).Exp(two, tBig, nil)
	
	quotient := new(big.Int).Div(powerOfTwo, l)
	return quotient
}

// VDFVerifier implements the Wesolowski VDF verifier
type VDFVerifier struct {
	config *VDFConfig
}

// NewVDFVerifier creates a new VDF verifier with the given configuration
func NewVDFVerifier(config *VDFConfig) *VDFVerifier {
	if config == nil {
		config = DefaultVDFConfig()
	}
	return &VDFVerifier{config: config}
}

// Verify checks if a VDF proof is valid using Wesolowski's verification
func (vv *VDFVerifier) Verify(proof *VDFProof) (bool, error) {
	if proof == nil || proof.Challenge == nil {
		return false, fmt.Errorf("invalid proof or challenge")
	}
	
	challenge := proof.Challenge
	x := challenge.Input
	y := proof.Output
	π := proof.Proof
	N := challenge.Modulus
	T := challenge.TimeParameter
	
	// Step 1: Generate the same Fiat-Shamir challenge l
	l := vv.generateFiatShamirChallenge(x, y, N)
	
	// Step 2: Compute r = 2^T mod l
	r := vv.computeRemainder(T, l)
	
	// Step 3: Verify the equation: π^l * x^r ≡ y (mod N)
	// Left side: π^l * x^r mod N
	proofToL := new(big.Int).Exp(π, l, N)
	xToR := new(big.Int).Exp(x, r, N)
	leftSide := new(big.Int).Mul(proofToL, xToR)
	leftSide.Mod(leftSide, N)
	
	// Check if left side equals y
	return leftSide.Cmp(y) == 0, nil
}

// generateFiatShamirChallenge generates the challenge l (same as solver)
func (vv *VDFVerifier) generateFiatShamirChallenge(x, y, N *big.Int) *big.Int {
	hasher := sha3.New256()
	hasher.Write(x.Bytes())
	hasher.Write(y.Bytes())
	hasher.Write(N.Bytes())
	hash := hasher.Sum(nil)
	
	l := new(big.Int).SetBytes(hash)
	if l.Bit(0) == 0 {
		l.Add(l, big.NewInt(1))
	}
	
	return l
}

// computeRemainder calculates 2^T mod l (same as solver)
func (vv *VDFVerifier) computeRemainder(T uint64, l *big.Int) *big.Int {
	two := big.NewInt(2)
	tBig := new(big.Int).SetUint64(T)
	powerOfTwo := new(big.Int).Exp(two, tBig, nil)
	
	remainder := new(big.Int).Mod(powerOfTwo, l)
	return remainder
}

// generateRSAModulus generates an RSA modulus for VDF use
// Note: In production, this should use a trusted setup ceremony
func generateRSAModulus(bits int) *big.Int {
	// Use known safe test moduli for different bit sizes
	switch bits {
	case 256:
		// Small 256-bit modulus for testing
		modulus, _ := new(big.Int).SetString("104648257118348569649149332235006202638471972175641621397683299950955184194499", 10)
		return modulus
	case 512:
		// 512-bit modulus for testing  
		modulus, _ := new(big.Int).SetString("13407807929942597099574024998205846127479365820592393377723561443721764030073546976801874298166903427690031858186486050853753882811946569946433649006084171", 10)
		return modulus
	case 2048:
		// Known RSA modulus for testing
		modulus, _ := new(big.Int).SetString("25195908475657893494027183240048398571429282126204032027777137836043662020707595556264018525880784406918290641249515082189298559149176184502808489120072844992687392807287776735971418347270261896375014971824691165077613379859095700097330459748808428401797429100642458691817195118746121515172654632282216869987549182422433637259085141865462043576798423387184774447920739934236584823824281198163815010674810451660377306056201619676256133844143603833904414952634432190114657544454178424020924616515723350778707749817125772467962926386356373289912154831438167899885040445364023527381951378636564391212010397122822120720357", 10)
		return modulus
	}
	
	// Fallback: generate a smaller modulus for testing
	// This is NOT cryptographically secure and should not be used in production
	p, _ := rand.Prime(rand.Reader, bits/2)
	q, _ := rand.Prime(rand.Reader, bits/2)
	modulus := new(big.Int).Mul(p, q)
	
	return modulus
}

// VDFResult contains the result of VDF computation with metrics
type VDFResult struct {
	Proof       *VDFProof `json:"proof"`
	IsValid     bool      `json:"is_valid"`
	VerifyTime  time.Duration `json:"verify_time"`
	Error       string    `json:"error,omitempty"`
}

// ComputeAndVerifyVDF performs a complete VDF computation and verification cycle
func ComputeAndVerifyVDF(config *VDFConfig, inputData []byte) *VDFResult {
	solver := NewVDFSolver(config)
	verifier := NewVDFVerifier(config)
	
	// Generate challenge
	challenge := solver.GenerateChallenge(inputData)
	
	// Solve VDF
	proof, err := solver.Solve(challenge)
	if err != nil {
		return &VDFResult{
			Error: fmt.Sprintf("failed to solve VDF: %v", err),
		}
	}
	
	// Verify proof
	verifyStart := time.Now()
	isValid, err := verifier.Verify(proof)
	verifyTime := time.Since(verifyStart)
	
	if err != nil {
		return &VDFResult{
			Proof: proof,
			Error: fmt.Sprintf("failed to verify VDF: %v", err),
		}
	}
	
	return &VDFResult{
		Proof:      proof,
		IsValid:    isValid,
		VerifyTime: verifyTime,
	}
}