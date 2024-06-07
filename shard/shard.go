package shard

import (
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/node"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

type Shard struct {
	Node *node.Node

	blockchain *core.BlockChain

	messageHub      core.MessageHub
	initialAddrList []common.Address // 分片初始时各节点的公钥地址，同时也是初始时对应委员会各节点的地址

	// 状态树中一开始就已经创建了所有账户，但为了真实模拟同步时的状态树数据，用这个变量记录已经发生过交易的账户
	// 同步状态树的时候，只发送这些发生过交易的账户
	activeAddrs map[common.Address]int
	// 自上一次重组后的交易中的账户
	tMPT_activeAddrs map[common.Address]int
	height2Reconfig  int
	tbchain_height   uint64

	// 同步模式是fastsync时，同步的区块数量
	fastsyncBlockNum int
}

func (s *Shard) AddInitialAddr(addr common.Address, nodeID uint32) {
	log.Debug(fmt.Sprintf("addInitialAddr... nodeID: %v addr: %x", nodeID, addr))
	s.initialAddrList = append(s.initialAddrList, addr)
}

func (s *Shard) GetNodeAddrs() []common.Address {
	return s.initialAddrList
}

func NewShard(shardID uint32, _node *node.Node, fastsyncBlockNum int, height2Reconfig int) *Shard {
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
		Node:             _node,
		initialAddrList:  []common.Address{*_node.GetAccount().GetAccountAddress()},
		blockchain:       bc,
		activeAddrs:      make(map[common.Address]int),
		tMPT_activeAddrs: make(map[common.Address]int),
		height2Reconfig:  height2Reconfig,
		fastsyncBlockNum: fastsyncBlockNum,
	}

	return shard
}

func (shard *Shard) GetShardID() uint32 {
	return shard.Node.NodeInfo.ShardID
}

func (shard *Shard) AddBlock(block *core.Block) {
	shard.blockchain.WriteBlock(block)
}

func (shard *Shard) GetBlockChain() *core.BlockChain {
	return shard.blockchain
}

func (shard *Shard) GetChainHeight() uint64 {
	return shard.blockchain.GetChainHeight()
}

func (shard *Shard) SetInitialAccountState(Addrs map[common.Address]struct{}, maxValue *big.Int) {
	statedb := shard.blockchain.GetStateDB()
	for addr := range Addrs {
		statedb.AddBalance(addr, maxValue)
		if curValue := statedb.GetBalance(addr); curValue.Cmp(maxValue) != 0 {
			log.Error("Opps, something wrong!", "curValue", curValue, "Set maxValue", maxValue)
		}

	}
}

func (s *Shard) SetMessageHub(hub core.MessageHub) {
	s.messageHub = hub
}

func (s *Shard) Start() {
	s.addGenesisTB()
}

func (s *Shard) Close() {
	// todo: fill it
}

/**
 * 将创世区块的信标写到信标链
 */
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

	s.messageHub.Send(core.MsgTypeShardSendGenesis, 0, genesis, nil)
}

/////////////////////////////////////////////////
///// 原本属于committee中worker的函数
/////////////////////////////////////////////////

/*





 */
////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////
// worker内部处理交易的函数
//////////////////////////////////////////

// /* 生成交易收据, 发送给客户端 */
// func (w *Worker) sendTXReceipt2Client(txs []*core.Transaction) {
// 	table := make(map[uint64]*result.TXReceipt)
// 	for _, tx := range txs {
// 		if tx.TXStatus == result.DefaultStatus {
// 			log.Error("record tx status miss!", "tx", tx)
// 		} else {
// 			table[tx.ID] = &result.TXReceipt{
// 				TxID:             tx.ID,
// 				ConfirmTimeStamp: tx.ConfirmTimestamp,
// 				TxStatus:         tx.TXStatus,
// 				ShardID:          int(w.comID),
// 				BlockHeight:      w.curHeight.Uint64(),
// 			}
// 		}
// 	}
// 	w.com.send2Client(table, txs)
// 	// result.SetTXReceiptV2(table)
// }

// /**
//  * 通知committee 有新区块产生
//  * 当committee触发重组时，该方法会被阻塞，进而导致worker被阻塞，直到重组完成
//  */
// func (w *Worker) InformNewBlock(block *core.Block) {
// 	w.com.NewBlockGenerated(block)
// }

// /* 生成区块，执行区块中的交易，确认状态转移，发送区块到分片，发送收据到客户端 */
// func (w *Worker) commit(timestamp int64) (*core.Block, error) {
// 	parentHeight := w.com.getStatusFromShard()
// 	stateDB := &state.StateDB{}
// 	log.Debug("com getStatusFromShard", "stateDB", stateDB, "parentHeight", parentHeight)

