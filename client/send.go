package client

import (
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"time"
)

/**
 * 按一定速率发送交易到分片
 * 目前的实现未通过网络传输
 */
func (c *Client) SendTXs(inject_speed int) {
	defer c.wg.Done()
	c.InjectTXs(c.cid, inject_speed)
}

/**
 * 按一定速率将客户端的交易注入到分片
 */
func (c *Client) InjectTXs(cid int, inject_speed int) {
	c.injectCnt = 0
	resBroadcastMap := make(map[uint64]uint64)

	c.InjectDoneMsgSent = false
	// 按秒注入
	for {
		time.Sleep(1000 * time.Millisecond) //fixme 应该记录下面的运行时间
		// start := time.Now().UnixMilli()
		rollbackTxSentCnt := c.sendRollbackTxs(inject_speed)

		cross2TxSentCnt := c.sendCross2Txs(inject_speed - rollbackTxSentCnt)

		c.injectCnt = c.sendPendingTxs(c.injectCnt, inject_speed-rollbackTxSentCnt-cross2TxSentCnt, resBroadcastMap)
		if c.CanStopV2() && !c.InjectDoneMsgSent {
			/* 通知各节点交易注入完成 */
			c.messageHub.Send(core.MsgTypeSetInjectDone2Nodes, uint32(c.cid), struct{}{}, nil)
			c.InjectDoneMsgSent = true
			if c.exitMode == 1 {
				break
			}
		}
		if c.CanStopV1() {
			break
		}

	}
	/* 记录广播结果 */
	result.SetBroadcastMap(resBroadcastMap)

}

func (c *Client) sendRollbackTxs(maxTxNum2Pack int) int {
	c.e_lock.Lock()
	defer c.e_lock.Unlock()
	rollbackTxSentCnt := utils.Min(maxTxNum2Pack, len(c.cross_tx_expired))
	if rollbackTxSentCnt == 0 {
		return 0
	}

	now := time.Now().Unix()

	/* 初始化 shardtxs */
	shardtxs := make([][]*core.Transaction, c.shard_num)
	for i := 0; i < c.shard_size; i++ {
		shardtxs[i] = make([]*core.Transaction, 0, rollbackTxSentCnt*2/c.shard_size+1)
	}
	for i := 0; i < rollbackTxSentCnt; i++ {
		txid := c.cross_tx_expired[i]
		tx := *c.txs_map[txid]
		tx.TXtype = core.RollbackTXType
		tx.TXStatus = result.DefaultStatus
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "client send rollback tx to committee", "time", now)
		shardtxs[tx.Sender_sid] = append(shardtxs[tx.Sender_sid], &tx)
	}
	/* 注入到各节点 */
	for i := 0; i < c.shard_size; i++ {
		c.messageHub.Send(core.MsgTypeClientInjectTX2Node, uint32(i), shardtxs[i], nil)
	}
	// 移除已发送的reply交易
	c.cross_tx_expired = c.cross_tx_expired[rollbackTxSentCnt:]
	return rollbackTxSentCnt
}

/* 从cross2队列中取交易发送，优先于pending队列, 交易被发送后从队列中移除*/
func (c *Client) sendCross2Txs(maxTxNum2Pack int) int {
	c.c2_lock.Lock()
	defer c.c2_lock.Unlock()
	cross2TxSentCnt := utils.Min(maxTxNum2Pack, len(c.cross2_txs))
	if cross2TxSentCnt == 0 {
		return 0
	}

	now := time.Now().Unix()

	/* 初始化 shardtxs */
	shardtxs := make([][]*core.Transaction, c.shard_num)
	for i := 0; i < c.shard_size; i++ {
		shardtxs[i] = make([]*core.Transaction, 0, cross2TxSentCnt*2/c.shard_size+1)
	}
	for i := 0; i < cross2TxSentCnt; i++ {
		tx := c.cross2_txs[i]
		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "client send cross2 to committee", "time", now)
		shardtxs[tx.Recipient_sid] = append(shardtxs[tx.Recipient_sid], tx)
	}
	/* 注入到各节点 */
	for i := 0; i < c.shard_size; i++ {
		c.messageHub.Send(core.MsgTypeClientInjectTX2Node, uint32(i), shardtxs[i], nil)
	}
	// 移除已发送的reply交易
	c.cross2_txs = c.cross2_txs[cross2TxSentCnt:]
	return cross2TxSentCnt
}

/* 从pending队列中取交易发送 */
func (c *Client) sendPendingTxs(cnt, maxTxNum2Pack int, resBroadcastMap map[uint64]uint64) int {
	if maxTxNum2Pack == 0 {
		return cnt
	}

	upperBound := utils.Min(cnt+maxTxNum2Pack, len(c.txs))
	/* 初始化 shardtxs */
	shardtxs := make([][]*core.Transaction, c.shard_size)
	for i := 0; i < c.shard_num; i++ {
		shardtxs[i] = make([]*core.Transaction, 0, maxTxNum2Pack*2/c.shard_size+1)
	}

	now := time.Now().Unix()

	for i := cnt; i < upperBound; i++ {
		tx := c.txs[i]
		// 根据发送地址和接收地址确认交易类型
		if tx.Sender_sid == tx.Recipient_sid {
			tx.TXtype = core.IntraTXType
		} else {
			tx.TXtype = core.CrossTXType1
		}

		tx.Timestamp = uint64(now)
		tx.Cid = uint64(c.cid)

		log.Trace("tracing transaction, ", "txid", tx.ID, "status", "client broadcast to committee", "time", now)

		resBroadcastMap[tx.ID] = tx.Timestamp
		shardtxs[tx.Sender_sid] = append(shardtxs[tx.Sender_sid], tx)
	}
	/* 注入到各节点 */
	for i := 0; i < c.shard_size; i++ {
		c.messageHub.Send(core.MsgTypeClientInjectTX2Node, uint32(i), shardtxs[i], nil)
	}
	/* 更新循环变量 */
	cnt = upperBound
	return cnt
}

/* 生成交易收据, 记录到result */
func (c *Client) recordTXReceipts(receipts map[uint64]*result.TXReceipt) {
	result.SetTXReceiptV2(receipts)
}
