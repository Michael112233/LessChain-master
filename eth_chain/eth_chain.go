package eth_chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"go-w3chain/cfg"
	"go-w3chain/log"
	"go-w3chain/utils"
	"math"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

type ContractTB struct {
	ShardID    uint32 `json:"shardID" gencodec:"required"`
	Height     uint64 `json:"height" gencodec:"required"`
	BlockHash  string `json:"blockHash" gencodec:"required"`
	TxHash     string `json:"txHash" gencodec:"required"`
	StatusHash string `json:"statusHash" gencodec:"required"`
}

func Connect(port int) (*ethclient.Client, error) {
	addr := fmt.Sprintf("http://%s:%d", cfg.GethIPAddr, port)
	client, err := ethclient.Dial(addr)
	return client, err
}

func myPrivateKey(comID, nodeID uint32, mode int) (*ecdsa.PrivateKey, error) {
	var account string
	if mode == 1 {
		account = cfg.GanacheChainAccounts[comID]
	} else if mode == 2 {
		nodeAddr := cfg.ComNodeTable[comID][nodeID]
		account = cfg.GetGethChainPrivateKey(nodeAddr)
	} else {
		log.Error("unknown chain mode", "mode", mode)
	}
	privateKey, err := crypto.HexToECDSA(account)
	if err != nil {
		log.Error(fmt.Sprintf("crypto.HexToECDSA fail. err: %v hex: %v addr: %v", err, account, cfg.ComNodeTable[comID][nodeID]))
		return nil, err
	}

	return privateKey, nil
}

// 获取信标链最新区块哈希
func GetLatestBlockHash(client *ethclient.Client) (common.Hash, uint64) {
	header, err := client.HeaderByNumber(context.Background(), nil)
	if err != nil {
		log.Error("get tbchain latest block header fail", "err", err)
	}
	height := header.Number.Uint64()
	return header.Hash(), height
}

func GetBlockHash(client *ethclient.Client, height uint64) (common.Hash, uint64) {
	header, err := client.HeaderByNumber(context.Background(), big.NewInt(int64(height)))
	if err != nil {
		log.Error("get tbchain block header fail", "height", height, "err", err)
	}
	got_height := header.Number.Uint64()
	return header.Hash(), got_height
}

// 部署合约
func DeployContract(client *ethclient.Client,
	mode int,
	genesisTBs []ContractTB,
	required_sig_cnt uint32,
	shard_num uint32,
	addrs [][]common.Address,
	chainID int,
) (common.Address, *abi.ABI, *big.Int, error) {

	// 编译 Solidity 合约并获取合约 ABI 和字节码
	contractABI, err := abi.JSON(strings.NewReader(MyContractABI()))
	if err != nil {
		return common.Address{}, nil, big.NewInt(0), err
	}
	bytecode := common.FromHex(myContractByteCode())

	// 获取私钥
	privateKey, err := myPrivateKey(0, 0, mode)
	if err != nil {
		log.Error(fmt.Sprintf("getPrivateKey err: %v", err))
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(chainID)))
	if err != nil {
		log.Error(fmt.Sprintf("bind.NewKeyedTransactorWithChainID err: %v", err))
	}

	// 部署合约
	log.Debug("addrs", "addr", addrs)
	address, tx, _, err := bind.DeployContract(auth, contractABI, bytecode, client, genesisTBs, required_sig_cnt, shard_num, addrs)
	if err != nil {
		log.Error(fmt.Sprintf("DeployContract err: %v", err))
	}

	// 等待交易被挖矿确认
	_, err = bind.WaitDeployed(context.Background(), client, tx)
	if err != nil {
		log.Error(fmt.Sprintf("WaitDeployed err: %v", err))
	}

	fmt.Printf("contract deploy. address: %v\n", address)

	return address, &contractABI, nil, nil
}

var (
	// lastNonce map[uint32]uint64 = make(map[uint32]uint64)
	// lastNonce map[string]uint64 = make(map[string]uint64)
	lastNonce uint64 = math.MaxUint64
	// 通过该锁使不同委员会的AddTB方法串行执行，避免一些并发调用导致的问题
	call_lock      sync.Mutex
	lowestGasPrice *big.Int = big.NewInt(0)
)

