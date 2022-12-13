



#### Types

type Trie struct {

}

#### Method

func NewTrie(root Hash, db TrieDB) (Trie, error)
```
trieDB := NewTrieDB()
hash := common.Hash("0xe1d943cc8f061a0c0b98162830b970395ac9315654824bf21b73b891365262f9")
trie, err := NewTrie(hash, trieDB)
```

func Hash() (error)
```
rootHash := trie.Hash()
```

func (Trie) Get(key []byte) ([]byte, error)
```
val, err := trie.Get([]byte("120"))
```

? func (Trie) GetNode(path []byte) ([]byte, error)
```
val, err := trie.Get([]byte("120"))
```

func (Trie) Update(key, value []byte) error
```
err := trie.Update([]byte("120"), []byte("qwe"))
```

func (Trie) Delete(key [byte]) error
```
err := trie.Delete([]byte("120"))
```

func (Trie) Commit() (Hash, error)
```
rootHash, nodeSet, err := trie.Commit()
```

#### Types
type TrieDB struct {

}

#### Method

func NewTrieDB(db DiskDB) (TrieDB, error)

func (TrieDB) Update(NodeSet) (error)
```
rootHash, nodeSet, err := trie.Commit()
triedb.Update(nodeSet)
```

func (TrieDB) Commit(Hash) (error)
```
rootHash, nodeSet, err := trie.Commit()
triedb.Update(nodeSet)
triedb.Commit(rootHash)
```

func (TrieDB) flush() (error)


? func (TrieDB) DeleteTrie(Hash) (error)
```
rootHash, nodeSet, err := trie.Commit()
triedb.Update(nodeSet)
```

#### Types
type NodeSet struct {

}

type DiskDB struct {

}



## test case
block insert => trie insert
block comfirm ? =? triedb commit ?
block rewind => triedb interator delete hash trie ? 


