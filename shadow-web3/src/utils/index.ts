/**
 * Shadowy Web3 Utilities
 */

import { ADDRESS_LENGTH, SATOSHIS_PER_SHADOW } from '../types';

/**
 * Address validation utilities
 */
export class AddressUtils {
  /**
   * Validate a Shadowy address (S-address or L-address)
   */
  static isValidAddress(address: string): boolean {
    if (!address || typeof address !== 'string') return false;
    
    switch (address[0]) {
      case 'S':
        // S-address: 51 characters starting with 'S'
        return address.length === ADDRESS_LENGTH.S_ADDRESS && 
               /^S[0-9a-fA-F]{50}$/.test(address);
      
      case 'L':
        // L-address: 41 characters starting with 'L' 
        return address.length === ADDRESS_LENGTH.L_ADDRESS && 
               /^L[0-9a-fA-F]{40}$/.test(address);
      
      default:
        return false;
    }
  }

  /**
   * Get address type
   */
  static getAddressType(address: string): 'S' | 'L' | null {
    if (!this.isValidAddress(address)) return null;
    return address[0] as 'S' | 'L';
  }

  /**
   * Format address for display (truncate middle)
   */
  static formatAddress(address: string, chars: number = 8): string {
    if (!address || address.length <= chars * 2 + 3) return address;
    return `${address.slice(0, chars + 1)}...${address.slice(-chars)}`;
  }
}

/**
 * Amount conversion utilities
 */
export class AmountUtils {
  /**
   * Convert SHADOW to satoshis
   */
  static shadowToSatoshis(shadow: number): number {
    return Math.round(shadow * SATOSHIS_PER_SHADOW);
  }

  /**
   * Convert satoshis to SHADOW
   */
  static satoshisToShadow(satoshis: number): number {
    return satoshis / SATOSHIS_PER_SHADOW;
  }

  /**
   * Format amount for display
   */
  static formatShadow(shadow: number, decimals: number = 8): string {
    return shadow.toFixed(decimals);
  }

  /**
   * Format amount with units
   */
  static formatShadowWithUnit(shadow: number, decimals: number = 8): string {
    return `${this.formatShadow(shadow, decimals)} SHADOW`;
  }
}

/**
 * Crypto utilities for browser environment
 */
export class CryptoUtils {
  /**
   * Generate secure random bytes using Web Crypto API
   */
  static async getRandomBytes(length: number): Promise<Uint8Array> {
    const array = new Uint8Array(length);
    crypto.getRandomValues(array);
    return array;
  }

  /**
   * Generate 64-byte seed for ML-DSA-87
   */
  static async generateSeed(): Promise<Uint8Array> {
    return this.getRandomBytes(64);
  }

  /**
   * Convert Uint8Array to base64
   */
  static arrayToBase64(array: Uint8Array): string {
    return btoa(String.fromCharCode(...array));
  }

  /**
   * Convert base64 to Uint8Array
   */
  static base64ToArray(base64: string): Uint8Array {
    const binary = atob(base64);
    const array = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      array[i] = binary.charCodeAt(i);
    }
    return array;
  }

  /**
   * Convert Uint8Array to hex string
   */
  static arrayToHex(array: Uint8Array): string {
    return Array.from(array)
      .map(b => b.toString(16).padStart(2, '0'))
      .join('');
  }

  /**
   * Convert hex string to Uint8Array
   */
  static hexToArray(hex: string): Uint8Array {
    const result = new Uint8Array(hex.length / 2);
    for (let i = 0; i < hex.length; i += 2) {
      result[i / 2] = parseInt(hex.substr(i, 2), 16);
    }
    return result;
  }
}

/**
 * Browser storage utilities for secure wallet storage
 */
export class StorageUtils {
  private static readonly WALLET_KEY_PREFIX = 'shadowy_wallet_';
  private static readonly SETTINGS_KEY = 'shadowy_settings';

  /**
   * Store wallet data securely in browser storage
   */
  static storeWallet(name: string, walletData: any, storage: 'localStorage' | 'sessionStorage' = 'localStorage'): boolean {
    try {
      const key = this.WALLET_KEY_PREFIX + name;
      const storageObj = storage === 'localStorage' ? window.localStorage : window.sessionStorage;
      
      // Encrypt wallet data (basic obfuscation - in production use proper encryption)
      const encrypted = btoa(JSON.stringify(walletData));
      storageObj.setItem(key, encrypted);
      
      return true;
    } catch (error) {
      console.error('Failed to store wallet:', error);
      return false;
    }
  }

