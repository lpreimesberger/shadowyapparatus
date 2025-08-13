/**
 * WASM Bridge for Shadowy Blockchain Operations
 * Handles loading and interfacing with shadowy.wasm
 */

import { CreateWalletOptions, WalletInfo, SignedTransaction, SendTransactionOptions } from '../types';
import { CryptoUtils } from '../utils';

// WASM Module Interface (matches the Go WASM exports)
interface ShadowyWasm {
  createWallet: (seed?: Uint8Array) => Promise<{
    address: string;
    privateKey: string;
    publicKey: string;
  }>;
  
  signTransaction: (
    privateKey: string,
    to: string,
    amount: number,
    fee: number,
    utxos?: any[]
  ) => Promise<SignedTransaction>;
  
  validateAddress: (address: string) => boolean;
  getAddressFromPrivateKey: (privateKey: string) => string;
}

export class WasmBridge {
  private wasmModule: ShadowyWasm | null = null;
  private isLoaded = false;
  private loadPromise: Promise<void> | null = null;

  constructor(private wasmUrl: string = './shadowy.wasm') {}

  /**
   * Load WASM module
   */
  async load(): Promise<void> {
    if (this.isLoaded) return;
    if (this.loadPromise) return this.loadPromise;

    this.loadPromise = this._loadWasm();
    return this.loadPromise;
  }

  private async _loadWasm(): Promise<void> {
    try {
      // Load WASM using the global Go WASM loader
      if (typeof window !== 'undefined' && (window as any).Go) {
        const go = new (window as any).Go();
        const result = await WebAssembly.instantiateStreaming(fetch(this.wasmUrl), go.importObject);
        go.run(result.instance);
        
        // Wait for WASM to initialize and expose functions
        await this._waitForWasmInit();
        
        this.wasmModule = {
          createWallet: (window as any).createWallet,
          signTransaction: (window as any).signTransaction,
          validateAddress: (window as any).validateAddress,
          getAddressFromPrivateKey: (window as any).getAddressFromPrivateKey
        };
        
        this.isLoaded = true;
      } else {
        throw new Error('Go WASM runtime not found. Please include wasm_exec.js');
      }
    } catch (error) {
      this.isLoaded = false;
      throw new Error(`Failed to load WASM: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  private async _waitForWasmInit(): Promise<void> {
    const maxWait = 5000; // 5 seconds
    const checkInterval = 100; // 100ms
    let waited = 0;

    return new Promise((resolve, reject) => {
      const check = () => {
        if ((window as any).createWallet) {
          resolve();
          return;
        }
        
        waited += checkInterval;
        if (waited >= maxWait) {
          reject(new Error('WASM initialization timeout'));
          return;
        }
        
        setTimeout(check, checkInterval);
      };
      
      check();
    });
  }

  /**
   * Create a new wallet
   */
  async createWallet(options: CreateWalletOptions = {}): Promise<WalletInfo & { privateKey: string; publicKey: string }> {
    await this.load();
    if (!this.wasmModule) throw new Error('WASM not loaded');

    try {
      const seed = options.seed || await CryptoUtils.generateSeed();
      const wallet = await this.wasmModule.createWallet(seed);
      
      const walletInfo: WalletInfo & { privateKey: string; publicKey: string } = {
        name: options.name || `wallet_${Date.now()}`,
        address: wallet.address,
        version: 1,
        createdAt: new Date().toISOString(),
        privateKey: wallet.privateKey,
        publicKey: wallet.publicKey
      };

      return walletInfo;
    } catch (error) {
      throw new Error(`Failed to create wallet: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Sign a transaction
   */
  async signTransaction(
    privateKey: string,
    options: SendTransactionOptions,
    utxos?: any[]
  ): Promise<SignedTransaction> {
    await this.load();
    if (!this.wasmModule) throw new Error('WASM not loaded');

    try {
      const fee = options.fee || 0.011; // Default fee in SHADOW
      
      return await this.wasmModule.signTransaction(
        privateKey,
        options.to,
        options.amount,
        fee,
        utxos
      );
    } catch (error) {
      throw new Error(`Failed to sign transaction: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Validate address format
   */
  async validateAddress(address: string): Promise<boolean> {
    await this.load();
    if (!this.wasmModule) throw new Error('WASM not loaded');

    try {
      return this.wasmModule.validateAddress(address);
    } catch (error) {
      console.warn('WASM address validation failed, falling back to local validation:', error);
      // Fallback to local validation
      const { AddressUtils } = await import('../utils');
      return AddressUtils.isValidAddress(address);
    }
  }

  /**
   * Get address from private key
   */
  async getAddressFromPrivateKey(privateKey: string): Promise<string> {
    await this.load();
    if (!this.wasmModule) throw new Error('WASM not loaded');

    try {
      return this.wasmModule.getAddressFromPrivateKey(privateKey);
    } catch (error) {
      throw new Error(`Failed to get address: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Check if WASM is loaded and ready
   */
  isReady(): boolean {
    return this.isLoaded && this.wasmModule !== null;
  }
}