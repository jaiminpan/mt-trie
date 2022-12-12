package trie

import (
	"github.com/jaiminpan/mt-trie/common"
)

// trieReader is a wrapper of the underlying node reader. It's not safe
// for concurrent usage.
type trieReader struct {
	owner common.Hash
	// reader Reader
}

// node retrieves the trie node with the provided trie node information.
// An MissingNodeError will be returned in case the node is not found or
// any error is encountered.
func (r *Trie) node(path []byte, hash common.Hash) (node, error) {
	owner := common.Hash{}
	if r.reader == nil {
		return nil, &MissingNodeError{Owner: owner, NodeHash: hash, Path: path}
	}
	node, err := r.reader.Node(owner, path, hash)
	if err != nil || node == nil {
		return nil, &MissingNodeError{Owner: owner, NodeHash: hash, Path: path, err: err}
	}
	return node, nil
}

// node retrieves the rlp-encoded trie node with the provided trie node
// information. An MissingNodeError will be returned in case the node is
// not found or any error is encountered.
func (r *Trie) nodeBlob(path []byte, hash common.Hash) ([]byte, error) {
	owner := common.Hash{}
	if r.reader == nil {
		return nil, &MissingNodeError{Owner: owner, NodeHash: hash, Path: path}
	}
	blob, err := r.reader.NodeBlob(owner, path, hash)
	if err != nil || len(blob) == 0 {
		return nil, &MissingNodeError{Owner: owner, NodeHash: hash, Path: path, err: err}
	}
	return blob, nil
}
