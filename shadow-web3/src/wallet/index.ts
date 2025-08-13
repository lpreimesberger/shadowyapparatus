/**
 * Shadowy Web3 Wallet Management
 * Browser-based wallet with secure key storage
 */

import { WalletInfo, CreateWalletOptions, Balance, UTXO, SendTransactionOptions, TransactionResult, ShadowyProvider, ShadowyEvents } from '../types';
import { StorageUtils, AddressUtils, EventEmitter } from '../utils';
import { WasmBridge } from '../wasm';

export class ShadowyWallet extends EventEmitter<ShadowyEvents> {
  private currentWallet: (WalletInfo & { privateKey: string; publicKey: string }) | null = null;
  private wasmBridge: WasmBridge;
  private provider: ShadowyProvider | null = null;
  private storageType: 'localStorage' | 'sessionStorage' | 'memory';

  constructor(
    wasmBridge: WasmBridge,
    provider?: ShadowyProvider,
    storageType: 'localStorage' | 'sessionStorage' | 'memory' = 'localStorage'
  ) {
    super();
    this.wasmBridge = wasmBridge;
    this.provider = provider || null;
    this.storageType = storageType;
  }

  /**
   * Create a new wallet
   */
  async createWallet(options: CreateWalletOptions = {}): Promise<WalletInfo> {
    try {
      const walletData = await this.wasmBridge.createWallet(options);
      
      // Store wallet securely (if not in memory mode)
      if (this.storageType !== 'memory') {
        const success = StorageUtils.storeWallet(walletData.name, {
          name: walletData.name,
          address: walletData.address,
          version: walletData.version,
          createdAt: walletData.createdAt,
          privateKey: walletData.privateKey,
          publicKey: walletData.publicKey
        }, this.storageType);

        if (!success) {
          throw new Error('Failed to store wallet securely');
        }
      }

      this.currentWallet = walletData;
      
      const walletInfo: WalletInfo = {
        name: walletData.name,
        address: walletData.address,
        version: walletData.version,
        createdAt: walletData.createdAt
      };

      this.emit('wallet:created', walletInfo);
      return walletInfo;
    } catch (error) {
      const errorObj = {
        code: 'WALLET_CREATE_FAILED',
        message: `Failed to create wallet: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
      this.emit('error', errorObj);
      throw new Error(errorObj.message);
    }
  }

  /**
   * Load existing wallet
   */
  async loadWallet(name: string): Promise<WalletInfo> {
    try {
      if (this.storageType === 'memory') {
        throw new Error('Cannot load wallet in memory mode');
      }

      const walletData = StorageUtils.loadWallet(name, this.storageType);
      if (!walletData) {
        throw new Error(`Wallet '${name}' not found`);
      }

      // Validate wallet data
      if (!walletData.privateKey || !walletData.address) {
        throw new Error('Invalid wallet data');
      }

      this.currentWallet = walletData;
      
      const walletInfo: WalletInfo = {
        name: walletData.name,
        address: walletData.address,
        version: walletData.version,
        createdAt: walletData.createdAt
      };

      this.emit('wallet:loaded', walletInfo);
      return walletInfo;
    } catch (error) {
      const errorObj = {
        code: 'WALLET_LOAD_FAILED',
        message: `Failed to load wallet: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
      this.emit('error', errorObj);
      throw new Error(errorObj.message);
    }
  }

  /**
   * Get current wallet info (without private key)
   */
  getCurrentWallet(): WalletInfo | null {
    if (!this.currentWallet) return null;
    
    return {
      name: this.currentWallet.name,
      address: this.currentWallet.address,
      version: this.currentWallet.version,
      createdAt: this.currentWallet.createdAt
    };
  }

  /**
   * List all available wallets
   */
  listWallets(): string[] {
    if (this.storageType === 'memory') return [];
    return StorageUtils.listWallets(this.storageType);
  }

  /**
   * Delete a wallet
   */
  async deleteWallet(name: string): Promise<boolean> {
    if (this.storageType === 'memory') return false;
    
    const success = StorageUtils.deleteWallet(name, this.storageType);
    
    // If deleting current wallet, clear it
    if (success && this.currentWallet?.name === name) {
      this.currentWallet = null;
    }
    
    return success;
  }

  /**
   * Get wallet balance (requires provider)
   */
  async getBalance(): Promise<Balance> {
    if (!this.currentWallet) {
      throw new Error('No wallet loaded');
    }
    
    if (!this.provider) {
      throw new Error('Provider not configured - cannot fetch balance');
    }

    try {
      return await this.provider.getBalance(this.currentWallet.address);
    } catch (error) {
      const errorObj = {
        code: 'BALANCE_FETCH_FAILED',
        message: `Failed to get balance: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
      this.emit('error', errorObj);
      throw new Error(errorObj.message);
    }
  }

  /**
   * Get wallet UTXOs (requires provider)
   */
  async getUTXOs(): Promise<UTXO[]> {
    if (!this.currentWallet) {
      throw new Error('No wallet loaded');
    }
    
    if (!this.provider) {
      throw new Error('Provider not configured - cannot fetch UTXOs');
    }

    try {
      return await this.provider.getUTXOs(this.currentWallet.address);
    } catch (error) {
      const errorObj = {
        code: 'UTXO_FETCH_FAILED',
        message: `Failed to get UTXOs: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
      this.emit('error', errorObj);
      throw new Error(errorObj.message);
    }
  }

  /**
   * Send transaction
   */
  async sendTransaction(options: SendTransactionOptions): Promise<TransactionResult> {
    if (!this.currentWallet) {
      throw new Error('No wallet loaded');
    }

    try {
      // Validate recipient address
      if (!AddressUtils.isValidAddress(options.to)) {
        throw new Error(`Invalid recipient address: ${options.to}`);
      }

      // Get UTXOs if provider is available
      let utxos: UTXO[] = [];
      if (this.provider) {
        utxos = await this.getUTXOs();
      }

      // Sign transaction using WASM
      const signedTx = await this.wasmBridge.signTransaction(
        this.currentWallet.privateKey,
        options,
        utxos
      );

      const result: TransactionResult = {
        txHash: signedTx.txHash,
        signature: signedTx.signature,
        rawTransaction: signedTx.transaction,
        status: 'signed',
        message: 'Transaction signed successfully'
      };

      this.emit('transaction:signed', result);

      // Broadcast if provider is available
      if (this.provider) {
        try {
          const broadcastResult = await this.provider.broadcastTransaction(signedTx);
          result.status = broadcastResult.status;
          result.message = broadcastResult.message;
          
          if (result.status === 'broadcast') {
            this.emit('transaction:broadcast', result);
          }
        } catch (error) {
          result.status = 'failed';
          result.message = `Broadcast failed: ${error instanceof Error ? error.message : 'Unknown error'}`;
        }
      }

      return result;
    } catch (error) {
      const result: TransactionResult = {
        txHash: '',
        signature: '',
        rawTransaction: '',
        status: 'failed',
        message: `Transaction failed: ${error instanceof Error ? error.message : 'Unknown error'}`
      };

      const errorObj = {
        code: 'TRANSACTION_FAILED',
        message: result.message || 'Transaction failed'
      };
      this.emit('error', errorObj);
      
      return result;
    }
  }

  /**
   * Validate an address
   */
  async validateAddress(address: string): Promise<boolean> {
    return await this.wasmBridge.validateAddress(address);
  }

  /**
   * Set provider for network operations
   */
  setProvider(provider: ShadowyProvider): void {
    this.provider = provider;
  }

  /**
   * Clear provider
   */
  clearProvider(): void {
    this.provider = null;
  }

  /**
   * Check if wallet can perform network operations
   */
  canPerformNetworkOperations(): boolean {
    return this.provider !== null;
  }

  /**
   * Lock wallet (clear from memory)
   */
  lock(): void {
    this.currentWallet = null;
  }

  /**
   * Check if wallet is unlocked
   */
  isUnlocked(): boolean {
    return this.currentWallet !== null;
  }
}