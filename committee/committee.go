package committee

import (
	"bytes"
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/node"
	"go-w3chain/result"
	"go-w3chain/utils"
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/trie"
)

type MultiSignData struct {
	MultiSignDone chan struct{}
	Vrfs          [][]byte
	Sigs          [][]byte
	Signers       []common.Address
}

type Committee struct {
	config     *core.CommitteeConfig
	worker     *Worker
	messageHub core.MessageHub

	multiSignData *MultiSignData
	multiSignLock sync.Mutex

	Node      *node.Node // 当前节点
	txPool    *TxPool
	oldTxPool *TxPool // 重组前的交易池，新委员会leader请求时返回其中的交易
	/* 计数器，初始等于客户端个数，每一个客户端发送注入完成信号时计数器减一 */
	injectNotDone        int32
	tbchain_height       uint64
	to_reconfig          bool   // 收到特定高度的信标链区块后设为true，准备重组
	reconfig_seed_height uint64 // 用于重组的种子，所有委员会必须统一

	shardSendStateChan chan *core.ShardSendState
}

func NewCommittee(comID uint32, clientCnt int, _node *node.Node, config *core.CommitteeConfig) *Committee {
	com := &Committee{
		config:             config,
		multiSignData:      &MultiSignData{},
		Node:               _node,
		injectNotDone:      int32(clientCnt),
		to_reconfig:        false,
		shardSendStateChan: make(chan *core.ShardSendState, 0),
	}
	log.Info("NewCommittee", "comID", comID, "nodeID", _node.NodeInfo.NodeID)

	return com
}

func (com *Committee) Start(nodeId uint32) {
	com.to_reconfig = false        // 防止重组后该值一直为true
	if utils.IsComLeader(nodeId) { // 只有委员会的leader节点会运行worker，即出块
		pool := NewTxPool(com.Node.NodeInfo.ComID)
		com.txPool = pool
		pool.setCommittee(com)

		worker := newWorker(com.config)
		com.worker = worker
		worker.setCommittee(com)
	}
	log.Debug("com.Start", "comID", com.Node.NodeInfo.ComID)
}

func (com *Committee) StartWorker() {
	com.worker.start()
}

func (com *Committee) Close() {
	if com.worker != nil {
		com.worker.close()
	}
}

func (com *Committee) SetInjectTXDone(cid uint32) {
	atomic.AddInt32(&com.injectNotDone, -1)
}

/* 交易注入完成且交易池空即可停止 */
func (com *Committee) CanStopV1() bool {
	return com.CanStopV2() && com.txPool.Empty()
}

/* 交易注入完成即可停止 */
func (com *Committee) CanStopV2() bool {
	return com.injectNotDone == 0
}

/**
 * 刚出完一个块判断是否达到重组条件
 * 当committee触发重组时，会在该方法会被阻塞，直到重组完成
 */
func (com *Committee) NewBlockGenerated(block *core.Block) {
	if com.to_reconfig {
		// 关闭worker
		com.worker.exitCh <- struct{}{}

		seed, height := com.GetEthChainBlockHash(com.reconfig_seed_height)
		msg := &core.InitReconfig{
			Seed:       seed,
			SeedHeight: height,
			ComID:      com.Node.NodeInfo.ComID,
		}
		com.Node.InitReconfig(msg)
		com.to_reconfig = false
	}

}

func (com *Committee) TXpool() *TxPool {
	return com.txPool
}

/*





 */

//////////////////////////////////////////
// committee 通过 messageHub 与外界通信的函数，以及被 messageHub 调用的函数
//////////////////////////////////////////

func (com *Committee) SetMessageHub(hub core.MessageHub) {
	com.messageHub = hub
}

/** 信标链主动向委员会推送新确认的信标时调用此函数
 * 一般情况下信标链应该只向委员会推送其关注的分片和高度的信标，这里进行了简化，默认全部推送
 */
func (com *Committee) AddTBs(tbblock *beaconChain.TBBlock) {
	// for shardID, tbs := range tbs_new {
	// 	for _, tb := range tbs {
	// 		c.tbs[shardID][tb.Height] = tb
	// 	}
	// }
	log.Debug(fmt.Sprintf("committee get tbchain confirm block... %v", tbblock))
	if tbblock.Height <= com.tbchain_height {
		return
	}
	com.tbchain_height = tbblock.Height

	// 收到特定高度的信标链区块后准备重组
	if com.tbchain_height > 0 && com.tbchain_height%uint64(com.config.Height2Reconfig) == 0 {
		com.reconfig_seed_height = com.tbchain_height
		com.to_reconfig = true
	}
}

/* 向信标链发起交易，更新委员会地址列表
 */
func (com *Committee) AdjustRecordedAddrs(addrs []common.Address, vrfs [][]byte, seedHeight uint64) {
	data := &core.AdjustAddrs{
		ComID:      com.Node.NodeInfo.ComID,
		Addrs:      addrs,
		Vrfs:       vrfs,
		SeedHeight: seedHeight,
	}
	com.messageHub.Send(core.MsgTypeComSendNewAddrs, com.Node.NodeInfo.NodeID, data, nil)
}

func (com *Committee) HandleClientSendtx(txs []*core.Transaction) {
	if com.txPool == nil { // 交易池尚未创建，丢弃该交易
		return
	}
	com.txPool.AddTxs(txs)
}

/**
 * 向客户端发送交易收据
 * 目前未实现通过网络传输，都是基于messageHub转发
 */
