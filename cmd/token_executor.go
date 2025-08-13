package cmd

import (
	"fmt"
	"log"
	"time"
)

// TokenExecutor handles the execution of token operations during transaction processing
type TokenExecutor struct {
	tokenState       *TokenState
	syndicateManager *SyndicateManager
}

// NewTokenExecutor creates a new token executor
func NewTokenExecutor(tokenState *TokenState, syndicateManager *SyndicateManager) *TokenExecutor {
	return &TokenExecutor{
		tokenState:       tokenState,
		syndicateManager: syndicateManager,
	}
}

// ExecuteTokenOperations processes all token operations in a transaction
// This should be called during block processing after basic transaction validation
func (te *TokenExecutor) ExecuteTokenOperations(tx *Transaction) (*TokenExecutionResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Starting execution of %d token operations", len(tx.TokenOps))
	
	// Log all token operations for debugging
	for i, op := range tx.TokenOps {
		log.Printf("🔍 [TOKEN_EXECUTOR] Operation %d: Type=%d, From=%s, To=%s, TokenID=%s, Amount=%d", 
			i, op.Type, op.From, op.To, op.TokenID, op.Amount)
	}
	
	if len(tx.TokenOps) == 0 {
		log.Printf("🔍 [TOKEN_EXECUTOR] No token operations to execute")
		return &TokenExecutionResult{Success: true}, nil
	}
	
	result := &TokenExecutionResult{
		Success: true,
		Operations: make([]TokenOpResult, 0, len(tx.TokenOps)),
		ShadowLocked: 0,
		ShadowReleased: 0,
	}
	
	// Process each token operation
	for i, tokenOp := range tx.TokenOps {
		log.Printf("🔍 [TOKEN_EXECUTOR] Executing token operation %d: type=%d, tokenID=%s", i, tokenOp.Type, tokenOp.TokenID)
		opResult, err := te.executeTokenOperation(tokenOp, i)
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("token operation %d failed: %v", i, err)
			
			// Rollback any operations that succeeded
			if err := te.rollbackOperations(result.Operations); err != nil {
				log.Printf("Warning: Failed to rollback token operations: %v", err)
			}
			
			return result, err
		}
		
		result.Operations = append(result.Operations, *opResult)
		result.ShadowLocked += opResult.ShadowLocked
		result.ShadowReleased += opResult.ShadowReleased
	}
	
	return result, nil
}

// executeTokenOperation processes a single token operation
func (te *TokenExecutor) executeTokenOperation(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	switch tokenOp.Type {
	case TOKEN_CREATE:
		return te.executeTokenCreate(tokenOp, index)
	case TOKEN_TRANSFER:
		return te.executeTokenTransfer(tokenOp, index)
	case TOKEN_MELT:
		return te.executeTokenMelt(tokenOp, index)
	case TRADE_OFFER:
		return te.executeTradeOffer(tokenOp, index)
	case TRADE_EXECUTE:
		return te.executeTradeExecute(tokenOp, index)
	case SYNDICATE_JOIN:
		return te.executeSyndicateJoin(tokenOp, index)
	case POOL_CREATE:
		return te.executePoolCreate(tokenOp, index)
	case POOL_SWAP:
		return te.executePoolSwap(tokenOp, index)
	default:
		return nil, fmt.Errorf("unknown token operation type: %d", tokenOp.Type)
	}
}

// executeTokenCreate processes a token creation operation
func (te *TokenExecutor) executeTokenCreate(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Creating token: %s", tokenOp.TokenID)
	
	if tokenOp.Metadata == nil {
		log.Printf("❌ [TOKEN_EXECUTOR] CREATE operation missing metadata for token %s", tokenOp.TokenID)
		return nil, fmt.Errorf("CREATE operation missing metadata")
	}
	
	log.Printf("🔍 [TOKEN_EXECUTOR] Token metadata: name=%s, ticker=%s, supply=%d, lock=%d", 
		tokenOp.Metadata.Name, tokenOp.Metadata.Ticker, tokenOp.Metadata.TotalSupply, tokenOp.Metadata.LockAmount)
	
	// Calculate total Shadow that will be locked
	shadowLocked := tokenOp.Metadata.TotalSupply * tokenOp.Metadata.LockAmount
	log.Printf("🔍 [TOKEN_EXECUTOR] Total shadow to lock: %d", shadowLocked)
	
	// Create the token in the state
	log.Printf("🔍 [TOKEN_EXECUTOR] Creating token in state...")
	err := te.tokenState.CreateToken(tokenOp.TokenID, tokenOp.Metadata)
	if err != nil {
		log.Printf("❌ [TOKEN_EXECUTOR] Failed to create token in state: %v", err)
		return nil, fmt.Errorf("failed to create token: %w", err)
	}
	log.Printf("✅ [TOKEN_EXECUTOR] Token created successfully in state")
	
	log.Printf("Created token %s (%s): %d tokens, %d Shadow locked", 
		tokenOp.TokenID, tokenOp.Metadata.Ticker, tokenOp.Metadata.TotalSupply, shadowLocked)
	
	return &TokenOpResult{
		Index:         index,
		Type:          TOKEN_CREATE,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          "",
		To:            tokenOp.To,
		ShadowLocked:  shadowLocked,
		ShadowReleased: 0,
		Success:       true,
	}, nil
}

