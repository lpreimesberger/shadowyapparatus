package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"golang.org/x/crypto/sha3"
)

// Transaction represents a Shadowy blockchain transaction
type Transaction struct {
	Version   int               `json:"version"`
	Inputs    []TransactionInput  `json:"inputs"`
	Outputs   []TransactionOutput `json:"outputs"`
	NotUntil  time.Time          `json:"not_until"` // ISO timestamp when transaction becomes valid
	Timestamp time.Time          `json:"timestamp"` // When transaction was created
	Nonce     uint64             `json:"nonce"`     // Prevent replay attacks
}

// TransactionInput represents a reference to a previous transaction output
type TransactionInput struct {
	PreviousTxHash string `json:"previous_tx_hash"` // Hash of previous transaction
	OutputIndex    uint32 `json:"output_index"`     // Index of output in previous transaction
	ScriptSig      string `json:"script_sig"`       // Signature script (for Bitcoin compatibility)
	Sequence       uint32 `json:"sequence"`         // Sequence number (for Bitcoin compatibility)
}

// TransactionOutput represents a payment to an address
type TransactionOutput struct {
	Value        uint64 `json:"value"`         // Amount in smallest unit (satoshi equivalent)
	ScriptPubKey string `json:"script_pubkey"` // Public key script (for Bitcoin compatibility)
	Address      string `json:"address"`       // Shadowy address to pay to
}

// JOSEHeader provides JOSE-style header information
type JOSEHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// SignedTransaction represents a ML-DSA signed transaction
type SignedTransaction struct {
	Transaction json.RawMessage `json:"transaction"` // Original transaction data
	Signature   string          `json:"signature"`   // ML-DSA-87 signature (hex)
	TxHash      string          `json:"tx_hash"`     // Transaction hash for reference
	SignerKey   string          `json:"signer_key"`  // Public key of signer (hex)
	Algorithm   string          `json:"algorithm"`   // Signature algorithm used
	Header      JOSEHeader      `json:"header"`      // JOSE-style header for compatibility
}

// TransactionSummary provides a human-readable view of transaction
type TransactionSummary struct {
	Hash      string    `json:"hash"`
	Version   int       `json:"version"`
	InputCount  int     `json:"input_count"`
	OutputCount int     `json:"output_count"`
	TotalValue uint64   `json:"total_value"`
	NotUntil  time.Time `json:"not_until"`
	Timestamp time.Time `json:"timestamp"`
	Nonce     uint64    `json:"nonce"`
	Valid     bool      `json:"valid"`
	Signer    string    `json:"signer"`
}

// NewTransaction creates a new transaction with defaults
func NewTransaction() *Transaction {
	return &Transaction{
		Version:   1,
		Inputs:    []TransactionInput{},
		Outputs:   []TransactionOutput{},
		NotUntil:  time.Now().UTC(),
		Timestamp: time.Now().UTC(),
		Nonce:     uint64(time.Now().UnixNano()),
	}
}

// AddInput adds an input to the transaction
func (tx *Transaction) AddInput(prevTxHash string, outputIndex uint32) {
	input := TransactionInput{
		PreviousTxHash: prevTxHash,
		OutputIndex:    outputIndex,
		ScriptSig:      "", // Will be filled during signing
		Sequence:       0xFFFFFFFF, // Bitcoin default
	}
	tx.Inputs = append(tx.Inputs, input)
}

// AddOutput adds an output to the transaction
func (tx *Transaction) AddOutput(address string, value uint64) {
	output := TransactionOutput{
		Value:        value,
		ScriptPubKey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address),
		Address:      address,
	}
	tx.Outputs = append(tx.Outputs, output)
}

// SetNotUntil sets when the transaction becomes valid
func (tx *Transaction) SetNotUntil(notUntil time.Time) {
	tx.NotUntil = notUntil.UTC()
}