// 存储信标到合约
func AddTB(client *ethclient.Client, contractAddr common.Address,
	abi *abi.ABI, mode int, tb *ContractTB,
	sigs [][]byte, vrfs [][]byte, seedHeight uint64,
	signers []common.Address, chainID int, nodeID uint32,
) error {

	call_lock.Lock()
	defer call_lock.Unlock()

	var comID uint32 = 0
	comID = tb.ShardID

	// 构造调用数据
	callData, err := abi.Pack("addTB", *tb, sigs, vrfs, seedHeight, signers)
	if err != nil {
		log.Error(fmt.Sprintf("abi.Pack err: %v", err))
		return err
	}

	// 通过私钥构造签名者
	privateKey, err := myPrivateKey(comID, nodeID, mode)
	if err != nil {
		log.Error(fmt.Sprintf("get myPrivateKey err: %v", err))
		return err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(chainID)))
	if err != nil {
		log.Error("bind.NewKeyedTransactorWithChainID err: %v", err)
		return err
	}

	// 设置交易参数
	auth.GasLimit = uint64(30000000) // 设置 gas 限制
	auth.Value = big.NewInt(0)       // 设置发送的以太币数量（如果有的话）

	var nonce uint64
	if lastNonce == math.MaxUint64 {
		// 如果在之前的交易中使用了相同的账户地址，而这些交易还未被确认（被区块打包），那么下一笔交易的nonce应该是
		// 当前账户的最新nonce+1。
		nonce, err = client.PendingNonceAt(context.Background(), auth.From)
		if err != nil {
			log.Error("client.PendingNonceAt err", "err", err)
			fmt.Println("client.PendingNonceAt err: ", err)
			return err
		}

		lastNonce = nonce
	} else {
		nonce = lastNonce + 1
		lastNonce = nonce
	}
	// nonce_lock.Unlock()

	if lowestGasPrice.Uint64() == 0 {
		gasPrice, err := client.SuggestGasPrice(context.Background())
		if err != nil {
			log.Error("client.SuggestGasPrice err", "err", err)
			fmt.Println("client.SuggestGasPrice err: ", err)
			return err
		}
		lowestGasPrice = gasPrice
	}

	// mustSend：循环发送直到交易发送成功
	for {
		tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), auth.GasLimit, lowestGasPrice, callData)

		// 签名交易
		signedTx, err := auth.Signer(auth.From, tx)
		if err != nil {
			log.Error("auth.Signer err", "err", err)
			fmt.Println("auth.Signer err: ", err)
			return err
		}

		// 发送交易
		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			log.Debug("client.SendTransaction err", "err", err, "txtype", "AddTB", "shardID", tb.ShardID, "height", tb.Height,
				"gasPrice", lowestGasPrice, "nonce", nonce)
			fmt.Println("client.SendTransaction err: ", err)

			if strings.Contains(err.Error(), "replacement transaction underpriced") || strings.Contains(err.Error(), "nonce too low") {
				nonce = nonce + 1
				lastNonce = nonce
			} else if strings.Contains(err.Error(), "transaction underpriced") {
				// 每次提升10%的手续费，并将该手续费作为最低手续费
				toAdd := new(big.Int).Div(lowestGasPrice, big.NewInt(10))
				lowestGasPrice = new(big.Int).Add(lowestGasPrice, toAdd)
			} else {
				return err
			}
		} else {
			// fmt.Printf("signedTX: %v\n", signedTx.Hash().Hex())
			break
		}
	}

	return nil
}