// executeTokenTransfer processes a token transfer operation
func (te *TokenExecutor) executeTokenTransfer(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	// Perform the transfer
	err := te.tokenState.TransferToken(tokenOp.TokenID, tokenOp.From, tokenOp.To, tokenOp.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to transfer tokens: %w", err)
	}
	
	log.Printf("Transferred %d tokens of %s from %s to %s", 
		tokenOp.Amount, tokenOp.TokenID, tokenOp.From, tokenOp.To)
	
	// Check if destination is an L-address (liquidity pool)
	log.Printf("🔍 [TOKEN_EXECUTOR] Checking if destination %s is L-address (len=%d, first_char=%c)", 
		tokenOp.To, len(tokenOp.To), tokenOp.To[0])
	if len(tokenOp.To) == 41 && tokenOp.To[0] == 'L' {
		log.Printf("🏊 [TOKEN_EXECUTOR] Detected liquidity provision to L-address %s", tokenOp.To)
		log.Printf("🏊 [TOKEN_EXECUTOR] Calling handleLiquidityProvision with: L-address=%s, provider=%s, tokenID=%s, amount=%d", 
			tokenOp.To, tokenOp.From, tokenOp.TokenID, tokenOp.Amount)
		err = te.handleLiquidityProvision(tokenOp.To, tokenOp.From, tokenOp.TokenID, tokenOp.Amount)
		if err != nil {
			log.Printf("❌ [TOKEN_EXECUTOR] Failed to handle liquidity provision: %v", err)
			// Don't fail the transaction, just log the error
		} else {
			log.Printf("✅ [TOKEN_EXECUTOR] Liquidity provision handled successfully")
		}
	} else {
		log.Printf("🔍 [TOKEN_EXECUTOR] Not an L-address, skipping liquidity provision")
	}
	
	return &TokenOpResult{
		Index:         index,
		Type:          TOKEN_TRANSFER,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          tokenOp.From,
		To:            tokenOp.To,
		ShadowLocked:  0,
		ShadowReleased: 0,
		Success:       true,
	}, nil
}

// handleLiquidityProvision processes when someone sends tokens to an L-address
func (te *TokenExecutor) handleLiquidityProvision(lAddress, provider, tokenID string, amount uint64) error {
	log.Printf("🏊 [LIQUIDITY] Processing liquidity provision: %s sent %d of token %s to pool %s", provider, amount, tokenID, lAddress)
	
	// Find the pool associated with this L-address
	log.Printf("🔍 [LIQUIDITY] Looking for pool with L-address: %s", lAddress)
	_, poolData, err := te.findPoolByLAddress(lAddress)
	if err != nil {
		log.Printf("❌ [LIQUIDITY] Failed to find pool for L-address %s: %v", lAddress, err)
		return fmt.Errorf("failed to find pool for L-address %s: %w", lAddress, err)
	}
	log.Printf("✅ [LIQUIDITY] Found pool: %s/%s with ShareTokenID: %s", poolData.TokenA, poolData.TokenB, poolData.ShareTokenID)
	
	// Validate that the token being sent is one of the pool's tokens
	// Special handling for "SHADOW" which represents the base currency
	if tokenID != poolData.TokenA && tokenID != poolData.TokenB {
		return fmt.Errorf("token %s is not part of pool %s/%s", tokenID, poolData.TokenA, poolData.TokenB)
	}
	
	// Special handling for SHADOW base currency
	if tokenID == "SHADOW" {
		log.Printf("🏊 [LIQUIDITY] Processing SHADOW base currency liquidity provision")
		if poolData.TokenA != "SHADOW" && poolData.TokenB != "SHADOW" {
			return fmt.Errorf("pool does not accept SHADOW base currency")
		}
	}
	
	// Get current pool reserves (tokens held by L-address)
	var reserveA, reserveB uint64
	
	// Handle SHADOW base currency reserves specially
	if poolData.TokenA == "SHADOW" {
		// For SHADOW, we would need to check the UTXO balance at the L-address
		// For now, assume 0 as this requires UTXO tracking
		reserveA = 0
		log.Printf("🏊 [LIQUIDITY] SHADOW reserve tracking not implemented - assuming 0")
	} else {
		reserveA, err = te.getTokenBalance(poolData.TokenA, lAddress)
		if err != nil {
			reserveA = 0
		}
	}
	
	if poolData.TokenB == "SHADOW" {
		// For SHADOW, we would need to check the UTXO balance at the L-address
		reserveB = 0
		log.Printf("🏊 [LIQUIDITY] SHADOW reserve tracking not implemented - assuming 0")
	} else {
		reserveB, err = te.getTokenBalance(poolData.TokenB, lAddress)
		if err != nil {
			reserveB = 0
		}
	}
	
	log.Printf("🏊 [LIQUIDITY] Pool reserves: %d %s, %d %s", reserveA, poolData.TokenA, reserveB, poolData.TokenB)
	
	// Get total LP token supply
	shareToken, err := te.tokenState.GetTokenInfo(poolData.ShareTokenID)
	if err != nil {
		return fmt.Errorf("failed to get LP token info: %w", err)
	}
	
	// Calculate LP tokens to mint
	// For single-sided liquidity provision, use the formula:
	// LP_tokens = (amount / pool_reserve) * total_LP_supply
	var lpTokensToMint uint64
	var currentReserve uint64
	
	if tokenID == poolData.TokenA {
		currentReserve = reserveA
	} else {
		currentReserve = reserveB
	}
	
	if currentReserve == 0 {
		// First liquidity provision - mint proportional to initial supply
		lpTokensToMint = amount * 1000 // Simple ratio for first provision
	} else {
		// Calculate proportional LP tokens: (amount / reserve) * total_supply
		lpTokensToMint = (amount * shareToken.TotalSupply) / currentReserve
	}
	
	if lpTokensToMint == 0 {
		return fmt.Errorf("calculated LP tokens to mint is 0")
	}
	
	log.Printf("🏊 [LIQUIDITY] Minting %d LP tokens directly to liquidity provider %s", lpTokensToMint, provider)
	
	// Mint new LP tokens directly to the liquidity provider
	err = te.tokenState.MintTokensTo(poolData.ShareTokenID, lpTokensToMint, provider)
	if err != nil {
		return fmt.Errorf("failed to mint LP tokens to provider: %w", err)
	}
	
	log.Printf("✅ [LIQUIDITY] Successfully provided liquidity: %s received %d LP tokens", provider, lpTokensToMint)
	return nil
}

