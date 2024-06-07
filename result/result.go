package result

import (
	"encoding/csv"
	"fmt"
	"go-w3chain/log"
	"os"
	"sort"
	"sync"

	"github.com/schollz/progressbar/v3"
)

type TXReceipt struct {
	TxID             uint64
	ConfirmTimeStamp uint64
	TxStatus         uint64
	ShardID          int

	BlockHeight uint64
	MKproof     []byte
}

var (
	res           *Result
	bar           *progressbar.ProgressBar
	IsProgressBar bool
	// w3rollbackInterval uint64 = uint64(20)
)

/* 内置函数，当包被import时自动执行此函数，init函数先于本包内的其他函数被执行 */
func init() {
	NewResult()
}

type Result struct {
	csvfilename string
	/* 注入的交易总数量 */
	Totalnum int
	/* 已完成的交易数量 */
	allComplished int
	/* 已完成的交易数量对应的负载（一笔跨分片交易可能导致2~3个负载） */
	WorkLoad int
	/* 交易开始广播的时间 */
	BroadcastMap []uint64
	/* 交易完成的时间 */
	ConfirmMap []uint64
	/* 所有交易的状态, AllTXStatus[i]是一个列表，因为一笔交易可能对应多个状态 */
	AllTXStatus [][]uint64
	/* 超时回滚的跨分片交易数量 */
	allRollBack int
	/* 每个分片已执行的负载 */
	workload4shard map[int]int // 每个分片的workload
	lock           sync.Mutex
}

func NewResult() {
	res = &Result{
		Totalnum:     0,
		WorkLoad:     0,
		BroadcastMap: make([]uint64, 0),
		ConfirmMap:   make([]uint64, 0),
	}
}
func GetResult() *Result {
	return res
}
func SetIsProgressBar(isuse bool) {
	IsProgressBar = isuse
}
func SetcsvFilename(logfile string) {
	strlen := len(logfile)
	res.csvfilename = logfile[0 : strlen-4]
	res.csvfilename += ".csv"
	fmt.Println(res.csvfilename)
}

/* workload(最坏情况的负载) 和 totalnum functions */
func SetTotalTXNum(totalnum int) {
	res.Totalnum = totalnum
	res.WorkLoad = 0
	res.allComplished = 0
	res.BroadcastMap = make([]uint64, totalnum)
	res.ConfirmMap = make([]uint64, totalnum)
	res.AllTXStatus = make([][]uint64, totalnum)
	res.workload4shard = make(map[int]int)
	/* 设置进度条, 基于交易数量而不是最坏情况的负载 */
	if IsProgressBar {
		// bar = progressbar.Default(int64(res.WorkLoad))
		bar = progressbar.Default(int64(totalnum))
	}

}

/* broadcast time functions */
func SetBroadcastMap(table map[uint64]uint64) {
	// if res.Totalnum != len(table) {
	// 	log.Error("res.Totalnum != len(table)")
	// }
	for i, t := range table {
		res.BroadcastMap[i] = t
	}
}
func GetBroadcastMap() []uint64 {
	return res.BroadcastMap
}

func SetTXReceiptV2(table map[uint64]*TXReceipt) {
	res.lock.Lock()
	defer res.lock.Unlock()

	complished := 0

	for k, v := range table {
		if v.TxStatus == DefaultStatus {
			log.Error("tx's status lost.", "txid", v.TxID)
		}
		res.AllTXStatus[k] = append(res.AllTXStatus[k], v.TxStatus)
		// if len(res.AllTXStatus[k]) > 2 {
		// 	log.Warn("TX has too much status!", "txid", k, "count", len(res.AllTXStatus[k]), "txStatusList", getStatusListStr(res.AllTXStatus[k]))
		// }
		// 每执行一笔交易或一笔子交易都使负载加一
		res.workload4shard[v.ShardID] += 1
		/* 判断交易是否完成 */
		if checkTXFinished(v.TxStatus) {
			complished++
			res.ConfirmMap[k] = v.ConfirmTimeStamp
		}
		/* 是否为回滚交易 */
		if v.TxStatus == RollbackSuccess {
			res.allRollBack += 1
		}
	}
	res.allComplished += complished
	/* 更新进度条 */
	if IsProgressBar {
		// bar.Add(len(table))
		bar.Add(complished)
	}
	/* 计算负载 */
	res.WorkLoad += len(table)
}

