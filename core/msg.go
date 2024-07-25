package core

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Msg struct {
	MsgType string
	Data    []byte
}

type ComGetHeight struct {
	From_comID     uint32
	Target_shardID uint32
}

type ComGetState struct {
	From_comID     uint32
	Target_shardID uint32
	AddrList       []common.Address
}

type ShardSendState struct {
	StatusTrieHash common.Hash
	AccountData    map[common.Address][]byte
	AccountsProofs map[common.Address][][]byte
	Height         *big.Int
}

type NodeSendInfo struct {
	NodeInfo *NodeInfo
	Addr     common.Address
}

type ShardSendGenesis struct {
	Addrs           []common.Address
	Gtb             *TimeBeacon
	ShardID         uint32
	Target_nodeAddr string
}

type BooterSendContract struct {
	Addr common.Address
}

type ComSendBlock struct {
	Block *Block
}

type ClientSetInjectDone struct {
	Cid uint32
}

type ComGetTx struct {
	From_nodeID uint32
	Txs         []*Transaction
}

type ComLeaderInitMultiSign struct {
	Seed       common.Hash
	SeedHeight uint64
	Tb         *TimeBeacon
}

type MultiSignReply struct {
	Request    *ComLeaderInitMultiSign
	VrfValue   []byte
	Sig        []byte
	PubAddress common.Address
	NodeInfo   *NodeInfo
}

type InitReconfig struct {
	Seed       common.Hash
	SeedHeight uint64
	ComID      uint32
	ComNodeNum uint32
}

type ReconfigResult struct {
	Seed         common.Hash
	SeedHeight   uint64
	Vrf          []byte
	Addr         common.Address
	OldNodeInfo  *NodeInfo
	Belong_ComID uint32
	NewComID     uint32
}

type ComReconfigResults struct {
	ComID      uint32
	Results    []*ReconfigResult
	ComNodeNum uint32
}

type AdjustAddrs struct {
	ComID      uint32
	Addrs      []common.Address
	Vrfs       [][]byte
	SeedHeight uint64
}

type GetPoolTx struct {
	ServerAddr   string // 向该地址请求交易
	ClientAddr   string // 交易发送回该地址
	RequestComID uint32 // 请求该委员会的交易
}

type GetSyncData struct {
	ServerAddr string
	ClientAddr string
	ShardID    uint32
	SyncType   string
}

type SyncData struct {
	ClientAddr string
	States     map[common.Address]*types.StateAccount
	Blocks     []*Block
}

type PoolTx struct {
	Pending         []*Transaction
	PendingRollback []*Transaction
}

// ////////////////////////////
// //// pbft module ///////
// ////////////////////////////
type NodeInfo struct {
	ShardID  uint32
	ComID    uint32
	NodeID   uint32
	NodeAddr string
}

func (n *NodeInfo) PrintNode() {
	v := []interface{}{
		n.NodeID,
		n.ShardID,
		n.NodeAddr,
	}
	fmt.Printf("%v\n", v)
}

type PbftRequest struct {
	MsgType string
	Msg     []byte // request message
	ReqTime int64  // request time
}

type PrePrepare struct {
	RequestMsg *PbftRequest // the request message should be pre-prepared
	Digest     []byte       // the digest of this request, which is the only identifier
	SeqID      uint64
}

type Prepare struct {
	Digest     []byte // To identify which request is prepared by this node
	SeqID      uint64
	SenderInfo *NodeInfo // To identify who send this message
}

type Commit struct {
	Digest     []byte // To identify which request is prepared by this node
	SeqID      uint64
	SenderInfo *NodeInfo // To identify who send this message
}

type Reply struct {
	MessageID  uint64
	SenderInfo *NodeInfo
	Result     bool
}

type RequestOldMessage struct {
	SeqStartHeight uint64
	SeqEndHeight   uint64
	ServerNode     *NodeInfo // send this request to the server node
	SenderInfo     *NodeInfo
}

type SendOldMessage struct {
	SeqStartHeight uint64
	SeqEndHeight   uint64
	OldRequest     []*PbftRequest
	ReceiverInfo   *NodeInfo
}

type ErrReport struct {
	NodeAddr string
	Err      string
}
