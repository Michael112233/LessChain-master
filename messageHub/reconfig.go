package messageHub

// import (
// 	"go-w3chain/beaconChain"
// 	"go-w3chain/log"
// 	"go-w3chain/node"
// 	"go-w3chain/utils"
// 	"time"
// )

// var randSeedHeight uint64

// func toReconfig(seedHeight uint64) {
// 	if isNotToReconfig == len(committees_ref) {
// 		randSeedHeight = seedHeight
// 	} else if randSeedHeight != seedHeight {
// 		log.Error("reconfig error, got different rand seed!")
// 	}
// 	isNotToReconfig -= 1
// 	if isNotToReconfig == 0 {
// 		// 这里需要开一个新线程，不要与最后一个调用此函数的委员会共用一个线程
// 		go reconfig(randSeedHeight)
// 		isNotToReconfig = len(committees_ref)
// 	}
// }

// /* 打乱allnodes_id，重新设定委员会中的节点 */
// func reconfig(seedHeight uint64) {
// 	seed := beaconChain.GetEthChainBlockHash(0, seedHeight)
// 	nodes_4_shards := make(map[uint32][]*node.Node)
// 	for _, node := range nodes_ref {
// 		vrfValue := node.GetAccount().GenerateVRFOutput(seed[:]).RandomValue
// 		node.VrfValue = vrfValue
// 		newShardID := utils.VrfValue2Shard(vrfValue, uint32(len(shards_ref)))
// 		nodes_4_shards[newShardID] = append(nodes_4_shards[newShardID], node)
// 	}

// 	time.Sleep(time.Duration(reconfigTime) * time.Second)

// 	// 更新各委员会的节点
// 	for _, com := range committees_ref {
// 		go com.SetReconfigRes(nodes_4_shards[uint32(com.GetCommitteeID())])
// 	}
// }