/* 打印 所有交易 statusList */
func PrintTXReceipt() {
	for txid, txStatusList := range res.AllTXStatus {
		if txid%1000 != 0 {
			continue
		}
		if len(res.AllTXStatus[txid]) == 0 {
			log.Debug("no status", "txid", txid)
			continue
		}
		log.Debug("txid=" + fmt.Sprint(txid) + " txStatusList=" + getStatusListStr(txStatusList))
	}

	// log.Debug("TXFinalStatus", "txid", 1, "txStatusList", getStatusListStr(res.AllTXStatus[1]))
}

func GetPercentage() {
	percentage := fmt.Sprintf("%d/%d", res.allComplished, res.Totalnum)
	log.Info("Uncomplish.." + percentage)
}

/* 交易停止注入即停止执行，有些交易可能还没执行 */
func GetThroughtPutAndLatencyV2() (float64, float64, float64, []int) {
	// if IsProgressBar {
	// 	bar.Finish()
	// }

	// workload4shard := make(map[uint64]uint64) // 每个分片对应的workload

	latencys := make([]uint64, res.Totalnum)
	sum := uint64(0)
	max_latency := 0
	max_latency_id := -1
	min_latency := 100000
	min_latency_id := -1
	begin := res.BroadcastMap[0]
	end := uint64(0)
	// executed_cnt := 0
	for i := 0; i < res.Totalnum; i++ {
		// 过滤掉未执行完成的交易
		if res.ConfirmMap[i] == 0 {
			continue
		}

		latencys[i] = res.ConfirmMap[i] - res.BroadcastMap[i]
		sum += latencys[i]
		if latencys[i] > uint64(max_latency) {
			max_latency = int(latencys[i])
			max_latency_id = i
		}
		if latencys[i] < uint64(min_latency) {
			min_latency = int(latencys[i])
			min_latency_id = i
		}
		if end < res.ConfirmMap[i] {
			end = res.ConfirmMap[i]
		}
	}

	usedTime := end - begin
	/* 记录数据到 csv 文件 */
	writeToCsv(latencys)

	/* 输出指标 */
	log.Info("input TX num: " + fmt.Sprint(res.Totalnum))
	log.Info("output TX num (num of TXs executed): " + fmt.Sprint(res.allComplished))
	log.Info("rollback TX num: " + fmt.Sprint(res.allRollBack))
	rollbackRate := float64(res.allRollBack) / float64(res.allComplished+res.allRollBack)
	log.Info("rollback rate: " + fmt.Sprint(rollbackRate))

	log.Info("used time: " + fmt.Sprint(usedTime) + " (s)")
	thrput := float64(res.allComplished) / float64(usedTime)
	log.Info("throughput: " + fmt.Sprint(thrput) + " (tx/s)")

	averageLatency := float64(sum) / float64(res.allComplished)
	log.Info("average latency: " + fmt.Sprint(averageLatency))
	log.Info("max latency: "+fmt.Sprint(max_latency), "txid", max_latency_id, "txStatus", getStatusListStr(res.AllTXStatus[max_latency_id]))
	log.Info("min latency: "+fmt.Sprint(min_latency), "txid", min_latency_id, "txStatus", getStatusListStr(res.AllTXStatus[min_latency_id]))

	log.Info("output workload: " + fmt.Sprint(res.WorkLoad) + " (TXs)")
	keys := make([]int, 0, len(res.workload4shard))
	for k := range res.workload4shard {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	overloads := make([]int, 0, len(keys))

	for _, k := range keys {
		// log.Info("workload for shard", "shardID", k, "workload", res.workload4shard[k])
		overloads = append(overloads, res.workload4shard[k])
	}
	log.Info("wordload for shard", "array", overloads)

	return thrput, averageLatency, rollbackRate, overloads
}

func writeToCsv(latencys []uint64) {
	log.Info("Write latencys to csv file.")
	File, err := os.OpenFile(res.csvfilename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Warn(" csv file open failed!")
	}
	defer File.Close()

	WriterCsv := csv.NewWriter(File)
	// broadcaststr := make([]string, len(latencys))
	// confirmstr := make([]string, len(latencys))
	latstr := make([]string, 0, res.allComplished)
	for i, lat := range latencys {
		if res.ConfirmMap[i] > 0 {
			latstr = append(latstr, fmt.Sprint(lat))
		}
		// broadcaststr[i] = fmt.Sprint(res.BroadcastMap[i])
		// confirmstr[i] = fmt.Sprint(res.ConfirmMap[i])
	}
	// err1 := WriterCsv.Write(broadcaststr)
	// err2 := WriterCsv.Write(confirmstr)
	err3 := WriterCsv.Write(latstr)
	if err3 != nil {
		log.Warn(" csv file write failed!")
	}
	WriterCsv.Flush() //刷新，不刷新是无法写入的
	log.Info("Write csv successed...")
}
