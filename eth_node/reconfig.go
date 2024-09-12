package eth_node

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/utils"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

type HandleReconfigMsgs struct {
}

var sum int
var finishSyncCh chan struct{}
var oldNodeTable map[uint32]map[uint32]string

// leader节点调用
func (n *EthNode) AddReconfigResult(res *core.ReconfigResult) {
	if n.NodeInfo.NodeID != 0 {
		log.Error(fmt.Sprintf("wrong invoking function. only leader node can invoke this function. curNodeInfo: %v", n.NodeInfo))
	}

	if n.reconfigResults == nil { // 第一次重组时
		n.reconfigResults = make([]*core.ReconfigResult, 0)
		n.reconfigResults = append(n.reconfigResults, res)
	} else {
		last := n.reconfigResults[len(n.reconfigResults)-1]
		if last.SeedHeight < res.SeedHeight { // 是上次重组时留下的
			n.reconfigResults = make([]*core.ReconfigResult, 0)
			n.reconfigResults = append(n.reconfigResults, res)
		} else { // 是当前重组
			n.reconfigResults = append(n.reconfigResults, res)
		}
	}
}

// leader节点调用
func (n *EthNode) AddReconfigResults(res *core.ComReconfigResults) {
	if n.com2ReconfigResults == nil { // 第一次重组时
		n.com2ReconfigResults = make(map[uint32]*core.ComReconfigResults)
		n.com2ReconfigResults[res.ComID] = res
	} else {
		for _, last := range n.com2ReconfigResults {
			if last.Results[0].SeedHeight < res.Results[0].SeedHeight { // 是上次重组时留下的
				n.com2ReconfigResults = make(map[uint32]*core.ComReconfigResults)
				n.com2ReconfigResults[res.ComID] = res
			} else { // 是当前重组
				if _, ok := n.com2ReconfigResults[res.ComID]; ok {
					log.Error(fmt.Sprintf("this committee's reconfig result has been received. why receive again? from_comID: %d", res.ComID))
				}
				n.com2ReconfigResults[res.ComID] = res
			}
			break // 拿到任意一个元素后即可结束遍历
		}
	}
}

func (n *EthNode) InitReconfig(data *core.InitReconfig) {
	oldNodeTable = cfg.ComNodeTable
	log.Debug("InitReconfig...", "comID", n.NodeInfo.ComID, "seedHeight", data.SeedHeight, "seed", data.Seed, "oldtable", oldNodeTable)
	n.com.SetOldTxPool()
	n.SetOldTxPool()
	//n.GetShardInfo()
	data.ComNodeNum = uint32(n.CommitteeSize)
	addrs := n.com.GetNodeAddrs()
	stateDB := n.com.GetStateDB()
	for i := 0; i < len(addrs); i++ {
		addr := addrs[i]
		state := stateDB.GetBalance(addr)
		log.Debug("Before Init", "addr", addr, "state", state)
	}
	n.messageHub.Send(core.MsgTypeLeaderInitReconfig, n.NodeInfo.ComID, data, nil)
}

func (n *EthNode) GetShardInfo() {
	//callback1 := func(res ...interface{}) {
	//	data := res[0].(*core.ExecutionInfo)
	//	log.Info("receive shard info")
	//	n.oppositeExecutionInfo = data
	//}
	//n.messageHub.Send(core.MsgTypeGetExecutionInfo, 1^n.NodeInfo.ComID, nil, callback1)
}

