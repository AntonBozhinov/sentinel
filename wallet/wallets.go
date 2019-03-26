package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const walletsFile = "./tmp/wallets.data"

// Wallets of the user
type Wallets struct {
	Wallets map[string]*Wallet
}

// CreateWallets create user wallets
func CreateWallets() (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet)
	err := wallets.LoadFile()
	return &wallets, err
}

// GetWallet from an address
func (ws *Wallets) GetWallet(address string) Wallet {
	fmt.Printf("%v\n", ws.Wallets)
	fmt.Printf("%s\n", address)
	w := *ws.Wallets[address]

	return w
}

// GetAllAddresses gets all user wallet addresses
func (ws *Wallets) GetAllAddresses() []string {
	var addresses []string
	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}
	return addresses
}

// AddWallet to add new wallet to wallets
func (ws *Wallets) AddWallet() string {
	wallet := MakeWallet()
	fmt.Println("Wallet added successfully!")
	address := fmt.Sprintf("%s", wallet.Address())

	ws.Wallets[address] = wallet
	return address
}

// LoadFile reads wallets file
func (ws *Wallets) LoadFile() error {
	if _, err := os.Stat(walletsFile); os.IsNotExist(err) {
		return err
	}
	var wallets Wallets
	fileContent, err := ioutil.ReadFile(walletsFile)
	if err != nil {
		log.Panicf("error reading wallets file: %v", err)
	}
	gob.Register(elliptic.P256())
	decoder := gob.NewDecoder(bytes.NewReader(fileContent))
	err = decoder.Decode(&wallets)
	if err != nil {
		log.Panicf("error decoding wallets file: %v", err)
	}
	ws.Wallets = wallets.Wallets
	return nil
}

// SaveFile saves user wallets data to e file
func (ws *Wallets) SaveFile() {
	var content bytes.Buffer
	gob.Register(elliptic.P256())
	encoder := gob.NewEncoder(&content)
	err := encoder.Encode(ws)
	if err != nil {
		log.Panicf("error encoding wallets: %v", err)
	}
	err = ioutil.WriteFile(walletsFile, content.Bytes(), 0644)
	if err != nil {
		log.Panicf("error writing wallets file: %v", err)
	}
}