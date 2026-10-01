package game

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"strings"
	"testing"
)

// TestNpcInfoMatchesClient19 rebuilds each SM_NPC_INFO the 1.9 server sent
// Wrathchild (testdata/npc-info-1.9.txt) from its npc's template and where it stood.
func TestNpcInfoMatchesClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	f, err := os.Open("testdata/npc-info-1.9.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := &Server{data: d}
	p := wrathchild(s)
	var checked int
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		want, err := hex.DecodeString(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		f32 := func(at int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(want[at:])) }
		npc := d.Npcs[int32(binary.LittleEndian.Uint32(want[16:]))]
		if npc == nil {
			t.Fatalf("npc %d isn't in the static data", binary.LittleEndian.Uint32(want[16:]))
		}
		o := &object{id: int32(binary.LittleEndian.Uint32(want[12:])), x: f32(0), y: f32(4), z: f32(8), npc: npc}
		o.heading = want[27]
		// The static id is 18 bytes from the end, after the move type.
		o.staticID = int32(binary.LittleEndian.Uint16(want[len(want)-18:]))
		got := s.npcInfo(o, p).Data[1:]
		// Some of the 1.9 server's npcs had walked already, and report their speed.
		if speed := want[65 : 65+4]; string(speed) != "\x00\x00\x00\x00" && len(want) == len(got) {
			copy(got[65:], speed)
		}
		if hex.EncodeToString(got) != fields[1] {
			t.Errorf("npc %d (object %d) differs from the 1.9 server's:\n%s", npc.ID, o.id, diffHex(hex.EncodeToString(got), fields[1]))
		}
		checked++
	}
	if checked != 72 {
		t.Errorf("checked %d packets", checked)
	}
}

func TestGatherableInfoMatchesClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	raw, err := os.ReadFile("testdata/gatherable-info-1.9.txt")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	for _, line := range lines {
		want, err := hex.DecodeString(strings.Fields(line)[1])
		if err != nil {
			t.Fatal(err)
		}
		f32 := func(at int) float32 { return math.Float32frombits(binary.LittleEndian.Uint32(want[at:])) }
		id := int32(binary.LittleEndian.Uint32(want[20:]))
		template := d.Gatherables[id]
		if template == nil {
			t.Fatalf("gatherable %d isn't in the static data", id)
		}
		o := &object{id: int32(binary.LittleEndian.Uint32(want[12:])), x: f32(0), y: f32(4), z: f32(8),
			staticID: int32(binary.LittleEndian.Uint32(want[16:])), gatherable: template}
		if got := hex.EncodeToString(gatherableInfo(o).Data[1:]); got != strings.Fields(line)[1] {
			t.Errorf("gatherable %d differs from the 1.9 server's:\n%s", id, diffHex(got, strings.Fields(line)[1]))
		}
	}
}
