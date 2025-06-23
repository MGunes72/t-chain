package types

import (
	"github.com/coinbase/kryptology/pkg/dkg/frost"
	"github.com/coinbase/kryptology/pkg/sharing"
)

type Validator struct {
	Account     Account `json:"account"`
	Share       *sharing.ShamirShare
	Participant *frost.DkgParticipant
}

func NewValidator(acc Account) *Validator {
	return &Validator{
		Account: acc,
	}
}
