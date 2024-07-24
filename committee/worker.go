package committee

import (
	"errors"
	"fmt"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/trie"
	"go-w3chain/utils"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	myTrie "go-w3chain/trie"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	minRecommitInterval = 3 * time.Second
)

type Worker struct {
	config *core.CommitteeConfig

	// Channels
	startCh chan struct{}
	exitCh  chan struct{}
	// headerCh chan<- struct{} // send to shard

	// atomic status counters
	running int32 // The indicator whether the consensus engine is running or not.

	wg sync.WaitGroup

	curHeight *big.Int

	com *Committee
}

func newWorker(config *core.CommitteeConfig) *Worker {
	worker := &Worker{
		config:  config,
		startCh: make(chan struct{}, 1), // at most 1 element
		exitCh:  make(chan struct{}, 1),
	}

	// Sanitize recommit interval if the user-specified one is too short.
	recommitTime := worker.config.RecommitTime
	if recommitTime < minRecommitInterval {
		log.Error("recommit interval too short", "provided interval", recommitTime, "min interval supported", minRecommitInterval)
	}

	worker.wg.Add(1)
	go worker.newWorkLoop(recommitTime)

	return worker
}

func (w *Worker) setCommittee(com *Committee) {
	w.com = com
}

//////////////////////////////////////////
// worker 生命周期函数
//////////////////////////////////////////

// start sets the running status as 1 and triggers new work submitting.
func (w *Worker) start() {
	// log.Info("worker start")
	atomic.StoreInt32(&w.running, 1)
	w.startCh <- struct{}{}
}

// stop sets the running status as 0.
func (w *Worker) stop() {
	atomic.StoreInt32(&w.running, 0)
}

// isRunning returns an indicator whether worker is running or not.
func (w *Worker) isRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// close terminates all background threads maintained by the worker.
// Note the worker does not support being closed multiple times.
func (w *Worker) close() {
	log.Debug("closing worker of this committee..", "comID", w.com.Node.NodeInfo.ComID)
	w.stop()
	w.exitCh <- struct{}{}
	w.wg.Wait()
	log.Debug("worker of this committee has been close!", "comID", w.com.Node.NodeInfo.ComID)
}

// newWorkLoop is a standalone goroutine to submit new sealing work upon received events.
func (w *Worker) newWorkLoop(recommit time.Duration) {
	defer w.wg.Done()
	var (
		timestamp int64
	)

	timer := time.NewTimer(0)
	defer timer.Stop()
	<-timer.C // discard the initial tick

	// commit aborts in-flight transaction execution with given signal and resubmits a new one.
	commit := func() {
		block, err := w.commit(timestamp)
		if err != nil {
			log.Error("worker commit block failed", "err", err)
		}

		// 获取信标链已确认的最新区块哈希和高度
		seed, height := w.com.GetEthChainBlockHash(w.com.tbchain_height)
		log.Debug(fmt.Sprint("com GetEthChainBlockHash"))

		w.broadcastTbInCommittee(block, seed, height)

		/* 通知committee 有新区块产生
		   当出完一个块需要重组时，worker会阻塞在这个函数内
		*/
		w.com.NewBlockGenerated(block)

		// 如果有重组，应在重组完成后再开始打包交易
		timer.Reset(recommit)

	}

	for {
		select {
		case <-w.exitCh:
			// log.Info("close worker..")
			// log.Debug("worker exitch", "comID", w.chain.GetChainID())
			return

		case <-w.startCh:
			// log.Debug("worker startch", "comID", w.chain.GetChainID())
			timer.Reset(recommit)

		case <-timer.C:
			// log.Debug("worker timer.c", "comID", w.chain.GetChainID())
			if w.isRunning() {
				timestamp = time.Now().Unix()
				commit()
			}

			// default:
			// 	log.Debug("worker default", "comID", w.chain.GetChainID())
			// 	time.Sleep(1000 * time.Millisecond)
		}
	}

}

/*





 */
////////////////////////////////////////////////////////////////////////////////
//////////////////////////////////////////
// worker内部处理交易的函数
//////////////////////////////////////////

/** 矿工出块以后，生成对应的区块信标，并广播到委员会中
 * 由委员会对象代表委员会中的节点，直接调用委员会的多签名方法
 */
func (w *Worker) broadcastTbInCommittee(block *core.Block, seed common.Hash, height uint64) {
	final_header := block.GetHeader()
	tb := &core.TimeBeacon{
		Height:     final_header.Number.Uint64(),
		ShardID:    uint32(w.com.Node.NodeInfo.ComID),
		BlockHash:  block.GetHash().Hex(),
		TxHash:     final_header.TxHash.Hex(),
		StatusHash: final_header.Root.Hex(),
	}

	signedTB := w.com.initMultiSign(tb, seed, height)

	w.com.SendTB(signedTB)
}

