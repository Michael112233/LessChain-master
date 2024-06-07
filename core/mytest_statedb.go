package core

import (
	"fmt"
	"math/big"

	"go-w3chain/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
)

// core/state/statedb_test.go
func UpdateLeaksTest() {
	db := rawdb.NewMemoryDatabase()
	state, _ := state.New(common.Hash{}, state.NewDatabase(db), nil)

	for i := byte(0); i < 255; i++ {
		addr := common.BytesToAddress([]byte{i})
		state.AddBalance(addr, big.NewInt(11*int64(i))) // 不是 big.NewInt(int64(11*i))
		state.SetNonce(addr, uint64(42*i))
		if i%2 == 0 {
			state.SetState(addr, common.BytesToHash([]byte{i, i, i}), common.BytesToHash([]byte{i, i, i, i}))
		}
		if i%3 == 0 {
			state.SetCode(addr, []byte{i, i, i, i, i})
		}
	}

	root := state.IntermediateRoot(false)
	fmt.Println(root)
	if err := state.Database().TrieDB().Commit(root, false, nil); err != nil {
		log.Info("can not commit trie %v to persistent database", root.Hex())
	}

	// // Ensure that no data was leaked into the database
	// it := db.NewIterator(nil, nil)
	// for it.Next() {
	// 	log.Info("State leaked into database: %x -> %x", it.Key(), it.Value())
	// }
	// it.Release()

	for i := byte(0); i < 255; i++ {
		addr := common.BytesToAddress([]byte{i})
		state.SetBalance(addr, big.NewInt(11*int64(i)))
		val := state.GetBalance(addr)
		fmt.Println(i, val)
	}

	proof, _ := state.GetProofByHash(common.Hash{1, 1, 1, 0})
	fmt.Println(proof)

	// statedb, err := state.New(parent.Root, bc.stateCache, bc.snaps)
	// stateCache: state.NewDatabaseWithConfig(db, &trie.Config{
	// 	Cache:     cacheConfig.TrieCleanLimit,
	// 	Journal:   cacheConfig.TrieCleanJournal,
	// 	Preimages: cacheConfig.Preimages,
	// })

}
