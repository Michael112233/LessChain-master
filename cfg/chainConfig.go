package cfg

import (
	"math/big"
)

type ChainConfig struct {
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection
}

var (
	AllProtocolChanges = &ChainConfig{
		ChainID: big.NewInt(-1),
	}

	GenesisDifficulty = big.NewInt(131072) // Difficulty of the Genesis block.
)
