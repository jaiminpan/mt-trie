package trie

import (
	"bytes"
	"fmt"

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

// newFlag returns the cache flag value for a newly created node.
func (t *Trie) newFlag() nodeFlag {
	return nodeFlag{dirty: true}
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

// The value bytes must not be modified by the caller.
func (t *Trie) Get(key []byte) []byte {
	res, err := t.TryGet(key)
	if err != nil {
	}
	return res
}

// The value bytes must not be modified by the caller.
func (t *Trie) TryGet(key []byte) ([]byte, error) {
	value, newroot, didResolve, err := t.tryGet(t.root, keybytesToHex(key), 0)
	if err == nil && didResolve {
		t.root = newroot
	}
	return value, err
}

func (t *Trie) tryGet(thenode node, key []byte, pos int) (value []byte, newnode node, didResolve bool, err error) {
	switch n := (thenode).(type) {
	case nil:
		return nil, nil, false, nil
	case valueNode:
		return n, n, false, nil
	case *shortNode:
		if len(key)-pos < len(n.Key) || !bytes.Equal(n.Key, key[pos:pos+len(n.Key)]) {
			// key not found in trie
			return nil, n, false, nil
		}
		value, newnode, didResolve, err = t.tryGet(n.Val, key, pos+len(n.Key))
		if err == nil && didResolve {
			n = n.copy()
			n.Val = newnode
		}
		return value, n, didResolve, err
	case *fullNode:
		value, newnode, didResolve, err = t.tryGet(n.Children[key[pos]], key, pos+1)
		if err == nil && didResolve {
			n = n.copy()
			n.Children[key[pos]] = newnode
		}
		return value, n, didResolve, err
	case hashNode:
		child, err := t.resolveAndTrack(n, key[:pos])
		if err != nil {
			return nil, n, true, err
		}
		value, newnode, _, err := t.tryGet(child, key, pos)
		return value, newnode, true, err
	default:
		panic(fmt.Sprintf("%T: invalid node: %v", thenode, thenode))
	}
}

func (t *Trie) resolve(n node, prefix []byte) (node, error) {
	if n, ok := n.(hashNode); ok {
		return t.resolveAndTrack(n, prefix)
	}
	return n, nil
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

func (t *Trie) Update(key, value []byte) {
	if err := t.tryUpdate(key, value); err != nil {
	}
}

// tryUpdate expects an RLP-encoded value and performs the core function
// for TryUpdate and TryUpdateAccount.
func (t *Trie) tryUpdate(key, value []byte) error {
	k := keybytesToHex(key)
	if len(value) != 0 {
		_, n, err := t.insert(t.root, nil, k, valueNode(value))
		if err != nil {
			return err
		}
		t.root = n
	} else {
		_, n, err := t.delete(t.root, nil, k)
		if err != nil {
			return err
		}
		t.root = n
	}
	return nil
}

func (t *Trie) insert(n node, prefix, key []byte, value node) (bool, node, error) {
	if len(key) == 0 {
		if v, ok := n.(valueNode); ok {
			val_dirty := !bytes.Equal(v, value.(valueNode))
			return val_dirty, value, nil
		}
		return true, value, nil
	}
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
		// whole key matches, keep this short node as is and only update the value.
		if matchlen == len(n.Key) {
			n_prefix := append(prefix, key[:matchlen]...)
			dirty, nn, err := t.insert(n.Val, n_prefix, key[matchlen:], value)
			if !dirty || err != nil {
				return false, n, err
			}
			return true, &shortNode{n.Key, nn, t.newFlag()}, nil
		}
		// Otherwise branch out at the index where they differ.
		branch := &fullNode{flags: t.newFlag()}
		var err error
		_, branch.Children[n.Key[matchlen]], err = t.insert(nil, append(prefix, n.Key[:matchlen+1]...), n.Key[matchlen+1:], n.Val)
		if err != nil {
			return false, nil, err
		}
		_, branch.Children[key[matchlen]], err = t.insert(nil, append(prefix, key[:matchlen+1]...), key[matchlen+1:], value)
		if err != nil {
			return false, nil, err
		}
		// Replace this shortNode with the branch if it occurs at index 0.
		if matchlen == 0 {
			return true, branch, nil
		}

		real_key := append(prefix, key[:matchlen]...)
		t.capture.onInsert(real_key)

		// Replace it with a short node leading up to the branch.
		return true, &shortNode{key[:matchlen], branch, t.newFlag()}, nil

	case *fullNode:
		n_prefix := append(prefix, key[0])
		dirty, nn, err := t.insert(n.Children[key[0]], n_prefix, key[1:], value)
		if !dirty || err != nil {
			return false, n, err
		}

		// replace node
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn
		return true, n, nil

	case nil:
		// New short node is created and track it.
		// The node identifier passed is the path from the root node.
		// valueNode won't be tracked since it's always embedded in its parent.
		t.capture.onInsert(prefix)
		return true, &shortNode{key, value, t.newFlag()}, nil

	case hashNode:
		// We've hit a part of the trie that isn't loaded yet. Load
		// the node and insert into it. This leaves all child nodes on
		// the path to the value in the trie.
		rn, err := t.resolveAndTrack(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.insert(rn, prefix, key, value)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v", n, n))
	}
}

// delete returns the new root of the trie with key deleted.
// It reduces the trie to minimal form by simplifying
// nodes on the way up after deleting recursively.
func (t *Trie) delete(n node, prefix, key []byte) (bool, node, error) {
	switch n := n.(type) {
	case *shortNode:
		matchlen := prefixLen(key, n.Key)
		if matchlen < len(n.Key) {
			return false, n, nil // don't replace n on mismatch
		}
		if matchlen == len(key) {
			// The matched short node is deleted entirely
			t.capture.onDelete(prefix)
			return true, nil, nil // remove n entirely for whole matches
		}
		// The key is longer than n.Key Remove the remaining suffix from the subtrie.
		// Child can never be nil here since the subtrie must contain at least
		// two other values with keys longer than n.Key.
		n_prefix := append(prefix, key[:len(n.Key)]...)
		dirty, child, err := t.delete(n.Val, n_prefix, key[len(n.Key):])
		if !dirty || err != nil {
			return false, n, err
		}
		switch child := child.(type) {
		case *shortNode:
			// The child shortNode is merged into its parent
			t.capture.onDelete(append(prefix, n.Key...))

			// Deleting from the subtrie reduced it to another short node.
			// Merge the nodes to avoid creating a shortNode{..., shortNode{...}}.
			// Use concat (which always creates a new slice) instead of append to
			// avoid modifying n.Key since it might be shared with other nodes.
			n_key := prefixConcat(n.Key, child.Key...)
			return true, &shortNode{n_key, child.Val, t.newFlag()}, nil
		default:
			return true, &shortNode{n.Key, child, t.newFlag()}, nil
		}

	case *fullNode:
		n_prefix := append(prefix, key[0])
		dirty, nn, err := t.delete(n.Children[key[0]], n_prefix, key[1:])
		if !dirty || err != nil {
			return false, n, err
		}
		n = n.copy()
		n.flags = t.newFlag()
		n.Children[key[0]] = nn

		// Because n is a full node, it must've contained at least two children
		// before the delete operation. If the new child value is non-nil, n still
		// has at least two children after the deletion, and cannot be reduced to
		// a short node.
		if nn != nil {
			return true, n, nil
		}
		// Reduction:
		// Check how many non-nil entries are left after deleting and
		// reduce the full node to a short node if only one entry is
		// left. Since n must've contained at least two children
		// before deletion (otherwise it would not be a full node) n
		// can never be reduced to nil.
		//
		// When the loop is done, pos contains the index of the single
		// value that is left in n or -2 if n contains at least two
		// values.
		pos := -1
		for i, cld := range &n.Children {
			if cld != nil {
				if pos == -1 {
					pos = i
				} else {
					pos = -2
					break
				}
			}
		}
		if pos >= 0 {
			if pos != 16 {
				// If the remaining entry is a short node, it replaces
				// n and its key gets the missing nibble tacked to the
				// front. This avoids creating an invalid
				// shortNode{..., shortNode{...}}.  Since the entry
				// might not be loaded yet, resolve it just for this check.
				cnode, err := t.resolve(n.Children[pos], append(prefix, byte(pos)))
				if err != nil {
					return false, nil, err
				}
				if cnode, ok := cnode.(*shortNode); ok {
					// Replace the entire full node with the short node.
					// Mark the original short node as deleted since the
					// value is embedded into the parent now.
					t.capture.onDelete(append(prefix, byte(pos)))

					k := append([]byte{byte(pos)}, cnode.Key...)
					return true, &shortNode{k, cnode.Val, t.newFlag()}, nil
				}
			}
			// Otherwise, n is replaced by a one-nibble short node
			// containing the child.
			return true, &shortNode{[]byte{byte(pos)}, n.Children[pos], t.newFlag()}, nil
		}
		// n still contains at least two values and cannot be reduced.
		return true, n, nil

	case valueNode:
		return true, nil, nil

	case nil:
		return false, nil, nil

	case hashNode:
		// We've hit a part of the trie that isn't loaded yet. Load
		// the node and delete from it. This leaves all child nodes on
		// the path to the value in the trie.
		rn, err := t.resolveAndTrack(n, prefix)
		if err != nil {
			return false, nil, err
		}
		dirty, nn, err := t.delete(rn, prefix, key)
		if !dirty || err != nil {
			return false, rn, err
		}
		return true, nn, nil

	default:
		panic(fmt.Sprintf("%T: invalid node: %v (%v)", n, n, key))
	}
}
