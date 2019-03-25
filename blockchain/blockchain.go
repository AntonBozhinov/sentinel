package blockchain

import (
	"encoding/hex"
	"fmt"
	"github.com/dgraph-io/badger"
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

// DBexists checks if the database has been initialized
func DBexists() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}
	return true
}

// AddBlock to the blockchain
func (chain *BlockChain) AddBlock(data []*CoinTransaction) {
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
}

// Genesis block of the blockchain
func Genesis(txn *CoinTransaction) *Block {
	return CreateBlock([]*CoinTransaction{txn}, []byte{})
}
// Continue from an existing database
func Continue(address string) *BlockChain {
	if DBexists() == false {
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


// InitBlockChain with persistence
func InitBlockChain(address string) *BlockChain {
	var lastHash []byte

	if DBexists() {
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
		cbtx := InitCoinTransaction(address, genesisData)
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

// FindUnspentTransactions finds all unspent transactions for an address
func (chain *BlockChain) FindUnspentTransactions(address string) []CoinTransaction {
	var unspentTxs []CoinTransaction
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
				if out.CanBeUnlocked(address) {
					unspentTxs = append(unspentTxs, *tx)
				}
			}
			if tx.IsCoinTransaction() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spendTXOs[inTxID] = append(spendTXOs[inTxID], in.Out)
				}				
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return unspentTxs
}

// FindUTXO finds all unspent transaction outputs
func (chain *BlockChain) FindUTXO(address string) []CoinTxOutput {
	var UTXOs []CoinTxOutput
	unspentTransactions := chain.FindUnspentTransactions(address)
	for _, tx := range unspentTransactions {
		for _, out := range tx.Outputs {
			if out.CanBeUnlocked(address) {
				UTXOs = append(UTXOs, out)
			}
		}
	}
	return UTXOs
}

// FindSpendableTransactions ensures that the address has sufficient funds
func (chain *BlockChain) FindSpendableTransactions(address string, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	unspentTx := chain.FindUnspentTransactions(address)
	accumulated := 0

	Work:
	for _, tx := range unspentTx {
		txID := hex.EncodeToString(tx.ID)
		for outIdx, out := range tx.Outputs {
			if out.CanBeUnlocked(address) && accumulated < amount {
				accumulated += out.Value
				unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				if accumulated >= amount {
					break Work
				}
			}
		}
	}

	return accumulated, unspentOuts
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
