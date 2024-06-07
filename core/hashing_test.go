package core

import (
	"testing"
)

type TimeBeaconValid struct {
	Height  uint64 `json:"height" gencodec:"required"`
	ShardID uint32 `json:"shardID" gencodec:"required"`
}

// 结构体成员变量不能是int类型，否则rlp编码会失败
type TimeBeaconInvalid struct {
	Height  uint64 `json:"height" gencodec:"required"`
	ShardID int    `json:"shardID" gencodec:"required"`
}

func TestRlpHashInvalid(t *testing.T) {
	for i := 1; i < 5; i++ {
		tb := &TimeBeaconInvalid{
			Height:  uint64(i),
			ShardID: 100,
		}
		hash, err := RlpHash(*tb)
		if err != nil {
			t.Error("hash=", hash, "err=", err)
		}
	}
}

func TestRlpHash(t *testing.T) {
	for i := 1; i < 5; i++ {
		tb := &TimeBeaconValid{
			Height:  uint64(i),
			ShardID: 100,
		}
		hash, err := RlpHash(*tb)
		if err != nil {
			t.Error("hash=", hash, "err=", err)
		}
	}
}
