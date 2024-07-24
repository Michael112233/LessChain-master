package eth_shard

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/trie"
	"go-w3chain/beaconChain"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/eth_node"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type MultiSignData struct {
	MultiSignDone chan struct{}
	Vrfs          [][]byte
	Sigs          [][]byte
	Signers       []common.Address
}

type Shard struct {
	// execution layer
	Node *eth_node.EthNode

	blockchain *core.BlockChain

	messageHub      core.MessageHub
	initialAddrList []common.Address // 分片初始时各节点的公钥地址，同时也是初始时对应委员会各节点的地址

	// 状态树中一开始就已经创建了所有账户，但为了真实模拟同步时的状态树数据，用这个变量记录已经发生过交易的账户
	// 同步状态树的时候，只发送这些发生过交易的账户
	activeAddrs     map[common.Address]int
	height2Reconfig int
	tbchain_height  uint64

	// 同步模式是fastsync时，同步的区块数量
	fastsyncBlockNum int

	// consensus layer
	config *core.CommitteeConfig
	worker *Worker

	// multiData
	multiSignData *MultiSignData
	multiSignLock sync.Mutex

	txPool    *TxPool
	oldTxPool *TxPool // 重组前的交易池，新委员会leader请求时返回其中的交易

	/* 计数器，初始等于客户端个数，每一个客户端发送注入完成信号时计数器减一 */
	injectNotDone        int32
	to_reconfig          bool   // 收到特定高度的信标链区块后设为true，准备重组
	reconfig_seed_height uint64 // 用于重组的种子，所有委员会必须统一
}

// execution layer

// 共识层部分初始化
func ExecutionInitialize(shardID uint32, _node *eth_node.EthNode, height2Reconfig int) *Shard {
	// 获取节点的数据库
	chainDB := _node.GetDB()

	genesisBlock := core.DefaultGenesisBlock()
	genesisBlock.MustCommit(chainDB)

	chainConfig := &cfg.ChainConfig{
		ChainID: big.NewInt(int64(shardID)),
	}

	bc, err := core.NewBlockChain(chainDB, nil, chainConfig)
	if err != nil {
		log.Error("NewShard NewBlockChain err", "err", err)
	}

	log.Info("NewShard", "shardID", shardID,
		"nodeID", _node.NodeInfo.NodeID)

	shard := &Shard{
		Node:            _node,
		initialAddrList: []common.Address{*_node.GetAccount().GetAccountAddress()},
		blockchain:      bc,
		activeAddrs:     make(map[common.Address]int),
		height2Reconfig: height2Reconfig,
	}

	return shard
}

func (s *Shard) Start() {
	log.Debug("Execution Start")
	s.ExecutionStart()
}

func (s *Shard) ExecutionStart() {
	s.addGenesisTB()
}

// get shard id
func (s *Shard) GetShardID() uint32 {
	return s.Node.NodeInfo.ShardID
}

// 将创世区块写入信标链中
func (s *Shard) addGenesisTB() {
	/* 写入到信标链 */
	genesisBlock := s.blockchain.CurrentBlock()
	g_header := genesisBlock.GetHeader()
	tb := &core.TimeBeacon{
		Height:     g_header.Number.Uint64(),
		ShardID:    s.GetShardID(),
		BlockHash:  genesisBlock.GetHash().Hex(),
		TxHash:     g_header.TxHash.Hex(),
		StatusHash: g_header.Root.Hex(),
	}

	addrs := make([]common.Address, 0)
	for _, addr := range s.initialAddrList {
		addrs = append(addrs, addr)
	}

	genesis := &core.ShardSendGenesis{
		Addrs:           addrs,
		Gtb:             tb,
		ShardID:         s.Node.NodeInfo.ShardID,
		Target_nodeAddr: cfg.BooterAddr,
	}

	log.Debug("start to send genesis")
	s.messageHub.Send(core.MsgTypeShardSendGenesis, 0, genesis, nil)
}

func (s *Shard) AddBlock(block *core.Block) {
	s.blockchain.WriteBlock(block)
}

func (s *Shard) SetInitialAccountState(Addrs map[common.Address]struct{}, maxValue *big.Int) {
	statedb := s.blockchain.GetStateDB()
	for addr := range Addrs {
		statedb.AddBalance(addr, maxValue)
		if curValue := statedb.GetBalance(addr); curValue.Cmp(maxValue) != 0 {
			log.Error("Opps, something wrong!", "curValue", curValue, "Set maxValue", maxValue)
		}

	}
}

