package beaconChain

import (
	"go-w3chain/core"
	"go-w3chain/eth_chain"
	"go-w3chain/log"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	genesisTBs map[uint32]*eth_chain.ContractTB = make(map[uint32]*eth_chain.ContractTB)
	// 每个委员会指配一个client，client与以太坊私链交互，获取gasPrcie、nonce等链与账户信息
	client *ethclient.Client
	// 缓存websocket返回的事件（代表确认信标），最多缓存100个已确认的信标
	eventChannel chan *eth_chain.Event = make(chan *eth_chain.Event, 100)
)

func (tbChain *BeaconChain) HandleBooterSendContract(data *core.BooterSendContract) {
	tbChain.contractAddr = data.Addr
	contractABI, err := abi.JSON(strings.NewReader(eth_chain.MyContractABI()))
	if err != nil {
		log.Error("get contracy abi fail", "err", err)
	}
	tbChain.contractAbi = &contractABI

	go eth_chain.SubscribeEvents(tbChain.cfg.Port, tbChain.contractAddr, eventChannel)
}

func (tbChain *BeaconChain) GetEthChainLatestBlockHash() (common.Hash, uint64) {
	client := tbChain.getEthClient()
	return eth_chain.GetLatestBlockHash(client)
}

func (tbChain *BeaconChain) GetEthChainBlockHash(height uint64) (common.Hash, uint64) {
	client := tbChain.getEthClient()
	return eth_chain.GetBlockHash(client, height)
}

// func (tbChain *BeaconChain) GetEthChainBlockHash(shardID uint32, height uint64) common.Hash {
// 	client := tbChain.getEthClient()
// 	return eth_chain.GetBlockHash(client, height)
// }

func (tbChain *BeaconChain) AddTimeBeacon2EthChain(signedtb *core.SignedTB, nodeID uint32) {
	tb := signedtb.TimeBeacon
	// 转化为合约中的结构（目前两结构的成员变量是完全相同的）
	contractTB := &eth_chain.ContractTB{
		ShardID:    tb.ShardID,
		Height:     tb.Height,
		BlockHash:  tb.BlockHash,
		TxHash:     tb.TxHash,
		StatusHash: tb.StatusHash,
	}
	if tb.Height == 0 {
		tbChain.AddEthChainGenesisTB(contractTB)
	} else {
		client := tbChain.getEthClient()
		err := eth_chain.AddTB(client, tbChain.contractAddr,
			tbChain.contractAbi, tbChain.mode, contractTB, signedtb.Sigs, signedtb.Vrfs,
			signedtb.SeedHeight, signedtb.Signers, tbChain.cfg.ChainId, nodeID)
		if err != nil {
			log.Error("eth_chain.AddTB err", "err", err)
		}
	}
	log.Debug("AddTbTXSent", "info", signedtb)
}

func (tbChain *BeaconChain) AdjustEthChainRecordedAddrs(addrs []common.Address, vrfs [][]byte, seedHeight uint64, comID uint32, nodeID uint32) {
	client := tbChain.getEthClient()
	err := eth_chain.AdjustRecordedAddrs(client, tbChain.contractAddr,
		tbChain.contractAbi, tbChain.mode, comID, addrs, vrfs, seedHeight, tbChain.cfg.ChainId, nodeID)
	if err != nil {
		log.Error("eth_chain.AdjustRecordedAddrs err", "err", err)
	}
	log.Debug("AdjustAddrsTXSent", "shardID", comID, "seedHeight", seedHeight)
}

