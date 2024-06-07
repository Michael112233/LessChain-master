package pbft

import (
	"bytes"
	"encoding/gob"
	"go-w3chain/core"
	"go-w3chain/log"
	"go-w3chain/utils"
)

// set 2d map, only for pbft maps, if the first parameter is true, then set the cntPrepareConfirm map,
// otherwise, cntCommitConfirm map will be set
func (p *PbftConsensusNode) set2DMap(isPrePareConfirm bool, key string, val *core.NodeInfo) {
	if isPrePareConfirm {
		if _, ok := p.cntPrepareConfirm[key]; !ok {
			p.cntPrepareConfirm[key] = make(map[*core.NodeInfo]bool)
		}
		p.cntPrepareConfirm[key][val] = true
	} else {
		if _, ok := p.cntCommitConfirm[key]; !ok {
			p.cntCommitConfirm[key] = make(map[*core.NodeInfo]bool)
		}
		p.cntCommitConfirm[key][val] = true
	}
}

// get the digest of request
func getDigest(r *core.PbftRequest) []byte {
	data := r
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	// 如果直接编码data的话，发送端和接收端对r编码得到的结果会不同
	err := enc.Encode(data.MsgType)
	if err != nil {
		log.Error("gobEncodeErr", "err", err)
	}
	err = enc.Encode(data.Msg)
	if err != nil {
		log.Error("gobEncodeErr", "err", err)
	}
	err = enc.Encode(data.ReqTime)
	if err != nil {
		log.Error("gobEncodeErr", "err", err)
	}

	encodedMsg := buf.Bytes()
	return utils.GetHash(encodedMsg)
}
