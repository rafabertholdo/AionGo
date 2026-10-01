package data

import "path/filepath"

// loadExpanders reads what each npc charges to expand the cube or the warehouse to a level: npc id → level → price.
func loadExpanders(dir, file, element string) (map[int32]map[int32]int32, error) {
	var xmlFile struct {
		Cube []expander `xml:"cube_npc"`
		Ware []expander `xml:"warehouse_npc"`
	}
	if err := loadXML(filepath.Join(dir, file), &xmlFile); err != nil {
		return nil, err
	}
	npcs := xmlFile.Cube
	if element == "warehouse" {
		npcs = xmlFile.Ware
	}
	out := map[int32]map[int32]int32{}
	for _, n := range npcs {
		out[n.ID] = map[int32]int32{}
		for _, e := range n.Expand {
			out[n.ID][e.Level] = e.Price
		}
	}
	return out, nil
}

type expander struct {
	ID     int32 `xml:"id,attr"`
	Expand []struct {
		Level int32 `xml:"level,attr"`
		Price int32 `xml:"price,attr"`
	} `xml:"expand"`
}
