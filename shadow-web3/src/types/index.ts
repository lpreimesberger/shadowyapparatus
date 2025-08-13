/**
 * Shadowy Web3 API Types
 * Post-quantum blockchain interface types
 */

// Network and Node Types
export interface NodeConfig {
  url: string;
  apiKey?: string;
  timeout?: number;
}

export interface NodeInfo {
  tipHeight: number;
  totalBlocks: number;
  totalTransactions: number;
  status: string;
  version?: string;
}

// Address and Balance Types
export interface Address {
  address: string;
  type: 'S' | 'L'; // S-address or L-address
}

export interface Balance {
  address: string;
  balance: number; // In SHADOW
  balanceSatoshis: number;
  confirmed: number;
  confirmedSatoshis: number;
  unconfirmed: number;
  unconfirmedSatoshis: number;
  totalReceived: number;
  totalReceivedSatoshis: number;
  totalSent: number;
  totalSentSatoshis: number;
  transactionCount: number;
  lastActivity?: string;
}

// UTXO Types
export interface UTXO {
  txid: string;
  vout: number;
  value: number; // In satoshis
  scriptPubkey: string;
  address: string;
  confirmations: number;
}

// Transaction Types
export interface TransactionInput {
  previousTxHash: string;
  outputIndex: number;
  scriptSig?: string;
  sequence?: number;
}

export interface TransactionOutput {
  value: number; // In satoshis
  scriptPubkey: string;
  address: string;
}

export interface Transaction {
  version: number;
  inputs: TransactionInput[];
  outputs: TransactionOutput[];
  locktime: number;
  timestamp: string;
}

export interface SignedTransaction {
  transaction: string; // JSON string
  signature: string; // Base64
  txHash: string;
  signerKey: string;
  algorithm: string;
  header: {
    alg: string;
    typ?: string;
  };
}

// Wallet Types
export interface WalletInfo {
  name: string;
  address: string;
  version: number;
  createdAt: string;
}

export interface CreateWalletOptions {
  name?: string;
  seed?: Uint8Array; // 64-byte seed for ML-DSA-87
}

// Transaction Building Types
export interface SendTransactionOptions {
  to: string;
  amount: number; // In SHADOW
  fee?: number; // In SHADOW, defaults to 0.011
  token?: string; // Token ID, defaults to 'SHADOW'
}

export interface TransactionResult {
  txHash: string;
  signature: string;
  rawTransaction: string;
  status: 'signed' | 'broadcast' | 'confirmed' | 'failed';
  message?: string;
}

// Error Types
export interface ShadowyError {
  code: string;
  message: string;
  details?: any;
}

// Events
export interface ShadowyEvents {
  'wallet:created': (wallet: WalletInfo) => void;
  'wallet:loaded': (wallet: WalletInfo) => void;
  'transaction:signed': (result: TransactionResult) => void;
  'transaction:broadcast': (result: TransactionResult) => void;
  'transaction:confirmed': (result: TransactionResult) => void;
  'error': (error: ShadowyError) => void;
  [key: string]: (...args: any[]) => void;
}

// Provider Types
export interface ShadowyProvider {
  // Network
  getNodeInfo(): Promise<NodeInfo>;
  getBalance(address: string): Promise<Balance>;
  getUTXOs(address: string): Promise<UTXO[]>;
  
  // Transactions
  broadcastTransaction(signedTx: SignedTransaction): Promise<TransactionResult>;
  
  // Address utilities
  isValidAddress(address: string): boolean;
}

// Web3 Instance Configuration
export interface ShadowyWeb3Config {
  // Node connection (optional - can work offline for wallet operations)
  node?: NodeConfig;
  
  // WASM configuration
  wasmUrl?: string; // Path to shadowy.wasm
  
  // Wallet storage (browser-based)
  storage?: 'localStorage' | 'sessionStorage' | 'memory';
  
  // Network (testnet, mainnet, etc.)
  network?: string;
}

// Utility Types
export type NetworkType = 'mainnet' | 'testnet' | 'devnet' | 'local';
export type CryptoAlgorithm = 'ML-DSA-87';

// Constants
export const SATOSHIS_PER_SHADOW = 100_000_000;
export const DEFAULT_FEE_SHADOW = 0.011;
export const CRYPTO_ALGORITHM: CryptoAlgorithm = 'ML-DSA-87';
export const ADDRESS_LENGTH = {
  S_ADDRESS: 51,
  L_ADDRESS: 41
} as const;