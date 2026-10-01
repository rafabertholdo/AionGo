package data

import (
	"path/filepath"
	"slices"
)

// Zone is a zones/zones_<map>.xml zone: a polygon of a map, between a bottom and a top, that says whether flying is
// allowed and whether it is breathable (no drowning).
type Zone struct {
	Name      string
	MapID     int32
	Priority  int32 // a lower one is the more important
	Fly       bool
	Breath    bool
	Top       float32
	Bottom    float32
	X, Y      []float32
	Neighbors []*Zone // the zones linked to it, by priority
}

// Contains is ZoneService.checkPointInZone: the height is checked first (unless the zone has none), then the
// point in the polygon.
func (z *Zone) Contains(x, y, height float32) bool {
	if (z.Top != 0 || z.Bottom != 0) && (height > z.Top || height < z.Bottom) {
		return false
	}
	inside := false
	for i, j := 0, len(z.X)-1; i < len(z.X); j, i = i, i+1 {
		if z.Y[i] < y && z.Y[j] >= y || z.Y[j] < y && z.Y[i] >= y {
			if z.X[i]+(y-z.Y[i])/(z.Y[j]-z.Y[i])*(z.X[j]-z.X[i]) < x {
				inside = !inside
			}
		}
	}
	return inside
}

type zoneXML struct {
	Name     string `xml:"name,attr"`
	MapID    int32  `xml:"mapid,attr"`
	Priority int32  `xml:"priority,attr"`
	Fly      bool   `xml:"fly,attr"`
	Breath   bool   `xml:"breath,attr"`
	Points   struct {
		Top    float32 `xml:"top,attr"`
		Bottom float32 `xml:"bottom,attr"`
		Points []struct {
			X float32 `xml:"x,attr"`
			Y float32 `xml:"y,attr"`
		} `xml:"point"`
	} `xml:"points"`
	Links []string `xml:"link"`
}

// loadZones is ZoneData and ZoneService.initializeZones: every zone of every map, and each one's neighbors.
// ponytail: AL-Game's tree sets put a zone before the others of the same priority; here the file's order stays.
func loadZones(dir string) (map[int32][]*Zone, error) {
	files, err := filepath.Glob(filepath.Join(dir, "zones/zones_*.xml"))
	if err != nil {
		return nil, err
	}
	byName := map[string]*Zone{}
	links := map[*Zone][]string{}
	zones := map[int32][]*Zone{}
	for _, file := range files {
		var doc struct {
			Zones []zoneXML `xml:"zone"`
		}
		if err := loadXML(file, &doc); err != nil {
			return nil, err
		}
		for _, x := range doc.Zones {
			z := &Zone{Name: x.Name, MapID: x.MapID, Priority: x.Priority, Fly: x.Fly, Breath: x.Breath,
				Top: x.Points.Top, Bottom: x.Points.Bottom}
			for _, p := range x.Points.Points {
				z.X, z.Y = append(z.X, p.X), append(z.Y, p.Y)
			}
			byName[z.Name] = z
			links[z] = x.Links
			zones[z.MapID] = append(zones[z.MapID], z)
		}
	}
	byPriority := func(a, b *Zone) int { return int(a.Priority - b.Priority) }
	for z, names := range links {
		for _, name := range names {
			if n := byName[name]; n != nil {
				z.Neighbors = append(z.Neighbors, n)
			}
		}
		slices.SortStableFunc(z.Neighbors, byPriority)
	}
	for _, list := range zones {
		slices.SortStableFunc(list, byPriority)
	}
	return zones, nil
}
