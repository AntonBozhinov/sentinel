package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"github.com/AntonBozhinov/sentinel/wallet"
	"golang.org/x/tools/container/intsets"
	"log"
	"math/big"
	"strings"
)

type CoinTransaction struct {
	ID      []byte
	Inputs  []CoinTxInput
	Outputs []CoinTxOutput
}


func FirstCoinTransaction(to, data string) *CoinTransaction {
	if data == "" {
		data = fmt.Sprintf("Coins to %s", to)
	}
	txIn := CoinTxInput{[]byte{}, -1,nil, []byte(data)}
	txOut := NewCoinTxOutput(intsets.MaxInt - 1, to)

	tx := CoinTransaction{
		ID:      nil,
		Inputs:  []CoinTxInput{txIn},
		Outputs: []CoinTxOutput{*txOut},
	}
	tx.SetID()
	return &tx
}

func (txn CoinTransaction) Serialize() []byte {
	var encoded bytes.Buffer
	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(txn)
	if err != nil {
		log.Panicf("error encoding a transaction: %v", err)
	}
	return encoded.Bytes()
}

func (txn *CoinTransaction) Hash() []byte {
	var hash [32]byte
	txCopy := *txn
	txCopy.ID = []byte{}
	hash = sha256.Sum256(txCopy.Serialize())
	return hash[:]
}



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

func (txn *CoinTransaction) IsCoinTransaction() bool {
	return len(txn.Inputs) == 1 && len(txn.Inputs[0].ID) == 0 && txn.Inputs[0].Out == -1
}

func (txn CoinTransaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]CoinTransaction) {
	if txn.IsCoinTransaction() {
		return
	}

	for _, in := range txn.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction is not correct")
		}
	}
	txCopy := txn.TrimmedCopy()

	for inId, in := range txCopy.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash
		txCopy.ID  = txCopy.Hash()
		txCopy.Inputs[inId].PubKey = nil
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, txCopy.ID)
		if err != nil {
			log.Panicf("error signing a transaction: %v", err)
		}
		signature := append(r.Bytes(), s.Bytes()...)
		txn.Inputs[inId].Signature = signature
	}

}

func (txn *CoinTransaction) TrimmedCopy() CoinTransaction {
	var inputs []CoinTxInput
	var outputs []CoinTxOutput

	for _, in := range txn.Inputs {
		inputs = append(inputs, CoinTxInput{in.ID, in.Out, nil, nil})
	}

	for _, out := range txn.Outputs {
		outputs = append(outputs, CoinTxOutput{out.Value, out.PubKeyHash})
	}
	txCopy := CoinTransaction{txn.ID, inputs, outputs}
	return txCopy
}

func (txn CoinTransaction) Verify(prevTXs map[string]CoinTransaction) bool {
	if txn.IsCoinTransaction() {
		return true
	}
	for _, in := range txn.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("error on transaction verification: Previous transaction does not exist")
		}
	}

	txCopy := txn.TrimmedCopy()
	curve := elliptic.P256()

	for inId, in := range txn.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		txCopy.Inputs[inId].Signature = nil
		txCopy.Inputs[inId].PubKey = prevTX.Outputs[in.Out].PubKeyHash
		txCopy.ID  = txCopy.Hash()
		txCopy.Inputs[inId].PubKey = nil

		r := big.Int{}
		s := big.Int{}
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		if ecdsa.Verify(&rawPubKey, txCopy.ID, &r, &s) == false {
			return  false
		}
	}
	return true
}

func (txn CoinTransaction) String() string {
	var lines []string
	lines = append(lines, fmt.Sprintf("CoinTransaction: %x", txn.ID))
	for i, in := range txn.Inputs {
		lines = append(lines, fmt.Sprintf("	Input: %d", i))
		lines = append(lines, fmt.Sprintf("		TXID: %x", in.ID))
		lines = append(lines, fmt.Sprintf("		Out: %d", in.Out))
		lines = append(lines, fmt.Sprintf("		Signature: %x", in.Signature))
		lines = append(lines, fmt.Sprintf("		PubKey: %x", in.PubKey))
	}

	for i, out := range txn.Outputs {
		lines = append(lines, fmt.Sprintf("	Output: %d", i))
		lines = append(lines, fmt.Sprintf("		Value: %d", out.Value))
		lines = append(lines, fmt.Sprintf("		Script: %x", out.PubKeyHash))
	}
	return strings.Join(lines, "\n")
}

func NewTransaction(from, to string, amount int, UTXO *UTXOSet) *CoinTransaction {
	var inputs []CoinTxInput
	var outputs []CoinTxOutput

	wallets, err := wallet.CreateWallets()
	if err != nil {
		log.Panicf("error fetching wallets: %v", err)
	}

	w := wallets.GetWallet(from)

	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)

	acc, validOutputs := UTXO.FindSpendableTransactions(pubKeyHash, amount)
	if acc < amount {
		log.Panic("error: not enough funds")
	}
	for txId, outs := range validOutputs {
		txID, err := hex.DecodeString(txId)
		if err != nil {
			log.Panicf("error decoding a transaction id: %v", err)
		}
		for _, out := range outs {
			input := CoinTxInput{
				ID:  txID,
				Out: out,
				Signature: nil,
				PubKey: w.PublicKey,
			}
			inputs = append(inputs, input)
		}
	}
	outputs = append(outputs, *NewCoinTxOutput(amount, to))
	if acc > amount {
		outputs = append(outputs, *NewCoinTxOutput(acc - amount, from))
	}

	tx := CoinTransaction{
		ID: nil,
		Inputs: inputs, 
		Outputs: outputs,
	}
	tx.ID = tx.Hash()
	UTXO.BlockChain.SignTransaction(&tx, w.PrivateKey)
	return &tx
}
