package consensus

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coinbase/kryptology/pkg/core/curves"
	"github.com/coinbase/kryptology/pkg/sharing"
	frost2 "github.com/coinbase/kryptology/pkg/ted25519/frost"

	"threshold-consensus/core/types"
	"threshold-consensus/crypto/frost"
	"threshold-consensus/log"
)

var (
	ErrNotEnoughValidators = errors.New("not enough validators")
	ErrInvalidBlock        = errors.New("invalid block")
	ErrInvalidSignature    = errors.New("invalid signature")
)

type Consensus struct {
	ChainID         string
	Validators      []*types.Validator
	Aggregator      *types.Validator
	Threshold       int
	Limit           int
	mu              sync.RWMutex
	lastBlock       *types.SignedBlock
	verificationKey curves.Point
	Curve           *curves.Curve
}

func NewConsensus(chainID string, validators []*types.Validator, threshold int, limit int) *Consensus {
	if len(validators) < 3 {
		log.Error("Not enough validators, must be at least 3")
	}

	return &Consensus{
		ChainID:    chainID,
		Validators: validators,
		Threshold:  threshold,
		Limit:      limit,
	}
}

// SelectAggregator selects the next validator to be the block aggregator
// using a round-robin approach
func (c *Consensus) SelectAggregator() *types.Validator {

	if len(c.Validators) == 0 {
		return nil
	}

	// If no aggregator is set, start with the first validator
	if c.Aggregator == nil {
		c.Aggregator = c.Validators[0]
		return c.Aggregator
	}

	// Find current aggregator's index
	currentIndex := -1
	for i, v := range c.Validators {
		if v == c.Aggregator {
			currentIndex = i
			break
		}
	}

	// Select next validator in round-robin fashion
	nextIndex := (currentIndex + 1) % len(c.Validators)
	c.Aggregator = c.Validators[nextIndex]
	return c.Aggregator
}

// CreateBlock creates a new block with the current aggregator
func (c *Consensus) CreateBlock(transactions []*types.Transaction, aggregator *types.Validator) (*types.Block, error) {
	blockNumber := 1
	if c.lastBlock != nil {
		blockNumber = c.lastBlock.Block.Header.BlockNumber + 1
	}

	header := types.NewHeader(
		blockNumber,
		c.ChainID,
		time.Now(),
		aggregator,
		c.Validators,
		c.lastBlock.Block.Header.Hash,
	)

	log.Info("Creating new block with block number: %d, time: %s", header.BlockNumber, header.BlockTime.String())

	return types.NewBlock(header, transactions)
}

// ValidateBlock validates a block before it's added to the chain
func (c *Consensus) ValidateBlock(block *types.SignedBlock) error {
	if block == nil {
		return ErrInvalidBlock
	}

	// Verify the block is signed by the aggregator
	if block.Validator != block.Block.Header.Aggregator {
		return ErrInvalidSignature
	}

	// Verify the block number is correct
	if c.lastBlock != nil && block.Block.Header.BlockNumber != c.lastBlock.Block.Header.BlockNumber+1 {
		return ErrInvalidBlock
	}

	// Verify the previous block hash matches
	if c.lastBlock != nil && block.Block.Header.PreviousHash != c.lastBlock.Block.Header.Hash {
		return ErrInvalidBlock
	}

	return nil
}

// AddBlock adds a validated block to the chain
func (c *Consensus) AddBlock(block *types.SignedBlock) error {
	if err := c.ValidateBlock(block); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.lastBlock = block
	return nil
}

// GetValidators returns the current list of validators
func (c *Consensus) GetValidators() []*types.Validator {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.Validators
}

// GetCurrentAggregator returns the current block aggregator
func (c *Consensus) GetCurrentAggregator() *types.Validator {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.Aggregator == nil {
		c.SelectAggregator() // Ensure we have an aggregator selected
	}

	log.Info("Getting current aggregator: %s", c.Aggregator.Account.Address.Hex())

	return c.Aggregator
}

// GetLastBlock returns the last block in the chain
func (c *Consensus) GetLastBlock() *types.SignedBlock {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastBlock
}

// ValidatorTask represents a concurrent task for a validator
type ValidatorTask struct {
	ValidatorID uint32
	Validator   *types.Validator
	Signer      *frost2.Signer
	Error       error
}

// SigningRound1Result holds the result of signing round 1
type SigningRound1Result struct {
	ValidatorID uint32
	Broadcast   *frost2.Round1Bcast
	Error       error
}

