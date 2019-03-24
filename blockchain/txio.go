package blockchain

// CoinTxInput is the transaction intput
type CoinTxInput struct {
	ID  []byte
	Out int
	Sig string
}

// CoinTxOutput is the transaction output
type CoinTxOutput struct {
	Value  int
	PubKey string
}

// CanUnlock a transaction
func (in *CoinTxInput) CanUnlock(data string) bool {
	return in.Sig == data
}

// CanBeUnlocked checks if a transaction output can be unlocked
func (out *CoinTxOutput) CanBeUnlocked(data string) bool {
	return out.PubKey == data
}