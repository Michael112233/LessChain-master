package pbft

import (
	"go-w3chain/core"
)

type PbftInsideExtraHandleMod interface {
	HandleinPropose() (bool, *core.PbftRequest)
	HandleinPrePrepare(*core.PrePrepare) bool
	HandleinPrepare(*core.Prepare) bool
	HandleinCommit(*core.Commit) bool
	HandleReqestforOldSeq(*core.RequestOldMessage) bool
	HandleforSequentialRequest(*core.SendOldMessage) bool
}
