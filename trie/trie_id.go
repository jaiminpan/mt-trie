package trie

import "github.com/jaiminpan/mt-trie/common"

// ID is the identifier for uniquely identifying a trie.
type ID struct {
	Root common.Hash // The root hash of trie
}

// TrieID constructs an identifier for a standard trie(not a second-layer trie)
// with provided root. It's mostly used in tests and some other tries like CHT trie.
func TrieID(root common.Hash) *ID {
	return &ID{
		Root: root,
	}
}
