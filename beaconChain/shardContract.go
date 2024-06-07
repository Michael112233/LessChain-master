package beaconChain

import (
	"go-w3chain/core"
	"go-w3chain/log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

/** 模拟信标链上的一个合约，验证对应分片的信标的多签名
 */
type ShardContract struct {
	shardID int
	/* 一笔调用该合约的交易需要多少位验证者共同签名才会有效 */
	required_validators_num_for_sign int
}

func NewShardContract(shardID int, required int) *ShardContract {
	contract := &ShardContract{
		shardID:                          shardID,
		required_validators_num_for_sign: required,
	}

	return contract
}

/** 验证一个地址是否有资格对该信标进行签名
 * 地址长度：20个字节
 * 未完成
 */
func checkAddressValidity(address common.Address) bool {
	// todo: 验证该地址
	return true
}

func (contract *ShardContract) VerifyTimeBeacon(tb *core.SignedTB) bool {
	msgHash := tb.TimeBeacon.Hash()
	sig_num := 0
	for i := 0; i < len(tb.Signers); i++ {
		sig := tb.Sigs[i]
		signer := tb.Signers[i]
		// 恢复公钥
		pubKeyBytes, err := crypto.Ecrecover(msgHash, sig)
		if err != nil {
			log.Error("shardContract recover Pubkey Fail.", "err", err)
		}

		pubkey, err := crypto.UnmarshalPubkey(pubKeyBytes)
		if err != nil {
			log.Error("UnmarshalPubkey fail.", "err", err)
		}

		recovered_addr := crypto.PubkeyToAddress(*pubkey)
		if !checkAddressValidity(recovered_addr) {
			log.Error("shardContract check address validity fail. This address has no right to sign this time beacon.")
		}
		checkSigPass := recovered_addr == signer
		if checkSigPass {
			sig_num += 1
			if sig_num >= contract.required_validators_num_for_sign {
				// log.Debug("contract verify signedTB... pass.", "shardID", contract.shardID, "# of sigs", len(tb.Sigs), "need", contract.required_validators_num_for_sign)
				return true
			}
		}
	}

	return false
}
