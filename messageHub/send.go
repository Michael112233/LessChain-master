package messageHub

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"io"
	"math/big"
	"net"
	"time"
)

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

func dial(addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Debug("DialTCPError", "target_addr", addr, "err", err)
		// 再dial一次
		log.Debug("Try dial again...")
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			log.Debug("DialTCPError", "target_addr", addr, "err", err)
			return nil, nil
		} else {
			log.Debug("dial success.", "target_addr", addr)
		}
	}
	return conn, nil
}

// 每隔一定时间尝试一次dial，直到成功
// 易造成死循环，谨慎使用
func mustDial(addr string, interval time.Duration) net.Conn {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(interval)
		} else {
			return conn
		}
	}
}

func packMsg(msgType string, data []byte) []byte {
	msg := &core.Msg{
		MsgType: msgType,
		Data:    data,
	}

	var buf bytes.Buffer
	msgEnc := gob.NewEncoder(&buf)
	err := msgEnc.Encode(msg)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "msg", msg)
	}

	msgBytes := buf.Bytes()

	// 前缀加上长度，防止粘包
	networkBuf := make([]byte, 4+len(msgBytes))
	binary.BigEndian.PutUint32(networkBuf[:4], uint32(len(msgBytes)))
	copy(networkBuf[4:], msgBytes)

	return networkBuf
}

func comGetHeightFromShard(shardID uint32, msg interface{}) *big.Int {
	data := msg.(*core.ComGetHeight)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ComGetHeight", buf.Bytes())

	// 从分片的leader节点处获取
	addr := cfg.NodeTable[shardID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"comGetHeightFromShard", shardID, -1, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	_, err = conn.Write(msg_bytes)
	if err != nil {
		log.Error("WriteError", "err", err)
	}
	log.Info("Msg Sent: ComGetHeight", "data", "empty data")

	// 等待回复

	// 首先读取消息长度的四个字节
	lengthBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, lengthBuf)
	if err != nil {
		log.Error("ReadLengthError", "err", err)
	}
	// 解析这四个字节为int32来获取消息长度
	msgLength := int(binary.BigEndian.Uint32(lengthBuf))

	// 根据消息长度分配缓冲区
	msgBuf := make([]byte, msgLength)
	_, err = io.ReadFull(conn, msgBuf)
	if err != nil {
		log.Error("ReadMsgError", "err", err)
	}

	height := new(big.Int)
	decodeBuf := bytes.NewReader(msgBuf)
	decoder := gob.NewDecoder(decodeBuf)
	err = decoder.Decode(height)
	if err != nil {
		log.Error("Failed to decode height using gob", "err", err)
	}

	log.Info("Msg Response Received: ComGetHeight", "height", height)

	return height
}

/*
	分片向一个中心化节点发送创世区块信标和初始账户地址

该消息只进行一次，不需通过长连接发送
*/
func shardSendGenesis(msg interface{}) {
	data := msg.(*core.ShardSendGenesis)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ShardSendGenesis", buf.Bytes())

	// 连接到目标地址
	conn, err := net.Dial("tcp", data.Target_nodeAddr)
	if err != nil {
		log.Error("DialError", "err", err)
	}
	defer conn.Close()

	// 写入消息
	_, err = conn.Write(msg_bytes)
	if err != nil {
		log.Error("WriteError", "err", err)
	}

	log.Info("Msg Sent: ShardSendGenesis", "data", data)
}

