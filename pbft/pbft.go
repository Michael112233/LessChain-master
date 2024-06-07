// The pbft consensus process

package pbft

import (
	"go-w3chain/core"
	"go-w3chain/pbft/pbft_log"
	"sync"
)

type PbftConsensusNode struct {
	NodeInfo *core.NodeInfo

	node_nums      uint32 // the number of nodes in this pfbt, denoted by N
	malicious_nums uint32 // f, 3f + 1 = N
	view           uint32 // denote the view of this pbft, the main node can be inferred from this variant

	// the control message and message checking utils in pbft
	sequenceID        uint64                             // the message sequence id of the pbft
	requestPool       map[string]*core.PbftRequest       // RequestHash to Request
	cntPrepareConfirm map[string]map[*core.NodeInfo]bool // count the prepare confirm message, [messageHash][Node]bool
	cntCommitConfirm  map[string]map[*core.NodeInfo]bool // count the commit confirm message, [messageHash][Node]bool
	isCommitBordcast  map[string]bool                    // denote whether the commit is broadcast
	isReply           map[string]bool                    // denote whether the message is reply
	replyCnt          int
	gotEnoughReply    map[uint64]bool   // leader 已收到消息的足够多reply
	height2Digest     map[uint64]string // sequence (block height) -> request, fast read

	// locks about pbft
	sequenceLock sync.Mutex // the lock of sequence
	lock         sync.Mutex // lock the stage
	askForLock   sync.Mutex // lock for asking for a serise of requests

	// seqID of other Shards, to synchronize
	seqIDMap   map[uint64]uint64
	seqMapLock sync.Mutex

	// logger
	pl *pbft_log.PbftLog
	// tcp control
	tcpPoolLock sync.Mutex

	// to handle the message in the pbft
	ihm PbftInsideExtraHandleMod

	messageHub core.MessageHub

	// notify uplayer that current consensus is done
	OneConsensusDone chan struct{}
}

// generate a pbft consensus for a node
func NewPbftNode(nodeInfo *core.NodeInfo, shardSize uint32, messageHandleType string) *PbftConsensusNode {
	p := new(PbftConsensusNode)
	p.node_nums = shardSize

	p.NodeInfo = nodeInfo

	p.sequenceID = 0
	p.requestPool = make(map[string]*core.PbftRequest)
	p.cntPrepareConfirm = make(map[string]map[*core.NodeInfo]bool)
	p.cntCommitConfirm = make(map[string]map[*core.NodeInfo]bool)
	p.isCommitBordcast = make(map[string]bool)
	p.isReply = make(map[string]bool)
	p.replyCnt = 0
	p.gotEnoughReply = make(map[uint64]bool)
	p.height2Digest = make(map[uint64]string)
	p.malicious_nums = (p.node_nums - 1) / 3
	p.view = 0

	p.seqIDMap = make(map[uint64]uint64)

	p.pl = pbft_log.NewPbftLog(nodeInfo.ShardID, nodeInfo.NodeID)

	// choose how to handle the messages in pbft or beyond pbft
	switch string(messageHandleType) {
	default:
		p.ihm = &RawPbftInsideExtraHandleMod{
			pbftNode: p,
		}
	}

	p.OneConsensusDone = make(chan struct{}, 1)

	return p
}

func (p *PbftConsensusNode) Reset() {
	p.requestPool = make(map[string]*core.PbftRequest)
	p.cntPrepareConfirm = make(map[string]map[*core.NodeInfo]bool)
	p.cntCommitConfirm = make(map[string]map[*core.NodeInfo]bool)
	p.isCommitBordcast = make(map[string]bool)
	p.isReply = make(map[string]bool)
	p.replyCnt = 0
	p.gotEnoughReply = make(map[uint64]bool)
	p.height2Digest = make(map[uint64]string)
}

func (p *PbftConsensusNode) SetMessageHub(hub core.MessageHub) {
	p.messageHub = hub
}

func (p *PbftConsensusNode) GetNodes_num() uint32 {
	return p.node_nums
}
