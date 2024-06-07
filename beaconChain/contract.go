package beaconChain

/* 信标链上验证多签名的合约 */
type Contract struct {
	contracts []*ShardContract
}

func NewContract(shardNum int, required int) *Contract {
	contracts := make([]*ShardContract, shardNum)
	for i := 0; i < shardNum; i++ {
		contracts[i] = NewShardContract(i, required)
	}

	return &Contract{
		contracts: contracts,
	}
}
