package trie

import (
	"github.com/jaiminpan/mt-trie/common"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")
)

// Trie is a Merkle Patricia Trie. Use New to create a trie that sits on
// top of a database. Whenever trie performs a commit operation, the generated
// nodes will be gathered and returned in a set. Once the trie is committed,
// it's not usable anymore. Callers have to re-create the trie with new root
// based on the updated trie database.
//
// Trie is not safe for concurrent use.
type Trie struct {
	root node

	// Keep track of the number leaves which have been inserted since the last
	// hashing operation. This number will not directly map to the number of
	// actually unhashed nodes.
	// unhashed int

	// reader is the handler trie can retrieve nodes from.
	reader *TrieDB

	// capture is the tool to track the trie changes.
	// It will be reset after each commit operation.
	capture *trieCapture
}

// New creates the trie instance with provided trie id and the read-only
// database. The state specified by trie id must be available, otherwise
// an error will be returned. The trie root specified by trie id can be
// zero hash or the sha3 hash of an empty string, then trie is initially
// empty, otherwise, the root node must be present in database or returns
// a MissingNodeError if not.
func New(id *ID, db *TrieDB) (*Trie, error) {
	trie := &Trie{
		reader: db,
	}
	if id.Root != (common.Hash{}) && id.Root != emptyRoot {
		rootnode, err := trie.resolveAndTrack(id.Root[:], nil)
		if err != nil {
			return nil, err
		}
		trie.root = rootnode
	}
	return trie, nil
}

// NewEmpty is a shortcut to create empty tree. It's mostly used in tests.
func NewEmpty(db *TrieDB) *Trie {
	tr, _ := New(TrieID(common.Hash{}), db)
	return tr
}

// resolveAndTrack loads node from the underlying store with the given node hash
// and path prefix and also tracks the loaded node blob in tracer treated as the
// node's original value. The rlp-encoded blob is preferred to be loaded from
// database because it's easy to decode node while complex to encode node to blob.
func (t *Trie) resolveAndTrack(n hashNode, prefix []byte) (node, error) {
	blob, err := t.nodeBlob(prefix, common.BytesToHash(n))
	if err != nil {
		return nil, err
	}
	// t.capture.onRead(prefix, blob)
	return mustDecodeNode(n, blob), nil
}

// Hash returns the root hash of the trie. It does not write to the
// database and can be used even if the trie doesn't have one.
func (t *Trie) Hash() common.Hash {
	hash, cached, _ := t.hashRoot()
	t.root = cached
	return common.BytesToHash(hash.(hashNode))
}

// hashRoot calculates the root hash of the given trie
func (t *Trie) hashRoot() (node, node, error) {
	if t.root == nil {
		return hashNode(emptyRoot[:]), nil, nil
	}
	// If the number of changes is below 100, we let one thread handle it
	h := newHasher()
	defer returnHasherToPool(h)
	hashed, cached := h.hash(t.root, true)
	return hashed, cached, nil
}
