package shard

import (
	"go-w3chain/log"
)

type command string

const prefixCMDLength = 12
const (
	cHeader command = "Header"
)

//默认前十二位为命令名称
func jointMessage(cmd command, content []byte) []byte {
	b := make([]byte, prefixCMDLength)
	for i, v := range []byte(cmd) {
		b[i] = v
	}
	joint := append(b, content...)
	return joint
}

//默认前十二位为命令名称
func splitMessage(message []byte) (string, []byte) {
	if len(message) < prefixCMDLength {
		log.Error("message len is smaller than prefixCMDLength", "len(message)", len(message), "prefixCMDLength", prefixCMDLength)
	}
	cmdBytes := message[:prefixCMDLength]
	newCMDBytes := make([]byte, 0)
	for _, v := range cmdBytes {
		if v != byte(0) {
			newCMDBytes = append(newCMDBytes, v)
		}
	}
	cmd := string(newCMDBytes)
	content := message[prefixCMDLength:]
	return cmd, content
}
