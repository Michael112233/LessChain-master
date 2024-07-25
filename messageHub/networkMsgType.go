package messageHub

const (
	ShardSendGenesis   string = "ShardSendGenesis"
	BooterSendContract string = "BooterSendContract"

	ComGetHeight string = "ComGetHeight"

	ComGetState    string = "ComGetState"
	ComGetTx       string = "ComGetTx"
	ShardSendState string = "ShardSendState"

	ClientSendTx        string = "ClientSendTx"
	ClientSetInjectDone string = "ClientSetInjectDone"
	ComSendTxReceipt    string = "ComSendTxReceipt"

	ComSendBlock string = "ComSendBlock"

	LeaderInitMultiSign string = "LeaderInitMultiSign"
	MultiSignReply      string = "MultiSignReply"

	LeaderInitReconfig                string = "LeaderInitReconfig"
	SendReconfigResult2ComLeader      string = "SendReconfigResult2ComLeader"
	SendReconfigResults2AllComLeaders string = "SendReconfigResults2AllComLeaders"
	SendReconfigResults2ComNodes      string = "SendReconfigResults2ComNodes"
	GetPoolTx                         string = "GetPoolTx" // 新leader向旧leader请求交易池中的交易
	GetSyncData                       string = "GetSyncData"
	SendNewNodeTable2Client           string = "SendNewNodeTable2Client"

	// pbft part
	CPrePrepare        string = "CPrePrepare"
	CPrepare           string = "CPrepare"
	CCommit            string = "CCommit"
	CReply             string = "CReply"
	CRequestOldrequest string = "CRequestOldrequest"
	CSendOldrequest    string = "CSendOldrequest"

	NodeSendInfo string = "NodeSendInfo"

	ReportError string = "ReportError"
	ReportAny   string = "ReportAny"
)
