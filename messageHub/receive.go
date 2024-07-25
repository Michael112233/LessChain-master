package messageHub

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/result"
	"go-w3chain/utils"
	"io"
	"net"
	"sync"
)

func listen(addr string, wg *sync.WaitGroup) {
	defer wg.Done()
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Error("Error setting up listener", "err", err)
	}
	log.Info(fmt.Sprintf("start listening on %s", addr))
	listenConn = ln
	defer ln.Close()

	for {
		// // 超过时间限制没有收到新的连接则退出
		// ln.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Second))
		conn, err := ln.Accept()
		if err != nil {
			log.Debug("Error accepting connection", "err", err)
			return
		}
		go handleConnection(conn, ln)

	}
}

func unpackMsg(packedMsg []byte) *core.Msg {
	var networkBuf bytes.Buffer
	networkBuf.Write(packedMsg)
	msgDec := gob.NewDecoder(&networkBuf)

	var msg core.Msg
	err := msgDec.Decode(&msg)
	if err != nil {
		log.Error("unpackMsgErr", "err", err, "msgBytes", packedMsg)
	}

	return &msg
}

func handleClientSendTx(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data []*core.Transaction
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err)
	}

	log.Info("Msg Received: ClientSendTx", "tx count", len(data))
	committee_ref.HandleClientSendtx(data)
}

func handleClientSetInjectDone(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data *core.ClientSetInjectDone
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err)
	}

	log.Info("Msg Received: ClientSetInjectDone", "clientID", data.Cid)
	committee_ref.SetInjectTXDone(data.Cid)
}

func handleComGetHeight(dataBytes []byte, conn net.Conn) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ComGetHeight
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ComGetHeight", "data", data)

	// 直接通过这个连接回复请求方
	height := shard_ref.HandleComGetHeight(&data)
	var buf1 bytes.Buffer
	encoder := gob.NewEncoder(&buf1)
	err = encoder.Encode(height)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}
	msgBytes := buf1.Bytes()

	// 前缀加上长度，防止粘包
	networkBuf := make([]byte, 4+len(msgBytes))
	binary.BigEndian.PutUint32(networkBuf[:4], uint32(len(msgBytes)))
	copy(networkBuf[4:], msgBytes)
	// 发送回复
	_, err = conn.Write(networkBuf)
	if err != nil {
		log.Error("WriteError", "err", err)
	}

}

func handleComGetState(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ComGetState
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ComGetState", "addr count", len(data.AddrList))
	shard_ref.HandleComGetState(&data)
}

func handleShardSendState(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ShardSendState
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ShardSendState", "addr count", len(data.AccountData))
	committee_ref.HandleShardSendState(&data)
}

func handleBooterSendContract(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.BooterSendContract
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: BooterSendContract", "data", data)

	if node_ref != nil {
		// 注意是节点处理而不是分片或委员会处理
		node_ref.HandleBooterSendContract(&data)
	}
	if client_ref != nil {
		client_ref.HandleBooterSendContract(&data)
	}
	tbChain_ref.HandleBooterSendContract(&data)

}

func handleComSendBlock(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ComSendBlock
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ComSendBlock", "tx count", len(data.Block.Transactions))

	shard_ref.HandleComSendBlock(&data)
}

func handleLeaderInitMultiSign(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ComLeaderInitMultiSign
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: LeaderInitMultiSign")

	committee_ref.HandleMultiSignRequest(&data)
}

func handleMultiSignReply(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.MultiSignReply
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: MultiSignReply")

	committee_ref.HandleMultiSignReply(&data)
}

//////////////////////////////////////////////////
////  client  ////
//////////////////////////////////////////////////

func handleComSendTxReceipt(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data []*result.TXReceipt
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ComSendTxReceipt", "tx count", len(data), "comID", data[0].ShardID, "blockHeight", data[0].BlockHeight)

	client_ref.HandleComSendTxReceipt(data)
}

//////////////////////////////////////////////////
////  booter  ////
//////////////////////////////////////////////////

func handleShardSendGenesis(dataBytes []byte) (exit bool) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ShardSendGenesis
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: ShardSendGenesis", "shardId", data.ShardID)

	exit = booter_ref.HandleShardSendGenesis(&data)
	return exit
}

