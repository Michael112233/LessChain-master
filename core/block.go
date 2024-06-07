package core

import (
	// "fmt"
	"encoding/binary"
	"go-w3chain/log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	EmptyRootHash = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Header represents a block Header in the Ethereum blockchain.
type Header struct {
	ParentHash common.Hash    `json:"parentHash"       gencodec:"required"`
	Coinbase   common.Address `json:"miner"            gencodec:"required"`
	Root       common.Hash    `json:"stateRoot"        gencodec:"required"`
	TxHash     common.Hash    `json:"transactionsRoot" gencodec:"required"`
	Difficulty *big.Int       `json:"difficulty"       gencodec:"required"`
	Number     *big.Int       `json:"number"           gencodec:"required"`
	Time       uint64         `json:"timestamp"        gencodec:"required"`
	// pow算法产生的哈希
	MixDigest common.Hash `json:"mixHash"`
	Nonce     BlockNonce  `json:"nonce"`

	// for sharding
	ShardID uint64
}

// 区块体：取消 Uncles  []*Header
type Body struct {
	Transactions []*Transaction
}

// 区块结构
type Block struct {
	Header       *Header
	Transactions Transactions

	// caches
	Hash common.Hash

	// 总难度，从开始区块到本区块（包括本区块）所有的难度的累加
	Td *big.Int
}

// Hash returns the block hash of the Header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *Header) Hash() common.Hash {
	hash, err := rlpHash(h)
	if err != nil {
		log.Error("block Header hash fail.", "err", err)
	}
	return hash
}

// core/types/block.go
func NewBlock(Header *Header, txs []*Transaction, hasher TrieHasher) *Block {
	b := &Block{Header: CopyHeader(Header), Td: new(big.Int)}

	// 设置交易根
	if len(txs) == 0 {
		b.Header.TxHash = EmptyRootHash
	} else {
		// Transactions(txs) 类型转换
		b.Header.TxHash = DeriveSha(Transactions(txs), hasher)
		b.Transactions = make(Transactions, len(txs))
		copy(b.Transactions, txs)
	}

	return b
}

// CopyHeader creates a deep copy of a block Header to prevent side effects from
// modifying a Header variable.
func CopyHeader(h *Header) *Header {
	cpy := *h
	// Difficulty, Number 为指针，所以要深拷贝   *big.Int
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	return &cpy
}

// Hash returns the keccak256 hash of b's Header.
// The hash is computed on the first call and cached thereafter.
func (b *Block) GetHash() common.Hash {
	if b.Hash == (common.Hash{}) {
		b.Hash = b.Header.Hash()
	}
	return b.Hash
}

func (b *Block) Number() *big.Int     { return new(big.Int).Set(b.Header.Number) }
func (b *Block) Difficulty() *big.Int { return new(big.Int).Set(b.Header.Difficulty) }

func (b *Block) NumberU64() uint64 { return b.Header.Number.Uint64() }

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}
func (b *Block) GetHeader() *Header { return CopyHeader(b.Header) }
func (b *Block) Root() common.Hash  { return b.Header.Root }

// Body returns the non-Header content of the block.
func (b *Block) Body() *Body { return &Body{b.Transactions} }

// WithBody returns a new block with the given transaction and uncle contents.
func (b *Block) WithBody(Transactions []*Transaction) *Block {
	block := &Block{
		Header:       CopyHeader(b.Header),
		Transactions: make([]*Transaction, len(Transactions)),
	}
	copy(block.Transactions, Transactions)

	return block
}
