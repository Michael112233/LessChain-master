package committee

import (
	"bytes"
	"fmt"
	"go-w3chain/core"
	"go-w3chain/data"
	"go-w3chain/log"
	"go-w3chain/node"
	"go-w3chain/utils"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"

	myTrie "go-w3chain/trie"
)

var (
	emptyCodeHash = crypto.Keccak256(nil)
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

// func utils.GetHash(val []byte) []byte {
// 	hasher := sha3.NewLegacyKeccak256()
// 	hasher.Write(val)
// 	return hasher.Sum(nil)
// }

func getStateDBAndTxs(txNum int) (*state.StateDB, common.Hash, *trie.SecureTrie, []*core.Transaction) {
	log.SetLogInfo(log.Lvl(4), "testMerkle.log")

	// 创建stateDB
	node := &node.Node{}
	db, err := node.OpenDatabase("chaindata", 0, 0, "", false)
	defer node.Close()
	if err != nil {
		log.Error("open database fail", err)
	}
	stateDB, err := state.New(common.Hash{}, state.NewDatabase(db), nil)
	if err != nil {
		log.Error("new stateDB fail", err)
	}

	// 读取交易数据
	txs := data.LoadETHData("../data/len3_data.csv", txNum)
	initValue := new(big.Int)
	initValue.SetString("10000000000", 10)
	// 初始化发送账户状态
	for _, tx := range txs {
		stateDB.SetBalance(*tx.Sender, initValue)
		if curValue := stateDB.GetBalance(*tx.Sender); curValue.Cmp(initValue) != 0 {
			log.Error("Opps, something wrong!", "curValue", curValue, "Set initValue", initValue)
		}
	}

	database := stateDB.Database().TrieDB()

	root := stateDB.IntermediateRoot(false)
	log.Debug("stateTrie rootHash", "data", root)
	// 将更改写入到数据库中
	stateDB.Commit(false)
	// stateDB中用的是secureTrie，所以要创建secureTrie实例，而不是Trie
	trie, err := trie.NewSecure(root, database)
	if err != nil {
		log.Error("trie.NewSecure error", "err", err, "trieRoot", root)
	}

	return stateDB, root, trie, txs
}

/* 测试由部分账户的merkle proof重建根哈希
只考虑一个账户的proof获取和修改
*/
func TestRebuildTrieFromOneProof(t *testing.T) {
	stateDB, trieRoot, sTrie, txs := getStateDBAndTxs(2)

	address := *txs[0].Sender
	accountState := getStateAccount(stateDB, address)
	encodedBytes, err := rlp.EncodeToBytes(accountState)
	if err != nil {
		log.Error("rlp encode err", "err", err)
	}
	// hash := utils.GetHash(encodedBytes)
	// log.Debug("rlpEncode accountState", "encodedBytes", encodedBytes, "hash", hash)
	log.Debug(fmt.Sprintf("txs[0].Sender  address: %x  hashOfaddress: %x  rlpEncodedOfStateAccount: %x", address, utils.GetHash(address[:]), encodedBytes))
	log.Debug(fmt.Sprintf("txs[0].Sender  stateAccount: %v", accountState))
	// 获得账户的证明
	memDB := memorydb.New()
	if err := sTrie.Prove(utils.GetHash(address.Bytes()), 0, memDB); err != nil {
		log.Error("Failed to prove for address", "address", address.Hex(), "err", err)
	}

	// 逐个从proof中解析出节点
	var proof [][]byte
	hash2Node := make(map[string]myTrie.Node)
	it := memDB.NewIterator(nil, nil)
	log.Debug("proof for this address")
	for it.Next() {
		proof = append(proof, it.Value())
		log.Debug(fmt.Sprintf("proof hash: %x  proof value (encode of node): %x", it.Key(), it.Value()))
		encodedNode := it.Value()
		hash := utils.GetHash(encodedNode)
		node := myTrie.MustDecodeNode(hash, encodedNode)
		hash2Node[string(hash[:])] = node
		switch node.(type) {
		case *myTrie.FullNode:
			fullNode := node.(*myTrie.FullNode)
			log.Debug(fmt.Sprintf("node type: %v  data: %v", "fullnode", fullNode.String()))
			// _, ok := fullNode.Children[6].(myTrie.HashNode)
			// log.Debug((fmt.Sprintf("fullnode's not nil child is hashnode: %v", ok)))
			log.Debug(fmt.Sprintf("encode of fullnode: %x", myTrie.NodeToBytes(fullNode)))
		case *myTrie.ShortNode:
			shortNode := node.(*myTrie.ShortNode)
			log.Debug(fmt.Sprintf("node type: %v  key (nibble): %v  value: %v", "shortnode", shortNode.Key, shortNode.Val))
			_, ok := shortNode.Val.(myTrie.ValueNode)
			log.Debug((fmt.Sprintf("shortnode's val is valuenode: %v", ok)))
			// 编码前要将key由nibble变回正常的字节
			// shortNode.Key = myTrie.HexToCompact(shortNode.Key)
			// log.Debug(fmt.Sprintf("encode of shortnode: %x", myTrie.NodeToBytes(shortNode)))
		case myTrie.ValueNode:
			log.Debug(fmt.Sprintf("node type: %v  data: %v", "valuenode", node))
		case myTrie.HashNode:
			log.Debug(fmt.Sprintf("node type: %v  data: %v", "hashnode", node))
		}
	}

	// rebuild过程中如果找到一个shortnode的val是valnode，则由其沿路组合出的key恢复出账户哈希，进而恢复账户
	// 看看以下map中是否有相关的键值对表示账户已被更新，若被更新则需重新计算rlp编码并替换val
	// 注意沿路组合出的key是nibble数组，需先转化为普通字节数组
	accountUpdateMap := make(map[string]*types.StateAccount)

	// 更新账户的值
	accountState.Balance = accountState.Balance.Add(accountState.Balance, big.NewInt(50000))
	accountUpdateMap[string(utils.GetHash(address[:]))] = accountState

	// for k, v := range accountUpdateMap {
	// 	log.Debug(fmt.Sprintf("accountUpdatedMap key: %x  value: %v", k, v))
	// }

	updatedTrieRoot := rebuildTrie(trieRoot, hash2Node, accountUpdateMap)
	log.Debug(fmt.Sprintf("updated trie root (by rebuild trie): %x", updatedTrieRoot))

	// 与stateDB上直接更新后的trieRoot比较
	stateDB.AddBalance(address, big.NewInt(50000))
	updatedTrieRoot2 := stateDB.IntermediateRoot(false)
	log.Debug(fmt.Sprintf("updated trie root (by stateDB): %x", updatedTrieRoot2))
	// 将更改写入到数据库中
	stateDB.Commit(false)

	// shard.IterateOverTrie(stateDB)

	log.Debug(fmt.Sprintf("address new value: %v", stateDB.GetBalance(address)))

}

/* 测试由部分账户的merkle proof重建根哈希
考虑多个账户的proof获取和修改
*/
func TestRebuildTrieFromProofs(t *testing.T) {
	stateDB, trieRoot, sTrie, txs := getStateDBAndTxs(100)

	packTxs := txs[:10]

	addrsMap := make(map[common.Address]struct{})
	for _, tx := range packTxs {
		log.Debug(fmt.Sprintf("%v", tx))
		addrsMap[*tx.Sender] = struct{}{}
		// addrsMap[*tx.Recipient] = struct{}{}
	}
	addrs := make([]common.Address, 0, len(addrsMap))
	for addr, _ := range addrsMap {
		addrs = append(addrs, addr)
	}
	log.Debug(fmt.Sprintf("len(addrs): %v", len(addrs)))

	hash2Node := make(map[string]myTrie.Node)
	for _, address := range addrs {
		// 获得账户的证明
		memDB := memorydb.New()
		if err := sTrie.Prove(utils.GetHash(address.Bytes()), 0, memDB); err != nil {
			log.Error("Failed to prove for address", "address", address.Hex(), "err", err)
		}

		// 逐个从proof中解析出节点
		var proof [][]byte
		it := memDB.NewIterator(nil, nil)
		log.Debug("")
		log.Debug(fmt.Sprintf("proof for address: %x", address))
		for it.Next() {
			proof = append(proof, it.Value())
			log.Debug(fmt.Sprintf("proof hash: %x  proof value (encode of node): %x", it.Key(), it.Value()))
			encodedNode := it.Value()
			hash := utils.GetHash(encodedNode)
			if _, ok := hash2Node[string(hash[:])]; ok { // 已经解析和存储过该node
				continue
			}
			node := myTrie.MustDecodeNode(hash, encodedNode)
			hash2Node[string(hash[:])] = node
			switch node.(type) {
			case *myTrie.FullNode:
				fullNode := node.(*myTrie.FullNode)
				log.Debug(fmt.Sprintf("node type: %v  data: %v", "fullnode", fullNode.String()))
				// _, ok := fullNode.Children[6].(myTrie.HashNode)
				// log.Debug((fmt.Sprintf("fullnode's not nil child is hashnode: %v", ok)))
				// log.Debug(fmt.Sprintf("encode of fullnode: %x", myTrie.NodeToBytes(fullNode)))
			case *myTrie.ShortNode:
				shortNode := node.(*myTrie.ShortNode)
				log.Debug(fmt.Sprintf("node type: %v  key (nibble): %v  value: %v", "shortnode", shortNode.Key, shortNode.Val))
				// _, ok := shortNode.Val.(myTrie.ValueNode)
				// log.Debug((fmt.Sprintf("shortnode's val is valuenode: %v", ok)))
				// 编码前要将key由nibble变回正常的字节
				// shortNode.Key = myTrie.HexToCompact(shortNode.Key)
				// log.Debug(fmt.Sprintf("encode of shortnode: %x", myTrie.NodeToBytes(shortNode)))
			case myTrie.ValueNode:
				log.Debug(fmt.Sprintf("node type: %v  data: %v", "valuenode", node))
			case myTrie.HashNode:
				log.Debug(fmt.Sprintf("node type: %v  data: %v", "hashnode", node))
			}
		}
	}

	// rebuild过程中如果找到一个shortnode的val是valnode，则由其沿路组合出的key恢复出账户哈希，进而恢复账户
	// 看看以下map中是否有相关的键值对表示账户已被更新，若被更新则需重新计算rlp编码并替换val
	// 注意沿路组合出的key是nibble数组，需先转化为普通字节数组
	accountUpdateMap := make(map[string]*types.StateAccount)

	addr2State := make(map[common.Address]*types.StateAccount)
	for _, tx := range packTxs {
		addr2State[*tx.Sender] = getStateAccount(stateDB, *tx.Sender)
	}

	// 更新账户的值
	for _, tx := range packTxs {
		addr := tx.Sender
		accountState := addr2State[*addr]
		accountState.Balance = accountState.Balance.Add(accountState.Balance, big.NewInt(50000))
		accountUpdateMap[string(utils.GetHash(addr[:]))] = accountState
	}

	// for k, v := range accountUpdateMap {
	// 	log.Debug(fmt.Sprintf("accountUpdatedMap key: %x  value: %v", k, v))
	// }

	updatedTrieRoot := rebuildTrie(trieRoot, hash2Node, accountUpdateMap)
	log.Debug(fmt.Sprintf("updated trie root (by rebuild trie): %x", updatedTrieRoot))

	// 与stateDB上直接更新后的trieRoot比较
	for _, tx := range packTxs {
		addr := tx.Sender
		stateDB.AddBalance(*addr, big.NewInt(50000))
	}
	updatedTrieRoot2 := stateDB.IntermediateRoot(false)
	log.Debug(fmt.Sprintf("updated trie root (by stateDB): %x", updatedTrieRoot2))
	// 将更改写入到数据库中
	stateDB.Commit(false)
}

func getStateAccount(stateDB *state.StateDB, address common.Address) *types.StateAccount {
	// 没法直接获得 stateObject.data，只能分别获取data的内容后自己构造
	balance := stateDB.GetBalance(address)
	nonce := stateDB.GetNonce(address)
	accountState := &types.StateAccount{
		Nonce:    nonce,
		Balance:  new(big.Int).Set(balance), //这里必须作拷贝，否则会更改到stateDB中的值
		Root:     emptyRoot,
		CodeHash: emptyCodeHash,
	}
	return accountState
}

/* 测试账户merkle proof的生成与验证
 */
func TestMerkle(t *testing.T) {
	stateDB, rootHash, sTrie, txs := getStateDBAndTxs(10)

	address := *txs[0].Sender
	log.Debug("test address", "data", address)

	accountState := getStateAccount(stateDB, address)
	log.Debug("original accountState", "data", accountState)

	encodedBytes, err := rlp.EncodeToBytes(accountState)
	if err != nil {
		log.Error("rlp encode err", "err", err)
	}
	hash := utils.GetHash(encodedBytes)
	log.Debug("rlpEncode accountState", "encodedBytes", encodedBytes, "hash", hash)

	// 确认下地址是否已被写到树中
	val := sTrie.Get(address[:])
	log.Debug("sTrie.Get(address[:])", "value", val)

	// 获得账户的证明
	memDB := memorydb.New()
	if err := sTrie.Prove(utils.GetHash(address.Bytes()), 0, memDB); err != nil {
		log.Error("Failed to prove for address", "address", address.Hex(), "err", err)
	}

	// 打印证明
	var proof [][]byte
	it := memDB.NewIterator(nil, nil)
	log.Debug("proof for this address")
	for it.Next() {
		proof = append(proof, it.Value())
		log.Debug(fmt.Sprintf("key: %v  value: %v", it.Key()[:], it.Value()[:]))
	}

	// ---------  假设中间proof通过网络传输  -----------

	// 验证proof，proofReader查找key时其实是将每个proof中的value进行哈希再与key对比，
	// 所以proof中不需要包含key
	proofDB := &proofReader{proof: proof}

	computedValue, err := trie.VerifyProof(rootHash, utils.GetHash(address.Bytes()), proofDB)
	if err != nil {
		log.Error("Failed to verify Merkle proof", "err", err, "address", address)
	}

	if !bytes.Equal(computedValue, encodedBytes) {
		log.Error("Merkle proof verification failed for address", "address", address,
			"computedValue", computedValue, "encodedBytes", encodedBytes)
	}

	// 恢复stateAccount对象
	var recoverAccount types.StateAccount
	err = rlp.DecodeBytes(computedValue, &recoverAccount)
	if err != nil {
		log.Error("rlp decode fail", "err", err)
	}

	log.Debug("recover accountState", "data", recoverAccount)

}
