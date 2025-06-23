package types

import (
	"encoding/json"
	"time"

	"github.com/coinbase/kryptology/pkg/ted25519/frost"

	"threshold-consensus/common"
	"threshold-consensus/crypto"
)

type Header struct {
	BlockNumber  int          `json:"block_number"`
	BlockTime    time.Time    `json:"block_time"`
	ChainID      string       `json:"chain_id"`
	Aggregator   *Validator   `json:"aggregator"`
	Validators   []*Validator `json:"validators"`
	Hash         *common.Hash `json:"hash"`
	PreviousHash *common.Hash `json:"previous_hash,omitempty"`
}

func NewHeader(blockNumber int, chainID string, time time.Time, aggregator *Validator, validators []*Validator, previousHash *common.Hash) Header {
	return Header{
		BlockNumber:  blockNumber,
		BlockTime:    time,
		ChainID:      chainID,
		Aggregator:   aggregator,
		Validators:   validators,
		PreviousHash: previousHash,
	}
}

type Block struct {
	Header       Header         `json:"header"`
	Hash         *common.Hash   `json:"hash"`
	Transactions []*Transaction `json:"transactions"`
}

func NewBlock(header Header, transactions []*Transaction) (*Block, error) {

	data, err := json.Marshal(header)
	if err != nil {
		return nil, err
	}

	hashBytes := crypto.Keccak256(data)
	hash := common.BytesToHash(hashBytes)

	header.Hash = &hash

	return &Block{
		Header:       header,
		Hash:         &hash,
		Transactions: transactions,
	}, nil
}

func (b Block) GetHeader() Header {
	return b.Header
}

type SignedBlock struct {
	Block     *Block
	Signature *frost.Signature
	Validator *Validator
}

func NewSignedBlock(block *Block, signature *frost.Signature, validator *Validator) *SignedBlock {
	return &SignedBlock{
		Block:     block,
		Signature: signature,
		Validator: validator,
	}
}
