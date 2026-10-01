package data

import "path/filepath"

// TradeList is a tradelist_template of npc_trade_list.xml: what an npc sells, as goods lists.
type TradeList struct {
	NPC      int32
	Abyss    bool
	SellRate int32 // the percent of the price the npc asks
	Tabs     []int32
}

// loadShops reads the npcs' trade lists and the goods lists they name (which list which items).
func loadShops(dir string) (map[int32]*TradeList, map[int32][]int32, error) {
	var lists struct {
		Templates []struct {
			NPC      int32 `xml:"npc_id,attr"`
			Abyss    bool  `xml:"abyss,attr"`
			SellRate int32 `xml:"sell_price_rate,attr"`
			Tabs     []struct {
				ID int32 `xml:"id,attr"`
			} `xml:"tradelist"`
		} `xml:"tradelist_template"`
	}
	if err := loadXML(filepath.Join(dir, "npc_trade_list.xml"), &lists); err != nil {
		return nil, nil, err
	}
	var goods struct {
		Lists []struct {
			ID    int32 `xml:"id,attr"`
			Items []struct {
				ID int32 `xml:"id,attr"`
			} `xml:"item"`
		} `xml:"list"`
	}
	if err := loadXML(filepath.Join(dir, "goodslists/goodslists.xml"), &goods); err != nil {
		return nil, nil, err
	}
	trade := map[int32]*TradeList{}
	for _, t := range lists.Templates {
		l := &TradeList{NPC: t.NPC, Abyss: t.Abyss, SellRate: t.SellRate}
		if l.SellRate == 0 {
			l.SellRate = 100
		}
		for _, tab := range t.Tabs {
			l.Tabs = append(l.Tabs, tab.ID)
		}
		trade[t.NPC] = l
	}
	items := map[int32][]int32{}
	for _, l := range goods.Lists {
		for _, item := range l.Items {
			items[l.ID] = append(items[l.ID], item.ID)
		}
	}
	return trade, items, nil
}