//////////////////////////////////////////////////
////  pbft module  ////
//////////////////////////////////////////////////
func handlePbftMsg(dataBytes []byte, dataType string) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	switch dataType {
	case CPrePrepare:
		var data core.PrePrepare
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v seqID: %d", dataType, node_ref.NodeInfo.ComID, data.SeqID))
		go pbftNode_ref.HandlePrePrepare(&data)
	case CPrepare:
		var data core.Prepare
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v from nodeID: %v", dataType, node_ref.NodeInfo.ComID, data.SenderInfo.NodeID))
		pbftNode_ref.HandlePrepare(&data)
	case CCommit:
		var data core.Commit
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v from nodeID: %v", dataType, node_ref.NodeInfo.ComID, data.SenderInfo.NodeID))
		pbftNode_ref.HandleCommit(&data)
	case CReply:
		var data core.Reply
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v from nodeID: %v", dataType, node_ref.NodeInfo.ComID, data.SenderInfo.NodeID))
		pbftNode_ref.HandleReply(&data)
	case CRequestOldrequest:
		var data core.RequestOldMessage
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v from nodeID: %v", dataType, node_ref.NodeInfo.ComID, data.SenderInfo.NodeID))
		pbftNode_ref.HandleRequestOldSeq(&data)
	case CSendOldrequest:
		var data core.SendOldMessage
		err := dataDec.Decode(&data)
		if err != nil {
			log.Error("decodeDataErr", "err", err, "dataBytes", data)
		}
		log.Info(fmt.Sprintf("Msg Received: %s ComID: %v", dataType, node_ref.NodeInfo.ComID))
		go pbftNode_ref.HandleSendOldSeq(&data)
	}

}

func handleNodeSendInfo(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.NodeSendInfo
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info("Msg Received: NodeSendInfo")

	node_ref.HandleNodeSendInfo(&data)
}

type ReceiveReconfigMsgs struct {
}

func handleLeaderInitReconfig(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.InitReconfig
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s comID: %d", LeaderInitReconfig, data.ComID))

	node_ref.HandleLeaderInitReconfig(&data)
}

func handleSendReconfigResult2ComLeader(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ReconfigResult
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s comID: %d nodeID: %d", SendReconfigResult2ComLeader, data.Belong_ComID, data.OldNodeInfo.NodeID))

	node_ref.HandleSendReconfigResult2ComLeader(&data)
}

func handleSendReconfigResults2AllComLeaders(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ComReconfigResults
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s from_comID: %d", SendReconfigResults2AllComLeaders, data.ComID))

	node_ref.HandleSendReconfigResults2AllComLeaders(&data)
}

func handleSendReconfigResults2ComNodes(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data map[uint32]*core.ComReconfigResults
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s", SendReconfigResults2ComNodes))

	node_ref.HandleSendReconfigResults2ComNodes(&data)
}

func handleGetPoolTx(dataBytes []byte, conn net.Conn) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.GetPoolTx
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s", GetPoolTx))

	// 直接通过这个连接回复请求方
	poolTx := committee_ref.HandleGetPoolTx(&data)
	var buf1 bytes.Buffer
	encoder := gob.NewEncoder(&buf1)
	err = encoder.Encode(poolTx)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}
	msgBytes := buf1.Bytes()

	// 前缀加上长度，防止粘包
	networkBuf := make([]byte, 4+len(msgBytes))
	binary.BigEndian.PutUint32(networkBuf[:4], uint32(len(msgBytes)))
	copy(networkBuf[4:], msgBytes)
	// 发送回复
	_, err = conn.Write(networkBuf)
	if err != nil {
		log.Error("WriteError", "err", err)
	}

	log.Info(fmt.Sprintf("Msg response Sent: %s len(Pending): %d len(PendingRollback): %d", GetPoolTx, len(poolTx.Pending), len(poolTx.PendingRollback)))
	log.Info(fmt.Sprintf("Analyse txPool size... sizeof txPool: %d", len(utils.EncodeAny(poolTx))))

}

func handleGetSyncData(dataBytes []byte, conn net.Conn) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.GetSyncData
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s syncMode: %s", GetSyncData, data.SyncType))

	// 直接通过这个连接回复请求方
	syncData := shard_ref.HandleGetSyncData(&data)
	var buf1 bytes.Buffer
	encoder := gob.NewEncoder(&buf1)
	err = encoder.Encode(syncData)
	if err != nil {
		log.Error("gobEncodeErr", "err", err, "data", data)
	}
	msgBytes := buf1.Bytes()

	// 前缀加上长度，防止粘包
	networkBuf := make([]byte, 4+len(msgBytes))
	binary.BigEndian.PutUint32(networkBuf[:4], uint32(len(msgBytes)))
	copy(networkBuf[4:], msgBytes)
	// 发送回复
	_, err = conn.Write(networkBuf)
	if err != nil {
		log.Error("WriteError", "err", err)
	}
	log.Info(fmt.Sprintf("Msg response Sent: %s syncMode: %s len(states): %d len(blocks): %d", GetSyncData, data.SyncType, len(syncData.States), len(syncData.Blocks)))
	log.Info(fmt.Sprintf("Analyse SyncData size... sizeof State(bytes): %d  sizeof Blocks(bytes): %d", len(utils.EncodeAny(syncData.States)), len(utils.EncodeAny(syncData.Blocks))))
}

