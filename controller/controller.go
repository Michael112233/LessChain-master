package controller

// GO111MODULE=on go run main.go

import (
	"fmt"
	beaconchain "go-w3chain/beaconChain"
	"go-w3chain/cfg"
	"go-w3chain/client"
	"go-w3chain/core"
	"go-w3chain/data"
	"go-w3chain/eth_node"
	"go-w3chain/eth_shard"
	"go-w3chain/log"
	"go-w3chain/messageHub"
	"go-w3chain/utils"
	"os"
	"sync"

	// "go-w3chain/miner"
	"go-w3chain/result"
	"time"
)

func runClient(allCfg *cfg.Cfg) {
	cid := allCfg.ClientId
	addr := cfg.ClientTable[uint32(cid)]

	client := client.NewClient(addr, cid, allCfg.Height2Rollback, allCfg.ShardNum, allCfg.ExitMode)
	log.Info("NewClient", "Info", client)

	// 加载交易数据
	data.LoadETHData(allCfg.DatasetDir, allCfg.MaxTxNum)
	data.SetTxShardId(allCfg.ShardNum)

	// 注入交易到客户端
	data.SetTX2ClientTable(allCfg.ClientNum)
	data.InjectTX2Client(client)

	client.Print()

	// 初始化信标链接口
	beaconChainConfig := &core.BeaconChainConfig{
		Mode:                 allCfg.BeaconChainMode,
		ChainId:              allCfg.BeaconChainID,
		Port:                 allCfg.BeaconChainPort,
		BlockInterval:        allCfg.TbchainBlockIntervalSecs,
		Height2Confirm:       uint64(allCfg.Height2Confirm),
		MultiSignRequiredNum: allCfg.MultiSignRequiredNum,
	}
	tbChain = beaconchain.NewTBChain(beaconChainConfig, allCfg.ShardNum)

	var wg sync.WaitGroup

	/* 创建消息中心(用于客户端和信标链的交互等) */
	messageHub := messageHub.NewMessageHub()

	/* 设置各个分片、委员会和客户端、信标链的通信渠道 */
	messageHub.Init(client, nil, nil, tbChain, allCfg.ShardNum, allCfg.ShardSize, allCfg.ComAllNodeNum, allCfg.ClientNum, &wg)

	startClient(client, allCfg.InjectSpeed, allCfg.RecommitIntervalSecs)
	toStopClient(client, allCfg.RecommitIntervalSecs, allCfg.LogProgressInterval,
		allCfg.IsLogProgress, allCfg.ExitMode)

	stopTBChain()
	messageHub.Close()

	wg.Wait()

	/* 打印交易执行结果 */
	result.PrintTXReceipt()
	thrput, avlatency, rollbackRate, overloads := result.GetThroughtPutAndLatencyV2()
	log.Info("GetThroughtPutAndLatency", "thrput", thrput, "avlatency", avlatency, "rollbackRate", rollbackRate, "overloads", overloads)
}

