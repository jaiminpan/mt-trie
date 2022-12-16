package trie

import (
	"fmt"

	"github.com/jaiminpan/mt-trie/common"
)

// leaf represents a trie leaf node
type leaf struct {
	blob   []byte      // raw blob of leaf
	parent common.Hash // the hash of parent node
}

// memoryNode is all the information we know about a single cached trie node
// in the memory.
type memoryNode struct {
	hash common.Hash // Node hash, computed by hashing rlp value, empty for deleted nodes
	size uint16      // Byte size of the useful cached data, 0 for deleted nodes
	node node        // Cached collapsed trie node, or raw rlp data, nil for deleted nodes
}

// nodeWithPrev wraps the memoryNode with the previous node value.
type nodeWithPrev struct {
	*memoryNode
	oldv []byte // RLP-encoded previous value, nil means it's non-existent
}

// nodesWithOrder represents a collection of dirty nodes which includes
// newly-inserted and updated nodes. The modification order of all nodes
// is represented by order list.
type nodesWithOrder struct {
	order []string                 // the path list of dirty nodes, sort by insertion order
	nodes map[string]*nodeWithPrev // the map of dirty nodes, keyed by node path
}

// NodeSet contains all dirty nodes collected during the commit operation.
// Each node is keyed by path. It's not thread-safe to use.
type NodeSet struct {
	owner   common.Hash       // the identifier of the trie
	updates *nodesWithOrder   // the set of updated nodes(newly inserted, updated)
	deletes map[string][]byte // the map of deleted nodes, keyed by node
	leaves  []*leaf           // the list of dirty leaves
}

// NewNodeSet initializes an empty node set to be used for tracking dirty nodes
// from a specific account or storage trie. The owner is zero for the account
// trie and the owning account address hash for storage tries.
func NewNodeSet(owner common.Hash) *NodeSet {
	return &NodeSet{
		owner: owner,
		updates: &nodesWithOrder{
			nodes: make(map[string]*nodeWithPrev),
		},
		deletes: make(map[string][]byte),
	}
}

// markUpdated marks the node as dirty(newly-inserted or updated) with provided node path,
// node object along with its previous value.
func (set *NodeSet) markUpdated(path []byte, node *memoryNode, oldv []byte) {
	set.updates.order = append(set.updates.order, string(path))
	set.updates.nodes[string(path)] = &nodeWithPrev{
		memoryNode: node,
		oldv:       oldv,
	}
}

// markDeleted marks the node as deleted with provided path and previous value.
func (set *NodeSet) markDeleted(path []byte, oldv []byte) {
	set.deletes[string(path)] = oldv
}

// addLeaf collects the provided leaf node into set.
func (set *NodeSet) addLeaf(node *leaf) {
	set.leaves = append(set.leaves, node)
}

// MergedNodeSet represents a merged dirty node set for a group of tries.
type MergedNodeSet struct {
	sets map[common.Hash]*NodeSet
}

func NewMergedNodeSet() *MergedNodeSet {
	return &MergedNodeSet{sets: make(map[common.Hash]*NodeSet)}
}

func (set *MergedNodeSet) Merge(other *NodeSet) error {
	_, present := set.sets[other.owner]
	if present {
		return fmt.Errorf("duplicate trie for owner %#x", other.owner)
	}
	set.sets[other.owner] = other
	return nil
}
