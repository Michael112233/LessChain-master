package data

/**
 * 交易分配生成：map[txid] sharid
 */

import (
	"encoding/csv"
	"go-w3chain/client"
	"go-w3chain/core"
	"go-w3chain/eth_shard"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"io"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
)

var (
	alltxs []*core.Transaction

	tx2ClientTable map[uint64]int // txid 映射到客户端ID

)

/**
 * 加载数据集, maxNum=-1则全加载
 */
func LoadETHData(filepath string, maxTxNum int) []*core.Transaction {
	f, err := os.Open(filepath)
	if err != nil {
		log.Error("Open dataset error!", ":", err)
	}
	reader := csv.NewReader(f)
	colname, err := reader.Read()
	// log.Debug("show label ", "colname:", colname)
	if err != nil {
		log.Error("Get reader handle error!", "err:", err, "colname:", colname)
	}
	txid := uint64(0)

	addrs := make(map[common.Address]struct{})

	for {
		row, err := reader.Read()
		if err != nil && err != io.EOF {
			log.Error("Read transaction error!", ":", err)
		}
		if err == io.EOF {
			break
		}
		// 该字段为 input，若不为空则表示调用或创建了智能合约，这类交易应该排除掉
		// if row[6] != "0x" {
		// 	continue
		// }
		sender := common.HexToAddress(row[0][2:])
		recipient := common.HexToAddress(row[1][2:])
		addrs[sender] = struct{}{}
		addrs[recipient] = struct{}{}
		value := new(big.Int)
		// value.SetString(row[6], 10)
		value.SetString("1", 10)
		tx := core.Transaction{
			TXtype:    core.UndefinedTXType,
			Sender:    &sender,
			Recipient: &recipient,
			Value:     value,
			ID:        txid,
		}
		alltxs = append(alltxs, &tx)
		txid += 1
		if (maxTxNum > 0) && (txid >= uint64(maxTxNum)) {
			break
		}
	}
	log.Debug("Load data completed", "total number", len(alltxs), "addr number", len(addrs), "first tx", alltxs[0])
	result.SetTotalTXNum(len(alltxs))

	return alltxs
}

/**
 * 根据交易发送地址和接收地址设置交易的发送分片和接收分片
 */
func SetTxShardId(shardNum int) {
	intraTxNum := 0
	for _, tx := range alltxs {
		sid := utils.Addr2Shard(tx.Sender.Hex(), shardNum) // id = 0,1,..
		tx.Sender_sid = uint32(sid)
		to_sid := utils.Addr2Shard(tx.Recipient.Hex(), shardNum) // id = 0,1,..
		tx.Recipient_sid = uint32(to_sid)
		if sid == to_sid {
			intraTxNum += 1
		}
	}
	log.Debug("TxNum of different type", "intra tx number", intraTxNum, "cross tx number", len(alltxs)-intraTxNum,
		"cross tx rate", float64(len(alltxs)-intraTxNum)/float64(len(alltxs)))
}

/**
 * 提前设置分片中的账户状态（金额）
 */
func SetShardInitialAccountState(shard *eth_shard.Shard) {
	addrs := make(map[common.Address]struct{}, 0)
	shardID := shard.GetShardID()

	for _, tx := range alltxs {
		if tx.Sender_sid == shardID {
			addrs[*tx.Sender] = struct{}{}
		}
		if tx.Recipient_sid == shardID {
			addrs[*tx.Recipient] = struct{}{}
		}
	}

	/* 将所有sender在对应分片中的初始金额设为一个极大值，确保之后注入的交易顺利执行 */
	maxValue := new(big.Int)
	maxValue.SetString("1000000000000000", 10)

	shard.SetInitialAccountState(addrs, maxValue)

	log.Info("SetShardsInitialState successed", "nodeID", shardID, "# of initial addr", len(addrs))
}

/**
* 实现交易到客户端的划分
 */
func SetTX2ClientTable(clientNum int) {
	tx2ClientTable = make(map[uint64]int)
	for _, tx := range alltxs {
		tx2ClientTable[tx.ID] = int(tx.ID) % clientNum
	}
}

/**
* 注入交易到指定客户端，一次性全部注入
 */
func InjectTX2Client(client *client.Client) {
	txlist := make([]*core.Transaction, 0)
	for _, tx := range alltxs {
		cid := tx2ClientTable[tx.ID]
		if cid == client.GetCid() {
			txlist = append(txlist, tx)
		}
	}
	client.Addtxs(txlist)
}

func PrintTXs(num int) {
	if num == -1 {
		num = len(alltxs)
	}
	for i := 0; i < num; i++ {
		log.Debug("shows TXs", "tx", alltxs[i])
		// log.Trace("shows TXs", "id", alltxs[i].ID, "broadcast time", alltxs[i].Timestamp, "Confirm time", alltxs[i].ConfirmTimestamp)
	}
}

// for debug
func GetAlltxs() []*core.Transaction {
	return alltxs
}

func ClearAll() {
	alltxs = nil
	tx2ClientTable = nil
}
