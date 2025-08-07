package main

import (
	"encoding/json"
	"time"
)

// Block represents a Shadowy blockchain block
type Block struct {
	Header BlockHeader `json:"header"`
	Body   BlockBody   `json:"body"`
}

// BlockHeader contains block metadata
type BlockHeader struct {
	Version           uint32    `json:"version"`
	PreviousBlockHash string    `json:"previous_block_hash"`
	MerkleRoot        string    `json:"merkle_root"`
	Timestamp         time.Time `json:"timestamp"`
	Height            uint64    `json:"height"`
	Nonce             uint64    `json:"nonce"`
	ChallengeSeed     string    `json:"challenge_seed"`
	ProofHash         string    `json:"proof_hash"`
	FarmerAddress     string    `json:"farmer_address"`
	PlotID            string    `json:"plot_id,omitempty"`
	Challenge         string    `json:"challenge,omitempty"`
	Proof             string    `json:"proof,omitempty"`
}

// BlockBody contains block transactions
type BlockBody struct {
	Transactions     []SignedTransaction `json:"transactions"`
	TxCount          uint32              `json:"tx_count"`
	TransactionsHash string              `json:"transactions_hash"`
}

// SignedTransaction represents a signed transaction
type SignedTransaction struct {
	Transaction json.RawMessage `json:"transaction"`
	Signature   string          `json:"signature"`
	TxHash      string          `json:"tx_hash"`
	SignerKey   string          `json:"signer_key"`
	Algorithm   string          `json:"algorithm"`
	Header      JOSEHeader      `json:"header"`
}

// Transaction represents a Shadowy blockchain transaction (parsed from SignedTransaction.Transaction)
type Transaction struct {
	Version   int                 `json:"version"`
	Inputs    []TransactionInput  `json:"inputs"`
	Outputs   []TransactionOutput `json:"outputs"`
	TokenOps  []TokenOperation    `json:"token_ops,omitempty"`
	NotUntil  time.Time          `json:"not_until"`
	Timestamp time.Time          `json:"timestamp"`
	Nonce     uint64             `json:"nonce"`
}

// TransactionInput represents a reference to a previous transaction output
type TransactionInput struct {
	PreviousTxHash string `json:"previous_tx_hash"`
	OutputIndex    uint32 `json:"output_index"`
	ScriptSig      string `json:"script_sig"`
	Sequence       uint32 `json:"sequence"`
}

// TransactionOutput represents a payment to an address
type TransactionOutput struct {
	Value        uint64 `json:"value"`
	ScriptPubKey string `json:"script_pubkey"`
	Address      string `json:"address"`
}

// TokenOpType represents the type of token operation (matches blockchain)
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

// TokenMetadata contains the immutable properties of a token
type TokenMetadata struct {
	Name         string `json:"name"`          // Human readable name (e.g. "Steve Coin")
	Ticker       string `json:"ticker"`        // Short symbol (e.g. "STEVE")
	TotalSupply  uint64 `json:"total_supply"`  // Fixed total supply (with decimals applied)
	Decimals     uint8  `json:"decimals"`      // Number of decimal places (0-18)
	LockAmount   uint64 `json:"lock_amount"`   // Shadow satoshi locked per token unit
	Creator      string `json:"creator"`       // Address of token creator
	CreationTime int64  `json:"creation_time"` // Unix timestamp of creation
	URI          string `json:"uri,omitempty"` // Optional URI for metadata/NFT content (max 128 chars)
}

// TokenOperation represents a token-related operation
type TokenOperation struct {
	Type     TokenOpType    `json:"type"`                // Operation type (as int from blockchain)
	TokenID  string         `json:"token_id"`            // Unique token identifier (hex)
	Amount   uint64         `json:"amount"`              // Token amount (with decimals applied)
	From     string         `json:"from,omitempty"`      // Source address (for TRANSFER/MELT)
	To       string         `json:"to,omitempty"`        // Destination address (for TRANSFER)
	Metadata *TokenMetadata `json:"metadata,omitempty"`  // Token metadata (for CREATE only)
}

