package client

import (
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/core"
	"go-w3chain/eth_chain"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

/**
 * 该函数在loadData后被调用一次，将分配到此客户端的交易加入队列中，并多设一个map用于快速根据交易ID找到交易
 */
func (c *Client) Addtxs(txs []*core.Transaction) {
	for _, tx := range txs {
		tx.RollbackHeight = uint64(c.rollbackHeight)
	}
	c.txs = append(c.txs, txs...)
	// log.Debug("clientAddtxs", "clientID", c.cid, "txs_len", len(c.txs))
	for _, tx := range c.txs {
		c.txs_map[tx.ID] = tx
	}
}

/** 客户端收到委员会发送的交易收据，存入本地
* 客户端不会立即对交易进行处理，比如发送跨片交易后半部分，
需要等到收到信标链的信标确认消息才会对交易进行处理
*/
func (c *Client) AddTXReceipts(receipts []*result.TXReceipt) {
	c.r_lock.Lock()
	defer c.r_lock.Unlock()
	for _, r := range receipts {
		c.tx_reply.PushBack(r)
	}

}

/** 获取信标
 * 先从本地缓存拿，如果本地没有再从信标链拿
 */
func (c *Client) GetTB(shardID uint32, height uint64) *beaconChain.ConfirmedTB {
	if tb, ok := c.tbs[shardID][height]; !ok {
		tb1 := c.getTBFromTBChain(shardID, height)
		c.tbs[shardID][height] = tb1
		return tb1
	} else {
		return tb
	}
}

/** 信标链主动向客户端推送新确认的信标时调用此函数
 * 一般情况下信标链应该只向客户端推送其关注的分片和高度的信标，这里进行了简化，默认全部推送
 * 客户端收到新确认信标后，遍历 tx_reply，如果某个交易的区块信标已确认，则对该交易进行后续处理
 * 客户端收到新确认信标后，检查是否有超时的跨片交易
 */
func (c *Client) AddTBs(tbblock *beaconChain.TBBlock) {
	log.Debug(fmt.Sprintf("client get tbchain confirm block... %v", tbblock))
	for shardID, tbs := range tbblock.Tbs {
		for _, tb := range tbs {
			// log.Debug("addTB to c.tbs", "shardID", shardID, "blockHeight", tb.Height)
			c.tbs[uint32(shardID)][tb.Height] = tb
			c.shard_cur_heights[uint32(shardID)] = uint64(utils.Max(int(c.shard_cur_heights[uint32(shardID)]), int(tb.Height)))
		}
	}
	c.tbchain_height = tbblock.Height
	c.processTXReceipts()
	c.checkExpiredTXs()

}

/**
* 遍历 tx_reply，如果某个交易的区块信标已确认，则对该交易进行后续处理：
如果交易已完成，则记录确认时间；如果交易未完成，则加入到新队列中等待被发送
*/
func (c *Client) processTXReceipts() {
	c.r_lock.Lock()
	defer c.r_lock.Unlock()
	c.c2_lock.Lock()
	defer c.c2_lock.Unlock()
	c.c1_c_lock.Lock()
	defer c.c1_c_lock.Unlock()

	to_record := make(map[uint64]*result.TXReceipt)
	l := c.tx_reply
	for e := l.Front(); e != nil; {
		n := e.Next()

		r := e.Value.(*result.TXReceipt)
		// 信标未确认，跳过
		if _, ok := c.tbs[uint32(r.ShardID)][r.BlockHeight]; !ok {
			// log.Debug("unconfirmed receipts", "shardID", r.ShardID, "blockHeight", r.BlockHeight)
			e = n
			continue
		}
		// 信标已确认，分情况处理
		tb := c.tbs[uint32(r.ShardID)][r.BlockHeight]
		r.ConfirmTimeStamp = tb.ConfirmTime
		to_record[r.TxID] = r
		if r.TxStatus == result.IntraSuccess {
			// do nothing
		} else if r.TxStatus == result.CrossTXType1Success {
			// 1. 加入cross2队列
			txid := r.TxID
			tx := *c.txs_map[txid]
			tx.TXtype = core.CrossTXType2
			tx.TXStatus = result.DefaultStatus
			tx.ConfirmHeight = c.tbchain_height
			tx.Cross1ConfirmHeight = c.shard_cur_heights[uint32(tx.Recipient_sid)]
			c.cross2_txs = append(c.cross2_txs, &tx)
			log.Trace("tracing transaction", "txid", r.TxID, "status", result.GetStatusString(r.TxStatus), "time", r.ConfirmTimeStamp)
			log.Trace("tracing transaction", "txid", r.TxID, "status", "client add tx to cross2_txs (list)", "time", r.ConfirmTimeStamp)
			// 2. 记录到 cross1_confirm_height_map 中
			c.cross1_confirm_height_map[r.TxID] = c.shard_cur_heights[uint32(tx.Recipient_sid)]
		} else if r.TxStatus == result.CrossTXType2Success {
			// 只要是确认了，就一定不是超时的
			// 删除cross1_confirm_height_map中的项
			if _, ok := c.cross1_confirm_height_map[r.TxID]; !ok {
				log.Error(fmt.Sprintf("err: tx %d should be in c.cross1_confirm_height_map, but actually not in.", r.TxID))
			} else {
				delete(c.cross1_confirm_height_map, r.TxID)
			}
			log.Trace("tracing transaction", "txid", r.TxID, "status", result.GetStatusString(r.TxStatus), "time", r.ConfirmTimeStamp)
		} else if r.TxStatus == result.RollbackSuccess {
			// res_status := result.GetResult().AllTXStatus[r.TxID]
			// if len(res_status) > 0 && res_status[len(res_status)-1] == result.CrossTXType2Success {
			// 	// 如果交易后半部分已经执行成功，回滚交易不能被执行
			// 	log.Error("this tx should not be rolled back!", "txid", r.TxID)
			// }
			log.Trace("tracing transaction", "txid", r.TxID, "status", result.GetStatusString(r.TxStatus), "time", r.ConfirmTimeStamp)
		}
		l.Remove(e)
		e = n
	}
	c.recordTXReceipts(to_record)

}

func (c *Client) HandleBooterSendContract(data *core.BooterSendContract) {
	c.contractAddr = data.Addr
	contractABI, err := abi.JSON(strings.NewReader(eth_chain.MyContractABI()))
	if err != nil {
		log.Error("get contracy abi fail", "err", err)
	}
	c.contractAbi = &contractABI
	// 启动 发动交易的线程
	c.StartInjectTxs()
}