// 执行交易
func (s *Shard) executeTransactions(txs []*core.Transaction) common.Hash {
	stateDB := s.blockchain.GetStateDB()
	now := time.Now().Unix()

	// log.Debug(fmt.Sprintf("shardTrieRoot: %x", stateDB.IntermediateRoot(false)))
	for _, tx := range txs {
		s.executeTransaction(tx, stateDB, now)
		s.activeAddrs[*tx.Sender] += 1
		s.activeAddrs[*tx.Recipient] += 1
	}

	root := stateDB.IntermediateRoot(false)
	stateDB.Commit(false)

	return root
}

func (s *Shard) executeTransaction(tx *core.Transaction, stateDB *state.StateDB, now int64) {
	state := stateDB
	if tx.TXtype == core.IntraTXType {
		state.SetNonce(*tx.Sender, state.GetNonce(*tx.Sender)+1)
		state.SubBalance(*tx.Sender, tx.Value)
		state.AddBalance(*tx.Recipient, tx.Value)
	} else if tx.TXtype == core.CrossTXType1 {
		state.SetNonce(*tx.Sender, state.GetNonce(*tx.Sender)+1)
		state.SubBalance(*tx.Sender, tx.Value)
	} else if tx.TXtype == core.CrossTXType2 {
		state.AddBalance(*tx.Recipient, tx.Value)
	} else if tx.TXtype == core.RollbackTXType {
		state.SetNonce(*tx.Sender, state.GetNonce(*tx.Sender)-1)
		state.AddBalance(*tx.Sender, tx.Value)
	} else {
		log.Error("Oops, something wrong! Cannot handle tx type", "cur shardID", s.GetShardID(), "type", tx.TXtype, "tx", tx)
	}
}

// consensus layer
func (s *Shard) ConsensusInitialize(shardID uint32, clientCnt int, _node *eth_node.EthNode, config *core.CommitteeConfig) *Shard {
	shard := &Shard{
		config:        config,
		multiSignData: &MultiSignData{},
		Node:          _node,
		injectNotDone: int32(clientCnt),
		to_reconfig:   false,
	}
	log.Info("NewCommittee", "shardID", shardID, "nodeID", _node.NodeInfo.NodeID)

	return shard
}

// start and close
func (s *Shard) ConsensusStart(nodeId uint32) {
	s.to_reconfig = false
	if utils.IsComLeader(nodeId) { // 只有委员会的leader节点会运行worker，即出块
		pool := NewTxPool(s.Node.NodeInfo.ComID)
		s.txPool = pool
		pool.setCommittee(s)

		worker := newWorker(s.config)
		s.worker = worker
		worker.setCommittee(s)
	}
	log.Debug("shard.Start", "shardID", s.Node.NodeInfo.ComID)
}

func (s *Shard) WorkerStart() {
	s.worker.start()
}

func (s *Shard) ConsensusClose() {
	if s.worker != nil {
		s.worker.close()
	}
}

// Txpool Injection
func (s *Shard) SetInjectTXDone() {
	atomic.AddInt32(&s.injectNotDone, -1)
}

func (s *Shard) CanStopV1() bool {
	return s.CanStopV2() && s.txPool.Empty()
}

func (s *Shard) CanStopV2() bool {
	return s.injectNotDone == 0
}

func (s *Shard) TXpool() *TxPool {
	return s.txPool
}

func (s *Shard) getBlockHeight() *big.Int {
	var blockHeight *big.Int

	blockHeight = s.HandleComGetHeight()

	return blockHeight
}

/** 信标链主动向委员会推送新确认的信标时调用此函数
 * 一般情况下信标链应该只向委员会推送其关注的分片和高度的信标，这里进行了简化，默认全部推送
 */
func (s *Shard) ConsensusAddTBs(tbblock *beaconChain.TBBlock) {
	log.Debug(fmt.Sprintf("committee get tbchain confirm block... %v", tbblock))
	if tbblock.Height <= s.tbchain_height {
		return
	}
	s.tbchain_height = tbblock.Height

	// 收到特定高度的信标链区块后准备重组
	if s.tbchain_height > 0 && s.tbchain_height%uint64(s.config.Height2Reconfig) == 0 {
		s.reconfig_seed_height = s.tbchain_height
		s.to_reconfig = true
	}
}

func (s *Shard) ExecutionAddTBs(tbblock *beaconChain.TBBlock) {
	log.Debug(fmt.Sprintf("shard get tbchain confirm block... %v", tbblock))

	s.tbchain_height = tbblock.Height
}

func (s *Shard) AdjustRecordedAddrs(addrs []common.Address, vrfs [][]byte, seedHeight uint64) {
	data := &core.AdjustAddrs{
		ComID:      s.Node.NodeInfo.ComID,
		Addrs:      addrs,
		Vrfs:       vrfs,
		SeedHeight: seedHeight,
	}
	s.messageHub.Send(core.MsgTypeComSendNewAddrs, s.Node.NodeInfo.NodeID, data, nil)
}

