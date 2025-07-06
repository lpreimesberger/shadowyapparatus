package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func createValidSignedTransaction() *SignedTransaction {
	tx := Transaction{
		Version: 1,
		Inputs: []TransactionInput{
			{
				PreviousTxHash: "abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678",
				OutputIndex:    0,
			},
		},
		Outputs: []TransactionOutput{
			{
				Value:        1000,
				ScriptPubKey: "OP_DUP OP_HASH160 S42618a7524a82df51c8a2406321e161de65073008806f042f0 OP_EQUALVERIFY OP_CHECKSIG",
				Address:      "S42618a7524a82df51c8a2406321e161de65073008806f042f0",
			},
		},
		NotUntil:  time.Now().UTC().Add(-time.Hour),
		Timestamp: time.Now().UTC(),
		Nonce:     1234567890,
	}

	txData, _ := json.Marshal(tx)
	
	validSignature := strings.Repeat("a", SignatureSize*2)

	return &SignedTransaction{
		Transaction: txData,
		Signature:   validSignature,
		TxHash:      "valid_transaction_hash_12345",
		SignerKey:   "valid_signer_key_data",
		Algorithm:   "ML-DSA-87",
		Header: JOSEHeader{
			Algorithm: "ML-DSA-87",
			Type:      "shadowy-tx",
		},
	}
}

func TestBasicTransactionValidator(t *testing.T) {
	validator := &BasicTransactionValidator{}

	if validator.Name() != "BasicTransactionValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidTransaction", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Valid transaction should pass validation: %v", err)
		}
	})

	t.Run("NilTransaction", func(t *testing.T) {
		err := validator.ValidateTransaction(nil)
		if err == nil {
			t.Error("Nil transaction should fail validation")
		}
	})

	t.Run("EmptyTxHash", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.TxHash = ""
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with empty hash should fail validation")
		}
	})

	t.Run("UnsupportedAlgorithm", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Algorithm = "SHA256"
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with unsupported algorithm should fail validation")
		}
	})

	t.Run("EmptySignature", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Signature = ""
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with empty signature should fail validation")
		}
	})

	t.Run("EmptySignerKey", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.SignerKey = ""
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with empty signer key should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with invalid JSON should fail validation")
		}
	})
}

func TestSignatureValidator(t *testing.T) {
	validator := &SignatureValidator{}

	if validator.Name() != "SignatureValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidSignature", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Valid signature should pass validation: %v", err)
		}
	})

	t.Run("EmptySignature", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Signature = ""
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Empty signature should fail validation")
		}
	})

	t.Run("IncorrectSignatureLength", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Signature = "too_short"
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Signature with incorrect length should fail validation")
		}
	})

	t.Run("InvalidHexCharacters", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Signature = strings.Repeat("z", SignatureSize*2) // 'z' is not valid hex
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Signature with invalid hex characters should fail validation")
		}
	})
}

