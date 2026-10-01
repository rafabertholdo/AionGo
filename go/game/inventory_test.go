package game

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"aionlightning/game/store"
)

// playSession is the packets of testdata/play-session-1.9.txt: direction, opcode name and payload, in order.
func playSession(t *testing.T) (rows [][3]string) {
	t.Helper()
	f, err := os.Open("testdata/play-session-1.9.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<22)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 5 {
			rows = append(rows, [3]string{fields[1], fields[2], fields[4]})
		}
	}
	return rows
}

// TestLootedItemsMatchClient19 rebuilds the SM_ADD_ITEMS of the loot the 1.9 server gave Wrathchild, and the
// SM_UPDATE_ITEMs that followed when the client put each item in a slot.
func TestLootedItemsMatchClient19(t *testing.T) {
	d := staticDataOrSkip(t)
	s := &Server{data: d}
	p := wrathchild(s)
	rows := playSession(t)
	checked := 0
	for i, row := range rows {
		if row[0] != "server" || row[1] != "SM_ADD_ITEMS" {
			continue
		}
		payload, _ := hex.DecodeString(row[2])
		// H 25, H 1, then D object id, D item id, H 0x24, D name id, H 0, H 0x16, C 0, H mask, Q count …
		if len(payload) < 40 || binary.LittleEndian.Uint16(payload[2:]) != 1 {
			continue
		}
		item := &store.Item{UniqueID: int32(binary.LittleEndian.Uint32(payload[4:])), ItemID: int32(binary.LittleEndian.Uint32(payload[8:])),
			Count: int64(binary.LittleEndian.Uint64(payload[25:])), Slot: int32(binary.LittleEndian.Uint16(payload[len(payload)-3:]))}
		if t := d.Items[item.ItemID]; t == nil || t.IsEquipment() || t.IsStigma() {
			continue
		}
		if got := hex.EncodeToString(s.addItemsPacket(p, item).Data[1:]); got != row[2] {
			t.Errorf("SM_ADD_ITEMS for item %d differs from the 1.9 server's:\n%s", item.ItemID, diffHex(got, row[2]))
		}
		checked++
		// The next SM_UPDATE_ITEM of that object is the client's CM_MOVE_ITEM answered.
		for _, later := range rows[i+1 : min(i+8, len(rows))] {
			if later[0] != "server" || later[1] != "SM_UPDATE_ITEM" {
				continue
			}
			update, _ := hex.DecodeString(later[2])
			if int32(binary.LittleEndian.Uint32(update)) != item.UniqueID {
				continue
			}
			item.Slot = int32(binary.LittleEndian.Uint16(update[len(update)-2:]))
			if got := hex.EncodeToString(s.updateItemPacket(p, item).Data[1:]); got != later[2] {
				t.Errorf("SM_UPDATE_ITEM for item %d differs from the 1.9 server's:\n%s", item.ItemID, diffHex(got, later[2]))
			}
			checked++
			break
		}
	}
	if checked < 2 {
		t.Errorf("only %d packets were checked", checked)
	}
}