// findPoolByLAddress finds the pool NFT and data associated with an L-address
func (te *TokenExecutor) findPoolByLAddress(lAddress string) (string, *LiquidityPoolData, error) {
	// Get all tokens and search for pools with matching L-address
	allTokens := te.tokenState.GetAllTokens()
	log.Printf("🔍 [LIQUIDITY] Searching through %d tokens for L-address %s", len(allTokens), lAddress)
	
	poolCount := 0
	for tokenID, metadata := range allTokens {
		if metadata.LiquidityPool != nil {
			poolCount++
			log.Printf("🔍 [LIQUIDITY] Found pool token %s with L-address %s", tokenID, metadata.LiquidityPool.LAddress)
			if metadata.LiquidityPool.LAddress == lAddress {
				log.Printf("✅ [LIQUIDITY] Found matching pool: tokenID=%s, L-address=%s", tokenID, lAddress)
				return tokenID, metadata.LiquidityPool, nil
			}
		}
	}
	
	log.Printf("❌ [LIQUIDITY] No pool found for L-address %s after searching %d pools", lAddress, poolCount)
	return "", nil, fmt.Errorf("no pool found for L-address %s", lAddress)
}

// getTokenBalance gets the token balance for an address
func (te *TokenExecutor) getTokenBalance(tokenID, address string) (uint64, error) {
	balances := te.tokenState.GetTokenBalances(tokenID)
	if balance, exists := balances[address]; exists {
		return balance, nil
	}
	return 0, fmt.Errorf("no balance found for token %s at address %s", tokenID, address)
}

// executeTokenMelt processes a token melt operation
func (te *TokenExecutor) executeTokenMelt(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	// Check if this is a syndicate membership NFT before melting
	tokenInfo, err := te.tokenState.GetTokenInfo(tokenOp.TokenID)
	if err == nil && tokenInfo.Syndicate != nil {
		// This is a syndicate NFT being melted, remove from syndicate manager
		if te.syndicateManager != nil {
			err = te.syndicateManager.RemoveMember(tokenOp.TokenID)
			if err != nil {
				log.Printf("⚠️ [TOKEN_EXECUTOR] Warning: Failed to remove member from syndicate manager: %v", err)
			}
		}
	}
	
	// Melt the tokens and get Shadow back
	shadowReleased, err := te.tokenState.MeltToken(tokenOp.TokenID, tokenOp.From, tokenOp.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to melt tokens: %w", err)
	}
	
	log.Printf("Melted %d tokens of %s from %s, released %d Shadow", 
		tokenOp.Amount, tokenOp.TokenID, tokenOp.From, shadowReleased)
	
	return &TokenOpResult{
		Index:         index,
		Type:          TOKEN_MELT,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          tokenOp.From,
		To:            "",
		ShadowLocked:  0,
		ShadowReleased: shadowReleased,
		Success:       true,
	}, nil
}

// executeTradeOffer processes a trade offer creation (locks asset in escrow NFT)
func (te *TokenExecutor) executeTradeOffer(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Creating trade offer: %s", tokenOp.TokenID)
	
	if tokenOp.Metadata == nil || tokenOp.Metadata.TradeOffer == nil {
		return nil, fmt.Errorf("TRADE_OFFER operation missing trade offer data")
	}
	
	tradeOffer := tokenOp.Metadata.TradeOffer
	log.Printf("🔍 [TOKEN_EXECUTOR] Trade offer: selling %d of %s for %d satoshi", 
		tradeOffer.LockedAmount, tradeOffer.LockedTokenID, tradeOffer.AskingPrice)
	
	// Step 1: Verify seller has the tokens they want to lock
	if tradeOffer.LockedTokenID == "SHADOW" {
		// For SHADOW trades, we'd need to check UTXO balance
		// For now, assume sufficient balance (this would be checked in validation)
		log.Printf("🔍 [TOKEN_EXECUTOR] Locking %d SHADOW satoshi in trade offer", tradeOffer.LockedAmount)
	} else {
		// Check token balance
		balance, err := te.tokenState.GetTokenBalance(tradeOffer.LockedTokenID, tradeOffer.Seller)
		if err != nil {
			return nil, fmt.Errorf("failed to check seller balance: %w", err)
		}
		
		if balance < tradeOffer.LockedAmount {
			return nil, fmt.Errorf("seller has insufficient balance: need %d, have %d", tradeOffer.LockedAmount, balance)
		}
		
		// Step 2: Transfer the locked tokens FROM seller TO the trade offer NFT
		// This effectively locks them in escrow
		err = te.tokenState.TransferToken(tradeOffer.LockedTokenID, tradeOffer.Seller, tokenOp.TokenID, tradeOffer.LockedAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to lock tokens in escrow: %w", err)
		}
		
		log.Printf("🔍 [TOKEN_EXECUTOR] Locked %d tokens of %s in trade offer NFT %s", 
			tradeOffer.LockedAmount, tradeOffer.LockedTokenID, tokenOp.TokenID)
	}
	
	// Step 3: Create the trade offer NFT itself
	err := te.tokenState.CreateToken(tokenOp.TokenID, tokenOp.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade offer NFT: %w", err)
	}
	
	log.Printf("✅ [TOKEN_EXECUTOR] Trade offer NFT created: %s", tokenOp.TokenID)
	
	return &TokenOpResult{
		Index:         index,
		Type:          TRADE_OFFER,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          tokenOp.From,
		To:            tokenOp.To,
		ShadowLocked:  tokenOp.Metadata.LockAmount, // NFT creation fee
		ShadowReleased: 0,
		Success:       true,
	}, nil
}

