package cfg

import (
	"encoding/json"
	"fmt"
	"go-w3chain/log"
	"io/ioutil"
	"path/filepath"
	"runtime"
)

// ganache 私链上有钱的账户私钥，用来发起提交信标的交易
// 目前每个账户负责一个分片的交易
var GanacheChainAccounts []string = []string{
	"d12c928f281ed4a05f6dbf434bbbac5706826d9b2e3966077ef580df14073eb3",
	"635ccd7f8cb78b293486ee535a8aac38d33b400e4833ed07d39d2841995e0cd6",
	"831d55e90f4a55085ccf8a9acf849d9a6ce00f46fb430e47118d23af308e1486",
	"d2c42ed9778acdf7f86ce013f5437cfa463f417c0523e5ceaa9e1f8ed48eef5e",
	"26ea2b1eebb43a50c0fc5f5451073ec0831f85621765fabad93a61132cb14d21",
	"1d41896df03f6971785b1e3927daa4eed3df9113a267d953b10bfd34775a1ef4",
	"127ab599973981d4282221e339386b34c15a6b1685e0062ce388afb2ac3f1610",
	"0406fd4b37b0fef67a4cd1ca447452a0fbe81ec972e8437c2d278614295d2412",
}

// geth 私链上有钱的账户私钥，用来发起提交信标的交易
// 目前每个账户负责一个或多个委员会的交易
// var GethChainAccounts []string = []string{
// 	"a65d8aa17661de2eebf80c481c1d359558c3674fdc6ad916ff56e468710f5fb9",
// 	"4454d10f67d9470646044fd8bad1f0fc6f1ba7f20045695389b38844c0e1f835",
// 	"f2f0a5d8ff4d1ee1cbda99e6313063f0977357db544ad2c05787071d5ee8a044",
// 	"fb235001be3bc0e2500e74b66ef11004c62ed57b15d07e2766a04e0034eb261c",
// 	"8cd8a4bd900eb2628f52429983f9f25342358b23fa4ed99d7d343c41badcf16a",
// 	"f8108cd35deae7352e45ec693cdea8e38e3bd85bbbaf85332e5287af228b836d",
// 	"c9856af76f135a475350b5727dad741352f0418c4a7f521c3e8cc9f5adf2a074",
// 	"ce0f724e265d4814dd65e7d70287915d3b10915333db7b6787cd35ed5c3b1b19",
// }

var gethChainPriKeys map[string]string
var privateKeyJsonFile = "../eth_chain/geth-chain-data/privateKeys.json"

func GetGethChainPrivateKey(nodeAddr string) string {
	if gethChainPriKeys == nil {
		gethChainPriKeys = make(map[string]string)
		allPrivateKeys := getAllPrivateKey(privateKeyJsonFile)
		addrList := getAllAddrs()
		for i, addr := range addrList {
			gethChainPriKeys[addr] = allPrivateKeys[i]
		}
	}
	return gethChainPriKeys[nodeAddr]
}

func getAllAddrs() []string {
	addrs := make([]string, 0)
	for _, shardAddrs := range NodeTable {
		for _, addr := range shardAddrs {
			addrs = append(addrs, addr)
		}
	}
	return addrs
}

func getAllPrivateKey(file string) []string {
	// 获取当前文件的路径
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		log.Error("Error retrieving current file path")
	}

	// 构造privateKeyJsonFile的绝对路径
	privateKeyJsonFile := filepath.Join(filepath.Dir(filename), file)
	// fmt.Println("Private Keys File Path:", privateKeyJsonFile)

	// 读取私钥文件
	data, err := ioutil.ReadFile(privateKeyJsonFile)
	if err != nil {
		fmt.Println("Error reading private keys file:", err)
		log.Error(fmt.Sprintf("Error reading private keys file: %v", err))
		return nil
	}

	// 解析JSON文件内容到map
	var privateKeysMap map[string]string
	err = json.Unmarshal(data, &privateKeysMap)
	if err != nil {
		fmt.Println("Error unmarshalling private keys:", err)
		log.Error(fmt.Sprintf("Error unmarshalling private keys: %v", err))
		return nil
	}

	// 将map的值转存到切片
	var privateKeys []string
	for _, privateKey := range privateKeysMap {
		privateKeys = append(privateKeys, privateKey)
	}

	return privateKeys
}
