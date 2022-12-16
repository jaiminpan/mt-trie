package trie

import (
	"errors"
	"fmt"
	"sync"

	"github.com/jaiminpan/mt-trie/accdb"
	"github.com/jaiminpan/mt-trie/common"
	"github.com/jaiminpan/mt-trie/rlp"
	"github.com/jaiminpan/mt-trie/types"
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

// inserts a simplified trie node into the memory database.
// All nodes inserted by this function will be reference tracked
// and in theory should only used for **trie nodes** insertion.
func (db *TrieDB) insert(hash common.Hash, node node) {
	// If the node's already cached, skip
	if _, ok := db.dirties[hash]; ok {
		return
	}

	// Create the cached entry for this node
	entry := &cachedNode{
		node: node,
	}
	entry.forChilds(func(child common.Hash) {
		if c := db.dirties[child]; c != nil {
			c.parents++
		}
	})

	db.dirties[hash] = entry
	entry.flushPrev = db.newest
	// Update the flush-list endpoints
	if db.oldest == (common.Hash{}) {
		db.oldest, db.newest = hash, hash
	} else {
		db.dirties[db.newest].flushNext, db.newest = hash, hash
	}
}

// inserts the dirty nodes in provided nodeset into database and
// link the account trie with multiple storage tries if necessary.
func (db *TrieDB) Update(nodes *MergedNodeSet) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	// Insert dirty nodes into the database. In the same tree, it must be
	// ensured that children are inserted first, then parent so that children
	// can be linked with their parent correctly.
	//
	// Note, the storage tries must be flushed before the account trie to
	// retain the invariant that children go into the dirty cache first.
	var order []common.Hash
	for owner := range nodes.sets {
		if owner == (common.Hash{}) {
			continue
		}
		order = append(order, owner)
	}
	if _, ok := nodes.sets[common.Hash{}]; ok {
		order = append(order, common.Hash{})
	}

	for _, owner := range order {
		subset := nodes.sets[owner]
		for _, path := range subset.updates.order {
			n, ok := subset.updates.nodes[path]
			if !ok {
				return fmt.Errorf("missing node %x %v", owner, path)
			}
			db.insert(n.hash, n.node)
		}
	}
	// Link up the account trie and storage trie
	// if the node points to an account trie leaf.
	if set, present := nodes.sets[common.Hash{}]; present {
		for _, leaf := range set.leaves {
			var account types.StateAccount
			if err := rlp.DecodeBytes(leaf.blob, &account); err != nil {
				return err
			}
			if account.Root != emptyRoot {
				db.reference(account.Root, leaf.parent)
			}
		}
	}
	return nil
}

func (db *TrieDB) reference(child common.Hash, parent common.Hash) {
	// If the node does not exist, it's a node pulled from disk, skip
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If the reference already exists, only duplicate for roots
	if db.dirties[parent].children == nil {
		db.dirties[parent].children = make(map[common.Hash]uint16)
	} else if _, ok = db.dirties[parent].children[child]; ok && parent != (common.Hash{}) {
		return
	}
	node.parents++
	db.dirties[parent].children[child]++
}

func (db *TrieDB) dereference(child common.Hash, parent common.Hash) {
	// Dereference the parent-child
	node := db.dirties[parent]

	if node.children != nil && node.children[child] > 0 {
		node.children[child]--
		if node.children[child] == 0 {
			delete(node.children, child)
		}
	}
	// If the child does not exist, it's a previously committed node.
	node, ok := db.dirties[child]
	if !ok {
		return
	}
	// If there are no more references to the child, delete it and cascade
	if node.parents > 0 {
		// This is a special cornercase where a node loaded from disk (i.e. not in the
		// memcache any more) gets reinjected as a new node (short node split into full,
		// then reverted into short), causing a cached node to have no parents. That is
		// no problem in itself, but don't make maxint parents out of it.
		node.parents--
	}
	if node.parents == 0 {
		// Remove the node from the flush-list
		switch child {
		case db.oldest:
			db.oldest = node.flushNext
			db.dirties[node.flushNext].flushPrev = common.Hash{}
		case db.newest:
			db.newest = node.flushPrev
			db.dirties[node.flushPrev].flushNext = common.Hash{}
		default:
			db.dirties[node.flushPrev].flushNext = node.flushNext
			db.dirties[node.flushNext].flushPrev = node.flushPrev
		}
		// Dereference all children and delete the node
		node.forChilds(func(hash common.Hash) {
			db.dereference(hash, child)
		})
		delete(db.dirties, child)
	}
}

// Commit iterates over all the children of a particular node, writes them out
// to disk, forcefully tearing down all references in both directions. As a side
// effect, all pre-images accumulated up to this point are also written.
//
// Note, this method is a non-synchronized mutator. It is unsafe to call this
// concurrently with other mutators.
func (db *TrieDB) Commit(node common.Hash) error {
	// Create a database batch to flush persistent data out. It is important that
	// outside code doesn't see an inconsistent state (referenced data removed from
	// memory cache during commit but not yet in persistent storage). This is ensured
	// by only uncaching existing data when the database write finalizes.
	batch := db.diskdb.NewBatch()

	uncacher := &cleaner{db}
	if err := db.commit(node, batch, uncacher); err != nil {
		return err
	}
	// Trie mostly committed to disk, flush any batch leftovers
	if err := batch.Write(); err != nil {
		return err
	}
	// Uncache any leftovers in the last batch
	db.lock.Lock()
	defer db.lock.Unlock()
	if err := batch.Replay(uncacher); err != nil {
		return err
	}
	batch.Reset()

	return nil
}

// commit is the private locked version of Commit.
func (db *TrieDB) commit(hash common.Hash, batch accdb.Batch, uncacher *cleaner) error {
	// If the node does not exist, it's a previously committed node
	node, ok := db.dirties[hash]
	if !ok {
		return nil
	}
	var err error
	node.forChilds(func(child common.Hash) {
		if err == nil {
			err = db.commit(child, batch, uncacher)
		}
	})
	if err != nil {
		return err
	}
	// If we've reached an optimal batch size, commit and start over
	if err := batch.Put(hash[:], node.rlp()); err != nil {
		// log.Crit("Failed to store trie node", "err", err)
	}
	if batch.ValueSize() >= accdb.IdealBatchSize {
		if err := batch.Write(); err != nil {
			return err
		}
		db.lock.Lock()
		err := batch.Replay(uncacher)
		batch.Reset()
		db.lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}