func (n *EthNode) HandleLeaderInitReconfig(data *core.InitReconfig) {
	oldNodeTable = cfg.ComNodeTable
	n.com.UpdateTbChainHeight(data.SeedHeight)

	acc := n.GetAccount()
	vrfValue := acc.GenerateVRFOutput(data.Seed[:]).RandomValue
	newComId := utils.VrfValue2Shard(vrfValue, uint32(n.committeeNum))
	log.Debug("HandleLeaderInitReconfig", "old", n.NodeInfo.ComID, "")

	reply := &core.ReconfigResult{
		Seed:         data.Seed,
		SeedHeight:   data.SeedHeight,
		Vrf:          vrfValue,
		Addr:         acc.accountAddr,
		OldNodeInfo:  n.NodeInfo,
		Belong_ComID: data.ComID,
		NewComID:     newComId,
	}
	n.reconfigResult = reply
	n.messageHub.Send(core.MsgTypeSendReconfigResult2ComLeader, data.ComID, reply, nil)
}

func (n *EthNode) HandleSendReconfigResult2ComLeader(data *core.ReconfigResult) {
	log.Info("HandleSendReconfigResult2ComLeader")
	// 省略对vrf的检查...

	n.reconfigResLock.Lock()

	n.AddReconfigResult(data)
	if len(n.reconfigResults) == int(n.CommitteeSize) {
		res := &core.ComReconfigResults{
			ComID:      n.NodeInfo.ComID,
			Results:    n.reconfigResults,
			ComNodeNum: uint32(n.CommitteeSize),
		}
		// 发给自己也用网络，不直接存，这样可以统一处理
		n.reconfigResLock.Unlock()
		n.messageHub.Send(core.MsgTypeSendReconfigResults2AllComLeaders, n.NodeInfo.ComID, res, nil)
	} else {
		n.reconfigResLock.Unlock()
	}
}

func (n *EthNode) HandleSendReconfigResults2AllComLeaders(data *core.ComReconfigResults) {
	//log.Info("HandleSendReconfigResults2AllComLeaders")
	// 省略对vrf的检查...

	n.reconfigResLock.Lock()
	defer n.reconfigResLock.Unlock()

	n.AddReconfigResults(data)
	log.Debug("HandleSendReconfigResults2AllComLeaders", "Reconfig Result", len(n.com2ReconfigResults))
	if len(n.com2ReconfigResults) == n.committeeNum {
		// 将所有vrf结果发送给委员会内的节点，包括发送者leader本身
		n.messageHub.Send(core.MsgTypeSendReconfigResults2ComNodes, n.NodeInfo.ComID, n.com2ReconfigResults, nil)
	}

}

func (n *EthNode) HandleSendReconfigResults2ComNodes(data *map[uint32]*core.ComReconfigResults) {
	log.Info("HandleSendReconfigResults2ComNodes")
	// 省略对vrf的检查...

	// 先得到每个新委员会中的节点结果
	newCom2Results := make(map[uint32][]*core.ReconfigResult)
	for _, rets := range *data {
		for _, res := range rets.Results {
			newCom2Results[res.NewComID] = append(newCom2Results[res.NewComID], res)
		}
	}
	// 对每个新委员会中结果按vrf从小到大排序
	for _, results := range newCom2Results {
		sort.Slice(results, func(i, j int) bool {
			return bytes.Compare(results[i].Vrf, results[j].Vrf) < 0
		})
	}

	// 更新ComNodeTable和本节点的NodeInfo
	localNodeInfo := n.NodeInfo
	newComNodeTable := make(map[uint32]map[uint32]string)
	var oldComLeaderAddr string
	var i uint32
	for i = 0; i < uint32(n.committeeNum); i++ {
		newComNodeTable[i] = make(map[uint32]string)
		for newID, result := range newCom2Results[i] {
			newComNodeTable[i][uint32(newID)] = result.OldNodeInfo.NodeAddr
			if *result.OldNodeInfo == *localNodeInfo {
				newNodeInfo := &core.NodeInfo{
					ShardID:  n.reconfigResult.NewComID,
					ComID:    n.reconfigResult.NewComID,
					NodeID:   uint32(newID),
					NodeAddr: localNodeInfo.NodeAddr,
				}
				log.Debug(fmt.Sprintf("local nodeInfo updated... before reconfiguration: %v after: %v", localNodeInfo, newNodeInfo))
				n.updateNodeInfo(newNodeInfo)
			}
			if result.OldNodeInfo.ComID == n.reconfigResult.NewComID && result.OldNodeInfo.NodeID == 0 { // 本节点所在的新委员会原来的leader
				oldComLeaderAddr = result.OldNodeInfo.NodeAddr
			}
		}
	}

	cfg.ComNodeTable = newComNodeTable
	// 各分片的新leader
	com2Leader := make(map[uint32]string)
	for shardID, list := range cfg.ComNodeTable {
		com2Leader[shardID] = list[0]
	}
	log.Debug(fmt.Sprintf("NewComLeaders：%v", com2Leader))
	for shardID, list := range cfg.ComNodeTable {
		log.Debug(fmt.Sprintf("comID: %d nodeAddrs: %v", shardID, list))
	}
	//n.oldComID = localNodeInfo.ComID

	n.EndReconfig(newCom2Results, oldComLeaderAddr)
}

