package blockchain

import (
	"bytes"
	"encoding/gob"
	"github.com/AntonBozhinov/sentinel/wallet"
	"log"
)

// CoinTxInput is the transaction intput
type 	CoinTxInput struct {
	ID  []byte
	Out int
	Signature []byte
	PubKey []byte
}

// CoinTxOutput is the transaction output
type CoinTxOutput struct {
	Value  int
	PubKeyHash []byte
}

type CoinTxOutputs struct {
	Outputs []CoinTxOutput
}

func (in *CoinTxInput) UsesKey(pubKeyHash []byte) bool {
	lockingHash := wallet.PublicKeyHash(in.PubKey)
	return bytes.Compare(lockingHash, pubKeyHash) == 0
}

func (out *CoinTxOutput) Lock(address []byte) {
	pubKeyHash := wallet.Base58Decode(address)

	pubKeyHash = pubKeyHash[1: len(pubKeyHash) - wallet.ChecksumLength]
	out.PubKeyHash = pubKeyHash
}
func (out *CoinTxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	return bytes.Compare(out.PubKeyHash, pubKeyHash) == 0
}


func NewCoinTxOutput(value int, address string) *CoinTxOutput {
	txo := &CoinTxOutput{
		Value: value,
		PubKeyHash: nil,
	}
	txo.Lock([]byte(address))
	return txo
}

func (outs CoinTxOutputs) Serialize() []byte {
	var buffer bytes.Buffer
	encode := gob.NewEncoder(&buffer)
	err := encode.Encode(outs)
	if err != nil {
		log.Panicf("error serializing the transaction buffer: %v", err)
	}
	return buffer.Bytes()
}

func DeserializeOutputs(data []byte) CoinTxOutputs {
	var outputs CoinTxOutputs
	decode := gob.NewDecoder(bytes.NewReader(data))
	err := decode.Decode(&outputs)
	if err != nil {
		log.Panicf("error deserializing transaction outputs: %v", err)
	}
	return outputs
}
