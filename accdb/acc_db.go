package accdb

// KeyValueReader wraps the Has and Get method of a backing data store.
type KeyValueReader interface {
	// Has retrieves if a key is present in the key-value data store.
	Has(key []byte) (bool, error)

	// Get retrieves the given key if it's present in the key-value data store.
	Get(key []byte) ([]byte, error)
}

// KeyValueWriter wraps the Put method of a backing data store.
type KeyValueWriter interface {
	// Put inserts the given value into the key-value data store.
	Put(key []byte, value []byte) error

	// Delete removes the key from the key-value data store.
	Delete(key []byte) error
}

type KeyValueStore interface {
	// KeyValueReader
	// KeyValueWriter
	// KeyValueStater
	// Batcher
	// Iteratee
	// Compacter
	// Snapshotter
	// io.Closer
}

// Database contains all the methods required by the high level database to not
// only access the key-value data store but also the chain freezer.
type Database interface {
	// KeyValueReader
	// KeyValueWriter
}
