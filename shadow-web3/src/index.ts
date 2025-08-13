/**
 * Shadowy Web3 API - Main Entry Point
 * TypeScript interface for Shadowy post-quantum blockchain
 */

import { 
  ShadowyWeb3Config, 
  NodeConfig, 
  WalletInfo, 
  CreateWalletOptions,
  SendTransactionOptions,
  TransactionResult,
  Balance,
  UTXO,
  NodeInfo,
  ShadowyEvents
} from './types';
import { ShadowyNetworkProvider } from './client/provider';
import { ShadowyWallet } from './wallet';
import { WasmBridge } from './wasm';
import { EventEmitter, StorageUtils } from './utils';

/**
 * Main Shadowy Web3 API Class
 */
export class ShadowyWeb3 extends EventEmitter<ShadowyEvents> {
  private provider: ShadowyNetworkProvider | null = null;
  private wallet: ShadowyWallet;
  private wasmBridge: WasmBridge;
  private config: ShadowyWeb3Config;

  constructor(config: ShadowyWeb3Config = {}) {
    super();
    
    this.config = {
      // Default configuration
      wasmUrl: './shadowy.wasm',
      storage: 'localStorage',
      network: 'local',
      ...config
    };

    // Initialize WASM bridge
    this.wasmBridge = new WasmBridge(this.config.wasmUrl);

    // Initialize provider if node config provided
    if (this.config.node) {
      this.provider = new ShadowyNetworkProvider(this.config.node);
    }

    // Initialize wallet
    this.wallet = new ShadowyWallet(
      this.wasmBridge, 
      this.provider || undefined, 
      this.config.storage
    );

    // Forward wallet events
    this.wallet.on('wallet:created', (wallet) => this.emit('wallet:created', wallet));
    this.wallet.on('wallet:loaded', (wallet) => this.emit('wallet:loaded', wallet));
    this.wallet.on('transaction:signed', (result) => this.emit('transaction:signed', result));
    this.wallet.on('transaction:broadcast', (result) => this.emit('transaction:broadcast', result));
    this.wallet.on('transaction:confirmed', (result) => this.emit('transaction:confirmed', result));
    this.wallet.on('error', (error) => this.emit('error', error));
  }

