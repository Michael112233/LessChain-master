package core

/*
leveldb是一个key-value数据库，所有数据都是以键-值对的形式存储。
key一般与hash相关，value一般是要存储的数据结构的RLP编码。区块存储时将区块头和区块体分开存储。

core/rawdb/accessors_chain.go: func WriteHeader, WriteBody, GetTransaction
core/blockchain.go: rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body())

https://blog.csdn.net/DDFFR/article/details/74517608/
>  深入讲解以太坊的数据存储 https://juejin.cn/post/6844903545938903048
*/

import (
	"fmt"

	// "github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"

	// "github.com/ethereum/go-ethereum/params"
	// "github.com/ethereum/go-ethereum/ethdb"
	// "github.com/ethereum/go-ethereum/consensus/ethash"
	// "github.com/ethereum/go-ethereum/consensus"
	// "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	// "github.com/ethereum/go-ethereum/core/vm"
)

// core/types/block_test.go:makeBenchBlock()
func MakeBenchBlockTest() {
	var (
		txs = make([]*Transaction, 70)
	)
	header := &Header{
		Difficulty: math.BigPow(11, 11),
		Number:     math.BigPow(2, 9),
		Time:       9876543,
	}
	fmt.Println("Difficulty:", header.Difficulty)
	fmt.Println("Nonce:", header.Nonce)

	for i := range txs {
		amount := math.BigPow(2, int64(i))
		tx := NewTransaction(0, common.Address{}, common.Address{}, 1, amount)
		txs[i] = tx
	}
	for _, tx := range txs {
		fmt.Println(tx.Value)
	}

	// block := NewBlock(header, txs, trie.NewStackTrie(nil))
	// return block

}

func newCanonicalTest() {

	// var block Block

	rawdb.NewMemoryDatabase()
	// batch := db.NewBatch()
	// rawdb.WriteBody(batch, block.Hash(), block.NumberU64(), block.Body())
}
