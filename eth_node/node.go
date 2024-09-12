package eth_node

import (
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/eth_chain"
	"go-w3chain/log"
	"go-w3chain/pbft"
	"go-w3chain/utils"
	"path/filepath"
	"strings"
	"sync"
)

type EthNode struct {
	NodeInfo         *core.NodeInfo
	nodeSendInfoLock sync.Mutex

	/** 该节点对应的账户 */
	w3Account *W3Account

	/** 存储该节点所有数据的目录，包括chaindata和keystore
	 * $Home/.w3Chain/shardi/nodej/
	 */
	DataDir string

	db ethdb.Database

	//shard         core.Shard
	//shardNum      int
	committeeNum  int
	CommitteeSize int // 委员会中所有节点，包括共识节点和非共识节点
	maxBlockSize  int

	com core.Shard

	oldTxPool *TxPool
	txPool    *TxPool

	/* 节点上一次运行vrf得到的结果 */
	VrfValue []byte
	pbftNode *pbft.PbftConsensusNode

	contractAddr common.Address
	contractAbi  *abi.ABI

	messageHub core.MessageHub

	reconfigMode          string
	reconfigResLock       sync.Mutex
	reconfigResult        *core.ReconfigResult                // 本节点的重组结果
	reconfigResults       []*core.ReconfigResult              // 本委员会所有节点的重组结果
	com2ReconfigResults   map[uint32]*core.ComReconfigResults // 所有委员会的节点的重组结果
	oppositeExecutionInfo *core.ExecutionInfo
	oldComID              uint32

	currentState *state.StateDB
	activeAddrs  map[common.Address]int
}

func NewNode(parentdataDir string, committeeNum, shardID, comID, nodeID, committeeSize, maxBlockSize int, reconfigMode string) *EthNode {
	nodeInfo := &core.NodeInfo{
		ShardID:  uint32(shardID),
		ComID:    uint32(comID),
		NodeID:   uint32(nodeID),
		NodeAddr: cfg.ComNodeTable[uint32(shardID)][uint32(nodeID)],
	}
	node := &EthNode{
		DataDir:       filepath.Join(parentdataDir, fmt.Sprintf("S%dN%d", shardID, nodeID)),
		committeeNum:  committeeNum,
		NodeInfo:      nodeInfo,
		maxBlockSize:  maxBlockSize,
		CommitteeSize: committeeSize,
		reconfigMode:  reconfigMode,
	}

	node.w3Account = NewW3Account(node.DataDir)
	printAccounts(node.w3Account)

	node.txPool = NewTxPool(node.NodeInfo.NodeID)
	node.txPool.setNode(node)

	db, err := node.OpenDatabase("chaindata", 0, 0, "", false)
	if err != nil {
		log.Error("open database fail", "nodeID", nodeID)
	}
	node.db = db

	// 节点刚创建时，shardID == ComID
	node.pbftNode = pbft.NewPbftNode(node.NodeInfo, uint32(4), "")

	return node
}

func (node *EthNode) HandleSendSync2Node(statelist map[common.Address]*types.StateAccount) {
	stateDB := node.com.GetStateDB()
	for addr, state := range statelist {
		stateDB.SetBalance(addr, state.Balance)
	}
	log.Info(fmt.Sprintf("finish sync to node %d com %d", node.NodeInfo.NodeID, node.NodeInfo.ComID))
}

func (node *EthNode) SetMessageHub(hub core.MessageHub) {
	node.messageHub = hub
}

func (node *EthNode) Start() {
	node.com.ConsensusStart(node.NodeInfo.NodeID)
	node.sendNodeInfo()
}

func (node *EthNode) sendNodeInfo() {
	if utils.IsComLeader(node.NodeInfo.NodeID) {
		return
	}
	info := &core.NodeSendInfo{
		NodeInfo: node.NodeInfo,
		Addr:     node.w3Account.accountAddr,
	}
	log.Debug(fmt.Sprintf("sendNodeInfo... addr: %x", info.Addr))
	node.messageHub.Send(core.MsgTypeNodeSendInfo2Leader, node.NodeInfo.ShardID, info, nil)
}

func (node *EthNode) sendTx2Com(txs []*core.Transaction) {
	//for {
	//	time.Sleep(1000 * time.Millisecond)
	//	txs, _ := node.txPool.Pending(node.maxBlockSize)
	//	log.Debug("SendTx2Com", "Start to send txs to committee:", len(txs))
	//	comGetTx := &core.ComGetTx{
	//		Txs:         txs,
	//		From_nodeID: node.GetNodeID(),
	//	}
	//	node.messageHub.Send(core.MsgTypeNodeSendTx2Com, node.NodeInfo.ShardID, comGetTx, nil)
	//	if node.txPool.Empty() {
	//		break
	//	}
	//}
	comGetTx := &core.ComGetTx{
		Txs:         txs,
		From_nodeID: node.GetNodeID(),
	}
	node.messageHub.Send(core.MsgTypeNodeSendTx2Com, node.NodeInfo.ShardID, comGetTx, nil)
}

