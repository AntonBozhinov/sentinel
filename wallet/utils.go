package wallet

import (
	"fmt"
	"log"
	"github.com/mr-tron/base58"
)

// Base58Encode bytes
func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input)
	return []byte(encode)
}

// Base58Decode bytes
func Base58Decode(input []byte) []byte {
	decode, err := base58.Decode(string(input[:]))
	if (err != nil) {
		log.Panicf("error decoding base58: %v", err)
	}
	return decode
}

// Address is the wallet address
func (w Wallet) Address() []byte{
	pubHash := PublicKeyHash(w.PublicKey)
	versionedHash := append([]byte{version}, pubHash...)
	checksum := Checksum(versionedHash)
	fullHash := append(versionedHash, checksum...)
	address := Base58Encode(fullHash)

	fmt.Printf("public key: %x\n", w.PublicKey)
	fmt.Printf("public hash: %x\n", pubHash)
	fmt.Printf("address: %s\n", address)

	return address
}