func (n *EthNode) updateNodeInfo(newNodeInfo *core.NodeInfo) {
	n.NodeInfo = newNodeInfo
	n.pbftNode.NodeInfo = newNodeInfo
}

func (n *EthNode) EndReconfig(newCom2Results map[uint32][]*core.ReconfigResult, oldComLeaderAddr string) {
	log.Info("EndReconfig")
	// 更新委员会节点数量
	n.CommitteeSize = len(newCom2Results[n.NodeInfo.ComID])
	log.Debug(fmt.Sprintf("after reconfiguration, com %d has %d nodes in total.", n.NodeInfo.ComID, n.CommitteeSize))
	if n.CommitteeSize < 4 { // pbft协议至少需要包括leader在内的3个节点
		report := &core.ErrReport{
			NodeAddr: n.NodeInfo.NodeAddr,
			Err:      fmt.Sprintf("after reconfiguration, com %d has less than 4 nodes, not enough for pbft consensus", n.NodeInfo.ComID),
		}
		n.messageHub.Send(core.MsgTypeReportError, 0, report, nil)
		log.Error(fmt.Sprintf("after reconfiguration, com %d has less than 4 nodes, not enough for pbft consensus", n.NodeInfo.ComID))
	}

	// 更新合约上的地址
	if utils.IsComLeader(n.NodeInfo.NodeID) {
		comResults := newCom2Results[n.NodeInfo.ComID]
		log.Debug("com2result", "ans", newCom2Results[n.NodeInfo.ComID])
		addrs := make([]common.Address, 0)
		vrfs := make([][]byte, 0)
		for _, res := range comResults {
			addrs = append(addrs, res.Addr)
			vrfs = append(vrfs, res.Vrf)
		}
		n.com.AdjustRecordedAddrs(addrs, vrfs, comResults[0].SeedHeight)
	}

	// 重组开始时已经调用过一次，此处再次调用，是因为重组过程节点可能继续收到客户端发送的交易
	n.com.SetOldTxPool()

	//n.com.Close()
	// 重新启动委员会和worker、新建交易池
	n.com.ConsensusStart(n.NodeInfo.NodeID)

	if utils.IsComLeader(n.NodeInfo.NodeID) { // 每个委员会的leader都会给客户端发送新表
		n.messageHub.Send(core.MsgTypeSendNewNodeTable2Client, 0, cfg.ComNodeTable, nil)
	}

	syncStartTime := time.Now()
	// 同步交易池
	var sizeofPoolTx int

	if utils.IsComLeader(n.NodeInfo.NodeID) {
		var poolTx *core.PoolTx
		if n.NodeInfo.NodeAddr == oldComLeaderAddr {
			poolTx = n.com.HandleGetPoolTx()
			n.com.SetPoolTx(poolTx)
		} else {
			getPoolTxsCh := make(chan struct{}, 1)
			callback := func(res ...interface{}) {
				poolTx = res[0].(*core.PoolTx)
				n.com.SetPoolTx(poolTx)

				getPoolTxsCh <- struct{}{}
			}
			request := &core.GetPoolTx{
				ServerAddr:   oldComLeaderAddr,
				ClientAddr:   n.NodeInfo.NodeAddr,
				RequestComID: n.NodeInfo.ComID,
			}

			n.messageHub.Send(core.MsgTypeGetPoolTx, n.NodeInfo.ComID, request, callback)
			// 等待交易池更新后再启动worker
			<-getPoolTxsCh
		}
		sizeofPoolTx = len(utils.EncodeAny(poolTx))
		log.Info("Reconfig Tx nums", "num", sizeofPoolTx)
		sum += sizeofPoolTx
		//n.messageHub.Send(core.MsgTypeGetResults, 0, sizeofPoolTx, nil)
	}

	if utils.IsComLeader(n.NodeInfo.NodeID) {
		// 根据不同同步方式，选择需要额外同步的内容
		//switch n.reconfigMode {
		//case "lesssync": // 存储共识分离，只需同步交易池
		//	n.lessSync(sizeofPoolTx, syncStartTime)
		//case "fullsync":
		//n.fullsync(sizeofPoolTx, syncStartTime)
		//case "fastsync":
		finishSyncCh = make(chan struct{}, 1)
		n.fastsync(sizeofPoolTx, syncStartTime)
		//case "tMPTsync":
		//	n.tMPTsync(sizeofPoolTx, syncStartTime)
		//default:
		//	log.Error("unknown reconfig mode", "mode", n.reconfigMode)
		//}
		<-finishSyncCh
	}

	log.Info("Start resetting pbft")

	// 重置pbft
	if utils.IsComLeader(n.NodeInfo.NodeID) {
		n.pbftNode.Reset()
	}

	log.Info("Try to start worker")
	if utils.IsComLeader(n.NodeInfo.NodeID) {
		log.Debug("reconfig", "leader", n.NodeInfo.NodeAddr)
		n.com.WorkerStart()
	}

	// 删除无用的长连接，释放系统资源，防止某些端口被强制关闭
	// n.messageHub.Send(core.MsgTypeClearConnection, 0, n.NodeInfo, nil)

	// todo
}

