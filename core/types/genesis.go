package types

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/MGunes72/t-chain/common"
)

type Genesis struct {
	ChainID    string       `json:"chainId"`
	Time       time.Time    `json:"time"`
	Validators []*Validator `json:"validators"`
}

type GenesisValidator struct {
	Account struct {
		Address string `json:"address"`
		Balance int64  `json:"balance"`
	} `json:"account"`
}

func LoadGenesis(filePath string) (*Genesis, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read genesis file: %v", err)
	}

	var genesisData struct {
		ChainID    string             `json:"chainId"`
		Time       time.Time          `json:"time"`
		Validators []GenesisValidator `json:"validators"`
	}

	err = json.Unmarshal(data, &genesisData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse genesis: %v", err)
	}

	genesis := &Genesis{
		ChainID: genesisData.ChainID,
		Time:    genesisData.Time,
	}

	// Convert validators
	for _, v := range genesisData.Validators {
		address := common.HexToAddress(v.Account.Address)
		account := NewAccount(&address, v.Account.Balance)
		validator := NewValidator(*account)
		genesis.Validators = append(genesis.Validators, validator)
	}

	return genesis, nil
}
