package trie

import (
	"fmt"

	"github.com/jaiminpan/mt-trie/common"
)

// MissingNodeError is returned by the trie functions (TryGet, TryUpdate, TryDelete)
// in the case where a trie node is not present in the local database. It contains
// information necessary for retrieving tthe missing node.
type MissingNodeError struct {
	Owner    common.Hash // owner of the trie if it's 2-layered trie
	NodeHash common.Hash // hash of the missing node
	Path     []byte      // hex-encoded path to the missing node
	err      error       // concrete error for missing trie node
}

// Unwrap returns the concrete error for missing trie node which
// allows us for further analysis outside.
func (err *MissingNodeError) Unwrap() error {
	return err.err
}

func (err *MissingNodeError) Error() string {
	if err.Owner == (common.Hash{}) {
		return fmt.Sprintf("missing trie node %x (path %x) %v", err.NodeHash, err.Path, err.err)
	}
	return fmt.Sprintf("missing trie node %x (owner %x) (path %x) %v", err.NodeHash, err.Owner, err.Path, err.err)
}
