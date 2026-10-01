package game

import (
	"encoding/binary"
	"math"
	"testing"

	"aionlightning/game/store"
)

// TestSiege has an admin capture a fortress for the Elyos and checks the owner, the broadcast and the influence.
func TestSiege(t *testing.T) {
	s := testServer(staticDataOrSkip(t))
	ann, _, annTap, bobTap := twoPlayers(t, s)
	if len(s.data.Sieges) == 0 {
		t.Skip("no siege locations in the static data")
	}
	saved := map[int32]store.SiegeOwner{}
	s.siegeDB = fakeSiegeDB(saved)
	fort := s.data.Sieges[0]
	if fort.Type != "FORTRESS" {
		t.Fatalf("first location is %s", fort.Type)
	}
	s.visMu.Lock()
	defer s.visMu.Unlock()
	s.adminSiege(ann, []string{"cap", "1011", "ely", "7"})
	if st := s.sieges[1011]; st == nil || st.race != "ELYOS" || st.legion != 7 || st.vulnerable {
		t.Fatalf("not captured: %+v", st)
	}
	if saved[1011] != (store.SiegeOwner{Race: "ELYOS", Legion: 7}) {
		t.Errorf("not saved: %v", saved)
	}
	if annTap.count(smSiegeLocationInfo) != 1 || bobTap.count(smInfluenceRatio) != 1 {
		t.Errorf("not broadcast: %v %v", annTap.counts, bobTap.counts)
	}
	w := s.influenceRatio()
	elyos := math.Float32frombits(binary.LittleEndian.Uint32(w.Data[5:9]))
	if elyos <= 0 || elyos >= 1 {
		t.Errorf("Elyos influence %v", elyos)
	}
	s.adminSiege(ann, []string{"set", "1011", "vul", "vul"})
	if st := s.sieges[1011]; !st.vulnerable || st.nextState != 1 {
		t.Errorf("not vulnerable: %+v", st)
	}
	s.adminSiege(ann, []string{"capture", "1011", "balaur"})
	if s.sieges[1011].legion != 0 || s.sieges[1011].race != "BALAUR" {
		t.Errorf("not recaptured: %+v", s.sieges[1011])
	}
	s.adminSiege(ann, []string{"capture", "999999", "ely"})
	s.adminSiege(ann, []string{"list"})
}

type fakeSiegeDB map[int32]store.SiegeOwner

func (f fakeSiegeDB) SaveSiegeOwner(id int32, o store.SiegeOwner) error { f[id] = o; return nil }
