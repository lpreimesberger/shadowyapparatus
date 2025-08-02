package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/sha3"
)

// Transaction represents a Shadowy blockchain transaction
type Transaction struct {
	Version   int                 `json:"version"`
	Inputs    []TransactionInput  `json:"inputs"`
	Outputs   []TransactionOutput `json:"outputs"`
	TokenOps  []TokenOperation    `json:"token_ops,omitempty"` // Token operations (optional)
	NotUntil  time.Time          `json:"not_until"`           // ISO timestamp when transaction becomes valid
	Timestamp time.Time          `json:"timestamp"`           // When transaction was created
	Nonce     uint64             `json:"nonce"`               // Prevent replay attacks
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

// TokenOpType represents the type of token operation
type TokenOpType int

const (
	TOKEN_CREATE TokenOpType = iota // Create a new token
	TOKEN_TRANSFER                  // Transfer tokens between addresses
	TOKEN_MELT                      // Melt tokens back to Shadow
	TRADE_OFFER                     // Create a trade offer (locks asset in NFT)
	TRADE_EXECUTE                   // Execute/accept a trade offer
	SYNDICATE_JOIN                  // Join a mining syndicate (creates membership NFT)
	POOL_CREATE                     // Create a new liquidity pool NFT
)

// String returns the string representation of TokenOpType
func (t TokenOpType) String() string {
	switch t {
	case TOKEN_CREATE:
		return "CREATE"
	case TOKEN_TRANSFER:
		return "TRANSFER"
	case TOKEN_MELT:
		return "MELT"
	case TRADE_OFFER:
		return "TRADE_OFFER"
	case TRADE_EXECUTE:
		return "TRADE_EXECUTE"
	case SYNDICATE_JOIN:
		return "SYNDICATE_JOIN"
	case POOL_CREATE:
		return "POOL_CREATE"
	default:
		return "UNKNOWN"
	}
}

// TokenOperation represents a token-related operation in a transaction
type TokenOperation struct {
	Type     TokenOpType   `json:"type"`                // Operation type
	TokenID  string        `json:"token_id"`            // Unique token identifier (hex)
	Amount   uint64        `json:"amount"`              // Token amount (with decimals applied)
	From     string        `json:"from,omitempty"`      // Source address (for TRANSFER/MELT)
	To       string        `json:"to,omitempty"`        // Destination address (for TRANSFER)
	Metadata *TokenMetadata `json:"metadata,omitempty"` // Token metadata (for CREATE only)
}

// TokenMetadata contains the immutable properties of a token
type TokenMetadata struct {
	Name         string `json:"name"`          // Human readable name (e.g. "Steve Coin")
	Ticker       string `json:"ticker"`        // Short symbol (e.g. "STEVE")
	TotalSupply  uint64 `json:"total_supply"`  // Fixed total supply (with decimals applied)
	Decimals     uint8  `json:"decimals"`      // Number of decimal places (0-18)
	LockAmount   uint64 `json:"lock_amount"`   // Shadow satoshi locked per token unit
	Creator      string          `json:"creator"`       // Address of token creator
	CreationTime int64           `json:"creation_time"` // Unix timestamp of creation
	URI          string          `json:"uri,omitempty"` // Optional URI for metadata/NFT content (max 128 chars)
	TradeOffer   *TradeOfferData `json:"trade_offer,omitempty"` // Trade offer data for marketplace NFTs
	Syndicate    *SyndicateData  `json:"syndicate,omitempty"`   // Syndicate membership data for mining pool NFTs
	LiquidityPool *LiquidityPoolData `json:"liquidity_pool,omitempty"` // Liquidity pool data
}

// TradeOfferData contains the details of a trade offer locked in an NFT
type TradeOfferData struct {
	LockedTokenID    string `json:"locked_token_id"`    // ID of token being sold
	LockedAmount     uint64 `json:"locked_amount"`      // Amount of token being sold
	AskingPrice      uint64 `json:"asking_price"`       // Price in SHADOW satoshi
	AskingTokenID    string `json:"asking_token_id,omitempty"` // If asking for tokens instead of SHADOW
	Seller           string `json:"seller"`             // Address of seller
	ExpirationTime   int64  `json:"expiration_time"`    // Unix timestamp when offer expires
	CreationTime     int64  `json:"creation_time"`      // Unix timestamp when offer was created
}

// SyndicateType represents the Four Guardian syndicates
type SyndicateType int

const (
	SyndicateSeiryu SyndicateType = iota // Azure Dragon (East) - 青龍
	SyndicateByakko                      // White Tiger (West) - 白虎
	SyndicateSuzaku                      // Vermillion Bird (South) - 朱雀
	SyndicateGenbu                       // Black Tortoise (North) - 玄武
	SyndicateAuto                        // Automatic assignment to lowest-capacity syndicate
)

// String returns the name of the syndicate
func (s SyndicateType) String() string {
	switch s {
	case SyndicateSeiryu:
		return "Seiryu"
	case SyndicateByakko:
		return "Byakko"
	case SyndicateSuzaku:
		return "Suzaku"
	case SyndicateGenbu:
		return "Genbu"
	case SyndicateAuto:
		return "Auto"
	default:
		return "Unknown"
	}
}

// Description returns the full description of the syndicate
func (s SyndicateType) Description() string {
	switch s {
	case SyndicateSeiryu:
		return "Azure Dragon of the East - 青龍"
	case SyndicateByakko:
		return "White Tiger of the West - 白虎"
	case SyndicateSuzaku:
		return "Vermillion Bird of the South - 朱雀"
	case SyndicateGenbu:
		return "Black Tortoise of the North - 玄武"
	case SyndicateAuto:
		return "Automatic Assignment to Lowest-Capacity Syndicate"
	default:
		return "Unknown Syndicate"
	}
}

// SyndicateData contains the details of a syndicate membership NFT
type SyndicateData struct {
	Syndicate        SyndicateType `json:"syndicate"`         // Which of the Four Guardians
	MinerAddress     string        `json:"miner_address"`     // Address of the miner joining
	ReportedCapacity uint64        `json:"reported_capacity"` // Self-reported storage capacity in bytes
	JoinTime         int64         `json:"join_time"`         // Unix timestamp when NFT was minted
	ExpirationTime   int64         `json:"expiration_time"`   // Unix timestamp when NFT expires (8 days max)
	RenewalCount     uint32        `json:"renewal_count"`     // How many times this membership has been renewed
}

// LiquidityPoolData contains the details of a liquidity pool NFT
type LiquidityPoolData struct {
	TokenA         string `json:"token_a"`          // First token ID in the pair (or "SHADOW")
	TokenB         string `json:"token_b"`          // Second token ID in the pair (or "SHADOW") 
	InitialRatioA  uint64 `json:"initial_ratio_a"`  // Initial amount of token A (defines k constant)
	InitialRatioB  uint64 `json:"initial_ratio_b"`  // Initial amount of token B (defines k constant)
	FeeRate        uint64 `json:"fee_rate"`         // Fee rate in basis points (e.g., 30 = 0.3%)
	LAddress       string `json:"l_address"`        // Pool's L-address (computed deterministically)
	ShareTokenID   string `json:"share_token_id"`   // Pool share token ID (owned by L-address)
	Creator        string `json:"creator"`          // Pool creator address
	CreationTime   int64  `json:"creation_time"`    // Unix timestamp of creation
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
		TokenOps:  []TokenOperation{},
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

// AddTokenOperation adds a token operation to the transaction
func (tx *Transaction) AddTokenOperation(tokenOp TokenOperation) {
	tx.TokenOps = append(tx.TokenOps, tokenOp)
}

// AddTokenCreate adds a token creation operation
func (tx *Transaction) AddTokenCreate(name, ticker string, totalSupply uint64, decimals uint8, lockAmount uint64, creator, uri string) {
	tokenID := generateTokenID(name, ticker, creator, tx.Timestamp)
	metadata := &TokenMetadata{
		Name:         name,
		Ticker:       ticker,
		TotalSupply:  totalSupply,
		Decimals:     decimals,
		LockAmount:   lockAmount,
		Creator:      creator,
		CreationTime: tx.Timestamp.Unix(),
		URI:          uri,
	}
	
	tokenOp := TokenOperation{
		Type:     TOKEN_CREATE,
		TokenID:  tokenID,
		Amount:   totalSupply,
		To:       creator, // Initial supply goes to creator
		Metadata: metadata,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddTokenTransfer adds a token transfer operation
func (tx *Transaction) AddTokenTransfer(tokenID string, amount uint64, from, to string) {
	tokenOp := TokenOperation{
		Type:    TOKEN_TRANSFER,
		TokenID: tokenID,
		Amount:  amount,
		From:    from,
		To:      to,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddTokenMelt adds a token melting operation
func (tx *Transaction) AddTokenMelt(tokenID string, amount uint64, from string) {
	tokenOp := TokenOperation{
		Type:    TOKEN_MELT,
		TokenID: tokenID,
		Amount:  amount,
		From:    from,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddTradeOffer creates a trade offer NFT that locks the specified asset
func (tx *Transaction) AddTradeOffer(lockedTokenID string, lockedAmount uint64, askingPrice uint64, askingTokenID, seller string, expirationHours int) {
	// Generate unique trade offer NFT ID
	tradeOfferID := generateTokenID("Trade Offer", "TRADE", seller, tx.Timestamp)
	
	// Create trade offer data
	tradeOffer := &TradeOfferData{
		LockedTokenID:  lockedTokenID,
		LockedAmount:   lockedAmount,
		AskingPrice:    askingPrice,
		AskingTokenID:  askingTokenID,
		Seller:         seller,
		ExpirationTime: tx.Timestamp.Unix() + int64(expirationHours*3600),
		CreationTime:   tx.Timestamp.Unix(),
	}
	
	// Create NFT metadata for the trade offer
	metadata := &TokenMetadata{
		Name:         "Trade Offer",
		Ticker:       "TRADE",
		TotalSupply:  1, // NFT
		Decimals:     0, // NFT
		LockAmount:   10000000, // 0.1 SHADOW base fee
		Creator:      seller,
		CreationTime: tx.Timestamp.Unix(),
		TradeOffer:   tradeOffer,
	}
	
	tokenOp := TokenOperation{
		Type:     TRADE_OFFER,
		TokenID:  tradeOfferID,
		Amount:   1, // NFT amount
		From:     seller, // Seller provides the locked asset
		To:       seller, // Trade offer NFT goes to seller initially
		Metadata: metadata,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddTradeExecute executes a trade offer by accepting it
func (tx *Transaction) AddTradeExecute(tradeOfferNFTID string, buyer string) {
	tokenOp := TokenOperation{
		Type:    TRADE_EXECUTE,
		TokenID: tradeOfferNFTID,
		Amount:  1, // NFT amount
		From:    buyer, // Buyer pays for the trade
		To:      "",    // Will be filled by execution logic
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddSyndicateJoin creates a syndicate membership NFT for mining pool participation
func (tx *Transaction) AddSyndicateJoin(syndicate SyndicateType, minerAddress string, reportedCapacity uint64, expirationDays int) {
	// Generate unique syndicate membership NFT ID
	syndicateID := generateTokenID("Syndicate "+syndicate.String(), "SYN_"+syndicate.String(), minerAddress, tx.Timestamp)
	
	// Create syndicate membership data
	syndicateData := &SyndicateData{
		Syndicate:        syndicate,
		MinerAddress:     minerAddress,
		ReportedCapacity: reportedCapacity,
		JoinTime:         tx.Timestamp.Unix(),
		ExpirationTime:   tx.Timestamp.Unix() + int64(expirationDays*24*3600), // Convert days to seconds
		RenewalCount:     0, // Initial join
	}
	
	// Create NFT metadata for the syndicate membership
	metadata := &TokenMetadata{
		Name:         "Syndicate " + syndicate.String() + " Membership",
		Ticker:       "SYN_" + syndicate.String(),
		TotalSupply:  1, // NFT
		Decimals:     0, // NFT
		LockAmount:   10000000, // 0.1 SHADOW base fee (same as trade offers)
		Creator:      minerAddress,
		CreationTime: tx.Timestamp.Unix(),
		URI:          "", // Could add syndicate badge URI later
		Syndicate:    syndicateData,
	}
	
	tokenOp := TokenOperation{
		Type:     SYNDICATE_JOIN,
		TokenID:  syndicateID,
		Amount:   1, // NFT amount
		From:     minerAddress, // Miner pays the fee
		To:       minerAddress, // Syndicate NFT goes to miner
		Metadata: metadata,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// AddPoolCreate creates a new liquidity pool NFT with L-address
func (tx *Transaction) AddPoolCreate(tokenA, tokenB string, initialRatioA, initialRatioB uint64, feeRate uint64, creator, name, ticker string) {
	// Generate pool NFT ID
	poolNFTID := generateTokenID(name, ticker, creator, tx.Timestamp)
	
	// Generate share token ID for this pool (high melt value for maximum shares)
	shareTokenID := generateTokenID(name+" Shares", ticker+"_SHARE", creator, tx.Timestamp) 
	
	// Create liquidity pool data (L-address will be computed after transaction creation)
	poolData := &LiquidityPoolData{
		TokenA:         tokenA,
		TokenB:         tokenB,
		InitialRatioA:  initialRatioA,
		InitialRatioB:  initialRatioB,
		FeeRate:        feeRate,
		LAddress:       "", // Will be computed deterministically from this transaction
		ShareTokenID:   shareTokenID,
		Creator:        creator,
		CreationTime:   tx.Timestamp.Unix(),
	}
	
	// Create NFT metadata for the pool
	metadata := &TokenMetadata{
		Name:         name,
		Ticker:       ticker,
		TotalSupply:  1, // Pool NFT (single instance)
		Decimals:     0, // NFT
		LockAmount:   500000000, // 5 SHADOW pool creation fee (high cost for permanent infrastructure)
		Creator:      creator,
		CreationTime: tx.Timestamp.Unix(),
		LiquidityPool: poolData,
	}
	
	tokenOp := TokenOperation{
		Type:     POOL_CREATE,
		TokenID:  poolNFTID,
		Amount:   1, // Pool NFT amount
		From:     creator,
		To:       creator, // Pool NFT goes to creator
		Metadata: metadata,
	}
	
	tx.AddTokenOperation(tokenOp)
}

// HasTokenOperations returns true if the transaction contains token operations
func (tx *Transaction) HasTokenOperations() bool {
	return len(tx.TokenOps) > 0
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
	
	if len(tx.Outputs) == 0 && len(tx.TokenOps) == 0 {
		return fmt.Errorf("transaction must have at least one output or token operation")
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
	
	// Validate token operations
	if err := tx.ValidateTokenOperations(); err != nil {
		return fmt.Errorf("invalid token operations: %w", err)
	}
	
	return nil
}

// ValidateTokenOperations validates all token operations in the transaction
func (tx *Transaction) ValidateTokenOperations() error {
	if len(tx.TokenOps) == 0 {
		return nil // No token operations to validate
	}
	
	// Track token operations to prevent conflicts
	seenTokenOps := make(map[string][]TokenOpType)
	
	for i, tokenOp := range tx.TokenOps {
		// Basic validation
		if err := validateTokenOperation(tokenOp, i); err != nil {
			return err
		}
		
		// Track operations for conflict detection
		seenTokenOps[tokenOp.TokenID] = append(seenTokenOps[tokenOp.TokenID], tokenOp.Type)
	}
	
	// Check for conflicting operations
	for tokenID, operations := range seenTokenOps {
		if err := validateTokenOperationConflicts(tokenID, operations); err != nil {
			return err
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
		// Reconstruct the key pair directly from the full private key bytes
		var privKey [PrivateKeySize]byte
		copy(privKey[:], privKeyBytes)

		// Reconstruct key pair from the full private key
		kp, err := NewKeyPairFromPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("failed to reconstruct key from legacy wallet private key: %w", err)
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

// generateTokenID creates a unique identifier for a token
func generateTokenID(name, ticker, creator string, timestamp time.Time) string {
	// Create deterministic but unique token ID based on token properties
	data := fmt.Sprintf("%s:%s:%s:%d", name, ticker, creator, timestamp.Unix())
	
	// Use SHAKE256 for quantum resistance
	hash := make([]byte, 32)
	shake := sha3.NewShake256()
	shake.Write([]byte(data))
	shake.Read(hash)
	
	return hex.EncodeToString(hash)
}

// generateLAddress creates an L-address from a pool creation transaction
func generateLAddress(txHash string) string {
	// L-addresses start with 'L' instead of 'S' and are derived from the pool creation transaction
	// This ensures the L-address is deterministic and tied to the pool creation
	
	// Use SHAKE256 for quantum resistance
	hash := make([]byte, 32)
	shake := sha3.NewShake256()
	shake.Write([]byte("L-ADDRESS:" + txHash))
	shake.Read(hash)
	
	// Convert to base58 or similar encoding and prefix with 'L'
	// For now, use hex encoding with L prefix
	return "L" + hex.EncodeToString(hash)[:40] // 40 chars to match typical address length
}

// validateTokenOperation validates a single token operation
func validateTokenOperation(tokenOp TokenOperation, index int) error {
	// Validate token ID
	if tokenOp.TokenID == "" {
		return fmt.Errorf("token operation %d: token ID cannot be empty", index)
	}
	
	if len(tokenOp.TokenID) != 64 {
		return fmt.Errorf("token operation %d: invalid token ID length (expected 64 hex chars)", index)
	}
	
	// Validate amount
	if tokenOp.Amount == 0 {
		return fmt.Errorf("token operation %d: amount cannot be zero", index)
	}
	
	// Type-specific validation
	switch tokenOp.Type {
	case TOKEN_CREATE:
		return validateTokenCreate(tokenOp, index)
	case TOKEN_TRANSFER:
		return validateTokenTransfer(tokenOp, index)
	case TOKEN_MELT:
		return validateTokenMelt(tokenOp, index)
	case TRADE_OFFER:
		return validateTradeOffer(tokenOp, index)
	case TRADE_EXECUTE:
		return validateTradeExecute(tokenOp, index)
	default:
		return fmt.Errorf("token operation %d: unknown operation type %d", index, tokenOp.Type)
	}
}

// validateTokenCreate validates token creation operation
func validateTokenCreate(tokenOp TokenOperation, index int) error {
	if tokenOp.Metadata == nil {
		return fmt.Errorf("token operation %d: CREATE operation requires metadata", index)
	}
	
	meta := tokenOp.Metadata
	
	// Validate name and ticker
	if meta.Name == "" {
		return fmt.Errorf("token operation %d: token name cannot be empty", index)
	}
	
	if meta.Ticker == "" {
		return fmt.Errorf("token operation %d: token ticker cannot be empty", index)
	}
	
	if len(meta.Name) > 64 {
		return fmt.Errorf("token operation %d: token name too long (max 64 chars)", index)
	}
	
	if len(meta.Ticker) > 16 {
		return fmt.Errorf("token operation %d: token ticker too long (max 16 chars)", index)
	}
	
	// Validate supply and decimals
	if meta.TotalSupply == 0 {
		return fmt.Errorf("token operation %d: total supply cannot be zero", index)
	}
	
	if meta.Decimals > 18 {
		return fmt.Errorf("token operation %d: too many decimal places (max 18)", index)
	}
	
	// Validate lock amount
	if meta.LockAmount == 0 {
		return fmt.Errorf("token operation %d: lock amount cannot be zero", index)
	}
	
	// Validate creator address
	if !IsValidAddress(meta.Creator) {
		return fmt.Errorf("token operation %d: invalid creator address", index)
	}
	
	// Validate URI if provided
	if meta.URI != "" {
		if len(meta.URI) > 128 {
			return fmt.Errorf("token operation %d: URI too long (max 128 chars)", index)
		}
		
		// Basic URI validation - check if it looks like a valid URI
		if !isValidURI(meta.URI) {
			return fmt.Errorf("token operation %d: invalid URI format", index)
		}
	}
	
	// Validate that amount matches total supply
	if tokenOp.Amount != meta.TotalSupply {
		return fmt.Errorf("token operation %d: operation amount must equal total supply for CREATE", index)
	}
	
	// Validate recipient (should be creator for CREATE)
	if tokenOp.To != meta.Creator {
		return fmt.Errorf("token operation %d: initial tokens must go to creator", index)
	}
	
	return nil
}

// validateTokenTransfer validates token transfer operation
func validateTokenTransfer(tokenOp TokenOperation, index int) error {
	// Should not have metadata
	if tokenOp.Metadata != nil {
		return fmt.Errorf("token operation %d: TRANSFER operation should not have metadata", index)
	}
	
	// Validate addresses
	if tokenOp.From == "" {
		return fmt.Errorf("token operation %d: from address cannot be empty", index)
	}
	
	if tokenOp.To == "" {
		return fmt.Errorf("token operation %d: to address cannot be empty", index)
	}
	
	if !IsValidAddress(tokenOp.From) {
		return fmt.Errorf("token operation %d: invalid from address", index)
	}
	
	if !IsValidAddress(tokenOp.To) {
		return fmt.Errorf("token operation %d: invalid to address", index)
	}
	
	if tokenOp.From == tokenOp.To {
		return fmt.Errorf("token operation %d: cannot transfer to same address", index)
	}
	
	return nil
}

// validateTokenMelt validates token melt operation  
func validateTokenMelt(tokenOp TokenOperation, index int) error {
	// Should not have metadata
	if tokenOp.Metadata != nil {
		return fmt.Errorf("token operation %d: MELT operation should not have metadata", index)
	}
	
	// Should not have To address
	if tokenOp.To != "" {
		return fmt.Errorf("token operation %d: MELT operation should not have to address", index)
	}
	
	// Validate from address
	if tokenOp.From == "" {
		return fmt.Errorf("token operation %d: from address cannot be empty", index)
	}
	
	if !IsValidAddress(tokenOp.From) {
		return fmt.Errorf("token operation %d: invalid from address", index)
	}
	
	return nil
}

// validateTradeOffer validates trade offer creation operation
func validateTradeOffer(tokenOp TokenOperation, index int) error {
	if tokenOp.Metadata == nil {
		return fmt.Errorf("token operation %d: TRADE_OFFER operation requires metadata", index)
	}
	
	if tokenOp.Metadata.TradeOffer == nil {
		return fmt.Errorf("token operation %d: TRADE_OFFER operation requires trade offer data", index)
	}
	
	trade := tokenOp.Metadata.TradeOffer
	
	// Validate locked token
	if trade.LockedTokenID == "" {
		return fmt.Errorf("token operation %d: locked token ID cannot be empty", index)
	}
	
	if trade.LockedAmount == 0 {
		return fmt.Errorf("token operation %d: locked amount cannot be zero", index)
	}
	
	// Validate asking price
	if trade.AskingPrice == 0 {
		return fmt.Errorf("token operation %d: asking price cannot be zero", index)
	}
	
	// Validate seller
	if !IsValidAddress(trade.Seller) {
		return fmt.Errorf("token operation %d: invalid seller address", index)
	}
	
	// Validate expiration time
	if trade.ExpirationTime <= trade.CreationTime {
		return fmt.Errorf("token operation %d: expiration time must be after creation time", index)
	}
	
	// Validate that this creates an NFT
	if tokenOp.Metadata.Decimals != 0 || tokenOp.Metadata.TotalSupply != 1 {
		return fmt.Errorf("token operation %d: trade offers must be NFTs (decimals=0, supply=1)", index)
	}
	
	return nil
}

// validateTradeExecute validates trade execution operation
func validateTradeExecute(tokenOp TokenOperation, index int) error {
	// Should not have metadata
	if tokenOp.Metadata != nil {
		return fmt.Errorf("token operation %d: TRADE_EXECUTE operation should not have metadata", index)
	}
	
	// Validate buyer address
	if tokenOp.From == "" {
		return fmt.Errorf("token operation %d: buyer address cannot be empty", index)
	}
	
	if !IsValidAddress(tokenOp.From) {
		return fmt.Errorf("token operation %d: invalid buyer address", index)
	}
	
	// Must be trading exactly 1 NFT
	if tokenOp.Amount != 1 {
		return fmt.Errorf("token operation %d: trade execute must involve exactly 1 NFT", index)
	}
	
	return nil
}

// validateTokenOperationConflicts checks for conflicting operations on the same token
func validateTokenOperationConflicts(tokenID string, operations []TokenOpType) error {
	// Check for multiple CREATE operations
	createCount := 0
	for _, op := range operations {
		if op == TOKEN_CREATE {
			createCount++
		}
	}
	
	if createCount > 1 {
		return fmt.Errorf("token %s: multiple CREATE operations not allowed in same transaction", tokenID)
	}
	
	// If CREATE is present, no other operations allowed
	if createCount > 0 && len(operations) > 1 {
		return fmt.Errorf("token %s: CREATE operation cannot be combined with other operations", tokenID)
	}
	
	return nil
}

// isValidURI performs basic URI validation
func isValidURI(uri string) bool {
	// Basic checks for common URI schemes
	if len(uri) == 0 {
		return false
	}
	
	// Check for valid URI schemes
	validSchemes := []string{"http://", "https://", "ipfs://", "ar://", "data:"}
	hasValidScheme := false
	for _, scheme := range validSchemes {
		if strings.HasPrefix(strings.ToLower(uri), scheme) {
			hasValidScheme = true
			break
		}
	}
	
	if !hasValidScheme {
		return false
	}
	
	// Check for invalid characters that would break URI parsing
	invalidChars := []string{" ", "\n", "\r", "\t"}
	for _, char := range invalidChars {
		if strings.Contains(uri, char) {
			return false
		}
	}
	
	return true
}

