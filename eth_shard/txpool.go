package eth_shard

/*
   本地交易: 在本地磁盘存储已发送的交易。这样，本地交易不会丢失，重启节点时可以重新加载到交易池，实时广播出去。
*/

import (
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type TxPool struct {
	// chain blockChain
	pending []*core.Transaction
	lock    sync.Mutex

	pendingRollback []*core.Transaction
	r_lock          sync.Mutex
	com             *Shard
}

// 该函数仅在重组后同步交易池时被使用
func (pool *TxPool) SetPending(txs []*core.Transaction) {
	pool.pending = append(txs, pool.pending...)
}

// 该函数仅在重组后同步交易池时被使用
func (pool *TxPool) SetPendingRollback(txs []*core.Transaction) {
	pool.pendingRollback = append(txs, pool.pendingRollback...)
}

func NewTxPool(shardID uint32) *TxPool {
	pool := &TxPool{}
	return pool
}

func (pool *TxPool) setCommittee(com *Shard) {
	pool.com = com
}

/* 重置交易池，返回新的交易池 */
func (pool *TxPool) Reset() *TxPool {
	// 标记交易池中剩下的交易为 dropped
	txs := append(pool.pending, pool.pendingRollback...)
	table := make(map[uint64]*result.TXReceipt)
	for _, tx := range txs {
		tx.TXStatus = result.Dropped
		table[tx.ID] = &result.TXReceipt{
			TxID:     tx.ID,
			TxStatus: tx.TXStatus,
			ShardID:  int(pool.com.Node.NodeInfo.ComID),
		}
	}
	result.SetTXReceiptV2(table)

	// 生成新的交易池
	newpool := NewTxPool(pool.com.Node.NodeInfo.ComID)
	newpool.setCommittee(pool.com)
	return newpool
}

/* 向交易池中添加交易，调用此方法的方法必须加锁 */
func (pool *TxPool) AddTxWithoutLock(tx *core.Transaction, now int64) {
	// TODO: 检查本地是否已存有该交易
	if tx.TXtype == core.RollbackTXType {
		pool.pendingRollback = append(pool.pendingRollback, tx)
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee add rollback tx to pool", "time", now)
	} else {
		pool.pending = append(pool.pending, tx)
		if tx.TXtype == core.CrossTXType2 {
			log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee add cross2 tx to pool", "time", now)
		} else {
			log.Trace("tracing transaction, ", "txid", tx.ID, "status", "committee add tx to pool", "time", now)
		}
	}
}

func (pool *TxPool) AddTxs(txs []*core.Transaction) {
	// log.Debug("11111")
	/* 需要对lock和r_lock都加锁的场景，都按照先lock再r_lock的顺序，避免死锁 */
	pool.lock.Lock()
	// log.Debug("22222")
	defer pool.lock.Unlock()
	pool.r_lock.Lock()
	// log.Debug("33333")
	defer pool.r_lock.Unlock()
	now := time.Now().Unix()
	// fmt.Printf("收到交易%v\n", txs)
	for _, tx := range txs {
		pool.AddTxWithoutLock(tx, now)
	}
	// log.Debug("44444")

	log.Debug("TxPoolAddTXs", "comID", pool.com.Node.NodeInfo.ComID, "txPoolPendingLen", pool.PendingLen(), "txPoolPendingRollbackLen", pool.PendingRollbackLen())
}

/* worker.commitTransaction 从队列取出交易 */
func (pool *TxPool) Pending(maxBlockSize int, parentBlockHeight *big.Int) ([]*core.Transaction, []common.Address) {
	/* 需要对lock和r_lock都加锁的场景，都按照先lock再r_lock的顺序，避免死锁 */
	pool.lock.Lock()
	defer pool.lock.Unlock()
	pool.r_lock.Lock()
	defer pool.r_lock.Unlock()
	/* 取交易 */
	txs := make([]*core.Transaction, 0)
	i := 0

	now := time.Now().Unix()

	// 从pendingRollback队列取交易，rollback交易优先于其他交易
	for {
		if i == maxBlockSize || i >= len(pool.pendingRollback) {
			break
		}
		tx := pool.pendingRollback[i]
		txs = append(txs, tx)
		i++
	}
	pool.pendingRollback = pool.pendingRollback[i:]

	// 从pending队列取交易
	maxBlockSize -= i
	i = 0
	pendinglen := len(pool.pending)
	for {
		if i == maxBlockSize || i >= pendinglen {
			break
		}
		tx := pool.pending[i]
		// 为保证交易原子性，cross2 交易应判断是否超时
		if tx.TXtype == core.CrossTXType2 {
			// log.Debug(fmt.Sprintf("txpool selecting cross2 tx... txid: %d shard cur height: %d cross1ConfirmHeight: %d rollbackHeight: %d",
			// 	tx.ID, parentBlockHeight.Uint64(), tx.Cross1ConfirmHeight, tx.RollbackHeight))
			// 如果新区块高度超过回滚高度，则丢弃交易
			if parentBlockHeight.Uint64()+1 > tx.Cross1ConfirmHeight+tx.RollbackHeight {
				// log.Debug(fmt.Sprintf("tx has expired. txid: %d shard cur height: %d cross1ConfirmHeight: %d rollbackHeight: %d",
				// 	tx.ID, parentBlockHeight.Uint64(), tx.Cross1ConfirmHeight, tx.RollbackHeight))
				// 如果cross2交易已超时，不会选择该交易进行打包，队列指针后移时需要将maxBlockSize也+1
				i++
				maxBlockSize++
				log.Trace("tracing transaction", "txid", tx.ID, "status", result.GetStatusString(result.CrossTXType2Fail), "time", now)
				continue
			}
		}
		txs = append(txs, tx)
		i++
	}
	/* update pool tx num */
	pool.pending = pool.pending[i:]

	// 获取与交易相关的账户状态
	addrs := getTxRelatedAddrs(txs)

	return txs, addrs
}

func getTxRelatedAddrs(txs []*core.Transaction) []common.Address {
	addrs := make(map[common.Address]struct{})
	for _, tx := range txs {
		switch tx.TXtype {
		case core.CrossTXType1, core.RollbackTXType:
			addrs[*tx.Sender] = struct{}{}
		case core.CrossTXType2:
			addrs[*tx.Recipient] = struct{}{}
		case core.IntraTXType:
			addrs[*tx.Sender] = struct{}{}
			addrs[*tx.Recipient] = struct{}{}
		default:
			log.Error("Unknown Txtype", "type", tx.TXtype)
		}
	}
	addrList := make([]common.Address, 0)
	for addr, _ := range addrs {
		addrList = append(addrList, addr)
	}

	return addrList
}

func (pool *TxPool) Empty() bool {
	return pool.PendingLen() == 0 && pool.PendingRollbackLen() == 0
}

func (pool *TxPool) PendingLen() int {
	return len(pool.pending)
}

func (pool *TxPool) PendingRollbackLen() int {
	return len(pool.pendingRollback)
}
