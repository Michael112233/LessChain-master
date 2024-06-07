package core

import (
	"time"
)

// Config is the configuration parameters of mining.
type CommitteeConfig struct {
	RecommitTime         time.Duration // The time interval for miner to re-create mining work.
	MaxBlockSize         int
	InjectSpeed          int
	Height2Reconfig      int
	MultiSignRequiredNum int
}

type BeaconChainConfig struct {
	/** 信标链的运行模式
	mode=0表示运行模拟信标链
	mode=1表示运行ganache搭建的以太坊私链
	mode=2表示运行geth搭建的以太坊私链 */
	Mode                 int
	ChainId              int
	Port                 int
	BlockInterval        int
	Height2Confirm       uint64
	MultiSignRequiredNum int
}