/* 生成交易收据, 发送给客户端 */
func (w *Worker) sendTXReceipt2Client(txs []*core.Transaction) {
	table := make(map[uint64]*result.TXReceipt)
	for _, tx := range txs {
		if tx.TXStatus == result.DefaultStatus {
			log.Error("record tx status miss!", "tx", tx)
		} else {
			table[tx.ID] = &result.TXReceipt{
				TxID:             tx.ID,
				ConfirmTimeStamp: tx.ConfirmTimestamp,
				TxStatus:         tx.TXStatus,
				ShardID:          int(w.com.Node.NodeInfo.ComID),
				BlockHeight:      w.curHeight.Uint64(),
			}
		}
	}
	w.com.send2Client(table, txs)
	// result.SetTXReceiptV2(table)
}

func analyseStates(states *core.ShardSendState) (map[common.Address]*types.StateAccount, map[string]trie.Node) {
	// 每个账户对应的具体状态
	addr2State := make(map[common.Address]*types.StateAccount)
	// merkle 路径上每个节点与其哈希的映射
	hash2Node := make(map[string]myTrie.Node)
	for addr, encodedState := range states.AccountData {
		var state types.StateAccount
		err := rlp.DecodeBytes(encodedState, &state)
		if err != nil {
			log.Error(fmt.Sprintf("rlp encode fail. err: %v", err))
		}
		addr2State[addr] = &state

		proofs := states.AccountsProofs[addr]
		for _, proof := range proofs {
			encodedNode := proof
			hash := utils.GetHash(encodedNode)
			// log.Debug(fmt.Sprintf("proof hash: %x", hash))
			if _, ok := hash2Node[string(hash[:])]; ok { // 已经解析和存储过该node
				continue
			}
			node := myTrie.MustDecodeNode(hash, encodedNode)
			hash2Node[string(hash[:])] = node
			// switch node.(type) {
			// case *myTrie.FullNode:
			// 	fullNode := node.(*myTrie.FullNode)
			// 	log.Debug(fmt.Sprintf("node type: %v  data: %v", "fullnode", fullNode.String()))
			// case *myTrie.ShortNode:
			// 	shortNode := node.(*myTrie.ShortNode)
			// 	log.Debug(fmt.Sprintf("node type: %v  key (nibble): %v  value: %v", "shortnode", shortNode.Key, shortNode.Val))
			// default:
			// 	log.Error(fmt.Sprintf("unexpected node type")) // proof中应该也不会出现valuenode或hashnode
			// }
		}
	}

	return addr2State, hash2Node
}

/* 生成区块，执行区块中的交易，确认状态转移，发送区块到分片，发送收据到客户端 */
func (w *Worker) commit(timestamp int64) (*core.Block, error) {
	// 获取分片最新的区块高度
	parentHeight := w.com.getBlockHeight()
	// 从交易池选取交易，排除掉超时的跨分片交易
	txs, addrs := w.com.txPool.Pending(w.config.MaxBlockSize, parentHeight)
	// 从分片获取交易相关账户的状态及证明
	states := w.com.getStatusFromShard(addrs)
	// 解析状态及证明
	addr2State, hash2Node := analyseStates(states)
	// 执行交易，更改账户状态
	updatedStates := make(map[string]*types.StateAccount) // 注意，key不是地址，是地址的哈希
	w.executeTransactions(txs, addr2State, updatedStates)

	/* commit and insert to blockchain */
	w.curHeight = parentHeight.Add(parentHeight, common.Big1)
	header := &core.Header{
		Difficulty: math.BigPow(11, 11),
		Number:     w.curHeight,
		Time:       uint64(timestamp),
		ShardID:    uint64(w.com.Node.NodeInfo.ComID),
	}
	block, err := w.Finalize(header, txs, hash2Node, states.StatusTrieHash, updatedStates)
	if err != nil {
		return nil, errors.New("failed to commit transition state: " + err.Error())
	}

	// pbft consensus in committee
	log.Debug(fmt.Sprintf("start running pbft... comID: %d", w.com.Node.NodeInfo.ComID))
	w.com.Node.RunPbft(block, w.exitCh)
	log.Debug(fmt.Sprintf("pbft done... comID: %d", w.com.Node.NodeInfo.ComID))

	// log.Debug("WorkerAccountState")
	// for _, tx := range txs {
	// 	log.Debug(fmt.Sprintf("tx type: %v", core.TxTypeStr(tx.TXtype)))
	// 	log.Debug(fmt.Sprintf("accountHash: %x  value: %v", utils.GetHash((*tx.Sender)[:]), addr2State[*tx.Sender]))
	// 	log.Debug(fmt.Sprintf("accountHash: %x  value: %v", utils.GetHash((*tx.Recipient)[:]), addr2State[*tx.Recipient]))
	// }

	w.com.AddBlock2Shard(block)
	/* 生成交易收据, 并发送到客户端 */
	w.sendTXReceipt2Client(txs)

	log.Debug("create block", "comID", w.com.Node.NodeInfo.ComID, "block Height", header.Number, "# tx", len(txs), "txpoolLen", w.com.txPool.PendingLen()+w.com.TXpool().PendingRollbackLen())
	// log.Trace("create block", "comID", w.com.Node.NodeInfo.ComID, "block Height", header.Number, "#TX", len(txs))

	return block, nil
}

