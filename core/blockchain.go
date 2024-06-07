package core

import (
	"errors"
	"go-w3chain/cfg"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	lru "github.com/hashicorp/golang-lru"
)

var (
	ErrNoGenesis = errors.New("genesis not found in chain")
	ErrUnknown   = errors.New("Unknown in blockchain")
)

const (
	blockCacheLimit       = 256
	TriesInMemoryInterval = 128
)

type txURI uint64

type BlockChain struct {
	chainConfig *cfg.ChainConfig // Chain configuration
	cacheConfig *CacheConfig     // Cache configuration for pruning

	db         ethdb.Database
	stateCache state.Database // State database to reuse between imports (contains state cache)

	statedb *state.StateDB // if 从简，直接用statedb

	genesisBlock *Block

	blockCache *lru.Cache // Cache for the most recent entire blocks

	currentBlock atomic.Value // Current head of the block chain

	blocks []*Block
}

// 设置 stateDB 的 config
// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	TrieCleanLimit      int           // Memory allowance (MB) to use for caching trie nodes in memory
	TrieCleanJournal    string        // Disk journal for saving clean cache entries.
	TrieCleanRejournal  time.Duration // Time interval to dump clean cache to disk periodically
	TrieCleanNoPrefetch bool          // Whether to disable heuristic state prefetching for followup blocks
	TrieDirtyLimit      int           // Memory limit (MB) at which to start flushing dirty trie nodes to disk
	TrieDirtyDisabled   bool          // Whether to disable trie write caching and GC altogether (archive node)
	TrieTimeLimit       time.Duration // Time limit after which to flush the current in-memory trie to disk
	SnapshotLimit       int           // Memory allowance (MB) to use for caching snapshot entries in memory
	Preimages           bool          // Whether to store preimage of trie key to the disk

	SnapshotWait bool // Wait for snapshot construction on startup. TODO(karalabe): This is a dirty hack for testing, nuke it
}

// defaultCacheConfig are the default caching values if none are specified by the
// user (also used during testing).
var defaultCacheConfig = &CacheConfig{
	TrieCleanLimit: 256,
	TrieDirtyLimit: 256,
	TrieTimeLimit:  5 * time.Minute,
	SnapshotLimit:  256,
	SnapshotWait:   true,
}

func NewBlockChain(db ethdb.Database, cacheConfig *CacheConfig, chainConfig *cfg.ChainConfig) (*BlockChain, error) {
	if cacheConfig == nil {
		cacheConfig = defaultCacheConfig
	}

	blockCache, _ := lru.New(blockCacheLimit)

	bc := &BlockChain{
		chainConfig: chainConfig,
		cacheConfig: cacheConfig,
		db:          db,
		stateCache: state.NewDatabaseWithConfig(db, &trie.Config{
			Cache:     cacheConfig.TrieCleanLimit,
			Journal:   cacheConfig.TrieCleanJournal,
			Preimages: cacheConfig.Preimages,
		}),
		statedb:    NewStateDB(db),
		blockCache: blockCache,
	}

	bc.genesisBlock = bc.GetBlockByNumber(0)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}

	// var nilBlock *Block
	bc.currentBlock.Store(bc.genesisBlock)

	head := bc.CurrentBlock()
	if _, err := state.New(head.Root(), bc.stateCache, nil); err != nil {
		return nil, ErrUnknown
	}

	bc.blocks = make([]*Block, 0)
	bc.blocks = append(bc.blocks, head)

	return bc, nil
}

func (bc *BlockChain) GetChainHeight() uint64 {
	return bc.CurrentBlock().NumberU64()
}

func (bc *BlockChain) GetStateDB() *state.StateDB {
	return bc.statedb
}

/**
 * 此处应该写入到数据库的，但为了加快程序运行速度未写入，只是将最块存到内存里
 */
func (bc *BlockChain) WriteBlock(block *Block) {
	bc.currentBlock.Store(block)
	bc.blocks = append(bc.blocks, block)
}

func (bc *BlockChain) AllBlocks() []*Block {
	return bc.blocks
}