func clientInjectTx2Com(comID uint32, msg interface{}) {
	data := msg.([]*core.Transaction)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ClientSendTx", buf.Bytes())

	// 发送给委员会的leader即可
	addr := cfg.ComNodeTable[comID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"clientInjectTx2Com", -1, comID, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: ClientSendTx", "targetComID", comID, "targetAddr", addr, "tx count", len(data))
}

func clientSetInjectDone2Nodes(cid uint32) {
	data := &core.ClientSetInjectDone{Cid: cid}
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ClientSetInjectDone", buf.Bytes())

	// 向所有节点发送交易注入完成信息
	var i, j uint32
	for i = 0; i < uint32(shardNum); i++ {
		for j = 0; j < uint32(comAllNodeNum); j++ {
			addr := cfg.NodeTable[i][j]
			conn, ok := conns2Node.Get(addr)
			if !ok {
				conn, err = dial(addr)
				if err != nil {
					log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
						"clientSetInjectDone2Nodes", i, -1, j, addr))
				}
				conns2Node.Add(addr, conn)
			}
			_, err := conn.Write(msg_bytes)
			// if err != nil {
			// 	panic(err)
			// }
			// 尝试重新建立连接后再发送一次，若依然失败则panic
			if err != nil {
				log.Debug(fmt.Sprint("write tcp error: ", err))
				conn, err = dial(addr)
				if err != nil {
					log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
						"clientSetInjectDone2Nodes", i, -1, j, addr))
				}
				conns2Node.Add(addr, conn)
				_, err := conn.Write(msg_bytes)
				if err != nil {
					panic(err)
				}
			}

			conn.Close()
			log.Info("Msg Sent: ClientSetInjectDone", "clientID", cid, "shardID", i, "nodeID", j)
		}
	}
}

func comGetStateFromShard(shardID uint32, msg interface{}) {
	data := msg.(*core.ComGetState)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ComGetState", buf.Bytes())

	// 从分片的leader节点获取
	addr := cfg.NodeTable[shardID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"comGetStateFromShard", shardID, -1, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: ComGetState", "addr count", len(data.AddrList))
}

func shardSendStateToCom(comID uint32, msg interface{}) {
	data := msg.(*core.ShardSendState)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ShardSendState", buf.Bytes())
	// 只发送给委员会的leader节点
	addr := cfg.ComNodeTable[comID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"shardSendStateToCom", -1, comID, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: ShardSendState", "addr count", len(data.AccountData))
}

func comSendBlock2Shard(shardID uint32, msg interface{}) {
	data := msg.(*core.ComSendBlock)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ComSendBlock", buf.Bytes())

	// 只发送给分片的leader节点
	addr := cfg.NodeTable[shardID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"comSendBlock2Shard", shardID, -1, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: ComSendBlock", "toShardID", shardID, "tx count", len(data.Block.Transactions))
}

func comSendReply2Client(clientID uint32, msg interface{}) {
	data := msg.([]*result.TXReceipt)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("ComSendTxReceipt", buf.Bytes())

	addr := cfg.ClientTable[clientID]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"comSendReply2Client", -1, -1, -1, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: ComSendTxReceipt", "toClientID", clientID, "tx count", len(data))
}

// 调用beaconChain包，再通过ethClient与ethchain交互
// 或者beaconChain监听到的ethchain事件，发送给客户端、委员会

func comAddTb2TBChain(nodeID uint32, msg interface{}) {
	data := msg.(*core.SignedTB)
	tbChain_ref.AddTimeBeacon(data, nodeID)
}

func getEthLatestBlock(callback func(...interface{})) {
	hash, height := tbChain_ref.GetEthChainLatestBlockHash()
	callback(hash, height)
}

func getEthBlock(msg interface{}, callback func(...interface{})) {
	height := msg.(uint64)
	hash, got_height := tbChain_ref.GetEthChainBlockHash(height)
	callback(hash, got_height)
}

func tbChainPushBlock2Client(msg interface{}) {
	if client_ref == nil {
		return
	}
	data := msg.(*beaconChain.TBBlock)
	client_ref.AddTBs(data)
}

func tbChainPushBlock2Com(msg interface{}) {
	if shard_ref == nil {
		return
	}
	data := msg.(*beaconChain.TBBlock)
	shard_ref.ExecutionAddTBs(data)
	shard_ref.ConsensusAddTBs(data)
}

func comLeaderInitMultiSign(comID uint32, msg interface{}) {
	data := msg.(*core.ComLeaderInitMultiSign)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(LeaderInitMultiSign, buf.Bytes())

	// 向委员会中的所有共识节点发送（包括自己）
	var i uint32
	for i = 0; i < uint32(min(shardSize, len(cfg.ComNodeTable[comID]))); i++ {
		addr := cfg.ComNodeTable[comID][i]
		if addr == "" {
			if i == 3 {
				// pbft最低允许只有3个节点
				continue
			} else {
				log.Error("address is empty.")
			}
		}
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"comLeaderInitMultiSign", -1, comID, i, addr))
			}
			conns2Node.Add(addr, conn)
		}
		_, err := conn.Write(msg_bytes)
		if err != nil {
			log.Debug(fmt.Sprintf("write err: %v", err))
		}
	}
	log.Info("Msg Sent: comLeaderInitMultiSign", "comID", comID)
}

