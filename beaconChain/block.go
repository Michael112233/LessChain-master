package beaconChain

import (
	"fmt"
	"go-w3chain/core"
	"go-w3chain/log"
	"time"
)

type TBBlock struct {
	/* 时间戳 */
	Time   uint64
	Height uint64
	/** key是分片ID，value是该分片未在信标链上链的区块的信标
	 * 一个分片可能对应多个信标，取决于分片出块速度 */
	Tbs [][]*ConfirmedTB
}

func (tbChain *BeaconChain) loop() {
	defer tbChain.wg.Done()
	log.Info("TBChain work loop start.")
	blockInterval := time.Duration(tbChain.cfg.BlockInterval) * time.Second
	timer := time.NewTimer(blockInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			if tbChain.mode == 0 || tbChain.mode == 1 || tbChain.mode == 2 {
				block := tbChain.GenerateBlock()
				if block != nil {
					tbChain.toPushBlock(block)
				}
				timer.Reset(blockInterval)
			} else {
				err := fmt.Errorf("unknown mode of tbChain! mode=%d", tbChain.mode)
				log.Error("err occurs", "err", err)
			}

		case <-tbChain.stopCh:
			log.Info("TBChain work loop stop.")
			return
		}
	}
}

func (tbChain *BeaconChain) GenerateBlock() *TBBlock {
	if tbChain.mode == 0 {
		return tbChain.generateSimulationChainBlock()
	} else if tbChain.mode == 1 || tbChain.mode == 2 {
		return tbChain.generateEthChainBlock()
	} else {
		log.Error("unknown mode", "mode", tbChain.mode)
		return nil
	}
}

func (tbChain *BeaconChain) generateSimulationChainBlock() *TBBlock {
	tbChain.lock_new.Lock()
	defer tbChain.lock_new.Unlock()
	tbChain.lock.Lock()
	defer tbChain.lock.Unlock()

	now := time.Now().Unix()
	tbChain.height += 1

	// confirmTBs := make(map[uint32][]*ConfirmedTB, 0)
	confirmTBs := make([][]*ConfirmedTB, tbChain.shardNum)
	for shardID, tbs := range tbChain.tbs_new {
		shardContract := tbChain.contract.contracts[shardID]
		for _, signedTb := range tbs {
			// todo: 验证多签名
			if !shardContract.VerifyTimeBeacon(signedTb) {
				log.Error("TBchain verify time beacon fail. this time beacon has no enough valid signatures!!")
			} else {
				log.Trace("TBchain verify time beacon success.")
			}
			confirmedTB := &ConfirmedTB{
				TimeBeacon:    signedTb.TimeBeacon,
				ConfirmTime:   uint64(now),
				ConfirmHeight: tbChain.height,
			}
			confirmTBs[uint32(shardID)] = append(confirmTBs[uint32(shardID)], confirmedTB)
		}
		tbChain.tbs[shardID] = append(tbChain.tbs[shardID], confirmTBs[uint32(shardID)]...)
	}

	block := &TBBlock{
		Tbs:    confirmTBs,
		Time:   uint64(now),
		Height: tbChain.height,
	}

	tbChain.tbs_new = make(map[int][]*core.SignedTB)

	log.Debug("tbchain generate block", "info", block)
	return block
}

/** 信标链生成新区块后，将已确认的区块（包含新的信标）发送给订阅者
* 实际情况下，应该是有监督节点监听信标链的新区块，并将其中的信标发送给订阅者。
这里进行了简化，直接跳过监督节点，由信标链发送给订阅者
* 订阅者包括客户端、委员会等需要获取信标辅助验证的角色
*/
func (tbChain *BeaconChain) toPushBlock(block *TBBlock) {
	if _, ok := tbChain.tbBlocks[block.Height]; ok {
		log.Error(fmt.Sprintf("contradictory tbchain block have same height. old block: %v, new block: %v",
			tbChain.tbBlocks[block.Height], block))
	}
	tbChain.tbBlocks[block.Height] = block

	confirmHeight := block.Height - tbChain.cfg.Height2Confirm
	if confirmBlock, ok := tbChain.tbBlocks[confirmHeight]; ok && confirmBlock != nil {
		tbChain.PushBlock2Client(confirmBlock)
		tbChain.PushBlock2Coms(confirmBlock)
	}
}

/** 信标链生成新区块后，将区块（包含新的信标）发送给客户端
 */
func (tbChain *BeaconChain) PushBlock2Client(block *TBBlock) {
	tbChain.messageHub.Send(core.MsgTypeTBChainPushTB2Client, 0, block, nil)
}

/** 信标链生成新区块后，将区块（包含新的信标）发送给委员会
 */
func (tbChain *BeaconChain) PushBlock2Coms(block *TBBlock) {
	tbChain.messageHub.Send(core.MsgTypeTBChainPushTB2Coms, 0, block, nil)
}
