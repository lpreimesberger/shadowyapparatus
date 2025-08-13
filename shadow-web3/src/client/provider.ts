/**
 * Shadowy Network Provider
 * Handles communication with Shadowy nodes
 */

import { 
  NodeConfig, 
  NodeInfo, 
  Balance, 
  UTXO, 
  SignedTransaction, 
  TransactionResult, 
  ShadowyProvider 
} from '../types';
import { AddressUtils, HttpUtils } from '../utils';

export class ShadowyNetworkProvider implements ShadowyProvider {
  private config: NodeConfig;

  constructor(config: NodeConfig) {
    this.config = {
      timeout: 10000,
      ...config
    };
  }

  /**
   * Get node information
   */
  async getNodeInfo(): Promise<NodeInfo> {
    const url = `${this.config.url}/api/v1/status`;
    const headers = this.getHeaders();
    
    try {
      const response = await HttpUtils.get(url, headers);
      return {
        tipHeight: response.tip_height || 0,
        totalBlocks: response.total_blocks || 0,
        totalTransactions: response.total_transactions || 0,
        status: response.status || 'unknown',
        version: response.version
      };
    } catch (error) {
      throw new Error(`Failed to get node info: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Get balance for an address
   */
  async getBalance(address: string): Promise<Balance> {
    if (!AddressUtils.isValidAddress(address)) {
      throw new Error(`Invalid address format: ${address}`);
    }

    const url = `${this.config.url}/api/v1/address/${address}/balance`;
    const headers = this.getHeaders();
    
    try {
      const response = await HttpUtils.get(url, headers);
      return {
        address: response.address,
        balance: response.balance || 0,
        balanceSatoshis: response.balance_satoshis || 0,
        confirmed: response.confirmed || 0,
        confirmedSatoshis: response.confirmed_satoshis || 0,
        unconfirmed: response.unconfirmed || 0,
        unconfirmedSatoshis: response.unconfirmed_satoshis || 0,
        totalReceived: response.total_received || 0,
        totalReceivedSatoshis: response.total_received_satoshis || 0,
        totalSent: response.total_sent || 0,
        totalSentSatoshis: response.total_sent_satoshis || 0,
        transactionCount: response.transaction_count || 0,
        lastActivity: response.last_activity
      };
    } catch (error) {
      throw new Error(`Failed to get balance: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Get UTXOs for an address
   */
  async getUTXOs(address: string): Promise<UTXO[]> {
    if (!AddressUtils.isValidAddress(address)) {
      throw new Error(`Invalid address format: ${address}`);
    }

    const url = `${this.config.url}/api/v1/utxos?address=${encodeURIComponent(address)}`;
    const headers = this.getHeaders();
    
    try {
      const response = await HttpUtils.get(url, headers);
      
      if (!Array.isArray(response)) {
        throw new Error('Invalid UTXO response format');
      }

      return response.map(utxo => ({
        txid: utxo.txid,
        vout: utxo.vout,
        value: utxo.value,
        scriptPubkey: utxo.script_pubkey || utxo.scriptPubkey || '',
        address: utxo.address,
        confirmations: utxo.confirmations || 0
      }));
    } catch (error) {
      throw new Error(`Failed to get UTXOs: ${error instanceof Error ? error.message : 'Unknown error'}`);
    }
  }

  /**
   * Broadcast a signed transaction
   */
  async broadcastTransaction(signedTx: SignedTransaction): Promise<TransactionResult> {
    const url = `${this.config.url}/api/v1/mempool/transactions`;
    const headers = this.getHeaders();
    
    try {
      // Parse transaction JSON if it's a string
      let transaction;
      try {
        transaction = typeof signedTx.transaction === 'string' 
          ? JSON.parse(signedTx.transaction) 
          : signedTx.transaction;
      } catch (error) {
        throw new Error('Invalid transaction format');
      }

      const payload = {
        transaction,
        signature: signedTx.signature,
        tx_hash: signedTx.txHash,
        signer_key: signedTx.signerKey,
        algorithm: signedTx.algorithm,
        header: signedTx.header
      };

      const response = await HttpUtils.post(url, payload, headers);

      return {
        txHash: signedTx.txHash,
        signature: signedTx.signature,
        rawTransaction: JSON.stringify(transaction),
        status: response.status === 'accepted' ? 'broadcast' : 'failed',
        message: response.message || 'Transaction broadcast'
      };
    } catch (error) {
      return {
        txHash: signedTx.txHash,
        signature: signedTx.signature,
        rawTransaction: signedTx.transaction,
        status: 'failed',
        message: `Broadcast failed: ${error instanceof Error ? error.message : 'Unknown error'}`
      };
    }
  }

  /**
   * Test connection to node
   */
  async testConnection(): Promise<boolean> {
    try {
      const url = `${this.config.url}/api/v1/health`;
      const headers = this.getHeaders();
      
      await HttpUtils.get(url, headers);
      return true;
    } catch (error) {
      return false;
    }
  }

  /**
   * Validate address format
   */
  isValidAddress(address: string): boolean {
    return AddressUtils.isValidAddress(address);
  }

  /**
   * Get request headers including API key if configured
   */
  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json'
    };

    if (this.config.apiKey) {
      headers['Authorization'] = `Bearer ${this.config.apiKey}`;
    }

    return headers;
  }

  /**
   * Update node configuration
   */
  updateConfig(config: Partial<NodeConfig>): void {
    this.config = { ...this.config, ...config };
  }

  /**
   * Get current configuration
   */
  getConfig(): NodeConfig {
    return { ...this.config };
  }
}