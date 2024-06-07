/*
该文件中生成的账户是以太坊私链上的账户，用来向以太坊私链发起交易，与系统中用到的w3account不同。
*/

package geth_chain_data

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
)

func generateEthAccount() {
	// 设定数据目录和密码文件的路径
	dataDir := "./data"
	passwordFile := "./emptyPsw.txt" // 这个文件应该包含用于加密新账户的密码，默认为空

	// 设定要创建的账户数量
	numAccounts := 59

	// 循环创建账户
	for i := 1; i <= numAccounts; i++ {
		fmt.Printf("Creating account %d\n", i)

		// 构建 geth 命令
		cmd := exec.Command("geth", "--datadir", dataDir, "account", "new", "--password", passwordFile)

		// 执行命令，并获取输出
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("Failed to create account %d: %v\n", i, err)
			return
		}
		fmt.Printf("Account %d created successfully:\n%s\n", i, output)
	}

}

type GenesisAlloc struct {
	Balance string `json:"balance"`
}

func recoverPrivateKeyAndWrite() {
	keystoreDir := "./data/keystore"        // keystore文件夹路径
	password := ""                          // 用于解锁keystore文件的密码
	privateKeysFile := "./privateKeys.json" // 输出私钥的json文件路径
	genesisFile := "./genesis.json"         // 输出genesis的json文件路径

	// 读取keystore目录下的所有文件
	files, err := ioutil.ReadDir(keystoreDir)
	if err != nil {
		fmt.Println("Error reading keystore directory:", err)
		return
	}

	privateKeys := make(map[string]string)
	genesisAlloc := make(map[string]GenesisAlloc)

	// 遍历文件，恢复并保存私钥
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		keyfilePath := filepath.Join(keystoreDir, file.Name())

		keyjson, err := ioutil.ReadFile(keyfilePath)
		if err != nil {
			fmt.Printf("Error reading key file %s: %v\n", keyfilePath, err)
			continue
		}

		key, err := keystore.DecryptKey(keyjson, password)
		if err != nil {
			fmt.Printf("Error decrypting key %s: %v\n", keyfilePath, err)
			continue
		}

		privateKey := key.PrivateKey
		address := key.Address.Hex()[2:] // 去掉前缀
		prikeyHex := fmt.Sprintf("%x", privateKey.D)
		// 排除掉不合法的私钥
		if len(prikeyHex) != 64 {
			fmt.Printf("priavteKey has invalid len. skip this one. privateKey: %x\n", prikeyHex)
			continue
		}
		privateKeys[address] = prikeyHex
		genesisAlloc[address] = GenesisAlloc{Balance: "1000000000000000000000"}

		fmt.Printf("successfully recover privatekey... current cnt: %v\n", len(privateKeys))
	}

	// 将私钥写入到json文件中
	data, err := json.MarshalIndent(privateKeys, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling private keys:", err)
		return
	}

	if err := ioutil.WriteFile(privateKeysFile, data, 0644); err != nil {
		fmt.Println("Error writing private keys to file:", err)
		return
	}

	// 读取现有的genesis文件
	genesisContent, err := ioutil.ReadFile(genesisFile)
	if err != nil {
		fmt.Println("Error reading genesis file:", err)
		return
	}

	// 解析genesis文件内容
	var genesisJSON map[string]interface{}
	if err := json.Unmarshal(genesisContent, &genesisJSON); err != nil {
		fmt.Println("Error unmarshalling genesis file:", err)
		return
	}

	// 更新alloc字段
	genesisJSON["alloc"] = genesisAlloc

	// 将更新后的genesis内容写回文件
	updatedGenesisContent, err := json.MarshalIndent(genesisJSON, "", "  ")
	if err != nil {
		fmt.Println("Error marshalling updated genesis content:", err)
		return
	}

	if err := ioutil.WriteFile(genesisFile, updatedGenesisContent, 0644); err != nil {
		fmt.Println("Error writing updated genesis content to file:", err)
		return
	}

	fmt.Println("Private keys have been written to", privateKeysFile, "key count", len(privateKeys))
	fmt.Println("Genesis file has been updated at", genesisFile)
}
