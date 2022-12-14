package trie

type trieCapture struct {
	insert map[string]struct{}
	delete map[string]struct{}
	origin map[string][]byte
}

// newTracer initializes the tracer for capturing trie changes.
func newTracer() *trieCapture {
	return &trieCapture{
		insert: make(map[string]struct{}),
		delete: make(map[string]struct{}),
		origin: make(map[string][]byte),
	}
}

// onRead tracks the newly loaded trie node and caches the rlp-encoded blob internally.
// Don't change the value outside of function since it's not deep-copied.
func (t *trieCapture) onRead(path []byte, val []byte) {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return
	}
	t.origin[string(path)] = val
}

// onInsert tracks the newly inserted trie node. If it's already in the deletion set
// (resurrected node), then just wipe it from the deletion set as the "untouched".
func (t *trieCapture) onInsert(path []byte) {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return
	}
	if _, present := t.delete[string(path)]; present {
		delete(t.delete, string(path))
		return
	}
	t.insert[string(path)] = struct{}{}
}

// onDelete tracks the newly deleted trie node. If it's already
// in the addition set, then just wipe it from the addition set
// as it's untouched.
func (t *trieCapture) onDelete(path []byte) {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return
	}
	if _, present := t.insert[string(path)]; present {
		delete(t.insert, string(path))
		return
	}
	t.delete[string(path)] = struct{}{}
}

// insertList returns the tracked inserted trie nodes in list format.
func (t *trieCapture) insertList() [][]byte {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return nil
	}
	var ret [][]byte
	for path := range t.insert {
		ret = append(ret, []byte(path))
	}
	return ret
}

// deleteList returns the tracked deleted trie nodes in list format.
func (t *trieCapture) deleteList() [][]byte {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return nil
	}
	var ret [][]byte
	for path := range t.delete {
		ret = append(ret, []byte(path))
	}
	return ret
}

// getOldv returns the cached original value of the specified node.
func (t *trieCapture) getOldv(path []byte) []byte {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return nil
	}
	return t.origin[string(path)]
}

// reset clears the content tracked by tracer.
func (t *trieCapture) reset() {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return
	}
	t.insert = make(map[string]struct{})
	t.delete = make(map[string]struct{})
	t.origin = make(map[string][]byte)
}

// copy returns a deep copied tracer instance.
func (t *trieCapture) copy() *trieCapture {
	// Tracer isn't used right now, remove this check later.
	if t == nil {
		return nil
	}
	var (
		insert = make(map[string]struct{})
		delete = make(map[string]struct{})
		origin = make(map[string][]byte)
	)
	for key := range t.insert {
		insert[key] = struct{}{}
	}
	for key := range t.delete {
		delete[key] = struct{}{}
	}
	for key, val := range t.origin {
		origin[key] = val
	}
	return &trieCapture{
		insert: insert,
		delete: delete,
		origin: origin,
	}
}
