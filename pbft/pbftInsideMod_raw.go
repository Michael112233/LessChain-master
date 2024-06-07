// addtional module for new consensus
package pbft

import (
	"go-w3chain/core"
)

// simple implementation of pbftHandleModule interface ...
// only for block request and use transaction relay
type RawPbftInsideExtraHandleMod struct {
	pbftNode *PbftConsensusNode
	// pointer to pbft data
}

// propose request with different types
func (rphm *RawPbftInsideExtraHandleMod) HandleinPropose() (bool, *core.PbftRequest) {
	return true, nil
}

// the diy operation in preprepare
func (rphm *RawPbftInsideExtraHandleMod) HandleinPrePrepare(ppmsg *core.PrePrepare) bool {
	return true
}

// the operation in prepare, and in pbft + tx relaying, this function does not need to do any.
func (rphm *RawPbftInsideExtraHandleMod) HandleinPrepare(pmsg *core.Prepare) bool {
	return true
}

// the operation in commit.
func (rphm *RawPbftInsideExtraHandleMod) HandleinCommit(cmsg *core.Commit) bool {
	return true
}

func (rphm *RawPbftInsideExtraHandleMod) HandleReqestforOldSeq(*core.RequestOldMessage) bool {
	return true
}

// the operation for sequential requests
func (rphm *RawPbftInsideExtraHandleMod) HandleforSequentialRequest(som *core.SendOldMessage) bool {
	return true
}