// 运行以太坊客户端
func runEthClient(allCfg *cfg.Cfg) {
	cid := allCfg.ClientId
	addr := cfg.ClientTable[uint32(cid)]

	client := client.NewClient(addr, cid, allCfg.Height2Rollback, allCfg.ShardNum, allCfg.ExitMode)
	log.Info("NewEthClient", "Info", client)

	// 加载交易数据，分配交易到每个节点中
	data.LoadETHData(allCfg.DatasetDir, allCfg.MaxTxNum)
	data.SetTxShardId(allCfg.ShardSize)

	// 注入交易到客户端
	data.SetTX2ClientTable(allCfg.ClientNum)
	data.InjectTX2Client(client)

	client.Print()

	// 初始化信标链接口
	beaconChainConfig := &core.BeaconChainConfig{
		Mode:                 allCfg.BeaconChainMode,
		ChainId:              allCfg.BeaconChainID,
		Port:                 allCfg.BeaconChainPort,
		BlockInterval:        allCfg.TbchainBlockIntervalSecs,
		Height2Confirm:       uint64(allCfg.Height2Confirm),
		MultiSignRequiredNum: allCfg.MultiSignRequiredNum,
	}
	tbChain = beaconchain.NewTBChain(beaconChainConfig, allCfg.ShardNum)

	var wg sync.WaitGroup

	/* 创建消息中心(用于客户端和信标链的交互等) */
	messageHub := messageHub.NewMessageHub()

	/* 设置各个分片、委员会和客户端、信标链的通信渠道 */
	messageHub.Init(client, nil, nil, tbChain, 1, allCfg.ShardSize, 0, allCfg.ClientNum, &wg)

	startClient(client, allCfg.InjectSpeed, allCfg.RecommitIntervalSecs)
	toStopClient(client, allCfg.RecommitIntervalSecs, allCfg.LogProgressInterval,
		allCfg.IsLogProgress, allCfg.ExitMode)

	stopTBChain()
	messageHub.Close()

	wg.Wait()

	/* 打印交易执行结果 */
	result.PrintTXReceipt()
	thrput, avlatency, rollbackRate, overloads := result.GetThroughtPutAndLatencyV2()
	log.Info("GetThroughtPutAndLatency", "thrput", thrput, "avlatency", avlatency, "rollbackRate", rollbackRate, "overloads", overloads)
}

func runNode(allCfg *cfg.Cfg) {
	shardId := allCfg.ShardId
	nodeId := allCfg.NodeId
	comId := shardId

	// 节点数据存储目录
	dataDir := cfg.DefaultDataDir()
	// for i := 0; i < len(cfg.NodeTable); i++ {
	// 	for j := 0; j < len(cfg.NodeTable[uint32(i)]); j++ {
	// 		log.Info(fmt.Sprintf("(%d, %d), %s", i, j, cfg.NodeTable[uint32(i)][uint32(j)]))
	// 	}
	// }
	// 创建节点
	node := eth_node.NewNode(dataDir, allCfg.ShardNum, shardId, comId, nodeId, allCfg.ShardSize, allCfg.ReconfigMode)
	defer closeEthNode(node)

	// TODO：建立分片内连接

	// 创建本节点对应的分片实例，用于执行分片的操作
	shard := eth_shard.ExecutionInitialize(uint32(shardId), node, allCfg.Height2Reconfig)
	node.SetShard(shard)

	// 初始化分片中的账户状态
	if utils.IsShardLeader(node.NodeInfo.NodeID) { // 目前不考虑分片重组和节点失败，只有分片leader需要设置初始状态
		data.LoadETHData(allCfg.DatasetDir, allCfg.MaxTxNum)
		data.SetTxShardId(allCfg.ShardNum)
		data.SetShardInitialAccountState(shard)
	}

	// 创建本节点对应的委员会实例
	committeeConfig := &core.CommitteeConfig{
		RecommitTime:         time.Duration(allCfg.RecommitIntervalSecs) * time.Second,
		MaxBlockSize:         allCfg.MaxBlockTXSize,
		InjectSpeed:          allCfg.InjectSpeed,
		Height2Reconfig:      allCfg.Height2Reconfig,
		MultiSignRequiredNum: allCfg.MultiSignRequiredNum,
	}
	com := shard.ConsensusInitialize(uint32(allCfg.ShardId), allCfg.ClientNum, node, committeeConfig)
	node.SetShard(com)

	// 初始化信标链接口
	beaconChainConfig := &core.BeaconChainConfig{
		Mode:                 allCfg.BeaconChainMode,
		ChainId:              allCfg.BeaconChainID,
		Port:                 allCfg.BeaconChainPort,
		BlockInterval:        allCfg.TbchainBlockIntervalSecs,
		Height2Confirm:       uint64(allCfg.Height2Confirm),
		MultiSignRequiredNum: allCfg.MultiSignRequiredNum,
	}
	tbChain = beaconchain.NewTBChain(beaconChainConfig, allCfg.ShardNum)

	var wg sync.WaitGroup

	/* 创建消息中心(用于委员会和信标链的交互等) */
	messageHub := messageHub.NewMessageHub()
	/* 设置各个分片、委员会和客户端、信标链的通信渠道 */
	messageHub.Init(nil, node, nil, tbChain, allCfg.ShardNum, allCfg.ShardSize, allCfg.ComAllNodeNum, allCfg.ClientNum, &wg)

	// 启动节点
	startNode(node)

	/* 循环打印进度；判断各客户端和委员会能否停止, 若能则停止 */
	toStopCommittee(node, allCfg.RecommitIntervalSecs, allCfg.LogProgressInterval,
		allCfg.IsLogProgress, allCfg.ExitMode)

	stopTBChain()
	messageHub.Close()
	wg.Wait()

}

