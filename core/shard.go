package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"math/big"
)

type Shard interface {
	//GetShardID() uint32
	//GetBlockChain() *BlockChain
	//GetChainHeight() uint64
	//
	//SetInitialAccountState(map[common.Address]struct{}, *big.Int)
	//AddInitialAddr(addr common.Address, nodeID uint32)
	//GetNodeAddrs() []common.Address
	AddBlock(*Block)
	//HandleGetSyncData(*GetSyncData) *SyncData

	SetMessageHub(MessageHub)

	Start()
	Close()
	CanStopV1() bool
	CanStopV2() bool
	ConsensusStart(id uint32)
	WorkerStart()
	SetOldTxPool()
	UpdateTbChainHeight(height uint64)
	AdjustRecordedAddrs(addrs []common.Address, vrfs [][]byte, height uint64)
	SetPoolTx(tx *PoolTx)
	HandleGetPoolTx() *PoolTx

	AddInitialAddr(common.Address, uint32)
	GetNodeAddrs() []common.Address

	GetStateDB() *state.StateDB
	SetInitialAccountState(map[common.Address]struct{}, *big.Int)
}
