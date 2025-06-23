package types

import (
	"crypto/sha256"

	"github.com/MGunes72/t-chain/common"
)

type Transaction struct {
	ID        string `json:"id"`
	Sender    string `json:"sender"`
	Receiver  string `json:"receiver"`
	Amount    int    `json:"amount"`
	Timestamp string `json:"timestamp"`
	Signature string `json:"signature"`
	Hash      []byte `json:"hash"`
}

func NewTransaction(id, sender, receiver string, amount int, timestamp string, signature string) (*Transaction, error) {

	tx := &Transaction{
		ID:        id,
		Sender:    sender,
		Receiver:  receiver,
		Amount:    amount,
		Timestamp: timestamp,
		Signature: signature,
	}

	hash, err := tx.CalculateHash()
	if err != nil {
		return nil, err
	}

	tx.Hash = hash

	return tx, nil
}

func (tx *Transaction) CalculateHash() ([]byte, error) {
	data, err := common.MarshalJSON(tx)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	return hash[:], nil
}