// executeTradeExecute processes trade execution (buyer accepts offer)
func (te *TokenExecutor) executeTradeExecute(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Executing trade for NFT: %s", tokenOp.TokenID)
	
	// Step 1: Get the trade offer NFT metadata
	tradeNFT, err := te.tokenState.GetTokenInfo(tokenOp.TokenID)
	if err != nil {
		return nil, fmt.Errorf("trade offer NFT not found: %w", err)
	}
	
	if tradeNFT.TradeOffer == nil {
		return nil, fmt.Errorf("token is not a trade offer NFT")
	}
	
	tradeOffer := tradeNFT.TradeOffer
	buyer := tokenOp.From
	
	log.Printf("🔍 [TOKEN_EXECUTOR] Trade details: buyer=%s, seller=%s, price=%d", 
		buyer, tradeOffer.Seller, tradeOffer.AskingPrice)
	
	// Step 2: Check if trade offer has expired
	// Note: In a real implementation, this would check current block time
	// For now, we'll let expired trades be executed (cleanup is manual)
	
	// Step 3: Verify buyer can afford the asking price
	if tradeOffer.AskingTokenID == "" || tradeOffer.AskingTokenID == "SHADOW" {
		// Payment in SHADOW - would need UTXO checking
		// For now, assume buyer has sufficient SHADOW
		log.Printf("🔍 [TOKEN_EXECUTOR] Payment: %d SHADOW satoshi", tradeOffer.AskingPrice)
	} else {
		// Payment in tokens
		buyerBalance, err := te.tokenState.GetTokenBalance(tradeOffer.AskingTokenID, buyer)
		if err != nil {
			return nil, fmt.Errorf("failed to check buyer balance: %w", err)
		}
		
		if buyerBalance < tradeOffer.AskingPrice {
			return nil, fmt.Errorf("buyer has insufficient balance: need %d, have %d", tradeOffer.AskingPrice, buyerBalance)
		}
		
		// Transfer payment from buyer to seller
		err = te.tokenState.TransferToken(tradeOffer.AskingTokenID, buyer, tradeOffer.Seller, tradeOffer.AskingPrice)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer payment: %w", err)
		}
		
		log.Printf("🔍 [TOKEN_EXECUTOR] Payment transferred: %d of %s from %s to %s", 
			tradeOffer.AskingPrice, tradeOffer.AskingTokenID, buyer, tradeOffer.Seller)
	}
	
	// Step 4: Transfer locked asset from trade NFT to buyer
	if tradeOffer.LockedTokenID == "SHADOW" {
		// For SHADOW, this would involve creating new UTXOs for the buyer
		// and consuming the locked SHADOW
		log.Printf("🔍 [TOKEN_EXECUTOR] Transferring %d SHADOW to buyer %s", tradeOffer.LockedAmount, buyer)
	} else {
		// Transfer tokens from trade NFT (escrow) to buyer
		err = te.tokenState.TransferToken(tradeOffer.LockedTokenID, tokenOp.TokenID, buyer, tradeOffer.LockedAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer locked asset to buyer: %w", err)
		}
		
		log.Printf("🔍 [TOKEN_EXECUTOR] Transferred %d of %s from escrow to buyer %s", 
			tradeOffer.LockedAmount, tradeOffer.LockedTokenID, buyer)
	}
	
	// Step 5: Destroy the trade offer NFT (trade is complete)
	// First transfer it to a null address or burn it
	err = te.tokenState.TransferToken(tokenOp.TokenID, tradeOffer.Seller, "BURNED", 1)
	if err != nil {
		// If transfer fails, try to melt it
		_, err = te.tokenState.MeltToken(tokenOp.TokenID, tradeOffer.Seller, 1)
		if err != nil {
			log.Printf("⚠️ [TOKEN_EXECUTOR] Warning: Failed to destroy trade NFT %s: %v", tokenOp.TokenID, err)
		}
	}
	
	log.Printf("✅ [TOKEN_EXECUTOR] Trade executed successfully! NFT %s destroyed", tokenOp.TokenID)
	
	return &TokenOpResult{
		Index:         index,
		Type:          TRADE_EXECUTE,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          buyer,
		To:            tradeOffer.Seller,
		ShadowLocked:  0,
		ShadowReleased: 0, // Could add the NFT melt value here
		Success:       true,
	}, nil
}