// SigningRound2Result holds the result of signing round 2
type SigningRound2Result struct {
	ValidatorID uint32
	Broadcast   *frost2.Round2Bcast
	Error       error
}

func (c *Consensus) CreateValidationKeys() {
	participants := frost.CreateDkgParticipants(c.Threshold, c.Limit)

	rnd1Bcast, rnd1P2p := frost.DKGRound1(participants)

	verificationKey, signingShares := frost.DKGRound2(participants, rnd1Bcast, rnd1P2p)

	log.Info("Distributed key generation completed for %d validators", c.Limit)

	for i := 0; i < c.Limit; i++ {
		c.Validators[i].Share = signingShares[uint32(i+1)]
		c.Validators[i].Participant = participants[uint32(i+1)]
	}

	log.Info("Created block validation key")

	c.verificationKey = verificationKey
}

// createSignersConcurrently creates all signers in parallel
func (c *Consensus) createSignersConcurrently(ctx context.Context, lcs map[uint32]curves.Scalar, validatorIDs []uint32) (map[uint32]*frost2.Signer, error) {
	signers := make(map[uint32]*frost2.Signer)
	signersMutex := sync.RWMutex{}

	var wg sync.WaitGroup
	errorChan := make(chan error, c.Threshold)

	for i := 0; i < c.Threshold; i++ {
		wg.Add(1)
		go func(validatorIndex int) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			default:
			}

			validatorID := validatorIDs[validatorIndex]
			signer, err := frost2.NewSigner(
				c.Validators[validatorIndex].Participant,
				validatorID,
				uint32(c.Threshold),
				lcs,
				validatorIDs,
				&frost2.Ed25519ChallengeDeriver{},
			)

			if err != nil {
				errorChan <- fmt.Errorf("failed to create signer %d: %w", validatorID, err)
				return
			}

			signersMutex.Lock()
			signers[validatorID] = signer
			signersMutex.Unlock()
		}(i)
	}

	// Wait for all signers to be created
	go func() {
		wg.Wait()
		close(errorChan)
	}()

	// Check for errors
	for err := range errorChan {
		if err != nil {
			return nil, errors.New("failed to create signers: " + err.Error())
		}
	}

	return signers, nil
}

// executeSigningRound1Concurrently executes signing round 1 for all validators concurrently
func (c *Consensus) executeSigningRound1Concurrently(ctx context.Context, signers map[uint32]*frost2.Signer) (map[uint32]*frost2.Round1Bcast, error) {
	resultChan := make(chan SigningRound1Result, len(signers))
	var wg sync.WaitGroup

	// Execute round 1 for each signer concurrently
	for validatorID, signer := range signers {
		wg.Add(1)
		go func(vID uint32, s *frost2.Signer) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				resultChan <- SigningRound1Result{
					ValidatorID: vID,
					Error:       ctx.Err(),
				}
				return
			default:
			}

			broadcast, err := s.SignRound1()
			resultChan <- SigningRound1Result{
				ValidatorID: vID,
				Broadcast:   broadcast,
				Error:       err,
			}
		}(validatorID, signer)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	sigRnd1Bcast := make(map[uint32]*frost2.Round1Bcast)
	for result := range resultChan {
		if result.Error != nil {
			return nil, errors.New("signing round 1 failed for validator " + fmt.Sprint(result.ValidatorID) + ": " + result.Error.Error())
		}
		sigRnd1Bcast[result.ValidatorID] = result.Broadcast
	}

	return sigRnd1Bcast, nil
}

// executeSigningRound2Concurrently executes signing round 2 for all validators concurrently
func (c *Consensus) executeSigningRound2Concurrently(ctx context.Context, signers map[uint32]*frost2.Signer, message []byte, sigRnd1Bcast map[uint32]*frost2.Round1Bcast) (map[uint32]*frost2.Round2Bcast, error) {
	resultChan := make(chan SigningRound2Result, len(signers))
	var wg sync.WaitGroup

	// Execute round 2 for each signer concurrently
	for validatorID, signer := range signers {
		wg.Add(1)
		go func(vID uint32, s *frost2.Signer) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				resultChan <- SigningRound2Result{
					ValidatorID: vID,
					Error:       ctx.Err(),
				}
				return
			default:
			}

			broadcast, err := s.SignRound2(message, sigRnd1Bcast)
			resultChan <- SigningRound2Result{
				ValidatorID: vID,
				Broadcast:   broadcast,
				Error:       err,
			}
		}(validatorID, signer)
	}

	// Wait for all to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	sigRng2BCast := make(map[uint32]*frost2.Round2Bcast)
	for result := range resultChan {
		if result.Error != nil {
			return nil, errors.New("signing round 2 failed for validator " + fmt.Sprint(result.ValidatorID) + ": " + result.Error.Error())
		}
		sigRng2BCast[result.ValidatorID] = result.Broadcast
	}

	return sigRng2BCast, nil
}

