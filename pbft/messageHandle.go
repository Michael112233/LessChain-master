package pbft

import (
	"go-w3chain/core"
	"time"

	"github.com/ethereum/go-ethereum/rlp"
)

func (p *PbftConsensusNode) SetSequenceID(id uint64) {
	p.sequenceLock.Lock()
	p.sequenceID = id
	p.sequenceLock.Unlock()
}

// this func is only invoked by main node
func (p *PbftConsensusNode) Propose(block *core.Block) {
	if p.view != p.NodeInfo.NodeID {
		return
	}

	p.sequenceLock.Lock()
	p.pl.Plog.Printf("C%dN%d get sequenceLock locked, now trying to propose... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, p.sequenceID)
	// propose
	// implement interface to generate propose
	p.ihm.HandleinPropose()

	rlp_block, err := rlp.EncodeToBytes(block)
	if err != nil {
		p.pl.Plog.Printf("C%dN%d could not rlp encode block\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)
	}

	r := &core.PbftRequest{
		Msg:     rlp_block,
		ReqTime: time.Now().Unix(),
		MsgType: "*block",
	}

	digest := getDigest(r)
	p.requestPool[string(digest)] = r
	p.pl.Plog.Printf("C%dN%d put the request into the pool ... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, p.sequenceID)

	ppmsg := &core.PrePrepare{
		RequestMsg: r,
		Digest:     digest,
		SeqID:      p.sequenceID,
	}
	p.height2Digest[p.sequenceID] = string(digest)

	// 通过hub，将preprepare消息发送至同委员会内其他节点
	p.messageHub.Send(core.MsgTypePbftPrePrepare, p.NodeInfo.ComID, ppmsg, nil)

}

func (p *PbftConsensusNode) HandlePrePrepare(ppmsg *core.PrePrepare) {
	p.pl.Plog.Printf("received the PrePrepare ... sequenceID: %d\n", ppmsg.SeqID)

	p.lock.Lock()
	defer p.lock.Unlock()

	debug := true
	// 假设只有重组后，原来的非共识节点变为共识节点时才会出现sequenceID不一致的问题
	// 方便起见，直接同步至新的sequenceID
	// 这是不安全的，仅能用于实验调试
	if debug {
		p.sequenceID = ppmsg.SeqID
	}

	flag := false
	if digest := getDigest(ppmsg.RequestMsg); string(digest) != string(ppmsg.Digest) {
		p.pl.Plog.Printf("C%dN%d : the digest is not consistent, so refuse to prepare.\n",
			p.NodeInfo.ComID, p.NodeInfo.NodeID)
	} else if p.sequenceID < ppmsg.SeqID {
		p.requestPool[string(getDigest(ppmsg.RequestMsg))] = ppmsg.RequestMsg
		p.height2Digest[ppmsg.SeqID] = string(getDigest(ppmsg.RequestMsg))
		p.pl.Plog.Printf("C%dN%d : the Sequence id is not consistent, so refuse to prepare... msg's sequenceID: %d local sequenceID: %d\n",
			p.NodeInfo.ComID, p.NodeInfo.NodeID, ppmsg.SeqID, p.sequenceID)
	} else {
		// do your operation in this interface
		flag = p.ihm.HandleinPrePrepare(ppmsg)
		p.requestPool[string(getDigest(ppmsg.RequestMsg))] = ppmsg.RequestMsg
		p.height2Digest[ppmsg.SeqID] = string(getDigest(ppmsg.RequestMsg))
	}
	// if the message is true, broadcast the prepare message
	if flag {
		pre := &core.Prepare{
			Digest:     ppmsg.Digest,
			SeqID:      ppmsg.SeqID,
			SenderInfo: p.NodeInfo,
		}
		p.messageHub.Send(core.MsgTypePbftPrepare, p.NodeInfo.ComID, pre, nil)
		p.pl.Plog.Printf("C%dN%d : has broadcast the prepare message ... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, ppmsg.SeqID)
	}
}

func (p *PbftConsensusNode) HandlePrepare(pmsg *core.Prepare) {
	p.pl.Plog.Printf("C%dN%d : received the Prepare from ...%d sequenceID: %d\n",
		p.NodeInfo.ComID, p.NodeInfo.NodeID, pmsg.SenderInfo.NodeID, pmsg.SeqID)

	p.lock.Lock()
	defer p.lock.Unlock()
	if _, ok := p.requestPool[string(pmsg.Digest)]; !ok {
		p.pl.Plog.Printf("C%dN%d : doesn't have the digest in the requst pool, refuse to commit... sequenceID: %d\n",
			p.NodeInfo.ComID, p.NodeInfo.NodeID, pmsg.SeqID)
	} else if p.sequenceID < pmsg.SeqID {
		p.pl.Plog.Printf("C%dN%d : inconsistent sequence ID, refuse to commit... msg's seqID: %d, local seqID: %d\n",
			p.NodeInfo.ComID, p.NodeInfo.NodeID, pmsg.SeqID, p.sequenceID)
	} else {
		// if needed more operations, implement interfaces

		p.ihm.HandleinPrepare(pmsg)

		p.set2DMap(true, string(pmsg.Digest), pmsg.SenderInfo)
		cnt := 0
		for range p.cntPrepareConfirm[string(pmsg.Digest)] {
			cnt++
		}
		// the main node will not send the prepare message
		specifiedcnt := int(2 * p.malicious_nums)
		if p.NodeInfo.NodeID != p.view {
			specifiedcnt -= 1
		}

		// if the node has received 2f messages (itself included), and it haven't committed, then it commit
		if cnt >= specifiedcnt && !p.isCommitBordcast[string(pmsg.Digest)] {
			p.pl.Plog.Printf("C%dN%d : is going to commit... sequnceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, pmsg.SeqID)
			// generate commit and broadcast
			c := &core.Commit{
				Digest:     pmsg.Digest,
				SeqID:      pmsg.SeqID,
				SenderInfo: p.NodeInfo,
			}

			p.messageHub.Send(core.MsgTypePbftCommit, p.NodeInfo.ComID, c, nil)
			p.isCommitBordcast[string(pmsg.Digest)] = true
			p.pl.Plog.Printf("C%dN%d : commit is broadcast... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, pmsg.SeqID)
		}
	}
}

func (p *PbftConsensusNode) reply(seqID uint64, digest []byte) {
	p.isReply[string(digest)] = true
	if p.NodeInfo.NodeID != p.view {
		reply := &core.Reply{
			MessageID:  seqID,
			SenderInfo: p.NodeInfo,
			Result:     true,
		}
		p.messageHub.Send(core.MsgTypePbftReply, p.NodeInfo.ComID, reply, nil)
	}
}

func (p *PbftConsensusNode) HandleCommit(cmsg *core.Commit) {
	p.pl.Plog.Printf("C%dN%d received the Commit from ...%d sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, cmsg.SenderInfo.NodeID, cmsg.SeqID)

	p.lock.Lock()
	defer p.lock.Unlock()

	p.set2DMap(false, string(cmsg.Digest), cmsg.SenderInfo)
	cnt := 0
	for range p.cntCommitConfirm[string(cmsg.Digest)] {
		cnt++
	}

	required_cnt := int(2 * p.malicious_nums)
	if cnt >= required_cnt && !p.isReply[string(cmsg.Digest)] {
		p.pl.Plog.Printf("C%dN%d : has received 2f + 1 commits ... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, cmsg.SeqID)
		// if this node is left behind, so it need to requst blocks
		// if _, ok := p.requestPool[string(cmsg.Digest)]; !ok {
		// 	// p.isReply[string(cmsg.Digest)] = true
		// 	p.askForLock.Lock()
		// 	// request the block
		// 	sn := &core.NodeInfo{
		// 		NodeID: p.view,
		// 		ComID:  p.NodeInfo.ComID,
		// 	}
		// 	orequest := &core.RequestOldMessage{
		// 		SeqStartHeight: p.sequenceID + 1,
		// 		SeqEndHeight:   cmsg.SeqID,
		// 		ServerNode:     sn,
		// 		SenderInfo:     p.NodeInfo,
		// 	}
		// 	p.pl.Plog.Printf("C%dN%d : is now requesting message (seq %d to %d) ... \n", p.NodeInfo.ComID, p.NodeInfo.NodeID, orequest.SeqStartHeight, orequest.SeqEndHeight)
		// 	p.messageHub.Send(core.MsgTypePbftRequestOldMessage, p.NodeInfo.ComID, orequest, nil)

		// } else {
		// implement interface
		p.ihm.HandleinCommit(cmsg)
		p.reply(cmsg.SeqID, cmsg.Digest)
		if p.NodeInfo.NodeID != p.view {
			p.pl.Plog.Printf("C%dN%d: this round of pbft %d is end \n", p.NodeInfo.ComID, p.NodeInfo.NodeID, p.sequenceID)
			p.sequenceID += 1
		}
		// }

		// // if this node is a main node, then unlock the sequencelock
		// if p.NodeInfo.NodeID == p.view {
		// 	p.sequenceLock.Unlock()
		// 	p.pl.Plog.Printf("C%dN%d get sequenceLock unlocked...\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)
		// }
	}
}

func (p *PbftConsensusNode) HandleReply(rmsg *core.Reply) {
	if p.NodeInfo.NodeID != p.view {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	if _, ok := p.gotEnoughReply[rmsg.MessageID]; ok {
		return
	}
	p.pl.Plog.Printf("C%dN%d received the Reply from ...%d sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, rmsg.SenderInfo.NodeID, rmsg.MessageID)

	p.replyCnt += 1
	if p.replyCnt >= int(2*p.malicious_nums) { // 去掉leader自己和f（=1）
		p.pl.Plog.Printf("C%dN%d : has received 2f replys ... sequenceID: %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, rmsg.MessageID)
		// p.ihm.HandleinReply(rmsg) // if needed, add it to the interface and implement it

		p.replyCnt = 0
		p.gotEnoughReply[rmsg.MessageID] = true

		p.pl.Plog.Printf("C%dN%d: this round of pbft %d is end \n", p.NodeInfo.ComID, p.NodeInfo.NodeID, p.sequenceID)
		p.sequenceID += 1
		p.OneConsensusDone <- struct{}{}

		p.pl.Plog.Printf("C%dN%d get sequenceLock unlocked...\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)
		p.sequenceLock.Unlock()
	}

}

// this func is only invoked by the main node,
// if the request is correct, the main node will send
// block back to the message sender.
// now this function can send both block and partition
func (p *PbftConsensusNode) HandleRequestOldSeq(rom *core.RequestOldMessage) {
	if p.view != p.NodeInfo.NodeID {
		rom = nil
		return
	}

	p.pl.Plog.Printf("C%dN%d : received the old message requst from ... %d", p.NodeInfo.ComID, p.NodeInfo.NodeID, rom.ServerNode.NodeID)
	rom.SenderInfo.PrintNode()

	oldR := make([]*core.PbftRequest, 0)
	for height := rom.SeqStartHeight; height <= rom.SeqEndHeight; height++ {
		if _, ok := p.height2Digest[height]; !ok {
			p.pl.Plog.Printf("C%dN%d : has no this digest to this height %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, height)
			break
		}
		if r, ok := p.requestPool[p.height2Digest[height]]; !ok {
			p.pl.Plog.Printf("C%dN%d : has no this message to this digest %d\n", p.NodeInfo.ComID, p.NodeInfo.NodeID, height)
			break
		} else {
			oldR = append(oldR, r)
		}
	}
	p.pl.Plog.Printf("C%dN%d : has generated the message to be sent\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)

	p.ihm.HandleReqestforOldSeq(rom)

	// send the block back
	sb := &core.SendOldMessage{
		SeqStartHeight: rom.SeqStartHeight,
		SeqEndHeight:   rom.SeqEndHeight,
		OldRequest:     oldR,
		ReceiverInfo:   rom.SenderInfo,
	}
	p.messageHub.Send(core.MsgTypePbftSendOldMessage, 0, sb, nil)
	p.pl.Plog.Printf("C%dN%d : send blocks\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)
}

// node requst blocks and receive blocks from the main node
func (p *PbftConsensusNode) HandleSendOldSeq(som *core.SendOldMessage) {
	p.pl.Plog.Printf("C%dN%d : has received the SendOldMessage message\n", p.NodeInfo.ComID, p.NodeInfo.NodeID)

	// implement interface for new consensus
	p.ihm.HandleforSequentialRequest(som)
	beginSeq := som.SeqStartHeight
	for idx, r := range som.OldRequest {
		p.requestPool[string(getDigest(r))] = r
		p.height2Digest[uint64(idx)+beginSeq] = string(getDigest(r))
		p.reply(uint64(idx)+beginSeq, getDigest(r))
		p.pl.Plog.Printf("this round of pbft %d is end \n", uint64(idx)+beginSeq)
	}
	p.sequenceID = som.SeqEndHeight + 1
	if rDigest, ok1 := p.height2Digest[p.sequenceID]; ok1 {
		if r, ok2 := p.requestPool[rDigest]; ok2 {
			ppmsg := &core.PrePrepare{
				RequestMsg: r,
				SeqID:      p.sequenceID,
				Digest:     getDigest(r),
			}
			flag := false
			flag = p.ihm.HandleinPrePrepare(ppmsg)
			if flag {
				pre := &core.Prepare{
					Digest:     ppmsg.Digest,
					SeqID:      ppmsg.SeqID,
					SenderInfo: p.NodeInfo,
				}

				// broadcast
				p.messageHub.Send(core.MsgTypePbftPrepare, 0, pre, nil)
				p.pl.Plog.Printf("C%dN%d : has broadcast the prepare message \n", p.NodeInfo.ComID, p.NodeInfo.NodeID)
			}
		}
	}

	p.askForLock.Unlock()
}
