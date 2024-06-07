package core

import (
	"go-w3chain/log"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/crypto/sha3"
)

type TimeBeacon struct {
	ShardID    uint32 `json:"shardID" gencodec:"required"`
	Height     uint64 `json:"height" gencodec:"required"`
	BlockHash  string `json:"blockHash" gencodec:"required"`
	TxHash     string `json:"txHash" gencodec:"required"`
	StatusHash string `json:"statusHash" gencodec:"required"`
}

/* 先 ABI 编码，再 Keccak256 哈希*/
func (tb *TimeBeacon) Hash() []byte {
	encoded := tb.AbiEncode()

	hash := sha3.NewLegacyKeccak256()
	hash.Write(encoded)

	res := hash.Sum(nil)
	// log.Debug("TimeBeacon AbiEncode", "shardID", tb.ShardID, "height", tb.Height, "got hash", res)
	return res
}

/* abiEncode 对应的是solidity的abi.encode，而不是abi.encodePacked，
只在特殊情况下，encode和encodePacked两者编码结果才相同 */
func (tb *TimeBeacon) AbiEncode() []byte {
	uint32Ty, e1 := abi.NewType("uint32", "uint32", nil)
	uint64Ty, e2 := abi.NewType("uint64", "uint64", nil)
	stringTy, e3 := abi.NewType("string", "string", nil)
	if e1 != nil || e2 != nil || e3 != nil {
		log.Error("abi.newtype err")
	}

	arguments := abi.Arguments{
		{
			Type: uint32Ty,
		},
		{
			Type: uint64Ty,
		},
		{
			Type: stringTy,
		},
		{
			Type: stringTy,
		},
		{
			Type: stringTy,
		},
	}

	bytes, err := arguments.Pack(
		tb.ShardID,
		tb.Height,
		tb.BlockHash,
		tb.TxHash,
		tb.StatusHash,
	)
	if err != nil {
		log.Error("arguments.pack", "err", err)
	}

	return bytes
}

func (tb *TimeBeacon) AbiEncodeV2() []byte {
	structTy, _ := abi.NewType("tuple", "struct ty", []abi.ArgumentMarshaling{
		{Name: "shardID", Type: "uint32"},
		{Name: "height", Type: "uint64"},
		{Name: "blockHash", Type: "string"},
		{Name: "txHash", Type: "string"},
		{Name: "StatusHash", Type: "string"},
	})

	args := abi.Arguments{
		{Type: structTy},
	}

	bytes, err := args.Pack(tb)
	if err != nil {
		log.Error("arguments.pack", "err", err)
	}

	return bytes
}

type SignedTB struct {
	TimeBeacon
	SeedHeight uint64
	Signers    []common.Address
	Sigs       [][]byte
	Vrfs       [][]byte
}
