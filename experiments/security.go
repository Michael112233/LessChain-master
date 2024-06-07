package experiments

import (
	"fmt"
	"go-w3chain/cfg"
	"go-w3chain/data"
	"go-w3chain/log"
	"path/filepath"
	"runtime"
	"strings"
)

// 随着epoch数量增加，单个分片失效影响的累计交易数量
func Func2() {
	cfg := cfg.DefaultCfg("cfg/debug.json")

	_, filename, _, _ := runtime.Caller(0)
	logFileName := strings.TrimSuffix(filepath.Base(filename), ".go") + ".log"
	log.SetLogInfo(log.Lvl(cfg.LogLevel), filepath.Join(filepath.Dir(filename), logFileName))

	shardNums := []int{1, 2, 4, 8}
	for _, shardNum := range shardNums {
		data.ClearAll()
		data.LoadETHData(cfg.DatasetDir, cfg.MaxTxNum)
		data.SetTxShardId(shardNum)
		// 失效分片ID
		failShard := 0
		alltxs := data.GetAlltxs()
		epochs := 10
		injectSpeed := len(alltxs) / epochs
		lowerbound := 0
		epoch_fail_cnts := make([]int, 0)
		epoch_fail_cnt := 0
		for i := 0; i < epochs; i++ {
			upperbound := min(len(alltxs), lowerbound+injectSpeed)
			txs := alltxs[lowerbound:upperbound]
			lowerbound = upperbound
			for _, tx := range txs {
				if tx.Sender_sid == uint32(failShard) || tx.Recipient_sid == uint32(failShard) {
					epoch_fail_cnt += 1
				}
			}
			epoch_fail_cnts = append(epoch_fail_cnts, epoch_fail_cnt)
		}
		log.Info("Func2 -- accumulated failed txs", "shardNum", shardNum, "len(alltxs)", len(alltxs), "failShard", fmt.Sprintf("shard%d", failShard))
		res := "["
		if len(epoch_fail_cnts) > 0 {
			res += fmt.Sprintf("%d", epoch_fail_cnts[0])
		}
		for i := 1; i < len(epoch_fail_cnts); i++ {
			res += fmt.Sprintf(",%d", epoch_fail_cnts[i])
		}
		res += "]"
		log.Info("Func2 -- result", "epoch_fail_cnts", res)
	}
}

// 不同分片数量下，单个分片失效对所有交易影响的比例
func Func1() {
	cfg := cfg.DefaultCfg("cfg/debug.json")

	_, filename, _, _ := runtime.Caller(0)
	logFileName := strings.TrimSuffix(filepath.Base(filename), ".go") + ".log"
	log.SetLogInfo(log.Lvl(cfg.LogLevel), filepath.Join(filepath.Dir(filename), logFileName))

	data.LoadETHData(cfg.DatasetDir, cfg.MaxTxNum)

	maxShardNum := 20
	shardNums := make([]int, maxShardNum)
	ratios := make([]float64, maxShardNum)
	for shardNum := 1; shardNum <= maxShardNum; shardNum++ {
		data.SetTxShardId(shardNum)
		alltxs := data.GetAlltxs()

		shardAffectTxCnt := make(map[uint32]int)
		for _, tx := range alltxs {
			var i uint32
			for i = 0; i < uint32(shardNum); i++ {
				if tx.Sender_sid == uint32(i) || tx.Recipient_sid == uint32(i) {
					shardAffectTxCnt[i] += 1
				}
			}
		}

		total := 0
		for i := 0; i < shardNum; i++ {
			total += shardAffectTxCnt[uint32(i)]
		}
		avg := total / shardNum
		ratio := float64(avg) / float64(len(alltxs))

		log.Info("Func1", "len(alltxs)", len(alltxs), "shardAffectTxCnt", shardAffectTxCnt, "avg", avg, "ratio", ratio)
		shardNums[shardNum-1] = shardNum
		ratios[shardNum-1] = ratio
	}
	log.Info("result", "shardNums", shardNums, "ratios", ratios)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