// executeSyndicateJoin processes syndicate membership NFT creation
func (te *TokenExecutor) executeSyndicateJoin(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Creating syndicate membership NFT: %s", tokenOp.TokenID)
	
	if tokenOp.Metadata == nil || tokenOp.Metadata.Syndicate == nil {
		return nil, fmt.Errorf("SYNDICATE_JOIN operation missing syndicate data")
	}
	
	syndicateData := tokenOp.Metadata.Syndicate
	log.Printf("🔍 [TOKEN_EXECUTOR] Syndicate join: %s joining %s with capacity %d bytes", 
		syndicateData.MinerAddress, syndicateData.Syndicate.String(), syndicateData.ReportedCapacity)
	
	// Basic validation
	if syndicateData.MinerAddress == "" {
		return nil, fmt.Errorf("syndicate join requires valid miner address")
	}
	
	if syndicateData.ExpirationTime <= syndicateData.JoinTime {
		return nil, fmt.Errorf("syndicate membership expiration must be after join time")
	}
	
	// Handle automatic assignment to lowest-capacity syndicate
	if syndicateData.Syndicate == SyndicateAuto && te.syndicateManager != nil {
		log.Printf("🔍 [TOKEN_EXECUTOR] Automatic syndicate assignment requested")
		
		// Get the lowest capacity syndicate
		assignedSyndicate := te.syndicateManager.GetLowestCapacitySyndicate()
		
		// Check if the assigned syndicate would violate dominance threshold (35%)
		if te.syndicateManager.CheckDominanceThreshold(assignedSyndicate) {
			log.Printf("⚠️ [TOKEN_EXECUTOR] %s dominance threshold (35%%) would be exceeded, finding alternative", 
				assignedSyndicate.String())
			
			// Find alternative syndicate (try all syndicates)
			alternativeFound := false
			for syndicate := SyndicateSeiryu; syndicate <= SyndicateGenbu; syndicate++ {
				if !te.syndicateManager.CheckDominanceThreshold(syndicate) {
					assignedSyndicate = syndicate
					alternativeFound = true
					log.Printf("🔍 [TOKEN_EXECUTOR] Alternative syndicate assigned: %s", assignedSyndicate.String())
					break
				}
			}
			
			if !alternativeFound {
				return nil, fmt.Errorf("all syndicates would exceed 35%% dominance threshold")
			}
		}
		
		// Update the syndicate data with the assigned syndicate
		syndicateData.Syndicate = assignedSyndicate
		log.Printf("✅ [TOKEN_EXECUTOR] Automatically assigned %s to %s", 
			syndicateData.MinerAddress, assignedSyndicate.Description())
	}
	
	// Create the syndicate membership NFT
	err := te.tokenState.CreateToken(tokenOp.TokenID, tokenOp.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to create syndicate membership NFT: %w", err)
	}
	
	// Add member to syndicate manager
	if te.syndicateManager != nil {
		err = te.syndicateManager.AddMember(tokenOp.TokenID, syndicateData)
		if err != nil {
			log.Printf("⚠️ [TOKEN_EXECUTOR] Warning: Failed to add member to syndicate manager: %v", err)
			// Don't fail the transaction, but log the warning
		}
	}
	
	log.Printf("✅ [TOKEN_EXECUTOR] Syndicate membership NFT created: %s for %s", 
		tokenOp.TokenID, syndicateData.Syndicate.Description())
	
	return &TokenOpResult{
		Index:         index,
		Type:          SYNDICATE_JOIN,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          tokenOp.From,
		To:            tokenOp.To,
		ShadowLocked:  tokenOp.Metadata.LockAmount, // NFT creation fee (0.1 SHADOW)
		ShadowReleased: 0,
		Success:       true,
	}, nil
}

// executePoolCreate processes a liquidity pool creation operation
func (te *TokenExecutor) executePoolCreate(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Creating liquidity pool: %s", tokenOp.TokenID)
	
	if tokenOp.Metadata == nil || tokenOp.Metadata.LiquidityPool == nil {
		return nil, fmt.Errorf("POOL_CREATE operation missing pool data")
	}
	
	poolData := tokenOp.Metadata.LiquidityPool
	log.Printf("🔍 [TOKEN_EXECUTOR] Pool data: %s/%s, ratio=%d:%d, fee=%d", 
		poolData.TokenA, poolData.TokenB, poolData.InitialRatioA, poolData.InitialRatioB, poolData.FeeRate)
	
	// Generate L-address from the transaction hash (we need the tx hash for this)
	// For now, we'll leave it empty and populate it in the HTTP handler
	// The L-address should be set by the caller before creating the transaction
	
	// Create the pool NFT in the state
	err := te.tokenState.CreateToken(tokenOp.TokenID, tokenOp.Metadata)
	if err != nil {
		log.Printf("❌ [TOKEN_EXECUTOR] Failed to create pool NFT: %v", err)
		return nil, fmt.Errorf("failed to create pool NFT: %w", err)
	}
	
	// Create the share token with high melt value
	if poolData.ShareTokenID != "" {
		// Add L-address suffix for uniqueness
		lAddressSuffix := poolData.LAddress[len(poolData.LAddress)-8:] // Last 8 chars of L-address
		shareMetadata := &TokenMetadata{
			Name:         tokenOp.Metadata.Name + " LP-" + lAddressSuffix,
			Ticker:       tokenOp.Metadata.Ticker + "_LP_" + lAddressSuffix,
			TotalSupply:  1000000000000, // 1 trillion shares initially (high precision)
			Decimals:     8,             // 8 decimal places for precision
			LockAmount:   1000000000,    // 10 SHADOW per share (high melt value)
			Creator:      poolData.LAddress, // Shares owned by L-address
			CreationTime: tokenOp.Metadata.CreationTime,
		}
		
		err = te.tokenState.CreateToken(poolData.ShareTokenID, shareMetadata)
		if err != nil {
			log.Printf("❌ [TOKEN_EXECUTOR] Failed to create share token: %v", err)
			return nil, fmt.Errorf("failed to create share token: %w", err)
		}
		log.Printf("✅ [TOKEN_EXECUTOR] Created share token: %s", poolData.ShareTokenID)
		
		// Transfer initial LP tokens to the pool creator
		// Initial liquidity provision gets 100% of the initial supply
		initialLPTokens := shareMetadata.TotalSupply // All initial shares go to the pool creator
		
		log.Printf("🔍 [TOKEN_EXECUTOR] Transferring %d LP tokens to pool creator %s", initialLPTokens, tokenOp.To)
		err = te.tokenState.TransferToken(poolData.ShareTokenID, poolData.LAddress, tokenOp.To, initialLPTokens)
		if err != nil {
			log.Printf("❌ [TOKEN_EXECUTOR] Failed to transfer LP tokens to pool creator: %v", err)
			return nil, fmt.Errorf("failed to transfer LP tokens to pool creator: %w", err)
		}
		log.Printf("✅ [TOKEN_EXECUTOR] Transferred %d LP tokens to pool creator %s", initialLPTokens, tokenOp.To)
	}
	
	log.Printf("✅ [TOKEN_EXECUTOR] Liquidity pool created: %s (%s/%s)", 
		tokenOp.TokenID, poolData.TokenA, poolData.TokenB)
	
	return &TokenOpResult{
		Index:         index,
		Type:          POOL_CREATE,
		TokenID:       tokenOp.TokenID,
		Amount:        tokenOp.Amount,
		From:          tokenOp.From,
		To:            tokenOp.To,
		ShadowLocked:  tokenOp.Metadata.LockAmount, // Pool creation fee (5.0 SHADOW)
		ShadowReleased: 0,
		Success:       true,
	}, nil
}