// WalletTransaction represents a transaction from a wallet perspective
type WalletTransaction struct {
	TxHash      string    `json:"tx_hash"`
	BlockHash   string    `json:"block_hash"`
	BlockHeight uint64    `json:"block_height"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "received", "sent", "token_transfer", etc.
	Amount      uint64    `json:"amount"`
	Fee         uint64    `json:"fee"`
	FromAddress string    `json:"from_address"`
	ToAddress   string    `json:"to_address"`
	TokenSymbol string    `json:"token_symbol,omitempty"`
	TokenAmount uint64    `json:"token_amount,omitempty"`
}

// WalletSummary represents wallet statistics
type WalletSummary struct {
	Address            string              `json:"address"`
	Balance            uint64              `json:"balance"`
	TransactionCount   int                 `json:"transaction_count"`
	BlocksMined        int                 `json:"blocks_mined"`
	FirstActivity      time.Time           `json:"first_activity"`
	LastActivity       time.Time           `json:"last_activity"`
	Transactions       []WalletTransaction `json:"transactions"`
}

// TokenInfo represents token statistics for the explorer
type TokenInfo struct {
	TokenID       string    `json:"token_id"`
	Name          string    `json:"name"`
	Ticker        string    `json:"ticker"`
	TotalSupply   uint64    `json:"total_supply"`
	Decimals      uint8     `json:"decimals"`
	Creator       string    `json:"creator"`
	CreationTime  time.Time `json:"creation_time"`
	CreationBlock uint64    `json:"creation_block"`
	URI           string    `json:"uri,omitempty"`
	
	// Statistics
	HolderCount    int       `json:"holder_count"`
	TransferCount  int       `json:"transfer_count"`
	LastActivity   time.Time `json:"last_activity"`
	TotalMelted    uint64    `json:"total_melted"`
	CirculatingSupply uint64 `json:"circulating_supply"`
	MeltValue      uint64    `json:"melt_value"` // Total SHADOW locked
}

// PaginatedTokens represents a paginated response of tokens
type PaginatedTokens struct {
	Tokens      []TokenInfo `json:"tokens"`
	CurrentPage int         `json:"current_page"`
	TotalPages  int         `json:"total_pages"`
	TotalTokens int64       `json:"total_tokens"`
	PerPage     int         `json:"per_page"`
}

// TokenHolder represents someone who holds a token
type TokenHolder struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

// TokenTransaction represents a token-specific transaction
type TokenTransaction struct {
	TxHash      string    `json:"tx_hash"`
	BlockHash   string    `json:"block_hash"`
	BlockHeight uint64    `json:"block_height"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"` // "create", "transfer", "melt"
	Amount      uint64    `json:"amount"`
	FromAddress string    `json:"from_address"`
	ToAddress   string    `json:"to_address"`
}

// TokenDetails represents detailed token information
type TokenDetails struct {
	TokenInfo
	Holders      []TokenHolder      `json:"holders"`
	Transactions []TokenTransaction `json:"recent_transactions"`
}

// JOSEHeader for JWT-style signing
type JOSEHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// BlockInfo is a simplified block representation for the explorer
type BlockInfo struct {
	Hash          string    `json:"hash"`
	Height        uint64    `json:"height"`
	Timestamp     time.Time `json:"timestamp"`
	TxCount       int       `json:"tx_count"`
	FarmerAddress string    `json:"farmer_address"`
	Size          int       `json:"size"`
}

// PaginatedBlocks represents a paginated response of blocks
type PaginatedBlocks struct {
	Blocks      []BlockInfo `json:"blocks"`
	CurrentPage int         `json:"current_page"`
	TotalPages  int         `json:"total_pages"`
	TotalBlocks int64       `json:"total_blocks"`
	PerPage     int         `json:"per_page"`
}

// NetworkStats represents blockchain network statistics
type NetworkStats struct {
	Height       uint64    `json:"height"`
	TotalBlocks  int64     `json:"total_blocks"`
	LastSync     time.Time `json:"last_sync"`
	SyncStatus   string    `json:"sync_status"`
	NodeURL      string    `json:"node_url"`
}

// LiquidityPool represents a liquidity pool
type LiquidityPool struct {
	PoolID         string    `json:"pool_id"`
	TokenA         string    `json:"token_a"`          // First token ID
	TokenB         string    `json:"token_b"`          // Second token ID (empty for SHADOW pairs)
	TokenASymbol   string    `json:"token_a_symbol"`   // Token A ticker
	TokenBSymbol   string    `json:"token_b_symbol"`   // Token B ticker
	ReserveA       uint64    `json:"reserve_a"`        // Token A reserves
	ReserveB       uint64    `json:"reserve_b"`        // Token B reserves
	TotalLiquidity uint64    `json:"total_liquidity"`  // LP tokens issued
	Creator        string    `json:"creator"`          // Pool creator address
	CreationTime   time.Time `json:"creation_time"`
	CreationBlock  uint64    `json:"creation_block"`
	
	// Statistics
	TradeCount     int       `json:"trade_count"`      // Number of trades
	VolumeA        uint64    `json:"volume_a"`         // Total volume in token A
	VolumeB        uint64    `json:"volume_b"`         // Total volume in token B
	LastActivity   time.Time `json:"last_activity"`
	APR            float64   `json:"apr"`              // Annual percentage return
	TVL            uint64    `json:"tvl"`              // Total value locked in SHADOW
}

// PaginatedPools represents a paginated response of pools
type PaginatedPools struct {
	Pools       []LiquidityPool `json:"pools"`
	CurrentPage int             `json:"current_page"`
	TotalPages  int             `json:"total_pages"`
	TotalPools  int64           `json:"total_pools"`
	PerPage     int             `json:"per_page"`
}

// PoolTransaction represents a pool-related transaction
type PoolTransaction struct {
	TxHash      string    `json:"tx_hash"`
	BlockHash   string    `json:"block_hash"`
	BlockHeight uint64    `json:"block_height"`
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // "create", "add_liquidity", "remove_liquidity", "swap"
	AmountA     uint64    `json:"amount_a"`    // Amount of token A
	AmountB     uint64    `json:"amount_b"`    // Amount of token B
	Address     string    `json:"address"`     // User address
	LPTokens    uint64    `json:"lp_tokens"`   // LP tokens minted/burned
}

// PoolDetails represents detailed pool information
type PoolDetails struct {
	LiquidityPool
	Transactions []PoolTransaction `json:"recent_transactions"`
}