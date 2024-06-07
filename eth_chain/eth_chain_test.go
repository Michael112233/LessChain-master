package eth_chain

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	client       *ethclient.Client
	contractAddr common.Address
	contractABI  *abi.ABI
)

// func TestConnect(t *testing.T) {
// 	var err error
// 	client, err = Connect(7545)
// 	assert.Equal(t, true, err == nil)

// 	id, err := client.NetworkID(context.Background())
// 	assert.Equal(t, true, err == nil)
// 	assert.Equal(t, int64(5777), id.Int64())

// 	// 不知为何，ganache链ID是5777，但与它连接时需要给定链ID是1337，metamask连接时也是设定链ID为1337
// 	SetChainID(1337)
// }

// func TestDeploy(t *testing.T) {
// 	var err error
// 	genesisTBs := make([]ContractTB, 2)
// 	genesisTBs[0] = ContractTB{
// 		ShardID: 0,
// 		Height:  0,
// 	}
// 	genesisTBs[1] = ContractTB{
// 		ShardID:   1,
// 		Height:    0,
// 		BlockHash: "0x111111",
// 	}

// 	contractAddr, contractABI, _, err = DeployContract(client, 2, genesisTBs, 2, 2)
// 	if err != nil {
// 		fmt.Println(err)
// 	}
// 	// fmt.Println(contractAddr)
// 	// fmt.Printf("%v\n", abi)
// 	// assert.Equal(t, true, err == nil)

// 	eventChannel := make(chan *Event, 10)
// 	go SubscribeEvents(7545, contractAddr, eventChannel)
// }

// func TestUseContract(t *testing.T) {
// 	got, err := GetTB(client, contractAddr, contractABI, 1, 0)
// 	assert.Equal(t, true, err == nil)
// 	expected := &ContractTB{
// 		ShardID:   1,
// 		Height:    0,
// 		BlockHash: "0x111111",
// 	}
// 	assert.Equal(t, expected, got)

// 	newTB := &ContractTB{
// 		ShardID:    0,
// 		Height:     1,
// 		StatusHash: "0x123456",
// 	}

// 	sigs := make([][]byte, 2)
// 	signers := make([]common.Address, 2)
// 	err = AddTB(client, contractAddr, contractABI, 2, newTB, sigs, signers)
// 	assert.Equal(t, true, err == nil)

// 	time.Sleep(12 * time.Second)

// 	got, err = GetTB(client, contractAddr, contractABI, 0, 1)
// 	assert.Equal(t, true, err == nil)
// 	assert.Equal(t, newTB, got)

// 	// for i := 0; i < 4; i++ {
// 	// 	time.Sleep(5 * time.Second)
// 	// 	newTB := &ContractTB{
// 	// 		Height:     uint64(i + 2),
// 	// 		ShardID:    0,
// 	// 		StatusHash: fmt.Sprintf("0xaaaa%d", i),
// 	// 	}
// 	// 	err := AddTB(client, contractAddr, contractABI, newTB)
// 	// 	assert.Equal(t, true, err == nil)
// 	// }
// }
