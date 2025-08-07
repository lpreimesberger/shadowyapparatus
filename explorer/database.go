package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
)

// Database handles BadgerDB operations for block storage
type Database struct {
	db *badger.DB
}

// NewDatabase creates a new database instance
func NewDatabase(path string) (*Database, error) {
	opts := badger.DefaultOptions(path)
	opts.Logger = nil // Disable BadgerDB logging to reduce noise
	
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	
	return &Database{db: db}, nil
}

// Close closes the database
func (d *Database) Close() error {
	return d.db.Close()
}

// StoreBlock stores a block in the database
func (d *Database) StoreBlock(blockHash string, block *Block) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store full block data
		blockKey := fmt.Sprintf("block:%s", blockHash)
		blockData, err := json.Marshal(block)
		if err != nil {
			return fmt.Errorf("failed to marshal block: %w", err)
		}
		
		if err := txn.Set([]byte(blockKey), blockData); err != nil {
			return fmt.Errorf("failed to store block: %w", err)
		}
		
		// Store height -> hash mapping for easy retrieval
		heightKey := fmt.Sprintf("height:%016d", block.Header.Height)
		if err := txn.Set([]byte(heightKey), []byte(blockHash)); err != nil {
			return fmt.Errorf("failed to store height mapping: %w", err)
		}
		
		// Update latest height
		latestHeightKey := []byte("latest_height")
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, block.Header.Height)
		if err := txn.Set(latestHeightKey, heightBytes); err != nil {
			return fmt.Errorf("failed to update latest height: %w", err)
		}
		
		return nil
	})
}

// GetBlock retrieves a block by hash
func (d *Database) GetBlock(blockHash string) (*Block, error) {
	var block Block
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := fmt.Sprintf("block:%s", blockHash)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &block)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &block, nil
}

// GetBlockByHeight retrieves a block by height
func (d *Database) GetBlockByHeight(height uint64) (*Block, error) {
	var blockHash string
	
	// First get the hash for this height
	err := d.db.View(func(txn *badger.Txn) error {
		key := fmt.Sprintf("height:%016d", height)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			blockHash = string(val)
			return nil
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	// Then get the block
	return d.GetBlock(blockHash)
}

// GetLatestHeight returns the latest block height
func (d *Database) GetLatestHeight() (uint64, error) {
	var height uint64
	
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("latest_height"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				height = 0
				return nil
			}
			return err
		}
		
		return item.Value(func(val []byte) error {
			if len(val) == 8 {
				height = binary.BigEndian.Uint64(val)
			}
			return nil
		})
	})
	
	return height, err
}