func sendMultiSignReply(comID uint32, msg interface{}) {
	data := msg.(*core.MultiSignReply)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(MultiSignReply, buf.Bytes())

	// 向委员会的leader节点发送

	addr := cfg.ComNodeTable[comID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"sendMultiSignReply", -1, comID, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	_, err = conn.Write(msg_bytes)
	if err != nil {
		panic(err)
	}

	log.Info("Msg Sent: multiSignReply", "comID", comID)
}

//////////////////////////////////////////////////
////  booter  ////
//////////////////////////////////////////////////

func booterSendContract(msg interface{}) {
	data := msg.(*core.BooterSendContract)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg("BooterSendContract", buf.Bytes())

	// 向所有节点发送合约地址等信息
	var i, j uint32
	for i = 0; i < uint32(shardNum); i++ {
		for j = 0; j < uint32(comAllNodeNum); j++ {
			addr := cfg.NodeTable[i][j]
			conn, ok := conns2Node.Get(addr)
			if !ok {
				conn, err = dial(addr)
				if err != nil {
					log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
						"booterSendContract", i, -1, j, addr))
				}
			}
			_, err := conn.Write(msg_bytes)
			if err != nil {
				panic(err)
			}
			conn.Close()
		}

	}

	// 向客户端发送合约地址等信息
	for i = 0; i < uint32(clientNum); i++ {
		addr := cfg.ClientTable[i]
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"booterSendContract", -1, -1, -1, addr))
			}
		}
		_, err := conn.Write(msg_bytes)
		if err != nil {
			panic(err)
		}
		conn.Close()
	}

	log.Info("Msg Sent: BooterSendContract", "data", data)

}

// ///////////////////
// /// pbft //////////
// ///////////////////
func sendPbftMsg(comID uint32, msg interface{}, msgType string) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	var err error

	switch msgType {
	case CPrePrepare:
		data := msg.(*core.PrePrepare)
		err = enc.Encode(data)
	case CPrepare:
		data := msg.(*core.Prepare)
		err = enc.Encode(data)
	case CCommit:
		data := msg.(*core.Commit)
		err = enc.Encode(data)
	case CReply:
		data := msg.(*core.Reply)
		err = enc.Encode(data)
	case CRequestOldrequest:
		data := msg.(*core.RequestOldMessage)
		err = enc.Encode(data)
	case CSendOldrequest:
		data := msg.(*core.SendOldMessage)
		err = enc.Encode(data)
		msg_bytes := packMsg(msgType, buf.Bytes())
		sendOldRequests(data, msg_bytes)
		return
	default:
		log.Error("unknown pbft msg type", "type", msgType)
	}

	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data(interface{})", msg)
	}

	// 序列化后的消息
	msg_bytes := packMsg(msgType, buf.Bytes())

	var i uint32
	nodeAddr := node_ref.NodeInfo.NodeAddr
	for i = 0; i < uint32(min(shardSize, len(cfg.ComNodeTable[comID]))); i++ { // original ShardSize
		if i > 0 && (msgType == CReply || msgType == CRequestOldrequest) { // reply、CRequestOldrequest 只需发给leader
			return
		}
		log.Debug(fmt.Sprintf("%d %v\n", uint32(min(shardSize, len(cfg.ComNodeTable[comID]))), cfg.ComNodeTable))
		addr := cfg.ComNodeTable[comID][i]

		log.Debug(fmt.Sprintf("addr: %v, comid: %v, i: %v\n", addr, comID, i))
		if addr == nodeAddr {
			continue // 不用发给自己
		}
		if addr == "" {
			if i == 3 {
				// pbft最低允许只有3个节点
				continue
			} else {
				log.Error("address is empty.")
			}
		}
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					msgType, -1, comID, i, addr))
			}
			conns2Node.Add(addr, conn)
		}
		writer := bufio.NewWriter(conn)
		writer.Write(msg_bytes)
		writer.Flush()

		if msgType == CPrePrepare {
			log.Info(fmt.Sprintf("Msg Sent: %s ComID: %v, to nodeID: %v seqID: %d", msgType, comID, i, msg.(*core.PrePrepare).SeqID))
		} else {
			log.Info(fmt.Sprintf("Msg Sent: %s ComID: %v, to nodeID: %v", msgType, comID, i))
		}
	}
}

