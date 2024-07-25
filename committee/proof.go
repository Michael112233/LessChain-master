package committee

import (
	"bytes"
	"errors"

	"github.com/ethereum/go-ethereum/crypto"
)

type proofReader struct {
	proof [][]byte
}

func (p *proofReader) Get(key []byte) ([]byte, error) {
	for _, node := range p.proof {
		if bytes.Equal(crypto.Keccak256(node), key) {
			return node, nil
		}
	}
	return nil, errors.New("key not found in proof")
}

func (p *proofReader) Has(key []byte) (bool, error) {
	_, err := p.Get(key)
	if err != nil {
		return false, nil
	}
	return true, nil
}