// GetBlocks retrieves blocks with pagination
func (d *Database) GetBlocks(page, perPage int) (*PaginatedBlocks, error) {
	latestHeight, err := d.GetLatestHeight()
	if err != nil {
		return nil, err
	}
	
	totalBlocks := int64(latestHeight + 1) // Heights start from 0
	totalPages := int((totalBlocks + int64(perPage) - 1) / int64(perPage))
	
	if page < 1 {
		page = 1
	}
	if page > totalPages && totalPages > 0 {
		page = totalPages
	}
	
	// Calculate which blocks to fetch (newest first)
	startHeight := latestHeight - uint64((page-1)*perPage)
	var blocks []BlockInfo
	
	err = d.db.View(func(txn *badger.Txn) error {
		for i := 0; i < perPage && startHeight >= uint64(i); i++ {
			height := startHeight - uint64(i)
			
			// Get hash for this height
			heightKey := fmt.Sprintf("height:%016d", height)
			item, err := txn.Get([]byte(heightKey))
			if err != nil {
				if err == badger.ErrKeyNotFound {
					continue
				}
				return err
			}
			
			var blockHash string
			err = item.Value(func(val []byte) error {
				blockHash = string(val)
				return nil
			})
			if err != nil {
				continue
			}
			
			// Get the block
			blockKey := fmt.Sprintf("block:%s", blockHash)
			blockItem, err := txn.Get([]byte(blockKey))
			if err != nil {
				continue
			}
			
			err = blockItem.Value(func(val []byte) error {
				var block Block
				if err := json.Unmarshal(val, &block); err != nil {
					return err
				}
				
				blockInfo := BlockInfo{
					Hash:          blockHash,
					Height:        block.Header.Height,
					Timestamp:     block.Header.Timestamp,
					TxCount:       int(block.Body.TxCount),
					FarmerAddress: block.Header.FarmerAddress,
					Size:          len(val),
				}
				
				blocks = append(blocks, blockInfo)
				return nil
			})
			
			if err != nil {
				log.Printf("Error processing block at height %d: %v", height, err)
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	return &PaginatedBlocks{
		Blocks:      blocks,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalBlocks: totalBlocks,
		PerPage:     perPage,
	}, nil
}

// GetBlockCount returns the total number of blocks
func (d *Database) GetBlockCount() (int64, error) {
	height, err := d.GetLatestHeight()
	if err != nil {
		return 0, err
	}
	return int64(height + 1), nil
}

// SetLastSyncTime stores the last sync timestamp
func (d *Database) SetLastSyncTime(t time.Time) error {
	return d.db.Update(func(txn *badger.Txn) error {
		timeBytes, _ := t.MarshalBinary()
		return txn.Set([]byte("last_sync"), timeBytes)
	})
}

// GetLastSyncTime retrieves the last sync timestamp
func (d *Database) GetLastSyncTime() (time.Time, error) {
	var syncTime time.Time
	
	err := d.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("last_sync"))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil // Return zero time
			}
			return err
		}
		
		return item.Value(func(val []byte) error {
			return syncTime.UnmarshalBinary(val)
		})
	})
	
	return syncTime, err
}

// ResetDatabase clears all explorer data for fresh sync
func (d *Database) ResetDatabase() error {
	return d.db.DropAll()
}

// StoreTransaction stores an individual transaction with address indexing
func (d *Database) StoreTransaction(tx *WalletTransaction) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store full transaction data
		txKey := fmt.Sprintf("tx:%s", tx.TxHash)
		txData, err := json.Marshal(tx)
		if err != nil {
			return fmt.Errorf("failed to marshal transaction: %w", err)
		}
		
		if err := txn.Set([]byte(txKey), txData); err != nil {
			return fmt.Errorf("failed to store transaction: %w", err)
		}
		
		// Index by from_address
		if tx.FromAddress != "" {
			fromKey := fmt.Sprintf("addr_tx:%s:%d:%s", tx.FromAddress, tx.BlockHeight, tx.TxHash)
			if err := txn.Set([]byte(fromKey), []byte(tx.TxHash)); err != nil {
				return fmt.Errorf("failed to store from_address index: %w", err)
			}
		}
		
		// Index by to_address
		if tx.ToAddress != "" {
			toKey := fmt.Sprintf("addr_tx:%s:%d:%s", tx.ToAddress, tx.BlockHeight, tx.TxHash)
			if err := txn.Set([]byte(toKey), []byte(tx.TxHash)); err != nil {
				return fmt.Errorf("failed to store to_address index: %w", err)
			}
		}
		
		return nil
	})
}

// GetWalletTransactions retrieves transactions for an address
func (d *Database) GetWalletTransactions(address string, limit int) ([]WalletTransaction, error) {
	var transactions []WalletTransaction
	
	err := d.db.View(func(txn *badger.Txn) error {
		// Create iterator for address transactions (newest first)
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		opts.Prefix = []byte(fmt.Sprintf("addr_tx:%s:", address))
		it := txn.NewIterator(opts)
		defer it.Close()
		
		count := 0
		for it.Rewind(); it.Valid() && count < limit; it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				txHash := string(val)
				
				// Get the full transaction
				txKey := fmt.Sprintf("tx:%s", txHash)
				txItem, err := txn.Get([]byte(txKey))
				if err != nil {
					return nil // Skip missing transactions
				}
				
				return txItem.Value(func(txData []byte) error {
					var walletTx WalletTransaction
					if err := json.Unmarshal(txData, &walletTx); err != nil {
						return nil // Skip invalid transactions
					}
					transactions = append(transactions, walletTx)
					return nil
				})
			})
			if err != nil {
				continue
			}
			count++
		}
		
		return nil
	})
	
	return transactions, err
}