func TestTemporalValidator(t *testing.T) {
	validator := &TemporalValidator{}

	if validator.Name() != "TemporalValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidTiming", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction with valid timing should pass: %v", err)
		}
	})

	t.Run("NotUntilInFuture", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.NotUntil = time.Now().UTC().Add(time.Hour) // Future time
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with future not_until should fail validation")
		}
	})

	t.Run("TimestampTooFarInFuture", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Timestamp = time.Now().UTC().Add(15 * time.Minute) // Too far in future
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with timestamp too far in future should fail validation")
		}
	})

	t.Run("TimestampTooOld", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Timestamp = time.Now().UTC().Add(-25 * time.Hour) // Too old
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with timestamp too old should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestFeeValidator(t *testing.T) {
	minFee := uint64(10)
	validator := &FeeValidator{MinFee: minFee}

	if validator.Name() != "FeeValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidFee", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction with valid fee should pass: %v", err)
		}
	})

	t.Run("ZeroValueOutput", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Outputs[0].Value = 0
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with zero value output should fail validation")
		}
	})

	t.Run("OutputValueBelowMinFee", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Outputs[0].Value = minFee - 1
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with output value below min fee should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestSizeValidator(t *testing.T) {
	maxSize := 50000 // Increased to accommodate ML-DSA-87 signatures
	validator := &SizeValidator{MaxTxSize: maxSize}

	if validator.Name() != "SizeValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidSize", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction with valid size should pass: %v", err)
		}
	})

	t.Run("TooManyInputs", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		
		for i := 0; i < 1001; i++ {
			parsedTx.Inputs = append(parsedTx.Inputs, TransactionInput{
				PreviousTxHash: "hash" + string(rune(i)),
				OutputIndex:    uint32(i),
			})
		}
		
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with too many inputs should fail validation")
		}
	})

	t.Run("TooManyOutputs", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		
		for i := 0; i < 1001; i++ {
			parsedTx.Outputs = append(parsedTx.Outputs, TransactionOutput{
				Value:   100,
				Address: "S42618a7524a82df51c8a2406321e161de65073008806f042f0",
			})
		}
		
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with too many outputs should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestDoubleSpendValidator(t *testing.T) {
	validator := &DoubleSpendValidator{}

	if validator.Name() != "DoubleSpendValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidTransaction", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Valid transaction should pass: %v", err)
		}
	})

	t.Run("DuplicateInputs", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		
		parsedTx.Inputs = append(parsedTx.Inputs, TransactionInput{
			PreviousTxHash: "abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678abcd1234efgh5678", // Same as first input
			OutputIndex:    0,                   // Same as first input
		})
		
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with duplicate inputs should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestAddressValidator(t *testing.T) {
	validator := &AddressValidator{}

	if validator.Name() != "AddressValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidAddress", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction with valid address should pass: %v", err)
		}
	})

	t.Run("IncorrectAddressLength", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Outputs[0].Address = "short_address"
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with incorrect address length should fail validation")
		}
	})

	t.Run("AddressNotStartingWithS", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Outputs[0].Address = "X42618a7524a82df51c8a2406321e161de65073008806f042f0"
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with address not starting with 'S' should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestNonceValidator(t *testing.T) {
	validator := NewNonceValidator()

	if validator.Name() != "NonceValidator" {
		t.Error("Incorrect validator name")
	}

	t.Run("ValidNonce", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := validator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction with valid nonce should pass: %v", err)
		}
	})

	t.Run("ZeroNonce", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Nonce = 0
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with zero nonce should fail validation")
		}
	})

	t.Run("TooLargeNonce", func(t *testing.T) {
		tx := createValidSignedTransaction()
		
		parsedTx := Transaction{}
		json.Unmarshal(tx.Transaction, &parsedTx)
		parsedTx.Nonce = uint64(1<<63) // Too large
		txData, _ := json.Marshal(parsedTx)
		tx.Transaction = txData

		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction with too large nonce should fail validation")
		}
	})

	t.Run("InvalidTransactionData", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.Transaction = []byte("invalid json")
		err := validator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Invalid transaction data should fail validation")
		}
	})
}

func TestCompositeValidator(t *testing.T) {
	basicValidator := &BasicTransactionValidator{}
	signatureValidator := &SignatureValidator{}
	
	compositeValidator := NewCompositeValidator("TestComposite", basicValidator, signatureValidator)

	if compositeValidator.Name() != "TestComposite" {
		t.Error("Incorrect composite validator name")
	}

	t.Run("AllValidatorsPass", func(t *testing.T) {
		tx := createValidSignedTransaction()
		err := compositeValidator.ValidateTransaction(tx)
		if err != nil {
			t.Errorf("Transaction should pass when all validators pass: %v", err)
		}
	})

	t.Run("OneValidatorFails", func(t *testing.T) {
		tx := createValidSignedTransaction()
		tx.TxHash = "" // This will cause BasicTransactionValidator to fail
		
		err := compositeValidator.ValidateTransaction(tx)
		if err == nil {
			t.Error("Transaction should fail when any validator fails")
		}

		if !strings.Contains(err.Error(), "BasicTransactionValidator") {
			t.Error("Error should indicate which validator failed")
		}
	})
}

func BenchmarkBasicTransactionValidator(b *testing.B) {
	validator := &BasicTransactionValidator{}
	tx := createValidSignedTransaction()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateTransaction(tx)
	}
}

func BenchmarkSignatureValidator(b *testing.B) {
	validator := &SignatureValidator{}
	tx := createValidSignedTransaction()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateTransaction(tx)
	}
}

func BenchmarkCompositeValidator(b *testing.B) {
	validator := NewCompositeValidator("Benchmark",
		&BasicTransactionValidator{},
		&SignatureValidator{},
		&TemporalValidator{},
		&FeeValidator{MinFee: 1},
	)
	tx := createValidSignedTransaction()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateTransaction(tx)
	}
}