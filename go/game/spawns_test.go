package game

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"math"
	"os"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/wire"
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
		s.initNpc(o)
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

func TestNpcCorpseVisibilityAndRespawn(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.spawn(p)
		o := monster(t, s, 1005)
		defer func() {
			o.decay.cancel()
			o.respawn.cancel()
			o.ai.stop()
			o.move.stop()
		}()
		var info []byte
		p.conn.tap = func(w *wire.Writer) {
			if w.Data[0] == smNpcInfo {
				info = append([]byte(nil), w.Data[1:]...)
			}
		}
		s.visMu.Lock()
		o.hp = o.maxHP / 2
		injured := s.npcInfo(o, p).Data[1:]
		if got, want := injured[49], byte(100*int64(o.hp)/int64(o.maxHP)); got != want {
			t.Errorf("injured HP = %d%%, want %d%%", got, want)
		}
		s.reduceNpcHP(o, o.maxHP, nil)
		// Re-enter visibility while the corpse still exists. It must stay dead
		// on the client, rather than look like a fresh spawn with 100% HP.
		p.X = 1200
		s.updateKnown(p)
		p.X = 1000
		s.updateKnown(p)
		if len(info) == 0 {
			t.Error("corpse was not announced on re-entering visibility")
		} else {
			if state := binary.LittleEndian.Uint16(info[25:]); state != o.state {
				t.Errorf("corpse state = %#x, want %#x", state, o.state)
			}
			if info[49] != 0 {
				t.Errorf("corpse HP = %d%%, want 0%%", info[49])
			}
		}
		s.visMu.Unlock()
		time.Sleep(decayTime(o))
		synctest.Wait()
		s.visMu.Lock()
		if p.seen[o.id] != nil {
			t.Error("decayed corpse remains visible")
		}
		info = nil
		s.visMu.Unlock()
		time.Sleep(time.Duration(o.interval)*time.Second - decayTime(o))
		synctest.Wait()
		s.visMu.Lock()
		defer s.visMu.Unlock()
		if o.dead || o.hp != o.maxHP || p.seen[o.id] != o {
			t.Fatalf("respawn: dead=%v HP=%d/%d seen=%v", o.dead, o.hp, o.maxHP, p.seen[o.id] == o)
		}
		if len(info) == 0 || info[49] != 100 || binary.LittleEndian.Uint16(info[25:]) != o.state {
			t.Fatal("respawn was not announced alive with full HP")
		}
		p.targetID = o.id
		selected := s.targetSelected(p).Data[1:]
		if hp := int32(binary.LittleEndian.Uint32(selected[10:])); hp != o.maxHP {
			t.Errorf("selected respawn HP = %d, want %d", hp, o.maxHP)
		}
		s.playerUseSkill(p, d.Skills[1351], 0, 0, 0, 0)
		if p.cast == nil {
			t.Fatal("cannot cast Flame Bolt on the respawn")
		}
		s.cancelSkill(p)
	})
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