// GetWalletSummary gets wallet statistics
func (d *Database) GetWalletSummary(address string) (*WalletSummary, error) {
	transactions, err := d.GetWalletTransactions(address, 50) // Get recent transactions
	if err != nil {
		return nil, err
	}
	
	summary := &WalletSummary{
		Address:      address,
		Transactions: transactions,
	}
	
	// Calculate statistics
	var balance uint64
	var blocksMined int
	var firstActivity, lastActivity time.Time
	
	for i, tx := range transactions {
		// Set first/last activity
		if i == 0 || tx.Timestamp.After(lastActivity) {
			lastActivity = tx.Timestamp
		}
		if i == 0 || tx.Timestamp.Before(firstActivity) {
			firstActivity = tx.Timestamp
		}
		
		// Calculate balance changes
		if tx.ToAddress == address {
			balance += tx.Amount
		}
		if tx.FromAddress == address {
			balance -= (tx.Amount + tx.Fee)
		}
		
		// Count mining rewards (transactions with no from_address typically)
		if tx.FromAddress == "" && tx.ToAddress == address {
			blocksMined++
		}
	}
	
	summary.Balance = balance
	summary.TransactionCount = len(transactions)
	summary.BlocksMined = blocksMined
	summary.FirstActivity = firstActivity
	summary.LastActivity = lastActivity
	
	return summary, nil
}

// StoreToken stores token information
func (d *Database) StoreToken(token *TokenInfo) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store full token data
		tokenKey := fmt.Sprintf("token:%s", token.TokenID)
		tokenData, err := json.Marshal(token)
		if err != nil {
			return fmt.Errorf("failed to marshal token: %w", err)
		}
		
		log.Printf("💾 Storing token with key: %s", tokenKey)
		if err := txn.Set([]byte(tokenKey), tokenData); err != nil {
			return fmt.Errorf("failed to store token: %w", err)
		}
		
		// Index by ticker for searching
		if token.Ticker != "" {
			tickerKey := fmt.Sprintf("token_ticker:%s:%s", token.Ticker, token.TokenID)
			log.Printf("💾 Creating ticker index: %s", tickerKey)
			if err := txn.Set([]byte(tickerKey), []byte(token.TokenID)); err != nil {
				return fmt.Errorf("failed to store ticker index: %w", err)
			}
		}
		
		// Index by name for searching
		if token.Name != "" {
			nameKey := fmt.Sprintf("token_name:%s:%s", token.Name, token.TokenID)
			log.Printf("💾 Creating name index: %s", nameKey)
			if err := txn.Set([]byte(nameKey), []byte(token.TokenID)); err != nil {
				return fmt.Errorf("failed to store name index: %w", err)
			}
		}
		
		// Index by creation time for sorting
		creationKey := fmt.Sprintf("token_time:%016d:%s", token.CreationTime.Unix(), token.TokenID)
		log.Printf("💾 Creating time index: %s", creationKey)
		if err := txn.Set([]byte(creationKey), []byte(token.TokenID)); err != nil {
			return fmt.Errorf("failed to store creation time index: %w", err)
		}
		
		log.Printf("✅ Token %s stored with all indexes", token.TokenID)
		return nil
	})
}