func sendOldRequests(data *core.SendOldMessage, msgBytes []byte) {
	addr := cfg.ComNodeTable[data.ReceiverInfo.ComID][data.ReceiverInfo.NodeID]
	var err error

	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"sendOldRequests", -1, data.ReceiverInfo.ComID, data.ReceiverInfo.NodeID, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msgBytes)
	writer.Flush()

	log.Info(fmt.Sprintf("Msg Sent: %s ComID: %v, to nodeID: %v", CSendOldrequest, data.ReceiverInfo.ComID, data.ReceiverInfo.NodeID))
}

func sendNodeInfo(comID uint32, msg interface{}) {
	data := msg.(*core.NodeSendInfo)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(NodeSendInfo, buf.Bytes())

	addr := cfg.ComNodeTable[comID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn = mustDial(addr, time.Second)
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: NodeSendInfo", "ComID", comID, "to nodeID", 0)
}

type SendReconfigMsgs struct {
}

func leaderInitReconfig(comID uint32, msg interface{}) {
	data := msg.(*core.InitReconfig)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(LeaderInitReconfig, buf.Bytes())

	var i uint32
	// 向包括共识节点在内的所有委员会内节点发送该消息
	for i = 0; i < data.ComNodeNum; i++ {
		addr := cfg.ComNodeTable[comID][i]
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"leaderInitReconfig", -1, comID, i, addr))
			}
			conns2Node.Add(addr, conn)
		}
		writer := bufio.NewWriter(conn)
		writer.Write(msg_bytes)
		writer.Flush()
		log.Info(fmt.Sprintf("Msg Sent: %s ComID: %d nodeID: %d", LeaderInitReconfig, comID, i))
	}

}

func sendReconfigResult2Leader(comID uint32, msg interface{}) {
	data := msg.(*core.ReconfigResult)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(SendReconfigResult2ComLeader, buf.Bytes())

	addr := cfg.ComNodeTable[comID][0]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"sendReconfigResult2Leader", -1, comID, 0, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info(fmt.Sprintf("Msg Sent: %s ComID: %d", SendReconfigResult2ComLeader, comID))
}

