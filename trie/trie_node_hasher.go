package trie

import (
	"sync"

	"github.com/jaiminpan/mt-trie/crypto"
	"github.com/jaiminpan/mt-trie/rlp"
)

// hasher is a type used for the trie Hash operation. A hasher has some
// internal preallocated temp space
type hasher struct {
	sha      crypto.KeccakState
	tmp      []byte
	encbuf   rlp.EncoderBuffer
	parallel bool // Whether to use parallel threads when hashing
}

// hasherPool holds pureHashers
var hasherPool = sync.Pool{
	New: func() interface{} {
		return &hasher{
			tmp:    make([]byte, 0, 550), // cap is as large as a full fullNode.
			sha:    crypto.NewKeccakState(),
			encbuf: rlp.NewEncoderBuffer(nil),
		}
	},
}

func newHasher() *hasher {
	h := hasherPool.Get().(*hasher)
	h.parallel = false
	return h
}

func returnHasherToPool(h *hasher) {
	hasherPool.Put(h)
}

// hash collapses a node down into a hash node, also returning a copy of the
// original node initialized with the computed hash to replace the original one.
func (h *hasher) hash(n node, force bool) (hashed node, cached node) {
	// Return the cached hash if it's available
	if hash, _ := n.cache(); hash != nil {
		return hash, n
	}
	// Trie not processed yet, walk the children
	switch n := n.(type) {
	case *shortNode:
		collapsed, cached := h.hashShortNodeChildren(n)
		hashed := h.shortnodeToHash(collapsed, force)
		// We need to retain the possibly _not_ hashed node, in case it was too
		// small to be hashed
		if hn, ok := hashed.(hashNode); ok {
			cached.flags.hash = hn
		} else {
			cached.flags.hash = nil
		}
		return hashed, cached
	case *fullNode:
		collapsed, cached := h.hashFullNodeChildren(n)
		hashed = h.fullnodeToHash(collapsed, force)
		if hn, ok := hashed.(hashNode); ok {
			cached.flags.hash = hn
		} else {
			cached.flags.hash = nil
		}
		return hashed, cached
	default:
		// Value and hash nodes don't have children so they're left as were
		return n, n
	}
}

// hashShortNodeChildren collapses the short node. The returned collapsed node
// holds a live reference to the Key, and must not be modified.
// The cached
func (h *hasher) hashShortNodeChildren(n *shortNode) (collapsed, cached *shortNode) {
	// Hash the short node's child, caching the newly hashed subtree
	collapsed, cached = n.copy(), n.copy()
	// Previously, we did copy this one. We don't seem to need to actually
	// do that, since we don't overwrite/reuse keys
	//cached.Key = common.CopyBytes(n.Key)
	collapsed.Key = hexToCompact(n.Key)
	// Unless the child is a valuenode or hashnode, hash it
	switch n.Val.(type) {
	case *fullNode, *shortNode:
		collapsed.Val, cached.Val = h.hash(n.Val, false)
	}
	return collapsed, cached
}

func (h *hasher) hashFullNodeChildren(n *fullNode) (collapsed, cached *fullNode) {
	// Hash the full node's children, caching the newly hashed subtrees
	cached, collapsed = n.copy(), n.copy()
	if h.parallel {
		var wg sync.WaitGroup
		wg.Add(16)
		for i := 0; i < 16; i++ {
			go func(i int) {
				hasher := newHasher()
				if child := n.Children[i]; child != nil {
					collapsed.Children[i], cached.Children[i] = hasher.hash(child, false)
				} else {
					collapsed.Children[i] = nilValueNode
				}
				returnHasherToPool(hasher)
				wg.Done()
			}(i)
		}
		wg.Wait()
	} else {
		for i := 0; i < 16; i++ {
			if child := n.Children[i]; child != nil {
				collapsed.Children[i], cached.Children[i] = h.hash(child, false)
			} else {
				collapsed.Children[i] = nilValueNode
			}
		}
	}
	return collapsed, cached
}

// shortnodeToHash creates a hashNode from a shortNode. The supplied shortnode
// should have hex-type Key, which will be converted (without modification)
// into compact form for RLP encoding.
// If the rlp data is smaller than 32 bytes, `nil` is returned.
func (h *hasher) shortnodeToHash(n *shortNode, force bool) node {
	n.encode(h.encbuf)
	h.tmp = h.encbuf.AppendToBytes(h.tmp[:0])
	h.encbuf.Reset(nil)
	enc := h.tmp

	if len(enc) < 32 && !force {
		return n // Nodes smaller than 32 bytes are stored inside their parent
	}

	hashed := make(hashNode, 32)
	crypto.HashData(h.sha, enc, hashed)
	return hashed
}

// shortnodeToHash is used to creates a hashNode from a set of hashNodes, (which
// may contain nil values)
func (h *hasher) fullnodeToHash(n *fullNode, force bool) node {
	n.encode(h.encbuf)
	h.tmp = h.encbuf.AppendToBytes(h.tmp[:0])
	h.encbuf.Reset(nil)
	enc := h.tmp

	if len(enc) < 32 && !force {
		return n // Nodes smaller than 32 bytes are stored inside their parent
	}

	hashed := make(hashNode, 32)
	crypto.HashData(h.sha, enc, hashed)
	return hashed
}
