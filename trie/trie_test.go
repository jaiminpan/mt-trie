package trie

import (
	"bytes"
	"testing"

	"github.com/jaiminpan/mt-trie/accdb"
	"github.com/jaiminpan/mt-trie/accdb/memorydb"
)

func NewMemoryDatabase() accdb.KeyValueStore {
	return memorydb.New()
}

func TestEmptyTrie(t *testing.T) {
	triedb := NewTrieDB(NewMemoryDatabase())
	trie := NewEmpty(triedb)
	res := trie.Hash()
	if res != emptyRoot {
		t.Errorf("expected %x got %x", emptyRoot, res)
	}
}

func TestMemoryUpdate(t *testing.T) {
	triedb := NewTrieDB(NewMemoryDatabase())
	trie := NewEmpty(triedb)

	key := make([]byte, 32)
	value := []byte("test")
	trie.Update(key, value)
	if !bytes.Equal(trie.Get(key), value) {
		t.Fatal("wrong value")
	}
}

func TestUpdate(t *testing.T) {
	triedb := NewTrieDB(NewMemoryDatabase())
	trie := NewEmpty(triedb)

	trie.Update([]byte("120000"), []byte("qwerqwerqwerqwerqwerqwerqwerqwer"))
	trie.Update([]byte("123456"), []byte("asdfasdfasdfasdfasdfasdfasdfasdf"))
	root, nodes, _ := trie.Commit(false)

	merged := NewMergedNodeSet()
	merged.Merge(nodes)

	triedb.Update(merged)
	triedb.Commit(root)

	trie, _ = New(TrieID(root), triedb)

	_, err := trie.TryGet([]byte("120000"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = trie.TryGet([]byte("123456"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	_, err = trie.TryGet([]byte("120099"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

/*
func TestRollback(t *testing.T) {

	triedb := NewTrieDB(NewMemoryDatabase())
	trie := NewEmpty(triedb)

	trie.Update([]byte("120000"), []byte("qwerqwerqwerqwerqwerqwerqwerqwer"))
	trie.Update([]byte("123456"), []byte("asdfasdfasdfasdfasdfasdfasdfasdf"))
	root, nodes, _ := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes))
	triedb.Commit(root)

	trie, _ = New(TrieID(root), triedb)
	_, err := trie.TryGet([]byte("120000"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	trie, _ = New(TrieID(root), triedb)
	err = trie.TryUpdate([]byte("120099"), []byte("zxcvzxcvzxcvzxcvzxcvzxcvzxcvzxcv"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = trie.TryUpdate([]byte("120000"), []byte("uiuiuiuiuiuiuiiuiui"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	err = trie.TryDelete([]byte("123456"))
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	root2, nodes2, _ := trie.Commit(false)
	triedb.Update(NewWithNodeSet(nodes2))
	triedb.Commit(root2)

	trie, _ = New(TrieID(root), triedb)
	if !bytes.Equal(trie.Get([]byte("120000")), []byte("qwerqwerqwerqwerqwerqwerqwerqwer")) {
		t.Fatal("wrong value")
	}

	trie, _ = New(TrieID(root2), triedb)
	if !bytes.Equal(trie.Get([]byte("120000")), []byte("uiuiuiuiuiuiuiiuiui")) {
		t.Fatal("wrong value")
	}
}


insert into trie
delete from trie
rollback to specific Hash

*/