func (s *Shard) HandleClientSendtx(txs []*core.Transaction) {
	if s.txPool == nil { // 交易池尚未创建，丢弃该交易
		return
	}
	s.txPool.AddTxs(txs)
}

func (s *Shard) send2Client(receipts map[uint64]*result.TXReceipt, txs []*core.Transaction) {
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
		s.messageHub.Send(core.MsgTypeCommitteeReply2Client, uint32(cid), msg2Client[cid], nil)
	}
}

func (s *Shard) getStatusFromShard(addrList []common.Address) *core.ShardSendState {
	request := &core.ComGetState{
		From_comID:     s.Node.NodeInfo.ComID,
		Target_shardID: s.Node.NodeInfo.ComID,
		AddrList:       addrList, // TODO: implement it
	}

	response := s.HandleComGetState(request)

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

func (s *Shard) AddBlock2Shard(block *core.Block) {
	comSendBlock := &core.ComSendBlock{
		Block: block,
	}
	s.HandleComSendBlock(comSendBlock)
}

func (s *Shard) SendTB(tb *core.SignedTB) {
	s.messageHub.Send(core.MsgTypeComAddTb2TBChain, s.Node.NodeInfo.NodeID, tb, nil)
}

func (s *Shard) GetEthChainBlockHash(height uint64) (common.Hash, uint64) {
	channel := make(chan struct{}, 1)
	var blockHash common.Hash
	var got_height uint64
	callback := func(ret ...interface{}) {
		blockHash = ret[0].(common.Hash)
		got_height = ret[1].(uint64)
		channel <- struct{}{}
	}
	s.messageHub.Send(core.MsgTypeGetBlockHashFromEthChain, s.Node.NodeInfo.ComID, height, callback)
	// 阻塞
	<-channel

	return blockHash, got_height
}

// reconfig
func (s *Shard) SetPoolTx(poolTx *core.PoolTx) {
	if s.txPool == nil {
		s.txPool = NewTxPool(s.Node.NodeInfo.ShardID)
	}
	s.txPool.lock.Lock()
	defer s.txPool.lock.Unlock()
	s.txPool.r_lock.Lock()
	defer s.txPool.r_lock.Unlock()

	s.txPool.SetPending(poolTx.Pending)
	s.txPool.SetPendingRollback(poolTx.PendingRollback)
	log.Debug(fmt.Sprintf("GetPoolTx pendingLen: %d pendingRollbackLen: %d", len(poolTx.Pending), len(poolTx.PendingRollback)))
}

func (s *Shard) HandleGetPoolTx() *core.PoolTx {
	s.txPool.lock.Lock()
	defer s.txPool.lock.Unlock()
	s.txPool.r_lock.Lock()
	defer s.txPool.r_lock.Unlock()
	poolTx := &core.PoolTx{
		Pending:         s.oldTxPool.pending,
		PendingRollback: s.oldTxPool.pendingRollback,
	}
	return poolTx
}

// 对oldTxpool的操作需要加上txpool的锁，防止多线程产生的错误
func (s *Shard) SetOldTxPool() {
	if s.txPool == nil {
		return
	}
	s.txPool.lock.Lock()
	defer s.txPool.lock.Unlock()
	s.txPool.r_lock.Lock()
	defer s.txPool.r_lock.Unlock()

	s.oldTxPool = s.txPool
	log.Debug("SetOldTxPool", "comID", s.Node.NodeInfo.ComID, "pendingLen", len(s.oldTxPool.pending), "rollbackLen", len(s.oldTxPool.pendingRollback))
}

func (s *Shard) UpdateTbChainHeight(height uint64) {
	if height > s.tbchain_height {
		s.tbchain_height = height
	}
}

func (s *Shard) NewBlockGenerated(block *core.Block) {
	if s.to_reconfig {
		// 关闭worker
		s.worker.exitCh <- struct{}{}

		seed, height := s.GetEthChainBlockHash(s.reconfig_seed_height)
		msg := &core.InitReconfig{
			Seed:       seed,
			SeedHeight: height,
			ComID:      s.Node.NodeInfo.ComID,
		}
		s.Node.InitReconfig(msg)
		s.to_reconfig = false
	}

}

func (s *Shard) SetMessageHub(hub core.MessageHub) {
	s.messageHub = hub
}

func (s *Shard) Close() {
	s.ConsensusClose()
}

func (s *Shard) AddInitialAddr(addr common.Address, nodeID uint32) {
	log.Debug(fmt.Sprintf("addInitialAddr... nodeID: %v addr: %x", nodeID, addr))
	s.initialAddrList = append(s.initialAddrList, addr)
}

func (s *Shard) GetNodeAddrs() []common.Address {
	return s.initialAddrList
}