func (tbChain *BeaconChain) generateEthChainBlock() *TBBlock {
	to_pack := false
	start_eth_height := uint64(0)
	if len(tbChain.geth_tbs_new) != 0 {
		for h, _ := range tbChain.geth_tbs_new {
			if start_eth_height == 0 {
				start_eth_height = h
			} else if h < start_eth_height {
				start_eth_height = h
			}
		}
	}
	for {
		if len(eventChannel) == 0 {
			break
		}

		event := <-eventChannel
		// tbChain.height = uint64(utils.Max(int(tbChain.height), ))
		if start_eth_height == 0 {
			start_eth_height = event.Eth_height
		}
		tb := &core.TimeBeacon{
			ShardID: event.ShardID,
			Height:  event.Height,
		}
		if _, ok := tbChain.geth_tbs_new[event.Eth_height]; !ok {
			tbChain.geth_tbs_new[event.Eth_height] = make(map[uint32][]*core.TimeBeacon)
		}
		tbChain.geth_tbs_new[event.Eth_height][tb.ShardID] = append(tbChain.geth_tbs_new[event.Eth_height][tb.ShardID], tb)

		if event.Eth_height > start_eth_height {
			to_pack = true
		}
	}

	// // 在当前设置中，当客户端注入交易完成时，会发送消息停止所有节点。
	// // 节点退出时，交易未被全部处理，因而一定有未被打包的信标。
	// // 因此，如果在运行时遇到len(tbChain.geth_tbs_new) == 0 的情况，可以肯定是因为没来得及获取到新确认的信标，直接返回nil即可
	// // 结合以上代码，每个节点最多落后其他节点一个信标链高度，仍可以通过信标链与其他节点保持“同步”
	// // 但这会导致该节点交易池对交易是否超时的判断有误吗？—— 不会，因为交易池判断依据是本分片的高度而非信标链的高度
	// if len(tbChain.geth_tbs_new) == 0 {
	// 	return nil
	// }
	if !to_pack { // 还没遇到下一个区块的信标，不能确定本区块信标全部接收到，暂不出块
		return nil
	}

	tbs_new := tbChain.geth_tbs_new[start_eth_height]

	now := time.Now().Unix()

	confirmTBs := make([][]*ConfirmedTB, tbChain.shardNum)
	for shardID, tbs := range tbs_new {
		for _, tb := range tbs {
			confirmedTB := &ConfirmedTB{
				TimeBeacon:    *tb,
				ConfirmTime:   uint64(now),
				ConfirmHeight: tbChain.height,
			}
			confirmTBs[shardID] = append(confirmTBs[shardID], confirmedTB)
		}
		tbChain.tbs[int(shardID)] = append(tbChain.tbs[int(shardID)], confirmTBs[shardID]...)
	}

	block := &TBBlock{
		Tbs:    confirmTBs,
		Time:   uint64(now),
		Height: start_eth_height,
	}

	delete(tbChain.geth_tbs_new, start_eth_height)

	log.Debug("TBchainGenerateBlock", "info", block)
	return block
}

func (tbChain *BeaconChain) AddEthChainGenesisTB(tb *eth_chain.ContractTB) (common.Address, *abi.ABI) {
	genesisTBs[tb.ShardID] = tb
	if len(genesisTBs) == tbChain.shardNum {
		// 转化为数组形式
		tbs := make([]eth_chain.ContractTB, tbChain.shardNum)
		for shardID, tb := range genesisTBs {
			tbs[shardID] = *tb
		}

		tbChain.deployContract(tbs)

		// go eth_chain.SubscribeEvents(tbChain.cfg.Port, tbChain.contractAddr, eventChannel)
		return tbChain.contractAddr, tbChain.contractAbi
	}
	return common.Address{}, nil
}

func (tbChain *BeaconChain) deployContract(genesisTBs []eth_chain.ContractTB) {
	// 创建合约，各分片创世区块作为构造函数的参数
	client := tbChain.getEthClient()

	tbChain.contractAddr, tbChain.contractAbi, _, _ = eth_chain.DeployContract(client,
		tbChain.mode, genesisTBs,
		uint32(tbChain.cfg.MultiSignRequiredNum),
		uint32(tbChain.shardNum),
		tbChain.addrs,
		tbChain.cfg.ChainId)
}

func (tbChain *BeaconChain) getEthClient() *ethclient.Client {
	if client == nil {
		var err error
		client, err = eth_chain.Connect(tbChain.cfg.Port)
		if err != nil {
			log.Error("could not connect to eth chain!", "err", err)
			panic(err)
		}
	}

	return client
}
