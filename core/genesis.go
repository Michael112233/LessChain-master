package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"

	"go-w3chain/cfg"
	"go-w3chain/log"
	"go-w3chain/utils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
)

var errGenesisNoConfig = errors.New("genesis has no chain configuration")

var DefaultAlloc = &GenesisAlloc{
	{1}: {Balance: big.NewInt(1), Storage: map[common.Hash]common.Hash{{1}: {1}}},
	{2}: {Balance: big.NewInt(2), Storage: map[common.Hash]common.Hash{{2}: {2}}},
}

type Genesis struct {
	Config *cfg.ChainConfig `json:"config"` // chainID ans so on

	Nonce      uint64         `json:"nonce"`
	Timestamp  uint64         `json:"timestamp"`
	Difficulty *big.Int       `json:"difficulty" gencodec:"required""` //
	Mixhash    common.Hash    `json:"mixHash"`
	Coinbase   common.Address `json:"coinbase"`
	Alloc      GenesisAlloc   `json:"alloc"      gencodec:"required"` // the initial state

	// These fields are used for consensus tests. Please don't use them
	// in actual genesis blocks.
	Number     uint64      `json:"number"`
	ParentHash common.Hash `json:"parentHash"`
}

// GenesisAlloc specifies the initial state that is part of the genesis block.
type GenesisAlloc map[common.Address]GenesisAccount

func (ga *GenesisAlloc) UnmarshalJSON(data []byte) error {
	m := make(map[common.UnprefixedAddress]GenesisAccount)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*ga = make(GenesisAlloc)
	for addr, a := range m {
		(*ga)[common.Address(addr)] = a
	}
	return nil
}

// flush adds allocated genesis accounts into a fresh new statedb and
// commit the state changes into the given database handler.
func (ga *GenesisAlloc) flush(db ethdb.Database) (common.Hash, error) {
	// creates a new state from a given trie.
	statedb, err := state.New(common.Hash{}, state.NewDatabase(db), nil)
	if err != nil {
		return common.Hash{}, err
	}
	for addr, account := range *ga {
		statedb.AddBalance(addr, account.Balance)
		statedb.SetCode(addr, account.Code)
		statedb.SetNonce(addr, account.Nonce)
		for key, value := range account.Storage {
			statedb.SetState(addr, key, value)
		}
	}
	root, err := statedb.Commit(false)
	if err != nil {
		return common.Hash{}, err
	}
	err = statedb.Database().TrieDB().Commit(root, true, nil)
	if err != nil {
		return common.Hash{}, err
	}
	return root, nil
}

// GenesisAccount is an account in the state of the genesis block.
type GenesisAccount struct {
	Code       []byte                      `json:"code,omitempty"`
	Storage    map[common.Hash]common.Hash `json:"storage,omitempty"`
	Balance    *big.Int                    `json:"balance" gencodec:"required"`
	Nonce      uint64                      `json:"nonce,omitempty"`
	PrivateKey []byte                      `json:"secretKey,omitempty"` // for tests
}

func CreateGenesisAllocJson() {
	var (
		alloc = DefaultAlloc
	)

	// blob, _ := json.Marshal(alloc)   // 不带缩进
	// 带缩进
	blob, err := json.MarshalIndent(alloc, "", "     ")
	if err != nil {
		utils.Fatalf("Failed to transalate GenesisAlloc to JSON: %v", err)
	}

	err = ioutil.WriteFile("genesisAlloc.json", blob, os.ModeAppend)
	if err != nil {
		utils.Fatalf("Failed to generate genesis.json: %v", err)
	}

	jsonFile, err := os.Open("genesisAlloc.json")
	defer jsonFile.Close()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		utils.Fatalf("Failed to read genesisAlloc.json: %v", err)
	}
	var reload GenesisAlloc
	reload.UnmarshalJSON(byteValue)
	fmt.Println(reload)

}

func CreateGenesisJson() {
	var (
		alloc = DefaultAlloc
	)
	genesis := &Genesis{
		Alloc: *alloc,
	}

	blob, err := json.MarshalIndent(genesis, "", "     ")
	if err != nil {
		utils.Fatalf("Failed to transalate genesis to JSON: %v", err)
	}

	err = ioutil.WriteFile("genesis.json", blob, os.ModeAppend)
	if err != nil {
		utils.Fatalf("Failed to generate genesis.json: %v", err)
	}

}

func LoadGenesisJson() {
	jsonFile, err := os.Open("genesis.json")
	defer jsonFile.Close()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		utils.Fatalf("Failed to read genesis.json: %v", err)
	}

	var reload Genesis
	err = json.Unmarshal(byteValue, &reload)
	if err != nil {
		utils.Fatalf("Failed to unmarshal genesis.json: %v", err)
	}
	fmt.Println(reload)
}

// ToBlock creates the genesis block and writes state of a genesis specification
// to the given database (or discards it if nil).
func (g *Genesis) ToBlock(db ethdb.Database) *Block {
	if db == nil {
		db = rawdb.NewMemoryDatabase()
	}
	root, err := g.Alloc.flush(db)
	if err != nil {
		panic(err)
	}

	head := &Header{
		ParentHash: g.ParentHash,
		Coinbase:   g.Coinbase,
		Root:       root,
		// TxHash
		Difficulty: g.Difficulty,
		Number:     new(big.Int).SetUint64(g.Number),
		Time:       g.Timestamp,

		MixDigest: g.Mixhash,
		Nonce:     EncodeNonce(g.Nonce),

		// ShardID
	}

	if g.Difficulty == nil && g.Mixhash == (common.Hash{}) {
		head.Difficulty = cfg.GenesisDifficulty
	}

	block := NewBlock(head, nil, trie.NewStackTrie(nil))

	// fmt.Println("In func ToBlock, block.header.TxHash:", block.header.TxHash)
	// fmt.Println("In func ToBlock, root:", head.Root)

	return block

}