// GetTokens retrieves tokens with pagination and optional search
func (d *Database) GetTokens(page, perPage int, search string) (*PaginatedTokens, error) {
	var tokens []TokenInfo
	var totalTokens int64
	
	log.Printf("🔍 DB: GetTokens called - page=%d, perPage=%d, search='%s'", page, perPage, search)
	
	err := d.db.View(func(txn *badger.Txn) error {
		// Get all keys and filter in Go code (more reliable than prefix iterator)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // We only want keys initially
		it := txn.NewIterator(opts)
		defer it.Close()
		
		var matchingKeys []string
		var searchPrefix string
		if search != "" {
			searchPrefix = fmt.Sprintf("token_ticker:%s", search)
		} else {
			searchPrefix = "token_time:"
		}
		
		// Collect all matching keys
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if strings.HasPrefix(key, searchPrefix) {
				matchingKeys = append(matchingKeys, key)
			}
		}
		
		log.Printf("📊 DB: Found %d keys matching prefix '%s'", len(matchingKeys), searchPrefix)
		
		// For token_time keys, we want newest first (reverse sort)
		if searchPrefix == "token_time:" {
			// Keys are already in format token_time:TIMESTAMP:TOKENID, so reverse sort works
			for i := len(matchingKeys)/2 - 1; i >= 0; i-- {
				opp := len(matchingKeys) - 1 - i
				matchingKeys[i], matchingKeys[opp] = matchingKeys[opp], matchingKeys[i]
			}
		}
		
		// Extract token IDs from the keys
		var tokenIDs []string
		for _, key := range matchingKeys {
			// Get the value (token ID) for each key
			item, err := txn.Get([]byte(key))
			if err != nil {
				log.Printf("❌ Failed to get value for key %s: %v", key, err)
				continue
			}
			
			err = item.Value(func(val []byte) error {
				tokenID := string(val)
				tokenIDs = append(tokenIDs, tokenID)
				return nil
			})
			if err != nil {
				log.Printf("❌ Failed to read value for key %s: %v", key, err)
				continue
			}
		}
		
		totalTokens = int64(len(tokenIDs))
		log.Printf("📊 DB: Found %d total tokens with prefix '%s'", totalTokens, searchPrefix)
		
		// Calculate pagination
		totalPages := int((totalTokens + int64(perPage) - 1) / int64(perPage))
		if page < 1 {
			page = 1
		}
		if page > totalPages && totalPages > 0 {
			page = totalPages
		}
		
		// Get tokens for current page
		start := (page - 1) * perPage
		end := start + perPage
		if end > len(tokenIDs) {
			end = len(tokenIDs)
		}
		
		log.Printf("📊 DB: Getting tokens %d-%d from %d total", start, end-1, len(tokenIDs))
		for i := start; i < end; i++ {
			tokenID := tokenIDs[i]
			tokenKey := fmt.Sprintf("token:%s", tokenID)
			
			item, err := txn.Get([]byte(tokenKey))
			if err != nil {
				log.Printf("❌ DB: Failed to get token %s: %v", tokenID, err)
				continue
			}
			
			err = item.Value(func(val []byte) error {
				var token TokenInfo
				if err := json.Unmarshal(val, &token); err != nil {
					log.Printf("❌ DB: Failed to unmarshal token %s: %v", tokenID, err)
					return nil // Skip invalid tokens
				}
				log.Printf("✅ DB: Loaded token %s (%s)", token.Name, token.Ticker)
				tokens = append(tokens, token)
				return nil
			})
			if err != nil {
				continue
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	totalPages := int((totalTokens + int64(perPage) - 1) / int64(perPage))
	
	return &PaginatedTokens{
		Tokens:      tokens,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalTokens: totalTokens,
		PerPage:     perPage,
	}, nil
}

// GetToken retrieves a single token by ID
func (d *Database) GetToken(tokenID string) (*TokenInfo, error) {
	var token TokenInfo
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := fmt.Sprintf("token:%s", tokenID)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &token)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &token, nil
}

// GetTokenDetails retrieves detailed token information including holders and transactions
func (d *Database) GetTokenDetails(tokenID string) (*TokenDetails, error) {
	token, err := d.GetToken(tokenID)
	if err != nil {
		return nil, err
	}
	
	// Get token transactions
	transactions, err := d.GetTokenTransactions(tokenID, 20)
	if err != nil {
		log.Printf("Failed to get token transactions: %v", err)
		transactions = []TokenTransaction{} // Continue with empty list
	}
	
	// Get token holders
	holders, err := d.GetTokenHolders(tokenID, 50)
	if err != nil {
		log.Printf("Failed to get token holders: %v", err)
		holders = []TokenHolder{} // Continue with empty list
	}
	
	return &TokenDetails{
		TokenInfo:    *token,
		Holders:      holders,
		Transactions: transactions,
	}, nil
}

// GetTokenTransactions retrieves transactions for a specific token
func (d *Database) GetTokenTransactions(tokenID string, limit int) ([]TokenTransaction, error) {
	var transactions []TokenTransaction
	
	err := d.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("token_tx:%s:", tokenID))
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		opts.Reverse = true // Newest first
		it := txn.NewIterator(opts)
		defer it.Close()
		
		count := 0
		for it.Rewind(); it.Valid() && count < limit; it.Next() {
			err := it.Item().Value(func(val []byte) error {
				var tokenTx TokenTransaction
				if err := json.Unmarshal(val, &tokenTx); err != nil {
					return nil // Skip invalid transactions
				}
				transactions = append(transactions, tokenTx)
				return nil
			})
			if err != nil {
				continue
			}
			count++
		}
		
		return nil
	})
	
	return transactions, err
}

