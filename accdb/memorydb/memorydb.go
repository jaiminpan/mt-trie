package memorydb

import "sync"

// Database is an ephemeral key-value store. Apart from basic data storage
// functionality it also supports batch writes and iterating over the keyspace in
// binary-alphabetical order.
type MemDB struct {
	db   map[string][]byte
	lock sync.RWMutex
}

// New returns a wrapped map with all the required database interface methods
// implemented.
func New() *MemDB {
	return &MemDB{
		db: make(map[string][]byte),
	}
}
