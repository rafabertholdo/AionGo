package game

import (
	"slices"
	"testing"

	"aionlightning/game/data"
)

// TestBuyAndSell has a player buy the first thing a shop sells and sell it back for a fifth of its price.
func TestBuyAndSell(t *testing.T) {
	d := staticDataOrSkip(t)
	s := testServer(d)
	var npcs []int32
	for id, list := range d.TradeLists {
		if d.Npcs[id] != nil && !list.Abyss && len(list.Tabs) > 0 && len(d.GoodsLists[list.Tabs[0]]) > 0 {
			npcs = append(npcs, id)
		}
	}
	if len(npcs) == 0 {
		t.Skip("no shop in the data")
	}
	slices.Sort(npcs)
	list := d.TradeLists[npcs[0]]
	var item *data.ItemTemplate
	for _, id := range d.GoodsLists[list.Tabs[0]] {
		if it := d.Items[id]; it != nil && it.Price > 0 {
			item = it
			break
		}
	}
	if item == nil {
		t.Skip("the shop sells nothing with a price")
	}
	p, tap := fighter(t, s, 1000)
	o := monster(t, s, 1005)
	o.npc = d.Npcs[npcs[0]]
	s.visMu.Lock()
	defer s.visMu.Unlock()
	p.kinah.Count = 1_000_000
	if s.buyFromShop(p, o, []goods{{data.Kinah, 1}}) || len(p.cube) != 0 {
		t.Fatalf("bought what the shop doesn't sell")
	}
	if !s.buyFromShop(p, o, []goods{{item.ID, 1}}) {
		t.Fatalf("couldn't buy item %d", item.ID)
	}
	price := kinahForBuy(kinahForBuy(int64(item.Price)) * int64(list.SellRate) / 100)
	if p.kinah.Count != 1_000_000-price || len(p.cube) != 1 {
		t.Errorf("kinah %d, want %d; %d items", p.kinah.Count, 1_000_000-price, len(p.cube))
	}
	if tap.count(smAddItems) == 0 {
		t.Errorf("the client wasn't told of the item")
	}
	before := p.kinah.Count
	if !s.sellToShop(p, []goods{{p.cube[0].UniqueID, 1}}) || len(p.cube) != 0 {
		t.Fatalf("couldn't sell it")
	}
	if got, want := p.kinah.Count-before, int64(item.Price)*20/100; got != want {
		t.Errorf("sold for %d, want %d", got, want)
	}
}