// executePoolSwap processes a pool swap operation with AMM calculations
func (te *TokenExecutor) executePoolSwap(tokenOp TokenOperation, index int) (*TokenOpResult, error) {
	log.Printf("🔍 [TOKEN_EXECUTOR] Executing pool swap: %s", tokenOp.TokenID)
	
	if tokenOp.Metadata == nil || tokenOp.Metadata.PoolSwap == nil {
		return nil, fmt.Errorf("POOL_SWAP operation missing swap data")
	}
	
	swap := tokenOp.Metadata.PoolSwap
	log.Printf("🔍 [TOKEN_EXECUTOR] Swap: %s -> %s via pool %s, amount=%d", 
		swap.InputTokenID, swap.OutputTokenID, swap.PoolLAddress, tokenOp.Amount)
	
	// Check expiration
	if !swap.NotAfter.IsZero() && swap.NotAfter.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("swap order has expired")
	}
	
	// Find the pool NFT that owns this L-address
	poolNFTID, poolData, err := te.findPoolByLAddress(swap.PoolLAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool: %w", err)
	}
	
	log.Printf("🔍 [TOKEN_EXECUTOR] Found pool NFT: %s, tokens: %s/%s", 
		poolNFTID, poolData.TokenA, poolData.TokenB)
	
	// Verify this pool can handle the swap
	if !((swap.InputTokenID == poolData.TokenA && swap.OutputTokenID == poolData.TokenB) ||
		 (swap.InputTokenID == poolData.TokenB && swap.OutputTokenID == poolData.TokenA)) {
		return nil, fmt.Errorf("pool %s cannot swap %s to %s", 
			swap.PoolLAddress, swap.InputTokenID, swap.OutputTokenID)
	}
	
	// Get current pool reserves
	var inputReserve, outputReserve uint64
	if swap.InputTokenID == poolData.TokenA {
		inputReserve, outputReserve = te.GetPoolReserves(poolData, swap.PoolLAddress)
	} else {
		outputReserve, inputReserve = te.GetPoolReserves(poolData, swap.PoolLAddress)
	}
	
	log.Printf("🔍 [TOKEN_EXECUTOR] Current reserves - Input: %d, Output: %d", inputReserve, outputReserve)
	
	// Calculate AMM output using constant product formula: k = x * y
	// For input amount deltaX, output amount deltaY = (y * deltaX) / (x + deltaX)
	// With fee: deltaY = (y * deltaX * (10000 - fee)) / ((x + deltaX) * 10000)
	inputAmount := tokenOp.Amount
	feeRate := poolData.FeeRate // fee rate in basis points
	
	// Calculate output amount with fee
	numerator := outputReserve * inputAmount * (10000 - feeRate)
	denominator := (inputReserve + inputAmount) * 10000
	
	if denominator == 0 {
		return nil, fmt.Errorf("pool has insufficient liquidity")
	}
	
	outputAmount := numerator / denominator
	if outputAmount == 0 {
		if outputReserve == 0 {
			return nil, fmt.Errorf("pool has no liquidity in output token - cannot swap")
		}
		return nil, fmt.Errorf("swap amount too small, would result in zero output")
	}
	
	log.Printf("🔍 [TOKEN_EXECUTOR] AMM calculation: %d input -> %d output (fee=%d bp)", 
		inputAmount, outputAmount, feeRate)
	
	// Check slippage protection
	if swap.MaxSlippage > 0 {
		// Calculate expected output without fee for slippage calculation
		expectedOutput := (outputReserve * inputAmount) / (inputReserve + inputAmount)
		actualSlippage := ((expectedOutput - outputAmount) * 10000) / expectedOutput
		
		if actualSlippage > swap.MaxSlippage {
			return nil, fmt.Errorf("slippage %d bp exceeds maximum %d bp", 
				actualSlippage, swap.MaxSlippage)
		}
	}
	
	// Check minimum received amount
	if swap.MinReceived > 0 && outputAmount < swap.MinReceived {
		if swap.AllOrNothing {
			return nil, fmt.Errorf("output %d below minimum %d (all-or-nothing)", 
				outputAmount, swap.MinReceived)
		}
		// Partial execution: reduce input to achieve minimum output
		// Solve: minOutput = (reserve_out * reduced_input * (10000 - fee)) / ((reserve_in + reduced_input) * 10000)
		// This requires solving a quadratic equation, for now just fail
		return nil, fmt.Errorf("partial execution not yet implemented, output %d below minimum %d", 
			outputAmount, swap.MinReceived)
	}
	
	// Execute the swap
	log.Printf("🔍 [TOKEN_EXECUTOR] Executing swap: %d %s -> %d %s", 
		inputAmount, swap.InputTokenID, outputAmount, swap.OutputTokenID)
	
	// Transfer input tokens from swapper to pool
	if swap.InputTokenID == "SHADOW" {
		// For SHADOW swaps, this would be handled by the blockchain's base transaction processing
		// We just need to update the pool's "shadow reserves" tracking
		log.Printf("🔍 [TOKEN_EXECUTOR] SHADOW input handled by base transaction processing")
	} else {
		// Transfer tokens from swapper to L-address
		err = te.tokenState.TransferToken(swap.InputTokenID, swap.SwapperAddress, swap.PoolLAddress, inputAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer input tokens to pool: %w", err)
		}
	}
	
	// Transfer output tokens from pool to swapper
	if swap.OutputTokenID == "SHADOW" {
		// For SHADOW output, we need to track this for the blockchain to process
		// The actual SHADOW transfer will be handled by creating appropriate transaction outputs
		log.Printf("🔍 [TOKEN_EXECUTOR] SHADOW output of %d will be handled by transaction outputs", outputAmount)
	} else {
		// Transfer tokens from L-address to swapper
		err = te.tokenState.TransferToken(swap.OutputTokenID, swap.PoolLAddress, swap.SwapperAddress, outputAmount)
		if err != nil {
			return nil, fmt.Errorf("failed to transfer output tokens from pool: %w", err)
		}
	}
	
	log.Printf("✅ [TOKEN_EXECUTOR] Pool swap completed: %d %s -> %d %s", 
		inputAmount, swap.InputTokenID, outputAmount, swap.OutputTokenID)
	
	return &TokenOpResult{
		Index:          index,
		Type:           POOL_SWAP,
		TokenID:        tokenOp.TokenID,
		Amount:         inputAmount,
		From:           tokenOp.From,
		To:             tokenOp.To,
		ShadowLocked:   0, // No shadow locked for swaps
		ShadowReleased: 0, // Shadow movement handled separately
		Success:        true,
	}, nil
}

