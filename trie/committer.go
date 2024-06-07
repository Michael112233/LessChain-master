// Copyright 2019 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package trie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/sha3"
)

// leafChanSize is the size of the leafCh. It's a pretty arbitrary number, to allow
// some parallelism but not incur too much memory overhead.
const leafChanSize = 200

// leaf represents a trie leaf value
type leaf struct {
	size int         // size of the rlp data (estimate)
	hash common.Hash // hash of rlp data
	Node Node        // the Node to commit
}

// committer is a type used for the trie Commit operation. A committer has some
// internal preallocated temp space, and also a callback that is invoked when
// leaves are committed. The leafs are passed through the `leafCh`,  to allow
// some level of parallelism.
// By 'some level' of parallelism, it's still the case that all leaves will be
// processed sequentially - onleaf will never be called in parallel or out of order.
type committer struct {
	sha crypto.KeccakState

	onleaf LeafCallback
	leafCh chan *leaf
}

// committers live in a global sync.Pool
var committerPool = sync.Pool{
	New: func() interface{} {
		return &committer{
			sha: sha3.NewLegacyKeccak256().(crypto.KeccakState),
		}
	},
}

// newCommitter creates a new committer or picks one from the pool.
func newCommitter() *committer {
	return committerPool.Get().(*committer)
}

func returnCommitterToPool(h *committer) {
	h.onleaf = nil
	h.leafCh = nil
	committerPool.Put(h)
}

// Commit collapses a Node down into a hash Node and inserts it into the database
func (c *committer) Commit(n Node, db *Database) (HashNode, int, error) {
	if db == nil {
		return nil, 0, errors.New("no db provided")
	}
	h, committed, err := c.commit(n, db)
	if err != nil {
		return nil, 0, err
	}
	return h.(HashNode), committed, nil
}

// commit collapses a Node down into a hash Node and inserts it into the database
func (c *committer) commit(n Node, db *Database) (Node, int, error) {
	// if this path is clean, use available cached data
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash, 0, nil
	}
	// Commit children, then parent, and remove remove the dirty flag.
	switch cn := n.(type) {
	case *ShortNode:
		// Commit child
		collapsed := cn.copy()

		// If the child is FullNode, recursively commit,
		// otherwise it can only be HashNode or ValueNode.
		var childCommitted int
		if _, ok := cn.Val.(*FullNode); ok {
			childV, committed, err := c.commit(cn.Val, db)
			if err != nil {
				return nil, 0, err
			}
			collapsed.Val, childCommitted = childV, committed
		}
		// The key needs to be copied, since we're delivering it to database
		collapsed.Key = hexToCompact(cn.Key)
		hashedNode := c.store(collapsed, db)
		if hn, ok := hashedNode.(HashNode); ok {
			return hn, childCommitted + 1, nil
		}
		return collapsed, childCommitted, nil
	case *FullNode:
		hashedKids, childCommitted, err := c.commitChildren(cn, db)
		if err != nil {
			return nil, 0, err
		}
		collapsed := cn.copy()
		collapsed.Children = hashedKids

		hashedNode := c.store(collapsed, db)
		if hn, ok := hashedNode.(HashNode); ok {
			return hn, childCommitted + 1, nil
		}
		return collapsed, childCommitted, nil
	case HashNode:
		return cn, 0, nil
	default:
		// nil, valuenode shouldn't be committed
		panic(fmt.Sprintf("%T: invalid Node: %v", n, n))
	}
}

// commitChildren commits the children of the given fullnode
func (c *committer) commitChildren(n *FullNode, db *Database) ([17]Node, int, error) {
	var (
		committed int
		children  [17]Node
	)
	for i := 0; i < 16; i++ {
		child := n.Children[i]
		if child == nil {
			continue
		}
		// If it's the hashed child, save the hash value directly.
		// Note: it's impossible that the child in range [0, 15]
		// is a ValueNode.
		if hn, ok := child.(HashNode); ok {
			children[i] = hn
			continue
		}
		// Commit the child recursively and store the "hashed" value.
		// Note the returned Node can be some embedded nodes, so it's
		// possible the type is not HashNode.
		hashed, childCommitted, err := c.commit(child, db)
		if err != nil {
			return children, 0, err
		}
		children[i] = hashed
		committed += childCommitted
	}
	// For the 17th child, it's possible the type is valuenode.
	if n.Children[16] != nil {
		children[16] = n.Children[16]
	}
	return children, committed, nil
}

// store hashes the Node n and if we have a storage layer specified, it writes
// the key/value pair to it and tracks any Node->child references as well as any
// Node->external trie references.
func (c *committer) store(n Node, db *Database) Node {
	// Larger nodes are replaced by their hash and stored in the database.
	var (
		hash, _ = n.cache()
		size    int
	)
	if hash == nil {
		// This was not generated - must be a small Node stored in the parent.
		// In theory, we should apply the leafCall here if it's not nil(embedded
		// Node usually contains value). But small value(less than 32bytes) is
		// not our target.
		return n
	} else {
		// We have the hash already, estimate the RLP encoding-size of the Node.
		// The size is used for mem tracking, does not need to be exact
		size = estimateSize(n)
	}
	// If we're using channel-based leaf-reporting, send to channel.
	// The leaf channel will be active only when there an active leaf-callback
	if c.leafCh != nil {
		c.leafCh <- &leaf{
			size: size,
			hash: common.BytesToHash(hash),
			Node: n,
		}
	} else if db != nil {
		// No leaf-callback used, but there's still a database. Do serial
		// insertion
		db.lock.Lock()
		db.insert(common.BytesToHash(hash), size, n)
		db.lock.Unlock()
	}
	return hash
}

// commitLoop does the actual insert + leaf callback for nodes.
func (c *committer) commitLoop(db *Database) {
	for item := range c.leafCh {
		var (
			hash = item.hash
			size = item.size
			n    = item.Node
		)
		// We are pooling the trie nodes into an intermediate memory cache
		db.lock.Lock()
		db.insert(hash, size, n)
		db.lock.Unlock()

		if c.onleaf != nil {
			switch n := n.(type) {
			case *ShortNode:
				if child, ok := n.Val.(ValueNode); ok {
					c.onleaf(nil, nil, child, hash)
				}
			case *FullNode:
				// For children in range [0, 15], it's impossible
				// to contain ValueNode. Only check the 17th child.
				if n.Children[16] != nil {
					c.onleaf(nil, nil, n.Children[16].(ValueNode), hash)
				}
			}
		}
	}
}

func (c *committer) makeHashNode(data []byte) HashNode {
	n := make(HashNode, c.sha.Size())
	c.sha.Reset()
	c.sha.Write(data)
	c.sha.Read(n)
	return n
}

// estimateSize estimates the size of an rlp-encoded Node, without actually
// rlp-encoding it (zero allocs). This method has been experimentally tried, and with a trie
// with 1000 leafs, the only errors above 1% are on small shortnodes, where this
// method overestimates by 2 or 3 bytes (e.g. 37 instead of 35)
func estimateSize(n Node) int {
	switch n := n.(type) {
	case *ShortNode:
		// A short Node contains a compacted key, and a value.
		return 3 + len(n.Key) + estimateSize(n.Val)
	case *FullNode:
		// A full Node contains up to 16 hashes (some nils), and a key
		s := 3
		for i := 0; i < 16; i++ {
			if child := n.Children[i]; child != nil {
				s += estimateSize(child)
			} else {
				s++
			}
		}
		return s
	case ValueNode:
		return 1 + len(n)
	case HashNode:
		return 1 + len(n)
	default:
		panic(fmt.Sprintf("Node type %T", n))
	}
}
