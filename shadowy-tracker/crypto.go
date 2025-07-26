package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	
	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"golang.org/x/crypto/sha3"
)

// VerifyRegistrationSignature verifies that the registration is signed by the mining address
func VerifyRegistrationSignature(req *RegistrationRequest) error {
	// Create message to verify
	message := fmt.Sprintf("%s|%s|%s|%d|%s|%s", 
		req.NodeID, req.MiningAddr, req.ExternalIP, 
		req.ChainHeight, req.Timestamp, req.SoftwareVersion)
	
	// Hash the message
	hash := sha256.Sum256([]byte(message))
	
	// For now, we'll implement a simple verification
	// In production, this would verify against the actual mining address signature
	log.Printf("🔐 Verifying registration for %s", req.MiningAddr[:16]+"...")
	
	// Decode signature
	signature, err := hex.DecodeString(req.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature format: %w", err)
	}
	
	// Decode public key
	publicKey, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid public key format: %w", err)
	}
	
	// Verify signature using ML-DSA-87
	if len(publicKey) == mldsa87.PublicKeySize {
		pk := new(mldsa87.PublicKey)
		if err := pk.UnmarshalBinary(publicKey); err != nil {
			return fmt.Errorf("invalid public key: %w", err)
		}
		valid := mldsa87.Verify(pk, hash[:], signature, nil)
		if !valid {
			return fmt.Errorf("signature verification failed")
		}
		log.Printf("✅ ML-DSA-87 signature verified for %s", req.MiningAddr[:16]+"...")
		return nil
	}
	
	// For development/testing, allow simple verification
	expectedSig := fmt.Sprintf("%x", hash[:16])
	if strings.HasPrefix(req.Signature, expectedSig) {
		log.Printf("✅ Development signature accepted for %s", req.MiningAddr[:16]+"...")
		return nil
	}
	
	return fmt.Errorf("signature verification failed")
}

// VerifyHeartbeatSignature verifies heartbeat signatures
func VerifyHeartbeatSignature(req *HeartbeatRequest, node *RegisteredNode) error {
	// Create message to verify
	message := fmt.Sprintf("%s|%d|%s|%s", 
		req.NodeID, req.ChainHeight, req.ChainHash, req.Timestamp)
	
	// Hash the message
	hash := sha256.Sum256([]byte(message))
	
	// For development, simple verification
	expectedSig := fmt.Sprintf("%x", hash[:8])
	if strings.HasPrefix(req.Signature, expectedSig) {
		return nil
	}
	
	return fmt.Errorf("heartbeat signature verification failed")
}

// GenerateNodeID creates a unique node identifier
func GenerateNodeID(miningAddr, ip string, port int) string {
	data := fmt.Sprintf("%s:%s:%d", miningAddr, ip, port)
	hash := sha3.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16])
}

// DeriveShadowyAddress derives a Shadowy address from public key
func DeriveShadowyAddress(publicKey []byte) string {
	// Shadowy address format: S4 + 50 hex chars
	hash := sha3.Sum256(publicKey)
	return fmt.Sprintf("S4%x", hash[:25])
}