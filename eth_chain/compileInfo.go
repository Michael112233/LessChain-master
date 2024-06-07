package eth_chain

import (
	"go-w3chain/log"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

func MyContractABI() string {
	// 获取当前正在执行的文件的路径
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Error("无法确定当前文件的路径")
	}

	// 构造与当前文件相同目录下的目标文件路径
	targetFile := filepath.Join(filepath.Dir(filename), "abi.json")

	// 读取目标文件
	content, err := ioutil.ReadFile(targetFile)
	if err != nil {
		log.Error("读取文件错误", "err", err)
	}

	abi := string(content)

	return abi
}

func myContractByteCode() string {
	// 获取当前正在执行的文件的路径
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Error("无法确定当前文件的路径")
	}

	// 构造与当前文件相同目录下的目标文件路径
	targetFile := filepath.Join(filepath.Dir(filename), "bytecode.txt")

	// 读取目标文件
	content, err := ioutil.ReadFile(targetFile)
	if err != nil {
		log.Error("读取文件错误", "err", err)
	}

	bytecode := string(content)

	return bytecode
}
