package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/pkg/errors"
	"log"
	"os"
	"runtime"
)

const (
	dbPath = "./tmp/blocks"
	dbFile = "./tmp/blocks/MANIFEST"
	lastHashKey = "lh"
	genesisData = "First transaction from Genesis"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type Iterator struct {
	CurrentHash []byte
	Database *badger.DB
}

func (bc *BlockChain) FindTransaction(ID []byte) (CoinTransaction, error) {
	iter := bc.Iterator()
	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return CoinTransaction{}, errors.New("transaction does not exists")
}

func (bc *BlockChain) SignTransaction(tx *CoinTransaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]CoinTransaction)

	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		if err != nil {
			log.Panicf("can not find a transaction with ID: %x", in.ID)
		}
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

func (bc *BlockChain) VerifyTransaction(tx *CoinTransaction) bool {
	prevTXs := make(map[string]CoinTransaction)

	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		if err != nil {
			log.Panicf("can not find a transaction with ID: %x", in.ID)
		}

		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}

func hasDB() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

func (chain *BlockChain) AddBlock(data []*CoinTransaction) *Block {
	var lastHash []byte
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(lastHashKey))
		if err != nil {
			log.Fatalf("error setting the genesis hash: %v", err)
		}
		lastHash, err = item.Value()
		return err
	})
	if err != nil {
		log.Fatalf("error adding a new block: %v", err)
	}
	newBlock := CreateBlock(data, lastHash)
	err = chain.Database.Update(func(txn*badger.Txn) error {
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		if err != nil {
			log.Fatalf("error saving the new block: %v", err)
		}
		err = txn.Set([]byte(lastHashKey), newBlock.Hash)
		chain.LastHash = lastHash
		return err
	})
	if err != nil {
		log.Fatalf("could not add new block: %v", err)
	}
	return newBlock
}

func Genesis(txn *CoinTransaction) *Block {
	return CreateBlock([]*CoinTransaction{txn}, []byte{})
}
func Continue(address string) *BlockChain {
	if hasDB() == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db, err := badger.Open(opts)
	if err != nil {
		log.Panicf("error opening the database: %v", err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			log.Panicf("error getting last hash: %v", err)
		}
		lastHash, err = item.Value()

		return err
	})
	if err != nil {
		log.Panicf("error setting last hash: %v", err)
	}

	chain := BlockChain{lastHash, db}

	return &chain
}


func InitBlockChain(address string) *BlockChain {
	var lastHash []byte

	if hasDB() {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db, err := badger.Open(opts)
	if err != nil {
		log.Panicf("error opening the database: %v", err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		cbtx := GenesisTransaction(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis created")
		err = txn.Set(genesis.Hash, genesis.Serialize())
		if err != nil {
			log.Panicf("error setting the genesis hash: %v", err)
		}
		err = txn.Set([]byte("lh"), genesis.Hash)

		lastHash = genesis.Hash

		return err

	})

	if err != nil {
		log.Panicf("error adding the genesis block: %v", err)
	}

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func (chain *BlockChain) FindUTXO() map[string]CoinTxOutputs {
	UTXO := make(map[string]CoinTxOutputs)
	spendTXOs := make(map[string][]int)
	iter := chain.Iterator()

	for {
		block := iter.Next()
		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)
			Outputs:
			for outInx, out := range tx.Outputs {
				if spendTXOs[txID] != nil {
					for _, spendOut := range spendTXOs[txID] {
						if spendOut == outInx {
							continue Outputs
						}
					}
				}
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}
			if tx.IsCoinTransaction() == false {
				for _, in := range tx.Inputs {
					intTxID := hex.EncodeToString(in.ID)
					spendTXOs[intTxID] = append(spendTXOs[intTxID], in.Out)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return UTXO
}


func (chain *BlockChain) Iterator() *Iterator {
	iter := &Iterator{chain.LastHash, chain.Database}
	return iter
}

func (iter *Iterator) Next() *Block {
	var block *Block
	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		if err != nil {
			log.Fatalf("error with iteration on hash: %x\n%v", iter.CurrentHash, err)
		}
		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)
		return err
	})
	if err != nil {
		log.Fatalf("error getting data from hash: %x\n%v", iter.CurrentHash, err)
	}
	iter.CurrentHash = block.PrevHash
	return block
}