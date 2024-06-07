package beaconChain

import (
	"encoding/hex"
	"fmt"
	"go-w3chain/core"
	"testing"

	"github.com/ethereum/go-ethereum/rlp"
)

func TestTBHash(t *testing.T) {
	tb1 := core.TimeBeacon{
		ShardID:    1,
		Height:     100,
		BlockHash:  "0x111111",
		TxHash:     "0x222222",
		StatusHash: "0x333333",
	}

	encoded := tb1.AbiEncode()
	// encodedV2 := tb1.AbiEncodeV2()

	bytes := tb1.Hash()
	hex := hex.EncodeToString(bytes)
	fmt.Printf("tb1: %v\nencoded: %v\nbytes: %v\nhex: %v\n", tb1, encoded, bytes, hex)
}

func TestRlpBlock(t *testing.T) {
	tb1 := core.TimeBeacon{
		Height:  100,
		ShardID: 1,
		// Initialize other fields accordingly
	}

	confirmedTB1 := &ConfirmedTB{
		TimeBeacon:    tb1,
		ConfirmTime:   1234567890,
		ConfirmHeight: 200,
	}

	tb2 := core.TimeBeacon{
		Height:  200,
		ShardID: 2,
		// Initialize other fields accordingly
	}

	confirmedTB2 := &ConfirmedTB{
		TimeBeacon:    tb2,
		ConfirmTime:   1234567890,
		ConfirmHeight: 300,
	}

	tbBlock := &TBBlock{
		Tbs:    [][]*ConfirmedTB{1: {confirmedTB1}, 2: {confirmedTB2}},
		Time:   1617788742,
		Height: 500,
	}

	// RLP 序列化
	encoded, err := rlp.EncodeToBytes(tbBlock)
	if err != nil {
		fmt.Println("RLP encoding error:", err)
		return
	}

	fmt.Printf("RLP encoded data: %x\n", encoded)

	// RLP 反序列化
	var decoded TBBlock
	err = rlp.DecodeBytes(encoded, &decoded)
	if err != nil {
		fmt.Println("RLP decoding error:", err)
		return
	}

	fmt.Println("Decoded data:", decoded)
}

func TestHashBlock(t *testing.T) {
	block := TBBlock{}
	hash, err := core.RlpHash(block)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Printf("hash = %v", hash)
}
