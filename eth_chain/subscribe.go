package eth_chain

import (
	"encoding/json"
	"fmt"
	"go-w3chain/cfg"
	"go-w3chain/log"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/websocket"
)

type WebSocketResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		Subscription string `json:"subscription"`
		Result       struct {
			LogIndex         string   `json:"logIndex"`
			TransactionIndex string   `json:"transactionIndex"`
			TransactionHash  string   `json:"transactionHash"`
			BlockHash        string   `json:"blockHash"`
			BlockNumber      string   `json:"blockNumber"`
			Address          string   `json:"address"`
			Data             string   `json:"data"`
			Topics           []string `json:"topics"`
			Type             string   `json:"type"`
		} `json:"result"`
	} `json:"params"`
}

type Event struct {
	Msg        string
	ShardID    uint32
	Height     uint64
	Eth_height uint64
}

func SubscribeEvents(port int, contractAddr common.Address, eventChannel chan *Event) {
	// WebSocket 连接地址
	url := fmt.Sprintf("ws://%s:%d", cfg.GethIPAddr, port)

	// 创建 WebSocket 连接
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Error("Failed to connect to eth_chain WebSocket:", "err", err)
	}
	defer conn.Close()

	// 订阅合约事件
	subscribeRequest := []byte(fmt.Sprintf(`{
		"jsonrpc": "2.0",
		"method": "eth_subscribe",
		"params": ["logs", {
			"address": "%v"
		}],
		"id": 1
	}`, contractAddr.Hex()))

	err = conn.WriteMessage(websocket.TextMessage, subscribeRequest)
	if err != nil {
		log.Error("Failed to send subscription request:", "err", err)
	}

	defer close(eventChannel)

	// 第一个返回消息
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Error("Failed to read message from eth_chain WebSocket:", "err", err)
	}
	log.Debug("websocket subscribe response", "content", string(message))

	// 处理事件通知
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Error("Failed to read message from eth_chain WebSocket:", "err", err)
		}

		// fmt.Println("Received message:", string(message))

		// // 在这里处理事件通知，解析事件数据等
		// if string(message)[2:4] == "id" {
		// 	continue
		// }
		var response WebSocketResponse
		err = json.Unmarshal(message, &response)
		if err != nil {
			fmt.Println("Failed to unmarshal JSON:", err)
			log.Debug("Failed to unmarshal JSON", "err", err)
			continue
		}

		heightStr := response.Params.Result.BlockNumber
		eth_height, err := strconv.ParseInt(heightStr, 0, 64)
		if err != nil {
			log.Error(fmt.Sprintf("parseInt fail. stringValue: %v err: %v", heightStr, err))
		}

		// log.Debug(fmt.Sprintf("suscribe ethchain height: %v", height))
		data := response.Params.Result.Data
		// fmt.Println("Data:", data)
		event := handleMessage(data, uint64(eth_height))
		if event == nil {
			continue
		}
		event.Eth_height = uint64(eth_height)
		if event.Msg == "addTB" {
			eventChannel <- event
		} else if strings.Contains(event.Msg, "addTB...") || strings.Contains(event.Msg, "adjustAddr") {
			log.Error(event.Msg)
		}
	}
}

func handleMessage(data string, eth_height uint64) *Event {
	eventABI := `
	[
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "string",
					"name": "message",
					"type": "string"
				},
				{
					"indexed": false,
					"internalType": "uint32",
					"name": "shardID",
					"type": "uint32"
				},
				{
					"indexed": false,
					"internalType": "uint64",
					"name": "height",
					"type": "uint64"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "addr",
					"type": "address"
				}
			],
			"name": "LogMessage",
			"type": "event"
		},
		{
			"anonymous": false,
			"inputs": [
				{
					"indexed": false,
					"internalType": "string",
					"name": "verifyType",
					"type": "string"
				},
				{
					"indexed": false,
					"internalType": "bytes32",
					"name": "msgHash",
					"type": "bytes32"
				},
				{
					"indexed": false,
					"internalType": "bytes",
					"name": "sig",
					"type": "bytes"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "recoverAddr",
					"type": "address"
				},
				{
					"indexed": false,
					"internalType": "address",
					"name": "expectedAddr",
					"type": "address"
				}
			],
			"name": "VerifyMessage",
			"type": "event"
		}
	]
	`
	abi, err := abi.JSON(strings.NewReader(eventABI))
	if err != nil {
		log.Error("abi.JSON fail", "err", err)
	}

	// 尝试解码为LogMessage事件
	decodedLogData := make(map[string]interface{})
	errLog := abi.UnpackIntoMap(decodedLogData, "LogMessage", common.FromHex(data))
	// 尝试解码为VerifyMessage事件
	decodedVerifyData := make(map[string]interface{})
	errVerify := abi.UnpackIntoMap(decodedVerifyData, "VerifyMessage", common.FromHex(data))

	if errLog != nil && errVerify != nil {
		log.Error("Failed to decode both LogMessage and VerifyMessage", "errLog", errLog, "errVerify", errVerify)
	}

	if errLog == nil {
		// 读取数据前可以先打印看看有哪些字段
		// 而且需要检查eventABI是否正确
		// fmt.Printf("decodedData: %v\n", decodedData)

		message := decodedLogData["message"].(string)
		shardID := decodedLogData["shardID"].(uint32)
		height := decodedLogData["height"].(uint64)

		addr := common.Address{}
		if decodedLogData["addr"] != nil {
			addr = decodedLogData["addr"].(common.Address)
		}
		// addr := decodedData["addr"].([32]byte)

		fmt.Printf("Eth_height: %d\t", eth_height)
		fmt.Printf("Message: %s\t", message)
		fmt.Printf("ShardID: %d\t", shardID)
		fmt.Printf("Height: %d\n", height)

		if (addr != common.Address{}) {
			fmt.Printf("Address: %v\n", addr)
			log.Debug(fmt.Sprintf("got LogEvent... Eth_height:%d Message:%s ShardID:%d Height:%d Address:%v",
				eth_height, message, shardID, height, addr))
		} else {
			log.Debug(fmt.Sprintf("got LogEvent... Eth_height:%d Message:%s ShardID:%d Height:%d",
				eth_height, message, shardID, height))
		}

		return &Event{
			Msg:     message,
			ShardID: shardID,
			Height:  height,
		}
	}

	if errVerify == nil {
		verifyType := decodedVerifyData["verifyType"].(string)
		msgHash := decodedVerifyData["msgHash"].([]byte)
		sig := decodedVerifyData["sig"].([]byte)
		recoverAddr := decodedVerifyData["recoverAddr"].(common.Address)
		expectedAddr := decodedVerifyData["expectedAddr"].(common.Address)

		log.Debug(fmt.Sprintf("got VerifyEvent... verifyType:%s msgHash:%x sig:%x recoverAddr:%x expectedAddr:%x",
			verifyType, msgHash, sig, recoverAddr, expectedAddr))
	}

	return nil
}