// GetTokenHolders retrieves holders for a specific token
func (d *Database) GetTokenHolders(tokenID string, limit int) ([]TokenHolder, error) {
	var holders []TokenHolder
	
	err := d.db.View(func(txn *badger.Txn) error {
		prefix := []byte(fmt.Sprintf("token_holder:%s:", tokenID))
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		
		count := 0
		for it.Rewind(); it.Valid() && count < limit; it.Next() {
			err := it.Item().Value(func(val []byte) error {
				var holder TokenHolder
				if err := json.Unmarshal(val, &holder); err != nil {
					return nil // Skip invalid holders
				}
				if holder.Balance > 0 { // Only include holders with positive balance
					holders = append(holders, holder)
				}
				return nil
			})
			if err != nil {
				continue
			}
			count++
		}
		
		return nil
	})
	
	return holders, err
}

// StoreTokenTransaction stores a token transaction
func (d *Database) StoreTokenTransaction(tokenID string, tx *TokenTransaction) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store transaction with timestamp-based key for sorting
		txKey := fmt.Sprintf("token_tx:%s:%016d:%s", tokenID, tx.Timestamp.Unix(), tx.TxHash)
		txData, err := json.Marshal(tx)
		if err != nil {
			return fmt.Errorf("failed to marshal token transaction: %w", err)
		}
		
		return txn.Set([]byte(txKey), txData)
	})
}

// UpdateTokenHolder updates token holder balance
func (d *Database) UpdateTokenHolder(tokenID, address string, balance uint64) error {
	return d.db.Update(func(txn *badger.Txn) error {
		holderKey := fmt.Sprintf("token_holder:%s:%s", tokenID, address)
		holder := TokenHolder{
			Address: address,
			Balance: balance,
		}
		
		holderData, err := json.Marshal(holder)
		if err != nil {
			return fmt.Errorf("failed to marshal token holder: %w", err)
		}
		
		return txn.Set([]byte(holderKey), holderData)
	})
}

