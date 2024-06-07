package utils

import (
	"crypto/ecdsa"
	"fmt"
	"go-w3chain/log"
	"reflect"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/vechain/go-ecvrf"
)

// VRFResult 包含 VRF 方法的输出结果
type VRFResult struct {
	Proof       []byte // VRF 证明
	RandomValue []byte // 随机数
}

// GenerateVRF 使用私钥进行 VRF 计算
//
// VRF算法：采用第三方库ecvrf
func GenerateVRF(privateKey *ecdsa.PrivateKey, input []byte) *VRFResult {
	output, proof, err := ecvrf.Secp256k1Sha256Tai.Prove(privateKey, input)
	if err != nil {
		log.Error("GenerateVRF fail", "err", err)
	}
	return &VRFResult{
		Proof:       proof,
		RandomValue: output,
	}
}

// VerifyVRF 使用公钥进行 VRF 结果验证
//
// VRF算法：采用第三方库ecvrf
func VerifyVRF(publicKey *ecdsa.PublicKey, input []byte, vrfResult *VRFResult) bool {
	output, err := ecvrf.Secp256k1Sha256Tai.Verify(publicKey, input, vrfResult.Proof)
	if err != nil {
		log.Error("VerifyVRF fail", "err", err)
	}

	return reflect.DeepEqual(output, vrfResult.RandomValue)
}

func test() {
	// 选择椭圆曲线，这里选择 secp256k1 曲线
	s, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		log.Error("generate private key fail", "err", err)
	}
	privateKey := s.ToECDSA()

	for i := 0; i < 3; i++ {
		// 构造输入数据
		inputData := []byte("This is some input data.")

		// 进行 VRF 计算
		vrfResult := GenerateVRF(privateKey, inputData)

		// 输出 VRF 结果
		fmt.Printf("VRF Proof: %x\n", vrfResult.Proof)
		fmt.Printf("Random Value: %x\n", vrfResult.RandomValue)

		// 验证 VRF 结果
		isValid := VerifyVRF(&privateKey.PublicKey, inputData, vrfResult)
		fmt.Println("VRF Verification:", isValid)
	}
}