  /**
   * Load wallet data from browser storage
   */
  static loadWallet(name: string, storage: 'localStorage' | 'sessionStorage' = 'localStorage'): any | null {
    try {
      const key = this.WALLET_KEY_PREFIX + name;
      const storageObj = storage === 'localStorage' ? window.localStorage : window.sessionStorage;
      
      const encrypted = storageObj.getItem(key);
      if (!encrypted) return null;
      
      // Decrypt wallet data
      const decrypted = atob(encrypted);
      return JSON.parse(decrypted);
    } catch (error) {
      console.error('Failed to load wallet:', error);
      return null;
    }
  }

  /**
   * List available wallets
   */
  static listWallets(storage: 'localStorage' | 'sessionStorage' = 'localStorage'): string[] {
    try {
      const storageObj = storage === 'localStorage' ? window.localStorage : window.sessionStorage;
      const keys = Object.keys(storageObj);
      
      return keys
        .filter(key => key.startsWith(this.WALLET_KEY_PREFIX))
        .map(key => key.replace(this.WALLET_KEY_PREFIX, ''));
    } catch (error) {
      console.error('Failed to list wallets:', error);
      return [];
    }
  }

  /**
   * Delete wallet from storage
   */
  static deleteWallet(name: string, storage: 'localStorage' | 'sessionStorage' = 'localStorage'): boolean {
    try {
      const key = this.WALLET_KEY_PREFIX + name;
      const storageObj = storage === 'localStorage' ? window.localStorage : window.sessionStorage;
      
      storageObj.removeItem(key);
      return true;
    } catch (error) {
      console.error('Failed to delete wallet:', error);
      return false;
    }
  }

  /**
   * Store settings
   */
  static storeSettings(settings: any): boolean {
    try {
      window.localStorage.setItem(this.SETTINGS_KEY, JSON.stringify(settings));
      return true;
    } catch (error) {
      console.error('Failed to store settings:', error);
      return false;
    }
  }

  /**
   * Load settings
   */
  static loadSettings(): any {
    try {
      const settings = window.localStorage.getItem(this.SETTINGS_KEY);
      return settings ? JSON.parse(settings) : {};
    } catch (error) {
      console.error('Failed to load settings:', error);
      return {};
    }
  }
}

/**
 * HTTP utilities for API communication
 */
export class HttpUtils {
  /**
   * Make HTTP request with timeout and error handling
   */
  static async request(url: string, options: RequestInit & { timeout?: number } = {}): Promise<Response> {
    const { timeout = 10000, ...fetchOptions } = options;
    
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);
    
    try {
      const response = await fetch(url, {
        ...fetchOptions,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...fetchOptions.headers,
        },
      });
      
      clearTimeout(timeoutId);
      return response;
    } catch (error) {
      clearTimeout(timeoutId);
      throw error;
    }
  }

  /**
   * GET request helper
   */
  static async get(url: string, headers?: Record<string, string>): Promise<any> {
    const response = await this.request(url, {
      method: 'GET',
      headers,
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    return response.json();
  }

  /**
   * POST request helper
   */
  static async post(url: string, data?: any, headers?: Record<string, string>): Promise<any> {
    const response = await this.request(url, {
      method: 'POST',
      headers,
      body: data ? JSON.stringify(data) : undefined,
    });
    
    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
    
    return response.json();
  }
}

/**
 * Event emitter for type-safe events
 */
export class EventEmitter<T extends Record<string, (...args: any[]) => void>> {
  private listeners: Map<keyof T, Set<T[keyof T]>> = new Map();

  on<K extends keyof T>(event: K, listener: T[K]): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(listener);
  }

  off<K extends keyof T>(event: K, listener: T[K]): void {
    const eventListeners = this.listeners.get(event);
    if (eventListeners) {
      eventListeners.delete(listener);
    }
  }

  emit<K extends keyof T>(event: K, ...args: Parameters<T[K]>): void {
    const eventListeners = this.listeners.get(event);
    if (eventListeners) {
      eventListeners.forEach(listener => {
        try {
          listener(...args);
        } catch (error) {
          console.error(`Error in event listener for ${String(event)}:`, error);
        }
      });
    }
  }

  removeAllListeners(event?: keyof T): void {
    if (event) {
      this.listeners.delete(event);
    } else {
      this.listeners.clear();
    }
  }
}