// StorePool stores liquidity pool information
func (d *Database) StorePool(pool *LiquidityPool) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store full pool data
		poolKey := fmt.Sprintf("pool:%s", pool.PoolID)
		poolData, err := json.Marshal(pool)
		if err != nil {
			return fmt.Errorf("failed to marshal pool: %w", err)
		}
		
		log.Printf("💾 Storing pool with key: %s", poolKey)
		if err := txn.Set([]byte(poolKey), poolData); err != nil {
			return fmt.Errorf("failed to store pool: %w", err)
		}
		
		// Index by token pair for searching
		pairKey := fmt.Sprintf("pool_pair:%s_%s:%s", pool.TokenA, pool.TokenB, pool.PoolID)
		log.Printf("💾 Creating pair index: %s", pairKey)
		if err := txn.Set([]byte(pairKey), []byte(pool.PoolID)); err != nil {
			return fmt.Errorf("failed to store pair index: %w", err)
		}
		
		// Index by creation time for sorting
		creationKey := fmt.Sprintf("pool_time:%016d:%s", pool.CreationTime.Unix(), pool.PoolID)
		log.Printf("💾 Creating time index: %s", creationKey)
		if err := txn.Set([]byte(creationKey), []byte(pool.PoolID)); err != nil {
			return fmt.Errorf("failed to store creation time index: %w", err)
		}
		
		// Index by TVL for sorting by value
		tvlKey := fmt.Sprintf("pool_tvl:%016d:%s", pool.TVL, pool.PoolID)
		log.Printf("💾 Creating TVL index: %s", tvlKey)
		if err := txn.Set([]byte(tvlKey), []byte(pool.PoolID)); err != nil {
			return fmt.Errorf("failed to store TVL index: %w", err)
		}
		
		log.Printf("✅ Pool %s stored with all indexes", pool.PoolID)
		return nil
	})
}

// GetPools retrieves pools with pagination and optional search
func (d *Database) GetPools(page, perPage int, search string) (*PaginatedPools, error) {
	var pools []LiquidityPool
	var totalPools int64
	
	log.Printf("🔍 DB: GetPools called - page=%d, perPage=%d, search='%s'", page, perPage, search)
	
	err := d.db.View(func(txn *badger.Txn) error {
		// Get all keys and filter in Go code (consistent with GetTokens approach)
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		
		var matchingKeys []string
		var searchPrefix string
		if search != "" {
			// Search by token pair
			searchPrefix = fmt.Sprintf("pool_pair:%s", search)
		} else {
			// Get all pools sorted by TVL (highest first)
			searchPrefix = "pool_tvl:"
		}
		
		// Collect all matching keys
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if strings.HasPrefix(key, searchPrefix) {
				matchingKeys = append(matchingKeys, key)
			}
		}
		
		log.Printf("📊 DB: Found %d keys matching prefix '%s'", len(matchingKeys), searchPrefix)
		
		// For pool_tvl keys, we want highest TVL first (reverse sort)
		if searchPrefix == "pool_tvl:" {
			for i := len(matchingKeys)/2 - 1; i >= 0; i-- {
				opp := len(matchingKeys) - 1 - i
				matchingKeys[i], matchingKeys[opp] = matchingKeys[opp], matchingKeys[i]
			}
		}
		
		// Extract pool IDs from the keys
		var poolIDs []string
		for _, key := range matchingKeys {
			item, err := txn.Get([]byte(key))
			if err != nil {
				log.Printf("❌ Failed to get value for key %s: %v", key, err)
				continue
			}
			
			err = item.Value(func(val []byte) error {
				poolID := string(val)
				poolIDs = append(poolIDs, poolID)
				return nil
			})
			if err != nil {
				log.Printf("❌ Failed to read value for key %s: %v", key, err)
				continue
			}
		}
		
		totalPools = int64(len(poolIDs))
		log.Printf("📊 DB: Found %d total pools with prefix '%s'", totalPools, searchPrefix)
		
		// Calculate pagination
		totalPages := int((totalPools + int64(perPage) - 1) / int64(perPage))
		if page < 1 {
			page = 1
		}
		if page > totalPages && totalPages > 0 {
			page = totalPages
		}
		
		// Get pools for current page
		start := (page - 1) * perPage
		end := start + perPage
		if end > len(poolIDs) {
			end = len(poolIDs)
		}
		
		log.Printf("📊 DB: Getting pools %d-%d from %d total", start, end-1, len(poolIDs))
		for i := start; i < end; i++ {
			poolID := poolIDs[i]
			poolKey := fmt.Sprintf("pool:%s", poolID)
			
			item, err := txn.Get([]byte(poolKey))
			if err != nil {
				log.Printf("❌ DB: Failed to get pool %s: %v", poolID, err)
				continue
			}
			
			err = item.Value(func(val []byte) error {
				var pool LiquidityPool
				if err := json.Unmarshal(val, &pool); err != nil {
					log.Printf("❌ DB: Failed to unmarshal pool %s: %v", poolID, err)
					return nil
				}
				log.Printf("✅ DB: Loaded pool %s/%s", pool.TokenASymbol, pool.TokenBSymbol)
				pools = append(pools, pool)
				return nil
			})
			if err != nil {
				continue
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	totalPages := int((totalPools + int64(perPage) - 1) / int64(perPage))
	
	return &PaginatedPools{
		Pools:       pools,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalPools:  totalPools,
		PerPage:     perPage,
	}, nil
}

// GetPool retrieves a single pool by ID
func (d *Database) GetPool(poolID string) (*LiquidityPool, error) {
	var pool LiquidityPool
	
	err := d.db.View(func(txn *badger.Txn) error {
		key := fmt.Sprintf("pool:%s", poolID)
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &pool)
		})
	})
	
	if err != nil {
		return nil, err
	}
	
	return &pool, nil
}

