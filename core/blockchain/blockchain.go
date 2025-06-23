package blockchain

import (
	"errors"
	"flag"

	"github.com/coinbase/kryptology/pkg/core/curves"

	"github.com/MGunes72/t-chain/common"
	"github.com/MGunes72/t-chain/consensus"
	"github.com/MGunes72/t-chain/core/types"
	"github.com/MGunes72/t-chain/log"
)

type Blockchain struct {
	ChainID      string
	GenesisBlock *types.Genesis
	Blocks       []*types.SignedBlock
	TxPool       *types.TxPool
	Consensus    *consensus.Consensus
}

func NewBlockchain() (*Blockchain, error) {
	genesisPath := flag.String("genesis", "genesis.json", "Path to genesis file")
	flag.Parse()

	genesis, err := types.LoadGenesis(*genesisPath)
	if err != nil {
		return nil, err
	}

	log.Info("Loaded genesis block, chain id: %s, time: %s ", genesis.ChainID, genesis.Time.String())

	bc := &Blockchain{
		ChainID:      genesis.ChainID,
		GenesisBlock: genesis,
		Blocks:       make([]*types.SignedBlock, 0),
		TxPool:       types.NewTxPool(),
	}

	// Initialize consensus with validators from genesis
	cons := consensus.NewConsensus(
		bc.ChainID,
		bc.GenesisBlock.Validators,
		2, // threshold (2 out of 3 validators required)
		3, // max validators
	)

	log.Info("Initialized consensus with %d validators", len(cons.Validators))

	bc.Consensus = cons

	bc.Consensus.Curve = curves.ED25519()
	bc.Consensus.CreateValidationKeys()

	genesisHeader := types.NewHeader(0, genesis.ChainID, genesis.Time, nil, genesis.Validators, nil)
	g, err := types.NewBlock(genesisHeader, nil)
	if err != nil {
		return nil, err
	}

	signedBlock, err := bc.Consensus.SignBlockConcurrent(g)
	if err != nil {
		log.Error("Failed to sign block: %v", err)
	}

	// Add block to chain
	if err := bc.Consensus.AddBlock(signedBlock); err != nil {
		log.Error("Failed to add block: %v", err)
	}

	bc.AddBlock(signedBlock)

	return bc, nil
}

func (bc *Blockchain) AddBlock(b *types.SignedBlock) error {
	if b.Block.Header.BlockNumber != len(bc.Blocks) {
		return errors.New("block index does not match the length of the blockchain")
	}

	if b.Block.Header.PreviousHash != nil && b.Block.Header.PreviousHash != bc.Blocks[len(bc.Blocks)-1].Block.Header.Hash {
		return errors.New("block previous hash does not match the last block's hash")
	}

	b.Block.Transactions = bc.TxPool.Transactions
	bc.Blocks = append(bc.Blocks, b)

	bc.TxPool.Clear()

	bc.Consensus.SelectAggregator()

	return nil
}

func (bc *Blockchain) GetBlockByNumber(blockNumber int) *types.SignedBlock {
	if blockNumber < 0 || blockNumber >= len(bc.Blocks) {
		return nil
	}
	return bc.Blocks[blockNumber]
}

func (bc *Blockchain) GetBlockByHash(blockHash *common.Hash) *types.SignedBlock {
	for _, b := range bc.Blocks {
		if b.Block.Header.Hash == blockHash {
			return b
		}
	}
	return nil
}

func (bc *Blockchain) GetLatestBlock() *types.SignedBlock {
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

func (bc *Blockchain) GetGenesisBlock() *types.Genesis {
	return bc.GenesisBlock
}

func (bc *Blockchain) GetChainID() string {
	return bc.ChainID
}

func (bc *Blockchain) GetBlockCount() int {
	return len(bc.Blocks)
}