func handleSendNewNodeTable2Client(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data map[uint32]map[uint32]string
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s", SendNewNodeTable2Client))

	cfg.ComNodeTable = data
}

func handleReportErr(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data core.ErrReport
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s", ReportError))

	log.Error("got err report.", "fromAddr", data.NodeAddr, "errMsg", data.Err)
}

func handleReportAny(dataBytes []byte) {
	var buf bytes.Buffer
	buf.Write(dataBytes)
	dataDec := gob.NewDecoder(&buf)

	var data string
	err := dataDec.Decode(&data)
	if err != nil {
		log.Error("decodeDataErr", "err", err, "dataBytes", data)
	}

	log.Info(fmt.Sprintf("Msg Received: %s", ReportAny))

	log.Info("got msg report.", "msg", data)
}

func handleConnection(conn net.Conn, ln net.Listener) {
	defer conn.Close()

	// reader := bufio.NewReader(conn)

	for {
		// 先接收消息长度，再读消息
		lenBuf := make([]byte, 4)
		_, err := io.ReadFull(conn, lenBuf)
		if err != nil {
			if err.Error() == "EOF" {
				// 发送端主动关闭连接
				return
			}
			// log.Error("Error reading from connection", "err", err)
			// 当一台机子上跑比较多节点时，总会出现突然的"Error reading from connection"
			// err="read tcp 127.0.0.1:19000->127.0.0.1:64134: wsarecv: An existing connection was forcibly closed by the remote host."
			// 的错误。这里尝试性地在遇到该错误时直接return，不终止程序。
			log.Debug("Error reading from connection", "err", err)
			return
		}
		length := int(binary.BigEndian.Uint32(lenBuf))
		packedMsg := make([]byte, length)
		_, err = io.ReadFull(conn, packedMsg)
		if err != nil {
			log.Error("Error reading from connection", "err", err)
		}

		msg := unpackMsg(packedMsg)
		switch msg.MsgType {
		// booter
		case ShardSendGenesis:
			exit := handleShardSendGenesis(msg.Data)
			if exit {
				ln.Close()
				return
			}
		case BooterSendContract:
			handleBooterSendContract(msg.Data)

		case ComGetHeight:
			handleComGetHeight(msg.Data, conn)

		case ComGetState:
			go handleComGetState(msg.Data)
		case ShardSendState:
			go handleShardSendState(msg.Data)

		case ClientSendTx:
			go handleClientSendTx(msg.Data)
		case ClientSetInjectDone:
			handleClientSetInjectDone(msg.Data)
		case ComSendTxReceipt:
			go handleComSendTxReceipt(msg.Data)

		case ComSendBlock:
			go handleComSendBlock(msg.Data)

		case LeaderInitMultiSign:
			handleLeaderInitMultiSign(msg.Data)
		case MultiSignReply:
			handleMultiSignReply(msg.Data)

		case LeaderInitReconfig:
			handleLeaderInitReconfig(msg.Data)
		case SendReconfigResult2ComLeader:
			handleSendReconfigResult2ComLeader(msg.Data)
		case SendReconfigResults2AllComLeaders:
			handleSendReconfigResults2AllComLeaders(msg.Data)
		case SendReconfigResults2ComNodes:
			handleSendReconfigResults2ComNodes(msg.Data)
		case GetPoolTx:
			handleGetPoolTx(msg.Data, conn)
		case GetSyncData:
			handleGetSyncData(msg.Data, conn)
		case SendNewNodeTable2Client:
			handleSendNewNodeTable2Client(msg.Data)
		// case SendTxPool:
		// 	handleSendTxPool(msg.Data)

		/////////////////////////
		//// pbft /////
		/////////////////////////
		case CPrePrepare, CPrepare, CCommit, CReply, CRequestOldrequest, CSendOldrequest:
			go handlePbftMsg(msg.Data, msg.MsgType)

		case NodeSendInfo:
			handleNodeSendInfo(msg.Data)

		case ReportError:
			handleReportErr(msg.Data)
		case ReportAny:
			handleReportAny(msg.Data)

		default:
			log.Error("Unknown message type received", "msgType", msg.MsgType)
		}
	}
}
