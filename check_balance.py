#!/usr/bin/env python3

import requests
import base64
import json

def get_latest_height():
    """Get the latest block height from Tendermint"""
    resp = requests.get('http://localhost:26657/status')
    data = resp.json()
    return int(data['result']['sync_info']['latest_block_height'])

def get_block(height):
    """Get a block from Tendermint"""
    resp = requests.get(f'http://localhost:26657/block?height={height}')
    return resp.json()

def decode_transaction(tx_b64):
    """Decode a base64-encoded transaction"""
    tx_bytes = base64.b64decode(tx_b64)
    return json.loads(tx_bytes)

def decode_inner_transaction(inner_tx_b64):
    """Decode the inner transaction from coinbase"""
    inner_bytes = base64.b64decode(inner_tx_b64)
    return json.loads(inner_bytes)

def calculate_balance(address):
    """Calculate balance for an address by scanning all blocks"""
    latest_height = get_latest_height()
    print(f"📊 Scanning {latest_height} blocks for address {address}")
    
    balance = 0
    mining_rewards = 0
    
    for height in range(1, latest_height + 1):
        try:
            block_data = get_block(height)
            txs = block_data['result']['block']['data']['txs']
            
            if not txs:
                continue
                
            for tx_b64 in txs:
                # Decode the signed transaction
                signed_tx = decode_transaction(tx_b64)
                
                # Check if it's a coinbase transaction to our address
                if signed_tx.get('algorithm') == 'coinbase' and signed_tx.get('signer_key') == address:
                    # Decode the inner transaction
                    inner_tx = decode_inner_transaction(signed_tx['transaction'])
                    
                    # Sum up outputs to our address
                    for output in inner_tx.get('outputs', []):
                        if output.get('address') == address:
                            reward_amount = output.get('value', 0)
                            balance += reward_amount
                            mining_rewards += 1
                            print(f"  🪙 Block {height}: +{reward_amount/100000000:.8f} SHADOW")
                            
        except Exception as e:
            print(f"❌ Error processing block {height}: {e}")
            continue
    
    print(f"\n💰 Final Balance for {address}:")
    print(f"   Balance: {balance/100000000:.8f} SHADOW ({balance:,} satoshis)")
    print(f"   Mining rewards: {mining_rewards} blocks")
    print(f"   Average reward: {(balance/mining_rewards/100000000):.8f} SHADOW per block" if mining_rewards > 0 else "")
    
    return balance

if __name__ == '__main__':
    # The mining address we're checking
    address = 'S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737'
    calculate_balance(address)