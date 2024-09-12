package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
)

type Shard interface {
	SetAllAddrs([]common.Address)
	AddBlock(*Block)
	HandleGetSyncData(*GetSyncData) *SyncData
	CommitSyncInfo(txs []*Transaction, addrlist []common.Address)
	SetMessageHub(MessageHub)

	Start()
	Close()
	CanStopV1() bool
	CanStopV2() bool
	ConsensusStart(id uint32)
	ExecutionReset(info *ExecutionInfo)
	WorkerStart()
	SetOldTxPool()
	UpdateTbChainHeight(height uint64)
	AdjustRecordedAddrs(addrs []common.Address, vrfs [][]byte, height uint64)
	SetPoolTx(tx *PoolTx)
	HandleGetPoolTx() *PoolTx
	HandleComGetState(*ComGetState) *ShardSendState

	AddInitialAddr(common.Address, uint32)
	GetNodeAddrs() []common.Address

	GetStateDB() *state.StateDB
	SetInitialAccountState(map[common.Address]struct{}, *big.Int)

	HandleLeaderTX([]*Transaction, uint32)
}
