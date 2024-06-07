package controller

import (
	"go-w3chain/beaconChain"
	"go-w3chain/client"
	"go-w3chain/log"
	"go-w3chain/node"
	"go-w3chain/result"
	"math"
	"time"
)

var tbChain *beaconChain.BeaconChain

func startNode(node *node.Node) {
	node.Start()
}

func startClient(c *client.Client, injectSpeed int, recommitIntervalSecs int) {
	c.Start(injectSpeed)
}

/**
 * 判断委员会能否停止, 若能则停止
 * 循环打印交易总执行进度
 */
func toStopCommittee(node *node.Node, recommitIntervalSecs,
	logProgressInterval int, isLogProgress bool, exitMode int) {
	log.Info("Monitor txpools and try to stop shards")
	sleepSecs := int(math.Ceil(float64(recommitIntervalSecs) / 2))
	iterNum := int(math.Ceil(float64(logProgressInterval) / float64(sleepSecs)))
	iter := 0
	if isLogProgress {
		log.Info("Set logProgressInterval(secs)", "iterNum*sleepSecs", iterNum*sleepSecs)
	} else {
		log.Info("Set log progress false")
	}

	for {
		canStop := false
		if exitMode == 0 {
			canStop = node.GetCommittee().CanStopV1()
		} else if exitMode == 1 {
			canStop = node.GetCommittee().CanStopV2()
		}

		if canStop {
			node.GetCommittee().Close()
			node.GetShard().Close()
			break
		}
		// 每出块间隔的一半时间打印一次进度
		time.Sleep(time.Duration(sleepSecs) * time.Second)
		/* 打印进度 */
		if isLogProgress {
			iter++
			if iter == iterNum {
				result.GetPercentage()
				iter = 0
			}
		}
	}
}

/**
 * 判断委员会能否停止, 若能则停止
 * 循环打印交易总执行进度
 */
func toStopClient(c *client.Client, recommitIntervalSecs,
	logProgressInterval int, isLogProgress bool, exitMode int) {
	log.Info("Monitor txpools and try to stop shards")
	sleepSecs := int(math.Ceil(float64(recommitIntervalSecs) / 2))
	iterNum := int(math.Ceil(float64(logProgressInterval) / float64(sleepSecs)))
	iter := 0
	if isLogProgress {
		log.Info("Set logProgressInterval(secs)", "iterNum*sleepSecs", iterNum*sleepSecs)
	} else {
		log.Info("Set log progress false")
	}

	for {
		canStop := false
		if exitMode == 0 {
			canStop = c.CanStopV1()
		} else if exitMode == 1 {
			canStop = c.CanStopV2() && c.InjectDoneMsgSent
		}
		c.LogQueues()
		if canStop {
			c.Close()
			break
		}
		// 每出块间隔的一半时间打印一次进度
		time.Sleep(time.Duration(sleepSecs) * time.Second)
		/* 打印进度 */
		if isLogProgress {
			iter++
			if iter == iterNum {
				result.GetPercentage()
				iter = 0
			}
		}
	}
}

func closeNode(node *node.Node) {
	node.Close()
}

func stopTBChain() {
	tbChain.Close()
}
