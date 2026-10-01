package game

import (
	"slices"
	"strings"
	"testing"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func TestCharacterInfoHasPlayerInfosLayout(t *testing.T) {
	// PlayerInfo.writePlayerInfo writes 456 bytes whatever the name's length:
	// the name in 44, and the equipment in 208.
	s := &Server{data: &data.Data{Experience: []int64{0, 650}, Items: map[int32]*data.ItemTemplate{
		1: {ID: 1, Slot: 8, EquipmentType: "ARMOR"},
	}}}
	for _, name := range []string{"Ab", "Wrathchild", "Abcdefghijklmnop"} {
		ch := &character{
			Character:  &store.Character{ID: 5, Name: name, Race: "ELYOS", Gender: "FEMALE", Class: "MAGE", Created: time.Now()},
			appearance: &store.Appearance{Height: 1},
			equipment:  []*store.Item{{ItemID: 1, Equipped: true, Slot: 8}},
		}
		w := &wire.Writer{}
		s.writeCharacterInfo(w, ch)
		if len(w.Data) != 456 {
			t.Fatalf("%s: %d bytes", name, len(w.Data))
		}
	}
}

func TestNames(t *testing.T) {
	if convertName("wRATHchild") != "Wrathchild" || convertName("") != "" {
		t.Fatal(convertName("wRATHchild"))
	}
	names, _ := compileNamePattern("")
	if !names.valid("Ab") || names.valid("A") || names.valid("Abc1") || names.valid("Abcdefghijklmnopq") {
		t.Fatal("name pattern")
	}
}

func TestIDFactorySkipsUsedIDs(t *testing.T) {
	f := newIDFactory([]int32{1, firstObjectID, firstObjectID + 1, firstObjectID + 3})
	if a, b := f.nextID(), f.nextID(); a != firstObjectID+2 || b != firstObjectID+4 {
		t.Fatal(a, b)
	}
	f.release(firstObjectID + 2)
	if f.nextID() != firstObjectID+2 {
		t.Fatal("released id not reused")
	}
}

func TestCharactersNeverPlayedFirstThenMostRecent(t *testing.T) {
	at := func(name string, lastOnline time.Time) *character {
		return &character{Character: &store.Character{Name: name, LastOnline: lastOnline}}
	}
	now := time.Now()
	characters := []*character{at("old", now.Add(-time.Hour)), at("recent", now), at("new", time.Time{})}
	slices.SortStableFunc(characters, byLastOnline)
	var names []string
	for _, c := range characters {
		names = append(names, c.Name)
	}
	if strings.Join(names, " ") != "new recent old" {
		t.Fatal(names)
	}
}
