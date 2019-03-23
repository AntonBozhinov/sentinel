package blockchain

import (
	"fmt"
	"log"
	"github.com/dgraph-io/badger"
)

const (
	dbPath = "./tmp/blocks"
	lastHashKey = "lh"
)

//BlockChain storage
type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

// Iterator is a helper structure for
// blockchain iteration
type Iterator struct {
	CurrentHash []byte
	Database *badger.DB
}

// AddBlock to the blockchain
func (chain *BlockChain) AddBlock(data []byte) {
	var lastHash []byte
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(lastHashKey))
		if (err != nil) {
			log.Fatalf("error setting the genesis hash: %v", err)
		}
		lastHash, err = item.Value()
		return err
	})
	if (err != nil) {
		log.Fatalf("error adding a new block: %v", err)
	}
	newBlock := CreateBlock(data, lastHash)
	err = chain.Database.Update(func(txn*badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		if (err != nil) {
			log.Fatalf("error saving the new block: %v", err)
		}
		err = txn.Set([]byte(lastHashKey), newBlock.Hash)
		chain.LastHash = lastHash
		return err;
	})
	if (err != nil) {
		log.Fatalf("could not add new block: %v", err)
	}
}

// Genesis block of the blockchain
func Genesis() *Block {
	return CreateBlock([]byte("Genesis"), []byte{})
}

// InitBlockChain with persistance
func InitBlockChain() *BlockChain {
	var lastHash []byte
	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath
	db, err := badger.Open(opts)
	// TODO: better error handling
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get([]byte(lastHashKey)); err == badger.ErrKeyNotFound {
			fmt.Println("No existing blockchain found")
			genesis := Genesis()
			fmt.Println("Genesis created")
			err = txn.Set(genesis.Hash, genesis.Serialize())
			if (err != nil) {
				log.Fatalf("error setting the genesis hash: %v", err)
			}
			err = txn.Set([]byte(lastHashKey), genesis.Hash)
			if (err != nil) {
				log.Fatalf("error setting the last hash key: %v", err)
			}
			lastHash = genesis.Hash
			return err
		}
		item, err := txn.Get([]byte(lastHashKey))
		if (err != nil) {
			log.Fatalf("error setting the last hash: %v", err)
		}
		lastHash, err = item.Value()
		return err
	})
	if (err != nil) {
		log.Fatalf("error getting the last hash: %v", err)
	}
	blockchain := &BlockChain{
		LastHash: lastHash,
		Database: db,
	}

	return blockchain
}

// Iterator returns a blockchain iterator
func (chain *BlockChain) Iterator() *Iterator {
	iter := &Iterator{chain.LastHash, chain.Database}
	return iter
}

// Next gets the next block in iteration
func (iter *Iterator) Next() *Block {
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		if (err != nil) {
			log.Fatalf("error with iteration on hash: %x\n%v", iter.CurrentHash, err)
		}
		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)
		return err
	})
	if (err != nil) {
		log.Fatalf("error getting data from hash: %x\n%v", iter.CurrentHash, err)
	}
	iter.CurrentHash = block.PrevHash
	return block
}
