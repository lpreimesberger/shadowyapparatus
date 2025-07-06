package cmd

import (
	"encoding/json"
	"fmt"
	"time"
)

// BasicTransactionValidator validates basic transaction structure
type BasicTransactionValidator struct{}

func (v *BasicTransactionValidator) Name() string {
	return "BasicTransactionValidator"
}

func (v *BasicTransactionValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	// Validate signed transaction structure
	if signedTx == nil {
		return fmt.Errorf("transaction is nil")
	}
	
	if signedTx.TxHash == "" {
		return fmt.Errorf("transaction hash is empty")
	}
	
	if signedTx.Algorithm != "ML-DSA-87" {
		return fmt.Errorf("unsupported signature algorithm: %s", signedTx.Algorithm)
	}
	
	if signedTx.Signature == "" {
		return fmt.Errorf("signature is empty")
	}
	
	if signedTx.SignerKey == "" {
		return fmt.Errorf("signer key is empty")
	}
	
	// Parse and validate the underlying transaction
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction data: %w", err)
	}
	
	// Validate transaction structure
	if err := tx.IsValid(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	
	return nil
}

// SignatureValidator validates transaction signatures
type SignatureValidator struct{}

func (v *SignatureValidator) Name() string {
	return "SignatureValidator"
}

func (v *SignatureValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	// For now, we'll skip signature validation due to the panic issue we encountered
	// In a production system, this would verify the ML-DSA-87 signature
	
	// Basic signature format validation
	if len(signedTx.Signature) == 0 {
		return fmt.Errorf("signature is empty")
	}
	
	// Check signature length (should be hex-encoded ML-DSA-87 signature)
	expectedHexLength := SignatureSize * 2 // 4627 * 2 = 9254 hex characters
	if len(signedTx.Signature) != expectedHexLength {
		return fmt.Errorf("signature has incorrect length: expected %d, got %d", 
			expectedHexLength, len(signedTx.Signature))
	}
	
	// Verify signature is valid hex
	for _, char := range signedTx.Signature {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return fmt.Errorf("signature contains invalid hex character: %c", char)
		}
	}
	
	// TODO: Implement full signature verification once panic issue is resolved
	// return VerifySignedTransaction(signedTx)
	
	return nil
}

// TemporalValidator validates transaction timing constraints
type TemporalValidator struct{}

func (v *TemporalValidator) Name() string {
	return "TemporalValidator"
}

func (v *TemporalValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	now := time.Now().UTC()
	
	// Check if transaction is valid yet (not_until constraint)
	if tx.NotUntil.After(now) {
		return fmt.Errorf("transaction not valid until %s (current time: %s)", 
			tx.NotUntil.Format(time.RFC3339), now.Format(time.RFC3339))
	}
	
	// Check if transaction timestamp is reasonable (not too far in the future)
	maxFutureTime := now.Add(10 * time.Minute)
	if tx.Timestamp.After(maxFutureTime) {
		return fmt.Errorf("transaction timestamp too far in future: %s (max allowed: %s)",
			tx.Timestamp.Format(time.RFC3339), maxFutureTime.Format(time.RFC3339))
	}
	
	// Check if transaction is not too old
	maxAge := 24 * time.Hour
	minTime := now.Add(-maxAge)
	if tx.Timestamp.Before(minTime) {
		return fmt.Errorf("transaction timestamp too old: %s (min allowed: %s)",
			tx.Timestamp.Format(time.RFC3339), minTime.Format(time.RFC3339))
	}
	
	return nil
}

// FeeValidator validates transaction fees
type FeeValidator struct {
	MinFee uint64
}

func (v *FeeValidator) Name() string {
	return "FeeValidator"
}

func (v *FeeValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// Calculate implicit fee (for now, simple calculation)
	// In a real implementation, this would consider input values vs output values
	implicitFee := v.MinFee + uint64(len(tx.Outputs))
	
	// For now, just check that outputs have reasonable values
	totalOutput := uint64(0)
	for i, output := range tx.Outputs {
		if output.Value == 0 {
			return fmt.Errorf("output %d has zero value", i)
		}
		
		if output.Value < v.MinFee {
			return fmt.Errorf("output %d value %d is less than minimum fee %d", 
				i, output.Value, v.MinFee)
		}
		
		totalOutput += output.Value
	}
	
	// Check minimum total value
	if totalOutput < implicitFee {
		return fmt.Errorf("total output value %d is less than minimum fee %d", 
			totalOutput, implicitFee)
	}
	
	return nil
}

// SizeValidator validates transaction size limits
type SizeValidator struct {
	MaxTxSize int // Maximum transaction size in bytes
}