/**
 * 自底向上哈希，得到新树根，并写到区块头中。
 * 根据交易列表得到交易树根，并写到区块头中
 * 根据区块头和交易列表构造区块
 */
func (w *Worker) Finalize(
	header *core.Header,
	txs []*core.Transaction,
	hash2Node map[string]myTrie.Node,
	trieRoot common.Hash,
	updadedStates map[string]*types.StateAccount,
) (*core.Block, error) {
	// 新的状态树根
	newTireRoot := rebuildTrie(trieRoot, hash2Node, updadedStates)

	header.Root = newTireRoot
	block := core.NewBlock(header, txs, trie.NewStackTrie(nil))
	return block, nil

}

func rebuildTrie(
	trieRoot common.Hash,
	hash2Node map[string]myTrie.Node,
	updadedStates map[string]*types.StateAccount,
) (rootHash common.Hash) {
	log.Debug(fmt.Sprintf("start rebuilding trie from root"))
	rootHash = common.BytesToHash(rebuildHelper(trieRoot[:], hash2Node, []byte{}, updadedStates))
	log.Debug(fmt.Sprintf("trie rebuild done. original trie root: %x  new trie root: %x", trieRoot, rootHash))
	return
}

// 获得由树根到当前节点的key，注意是nibble
func getCurrentKey(prefix []byte, nodeKey []byte) []byte {
	key := make([]byte, len(prefix))
	copy(key, prefix)
	key = append(key, nodeKey...)
	return key
}

func rebuildHelper(
	hash []byte,
	hash2Node map[string]myTrie.Node,
	keyPrefix []byte,
	updadedStates map[string]*types.StateAccount,
) []byte {
	// log.Debug(fmt.Sprintf("current hash: %x", hash))
	node, ok := hash2Node[string(hash[:])]
	if !ok {
		// log.Debug(fmt.Sprintf("node not recorded for hash: %x", hash))
		return hash
	}
	switch node.(type) {
	case *myTrie.FullNode:
		fullNode := node.(*myTrie.FullNode)
		for i, child := range fullNode.Children {
			if child != nil {
				childHash, ok := child.(myTrie.HashNode)
				if !ok {
					log.Error(fmt.Sprintf("fullnode's not nil child is not of HashNode type?! why? hash: %x", childHash))
				}
				temp := rebuildHelper(childHash, hash2Node, getCurrentKey(keyPrefix, []byte{byte(i)}), updadedStates)
				fullNode.Children[i] = myTrie.HashNode(temp)
			}
		}
		newHash := utils.GetHash(myTrie.NodeToBytes(fullNode))
		// log.Debug(fmt.Sprintf("node original hash: %x  new hash: %x", hash, newHash))
		return newHash
	case *myTrie.ShortNode:
		shortNode := node.(*myTrie.ShortNode)
		curKey := getCurrentKey(keyPrefix, shortNode.Key)
		switch shortNode.Val.(type) {
		case myTrie.HashNode:
			temp := rebuildHelper(shortNode.Val.(myTrie.HashNode), hash2Node, curKey, updadedStates)
			shortNode.Val = myTrie.HashNode(temp)
			shortNode.Key = myTrie.HexToCompact(shortNode.Key)
			newHash := utils.GetHash(myTrie.NodeToBytes(shortNode))
			// log.Debug(fmt.Sprintf("node original hash: %x  new hash: %x", hash, newHash))
			return newHash
		case myTrie.ValueNode:
			recoverAddressHash := myTrie.HexToKeybytes(curKey)
			stateAccount, ok := updadedStates[string(recoverAddressHash)]
			if ok { // 该地址的状态被更新过
				encodedBytes, err := rlp.EncodeToBytes(stateAccount)
				// log.Debug(fmt.Sprintf("stateAccount data: %v encodedBytes: %v", stateAccount, encodedBytes))
				// log.Debug(fmt.Sprintf("Balance: %v, %v, %v, %v", stateAccount.Balance, stateAccount.Nonce, stateAccount.Root, stateAccount.CodeHash))
				if err != nil {
					log.Error("rlp encode err", "err", err)
				}
				shortNode.Val = myTrie.ValueNode(encodedBytes)
			}
			shortNode.Key = myTrie.HexToCompact(shortNode.Key)
			newHash := utils.GetHash(myTrie.NodeToBytes(shortNode))
			// log.Debug(fmt.Sprintf("node original hash: %x  new hash: %x", hash, newHash))
			return newHash
		default:
			log.Error("shortnode's val unknown type")
		}
	default:
		log.Error("unknown node type")
	}
	return nil
}

