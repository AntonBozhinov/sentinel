package blockchain

import (
	"bytes"
	"encoding/hex"
	"github.com/dgraph-io/badger"
	"log"
)

var (
	utxoPrefix = []byte("utxo-")
	prefixLength = len(utxoPrefix)
)

type UTXOSet struct {
	BlockChain *BlockChain
}

func (u UTXOSet) CountTransactions() int {
	db := u.BlockChain.Database
	counter := 0
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}
		return nil
	})

	if err != nil {
		log.Panicf("error counting transactions: %v\n", err)
	}
	return counter
}

func (u UTXOSet) FindUnspentTransactions(pubKeyHash []byte) []CoinTxOutput {
	var UTXOs []CoinTxOutput
	db := u.BlockChain.Database
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			v, err := item.Value()
			if err != nil {
				log.Panicf("error retrieving the item value: %v\n", err)
			}
			outs := DeserializeOutputs(v)
			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panicf("error finding unspend transactions: %v\n", err)
	}
	return UTXOs
}

func (u UTXOSet) FindSpendableTransactions(pubKeyHash []byte, amount int) (int, map[string][]int) {
	unspentOuts := make(map[string][]int)
	accumulated := 0
	db := u.BlockChain.Database
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			v, err := item.Value()
			if err != nil {
				return err
			}
			k = bytes.TrimPrefix(k, utxoPrefix)
			txID := hex.EncodeToString(k)
			outs := DeserializeOutputs(v)
			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Panicf("error finding spendable transactions: %v\n", err)
	}

	return accumulated, unspentOuts
}

func (u *UTXOSet) Update(block *Block) {
	db := u.BlockChain.Database;
	err := db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transactions {
			if tx.IsCoinTransaction() == false {
				for _, in := range tx.Inputs {
					updatedOuts := CoinTxOutputs{}
					ID := append(utxoPrefix, in.ID...)
					item, err := txn.Get(ID)
					if err != nil {
						log.Panicf("error getting id: %s\n%v", ID, err)
					}
					v, err := item.Value()
					if err != nil {
						log.Panicf("error getting id: %s\n%v", ID, err)
					}
					outs := DeserializeOutputs(v)
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}

					if len(updatedOuts.Outputs) == 0 {
						if err := txn.Delete(ID); err != nil {
							log.Panicf("error deleting output with id: %x\n%v\n", ID, err)
						}
					} else {
						if err := txn.Set(ID, updatedOuts.Serialize()); err != nil {
							log.Panicf("error setting outputs for id: %x\n%v\n", ID, err)
						}
					}
				}
			}
			newOutputs := CoinTxOutputs{}
			for _, out := range tx.Outputs {
				newOutputs.Outputs = append(newOutputs.Outputs, out)
			}
			txID := append(utxoPrefix, tx.ID...)
			if err := txn.Set(txID, newOutputs.Serialize()); err != nil {
				log.Panicf("error setting the new outputs for transaction id: %x\n%v\n", txID, err)
			}
		}
		return nil
	})
	if err != nil {
		log.Panicf("error updating UTXOSet: %v\n", err)
	}
}

func (u UTXOSet) Reindex() {
	db := u.BlockChain.Database

	u.DeleteByPrefix(utxoPrefix)

	UTXO := u.BlockChain.FindUTXO()

	err := db.Update(func(txn *badger.Txn) error {
		for txId, outs := range UTXO {
			key, err := hex.DecodeString(txId)
			if err != nil {
				return err
			}
			key = append(utxoPrefix, key...)
			err = txn.Set(key, outs.Serialize())
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		log.Panicf("error reindexing unspent transaction outputs (UTXO): %v", err)
	}
}

func (u *UTXOSet) DeleteByPrefix(prefix []byte)  {
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := u.BlockChain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err :=txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}
	// this is the optimal amount of keys we can delete with badgerDB
	collectSize := 100000
	err := u.BlockChain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()
		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panicf("error deleting keys with prefix: %s'\n%v",prefix, err)
				}
				keysForDelete = make([][]byte, 0,  collectSize)
				keysCollected = 0
			}
		}
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panicf("error deleting keys with prefix: %s'\n%v",prefix, err)
			}
		}

		return nil
	})
	if err != nil {
		log.Panicf("error deleting keys with prefix: %s'\n%v",prefix, err)
	}
}