func AdjustRecordedAddrs(client *ethclient.Client, contractAddr common.Address,
	abi *abi.ABI, mode int,
	comID uint32, addrs []common.Address,
	vrfs [][]byte, seedHeight uint64,
	chainID int, nodeID uint32,
) error {

	call_lock.Lock()
	defer call_lock.Unlock()
	// 构造调用数据
	callData, err := abi.Pack("adjustRecordedAddrs", addrs, vrfs, seedHeight)
	if err != nil {
		log.Error(fmt.Sprint("abi.Pack err: ", err))
		return err
	}

	// 通过私钥构造签名者
	privateKey, err := myPrivateKey(comID, nodeID, mode)
	if err != nil {
		log.Error(fmt.Sprint("get myPrivateKey err: ", err))
		return err
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, big.NewInt(int64(chainID)))
	if err != nil {
		log.Error(fmt.Sprint("bind.NewKeyedTransactorWithChainID err: ", err))
		return err
	}

	// 设置交易参数
	auth.GasLimit = uint64(30000000) // 设置 gas 限制
	auth.Value = big.NewInt(0)       // 设置发送的以太币数量（如果有的话）

	var nonce uint64
	if lastNonce == math.MaxUint64 {
		// 如果在之前的交易中使用了相同的账户地址，而这些交易还未被确认（被区块打包），那么下一笔交易的nonce应该是
		// 当前账户的最新nonce+1。
		nonce, err = client.PendingNonceAt(context.Background(), auth.From)
		if err != nil {
			log.Error("client.PendingNonceAt err", "err", err)
			fmt.Println("client.PendingNonceAt err: ", err)
			return err
		}

		lastNonce = nonce
	} else {
		nonce = lastNonce + 1
		lastNonce = nonce
	}
	// nonce_lock.Unlock()

	if lowestGasPrice.Uint64() == 0 {
		gasPrice, err := client.SuggestGasPrice(context.Background())
		if err != nil {
			log.Error("client.SuggestGasPrice err", "err", err)
			fmt.Println("client.SuggestGasPrice err: ", err)
			return err
		}
		lowestGasPrice = gasPrice
	}

	// mustSend：循环发送直到交易发送成功
	for {
		tx := types.NewTransaction(nonce, contractAddr, big.NewInt(0), auth.GasLimit, lowestGasPrice, callData)

		// 签名交易
		signedTx, err := auth.Signer(auth.From, tx)
		if err != nil {
			log.Error("auth.Signer err", "err", err)
			fmt.Println("auth.Signer err: ", err)
			return err
		}

		// 发送交易
		err = client.SendTransaction(context.Background(), signedTx)
		if err != nil {
			log.Debug("client.SendTransaction err", "err", err, "txtype", "adjustRecordedAddrs", "shardID", comID, "seedHeight", seedHeight,
				"gasPrice", lowestGasPrice, "nonce", nonce)
			fmt.Println("client.SendTransaction err: ", err)

			if strings.Contains(err.Error(), "replacement transaction underpriced") || strings.Contains(err.Error(), "nonce too low") {
				nonce = nonce + 1
				lastNonce = nonce
			} else if strings.Contains(err.Error(), "transaction underpriced") {
				// 每次提升10%的手续费，并将该手续费作为最低手续费
				toAdd := new(big.Int).Div(lowestGasPrice, big.NewInt(10))
				lowestGasPrice = new(big.Int).Add(lowestGasPrice, toAdd)
			} else {
				return err
			}
		} else {
			// fmt.Printf("signedTX: %v\n", signedTx.Hash().Hex())
			break
		}
	}

	return nil

}

// 从合约读取信标
func GetTB(client *ethclient.Client, contractAddr common.Address, abi *abi.ABI, shardID uint32, height uint64) (*ContractTB, error) {
	callData, err := abi.Pack("getTB", shardID, height)
	if err != nil {
		fmt.Println("abi.Pack err: ", err)
		return nil, err
	}

	msg := ethereum.CallMsg{
		To:   &contractAddr,
		Data: callData,
	}

	// callContract 向合约发送一笔只读调用
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		fmt.Println("client.CallContract err: ", err)
		return nil, err
	}
	// fmt.Printf("result: %v\n", result)

	tb := resolveTB(result, abi)

	return tb, nil
}

func resolveTB(resultData []byte, abi *abi.ABI) *ContractTB {
	response, err := abi.Methods["getTB"].Outputs.Unpack(resultData)
	if err != nil {
		fmt.Println("解析返回数据失败:", err)
		return nil
	}

	contractTB := response[0]
	// fmt.Printf("%v\n", contractTB)

	fields := utils.GetFieldValues(contractTB)
	tb := &ContractTB{
		Height:     fields["Height"].(uint64),
		ShardID:    fields["ShardID"].(uint32),
		BlockHash:  fields["BlockHash"].(string),
		TxHash:     fields["TxHash"].(string),
		StatusHash: fields["StatusHash"].(string),
	}

	return tb
}
