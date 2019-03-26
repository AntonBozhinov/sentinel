package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"golang.org/x/crypto/ripemd160"
	"log"
)

const (
	ChecksumLength = 4
	version        = byte(0x00)
)

// Wallet in the blockchain
type Wallet struct {
	PrivateKey ecdsa.PrivateKey
	PublicKey []byte
}

// ValidateAddress validates an address by comparing checksum
func ValidateAddress(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))
	actualChecksum := pubKeyHash[len(pubKeyHash) -ChecksumLength:]
	version := pubKeyHash[0]
	pubKeyHash = pubKeyHash[1:len(pubKeyHash) -ChecksumLength]
	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...))
	return bytes.Compare(actualChecksum, targetChecksum) == 0
}

// NewKeyPair generates new public/private key pair for a wallet
func NewKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panicf("error generating a key pair: %v", err)
	}
	pub := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)
	return *private, pub
}

// MakeWallet creates new Wallet
func MakeWallet() *Wallet {
	private, public := NewKeyPair()
	w := Wallet{
		PrivateKey: private,
		PublicKey: public,
	}
	return &w
}

// PublicKeyHash hash a public key
func PublicKeyHash(pubKey []byte) []byte {
	pubHash := sha256.Sum256(pubKey)
	hasher := ripemd160.New()
	_ , err := hasher.Write(pubHash[:])
	if err != nil {
		log.Panicf("error with ripemd160 hash write: %v", err)
	}
	publicRipMd := hasher.Sum(nil)
	return publicRipMd
}

// Checksum of the wallet
func Checksum(payload []byte) []byte {
	firstHash := sha256.Sum256(payload)
	secondHash := sha256.Sum256(firstHash[:])
	return secondHash[:ChecksumLength]
}