// 	w.curHeight = parentHeight.Add(parentHeight, common.Big1)
// 	header := &core.Header{
// 		Difficulty: math.BigPow(11, 11),
// 		Number:     w.curHeight,
// 		Time:       uint64(timestamp),
// 		ShardID:    uint64(w.comID),
// 	}

// 	txs := w.com.txPool.Pending(w.config.MaxBlockSize, parentHeight)

// 	w.commitTransactions(txs, stateDB)
// 	/* commit and insert to blockchain */
// 	block, err := w.Finalize(header, txs, stateDB)
// 	if err != nil {
// 		return nil, errors.New("failed to commit transition state: " + err.Error())
// 	}

// 	w.com.AddBlock2Shard(block)
// 	/* 生成交易收据, 并发送到客户端 */
// 	w.sendTXReceipt2Client(txs)

// 	log.Debug("create block", "comID", w.comID, "block Height", header.Number, "# tx", len(txs), "txpoolLen", w.com.txPool.PendingLen()+w.com.TXpool().PendingRollbackLen())
// 	// log.Trace("create block", "comID", w.comID, "block Height", header.Number, "#TX", len(txs))

// 	return block, nil

// }

// /**
//  * 将更新的stateObjects写到MPT树上，得到新树根，并写到区块头中。
//  * 根据交易列表得到交易树根，并写到区块头中
//  * 根据区块头和交易列表构造区块
//  */
// func (w *Worker) Finalize(header *core.Header, txs []*core.Transaction, stateDB *state.StateDB) (*core.Block, error) {
// 	state := stateDB
// 	hashroot, err := state.Commit(false)
// 	if err != nil {
// 		return nil, err
// 	}
// 	header.Root = hashroot
// 	block := core.NewBlock(header, txs, trie.NewStackTrie(nil))
// 	return block, nil

// }

func (s *Shard) AddTBs(tbblock *beaconChain.TBBlock) {
	log.Debug(fmt.Sprintf("shard get tbchain confirm block... %v", tbblock))

	s.tbchain_height = tbblock.Height
}

/*
* 执行打包的交易，更新stateObjects
 */
func (s *Shard) executeTransactions(txs []*core.Transaction) common.Hash {
	stateDB := s.blockchain.GetStateDB()
	now := time.Now().Unix()

	// log.Debug(fmt.Sprintf("shardTrieRoot: %x", stateDB.IntermediateRoot(false)))
	for _, tx := range txs {
		s.executeTransaction(tx, stateDB, now)
		s.activeAddrs[*tx.Sender] += 1
		s.activeAddrs[*tx.Recipient] += 1
		s.tMPT_activeAddrs[*tx.Sender] += 1
		s.tMPT_activeAddrs[*tx.Recipient] += 1
	}

	root := stateDB.IntermediateRoot(false)
	stateDB.Commit(false)

	// log.Debug("ShardAccountState")
	// for _, tx := range txs {
	// 	log.Debug(fmt.Sprintf("txType: %v", core.TxTypeStr(tx.TXtype)))
	// 	log.Debug(fmt.Sprintf("accountHash: %x  data : %v  value: %v", getHash((*tx.Sender)[:]), stateDB.GetNonce(*tx.Sender), stateDB.GetBalance(*tx.Sender)))
	// 	log.Debug(fmt.Sprintf("accountHash: %x  data : %v  value: %v", getHash((*tx.Recipient)[:]), stateDB.GetNonce(*tx.Recipient), stateDB.GetBalance(*tx.Recipient)))
	// }

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

func IterateOverTrie(stateDB *state.StateDB) {
	database := stateDB.Database().TrieDB()

	root := stateDB.IntermediateRoot(false)
	log.Debug("stateTrie rootHash", "data", root)
	// 将更改写入到数据库中
	stateDB.Commit(false)
	// stateDB中用的是secureTrie，所以要创建secureTrie实例，而不是Trie
	stateTrie, err := trie.NewSecure(root, database)
	if err != nil {
		log.Error("trie.NewSecure error", "err", err, "trieRoot", root)
	}

	it := stateTrie.NodeIterator([]byte{})
	for it.Next(true) {
		if it.Leaf() {
			var acc types.StateAccount
			if err := rlp.DecodeBytes(it.LeafBlob(), &acc); err != nil {
				log.Error(fmt.Sprintf("decode err: %v", err))
			}
			addrHash := it.LeafKey()
			// balance := new(big.Int).Set(acc.Balance)
			log.Debug(fmt.Sprintf("Address: %x account data: %v", addrHash, acc))
		}
	}
}