func sendReconfigResults2AllLeaders(comID uint32, msg interface{}) {
	data := msg.(*core.ComReconfigResults)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(SendReconfigResults2AllComLeaders, buf.Bytes())

	// 先拷贝待发送地址，否则leader接收到该消息后可能更新地址表
	// 导致某些地址缺失或错误
	target_addrs := make(map[uint32]string)
	for comID, list := range cfg.ComNodeTable {
		target_addrs[comID] = list[0]
	}
	var i uint32
	for i = 0; i < uint32(shardNum); i++ {
		addr := target_addrs[i]
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"sendReconfigResults2AllLeaders", -1, i, 0, addr))
			}
			conns2Node.Add(addr, conn)
		}
		writer := bufio.NewWriter(conn)
		writer.Write(msg_bytes)
		writer.Flush()
		log.Info(fmt.Sprintf("Msg Sent: %s from_ComID: %d to_ComID: %d to_addr: %s", SendReconfigResults2AllComLeaders, comID, i, addr))
	}
}

func sendReconfigResults2ComNodes(comID uint32, msg interface{}) {
	data := msg.(map[uint32]*core.ComReconfigResults)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(SendReconfigResults2ComNodes, buf.Bytes())

	var i uint32
	// 先拷贝待发送地址，否则leader接收到该消息后可能更新地址表
	// 导致某些地址缺失或错误
	target_addrs := cfg.ComNodeTable[comID]
	for i = 0; i < data[comID].ComNodeNum; i++ {
		addr := target_addrs[i]
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"sendReconfigResults2ComNodes", -1, comID, i, addr))
			}
			conns2Node.Add(addr, conn)
		}
		writer := bufio.NewWriter(conn)
		writer.Write(msg_bytes)
		writer.Flush()
		log.Info(fmt.Sprintf("Msg Sent: %s ComID: %d to_nodeID: %d, to_addr: %s", SendReconfigResults2ComNodes, comID, i, addr))
	}
}

func comSendNewAddrs(nodeID uint32, msg interface{}) {
	data := msg.(*core.AdjustAddrs)
	tbChain_ref.SetAddrs(data.Addrs, data.Vrfs, data.SeedHeight, data.ComID, nodeID)
}

func sendNewNodeTable2Client(msg interface{}) {
	data := msg.(map[uint32]map[uint32]string)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(SendNewNodeTable2Client, buf.Bytes())

	var i uint32
	for i = 0; i < uint32(clientNum); i++ {
		addr := cfg.ClientTable[i]
		conn, ok := conns2Node.Get(addr)
		if !ok {
			conn, err = dial(addr)
			if err != nil {
				log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
					"sendNewNodeTable2Client", -1, -1, -1, addr))
			}
			conns2Node.Add(addr, conn)
		}
		writer := bufio.NewWriter(conn)
		writer.Write(msg_bytes)
		writer.Flush()
		log.Info(fmt.Sprintf("Msg Sent: %s to_clientID: %d", SendNewNodeTable2Client, i))
	}

}

func sendGetPoolTx(comID uint32, msg interface{}, callback func(...interface{})) {
	data := msg.(*core.GetPoolTx)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(GetPoolTx, buf.Bytes())

	// 从分片的leader节点处获取
	addr := data.ServerAddr
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"sendNewNodeTable2Client", -1, -1, -1, addr))
		}
		conns2Node.Add(addr, conn)
	}
	_, err = conn.Write(msg_bytes)
	if err != nil {
		log.Error("WriteError", "err", err)
	}
	log.Info(fmt.Sprintf("Msg Sent: %s data: %v", GetPoolTx, "empty data"))

	// 等待回复

	// 首先读取消息长度的四个字节
	lengthBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, lengthBuf)
	if err != nil {
		log.Error("ReadLengthError", "err", err)
	}
	// 解析这四个字节为int32来获取消息长度
	msgLength := int(binary.BigEndian.Uint32(lengthBuf))

	// 根据消息长度分配缓冲区
	msgBuf := make([]byte, msgLength)
	_, err = io.ReadFull(conn, msgBuf)
	if err != nil {
		log.Error("ReadMsgError", "err", err)
	}

	poolTx := new(core.PoolTx)
	decodeBuf := bytes.NewReader(msgBuf)
	decoder := gob.NewDecoder(decodeBuf)
	err = decoder.Decode(poolTx)
	if err != nil {
		log.Error("Failed to decode", "err", err)
	}

	log.Info(fmt.Sprintf("Msg Response Received: %s pendingLen: %d pendingRollbackLen: %d", GetPoolTx, len(poolTx.Pending), len(poolTx.PendingRollback)))

	callback(poolTx)
}

