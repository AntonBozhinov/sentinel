package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

// CoinTransaction in the blockchain
type CoinTransaction struct {
	ID      []byte
	Inputs  []CoinTxInput
	Outputs []CoinTxOutput
}


// InitCoinTransaction initialize a coin transaction
func InitCoinTransaction(to, data string) *CoinTransaction {
	if data == "" {
		data = fmt.Sprintf("Coins to %s", to)
	}
	txin := CoinTxInput{[]byte{}, -1, data}
	txout := CoinTxOutput{100, to}

	tx := CoinTransaction{
		ID:      nil,
		Inputs:  []CoinTxInput{txin},
		Outputs: []CoinTxOutput{txout},
	}
	tx.SetID()
	return &tx
}

// SetID calculates a transaction id
func (txn *CoinTransaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte
	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(txn)
	if err != nil {
		log.Fatalf("error encoding transaction id: %v", err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	txn.ID = hash[:]
}

// IsCoinTransaction validates initial state of coin transaction
func (txn *CoinTransaction) IsCoinTransaction() bool {
	return len(txn.Inputs) == 1 && len(txn.Inputs[0].ID) == 0 && txn.Inputs[0].Out == -1
}

// NewTransaction creates new transaction
func NewTransaction(from, to string, amount int, chain *BlockChain) *CoinTransaction {
	var inputs []CoinTxInput
	var outputs []CoinTxOutput

	acc, validOutputs := chain.FindSpendableTransactions(from, amount)
	if acc < amount {
		log.Panic("Error: not enough funds")
	}
	for txid, outs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panicf("error decoding a transaction id: %v", err)
		}
		for _, out := range outs {
			input := CoinTxInput{
				ID:  txID,
				Out: out,
				Sig: from,
			}
			inputs = append(inputs, input)
		}
	}

	outputs = append(outputs, CoinTxOutput{amount, to})
	if acc > amount {
		outputs = append(outputs, CoinTxOutput{acc - amount, from})
	}

	tx := CoinTransaction{
		ID: nil,
		Inputs: inputs, 
		Outputs: outputs,
	}
	tx.SetID()
	return &tx
}