// write writes the json marshaled genesis state into database
// with the given block hash as the unique identifier.
func (ga *GenesisAlloc) write(db ethdb.KeyValueWriter, hash common.Hash) error {
	blob, err := json.Marshal(ga)
	if err != nil {
		return err
	}
	WriteGenesisState(db, hash, blob)
	return nil
}

// // Commit writes the block and state of a genesis specification to the database.
// // The block is committed as the canonical head block.
func (g *Genesis) Commit(db ethdb.Database) (*Block, error) {
	block := g.ToBlock(db)
	// 看 block.Number() 的符号， >0 返回1，<0返回-1， 0返回0
	if block.Number().Sign() != 0 {
		return nil, errors.New("can't commit genesis block with number > 0")
	}

	config := g.Config
	if config == nil {
		config = cfg.AllProtocolChanges
	}

	if err := g.Alloc.write(db, block.GetHash()); err != nil {
		return nil, err
	}

	// stores the total difficulty ， 创世区块 就是 block.Difficulty()
	WriteTd(db, block.GetHash(), block.NumberU64(), block.Difficulty())
	WriteBlock(db, block)
	WriteCanonicalHash(db, block.GetHash(), block.NumberU64())
	WriteHeadBlockHash(db, block.GetHash())
	WriteHeadFastBlockHash(db, block.GetHash())
	WriteHeadHeaderHash(db, block.GetHash())
	WriteChainConfig(db, block.GetHash(), config)

	// fmt.Println("In func Commit, success ")

	return block, nil
}

func SetupGenesisBlock(db ethdb.Database, genesis *Genesis) (*cfg.ChainConfig, common.Hash, error) {
	// wrong case:
	if genesis != nil && genesis.Config == nil {
		return cfg.AllProtocolChanges, common.Hash{}, errGenesisNoConfig
	}

	// case 1: Just commit the new block if there is no stored genesis block.
	stored := ReadCanonicalHash(db, 0)
	if (stored == common.Hash{}) {
		if genesis == nil {
			log.Info("Writing default shard #1 genesis block")
			genesis = DefaultGenesisBlock()
		} else {
			log.Info("Writing custom genesis block")
		}
		block, err := genesis.Commit(db)
		if err != nil {
			return genesis.Config, common.Hash{}, err
		}
		return genesis.Config, block.GetHash(), nil
	}

	// case 2: We have the genesis block in database(perhaps in ancient database)
	// but the corresponding state is missing.
	header := ReadHeader(db, stored, 0)
	if _, err := state.New(header.Root, state.NewDatabaseWithConfig(db, nil), nil); err != nil {
		if genesis == nil {
			genesis = DefaultGenesisBlock()
		}
		// Ensure the stored genesis matches with the given one.
		hash := genesis.ToBlock(nil).GetHash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
		block, err := genesis.Commit(db)
		if err != nil {
			return genesis.Config, hash, err
		}
		return genesis.Config, block.GetHash(), nil
	}

	// case 3: db 已有 genesis, state 也存在, stored 不为空
	// 检查 当前的 genesis 是否 与 db 的匹配
	if genesis != nil {
		hash := genesis.ToBlock(nil).GetHash()
		if hash != stored {
			return genesis.Config, hash, &GenesisMismatchError{stored, hash}
		}
	}

	// case 4: db 存在 genesis , state 存在 ，stored 不为空
	// 但 db 的 storedcfg 为空
	newcfg := genesis.configOrDefault(stored)
	storedcfg := ReadChainConfig(db, stored)
	if storedcfg == nil {
		log.Warn("Found genesis block without chain config")
		WriteChainConfig(db, stored, newcfg)
		return newcfg, stored, nil
	}

	// case 5: db 存在 genesis , state 存在.  storedcfg 不为空
	// genesis 为空,  则以本地的db为准。否则更换 本地genesis 的 chainID
	if genesis == nil {
		newcfg = storedcfg
	}
	// 检查  height
	height := ReadHeaderNumber(db, ReadHeadHeaderHash(db))
	if height == nil {
		return newcfg, stored, fmt.Errorf("missing block number for head header hash")
	}
	WriteChainConfig(db, stored, newcfg)
	log.Info("Change chainConfig")
	return newcfg, stored, nil
}

// GenesisMismatchError is raised when trying to overwrite an existing
// genesis block with an incompatible one.
type GenesisMismatchError struct {
	Stored, New common.Hash
}

func (e *GenesisMismatchError) Error() string {
	return fmt.Sprintf("database contains incompatible genesis (have %x, new %x)", e.Stored, e.New)
}

// DefaultGenesisBlock returns the Ethereum main net genesis block.
func DefaultGenesisBlock() *Genesis {
	return &Genesis{
		Config:     cfg.AllProtocolChanges,
		Nonce:      66,
		Difficulty: big.NewInt(17179869184),
		Alloc:      *DefaultAlloc,
	}
}

// MustCommit writes the genesis block and state to db, panicking on error.
// The block is committed as the canonical head block.
func (g *Genesis) MustCommit(db ethdb.Database) *Block {
	block, err := g.Commit(db)
	if err != nil {
		panic(err)
	}
	return block
}

func (g *Genesis) configOrDefault(ghash common.Hash) *cfg.ChainConfig {
	switch {
	case g != nil:
		return g.Config
	default:
		return cfg.AllProtocolChanges
	}
}
