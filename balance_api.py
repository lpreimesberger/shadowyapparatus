#!/usr/bin/env python3

from http.server import HTTPServer, BaseHTTPRequestHandler
import json
import requests
import base64
import urllib.parse
import sys

class BalanceHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        # Parse the path to extract address
        print(f"Received request: {self.path}")
        parts = self.path.split('/')
        print(f"Parts: {parts}")
        
        if self.path.startswith('/api/v1/address/'):
            if len(parts) >= 5 and parts[4] == 'balance':
                address = parts[3]
                print(f"Extracted address: {address}")
                self.handle_balance_request(address)
            else:
                print(f"Invalid path structure, parts length: {len(parts)}")
                self.send_404()
        else:
            print("Path doesn't start with /api/v1/address/")
            self.send_404()
    
    def handle_balance_request(self, address):
        try:
            balance_info = self.calculate_balance(address)
            
            # Send response
            self.send_response(200)
            self.send_header('Content-Type', 'application/json')
            self.send_header('Access-Control-Allow-Origin', '*')
            self.end_headers()
            
            response = {
                "address": address,
                "balance": balance_info["balance"] / 100000000.0,  # Convert to SHADOW
                "balance_satoshis": balance_info["balance"],
                "confirmed": balance_info["balance"] / 100000000.0,
                "confirmed_satoshis": balance_info["balance"], 
                "unconfirmed": 0.0,
                "unconfirmed_satoshis": 0,
                "total_received": balance_info["balance"] / 100000000.0,
                "total_received_satoshis": balance_info["balance"],
                "total_sent": 0.0,
                "total_sent_satoshis": 0,
                "transaction_count": balance_info["mining_rewards"],
                "last_activity": ""
            }
            
            self.wfile.write(json.dumps(response).encode())
            
        except Exception as e:
            print(f"Error calculating balance: {e}")
            self.send_error(500, f"Error calculating balance: {str(e)}")
    
    def calculate_balance(self, address):
        """Calculate balance by scanning Tendermint blocks"""
        try:
            # Get latest height
            resp = requests.get('http://localhost:26657/status')
            data = resp.json()
            latest_height = int(data['result']['sync_info']['latest_block_height'])
            
            balance = 0
            mining_rewards = 0
            
            # Scan all blocks (optimize later if needed)
            for height in range(1, latest_height + 1):
                try:
                    resp = requests.get(f'http://localhost:26657/block?height={height}')
                    block_data = resp.json()
                    txs = block_data['result']['block']['data']['txs']
                    
                    for tx_b64 in txs:
                        # Decode signed transaction
                        tx_bytes = base64.b64decode(tx_b64)
                        signed_tx = json.loads(tx_bytes)
                        
                        # Check if coinbase to our address
                        if (signed_tx.get('algorithm') == 'coinbase' and 
                            signed_tx.get('signer_key') == address):
                            
                            # Decode inner transaction
                            inner_bytes = base64.b64decode(signed_tx['transaction'])
                            inner_tx = json.loads(inner_bytes)
                            
                            # Sum outputs
                            for output in inner_tx.get('outputs', []):
                                if output.get('address') == address:
                                    balance += output.get('value', 0)
                                    mining_rewards += 1
                                    
                except Exception as e:
                    print(f"Error processing block {height}: {e}")
                    continue
                    
            return {
                "balance": balance,
                "mining_rewards": mining_rewards
            }
            
        except Exception as e:
            raise Exception(f"Failed to calculate balance: {str(e)}")
    
    def send_404(self):
        self.send_response(404)
        self.send_header('Content-Type', 'text/plain')
        self.end_headers()
        self.wfile.write(b'Not Found')

if __name__ == '__main__':
    port = 8082  # Different port to avoid conflicts
    server = HTTPServer(('localhost', port), BalanceHandler)
    print(f"🌐 Balance API server starting on http://localhost:{port}")
    print(f"📊 Example: http://localhost:{port}/api/v1/address/S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737/balance")
    server.serve_forever()