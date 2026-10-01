package data

import "path/filepath"

// TeleLocation is a place a teleporter sends a player to (teleport_location.xml).
type TeleLocation struct {
	ID      int32   `xml:"loc_id,attr"`
	MapID   int32   `xml:"mapid,attr"`
	X       float32 `xml:"posX,attr"`
	Y       float32 `xml:"posY,attr"`
	Z       float32 `xml:"posZ,attr"`
	Heading int32   `xml:"heading,attr"`
}

// Destination is one of the places an npc teleports to, and what it costs.
type Destination struct {
	LocID      int32 `xml:"loc_id,attr"`
	TeleportID int32 `xml:"teleportid,attr"` // the flight path, of flight teleporters
	Price      int32 `xml:"price,attr"`
}

// Teleporter is a teleporter_template of npc_teleporter.xml.
type Teleporter struct {
	NPC        int32         `xml:"npc_id,attr"`
	TeleportID int32         `xml:"teleportId,attr"` // the map the client shows
	Type       string        `xml:"type,attr"`       // REGULAR or FLIGHT
	Locations  []Destination `xml:"locations>telelocation"`
}

func (t *Teleporter) Destination(loc int32) *Destination {
	for i := range t.Locations {
		if t.Locations[i].LocID == loc {
			return &t.Locations[i]
		}
	}
	return nil
}

func loadTeleporters(dir string) (map[int32]*Teleporter, map[int32]*TeleLocation, error) {
	var teleporters struct {
		List []*Teleporter `xml:"teleporter_template"`
	}
	if err := loadXML(filepath.Join(dir, "npc_teleporter.xml"), &teleporters); err != nil {
		return nil, nil, err
	}
	var places struct {
		List []*TeleLocation `xml:"teleloc_template"`
	}
	if err := loadXML(filepath.Join(dir, "teleport_location.xml"), &places); err != nil {
		return nil, nil, err
	}
	byNPC, byID := map[int32]*Teleporter{}, map[int32]*TeleLocation{}
	for _, t := range teleporters.List {
		byNPC[t.NPC] = t
	}
	for _, l := range places.List {
		byID[l.ID] = l
	}
	return byNPC, byID, nil
}
