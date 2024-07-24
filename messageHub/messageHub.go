package messageHub

import (
	"encoding/gob"
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/client"
	"go-w3chain/core"
	"go-w3chain/eth_node"
	"go-w3chain/eth_shard"
	"go-w3chain/log"
	"go-w3chain/pbft"
	"net"
	"sync"
)

var (
	shard_ref *eth_shard.Shard
	//committee_ref *committee.Committee
	client_ref   *client.Client
	node_ref     *eth_node.EthNode
	pbftNode_ref *pbft.PbftConsensusNode
	booter_ref   *eth_node.Booter
	tbChain_ref  *beaconChain.BeaconChain

	shardNum      int
	shardSize     int
	comAllNodeNum int // 包括共识节点和非共识节点，该字段仅在初始化时有效
	clientNum     int
	conns2Node    *ConnectionsMap
	listenConn    net.Listener
)

func init() {
	conns2Node = NewConnectionsMap()
	gob.Register(core.Msg{})
	gob.Register(core.BooterSendContract{})

}

type GoodMessageHub struct {
	mid      int
	exitChan chan struct{}
}

func NewMessageHub() *GoodMessageHub {
	hub := &GoodMessageHub{
		mid:      1,
		exitChan: make(chan struct{}, 1),
	}
	return hub
}

func (hub *GoodMessageHub) Init(client *client.Client, node *eth_node.EthNode, booter *eth_node.Booter,
	tbChain *beaconChain.BeaconChain, _shardNum int, _shardSize, _shardAllNodeNum, _clientNum int, wg *sync.WaitGroup) {
	clientNum = _clientNum
	client_ref = client

	node_ref = node
	if node_ref != nil {
		shard_ref = node.GetShard().(*eth_shard.Shard)
		//committee_ref = node.GetCommittee().(*committee.Committee)
		pbftNode_ref = node.GetPbftNode()
	}

	booter_ref = booter
	tbChain_ref = tbChain
	shardNum = _shardNum
	shardSize = _shardSize
	comAllNodeNum = _shardAllNodeNum
	log.Info("messageHubInit", "shardNum", shardNum)

	//if committee_ref != nil {
	//	committee_ref.SetMessageHub(hub)
	//}
	if shard_ref != nil {
		shard_ref.SetMessageHub(hub)
	}
	if pbftNode_ref != nil {
		pbftNode_ref.SetMessageHub(hub)
	}
	if client_ref != nil {
		client_ref.SetMessageHub(hub)
		wg.Add(1)
		go listen(client.GetAddr(), wg)
	}
	// 目前所有角色都需要连接tbchain
	if tbChain_ref == nil {
		log.Error("tbchain instance is nil")
	}
	tbChain_ref.SetMessageHub(hub)

	if node_ref != nil {
		node_ref.SetMessageHub(hub)
		wg.Add(1)
		go listen(node_ref.GetAddr(), wg)
	}
	if booter_ref != nil {
		booter_ref.SetMessageHub(hub)
		wg.Add(1)
		go listen(booter.GetAddr(), wg)
	}

}

func (hub GoodMessageHub) Close() {
	// 关闭所有tcp连接，防止资源泄露
	log.Debug(fmt.Sprintf("messageHub closing..."))
	for _, conn := range conns2Node.connections {
		conn.Close()
	}
	listenConn.Close()
	log.Debug(fmt.Sprintf("messageHub is close."))
}
