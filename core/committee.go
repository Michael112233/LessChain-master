package core

import "github.com/ethereum/go-ethereum/common"

type Committee interface {
	Start(nodeId uint32)
	Close()

	SetInjectTXDone(uint32)
	CanStopV1() bool
	CanStopV2() bool

	NewBlockGenerated(*Block)

	StartWorker()

	AdjustRecordedAddrs(addrs []common.Address, vrfs [][]byte, seedHeight uint64)
	SetPoolTx(*PoolTx)
	SetOldTxPool()
	HandleGetPoolTx(*GetPoolTx) *PoolTx
	UpdateTbChainHeight(height uint64)
}
