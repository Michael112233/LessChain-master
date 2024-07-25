package node

import (
	"fmt"
	"go-w3chain/beaconChain"
	"go-w3chain/cfg"
	"go-w3chain/core"
	"go-w3chain/eth_chain"
	"go-w3chain/log"
	"net"
	"strconv"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type Booter struct {
	addrConfig  *core.NodeAddrConfig
	tbchain     *beaconChain.BeaconChain
	messageHub  core.MessageHub
	genesisLock sync.Mutex
}

func NewBooter() *Booter {
	host, port_str, err := net.SplitHostPort(cfg.BooterAddr)

	if err != nil {
		log.Error("invalid node address!", "addr", cfg.BooterAddr)
	}
	port, _ := strconv.ParseInt(port_str, 10, 32)
	cfg := &core.NodeAddrConfig{
		Name: "booter",
		Host: host,
		Port: int(port),
	}
	return &Booter{
		addrConfig: cfg,
	}
}

func (booter *Booter) SetTBchain(tbchain *beaconChain.BeaconChain) {
	booter.tbchain = tbchain
}

func (booter *Booter) SetMessageHub(hub core.MessageHub) {
	booter.messageHub = hub
}

func (booter *Booter) GetAddr() string {
	return fmt.Sprintf("%s:%d", booter.addrConfig.Host, booter.addrConfig.Port)
}

/* booter接收各个分片的创世区块信标和初始账户列表
收集齐后部署信标链上的合约，并返回退出booter监听线程的信号 */
func (booter *Booter) HandleShardSendGenesis(data *core.ShardSendGenesis) (exit bool) {
	booter.genesisLock.Lock()
	defer booter.genesisLock.Unlock()
	exit = false
	// 调用tbchain的方法
	booter.tbchain.SetAddrs(data.Addrs, nil, 0, data.Gtb.ShardID, 0)
	contractTB := &eth_chain.ContractTB{
		ShardID:    data.Gtb.ShardID,
		Height:     data.Gtb.Height,
		BlockHash:  data.Gtb.BlockHash,
		TxHash:     data.Gtb.TxHash,
		StatusHash: data.Gtb.StatusHash,
	}
	contractAddr, _ := booter.tbchain.AddEthChainGenesisTB(contractTB)
	if (contractAddr != common.Address{}) {
		exit = true
		msg := &core.BooterSendContract{
			Addr: contractAddr,
		}
		booter.messageHub.Send(core.MsgTypeBooterSendContract, 0, msg, nil)
	}
	return
}
