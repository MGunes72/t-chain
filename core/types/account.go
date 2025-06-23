package types

import (
	"threshold-consensus/common"
)

type Account struct {
	Address *common.Address `json:"address"`
	Balance int64           `json:"balance"`
	Nonce   int64           `json:"nonce"`
}

func NewAccount(address *common.Address, balance int64) *Account {
	return &Account{
		Address: address,
		Balance: balance,
		Nonce:   0,
	}
}

func (a *Account) AddFunds(amount int64) {
	a.Balance += amount
}

func (a *Account) SubtractFunds(amount int64) {
	if a.Balance >= amount {
		a.Balance -= amount
	} else {
		a.Balance = 0
	}
}

func (a *Account) GetBalance() int64 {
	return a.Balance
}

func (a *Account) GetAddress() *common.Address {
	return a.Address
}

func (a *Account) GetNonce() int64 {
	return a.Nonce
}
