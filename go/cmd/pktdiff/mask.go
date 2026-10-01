package main

import "encoding/binary"

// masker blanks the fields that differ between runs of the same scenario, so
// only real protocol differences remain.
type masker struct {
	kinds map[string]bool
	ids   map[uint32]bool // object ids seen so far in this log, masked wherever they appear afterwards
}

func newMasker(_ []Packet, kinds []string) *masker {
	return &masker{kinds: set(kinds), ids: map[uint32]bool{}}
}

// learn records the object ids a server packet introduces.
func (m *masker) learn(op string, body []byte) {
	for _, f := range fields[op] {
		if f.kind == "id" && m.kinds["id"] && f.offset+4 <= len(body) {
			m.ids[binary.LittleEndian.Uint32(body[f.offset:])] = true
		}
	}
}

func (m *masker) apply(op string, body []byte) []byte {
	m.learn(op, body)
	out := append([]byte(nil), body...)
	for _, f := range fields[op] {
		if m.kinds[f.kind] && f.offset+f.length <= len(out) {
			for i := 0; i < f.length; i++ {
				out[f.offset+i] = 0xEE
			}
		}
	}
	// Ids also appear inside other packets (attacker, owner, target).
	for i := 0; m.kinds["id"] && i+4 <= len(out); i++ {
		if id := binary.LittleEndian.Uint32(out[i:]); id > 0xFF && m.ids[id] {
			copy(out[i:], []byte{0xEE, 0xEE, 0xEE, 0xEE})
			i += 3
		}
	}
	return out
}
