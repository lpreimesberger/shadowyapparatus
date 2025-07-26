package main

const testnet0 = "7e8b843f4620d7cd93232ccb4bbd16c9d8c7904a7dd039fbff30c0f7c455c288"

func hash2chain(thisHash string) string {
	switch thisHash {
	case testnet0:
		return "testnet0"
	default:
		return thisHash
	}

}

// testnet0
const activeGenesis = `
{
  "header": {
    "version": 1,
    "previous_block_hash": "0000000000000000000000000000000000000000000000000000000000000000",
    "merkle_root": "5a93041a7e64347670074fe54e68812267ba45d32b7dd2d03a7a4d14f4d94166",
    "timestamp": "2025-07-23T21:48:54.690434478Z",
    "height": 0,
    "nonce": 0,
    "challenge_seed": "genesis_challenge",
    "proof_hash": "genesis_proof",
    "farmer_address": "genesis_farmer"
  },
  "body": {
    "transactions": [
      {
        "transaction": {
          "version": 1,
          "inputs": [],
          "outputs": [
            {
              "value": 100000000,
              "script_pubkey": "",
              "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
            }
          ],
          "not_until": "2025-07-23T21:48:54.690434478Z",
          "timestamp": "2025-07-23T21:48:54.690434478Z",
          "nonce": 0
        },
        "signature": "genesis_signature",
        "tx_hash": "5a93041a7e64347670074fe54e68812267ba45d32b7dd2d03a7a4d14f4d94166",
        "signer_key": "genesis_signer",
        "algorithm": "genesis",
        "header": {
          "alg": "genesis",
          "typ": "JWT"
        }
      }
    ],
    "tx_count": 1
  },
  "genesis_timestamp": "2025-07-23T21:48:54.690434478Z",
  "network_id": "shadowy-mainnet",
  "initial_supply": 100000000
}
`