// SignBlockConcurrent is an alternative implementation that uses worker pools
func (c *Consensus) SignBlockConcurrent(block *types.Block) (*types.SignedBlock, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scheme, err := sharing.NewShamir(uint32(c.Threshold), uint32(c.Limit), c.Curve)
	if err != nil {
		return nil, errors.New("failed to create Shamir scheme: " + err.Error())
	}

	log.Info("Shamir scheme created with threshold: %d, limit: %d", c.Threshold, c.Limit)

	// Get validator IDs for active signers
	validatorIDs := make([]uint32, c.Threshold)
	for i := 0; i < c.Threshold; i++ {
		validatorIDs[i] = c.Validators[i].Share.Id
	}

	lcs, err := scheme.LagrangeCoeffs(validatorIDs)
	if err != nil {
		return nil, errors.New("failed to create Lagrange coefficients: " + err.Error())
	}

	log.Info("Lagrange coefficients created for %d validators", len(validatorIDs))

	// Use worker pool pattern for better resource management
	signers, err := c.createSignersWithWorkerPool(ctx, lcs, validatorIDs)
	if err != nil {
		return nil, err
	}

	log.Info("Signers created for signing the block with block number: %d", block.Header.BlockNumber)

	// Execute signing rounds with worker pools
	sigRnd1Bcast, err := c.executeRound1WithWorkerPool(ctx, signers)
	if err != nil {
		return nil, err
	}

	log.Info("Signing round 1 completed for block number: %d", block.Header.BlockNumber)

	sigRng2BCast, err := c.executeRound2WithWorkerPool(ctx, signers, block.Hash.Bytes(), sigRnd1Bcast)
	if err != nil {
		return nil, err
	}

	log.Info("Signing round 2 completed for block number: %d", block.Header.BlockNumber)

	// Round 3 (aggregation) - typically done by coordinator
	sigRng3BCast, err := signers[validatorIDs[0]].SignRound3(sigRng2BCast)
	if err != nil {
		return nil, errors.New("failed to sign round 3: " + err.Error())
	}

	log.Info("Signing round 3 completed for block number: %d", block.Header.BlockNumber)

	signature := &frost2.Signature{
		Z: sigRng3BCast.Z,
		C: sigRng3BCast.C,
	}

	// Verify signature
	_, err = frost2.Verify(c.Curve, frost2.Ed25519ChallengeDeriver{}, c.verificationKey, block.Hash.Bytes(), signature)
	if err != nil {
		return nil, errors.New("signature verification failed: " + err.Error())
	}

	log.Info("Signature verified for block number: %d", block.Header.BlockNumber)

	return types.NewSignedBlock(block, signature, c.Aggregator), nil
}

// createSignersWithWorkerPool creates signers using a worker pool pattern
func (c *Consensus) createSignersWithWorkerPool(ctx context.Context, lcs map[uint32]curves.Scalar, validatorIDs []uint32) (map[uint32]*frost2.Signer, error) {
	numWorkers := min(c.Threshold, 3) // Limit concurrent workers
	taskChan := make(chan ValidatorTask, c.Threshold)
	resultChan := make(chan ValidatorTask, c.Threshold)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.signerCreationWorker(ctx, lcs, validatorIDs, taskChan, resultChan)
		}()
	}

	// Send tasks
	go func() {
		for i := 0; i < c.Threshold; i++ {
			select {
			case taskChan <- ValidatorTask{
				ValidatorID: validatorIDs[i],
				Validator:   c.Validators[i],
			}:
			case <-ctx.Done():
				return
			}
		}
		close(taskChan)
	}()

	// Wait for workers to finish
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	signers := make(map[uint32]*frost2.Signer)
	for result := range resultChan {
		if result.Error != nil {
			return nil, errors.New("failed to create signer for validator " + fmt.Sprint(result.ValidatorID) + ": " + result.Error.Error())
		}
		signers[result.ValidatorID] = result.Signer
	}

	return signers, nil
}