// GetPoolReserves returns the current reserves for a pool (tokenA reserve, tokenB reserve)
func (te *TokenExecutor) GetPoolReserves(poolData *LiquidityPoolData, lAddress string) (uint64, uint64) {
	var reserveA, reserveB uint64
	
	// Get token A balance of the pool
	if poolData.TokenA == "SHADOW" {
		// For SHADOW, we need to query the blockchain's UTXO set
		// For now, assume it's tracked somewhere - this needs integration with blockchain state
		reserveA = poolData.InitialRatioA // Fallback to initial ratio
	} else {
		balance, err := te.tokenState.GetTokenBalance(poolData.TokenA, lAddress)
		if err != nil {
			reserveA = 0
		} else {
			reserveA = balance
		}
	}
	
	// Get token B balance of the pool
	if poolData.TokenB == "SHADOW" {
		// For SHADOW, we need to query the blockchain's UTXO set
		reserveB = poolData.InitialRatioB // Fallback to initial ratio
	} else {
		balance, err := te.tokenState.GetTokenBalance(poolData.TokenB, lAddress)
		if err != nil {
			reserveB = 0
		} else {
			reserveB = balance
		}
	}
	
	return reserveA, reserveB
}

// rollbackOperations attempts to rollback completed operations (best effort)
func (te *TokenExecutor) rollbackOperations(operations []TokenOpResult) error {
	// Process in reverse order
	for i := len(operations) - 1; i >= 0; i-- {
		op := operations[i]
		if !op.Success {
			continue
		}
		
		switch op.Type {
		case TOKEN_CREATE:
			// For CREATE, we'd need to remove the token entirely
			// This is complex and dangerous, so we'll log an error
			log.Printf("ERROR: Cannot rollback token creation for %s - manual intervention required", op.TokenID)
			
		case TOKEN_TRANSFER:
			// Reverse the transfer
			err := te.tokenState.TransferToken(op.TokenID, op.To, op.From, op.Amount)
			if err != nil {
				log.Printf("ERROR: Failed to rollback transfer for token %s: %v", op.TokenID, err)
			} else {
				log.Printf("Rolled back transfer of %d tokens of %s", op.Amount, op.TokenID)
			}
			
		case TOKEN_MELT:
			// For MELT, we'd need to recreate tokens and re-lock Shadow
			// This is complex and may not be possible if Shadow was already distributed
			log.Printf("ERROR: Cannot rollback token melt for %s - manual intervention required", op.TokenID)
		}
	}
	
	return nil
}