// Hash calculates the transaction hash
func (tx *Transaction) Hash() (string, error) {
	// Create a copy without any signature data for hashing
	hashTx := *tx
	for i := range hashTx.Inputs {
		hashTx.Inputs[i].ScriptSig = ""
	}
	
	data, err := json.Marshal(hashTx)
	if err != nil {
		return "", fmt.Errorf("failed to marshal transaction for hashing: %w", err)
	}
	
	// Use SHAKE256 for quantum resistance
	hash := make([]byte, 32)
	shake := sha3.NewShake256()
	shake.Write(data)
	shake.Read(hash)
	
	return hex.EncodeToString(hash), nil
}

// TotalInputValue calculates total value of inputs (requires UTXO lookup)
func (tx *Transaction) TotalInputValue() uint64 {
	// In a real implementation, this would look up UTXOs
	// For now, return 0 as we don't have a UTXO set
	return 0
}

// TotalOutputValue calculates total value of outputs
func (tx *Transaction) TotalOutputValue() uint64 {
	total := uint64(0)
	for _, output := range tx.Outputs {
		total += output.Value
	}
	return total
}

// IsValid performs basic transaction validation
func (tx *Transaction) IsValid() error {
	if tx.Version <= 0 {
		return fmt.Errorf("invalid version: %d", tx.Version)
	}
	
	if len(tx.Inputs) == 0 && tx.Version > 1 {
		return fmt.Errorf("transaction must have at least one input (unless coinbase)")
	}
	
	if len(tx.Outputs) == 0 {
		return fmt.Errorf("transaction must have at least one output")
	}
	
	if tx.NotUntil.After(time.Now().UTC()) {
		return fmt.Errorf("transaction not valid until %s", tx.NotUntil.Format(time.RFC3339))
	}
	
	// Validate outputs
	for i, output := range tx.Outputs {
		if output.Value == 0 {
			return fmt.Errorf("output %d has zero value", i)
		}
		
		if !IsValidAddress(output.Address) {
			return fmt.Errorf("output %d has invalid address: %s", i, output.Address)
		}
	}
	
	// Validate inputs
	for i, input := range tx.Inputs {
		if input.PreviousTxHash == "" && tx.Version > 1 {
			return fmt.Errorf("input %d has empty previous transaction hash", i)
		}
		
		if len(input.PreviousTxHash) != 64 && input.PreviousTxHash != "" {
			return fmt.Errorf("input %d has invalid previous transaction hash length", i)
		}
	}
	
	return nil
}

// Summary creates a human-readable summary
func (tx *Transaction) Summary() TransactionSummary {
	hash, _ := tx.Hash()
	
	return TransactionSummary{
		Hash:        hash,
		Version:     tx.Version,
		InputCount:  len(tx.Inputs),
		OutputCount: len(tx.Outputs),
		TotalValue:  tx.TotalOutputValue(),
		NotUntil:    tx.NotUntil,
		Timestamp:   tx.Timestamp,
		Nonce:       tx.Nonce,
		Valid:       tx.IsValid() == nil,
		Signer:      "", // Will be filled by signing process
	}
}

// SignTransaction signs a transaction using ML-DSA-87
func SignTransaction(tx *Transaction, keyPair *KeyPair) (*SignedTransaction, error) {
	// Marshal transaction for signing
	payload, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}
	
	// Sign the transaction payload
	signature, err := keyPair.Sign(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	
	// Get transaction hash
	txHash, err := tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
	}
	
	// Create signed transaction
	signedTx := &SignedTransaction{
		Transaction: json.RawMessage(payload),
		Signature:   hex.EncodeToString(signature),
		TxHash:      txHash,
		SignerKey:   keyPair.PublicKeyHex(),
		Algorithm:   "ML-DSA-87",
		Header: JOSEHeader{
			Algorithm: "ML-DSA-87",
			Type:      "shadowy-tx",
		},
	}
	
	return signedTx, nil
}

