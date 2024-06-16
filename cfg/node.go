package cfg

import "fmt"

var (
	GethIPAddr   string
	BooterAddr   string
	ClientTable  map[uint32]string
	NodeTable    map[uint32]map[uint32]string // 分片->节点
	ComNodeTable map[uint32]map[uint32]string // 委员会->节点
)

// 包被引用时自动执行init函数
func init() {
	GethIPAddr = "192.168.3.4"
	BooterAddr = "192.168.3.4:" + fmt.Sprint(18000)

	// 配置1个客户端，但为了可扩展，也实现成map的形式
	ClientTable = make(map[uint32]string)
	ClientTable[0] = "192.168.3.4:" + fmt.Sprint(19000)

	// 从50002开始配置分片节点
	// 配置每个分片20个节点的地址
	NodeTable = make(map[uint32]map[uint32]string)

	hostTable := make(map[uint32]string)
	startPortTable := make(map[uint32]uint32)
	var startPort uint32 = 20000
	hostTable[0] = "192.168.3.2"
	startPortTable[0] = startPort
	hostTable[1] = "192.168.3.6"
	startPortTable[1] = startPort
	hostTable[2] = "192.168.3.7"
	startPortTable[2] = startPort
	hostTable[3] = "192.168.3.8"
	startPortTable[3] = startPort
	hostTable[4] = "192.168.3.9"
	startPortTable[4] = startPort
	hostTable[5] = "192.168.3.10"
	startPortTable[5] = startPort
	hostTable[6] = "192.168.3.11"
	startPortTable[6] = startPort
	hostTable[7] = "192.168.3.12"
	startPortTable[7] = startPort

	for k := 1; k <= 3; k++ {
		startPort += 20
		for l := 0; l < 8; l++ {
			hostTable[uint32(l+k*8)] = hostTable[uint32(l)]
			startPortTable[uint32(l+k*8)] = startPort
		}
	}

	var i, j uint32
	for i = 0; i < uint32(len(hostTable)); i++ {
		NodeTable[i] = make(map[uint32]string)
		for j = 0; j < 20; j++ {
			NodeTable[i][j] = hostTable[i] + ":" + fmt.Sprint(startPortTable[i]+j)
		}
	}

	// 初始时ComNodeTable与NodeTable相等，重组时会发现变化
	ComNodeTable = NodeTable

}
