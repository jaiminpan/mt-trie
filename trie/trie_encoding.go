package trie

func hexToCompact(hex []byte) []byte {
	terminator := byte(0)
	if hasTerm(hex) {
		terminator = 1
		hex = hex[:len(hex)-1]
	}
	buf := make([]byte, len(hex)/2+1)
	buf[0] = terminator << 5 // the flag byte
	if len(hex)&1 == 1 {
		buf[0] |= 1 << 4 // odd flag
		buf[0] |= hex[0] // first nibble is contained in the first byte
		hex = hex[1:]
	}
	decodeNibbles(hex, buf[1:])
	return buf
}

func compactToHex(compact []byte) []byte {
	if len(compact) == 0 {
		return compact
	}
	base := keybytesToHex(compact)
	// delete terminator flag
	if base[0] < 2 {
		base = base[:len(base)-1]
	}
	// apply odd flag
	chop := 2 - base[0]&1
	return base[chop:]
}

func keybytesToHex(str []byte) []byte {
	l := len(str)*2 + 1
	var nibbles = make([]byte, l)
	for i, b := range str {
		nibbles[i*2] = b / 16
		nibbles[i*2+1] = b % 16
	}
	nibbles[l-1] = 16
	return nibbles
}

func decodeNibbles(nibbles []byte, bytes []byte) {
	for bi, ni := 0, 0; ni < len(nibbles); bi, ni = bi+1, ni+2 {
		bytes[bi] = nibbles[ni]<<4 | nibbles[ni+1]
	}
}

// prefixLen returns the length of the common prefix of a and b.
func prefixLen(a, b []byte) int {
	a_len, b_len := len(a), len(b)
	length := a_len
	if b_len < a_len {
		length = b_len
	}

	for i := 0; i < length; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return length
}

func prefixConcat(s1 []byte, s2 ...byte) []byte {
	r := make([]byte, len(s1)+len(s2))
	copy(r, s1)
	copy(r[len(s1):], s2)
	return r
}