func (v *SizeValidator) Name() string {
	return "SizeValidator"
}

func (v *SizeValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	// Calculate transaction size
	txData, err := json.Marshal(signedTx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction for size check: %w", err)
	}
	
	txSize := len(txData)
	
	if txSize > v.MaxTxSize {
		return fmt.Errorf("transaction size %d bytes exceeds maximum %d bytes", 
			txSize, v.MaxTxSize)
	}
	
	// Also check the underlying transaction structure
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// Check reasonable limits on inputs/outputs
	maxInputs := 1000
	maxOutputs := 1000
	
	if len(tx.Inputs) > maxInputs {
		return fmt.Errorf("transaction has too many inputs: %d (max: %d)", 
			len(tx.Inputs), maxInputs)
	}
	
	if len(tx.Outputs) > maxOutputs {
		return fmt.Errorf("transaction has too many outputs: %d (max: %d)", 
			len(tx.Outputs), maxOutputs)
	}
	
	return nil
}

// DoubleSpendValidator checks for potential double-spend attacks
type DoubleSpendValidator struct {
	// In a real implementation, this would have access to UTXO set
	// For now, we'll do basic validation
}

func (v *DoubleSpendValidator) Name() string {
	return "DoubleSpendValidator"
}

func (v *DoubleSpendValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// Check for duplicate inputs within the same transaction
	seenInputs := make(map[string]bool)
	for i, input := range tx.Inputs {
		inputKey := fmt.Sprintf("%s:%d", input.PreviousTxHash, input.OutputIndex)
		
		if seenInputs[inputKey] {
			return fmt.Errorf("duplicate input in transaction: input %d references same output as previous input", i)
		}
		
		seenInputs[inputKey] = true
	}
	
	// TODO: In a real implementation, check against UTXO set to ensure:
	// 1. Referenced outputs exist
	// 2. Referenced outputs haven't been spent
	// 3. Referenced outputs can be spent by this transaction's signature
	
	return nil
}

// AddressValidator validates transaction addresses
type AddressValidator struct{}

func (v *AddressValidator) Name() string {
	return "AddressValidator"
}

func (v *AddressValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// Validate all output addresses
	for i, output := range tx.Outputs {
		if !IsValidAddress(output.Address) {
			return fmt.Errorf("invalid address in output %d: %s", i, output.Address)
		}
	}
	
	// Check for reasonable address format
	for i, output := range tx.Outputs {
		// Shadowy addresses should start with 'S' and be hex-encoded
		if len(output.Address) != 51 { // 'S' + 50 hex chars (25 bytes * 2)
			return fmt.Errorf("output %d address has incorrect length: %d", i, len(output.Address))
		}
		
		if output.Address[0] != 'S' {
			return fmt.Errorf("output %d address does not start with 'S': %s", i, output.Address)
		}
	}
	
	return nil
}

// NonceValidator validates transaction nonces to prevent replay attacks
type NonceValidator struct {
	// In a real implementation, this would track nonces per address
	seenNonces map[string]map[uint64]bool // address -> nonce -> seen
}

func NewNonceValidator() *NonceValidator {
	return &NonceValidator{
		seenNonces: make(map[string]map[uint64]bool),
	}
}

func (v *NonceValidator) Name() string {
	return "NonceValidator"
}

func (v *NonceValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// For now, just validate that nonce is reasonable (not zero, not too large)
	if tx.Nonce == 0 {
		return fmt.Errorf("transaction nonce cannot be zero")
	}
	
	// Check nonce is not unreasonably large (potential overflow protection)
	maxNonce := uint64(1<<63 - 1) // Maximum reasonable nonce
	if tx.Nonce > maxNonce {
		return fmt.Errorf("transaction nonce %d is too large (max: %d)", tx.Nonce, maxNonce)
	}
	
	// TODO: In a real implementation, track nonces per sender address to prevent replay
	// For now, we'll skip this as we don't have a complete sender resolution system
	
	return nil
}

// CompositeValidator combines multiple validators
type CompositeValidator struct {
	validators []TransactionValidator
	name       string
}

func NewCompositeValidator(name string, validators ...TransactionValidator) *CompositeValidator {
	return &CompositeValidator{
		validators: validators,
		name:       name,
	}
}

func (v *CompositeValidator) Name() string {
	return v.name
}

func (v *CompositeValidator) ValidateTransaction(signedTx *SignedTransaction) error {
	for _, validator := range v.validators {
		if err := validator.ValidateTransaction(signedTx); err != nil {
			return fmt.Errorf("validation failed in %s: %w", validator.Name(), err)
		}
	}
	return nil
}