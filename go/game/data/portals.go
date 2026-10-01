package data

import "path/filepath"

// Point is a place in a map.
type Point struct {
	MapID int32   `xml:"mapid,attr"`
	X     float32 `xml:"x,attr"`
	Y     float32 `xml:"y,attr"`
	Z     float32 `xml:"z,attr"`
	Race  string  `xml:"race,attr"`
}

// Portal is a portal of portal_templates.xml: an npc that takes players to an instance or another place.
type Portal struct {
	NPC      int32   `xml:"npcid,attr"`
	Instance bool    `xml:"instance,attr"`
	Group    bool    `xml:"group,attr"`
	MinLevel int32   `xml:"minlevel,attr"`
	MaxLevel int32   `xml:"maxlevel,attr"`
	Race     string  `xml:"race,attr"`
	TitleID  int32   `xml:"titleid,attr"`
	Entry    []Point `xml:"entrypoint"` // where a player is sent when it leaves the instance, by race
	Exit     Point   `xml:"exitpoint"`  // where the portal leads
}

func loadPortals(dir string) (map[int32]*Portal, []*Portal, error) {
	var file struct {
		List []*Portal `xml:"portal"`
	}
	if err := loadXML(filepath.Join(dir, "portals/portal_templates.xml"), &file); err != nil {
		return nil, nil, err
	}
	portals := map[int32]*Portal{}
	for _, p := range file.List {
		portals[p.NPC] = p
	}
	return portals, file.List, nil
}

// InstancePortal is PortalData.getInstancePortalTemplate: the portal that leads into the instance map for the race.
func (d *Data) InstancePortal(world int32, race string) *Portal {
	for _, p := range d.PortalList {
		if p.Instance && p.Exit.MapID == world && (p.Race == "" || p.Race == race || p.Race == "ALL") {
			return p
		}
	}
	return nil
}

// EntryFor is where a player of the race leaves the portal's instance to.
func (p *Portal) EntryFor(race string) *Point {
	for i := range p.Entry {
		if p.Entry[i].Race == "" || p.Entry[i].Race == race {
			return &p.Entry[i]
		}
	}
	return nil
}
