package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
)

// Block of the chain
type Block struct {
	Transactions []*CoinTransaction
	PrevHash []byte
	Hash     []byte
	Nonce    int
}

// HashTransaction hashes combined transactions
func (b *Block) HashTransaction() []byte {
	var txHashes [][]byte
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Serialize())
	}
	tree := NewMerkleTree(txHashes)

	return tree.RootNode.Data
}

// CreateBlock creates new Block on the blockchain
func CreateBlock(txns []*CoinTransaction, prevHash []byte) *Block {
	block := &Block{
		Transactions: txns,
		PrevHash: prevHash,
		Hash:     []byte{},
	}
	pow := NewProof(block)
	nonce, hash := pow.Run()
	block.Hash = hash[:]
	block.Nonce = nonce
	return block
}

// Serialize a block
func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)
	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}
	return res.Bytes()
}

// Deserialize a block
func Deserialize(data []byte) *Block {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&block)
	if err != nil {
		log.Panic(err)
	}
	return &block
}

