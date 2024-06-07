package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
)

/*
Nonce handling
+ Create a new state object if the recipient is \0*32
+ Value transfer
+ Derive new state root
*/

func NewStateDB(db ethdb.Database) *state.StateDB {
	state, _ := state.New(common.Hash{}, state.NewDatabase(db), nil)
	return state
}