/*
* 执行打包的交易，更新stateObjects
 */
func (w *Worker) executeTransactions(
	txs []*core.Transaction,
	addr2State map[common.Address]*types.StateAccount,
	updatedStates map[string]*types.StateAccount,
) {
	now := time.Now().Unix()
	for _, tx := range txs {
		w.executeTransaction(tx, now, addr2State, updatedStates)
	}
}

func addBalance(state *types.StateAccount, val *big.Int) {
	state.Balance = new(big.Int).Add(state.Balance, val)
}

func subBalance(state *types.StateAccount, val *big.Int) {
	state.Balance = new(big.Int).Sub(state.Balance, val)
}

func addNonceByOne(state *types.StateAccount) {
	state.Nonce = state.Nonce + 1
}

func subNonceByOne(state *types.StateAccount) {
	state.Nonce = state.Nonce - 1
}

func (w *Worker) executeTransaction(
	tx *core.Transaction,
	now int64,
	addr2State map[common.Address]*types.StateAccount,
	updatedStates map[string]*types.StateAccount,
) {
	tx.TXStatus = result.DefaultStatus
	if tx.TXtype == core.IntraTXType {
		senderState := addr2State[*tx.Sender]
		addNonceByOne(senderState)
		subBalance(addr2State[*tx.Sender], tx.Value)
		updatedStates[string(utils.GetHash((*tx.Sender)[:]))] = senderState
		receiverState := addr2State[*tx.Recipient]
		addBalance(receiverState, tx.Value)
		updatedStates[string(utils.GetHash((*tx.Recipient)[:]))] = receiverState
		tx.TXStatus = result.IntraSuccess
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee commit intra tx", "time", now)
	} else if tx.TXtype == core.CrossTXType1 {
		senderState := addr2State[*tx.Sender]
		addNonceByOne(senderState)
		subBalance(addr2State[*tx.Sender], tx.Value)
		updatedStates[string(utils.GetHash((*tx.Sender)[:]))] = senderState
		tx.TXStatus = result.CrossTXType1Success
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee commit cross1 tx", "time", now, "tbchain_height", w.com.tbchain_height)
	} else if tx.TXtype == core.CrossTXType2 {
		receiverState := addr2State[*tx.Recipient]
		addBalance(receiverState, tx.Value)
		updatedStates[string(utils.GetHash((*tx.Recipient)[:]))] = receiverState
		tx.TXStatus = result.CrossTXType2Success
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee commit cross2 tx", "time", now,
			"tbchain_height", w.com.tbchain_height, "cross1ConfirmHeight", tx.ConfirmHeight, "txRollbackHeight", tx.RollbackHeight)
	} else if tx.TXtype == core.RollbackTXType {
		senderState := addr2State[*tx.Sender]
		subNonceByOne(senderState)
		addBalance(senderState, tx.Value)
		updatedStates[string(utils.GetHash((*tx.Sender)[:]))] = senderState
		tx.TXStatus = result.RollbackSuccess
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee commit rollback tx", "time", now)
	} else {
		log.Error("Oops, something wrong! Cannot handle tx type", "cur comID", w.com.Node.NodeInfo.ComID, "type", tx.TXtype, "tx", tx)
	}
	// tx.ConfirmTimestamp = uint64(now)
}

// func (w *Worker) Reconfig() {
// 	log.Info("start reconfiguration...", "before that this committee belongs to shard", w.com.Node.NodeInfo.ComID)

// }
