package beaconChain

import (
	"go-w3chain/core"
	"go-w3chain/log"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type ConfirmedTB struct {
	core.TimeBeacon
	ConfirmTime uint64
	/* 信标链上包含该信标的区块的高度，注意不是分片自身的区块高度 */
	ConfirmHeight uint64
}

type BeaconChain struct {
	cfg  *core.BeaconChainConfig
	mode int

	shardNum   int
	messageHub core.MessageHub

	/* key是分片ID，value是该分片每个高度区块的信标 */
	tbs  map[int][]*ConfirmedTB
	lock sync.Mutex
	/* 已提交到信标链但还未被信标链打包 */
	tbs_new  map[int][]*core.SignedTB
	lock_new sync.Mutex
	height   uint64
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// geth 私链最新高度的区块打包的信标
	geth_tbs_new map[uint64]map[uint32][]*core.TimeBeacon

	contract *Contract
	addrs    [][]common.Address

	contractAddr common.Address
	contractAbi  *abi.ABI

	tbBlocks map[uint64]*TBBlock
}

/** 新建一条信标链
 * required 表示一个信标需要收到的多签名最小数量
 */
func NewTBChain(cfg *core.BeaconChainConfig, shardNum int) *BeaconChain {
	tbChain := &BeaconChain{
		cfg:          cfg,
		mode:         cfg.Mode,
		shardNum:     shardNum,
		tbs:          make(map[int][]*ConfirmedTB),
		tbs_new:      make(map[int][]*core.SignedTB),
		geth_tbs_new: make(map[uint64]map[uint32][]*core.TimeBeacon),
		height:       0,
		stopCh:       make(chan struct{}),
		contract:     NewContract(shardNum, cfg.MultiSignRequiredNum),
		addrs:        make([][]common.Address, shardNum),
		tbBlocks:     make(map[uint64]*TBBlock),
	}
	log.Info("NewTBChain")
	tbChain.wg.Add(1)
	go tbChain.loop()
	return tbChain
}

func (tbChain *BeaconChain) Close() {
	close(tbChain.stopCh)
	tbChain.wg.Wait()
	log.Info("tbchain close")
}

func (tbChain *BeaconChain) SetMessageHub(hub core.MessageHub) {
	tbChain.messageHub = hub
}

func (tbChain *BeaconChain) AddTimeBeacon(tb *core.SignedTB, nodeID uint32) {
	if tbChain.mode == 0 {
		tbChain.AddTimeBeacon2SimulationChain(tb)
	} else if tbChain.mode == 1 || tbChain.mode == 2 {
		tbChain.AddTimeBeacon2EthChain(tb, nodeID)
	} else {
		log.Error("unknown beaconChain mode!", "mode", tbChain.mode)
	}
}

func (tbChain *BeaconChain) SetAddrs(addrs []common.Address, vrfs [][]byte, seedHeight uint64, comID uint32, nodeID uint32) {
	if tbChain.mode == 1 || tbChain.mode == 2 {
		tbChain.addrs[comID] = addrs
		if seedHeight > 0 {
			tbChain.AdjustEthChainRecordedAddrs(addrs, vrfs, seedHeight, comID, nodeID)
		}
	}
}

/** 调用这个函数，相当于在信标链上发起一笔交易
 * tb会被暂时存下，等待信标链打包时处理
 * 信标链打包时，会调用合约验证tb的多签名合法性，验证通过才会打包该交易，即确认该信标
 */
func (tbChain *BeaconChain) AddTimeBeacon2SimulationChain(tb *core.SignedTB) {
	if tb.Height == 0 {
		tbChain.AddGenesisTB(tb)
		return
	}
	tbChain.lock_new.Lock()
	defer tbChain.lock_new.Unlock()
	tbs_shard := tbChain.tbs[int(tb.ShardID)]
	tbs_shard_new := tbChain.tbs_new[int(tb.ShardID)]
	if tb.Height != uint64(len(tbs_shard)+len(tbs_shard_new)) {
		log.Warn("Could not add time beacon because the height didn't match!", "expected", len(tbs_shard)+len(tbs_shard_new), "got", tb.Height)
	}
	tbChain.tbs_new[int(tb.ShardID)] = append(tbChain.tbs_new[int(tb.ShardID)], tb)
	log.Debug("AddTimeBeacon", "info", tb)
}

func (tbChain *BeaconChain) AddGenesisTB(signedTb *core.SignedTB) {
	tbChain.lock.Lock()
	defer tbChain.lock.Unlock()
	tb := signedTb.TimeBeacon
	tbs_shard := tbChain.tbs[int(tb.ShardID)]
	if tb.Height != uint64(len(tbs_shard)) {
		log.Warn("Could not add time beacon because the height didn't match!", "expected", len(tbs_shard), "got", tb.Height)
	}
	confirmedTb := &ConfirmedTB{
		TimeBeacon:    tb,
		ConfirmTime:   uint64(time.Now().Unix()),
		ConfirmHeight: 0,
	}
	tbChain.tbs[int(tb.ShardID)] = append(tbChain.tbs[int(tb.ShardID)], confirmedTb)
	log.Debug("AddGenesisTimeBeacon", "info", tb)

}

func (tbChain *BeaconChain) GetTimeBeacon(shardID int, height uint64) *ConfirmedTB {
	tbs_shard := tbChain.tbs[shardID]
	if height >= uint64(len(tbs_shard)) {
		log.Warn("Could not get the time beacon because the requested height was larger than existing confirmed height!", "requested", height, "max height", len(tbs_shard))
		return nil
	}
	return tbs_shard[height]
}