// ValidateTokenOperationExecution validates that token operations can be executed
// This should be called during transaction validation before adding to mempool
func (te *TokenExecutor) ValidateTokenOperationExecution(tx *Transaction) error {
	log.Printf("🔍 [TOKEN_EXECUTOR] Validating %d token operations", len(tx.TokenOps))
	
	for i, tokenOp := range tx.TokenOps {
		log.Printf("🔍 [TOKEN_EXECUTOR] Validating operation %d: type=%d, tokenID=%s", i, tokenOp.Type, tokenOp.TokenID)
		
		switch tokenOp.Type {
		case TOKEN_CREATE:
			log.Printf("🔍 [TOKEN_EXECUTOR] Checking if token %s already exists", tokenOp.TokenID)
			// Check if token already exists
			if _, err := te.tokenState.GetTokenInfo(tokenOp.TokenID); err == nil {
				log.Printf("❌ [TOKEN_EXECUTOR] Token %s already exists", tokenOp.TokenID)
				return fmt.Errorf("token operation %d: token %s already exists", i, tokenOp.TokenID)
			}
			log.Printf("✅ [TOKEN_EXECUTOR] Token %s does not exist, validation passed", tokenOp.TokenID)
			
		case TOKEN_TRANSFER:
			// Check if token exists
			if _, err := te.tokenState.GetTokenInfo(tokenOp.TokenID); err != nil {
				return fmt.Errorf("token operation %d: token %s does not exist", i, tokenOp.TokenID)
			}
			
			// Check if sender has sufficient balance
			balance, err := te.tokenState.GetTokenBalance(tokenOp.TokenID, tokenOp.From)
			if err != nil {
				return fmt.Errorf("token operation %d: failed to get balance: %w", i, err)
			}
			
			if balance < tokenOp.Amount {
				return fmt.Errorf("token operation %d: insufficient balance: have %d, need %d", 
					i, balance, tokenOp.Amount)
			}
			
		case TOKEN_MELT:
			// Check if token exists
			if _, err := te.tokenState.GetTokenInfo(tokenOp.TokenID); err != nil {
				return fmt.Errorf("token operation %d: token %s does not exist", i, tokenOp.TokenID)
			}
			
			// Check if sender has sufficient balance
			balance, err := te.tokenState.GetTokenBalance(tokenOp.TokenID, tokenOp.From)
			if err != nil {
				return fmt.Errorf("token operation %d: failed to get balance: %w", i, err)
			}
			
			if balance < tokenOp.Amount {
				return fmt.Errorf("token operation %d: insufficient balance for melt: have %d, need %d", 
					i, balance, tokenOp.Amount)
			}
			
		case TRADE_OFFER:
			// Validate trade offer metadata
			if tokenOp.Metadata == nil || tokenOp.Metadata.TradeOffer == nil {
				return fmt.Errorf("token operation %d: TRADE_OFFER missing trade data", i)
			}
			
			tradeOffer := tokenOp.Metadata.TradeOffer
			
			// Check if seller has sufficient balance of locked token
			if tradeOffer.LockedTokenID != "SHADOW" {
				if _, err := te.tokenState.GetTokenInfo(tradeOffer.LockedTokenID); err != nil {
					return fmt.Errorf("token operation %d: locked token %s does not exist", i, tradeOffer.LockedTokenID)
				}
				
				balance, err := te.tokenState.GetTokenBalance(tradeOffer.LockedTokenID, tradeOffer.Seller)
				if err != nil {
					return fmt.Errorf("token operation %d: failed to get seller balance: %w", i, err)
				}
				
				if balance < tradeOffer.LockedAmount {
					return fmt.Errorf("token operation %d: seller insufficient balance: have %d, need %d", 
						i, balance, tradeOffer.LockedAmount)
				}
			}
			
		case TRADE_EXECUTE:
			// Check if trade offer NFT exists
			tradeNFT, err := te.tokenState.GetTokenInfo(tokenOp.TokenID)
			if err != nil {
				return fmt.Errorf("token operation %d: trade offer NFT %s does not exist", i, tokenOp.TokenID)
			}
			
			if tradeNFT.TradeOffer == nil {
				return fmt.Errorf("token operation %d: token %s is not a trade offer", i, tokenOp.TokenID)
			}
			
			tradeOffer := tradeNFT.TradeOffer
			
			// Check if buyer has sufficient balance to pay
			if tradeOffer.AskingTokenID != "" && tradeOffer.AskingTokenID != "SHADOW" {
				buyerBalance, err := te.tokenState.GetTokenBalance(tradeOffer.AskingTokenID, tokenOp.From)
				if err != nil {
					return fmt.Errorf("token operation %d: failed to get buyer balance: %w", i, err)
				}
				
				if buyerBalance < tradeOffer.AskingPrice {
					return fmt.Errorf("token operation %d: buyer insufficient balance: have %d, need %d", 
						i, buyerBalance, tradeOffer.AskingPrice)
				}
			}
			
		case SYNDICATE_JOIN:
			// Validate syndicate join metadata
			if tokenOp.Metadata == nil || tokenOp.Metadata.Syndicate == nil {
				return fmt.Errorf("token operation %d: SYNDICATE_JOIN missing syndicate data", i)
			}
			
			syndicateData := tokenOp.Metadata.Syndicate
			
			// Validate miner address
			if syndicateData.MinerAddress == "" {
				return fmt.Errorf("token operation %d: syndicate join requires valid miner address", i)
			}
			
			// Validate syndicate type
			if syndicateData.Syndicate < SyndicateSeiryu || syndicateData.Syndicate > SyndicateAuto {
				return fmt.Errorf("token operation %d: invalid syndicate type %d", i, syndicateData.Syndicate)
			}
			
			// Validate expiration time
			if syndicateData.ExpirationTime <= syndicateData.JoinTime {
				return fmt.Errorf("token operation %d: syndicate expiration must be after join time", i)
			}
			
			// Validate maximum 8-day lifespan (8 * 24 * 3600 = 691200 seconds)
			maxLifespan := int64(8 * 24 * 3600)
			if syndicateData.ExpirationTime > syndicateData.JoinTime + maxLifespan {
				return fmt.Errorf("token operation %d: syndicate membership cannot exceed 8 days", i)
			}
			
			// Validate that this creates an NFT
			if tokenOp.Metadata.Decimals != 0 || tokenOp.Metadata.TotalSupply != 1 {
				return fmt.Errorf("token operation %d: syndicate memberships must be NFTs (decimals=0, supply=1)", i)
			}
			
			// TODO: Add future validation in Phase 2:
			// - Check if syndicate has won more than 35% of past 2016 blocks
			// - Validate reported capacity against network baseline
			// - Check if miner already has active membership
		}
	}
	
	return nil
}

// TokenExecutionResult represents the result of executing token operations in a transaction
type TokenExecutionResult struct {
	Success        bool            `json:"success"`
	Error          string          `json:"error,omitempty"`
	Operations     []TokenOpResult `json:"operations"`
	ShadowLocked   uint64          `json:"shadow_locked"`   // Total Shadow locked by this transaction
	ShadowReleased uint64          `json:"shadow_released"` // Total Shadow released by this transaction
}

// TokenOpResult represents the result of executing a single token operation
type TokenOpResult struct {
	Index          int          `json:"index"`
	Type           TokenOpType  `json:"type"`
	TokenID        string       `json:"token_id"`
	Amount         uint64       `json:"amount"`
	From           string       `json:"from,omitempty"`
	To             string       `json:"to,omitempty"`
	ShadowLocked   uint64       `json:"shadow_locked"`
	ShadowReleased uint64       `json:"shadow_released"`
	Success        bool         `json:"success"`
	Error          string       `json:"error,omitempty"`
}