// GetPoolDetails retrieves detailed pool information including transactions
func (d *Database) GetPoolDetails(poolID string) (*PoolDetails, error) {
	pool, err := d.GetPool(poolID)
	if err != nil {
		return nil, err
	}
	
	// Get pool transactions
	transactions, err := d.GetPoolTransactions(poolID, 20)
	if err != nil {
		log.Printf("Failed to get pool transactions: %v", err)
		transactions = []PoolTransaction{}
	}
	
	return &PoolDetails{
		LiquidityPool: *pool,
		Transactions:  transactions,
	}, nil
}

// GetPoolTransactions retrieves transactions for a specific pool
func (d *Database) GetPoolTransactions(poolID string, limit int) ([]PoolTransaction, error) {
	var transactions []PoolTransaction
	
	err := d.db.View(func(txn *badger.Txn) error {
		// Get all keys and filter for pool transactions
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		
		var matchingKeys []string
		searchPrefix := fmt.Sprintf("pool_tx:%s:", poolID)
		
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if strings.HasPrefix(key, searchPrefix) {
				matchingKeys = append(matchingKeys, key)
			}
		}
		
		// Sort by timestamp (newest first) - keys have timestamp embedded
		for i := len(matchingKeys)/2 - 1; i >= 0; i-- {
			opp := len(matchingKeys) - 1 - i
			matchingKeys[i], matchingKeys[opp] = matchingKeys[opp], matchingKeys[i]
		}
		
		count := 0
		for _, key := range matchingKeys {
			if count >= limit {
				break
			}
			
			item, err := txn.Get([]byte(key))
			if err != nil {
				continue
			}
			
			err = item.Value(func(val []byte) error {
				var poolTx PoolTransaction
				if err := json.Unmarshal(val, &poolTx); err != nil {
					return nil
				}
				transactions = append(transactions, poolTx)
				return nil
			})
			if err != nil {
				continue
			}
			count++
		}
		
		return nil
	})
	
	return transactions, err
}

// StorePoolTransaction stores a pool transaction
func (d *Database) StorePoolTransaction(poolID string, tx *PoolTransaction) error {
	return d.db.Update(func(txn *badger.Txn) error {
		// Store transaction with timestamp-based key for sorting
		txKey := fmt.Sprintf("pool_tx:%s:%016d:%s", poolID, tx.Timestamp.Unix(), tx.TxHash)
		txData, err := json.Marshal(tx)
		if err != nil {
			return fmt.Errorf("failed to marshal pool transaction: %w", err)
		}
		
		return txn.Set([]byte(txKey), txData)
	})
}