// signerCreationWorker is a worker function for creating signers
func (c *Consensus) signerCreationWorker(ctx context.Context, lcs map[uint32]curves.Scalar, validatorIDs []uint32, taskChan <-chan ValidatorTask, resultChan chan<- ValidatorTask) {
	for task := range taskChan {
		select {
		case <-ctx.Done():
			resultChan <- ValidatorTask{
				ValidatorID: task.ValidatorID,
				Error:       ctx.Err(),
			}
			return
		default:
		}

		signer, err := frost2.NewSigner(
			task.Validator.Participant,
			task.ValidatorID,
			uint32(c.Threshold),
			lcs,
			validatorIDs,
			&frost2.Ed25519ChallengeDeriver{},
		)

		if err != nil {
			resultChan <- ValidatorTask{
				ValidatorID: task.ValidatorID,
				Error:       fmt.Errorf("failed to create signer %d: %w", task.ValidatorID, err),
			}
			continue
		}

		// You'll need to adapt this to store the signer properly
		resultChan <- ValidatorTask{
			ValidatorID: task.ValidatorID,
			Signer:      signer,
			Error:       nil,
		}
	}
}

// executeRound1WithWorkerPool executes round 1 using worker pool
func (c *Consensus) executeRound1WithWorkerPool(ctx context.Context, signers map[uint32]*frost2.Signer) (map[uint32]*frost2.Round1Bcast, error) {
	numWorkers := min(len(signers), 3)
	taskChan := make(chan uint32, len(signers))
	resultChan := make(chan SigningRound1Result, len(signers))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.round1Worker(ctx, signers, taskChan, resultChan)
		}()
	}

	// Send tasks
	go func() {
		for validatorID := range signers {
			select {
			case taskChan <- validatorID:
			case <-ctx.Done():
				return
			}
		}
		close(taskChan)
	}()

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make(map[uint32]*frost2.Round1Bcast)
	for result := range resultChan {
		if result.Error != nil {
			return nil, errors.New("signing round 1 failed for validator " + fmt.Sprint(result.ValidatorID) + ": " + result.Error.Error())
		}
		results[result.ValidatorID] = result.Broadcast
	}

	return results, nil
}

// round1Worker processes round 1 signing tasks
func (c *Consensus) round1Worker(ctx context.Context, signers map[uint32]*frost2.Signer, taskChan <-chan uint32, resultChan chan<- SigningRound1Result) {
	for validatorID := range taskChan {
		select {
		case <-ctx.Done():
			resultChan <- SigningRound1Result{
				ValidatorID: validatorID,
				Error:       ctx.Err(),
			}
			return
		default:
		}

		signer := signers[validatorID]
		broadcast, err := signer.SignRound1()

		resultChan <- SigningRound1Result{
			ValidatorID: validatorID,
			Broadcast:   broadcast,
			Error:       err,
		}
	}
}

// executeRound2WithWorkerPool executes round 2 using worker pool
func (c *Consensus) executeRound2WithWorkerPool(ctx context.Context, signers map[uint32]*frost2.Signer, message []byte, sigRnd1Bcast map[uint32]*frost2.Round1Bcast) (map[uint32]*frost2.Round2Bcast, error) {
	numWorkers := min(len(signers), 3)
	taskChan := make(chan uint32, len(signers))
	resultChan := make(chan SigningRound2Result, len(signers))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.round2Worker(ctx, signers, message, sigRnd1Bcast, taskChan, resultChan)
		}()
	}

	// Send tasks
	go func() {
		for validatorID := range signers {
			select {
			case taskChan <- validatorID:
			case <-ctx.Done():
				return
			}
		}
		close(taskChan)
	}()

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	results := make(map[uint32]*frost2.Round2Bcast)
	for result := range resultChan {
		if result.Error != nil {
			return nil, errors.New("signing round 2 failed for validator " + fmt.Sprint(result.ValidatorID) + ": " + result.Error.Error())
		}
		results[result.ValidatorID] = result.Broadcast
	}

	return results, nil
}

// round2Worker processes round 2 signing tasks
func (c *Consensus) round2Worker(ctx context.Context, signers map[uint32]*frost2.Signer, message []byte, sigRnd1Bcast map[uint32]*frost2.Round1Bcast, taskChan <-chan uint32, resultChan chan<- SigningRound2Result) {
	for validatorID := range taskChan {
		select {
		case <-ctx.Done():
			resultChan <- SigningRound2Result{
				ValidatorID: validatorID,
				Error:       ctx.Err(),
			}
			return
		default:
		}

		signer := signers[validatorID]
		broadcast, err := signer.SignRound2(message, sigRnd1Bcast)

		resultChan <- SigningRound2Result{
			ValidatorID: validatorID,
			Broadcast:   broadcast,
			Error:       err,
		}
	}
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