/* booterNode 的作用是接收各分片的创世区块信标及初始地址，部署合约并返回合约地址 */
func runBooterNode(allCfg *cfg.Cfg) {
	// 初始化信标链接口
	beaconChainConfig := &core.BeaconChainConfig{
		Mode:                 allCfg.BeaconChainMode,
		ChainId:              allCfg.BeaconChainID,
		Port:                 allCfg.BeaconChainPort,
		BlockInterval:        allCfg.TbchainBlockIntervalSecs,
		Height2Confirm:       uint64(allCfg.Height2Confirm),
		MultiSignRequiredNum: allCfg.MultiSignRequiredNum,
	}
	tbChain = beaconchain.NewTBChain(beaconChainConfig, allCfg.ShardNum)
	defer stopTBChain()

	booter := eth_node.NewBooter()
	booter.SetTBchain(tbChain)

	var wg sync.WaitGroup

	/* 创建消息中心(用于委员会和信标链的交互等) */
	messageHub := messageHub.NewMessageHub()
	/* 设置各个分片、委员会和客户端、信标链的通信渠道 */
	messageHub.Init(nil, nil, booter, tbChain, allCfg.ShardNum, allCfg.ShardSize, allCfg.ComAllNodeNum, allCfg.ClientNum, &wg)
	defer messageHub.Close()

	wg.Wait()

}

func Main(cfgfilename string, role string, shardNum, shardID, nodeID int32) {
	cfg := cfg.DefaultCfg(cfgfilename)
	// 命令行参数覆写配置文件的参数
	cfg.Role = role
	cfg.ShardNum = int(shardNum)
	cfg.ShardId = int(shardID)
	cfg.NodeId = int(nodeID)

	/* 设置日志存储路径 */
	// 检查logs文件夹是否存在
	if _, err := os.Stat("logs"); os.IsNotExist(err) {
		// 如果logs文件夹不存在，则创建它
		err := os.Mkdir("logs", 0755)
		if err != nil {
			fmt.Println("Error creating logs directory:", err)
		}
	}
	if cfg.LogFile == "" {
		if cfg.Role == "node" {
			cfg.LogFile = fmt.Sprintf("logs/S%dN%d.log", cfg.ShardId, cfg.NodeId)
		} else {
			cfg.LogFile = fmt.Sprintf("logs/%s.log", cfg.Role)
		}
	}
	fmt.Println("log file:", cfg.LogFile)
	log.SetLogInfo(log.Lvl(cfg.LogLevel), cfg.LogFile)

	/* 设置 是否使用 progressbar */
	result.SetIsProgressBar(cfg.IsProgressBar)
	result.SetcsvFilename(cfg.LogFile)

	/* 打印超参数 */
	log.Info(fmt.Sprintf("%#v", cfg))

	switch role {
	case "client":
		runEthClient(cfg)
	case "node":
		runNode(cfg)
	case "booter":
		runBooterNode(cfg)
	default:
		log.Error("unknown roleType in Ethereum", "type", role)
	}

	/* 结束 */
	log.Info(" Run finished! Bye~")
	fmt.Println(" Run finished! Bye~")
}
