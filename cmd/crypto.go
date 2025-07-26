package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"unsafe"

	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"golang.org/x/crypto/sha3"
)

const (
	SeedSize       = mldsa87.SeedSize       // 32 bytes - for key generation
	PrivateKeySize = mldsa87.PrivateKeySize // 4896 bytes - full private key
	PublicKeySize  = mldsa87.PublicKeySize  // 2592 bytes
	SignatureSize  = mldsa87.SignatureSize  // 4627 bytes
	AddressSize    = 20                     // Keep same address size
	IdentifierSize = 16                     // Keep same identifier size
)

type KeyPair struct {
	Seed       [SeedSize]byte       // Store seed for key reconstruction
	PrivateKey [PrivateKeySize]byte // Cache full private key
	PublicKey  [PublicKeySize]byte
	Address    [AddressSize]byte
	Identifier [IdentifierSize]byte
}

func GenerateKeyPair() (*KeyPair, error) {
	// Generate a random seed
	seed := make([]byte, SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, fmt.Errorf("failed to generate random seed: %w", err)
	}
	
	// Convert to fixed-size array
	var seedArray [SeedSize]byte
	copy(seedArray[:], seed)
	
	return NewKeyPairFromSeed(seedArray)
}

func NewKeyPairFromSeed(seed [SeedSize]byte) (*KeyPair, error) {
	// Generate key pair from seed
	pubKey, privKey := mldsa87.NewKeyFromSeed(&seed)
	if pubKey == nil || privKey == nil {
		return nil, fmt.Errorf("failed to generate ML-DSA key pair from seed")
	}
	
	kp := &KeyPair{}
	kp.Seed = seed
	
	privKeyBytes := privKey.Bytes()
	pubKeyBytes := pubKey.Bytes()
	
	copy(kp.PrivateKey[:], privKeyBytes)
	copy(kp.PublicKey[:], pubKeyBytes)
	
	kp.Address = generateAddress(pubKeyBytes)
	kp.Identifier = generateIdentifier(kp.PublicKey[:])
	
	return kp, nil
}

func generateAddress(pubKey []byte) [AddressSize]byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write(pubKey)
	fullHash := hash.Sum(nil)
	
	var addr [AddressSize]byte
	copy(addr[:], fullHash[12:])
	return addr
}

func generateIdentifier(data []byte) [IdentifierSize]byte {
	shake := sha3.NewShake128()
	shake.Write(data)
	
	var identifier [IdentifierSize]byte
	shake.Read(identifier[:])
	return identifier
}

func (kp *KeyPair) PrivateKeyHex() string {
	return hex.EncodeToString(kp.PrivateKey[:])
}

func (kp *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(kp.PublicKey[:])
}

func (kp *KeyPair) AddressHex() string {
	return hex.EncodeToString(kp.Address[:])
}

func (kp *KeyPair) IdentifierHex() string {
	return hex.EncodeToString(kp.Identifier[:])
}

func (kp *KeyPair) SeedHex() string {
	return hex.EncodeToString(kp.Seed[:])
}

func (kp *KeyPair) Sign(message []byte) (signature []byte, err error) {
	// Check that seed is valid (not all zeros)
	allZeros := true
	for _, b := range kp.Seed {
		if b != 0 {
			allZeros = false
			break
		}
	}
	if allZeros {
		return nil, fmt.Errorf("key seed is empty or invalid")
	}
	
	// Reconstruct private key from seed (safe approach)
	_, privKey := mldsa87.NewKeyFromSeed(&kp.Seed)
	if privKey == nil {
		return nil, fmt.Errorf("failed to reconstruct private key from seed")
	}
	
	// Sign the message
	signature = make([]byte, SignatureSize)
	signErr := mldsa87.SignTo(privKey, message, nil, false, signature)
	if signErr != nil {
		return nil, fmt.Errorf("failed to sign message: %w", signErr)
	}
	
	return signature, nil
}

func VerifySignature(pubKeyBytes, message, signature []byte) bool {
	if len(pubKeyBytes) != PublicKeySize {
		return false
	}
	
	if len(signature) != SignatureSize {
		return false
	}
	
	pubKey := (*mldsa87.PublicKey)(unsafe.Pointer(&pubKeyBytes[0]))
	return mldsa87.Verify(pubKey, message, nil, signature)
}

func FindClosestMatch(target [IdentifierSize]byte, identifiers [][IdentifierSize]byte) int {
	if len(identifiers) == 0 {
		return -1
	}
	
	bestIndex := 0
	bestDistance := hammingDistance(target, identifiers[0])
	
	for i := 1; i < len(identifiers); i++ {
		distance := hammingDistance(target, identifiers[i])
		if distance < bestDistance {
			bestDistance = distance
			bestIndex = i
		}
	}
	
	return bestIndex
}

func hammingDistance(a, b [IdentifierSize]byte) int {
	distance := 0
	for i := 0; i < IdentifierSize; i++ {
		xor := a[i] ^ b[i]
		for xor != 0 {
			distance++
			xor &= xor - 1
		}
	}
	return distance
}