func sendGetSyncData(comID uint32, msg interface{}, callback func(...interface{})) {
	data := msg.(*core.GetSyncData)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(GetSyncData, buf.Bytes())

	// 从分片的leader节点处获取
	addr := data.ServerAddr
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				"sendNewNodeTable2Client", -1, -1, -1, addr))
		}
		conns2Node.Add(addr, conn)
	}
	_, err = conn.Write(msg_bytes)
	if err != nil {
		log.Error("WriteError", "err", err)
	}
	log.Info(fmt.Sprintf("Msg Sent: %s syncMode: %v", GetSyncData, data.SyncType))

	// 等待回复

	// 首先读取消息长度的四个字节
	lengthBuf := make([]byte, 4)
	_, err = io.ReadFull(conn, lengthBuf)
	if err != nil {
		log.Error("ReadLengthError", "err", err)
	}
	// 解析这四个字节为int32来获取消息长度
	msgLength := int(binary.BigEndian.Uint32(lengthBuf))

	// 根据消息长度分配缓冲区
	msgBuf := make([]byte, msgLength)
	_, err = io.ReadFull(conn, msgBuf)
	if err != nil {
		log.Error("ReadMsgError", "err", err)
	}

	syncData := new(core.SyncData)
	decodeBuf := bytes.NewReader(msgBuf)
	decoder := gob.NewDecoder(decodeBuf)
	err = decoder.Decode(syncData)
	if err != nil {
		log.Error("Failed to decode", "err", err)
	}

	log.Info(fmt.Sprintf("Msg Response Received: %s syncMode: %s", GetSyncData, data.SyncType))

	callback(syncData)
}

// 清理多余的长连接
func clearConnection(msg interface{}) {
	nodeInfo := msg.(*core.NodeInfo)
	to_preserved := make(map[string]struct{})
	// 保留与本委员会所有节点的连接
	for _, addr := range cfg.ComNodeTable[nodeInfo.ComID] {
		to_preserved[addr] = struct{}{}
	}
	if utils.IsComLeader(nodeInfo.NodeID) {
		// 保留与其他委员会的leader节点的连接
		for i := 0; i < shardNum; i++ {
			to_preserved[cfg.ComNodeTable[uint32(i)][0]] = struct{}{}
		}
	}
	before_cnt := len(conns2Node.connections)
	for addr, conn := range conns2Node.connections {
		if _, ok := to_preserved[addr]; !ok {
			conn.Close()
			conns2Node.Remove(addr)
		}
	}
	log.Debug(fmt.Sprintf("remove unused tcp connections. before count: %d after count: %d", before_cnt, len(conns2Node.connections)))
}

func reportError(clientID uint32, msg interface{}) {
	data := msg.(*core.ErrReport)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(ReportError, buf.Bytes())

	addr := cfg.ClientTable[clientID]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				reportError, -1, -1, -1, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: reportError", "toClientID", clientID, "err", data.Err)
}

func reportAny(clientID uint32, msg interface{}) {
	data := msg.(string)
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(data)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}

	// 序列化后的消息
	msg_bytes := packMsg(ReportAny, buf.Bytes())

	addr := cfg.ClientTable[clientID]
	conn, ok := conns2Node.Get(addr)
	if !ok {
		conn, err = dial(addr)
		if err != nil {
			log.Error(fmt.Sprintf("Dial Error. caller: %s targetShardID: %d targetComID: %d targetNodeID: %d targetAddr: %s",
				ReportAny, -1, -1, -1, addr))
		}
		conns2Node.Add(addr, conn)
	}
	writer := bufio.NewWriter(conn)
	writer.Write(msg_bytes)
	writer.Flush()

	log.Info("Msg Sent: reportAny", "toClientID", clientID, "dataStr", data)
}