func (n *EthNode) lessSync(sizeofPoolTx int, syncStartTime time.Time) {
	elapsed := time.Since(syncStartTime)
	reportMsg := fmt.Sprintf("shardID: %d msgType: %s sizeof states(bytes): %d sizeof blocks(bytes): %d sizeof poolTx(bytes): %d sync time: %d",
		n.NodeInfo.ComID, "lesssync", 0, 0, sizeofPoolTx, elapsed.Milliseconds())
	n.messageHub.Send(core.MsgTypeReportAny, 0, reportMsg, nil)
}

//	func (n *Node) tMPTsync(sizeofPoolTx int, syncStartTime time.Time) {
//		shardLeader := cfg.NodeTable[n.NodeInfo.ComID][0]
//		request := &core.GetSyncData{
//			ServerAddr: shardLeader,
//			ClientAddr: n.NodeInfo.NodeAddr,
//			ShardID:    n.NodeInfo.ComID,
//			SyncType:   "tMPTsync",
//		}
//		var data *core.SyncData
//		if n.NodeInfo.NodeAddr == shardLeader {
//			data = n.shard.HandleGetSyncData(request)
//		} else {
//			getSyncDataCh := make(chan struct{}, 1)
//			callback := func(res ...interface{}) {
//				data = res[0].(*core.SyncData)
//				log.Debug("tMPTsync data received", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
//				getSyncDataCh <- struct{}{}
//			}
//			n.messageHub.Send(core.MsgTypeGetSyncData, n.NodeInfo.ComID, request, callback)
//			// 等待交易池更新后再启动worker
//			<-getSyncDataCh
//		}
//
//		elapsed := time.Since(syncStartTime)
//		reportMsg := fmt.Sprintf("shardID: %d msgType: %s sizeof states(bytes): %d sizeof blocks(bytes): %d sizeof poolTx(bytes): %d sync time: %d",
//			n.NodeInfo.ComID, "tMPTsync", len(utils.EncodeAny(data.States)), len(utils.EncodeAny(data.Blocks)), sizeofPoolTx, elapsed.Milliseconds())
//		n.messageHub.Send(core.MsgTypeReportAny, 0, reportMsg, nil)
//	}
//func (n *EthNode) fullsync(sizeofPoolTx int, syncStartTime time.Time) {
//	shardLeader := cfg.NodeTable[n.NodeInfo.ComID][0]
//	request := &core.GetSyncData{
//		ServerAddr: shardLeader,
//		ClientAddr: n.NodeInfo.NodeAddr,
//		ShardID:    n.NodeInfo.ComID,
//		SyncType:   "fullsync",
//	}
//	var data *core.SyncData
//	if n.NodeInfo.NodeAddr == shardLeader {
//		data = n.com.HandleGetSyncData(request)
//		log.Debug("327 fullsync", "State Num", len(data.States))
//	} else {
//		getSyncDataCh := make(chan struct{}, 1)
//		callback := func(res ...interface{}) {
//			data = res[0].(*core.SyncData)
//			log.Debug("fullsync data recei ved", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
//			sum := len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
//			stateDB := n.com.GetStateDB()
//			log.Debug("fullsync", "State Num", len(data.States))
//			for key, _ := range data.States {
//				log.Debug("fullsync", "addr", key, "balance", stateDB.GetBalance(key))
//			}
//			block := data.Blocks[len(data.Blocks)-1]
//			for i := 0; i < len(block.Transactions); i++{
//				log.Debug("fullsync", "Sender", block.Transactions[i].Sender_sid, "Receiver", block.Transactions[i].Recipient_sid, "value", block.Transactions[i].Value)
//			}
//			n.messageHub.Send(core.MsgTypeGetResults, uint32(0), sum, nil)
//			getSyncDataCh <- struct{}{}
//		}
//		n.messageHub.Send(core. MsgTypeGetSyncData, n.NodeInfo.ComID, request, callback)
//		// 等待交易池更新后再启动worker
//		<-getSyncDataCh
//	}
//
//	elapsed := time.Since(syncStartTime)
//	reportMsg := fmt.Sprintf("shardID: %d msgType: %s sizeof states(bytes): %d sizeof blocks(bytes): %d sizeof poolTx(bytes): %d sync time: %d",
//		n.NodeInfo.ComID, "fullsync", len(utils.EncodeAny(data.States)), len(utils.EncodeAny(data.Blocks)), sizeofPoolTx, elapsed.Milliseconds())
//	n.messageHub.Send(core.MsgTypeReportAny, 0, reportMsg, nil)
//}

