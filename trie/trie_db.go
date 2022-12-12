package trie

import (
	"errors"
	"sync"

	"github.com/jaiminpan/mt-trie/accdb"
	"github.com/jaiminpan/mt-trie/common"
)

type TrieDB struct {
	diskdb accdb.KeyValueStore // Persistent storage for matured trie nodes

	dirties map[common.Hash]*cachedNode // Data and references relationships of dirty trie nodes
	oldest  common.Hash                 // Oldest tracked node, flush-list head
	newest  common.Hash                 // Newest tracked node, flush-list tail

	lock sync.RWMutex
}

// creates a new trie database to store ephemeral trie content before
// its written out to disk or garbage collected. No read cache is created, so all
// data retrievals will hit the underlying disk database.
func NewTrieDB(diskdb accdb.KeyValueStore) *TrieDB {
	db := &TrieDB{
		diskdb: diskdb,
		dirties: map[common.Hash]*cachedNode{{}: {
			children: make(map[common.Hash]uint16),
		}},
	}
	return db
}

// NewDatabaseWithConfig creates a new trie database to store ephemeral trie content
// before its written out to disk or garbage collected. It also acts as a read cache
// for nodes loaded from disk.
func NewTrieDBWithConfig(diskdb accdb.KeyValueStore) *TrieDB {
	db := &TrieDB{
		diskdb: diskdb,
		dirties: map[common.Hash]*cachedNode{{}: {
			children: make(map[common.Hash]uint16),
		}},
	}
	return db
}

// Node retrieves the trie node with the given node hash.
// No error will be returned if the node is not found.
func (db *TrieDB) Node(_ common.Hash, _ []byte, hash common.Hash) (node, error) {
	return db.node(hash), nil
}

// NodeBlob retrieves the RLP-encoded trie node blob with the given node hash.
// No error will be returned if the node is not found.
func (db *TrieDB) NodeBlob(_ common.Hash, _ []byte, hash common.Hash) ([]byte, error) {
	blob, _ := db.nodeBlob(hash)
	return blob, nil
}

// node retrieves a cached trie node from memory, or returns nil if none can be
// found in the memory cache.
func (db *TrieDB) node(hash common.Hash) node {

	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.obj(hash)
	}

	// Content unavailable in memory, attempt to retrieve from disk
	enc, err := db.diskdb.Get(hash[:])
	if err != nil || enc == nil {
		return nil
	}
	// The returned value from database is in its own copy,
	// safe to use mustDecodeNodeUnsafe for decoding.
	return mustDecodeNodeUnsafe(hash[:], enc)
}

// Node retrieves an encoded cached trie node from memory. If it cannot be found
// cached, the method queries the persistent database for the content.
func (db *TrieDB) nodeBlob(hash common.Hash) ([]byte, error) {
	// It doesn't make sense to retrieve the metaroot
	if hash == (common.Hash{}) {
		return nil, errors.New("not found")
	}
	// Retrieve the node from the dirty cache if available
	db.lock.RLock()
	dirty := db.dirties[hash]
	db.lock.RUnlock()

	if dirty != nil {
		return dirty.rlp(), nil
	}

	// Content unavailable in memory, attempt to retrieve from disk
	enc, _ := db.diskdb.Get(hash[:])
	if len(enc) == 0 {
		return nil, errors.New("not found")
	}
	return enc, nil
}

// Nodes retrieves the hashes of all the nodes cached within the memory database.
// This method is extremely expensive and should only be used to validate internal
// states in test code.
func (db *TrieDB) Nodes() []common.Hash {
	db.lock.RLock()
	defer db.lock.RUnlock()

	var hashes = make([]common.Hash, 0, len(db.dirties))
	for hash := range db.dirties {
		if hash != (common.Hash{}) { // Special case for "root" references/nodes
			hashes = append(hashes, hash)
		}
	}
	return hashes
}