// VerifySignedTransaction verifies a ML-DSA signed transaction
func VerifySignedTransaction(signedTx *SignedTransaction) (*Transaction, error) {
	// Decode signature
	signature, err := hex.DecodeString(signedTx.Signature)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}
	
	// Decode public key
	pubKeyBytes, err := hex.DecodeString(signedTx.SignerKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signer public key: %w", err)
	}
	
	// Verify signature against transaction data
	if !VerifySignature(pubKeyBytes, signedTx.Transaction, signature) {
		return nil, fmt.Errorf("signature verification failed")
	}
	
	// Unmarshal transaction
	var tx Transaction
	if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	
	// Verify transaction hash matches
	calculatedHash, err := tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate transaction hash: %w", err)
	}
	
	if calculatedHash != signedTx.TxHash {
		return nil, fmt.Errorf("transaction hash mismatch")
	}
	
	// Validate transaction
	if err := tx.IsValid(); err != nil {
		return nil, fmt.Errorf("invalid transaction: %w", err)
	}
	
	return &tx, nil
}

// SignTransactionWithWallet signs a transaction using a wallet
func SignTransactionWithWallet(tx *Transaction, wallet *WalletFile) (*SignedTransaction, error) {
	// Create transaction JSON
	txData, err := json.Marshal(tx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal transaction: %w", err)
	}
	
	// Generate transaction hash
	hash, err := tx.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to generate transaction hash: %w", err)
	}
	
	// Parse wallet key for signing
	keyPair, err := parseWalletKey(wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to parse wallet key: %w", err)
	}
	
	// Sign the transaction data
	signature, err := keyPair.Sign(txData)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	
	// Create signed transaction
	signedTx := &SignedTransaction{
		Transaction: txData,
		Signature:   hex.EncodeToString(signature),
		TxHash:      hash,
		SignerKey:   keyPair.PublicKeyHex(),
		Algorithm:   "ML-DSA-87",
		Header: JOSEHeader{
			Algorithm: "ML-DSA-87",
			Type:      "shadowy-tx",
		},
	}
	
	return signedTx, nil
}

// parseWalletKey converts wallet to KeyPair for signing
func parseWalletKey(wallet *WalletFile) (*KeyPair, error) {
	// Validate wallet input
	if wallet == nil {
		return nil, fmt.Errorf("wallet is nil")
	}
	if wallet.PrivateKey == "" {
		return nil, fmt.Errorf("wallet private key is empty")
	}
	if wallet.PublicKey == "" {
		return nil, fmt.Errorf("wallet public key is empty")
	}
	if wallet.Address == "" {
		return nil, fmt.Errorf("wallet address is empty")
	}
	
	// Try to decode as seed first (new format), then fall back to full private key (old format)
	privKeyBytes, err := hex.DecodeString(wallet.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}
	
	// Check if this is a seed (32 bytes) or full private key (4896 bytes)
	if len(privKeyBytes) == SeedSize {
		// New format: private key is actually a seed
		var seed [SeedSize]byte
		copy(seed[:], privKeyBytes)
		
		// Validate seed is not all zeros
		allZeros := true
		for _, b := range seed {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			return nil, fmt.Errorf("wallet seed contains only zeros (invalid key)")
		}
		
		// Generate key pair from seed
		return NewKeyPairFromSeed(seed)
		
	} else if len(privKeyBytes) == PrivateKeySize {
		// Old format: full private key stored (legacy wallets)
		// We need to extract the seed from the full private key
		// For now, we'll use a hash of the first 32 bytes as seed (not ideal but maintains compatibility)
		var seed [SeedSize]byte
		copy(seed[:], privKeyBytes[:SeedSize])
		
		// Validate seed is not all zeros
		allZeros := true
		for _, b := range seed {
			if b != 0 {
				allZeros = false
				break
			}
		}
		if allZeros {
			return nil, fmt.Errorf("extracted seed from legacy wallet contains only zeros")
		}
		
		// Try to generate key pair from extracted seed
		kp, err := NewKeyPairFromSeed(seed)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct key from legacy wallet: %w", err)
		}
		
		// Verify the public key matches (to ensure we reconstructed correctly)
		if kp.PublicKeyHex() != wallet.PublicKey {
			return nil, fmt.Errorf("legacy wallet key reconstruction failed: public key mismatch")
		}
		
		return kp, nil
		
	} else {
		return nil, fmt.Errorf("invalid private key size: expected %d (seed) or %d (full key), got %d", 
			SeedSize, PrivateKeySize, len(privKeyBytes))
	}
}

