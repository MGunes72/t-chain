package crypto

import (
	"crypto/ed25519"
	"hash"

	"golang.org/x/crypto/sha3"

	"threshold-consensus/common"
)

type KeccakState interface {
	hash.Hash
	Read([]byte) (int, error)
}

func NewKeccakState() KeccakState {
	return sha3.NewLegacyKeccak256().(KeccakState)
}

func Keccak256(data ...[]byte) []byte {
	b := make([]byte, 32)
	d := NewKeccakState()
	for _, b := range data {
		d.Write(b)
	}
	d.Read(b)
	return b
}

func PubkeyToAddress(p ed25519.PublicKey) common.Address {
	return common.BytesToAddress(Keccak256(p))
}