/* 用于分片、委员会、客户端、信标链传送消息 */
func (hub *GoodMessageHub) Send(msgType uint32, id uint32, msg interface{}, callback func(res ...interface{})) {
	switch msgType {
	case core.MsgTypeComGetHeightFromShard:
		height := comGetHeightFromShard(id, msg)
		callback(height)

	case core.MsgTypeShardSendGenesis:
		shardSendGenesis(msg)
	case core.MsgTypeBooterSendContract:
		booterSendContract(msg)

	case core.MsgTypeComGetStateFromShard:
		comGetStateFromShard(id, msg)
	case core.MsgTypeShardSendStateToCom:
		go shardSendStateToCom(id, msg)

	case core.MsgTypeClientInjectTX2Committee:
		go clientInjectTx2Com(id, msg)
	case core.MsgTypeSetInjectDone2Nodes:
		clientSetInjectDone2Nodes(id)

	case core.MsgTypeSendBlock2Shard:
		go comSendBlock2Shard(id, msg)

	case core.MsgTypeCommitteeReply2Client:
		go comSendReply2Client(id, msg)

	case core.MsgTypeLeaderInitMultiSign:
		comLeaderInitMultiSign(id, msg)
	case core.MsgTypeSendMultiSignReply:
		sendMultiSignReply(id, msg)

	case core.MsgTypeLeaderInitReconfig:
		leaderInitReconfig(id, msg)
	case core.MsgTypeSendReconfigResult2ComLeader:
		sendReconfigResult2Leader(id, msg)
	case core.MsgTypeSendReconfigResults2AllComLeaders:
		sendReconfigResults2AllLeaders(id, msg)
	case core.MsgTypeSendReconfigResults2ComNodes:
		sendReconfigResults2ComNodes(id, msg)
	case core.MsgTypeGetPoolTx:
		sendGetPoolTx(id, msg, callback)
	case core.MsgTypeGetSyncData:
		sendGetSyncData(id, msg, callback)
	case core.MsgTypeComSendNewAddrs:
		comSendNewAddrs(id, msg)
	case core.MsgTypeSendNewNodeTable2Client:
		sendNewNodeTable2Client(msg)

	////////////////////
	// 通过beaconChain模块中的ethclient与ethChain交互
	///////////////////
	case core.MsgTypeGetLatestBlockHashFromEthChain:
		getEthLatestBlock(callback)
	case core.MsgTypeGetBlockHashFromEthChain:
		getEthBlock(msg, callback)

	case core.MsgTypeComAddTb2TBChain:
		comAddTb2TBChain(id, msg)
	case core.MsgTypeTBChainPushTB2Client:
		tbChainPushBlock2Client(msg)
	case core.MsgTypeTBChainPushTB2Coms:
		tbChainPushBlock2Com(msg)

	////////////////////
	///// pbft  ////////
	////////////////////
	case core.MsgTypePbftPrePrepare:
		sendPbftMsg(id, msg, CPrePrepare)
	case core.MsgTypePbftPrepare:
		sendPbftMsg(id, msg, CPrepare)
	case core.MsgTypePbftCommit:
		sendPbftMsg(id, msg, CCommit)
	case core.MsgTypePbftReply:
		sendPbftMsg(id, msg, CReply)
	case core.MsgTypePbftRequestOldMessage:
		sendPbftMsg(id, msg, CRequestOldrequest)
	case core.MsgTypePbftSendOldMessage:
		sendPbftMsg(id, msg, CSendOldrequest)

	case core.MsgTypeNodeSendInfo2Leader:
		sendNodeInfo(id, msg)

	case core.MsgTypeClearConnection:
		clearConnection(msg)
	case core.MsgTypeReportError:
		reportError(id, msg)
	case core.MsgTypeReportAny:
		reportAny(id, msg)
	}

}
