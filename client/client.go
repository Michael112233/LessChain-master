package client

import (
	"container/list"
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

type Client struct {
	host string
	port string

	exitMode int

	cid            int
	rollbackHeight int

	messageHub core.MessageHub

	stopCh chan struct{}

	shard_num int

	/* 待注入到分片的交易*/
	txs []*core.Transaction

	/* txs 中交易ID到交易的映射 */
	txs_map map[uint64]*core.Transaction

	/* 已注入到分片的交易数量 */
	injectCnt         int
	InjectDoneMsgSent bool

	/* 委员会发送给客户端的待处理的交易收据 */
	tx_reply *list.List
	/* tx_reply 的锁 */
	r_lock sync.Mutex

	/**
	 * 跨片交易前半部分在信标链上确认时接收分片的确认高度
	 * key：交易ID	value：确认高度
	 * 使用记得加锁
	 */
	cross1_confirm_height_map map[uint64]uint64
	/* cross1_confirm_height_map 的锁 */
	c1_c_lock sync.RWMutex

	/* 待发送的跨片交易后半部分交易，由 cross1_tx_reply 中的交易在收到信标确认后转化过来。使用记得加锁*/
	cross2_txs []*core.Transaction
	/* cross2_txs的锁 */
	c2_lock sync.Mutex

	/* 跨片交易超时队列，客户端将向源分片发送回滚交易。使用记得加锁 */
	cross_tx_expired []uint64
	e_lock           sync.Mutex

	/* 本地存储的信标 */
	tbs map[uint32]map[uint64]*beaconChain.ConfirmedTB
	/* 各个分片已在信标链确认的最新高度 */
	shard_cur_heights map[uint32]uint64

	tbchain_height uint64

	contractAddr common.Address
	contractAbi  *abi.ABI

	injectSpeed int

	wg sync.WaitGroup
}

func NewClient(addr string, id, rollbackHeight, shardNum int, exitMode int) *Client {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Error("invalid node address!", "addr", addr)
	}
	c := &Client{
		host:                      host,
		port:                      port,
		exitMode:                  exitMode,
		cid:                       id,
		rollbackHeight:            rollbackHeight,
		stopCh:                    make(chan struct{}),
		txs:                       make([]*core.Transaction, 0),
		txs_map:                   make(map[uint64]*core.Transaction),
		tx_reply:                  list.New(),
		shard_num:                 shardNum,
		cross1_confirm_height_map: make(map[uint64]uint64),
		tbs:                       make(map[uint32]map[uint64]*beaconChain.ConfirmedTB),
		shard_cur_heights:         make(map[uint32]uint64),
	}

	for shardID := 0; shardID < shardNum; shardID++ {
		c.tbs[uint32(shardID)] = make(map[uint64]*beaconChain.ConfirmedTB)
		c.shard_cur_heights[uint32(shardID)] = 0
	}
	return c
}

func (c *Client) Start(injectSpeed int) {
	c.injectSpeed = injectSpeed
}

func (c *Client) StartInjectTxs() {
	c.wg.Add(1)
	go c.SendTXs(c.injectSpeed)
}

func (c *Client) SetMessageHub(hub core.MessageHub) {
	c.messageHub = hub
}

func (c *Client) HandleComSendTxReceipt(receipts []*result.TXReceipt) {
	c.AddTXReceipts(receipts)
}

/* 检查是否有超时的跨片交易 */
func (c *Client) checkExpiredTXs() {
	now := time.Now().Unix()
	c.c1_c_lock.Lock()
	expired_txs := make([]uint64, 0, len(c.cross1_confirm_height_map)/2)
	for txid, confirmHeight := range c.cross1_confirm_height_map {
		tx := c.txs_map[txid]
		if c.shard_cur_heights[uint32(tx.Recipient_sid)] > confirmHeight+tx.RollbackHeight {
			// 这里的if语句中是 > 而非 >= 有重要的含义
			// 虽然在checkExpiredTXs之前已经processTXReceipts了，但不排除一种可能
			// 即client此时还未收到cross2的reply
			// 用 > 号相当于给更多的缓冲时间，防止一笔交易同时出现rollback和cross2两种状态
			expired_txs = append(expired_txs, txid)
			log.Trace("tracing transaction", "txid", txid, "status", "client add tx to expired_tx", "time", now)
			delete(c.cross1_confirm_height_map, txid)
		}
	}
	c.c1_c_lock.Unlock()
	c.e_lock.Lock()
	defer c.e_lock.Unlock()
	c.cross_tx_expired = append(c.cross_tx_expired, expired_txs...)
	// log.Debug("checkExpiredTXs", "cid", c.cid, "cross1_confirm_height_map_len", len(c.cross1_confirm_height_map), "expired_tx_len", len(expired_txs))

}

func (c *Client) Close() {
	close(c.stopCh)
	c.wg.Wait()
	log.Info("client stop.", "clientID", c.cid)
}

func (c *Client) Print() {
	msg := fmt.Sprintf("client %d tx number: %d", c.cid, len(c.txs))
	log.Debug(msg)
}

func (c *Client) getTBFromTBChain(shardID uint32, height uint64) *beaconChain.ConfirmedTB {
	var tb *beaconChain.ConfirmedTB
	callback := func(res ...interface{}) {
		tb = res[0].(*beaconChain.ConfirmedTB)
	}
	c.messageHub.Send(core.MsgTypeGetTB, shardID, height, callback)
	return tb
}

func (c *Client) GetAddr() string {
	return fmt.Sprintf("%s:%s", c.host, c.port)
}

/* 验证一笔交易的merkle proof，tb 是该交易对应分片和高度的区块信标
* 目前未实现具体逻辑，直接假设验证通过
 */
func VerifyTxMKproof(proof []byte, tb *core.TimeBeacon) bool {
	return true
}

/* 所有交易执行完成则结束 */
func (c *Client) CanStopV1() bool {
	return len(c.txs) == c.injectCnt && c.tx_reply.Len() == 0 && len(c.cross_tx_expired) == 0 &&
		len(c.cross2_txs) == 0 && len(c.cross1_confirm_height_map) == 0
}

func (c *Client) LogQueues() {
	log.Debug("client queue length", "tx_reply", c.tx_reply.Len(), "cross1_confirm_height_map", len(c.cross1_confirm_height_map),
		"cross2_txs", len(c.cross2_txs), "cross_tx_expired", len(c.cross_tx_expired),
		"c.injectCnt", c.injectCnt, "len(c.txs)", len(c.txs))
}

/* 客户端初始交易注入完成则结束 */
func (c *Client) CanStopV2() bool {
	return c.injectCnt == len(c.txs)
}

func (c *Client) GetCid() int {
	return c.cid
}