func (com *Committee) send2Client(receipts map[uint64]*result.TXReceipt, txs []*core.Transaction) {
	// 分客户端
	msg2Client := make(map[int][]*result.TXReceipt)
	for _, tx := range txs {
		cid := int(tx.Cid)
		if _, ok := msg2Client[cid]; !ok {
			msg2Client[cid] = make([]*result.TXReceipt, 0, len(receipts))
		}
		if r, ok := receipts[tx.ID]; ok {
			msg2Client[cid] = append(msg2Client[cid], r)
		}
	}
	for cid := range msg2Client {
		com.messageHub.Send(core.MsgTypeCommitteeReply2Client, uint32(cid), msg2Client[cid], nil)
	}
}

func (com *Committee) getBlockHeight() *big.Int {
	var blockHeight *big.Int
	callback := func(res ...interface{}) {
		blockHeight = res[0].(*big.Int)
	}

	data := &core.ComGetHeight{
		From_comID:     com.Node.NodeInfo.ComID,
		Target_shardID: com.Node.NodeInfo.ComID,
	}
	com.messageHub.Send(core.MsgTypeComGetHeightFromShard, com.Node.NodeInfo.ComID, data, callback)

	return blockHeight
}

/* 从对应的分片获取账户状态和证明
 */
func (com *Committee) getStatusFromShard(addrList []common.Address) *core.ShardSendState {
	request := &core.ComGetState{
		From_comID:     com.Node.NodeInfo.ComID,
		Target_shardID: com.Node.NodeInfo.ComID,
		AddrList:       addrList, // TODO: implement it
	}
	com.messageHub.Send(core.MsgTypeComGetStateFromShard, com.Node.NodeInfo.ComID, request, nil)

	response := <-com.shardSendStateChan

	// Validate Merkle proofs for each address
	rootHash := response.StatusTrieHash

	for address, accountData := range response.AccountData {
		proofDB := &proofReader{proof: response.AccountsProofs[address]}

		computedValue, err := trie.VerifyProof(rootHash, utils.GetHash(address.Bytes()), proofDB)
		if err != nil {
			log.Error("Failed to verify Merkle proof", "err", err, "address", address)
			return nil
		}
		if !bytes.Equal(computedValue, accountData) {
			log.Error("Merkle proof verification failed for address", "address", address)
			return nil
		}
	}

	log.Info("getStatusFromShard and verify merkle proof succeed.")

	return response
}

func (com *Committee) HandleShardSendState(response *core.ShardSendState) {
	com.shardSendStateChan <- response
}

/**
 * 将新区块发送给对应分片
 */
func (com *Committee) AddBlock2Shard(block *core.Block) {
	comSendBlock := &core.ComSendBlock{
		Block: block,
	}
	com.messageHub.Send(core.MsgTypeSendBlock2Shard, com.Node.NodeInfo.ComID, comSendBlock, nil)
}

/**
 * 将新区块的信标发送到信标链
 */
func (com *Committee) SendTB(tb *core.SignedTB) {
	com.messageHub.Send(core.MsgTypeComAddTb2TBChain, com.Node.NodeInfo.NodeID, tb, nil)
}

func (com *Committee) GetEthChainBlockHash(height uint64) (common.Hash, uint64) {
	channel := make(chan struct{}, 1)
	var blockHash common.Hash
	var got_height uint64
	callback := func(ret ...interface{}) {
		blockHash = ret[0].(common.Hash)
		got_height = ret[1].(uint64)
		channel <- struct{}{}
	}
	com.messageHub.Send(core.MsgTypeGetBlockHashFromEthChain, com.Node.NodeInfo.ComID, height, callback)
	// 阻塞
	<-channel

	return blockHash, got_height
}

/*
	////////////////////////////////////////////////

	///////////////////////////////////////////////

*/

func (com *Committee) GetCommitteeID() uint32 {
	return com.Node.NodeInfo.ComID
}

// 该方法仅在重组后同步交易池时使用
// 由于接收到旧leader的交易之前，新leader已经接收到client发送的交易，所以需要将旧leader交易插到队列最前
func (com *Committee) SetPoolTx(poolTx *core.PoolTx) {
	if com.txPool == nil {
		com.txPool = NewTxPool(com.Node.NodeInfo.ShardID)
	}
	com.txPool.lock.Lock()
	defer com.txPool.lock.Unlock()
	com.txPool.r_lock.Lock()
	defer com.txPool.r_lock.Unlock()

	com.txPool.SetPending(poolTx.Pending)
	com.txPool.SetPendingRollback(poolTx.PendingRollback)
	log.Debug(fmt.Sprintf("GetPoolTx pendingLen: %d pendingRollbackLen: %d", len(poolTx.Pending), len(poolTx.PendingRollback)))
}

func (com *Committee) HandleGetPoolTx(request *core.GetPoolTx) *core.PoolTx {
	com.txPool.lock.Lock()
	defer com.txPool.lock.Unlock()
	com.txPool.r_lock.Lock()
	defer com.txPool.r_lock.Unlock()
	poolTx := &core.PoolTx{
		Pending:         com.oldTxPool.pending,
		PendingRollback: com.oldTxPool.pendingRollback,
	}
	return poolTx
}

// 对oldTxpool的操作需要加上txpool的锁，防止多线程产生的错误
func (com *Committee) SetOldTxPool() {
	if com.txPool == nil {
		return
	}
	com.txPool.lock.Lock()
	defer com.txPool.lock.Unlock()
	com.txPool.r_lock.Lock()
	defer com.txPool.r_lock.Unlock()

	com.oldTxPool = com.txPool
	log.Debug("SetOldTxPool", "comID", com.Node.NodeInfo.ComID, "pendingLen", len(com.oldTxPool.pending), "rollbackLen", len(com.oldTxPool.pendingRollback))
}

func (com *Committee) UpdateTbChainHeight(height uint64) {
	if height > com.tbchain_height {
		com.tbchain_height = height
	}
}
