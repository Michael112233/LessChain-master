package committee

import (
	"fmt"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/node"

	"github.com/ethereum/go-ethereum/common"
)

/** 委员会中的节点对信标进行多签名
由委员会的leader发起
*/
func (com *Committee) initMultiSign(tb *core.TimeBeacon, seed common.Hash, height uint64) *core.SignedTB {
	// 发送消息
	r := &core.ComLeaderInitMultiSign{
		Seed:       seed,
		SeedHeight: height,
		Tb:         tb,
	}

	com.multiSignData.Signers = make([]common.Address, 0)
	com.multiSignData.Sigs = make([][]byte, 0)
	com.multiSignData.Vrfs = make([][]byte, 0)
	com.multiSignData.MultiSignDone = make(chan struct{}, 1)
	com.messageHub.Send(core.MsgTypeLeaderInitMultiSign, com.Node.NodeInfo.ComID, r, nil)
	// 等待多签名完成
	select {
	case <-com.multiSignData.MultiSignDone:
		break
	case <-com.worker.exitCh:
		com.worker.exitCh <- struct{}{}
		break
	}

	return &core.SignedTB{
		TimeBeacon: *tb,
		Signers:    com.multiSignData.Signers,
		Sigs:       com.multiSignData.Sigs,
		Vrfs:       com.multiSignData.Vrfs,
		SeedHeight: height,
	}
}

func (com *Committee) HandleMultiSignRequest(request *core.ComLeaderInitMultiSign) {
	seed := request.Seed
	tb := request.Tb

	account := com.Node.GetAccount()

	vrf := account.GenerateVRFOutput(seed[:])
	if !vrfResultIsGood(vrf.RandomValue) {
		return
	}

	reply := &core.MultiSignReply{
		Request:    request,
		PubAddress: *account.GetAccountAddress(),
		Sig:        account.SignHash(tb.Hash()),
		VrfValue:   vrf.RandomValue,
		NodeInfo:   com.Node.GetPbftNode().NodeInfo,
	}

	com.messageHub.Send(core.MsgTypeSendMultiSignReply, com.Node.NodeInfo.ComID, reply, nil)
}

func (com *Committee) HandleMultiSignReply(reply *core.MultiSignReply) {
	com.multiSignLock.Lock()
	defer com.multiSignLock.Unlock()

	if len(com.multiSignData.Signers) >= com.config.MultiSignRequiredNum {
		return
	}

	if !node.VerifySignature(reply.Request.Seed[:], reply.VrfValue, reply.PubAddress) {
		log.Debug(fmt.Sprintf("vrf verification not pass.. nodeID: %d", reply.NodeInfo.NodeID))
		return
	}
	if !vrfResultIsGood(reply.VrfValue) {
		log.Debug(fmt.Sprintf("vrf not good.. nodeID: %d", reply.NodeInfo.NodeID))
		return
	}
	tbHash := reply.Request.Tb.Hash()
	if !node.VerifySignature(tbHash, reply.Sig, reply.PubAddress) {
		log.Debug(fmt.Sprintf("signature verification not pass.. nodeID: %d", reply.NodeInfo.NodeID))
		return
	}
	com.multiSignData.Signers = append(com.multiSignData.Signers, reply.PubAddress)
	com.multiSignData.Sigs = append(com.multiSignData.Sigs, reply.Sig)
	com.multiSignData.Vrfs = append(com.multiSignData.Vrfs, reply.VrfValue)

	if len(com.multiSignData.Sigs) == com.config.MultiSignRequiredNum { // 收到足够签名
		com.multiSignData.MultiSignDone <- struct{}{}
	}

}

/* 判断一个VRF生成的随机数是否满足条件
该方法需与合约的验证方法一致
*/
func vrfResultIsGood(val []byte) bool {
	// 注意，如果是直接用签名方式生成vrf，则最后一个字节只会是0或1
	// return val[0] > 50
	return val[0] >= 0
}