func (node *EthNode) HandleClientSendtx(txs []*core.Transaction) {
	log.Debug(fmt.Sprintf("HandleClientSendtx txs=%v", len(txs)))
	if node.txPool == nil { // 交易池尚未创建，丢弃该交易
		return
	}
	//node.txPool.AddTxs(txs)
	//if len(txs) > 0 {
	node.sendTx2Com(txs)
	//}
}

func (node *EthNode) RunPbft(block *core.Block, exit chan struct{}) {
	node.pbftNode.SetSequenceID(block.NumberU64())
	node.pbftNode.Propose(block)
	// wait till consensus is complete

	select {
	case <-node.pbftNode.OneConsensusDone:
		log.Info("one consensus done")
		return
	case <-exit:
		exit <- struct{}{}
		return
	}

}

func (node *EthNode) SetShard(shard core.Shard) {
	node.com = shard
}

func (node *EthNode) GetShard() core.Shard {
	return node.com
}

func (node *EthNode) HandleGetOppositeShard() *core.Shard {
	return &(node.com)
}

func (node *EthNode) SetOldTxPool() {
	if node.txPool == nil {
		return
	}
	node.txPool.lock.Lock()
	defer node.txPool.lock.Unlock()
	node.txPool.r_lock.Lock()
	defer node.txPool.r_lock.Unlock()

	node.oldTxPool = node.txPool
	log.Debug("SetOldTxPool", "ShardID", node.NodeInfo.ShardID, "pendingLen", len(node.oldTxPool.pending), "rollbackLen", len(node.oldTxPool.pendingRollback))
}

func (node *EthNode) GetPbftNode() *pbft.PbftConsensusNode {
	return node.pbftNode
}

func (node *EthNode) Close() {
	log.Info("Time to close Eth Node")
	node.CloseDatabase()
}

// ResolvePath returns the absolute path of a resource in the instance directory.
func (n *EthNode) ResolvePath(x string) string {
	return filepath.Join(n.DataDir, x)
}

func (n *EthNode) GetDB() ethdb.Database {
	return n.db
}

func (n *EthNode) GetAddr() string {
	return n.NodeInfo.NodeAddr
}

func (n *EthNode) GetAccount() *W3Account {
	return n.w3Account
}

//func (n *Node) HandleNodeSendInfo(info *core.NodeSendInfo) {
//	n.nodeSendInfoLock.Lock()
//	defer n.nodeSendInfoLock.Unlock()
//
//	n.shard.AddInitialAddr(info.Addr, info.NodeInfo.NodeID)
//	if len(n.shard.GetNodeAddrs()) == int(n.comAllNodeNum) {
//		n.shard.Start()
//	}
//}

func (n *EthNode) HandleBooterSendContract(data *core.BooterSendContract) {
	n.contractAddr = data.Addr
	contractABI, err := abi.JSON(strings.NewReader(eth_chain.MyContractABI()))
	if err != nil {
		log.Error("get contracy abi fail", "err", err)
	}
	n.contractAbi = &contractABI
	// 启动 worker，满足三个条件： 1.是leader节点；2.收到合约地址；3.和委员会内所有节点建立起联系
	if utils.IsComLeader(n.NodeInfo.NodeID) {
		n.com.WorkerStart()
	}
}

//func (n *EthNode) ComGetState() *core.ShardSendState {
//
//	return nil
//}

/*
	//////////////////////////////////////////////////////////////
	节点的数据库相关的操作，包括打开、关闭等
	/////////////////////////////////////////////////////////////
*/

// OpenDatabase opens an existing database with the given name (or creates one if no
// previous can be found) from within the node'node instance directory.
func (n *EthNode) OpenDatabase(name string, cache, handles int, namespace string, readonly bool) (ethdb.Database, error) {
	// namepsace = "", file = /home/pengxiaowen/.brokerChain/xxx/name
	// cache , handle = 0, readonly = false
	var err error
	n.db, err = rawdb.NewLevelDBDatabase(n.ResolvePath(name), cache, handles, namespace, readonly)

	log.Trace("openDatabase", "node dataDir", n.DataDir)
	// log.Trace("Database", "node keyDir", n.keyDir)
	// log.Trace("Database", "node chaindata", n.ResolvePath(name))

	return n.db, err
}

func (n *EthNode) CloseDatabase() {
	err := n.db.Close()
	if err != nil {
		log.Error("closeDatabase fail.", "nodeInfo", n.NodeInfo)
	}
	// log.Debug("closeDatabase", "nodeID", n.NodeInfo.NodeID)
}

func (n *EthNode) GetNodeID() uint32 {
	return n.NodeInfo.NodeID
}

func (n *EthNode) HandleNodeSendInfo(info *core.NodeSendInfo) {
	n.nodeSendInfoLock.Lock()
	defer n.nodeSendInfoLock.Unlock()

	n.com.AddInitialAddr(info.Addr, info.NodeInfo.NodeID)
	log.Debug(fmt.Sprintf("%d %d\n", len(n.com.GetNodeAddrs()), n.CommitteeSize))
	if len(n.com.GetNodeAddrs()) == int(n.CommitteeSize) {
		n.com.Start()
	}
}
