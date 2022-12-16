package memorydb

import (
	"errors"
	"sync"

	"github.com/jaiminpan/mt-trie/accdb"
	"github.com/jaiminpan/mt-trie/common"
)

var (
	// errMemorydbClosed is returned if a memory database was already closed at the
	// invocation of a data access operation.
	errMemorydbClosed = errors.New("database closed")

	// errMemorydbNotFound is returned if a key is requested that is not found in
	// the provided memory database.
	errMemorydbNotFound = errors.New("not found")
)

// Database is an ephemeral key-value store. Apart from basic data storage
// functionality it also supports batch writes and iterating over the keyspace in
// binary-alphabetical order.
type MemDB struct {
	kv   map[string][]byte
	lock sync.RWMutex
}

// New returns a wrapped map with all the required database interface methods
// implemented.
func New() *MemDB {
	return &MemDB{
		kv: make(map[string][]byte),
	}
}

// Has retrieves if a key is present in the key-value store.
func (db *MemDB) Has(key []byte) (bool, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if db.kv == nil {
		return false, errMemorydbClosed
	}
	_, ok := db.kv[string(key)]
	return ok, nil
}

// Get retrieves the given key if it's present in the key-value store.
func (db *MemDB) Get(key []byte) ([]byte, error) {
	db.lock.RLock()
	defer db.lock.RUnlock()

	if db.kv == nil {
		return nil, errMemorydbClosed
	}
	if entry, ok := db.kv[string(key)]; ok {
		return common.CopyBytes(entry), nil
	}
	return nil, errMemorydbNotFound
}

// Put inserts the given value into the key-value store.
func (db *MemDB) Put(key []byte, value []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if db.kv == nil {
		return errMemorydbClosed
	}
	db.kv[string(key)] = common.CopyBytes(value)
	return nil
}

// Delete removes the key from the key-value store.
func (db *MemDB) Delete(key []byte) error {
	db.lock.Lock()
	defer db.lock.Unlock()

	if db.kv == nil {
		return errMemorydbClosed
	}
	delete(db.kv, string(key))
	return nil
}

// keyvalue is a key-value tuple tagged with a deletion field to allow creating
// memory-database write batches.
type keyvalue struct {
	key    []byte
	value  []byte
	delete bool
}

// batch is a write-only memory batch that commits changes to its host
// database when Write is called. A batch cannot be used concurrently.
type memBatch struct {
	db     *MemDB
	writes []keyvalue
	size   int
}

// NewBatch creates a write-only key-value store that buffers changes to its host
// database until a final write is called.
func (db *MemDB) NewBatch() accdb.Batch {
	return &memBatch{
		db: db,
	}
}

// Put inserts the given value into the batch for later committing.
func (b *memBatch) Put(key, value []byte) error {
	b.writes = append(b.writes, keyvalue{common.CopyBytes(key), common.CopyBytes(value), false})
	b.size += len(key) + len(value)
	return nil
}

// Delete inserts the a key removal into the batch for later committing.
func (b *memBatch) Delete(key []byte) error {
	b.writes = append(b.writes, keyvalue{common.CopyBytes(key), nil, true})
	b.size += len(key)
	return nil
}

// ValueSize retrieves the amount of data queued up for writing.
func (b *memBatch) ValueSize() int {
	return b.size
}

// Write flushes any accumulated data to the memory database.
func (b *memBatch) Write() error {
	b.db.lock.Lock()
	defer b.db.lock.Unlock()

	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			delete(b.db.kv, string(keyvalue.key))
			continue
		}
		b.db.kv[string(keyvalue.key)] = keyvalue.value
	}
	return nil
}

// Reset resets the batch for reuse.
func (b *memBatch) Reset() {
	b.writes = b.writes[:0]
	b.size = 0
}

// Replay
// Replay replays the batch contents.
func (b *memBatch) Replay(w accdb.KeyValueWriter) error {
	for _, keyvalue := range b.writes {
		if keyvalue.delete {
			if err := w.Delete(keyvalue.key); err != nil {
				return err
			}
			continue
		}
		if err := w.Put(keyvalue.key, keyvalue.value); err != nil {
			return err
		}
	}
	return nil
}
