package core

type MessageHub interface {
	Send(msgType uint32, id uint32, msg interface{}, callback func(...interface{}))
}

const (
	MsgTypeNone uint32 = iota

	MsgTypeComGetHeightFromShard

	MsgTypeComGetStateFromShard
	MsgTypeShardSendStateToCom

	MsgTypeShardSendGenesis
	MsgTypeBooterSendContract

	MsgTypeClientInjectTX2Committee
	MsgTypeCommitteeReply2Client
	MsgTypeSetInjectDone2Nodes
	MsgTypeComAddTb2TBChain
	// MsgTypeCommitteeInitialAddrs
	MsgTypeComSendNewAddrs
	MsgTypeGetTB

	MsgTypeSendBlock2Shard
	MsgTypeReady4Reconfig
	MsgTypeTBChainPushTB2Client
	MsgTypeTBChainPushTB2Coms

	MsgTypeGetLatestBlockHashFromEthChain
	MsgTypeGetBlockHashFromEthChain

	MsgTypeLeaderInitMultiSign
	MsgTypeSendMultiSignReply

	MsgTypeLeaderInitReconfig
	MsgTypeSendReconfigResult2ComLeader
	MsgTypeSendReconfigResults2AllComLeaders
	MsgTypeSendReconfigResults2ComNodes
	MsgTypeGetPoolTx // 新leader向旧leader请求交易池中的交易
	MsgTypeGetSyncData
	MsgTypeShardSendFastSyncData
	MsgTypeSendNewNodeTable2Client

	//////////////////////
	//// pbft module /////
	//////////////////////
	MsgTypePbftPropose
	MsgTypePbftPrePrepare
	MsgTypePbftPrepare
	MsgTypePbftCommit
	MsgTypePbftReply
	MsgTypePbftRequestOldMessage
	MsgTypePbftSendOldMessage

	MsgTypeNodeSendInfo2Leader

	MsgTypeClearConnection

	MsgTypeReportError
	MsgTypeReportAny
)
