package memorydb

import (
	"errors"
	"sync"

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
