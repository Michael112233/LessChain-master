package main

// GO111MODULE=on go run main.go

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"go-w3chain/controller"

	"github.com/spf13/pflag"
)

type Args struct {
	Mode     string
	Role     string
	ShardNum int32
	ShardId  int32
	NodeId   int32
}

var mode = pflag.StringP("mode", "m", "debug", "mode (run or debug)")
var role = pflag.StringP("role", "r", "booter", "role type (booter, node or client)")
var shardNum = pflag.Int32P("shardNum", "S", 1, "number of shards(and committees)")
var shardId = pflag.Int32P("shardId", "s", 0, "shard id")
var nodeId = pflag.Int32P("nodeId", "n", 0, "node id")

/** go build -o brokerChain.exe
 * brokerChain.exe -m run >> nohup.out 2>&1
 */
func main() {
	pflag.Parse()

	// args := &Args{
	// 	Mode:     *mode,
	// 	Role:     *role,
	// 	ShardNum: *shardNum,
	// 	ShardId:  *shardId,
	// 	NodeId:   *nodeId,
	// }

	cfgfilename := "cfg/debug.json"
	if *mode == "run" {
		cfgfilename = "cfg/run.json"
	} else if *mode != "debug" {
		fmt.Println("wrong mode")
		return
	}
	fmt.Println("cfg file:", cfgfilename)

	// if *role == "node" && *nodeId < 4 {
	// 	var wg sync.WaitGroup
	// 	wg.Add(1)
	// 	go OpenNewTerminalsAndRun(&wg, args)
	// 	wg.Wait()
	// }

	controller.Main(cfgfilename, *role, *shardNum, *shardId, *nodeId)
	// closeTerminalWindow(*nodeId)
}

func OpenNewTerminalsAndRun(wg *sync.WaitGroup, args *Args) {
	defer wg.Done()
	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current working directory:", err)
		return
	}

	for i := 1; i < 2; i++ {
		// 构建新的命令
		newNodeId := args.NodeId + 4*int32(i)
		cmdStr := fmt.Sprintf(`tell application "Terminal" to do script "cd %s && ./lessChain -r %s -S %d -s %d -n %d"`, cwd, args.Role, args.ShardNum, args.ShardId, newNodeId)

		// 执行命令以打开新的 Terminal 窗口并运行指定的命令
		cmd := exec.Command("osascript", "-e", cmdStr)
		err = cmd.Start()
		if err != nil {
			fmt.Println("Error opening new terminal:", err)
		}
	}

}

func closeTerminalWindow(nodeID int32) {
	if nodeID < 4 {
		return // 不关闭 -n 小于4的窗口
	}
	cmdStr := fmt.Sprintf(`tell application "Terminal" to close (every window whose name contains "-n %d")`, nodeID)
	cmd := exec.Command("osascript", "-e", cmdStr)
	err := cmd.Run()
	if err != nil {
		fmt.Println("Error closing terminal:", err)
	}
}
