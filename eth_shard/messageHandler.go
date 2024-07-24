package eth_shard

import (
	"fmt"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/utils"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	"golang.org/x/crypto/sha3"
)

var (
	emptyCodeHash = crypto.Keccak256(nil)
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

func getHash(val []byte) []byte {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(val)
	return hasher.Sum(nil)
}

func (s *Shard) HandleComGetState(request *core.ComGetState) *core.ShardSendState {
	stateDB := s.blockchain.GetStateDB()
	// 先看看stateDB中有没有对应账户的节点，没有则先创建节点并更新trie
	for _, address := range request.AddrList {
		stateDB.GetOrNewStateObject(address)
	}

	// 获取最新的状态树
	root := stateDB.IntermediateRoot(false)
	stateDB.Commit(false)
	database := stateDB.Database().TrieDB()
	trie, err := trie.NewSecure(root, database)
	if err != nil {
		log.Error("trie.NewSecure error", "err", err, "trieRoot", root)
		return nil
	}

	accountsData := make(map[common.Address][]byte)
	accountsProofs := make(map[common.Address][][]byte)
	for _, address := range request.AddrList {
		// 获取状态对象的数据
		accountState := &types.StateAccount{
			Nonce:    stateDB.GetNonce(address),
			Balance:  stateDB.GetBalance(address),
			Root:     emptyRoot,
			CodeHash: emptyCodeHash,
		}

		enc, err := rlp.EncodeToBytes(accountState)
		if err != nil {
			log.Error("Failed to encode state object", "err", err)
		}
		accountsData[address] = enc

		// 生成merkle proof
		memDB := memorydb.New()
		if err := trie.Prove(getHash(address.Bytes()), 0, memDB); err != nil {
			log.Error("Failed to prove for address", "address", address.Hex(), "err", err)
		}

		var proofs [][]byte
		it := memDB.NewIterator(nil, nil)
		for it.Next() {
			proofs = append(proofs, it.Value())
		}

		accountsProofs[address] = proofs
	}

	response := &core.ShardSendState{
		StatusTrieHash: root,
		AccountData:    accountsData,
		AccountsProofs: accountsProofs,
		Height:         s.blockchain.CurrentBlock().Number(),
	}

	return response
}

func (s *Shard) HandleComGetHeight() *big.Int {
	height := s.blockchain.CurrentBlock().Number()
	return height
}

/*
	分片收到区块后，执行其中的交易，并将得到的状态树根与区块中的状态树根比较

若两者相等，说明委员会由merkle proof rebuild得到的树根是正确的
*/
func (s *Shard) HandleComSendBlock(data *core.ComSendBlock) {
	block := data.Block
	s.AddBlock(block)

	trieRoot := s.executeTransactions(block.Transactions)
	if trieRoot != block.Header.Root {
		// 需要考虑到一种可能出错的情况，即分片发送给委员会root之后，由于新创建账户而修改了root
		// 这种情况会在委员会连续向分片获取不同账户列表的状态时发生，应尽量避免
		log.Error(fmt.Sprintf("trie root not the same. trieRoot in shard: %x  trieRoot from committee: %x", trieRoot, block.Header.Root))
	} else {
		log.Debug(fmt.Sprintf("shard execute txs done and verify trie root pass. current trie root: %x", trieRoot))
	}
}

func (s *Shard) HandleGetSyncData(data *core.GetSyncData) *core.SyncData {
	switch data.SyncType {
	case "fastsync": // 只同步状态树和最近区块
		// 状态树
		states := make(map[common.Address]*types.StateAccount)
		stateDB := s.blockchain.GetStateDB()
		for address, _ := range s.activeAddrs {
			accountState := &types.StateAccount{
				Nonce:    stateDB.GetNonce(address),
				Balance:  stateDB.GetBalance(address),
				Root:     emptyRoot,
				CodeHash: emptyCodeHash,
			}
			states[address] = accountState
		}

		// 最近区块
		blocks := s.blockchain.AllBlocks()
		syncBlockNum := utils.Min(len(blocks), s.fastsyncBlockNum)
		blocks2sync := blocks[len(blocks)-syncBlockNum:]

		syncData := &core.SyncData{
			ClientAddr: data.ClientAddr,
			States:     states,
			Blocks:     blocks2sync,
		}

		return syncData

	case "fullsync": // 同步全部区块
		blocks := s.blockchain.AllBlocks()
		syncData := &core.SyncData{
			ClientAddr: data.ClientAddr,
			States:     make(map[common.Address]*types.StateAccount),
			Blocks:     blocks,
		}

		return syncData

	default:
		log.Error("unknown sync type", "type", data.SyncType)
	}
	return nil
}
