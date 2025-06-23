package main

import (
	"encoding/hex"
	"time"

	"threshold-consensus/core/blockchain"
	"threshold-consensus/log"
)

func main() {
	// Create blockchain with genesis block
	bc, err := blockchain.NewBlockchain()
	if err != nil {
		log.Error("Failed to create blockchain: %v", err)
		return
	}

	// Create a ticker for block production
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Get current aggregator
		aggregator := bc.Consensus.GetCurrentAggregator()
		if aggregator == nil {
			log.Error("No aggregator available")
			continue
		}

		// Create new block
		block, err := bc.Consensus.CreateBlock(bc.TxPool.GetTransactions(), aggregator)
		if block == nil || err != nil {
			log.Error("Failed to create block")
			continue
		}

		signedBlock, err := bc.Consensus.SignBlockConcurrent(block)
		if err != nil {
			log.Error("Failed to sign block: %v", err)
			continue
		}

		// Add block to chain
		if err := bc.Consensus.AddBlock(signedBlock); err != nil {
			log.Error("Failed to add block: %v", err)
			continue
		}

		bc.AddBlock(signedBlock)
		log.Info("New block added by validator %s, block number: %d, hash: %s",
			hex.EncodeToString(aggregator.Account.Address[:]),
			block.Header.BlockNumber,
			block.Hash.Hex(),
		)
	}
}