func (n *EthNode) fastsync(sizeofPoolTx int, syncStartTime time.Time) {
	log.Debug("fastsync", "nodetable", cfg.ComNodeTable)
	shardLeader := oldNodeTable[n.NodeInfo.ComID][0]
	var addrlist []common.Address
	sum := 0
	request := &core.GetSyncData{
		ServerAddr: shardLeader,
		ClientAddr: n.NodeInfo.NodeAddr,
		ShardID:    n.NodeInfo.ComID,
		SyncType:   "fastsync",
	}
	request1 := &core.GetSyncData{
		ServerAddr: oldNodeTable[1^(n.NodeInfo.ComID)][0],
		ClientAddr: n.NodeInfo.NodeAddr,
		ShardID:    1 ^ n.NodeInfo.ComID,
		SyncType:   "fastsync",
	}
	getSyncDataCh := make(chan struct{}, 1)
	getSyncDataCh1 := make(chan struct{}, 1)
	var data *core.SyncData
	statelist := make(map[common.Address]*types.StateAccount)

	if request.ServerAddr != n.NodeInfo.NodeAddr {
		callback := func(res ...interface{}) {
			data = res[0].(*core.SyncData)
			log.Debug("fastsync data received", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
			stateDB := n.com.GetStateDB()
			for addr, state := range data.States {
				//log.Debug("fastsync1", "addr", addr, "state", state)
				addrlist = append(addrlist, addr)
				stateDB.SetBalance(addr, state.Balance)
				statelist[addr] = state
			}
			//n.com.AddBlock(data.Blocks[len(data.Blocks)-1])
			sum = len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
			//n.messageHub.Send(core.MsgTypeGetResults, uint32(0), sum, nil)

			getSyncDataCh <- struct{}{}
		}
		n.messageHub.Send(core.MsgTypeGetSyncData, n.NodeInfo.ComID, request, callback)
		sum = len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
		log.Debug("output", "sum", sum, "total", len(data.States))
	} else {
		//data = n.com.HandleGetSyncData(request)
		//log.Debug("fastsync data received", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
		//stateDB := n.com.GetStateDB()
		//for addr, state := range data.States {
		//	addrlist = append(addrlist, addr)
		//	//log.Debug("fastsync1", "addr", addr, "state", state)
		//	stateDB.SetBalance(addr, state.Balance)
		//}
		//sum += len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
		getSyncDataCh <- struct{}{}
	}

	if request1.ServerAddr != n.NodeInfo.NodeAddr {
		callback1 := func(res ...interface{}) {
			data = res[0].(*core.SyncData)
			log.Debug("fastsync data received", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
			stateDB := n.com.GetStateDB()
			for addr, state := range data.States {
				addrlist = append(addrlist, addr)
				stateDB.SetBalance(addr, state.Balance)
				statelist[addr] = state
			}

			sum += len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
			//log.Debug("output", "sum", sum)
			//n.messageHub.Send(core.MsgTypeGetResults, uint32(0), sum, nil)
			getSyncDataCh1 <- struct{}{}
		}
		n.messageHub.Send(core.MsgTypeGetSyncData, 1^n.NodeInfo.ComID, request1, callback1)
		log.Debug("output", "sum", sum, "total", len(data.States)+len(data.States))
	} else {
		//data = n.com.HandleGetSyncData(request1)
		//log.Debug("fastsync data received", "len(states)", len(data.States), "len(blocks)", len(data.Blocks))
		//stateDB := n.com.GetStateDB()
		//for addr, state := range data.States {
		//	addrlist = append(addrlist, addr)
		//	//log.Debug("fastsync1", "addr", addr, "state", state)
		//	stateDB.SetBalance(addr, state.Balance)
		//}
		//sum += len(utils.EncodeAny(data.States)) + len(utils.EncodeAny(data.Blocks))
		getSyncDataCh1 <- struct{}{}
	}
	// 等待交易池更新后再启动worker
	<-getSyncDataCh1
	<-getSyncDataCh

	n.com.SetAllAddrs(addrlist)
	n.messageHub.Send(core.MsgTypeGetResults, uint32(0), sum, nil)
	elapsed := time.Since(syncStartTime)
	reportMsg := fmt.Sprintf("shardID: %d msgType: %s sizeof states(bytes): %d sizeof blocks(bytes): %d sizeof poolTx(bytes): %d sync time: %d",
		n.NodeInfo.ComID, "fastsync", len(utils.EncodeAny(data.States)), len(utils.EncodeAny(data.Blocks)), sizeofPoolTx, elapsed.Milliseconds())
	n.messageHub.Send(core.MsgTypeReportAny, 0, reportMsg, nil)
	//n.messageHub.Send(core.MsgTypeSendSync2Node, n.NodeInfo.ComID, statelist, nil)

	finishSyncCh <- struct{}{}
}