  /**
   * Initialize the Web3 instance
   * Loads WASM and connects to provider if configured
   */
  async initialize(): Promise<void> {
    try {
      // Load WASM module
      await this.wasmBridge.load();

      // Test provider connection if configured
      if (this.provider) {
        const isConnected = await this.provider.testConnection();
        if (!isConnected) {
          console.warn('Provider connection failed - working in offline mode');
        }
      }
    } catch (error) {
      const errorObj = {
        code: 'INIT_FAILED',
        message: `Failed to initialize ShadowyWeb3: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
      this.emit('error', errorObj);
      throw new Error(errorObj.message);
    }
  }

  // === Wallet Management ===

  /**
   * Create a new wallet
   */
  async createWallet(options: CreateWalletOptions = {}): Promise<WalletInfo> {
    return await this.wallet.createWallet(options);
  }

  /**
   * Load an existing wallet
   */
  async loadWallet(name: string): Promise<WalletInfo> {
    return await this.wallet.loadWallet(name);
  }

  /**
   * Get current wallet info
   */
  getCurrentWallet(): WalletInfo | null {
    return this.wallet.getCurrentWallet();
  }

  /**
   * List all available wallets
   */
  listWallets(): string[] {
    return this.wallet.listWallets();
  }

  /**
   * Delete a wallet
   */
  async deleteWallet(name: string): Promise<boolean> {
    return await this.wallet.deleteWallet(name);
  }

  /**
   * Lock current wallet (clear from memory)
   */
  lockWallet(): void {
    this.wallet.lock();
  }

  /**
   * Check if a wallet is currently loaded
   */
  isWalletUnlocked(): boolean {
    return this.wallet.isUnlocked();
  }

  // === Blockchain Operations ===

  /**
   * Get wallet balance
   */
  async getBalance(): Promise<Balance> {
    return await this.wallet.getBalance();
  }

  /**
   * Get balance for any address
   */
  async getAddressBalance(address: string): Promise<Balance> {
    if (!this.provider) {
      throw new Error('Provider not configured - cannot fetch balance');
    }
    return await this.provider.getBalance(address);
  }

  /**
   * Get wallet UTXOs
   */
  async getUTXOs(): Promise<UTXO[]> {
    return await this.wallet.getUTXOs();
  }

  /**
   * Get UTXOs for any address
   */
  async getAddressUTXOs(address: string): Promise<UTXO[]> {
    if (!this.provider) {
      throw new Error('Provider not configured - cannot fetch UTXOs');
    }
    return await this.provider.getUTXOs(address);
  }

  /**
   * Send transaction
   */
  async sendTransaction(options: SendTransactionOptions): Promise<TransactionResult> {
    return await this.wallet.sendTransaction(options);
  }

  /**
   * Validate address format
   */
  async validateAddress(address: string): Promise<boolean> {
    return await this.wallet.validateAddress(address);
  }

  // === Network Operations ===

  /**
   * Get node information
   */
  async getNodeInfo(): Promise<NodeInfo> {
    if (!this.provider) {
      throw new Error('Provider not configured - cannot get node info');
    }
    return await this.provider.getNodeInfo();
  }

  /**
   * Test connection to node
   */
  async testConnection(): Promise<boolean> {
    if (!this.provider) return false;
    return await this.provider.testConnection();
  }

  // === Configuration ===

  /**
   * Connect to a node
   */
  connectToNode(nodeConfig: NodeConfig): void {
    this.config.node = nodeConfig;
    this.provider = new ShadowyNetworkProvider(nodeConfig);
    this.wallet.setProvider(this.provider);
  }

  /**
   * Disconnect from node (work offline)
   */
  disconnectFromNode(): void {
    this.provider = null;
    this.config.node = undefined;
    this.wallet.clearProvider();
  }

  /**
   * Check if connected to a node
   */
  isConnected(): boolean {
    return this.provider !== null;
  }

  /**
   * Check if can perform network operations
   */
  canPerformNetworkOperations(): boolean {
    return this.wallet.canPerformNetworkOperations();
  }

  /**
   * Get current configuration
   */
  getConfig(): ShadowyWeb3Config {
    return { ...this.config };
  }

  /**
   * Update configuration
   */
  updateConfig(config: Partial<ShadowyWeb3Config>): void {
    this.config = { ...this.config, ...config };
    
    // Update provider if node config changed
    if (config.node && this.provider) {
      this.provider.updateConfig(config.node);
    }
  }

  // === Settings Persistence ===

  /**
   * Save current settings to browser storage
   */
  saveSettings(): boolean {
    const settings = {
      config: this.config,
      lastWallet: this.wallet.getCurrentWallet()?.name
    };
    return StorageUtils.storeSettings(settings);
  }

  /**
   * Load settings from browser storage
   */
  loadSettings(): any {
    return StorageUtils.loadSettings();
  }

  // === Static Factory Methods ===

  /**
   * Create a ShadowyWeb3 instance connected to a local node
   */
  static createLocal(port: number = 8080): ShadowyWeb3 {
    return new ShadowyWeb3({
      node: {
        url: `http://localhost:${port}`
      },
      network: 'local'
    });
  }

  /**
   * Create a ShadowyWeb3 instance for offline use
   */
  static createOffline(): ShadowyWeb3 {
    return new ShadowyWeb3({
      storage: 'sessionStorage'
    });
  }
}

// Export all types and utilities
export * from './types';
export * from './utils';
export { ShadowyNetworkProvider } from './client/provider';
export { ShadowyWallet } from './wallet';
export { WasmBridge } from './wasm';

// Default export
export default ShadowyWeb3;