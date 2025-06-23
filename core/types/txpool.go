package types

import (
	"bytes"
	"sync"
)

var transactionPool []*Transaction
var transactionLock sync.Mutex

type TxPool struct {
	Transactions []*Transaction
}

func NewTxPool() *TxPool {
	return &TxPool{
		Transactions: make([]*Transaction, 0),
	}
}

func (tp *TxPool) AddTransaction(tx *Transaction) {
	transactionLock.Lock()
	defer transactionLock.Unlock()

	for _, existingTx := range transactionPool {
		if existingTx.ID == tx.ID {
			return
		}
	}

	transactionPool = append(transactionPool, tx)
	tp.Transactions = append(tp.Transactions, tx)
}

func (tp *TxPool) GetTransactions() []*Transaction {
	transactionLock.Lock()
	defer transactionLock.Unlock()

	txs := make([]*Transaction, len(transactionPool))
	copy(txs, transactionPool)
	return txs
}

func (tp *TxPool) GetTransactionByHash(hash []byte) *Transaction {
	transactionLock.Lock()
	defer transactionLock.Unlock()

	for _, tx := range transactionPool {
		if bytes.Equal(tx.Hash, hash) {
			return tx
		}
	}
	return nil
}

func (tp *TxPool) Clear() {
	transactionLock.Lock()
	defer transactionLock.Unlock()

	transactionPool = make([]*Transaction, 0)
	tp.Transactions = make([]*Transaction, 0)
}
