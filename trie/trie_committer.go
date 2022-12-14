package trie

import (
	"fmt"

	"github.com/jaiminpan/mt-trie/common"
)

// committer is the tool used for the trie Commit operation. The committer will
// capture all dirty nodes during the commit process and keep them cached in
// insertion order.
type committer struct {
	nodes       *NodeSet
	capture     *trieCapture
	collectLeaf bool
}

// newCommitter creates a new committer or picks one from the pool.
func newCommitter(nodes *NodeSet, capture *trieCapture, collectLeaf bool) *committer {
	return &committer{
		nodes:       nodes,
		capture:     capture,
		collectLeaf: collectLeaf,
	}
}

// Commit collapses a node down into a hash node and returns it along with the modified nodeset.
func (c *committer) Commit(n node) (hashNode, *NodeSet, error) {
	h, err := c.commit(nil, n)
	if err != nil {
		return nil, nil, err
	}
	// Some nodes can be deleted from trie which can't be captured by committer itself.
	// Iterate all deleted nodes tracked by tracer and marked them as deleted
	// only if they are present in database previously.
	for _, path := range c.capture.deleteList() {
		// There are a few possibilities for this scenario(the node is deleted
		// but not present in database previously), for example the node was
		// embedded in the parent and now deleted from the trie.
		// In this case it's noop from database's perspective.
		oldv := c.capture.getOldv(path)
		if len(oldv) == 0 {
			continue
		}
		c.nodes.markDeleted(path, oldv)
	}
	return h.(hashNode), c.nodes, nil
}

// commit collapses a node down into a hash node and returns it.
func (c *committer) commit(path []byte, n node) (node, error) {
	// if this path is clean, use available cached data
	hash, dirty := n.cache()
	if hash != nil && !dirty {
		return hash, nil
	}
	// Commit children, then parent, and remove the dirty flag.
	switch cn := n.(type) {
	case *shortNode:
		// Commit child
		collapsed := cn.copy()

		// If the child is fullNode, recursively commit,
		// otherwise it can only be hashNode or valueNode.
		if _, ok := cn.Val.(*fullNode); ok {
			childV, err := c.commit(append(path, cn.Key...), cn.Val)
			if err != nil {
				return nil, err
			}
			collapsed.Val = childV
		}
		// The key needs to be copied,
		// since we're adding it to the modified nodeset.
		collapsed.Key = hexToCompact(cn.Key)
		return c.nodeCommit(path, collapsed)

	case *fullNode:
		hashedKids, err := c.commitChildren(path, cn)
		if err != nil {
			return nil, err
		}
		collapsed := cn.copy()
		collapsed.Children = hashedKids

		return c.nodeCommit(path, collapsed)
	case hashNode:
		return cn, nil
	default:
		// nil, valuenode shouldn't be committed
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// commitChildren commits the children of the given fullnode
func (c *committer) commitChildren(path []byte, n *fullNode) ([17]node, error) {
	var children [17]node
	for i := 0; i < 16; i++ {
		child := n.Children[i]
		if child == nil {
			continue
		}
		// If it's the hashed child, save the hash value directly.
		// Note: it's impossible that the child in range [0, 15] is a valueNode.
		if hn, ok := child.(hashNode); ok {
			children[i] = hn
			continue
		}
		// Commit the child recursively and store the "hashed" value.
		// Note the returned node can be some embedded nodes, so it's
		// possible the type is not hashNode.
		hashed, err := c.commit(append(path, byte(i)), child)
		if err != nil {
			return children, err
		}
		children[i] = hashed
	}
	// For the 17th child, it's possible the type is valuenode.
	if n.Children[16] != nil {
		children[16] = n.Children[16]
	}
	return children, nil
}

func (c *committer) nodeCommit(path []byte, collapsed node) (node, error) {

	// Larger nodes are replaced by their hash and stored in the database.
	hash, _ := collapsed.cache()

	// This was not generated - must be a small node stored in the parent.
	// In theory, we should check if the node is leaf here (embedded node
	// usually is leaf node). But small value (less than 32bytes) is not
	// our target (leaves in account trie only).
	if hash != nil {
		// We have the hash already, estimate the RLP encoding-size of the node.
		// The size is used for mem tracking, does not need to be exact
		var (
			nhash = common.BytesToHash(hash)
			mnode = &memoryNode{
				hash: nhash,
				node: simplifyNode(collapsed),
				// size: uint16(estimateSize(collapsed)),
			}
		)
		// Collect the dirty node to nodeset for return.
		oldv := c.capture.getOldv(path)
		c.nodes.markUpdated(path, mnode, oldv)

		// Collect the corresponding leaf node if it's required. We don't check
		// full node since it's impossible to store value in fullNode. The key
		// length of leaves should be exactly same.
		if c.collectLeaf {
			if sn, ok := collapsed.(*shortNode); ok {
				if val, ok := sn.Val.(valueNode); ok {
					c.nodes.addLeaf(&leaf{blob: val, parent: nhash})
				}
			}
		}
		return hash, nil
	}

	if hn, ok := collapsed.(hashNode); ok {
		return hn, nil
	}
	// The node now is embedded in its parent. Mark the node as
	// deleted if it's present in database previously. It's equivalent
	// as deletion from database's perspective.
	if oldv := c.capture.getOldv(path); len(oldv) != 0 {
		c.nodes.markDeleted(path, oldv)
	}
	return collapsed, nil
}
