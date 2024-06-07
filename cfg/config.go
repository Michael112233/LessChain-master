package cfg

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Cfg struct {
	LogLevel            int    `json:"LogLevel"`
	LogFile             string `json:"LogFile"`
	IsProgressBar       bool   `json:"IsProgressBar"`
	IsLogProgress       bool   `json:"IsLogProgress"`
	LogProgressInterval int    `json:"LogProgressInterval"`

	Role string `json:"Role"`

	ClientNum int `json:"ClientNum"`
	ClientId  int `json:"ClientId"`

	ShardNum             int `json:"ShardNum"`
	ShardId              int `json:"ShardId"`
	ShardSize            int `json:"ShardSize"`
	ComAllNodeNum        int `json:"ComAllNodeNum"`
	NodeId               int `json:"NodeId"`
	MultiSignRequiredNum int `json:"MultiSignRequiredNum"`

	MaxTxNum             int    `json:"MaxTxNum"`
	InjectSpeed          int    `json:"InjectSpeed"`
	RecommitIntervalSecs int    `json:"RecommitInterval"`
	Height2Rollback      int    `json:"Height2Rollback"`
	Height2Reconfig      int    `json:"Height2Reconfig"`
	ReconfigMode         string `json:"ReconfigMode"`
	FastsyncBlockNum     int    `json:"FastsyncBlockNum"`
	Height2Confirm       int    `json:"Height2Confirm"`
	MaxBlockTXSize       int    `json:"MaxBlockTXSize"`
	DatasetDir           string `json:"DatasetDir"`

	TbchainBlockIntervalSecs int `json:"TbchainBlockIntervalSecs"`
	BeaconChainMode          int `json:"BeaconChainMode"`
	BeaconChainID            int `json:"BeaconChainID"`
	BeaconChainPort          int `json:"BeaconChainPort"`
	ExitMode                 int `json:"ExitMode"`
	ReconfigTime             int `json:"ReconfigTime"`
}

var (
	defaultCfg *Cfg = nil
)

func DefaultCfg(cfgFilename string) *Cfg {
	if defaultCfg == nil {
		defaultCfg = ReadCfg(cfgFilename)
	}
	return defaultCfg
}

func ReadCfg(filename string) *Cfg {
	jsonFile, err := os.Open(filename)
	if err != nil {
		fmt.Println("error opening json file")
		os.Exit(1)
	}
	defer jsonFile.Close()
	jsonData, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		fmt.Println("error reading json file")
		os.Exit(1)
	}
	var cfg Cfg
	json.Unmarshal(jsonData, &cfg)
	// log.Info(fmt.Sprintf("%#v", cfg)) // 此处还未初始化
	return &cfg
}

const (
	datadir = ".lessChain"
)

/** 返回所有节点存储数据的默认父路径
 * $Home/.lessChain/
 */
func DefaultDataDir() string {
	// Try to place the data folder in the user's home dir
	home := os.Getenv("HOME")
	return filepath.Join(home, datadir)

}
