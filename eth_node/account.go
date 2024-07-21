package eth_node

import (
	"crypto/ecdsa"
	"fmt"
	"go-w3chain/log"
	"go-w3chain/utils"
	"path/filepath"

	secp256k1 "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	KeyStoreDir string = "keystore"
	/* 默认账户的名字，同时也是其钱包密码 */
	defaultAccountName string = "account1"
)

/** w3chain中的账户结构，对以太坊的keystore做了简单封装
 * 提供创建账户、对消息哈希进行签名、验证签名的功能
 * 目前一个节点对应一个账户
 */
type W3Account struct {
	privateKey  *ecdsa.PrivateKey
	pubKey      *ecdsa.PublicKey
	accountAddr common.Address
	keyDir      string
}

func NewW3Account(nodeDatadir string) *W3Account {
	_privateKey := newPrivateKey()

	w3Account := &W3Account{
		privateKey: _privateKey,
		keyDir:     filepath.Join(nodeDatadir, KeyStoreDir),
	}
	w3Account.pubKey = &w3Account.privateKey.PublicKey
	w3Account.accountAddr = crypto.PubkeyToAddress(*w3Account.pubKey)
	// fmt.Printf("create addr: %v\n", w3Account.accountAddr)
	return w3Account
}

func newPrivateKey() *ecdsa.PrivateKey {
	// 选择椭圆曲线，这里选择 secp256k1 曲线
	s, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		log.Error("generate private key fail", "err", err)
	}
	privateKey := s.ToECDSA()
	return privateKey
}

func (w3Account *W3Account) SignHash(hash []byte) []byte {
	sig, err := crypto.Sign(hash, w3Account.privateKey)
	if err != nil {
		log.Error("signHashFail", "err", err)
		return []byte{}
	}
	// log.Trace("w3account sign hash", "msgHash", hash, "address", w3Account.accountAddr, "sig", sig)

	return sig
}

/*
	这个方法是被该账户以外的其他账户调用，以验证签名的正确性的

所以不能直接获取公钥和地址，要从签名中恢复
*/
func VerifySignature(msgHash []byte, sig []byte, expected_addr common.Address) bool {
	// 恢复公钥
	pubKeyBytes, err := crypto.Ecrecover(msgHash, sig)
	if err != nil {
		log.Error("ecrecover fail", "err", err)
		// fmt.Printf("ecrecover err: %v\n", err)
	}

	pubkey, err := crypto.UnmarshalPubkey(pubKeyBytes)
	if err != nil {
		log.Error("UnmarshalPubkey fail", "err", err)
		// fmt.Printf("UnmarshalPubkey err: %v\n", err)
	}

	recovered_addr := crypto.PubkeyToAddress(*pubkey)
	return recovered_addr == expected_addr
}

/* 接收一个随机种子，用私钥生成一个随机数输出和对应的证明 */
func (w3Account *W3Account) GenerateVRFOutput(randSeed []byte) *utils.VRFResult {
	// vrfResult := utils.GenerateVRF(w3Account.privateKey, randSeed)
	sig := w3Account.SignHash(randSeed)
	vrfResult := &utils.VRFResult{
		RandomValue: sig,
		Proof:       w3Account.accountAddr[:],
	}
	return vrfResult
}

/* 接收随机数输出和对应证明，用公钥验证该随机数输出是否合法 */
func (w3Account *W3Account) VerifyVRFOutput(vrfResult *utils.VRFResult, randSeed []byte) bool {
	// return utils.VerifyVRF(w3Account.pubKey, randSeed, vrfResult)
	return VerifySignature(randSeed, vrfResult.RandomValue, common.BytesToAddress(vrfResult.Proof))
}

func printAccounts(w3Account *W3Account) {
	log.Info("node account", "keyDir", w3Account.keyDir, "address", w3Account.accountAddr)
	log.Info(fmt.Sprintf("privateKey: %x", crypto.FromECDSA(w3Account.privateKey)))
}

func (w3Account *W3Account) GetAccountAddress() *common.Address {
	return &w3Account.accountAddr
}

/* 下面两个函数是不用以太坊库实现的签名和验证方法 */
// func (w3Account *W3Account) SignHash(hash []byte) []byte {
// 	r, s, err := ecdsa.Sign(rand.Reader, w3Account.privateKey, hash)
// 	if err != nil {
// 		log.Error("signHashFail", "err", err)
// 		return []byte{}
// 	}
// 	signature := append(r.Bytes(), s.Bytes()...)
// 	log.Trace("w3account sign hash", "msgHash", hash, "address", w3Account.accountAddr, "sig", signature)

// 	return signature
// }

// func (w3Account *W3Account) VerifySignature(msgHash []byte, sig []byte) bool {
// 	rBytes := sig[:len(sig)/2]
// 	sBytes := sig[len(sig)/2:]
// 	r := new(big.Int).SetBytes(rBytes)
// 	s := new(big.Int).SetBytes(sBytes)

// 	valid := ecdsa.Verify(w3Account.pubKey, msgHash, r, s)

// 	return valid
// }
