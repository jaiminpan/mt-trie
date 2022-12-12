package trie

import (
	"fmt"
	"io"

	"github.com/jaiminpan/mt-trie/common"
)

// rawNode is a simple binary blob used to differentiate between collapsed trie
// nodes and already encoded RLP binary blobs (while at the same time store them
// in the same cache fields).
type rawNode []byte

func (n rawNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawNode) fstring(ind string) string { panic("this should never end up in a live trie") }

func (n rawNode) EncodeRLP(w io.Writer) error {
	_, err := w.Write(n)
	return err
}

// rawFullNode represents only the useful data content of a full node, with the
// caches and flags stripped out to minimize its data storage. This type honors
// the same RLP encoding as the original parent.
type rawFullNode [17]node

func (n rawFullNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawFullNode) fstring(ind string) string { panic("this should never end up in a live trie") }

func (n rawFullNode) EncodeRLP(w io.Writer) error {
	n.encode(w)
	return nil
}

// rawShortNode represents only the useful data content of a short node, with the
// caches and flags stripped out to minimize its data storage. This type honors
// the same RLP encoding as the original parent.
type rawShortNode struct {
	Key []byte
	Val node
}

func (n rawShortNode) cache() (hashNode, bool)   { panic("this should never end up in a live trie") }
func (n rawShortNode) fstring(ind string) string { panic("this should never end up in a live trie") }

type cachedNode struct {
	node node   // Cached collapsed trie node, or raw rlp data
	size uint16 // Byte size of the useful cached data

	parents  uint32                 // Number of live nodes referencing this one
	children map[common.Hash]uint16 // External children referenced by this node

	flushPrev common.Hash // Previous node in the flush-list
	flushNext common.Hash // Next node in the flush-list
}

// rlp returns the raw rlp encoded blob of the cached trie node, either directly
// from the cache, or by regenerating it from the collapsed node.
func (n *cachedNode) rlp() []byte {
	if node, ok := n.node.(rawNode); ok {
		return node
	}
	return nodeToBytes(n.node)
}

// obj returns the decoded and expanded trie node, either directly from the cache,
// or by regenerating it from the rlp encoded blob.
func (n *cachedNode) obj(hash common.Hash) node {
	if node, ok := n.node.(rawNode); ok {
		// The raw-blob format nodes are loaded either from the
		// clean cache or the database, they are all in their own
		// copy and safe to use unsafe decoder.
		return mustDecodeNodeUnsafe(hash[:], node)
	}
	return expandNode(hash[:], n.node)
}

// expandNode traverses the node hierarchy of a collapsed storage node and converts
// all fields and keys into expanded memory form.
func expandNode(hash hashNode, n node) node {
	switch n := n.(type) {
	case *rawShortNode:
		// Short nodes need key and child expansion
		return &shortNode{
			Key: compactToHex(n.Key),
			Val: expandNode(nil, n.Val),
			flags: nodeFlag{
				hash: hash,
			},
		}

	case rawFullNode:
		// Full nodes need child expansion
		node := &fullNode{
			flags: nodeFlag{
				hash: hash,
			},
		}
		for i := 0; i < len(node.Children); i++ {
			if n[i] != nil {
				node.Children[i] = expandNode(nil, n[i])
			}
		}
		return node

	case valueNode, hashNode:
		return n

	default:
		panic(fmt.Sprintf("unknown node type: %T", n))
	}
}
