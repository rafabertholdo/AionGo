package data

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
)

// NpcTemplate is an npc_template of npcs/*.xml.
type NpcTemplate struct {
	ID      int32   `xml:"npc_id,attr"`
	Level   int32   `xml:"level,attr"`
	Name    string  `xml:"name,attr"`
	NameID  int32   `xml:"name_id,attr"`
	TitleID int32   `xml:"title_id,attr"`
	Type    string  `xml:"npc_type,attr"`
	Height  float32 `xml:"height,attr"`
	Tribe   string  `xml:"tribe,attr"`
	HPGauge int32   `xml:"hpgauge,attr"`
	State   int32   `xml:"state,attr"`
	Rank    string  `xml:"rank,attr"`
	Race    string  `xml:"race,attr"`
	SRange  int32   `xml:"srange,attr"` // how far it notices players: the aggro range
	Stats   struct {
		MaxHP         int32   `xml:"maxHp,attr"`
		MaxMP         int32   `xml:"maxMp,attr"`
		MaxXP         int32   `xml:"maxXp,attr"`
		Block         int32   `xml:"block,attr"`
		Parry         int32   `xml:"parry,attr"`
		MDef          int32   `xml:"mdef,attr"`
		PDef          int32   `xml:"pdef,attr"`
		Crit          int32   `xml:"crit,attr"`
		Power         int32   `xml:"power,attr"`
		Evasion       int32   `xml:"evasion,attr"`
		Accuracy      int32   `xml:"accuracy,attr"`
		MagicAccuracy int32   `xml:"magic_accuracy,attr"`
		WalkSpeed     float32 `xml:"walk_speed,attr"`
		RunSpeed      float32 `xml:"run_speed,attr"`
		RunSpeedFight float32 `xml:"run_speed_fight,attr"`
	} `xml:"stats"`
	Equipment []int32 `xml:"equipment>item"`
	Kisk      *struct {
		UseMask    int32 `xml:"usemask,attr"`
		Members    int32 `xml:"members,attr"`
		Resurrects int32 `xml:"resurrects,attr"`
	} `xml:"kisk_stats"`
}

// GatherableTemplate is a gatherable_template of gatherables/*.xml.
type GatherableTemplate struct {
	ID           int32      `xml:"id,attr"`
	NameID       int32      `xml:"nameId,attr"`
	HarvestCount int32      `xml:"harvestCount,attr"`
	SkillLevel   int32      `xml:"skillLevel,attr"`
	HarvestSkill int32      `xml:"harvestSkill,attr"`
	SuccessAdj   int32      `xml:"successAdj,attr"`
	FailureAdj   int32      `xml:"failureAdj,attr"`
	Materials    []Material `xml:"materials>material"`
}

// Material is what a gatherable gives: an item, and how likely it is to give it.
type Material struct {
	Rate   int32 `xml:"rate,attr"`
	NameID int32 `xml:"nameid,attr"`
	ItemID int32 `xml:"itemid,attr"`
}

// Spot is one place a spawn group can put its npc.
type Spot struct {
	X        float32 `xml:"x,attr"`
	Y        float32 `xml:"y,attr"`
	Z        float32 `xml:"z,attr"`
	Heading  int32   `xml:"h,attr"`
	Walker   int32   `xml:"w,attr"`
	Random   int32   `xml:"rw,attr"`
	StaticID int32   `xml:"staticid,attr"`
	Fly      int32   `xml:"fly,attr"`
}

// SpawnGroup is a spawn of spawns/*/*.xml: an npc, how many of it stand at
// once (its pool) and the spots they take, in order.
type SpawnGroup struct {
	Map      int32  `xml:"map,attr"`
	NpcID    int32  `xml:"npcid,attr"`
	Pool     int32  `xml:"pool,attr"`
	Interval int32  `xml:"interval,attr"`
	Time     string `xml:"time,attr"`
	Handler  string `xml:"handler,attr"`
	Anchor   string `xml:"anchor,attr"`
	Random   int32  `xml:"rw,attr"`
	Spots    []Spot `xml:"object"`
}

// RouteStep is a stop of a walking npc's route.
type RouteStep struct {
	X, Y, Z  float32
	RestTime int32 // seconds
}

// Route is a walker_template of npc_walker.xml: the stops a walking npc goes through, in order.
type Route []RouteStep

// Tribe is a tribe of tribe_relations.xml.
type Tribe struct {
	Name    string   `xml:"name,attr"`
	Base    string   `xml:"base,attr"`
	Aggro   []string `xml:"aggro>to"`
	Friend  []string `xml:"friend>to"`
	Support []string `xml:"support>to"`
	Hostile []string `xml:"hostile>to"`
}

// Relation is how the first tribe treats the second (TribeRelationsData).
func (t Tribes) has(name string, list func(*Tribe) []string, other string) bool {
	tribe := t[name]
	return tribe != nil && slices.Contains(list(tribe), other)
}

func (t Tribes) IsAggressive(name, other string) bool {
	return t.has(name, func(x *Tribe) []string { return x.Aggro }, other)
}
func (t Tribes) IsFriendly(name, other string) bool {
	return t.has(name, func(x *Tribe) []string { return x.Friend }, other)
}
func (t Tribes) IsSupport(name, other string) bool {
	return t.has(name, func(x *Tribe) []string { return x.Support }, other)
}
func (t Tribes) IsHostile(name, other string) bool {
	return t.has(name, func(x *Tribe) []string { return x.Hostile }, other)
}

// HasAggressive is whether the tribe is aggressive to any tribe.
func (t Tribes) HasAggressive(name string) bool { return t[name] != nil && len(t[name].Aggro) > 0 }

// HasHostile is whether the tribe is hostile to any tribe.
func (t Tribes) HasHostile(name string) bool { return t[name] != nil && len(t[name].Hostile) > 0 }

// IsGuard is Npc.isGuard: a guard of either race.
func (t Tribes) IsGuard(name string) bool {
	return t.isGuard(name, guardDark) || t.isGuard(name, guardLight)
}

// Tribes is tribe_relations.xml, by tribe name.
type Tribes map[string]*Tribe

// Tribe names the guards of each race go by (Tribe.GUARD_LIGHT, Tribe.GUARD_DARK).
const (
	guardLight = "GUARD"
	guardDark  = "GUARD_DARK"
)

func (t Tribes) isGuard(name, guard string) bool {
	for tribe := t[name]; tribe != nil; tribe = t[tribe.Base] {
		if tribe.Name == guard {
			return true
		}
		if tribe.Base == "" {
			break
		}
	}
	return false
}

// AggroIcon is Player.isAggroIconTo: whether npcs of the tribe show as aggressive to a player of the race.
func (t Tribes) AggroIcon(asmodian bool, tribe string) bool {
	guard, enemy := guardDark, "PC"
	if asmodian {
		guard, enemy = guardLight, "PC_DARK"
	}
	if t.isGuard(tribe, guard) {
		return true
	}
	return t[tribe] != nil && slices.Contains(t[tribe].Aggro, enemy)
}

// loadWorld reads the npcs, gatherables, tribes and spawns.
func (d *Data) loadWorld(dir string) error {
	d.Npcs = map[int32]*NpcTemplate{}
	var npcs struct {
		Templates []*NpcTemplate `xml:"npc_template"`
	}
	for _, file := range xmlFiles(filepath.Join(dir, "npcs")) {
		npcs.Templates = nil
		if err := loadXML(file, &npcs); err != nil {
			return err
		}
		for _, t := range npcs.Templates {
			d.Npcs[t.ID] = t
		}
	}
	d.Gatherables = map[int32]*GatherableTemplate{}
	var gatherables struct {
		Templates []*GatherableTemplate `xml:"gatherable_template"`
	}
	for _, file := range xmlFiles(filepath.Join(dir, "gatherables")) {
		gatherables.Templates = nil
		if err := loadXML(file, &gatherables); err != nil {
			return err
		}
		for _, t := range gatherables.Templates {
			d.Gatherables[t.ID] = t
		}
	}
	var tribes struct {
		Tribes []*Tribe `xml:"tribe"`
	}
	d.Tribes = Tribes{}
	for _, file := range xmlFiles(filepath.Join(dir, "tribe")) {
		tribes.Tribes = nil
		if err := loadXML(file, &tribes); err != nil {
			return err
		}
		for _, t := range tribes.Tribes {
			d.Tribes[t.Name] = t
		}
	}
	var walkers struct {
		Templates []struct {
			ID    int32 `xml:"route_id,attr"`
			Steps []struct {
				Step int32   `xml:"step,attr"`
				X    float32 `xml:"loc_x,attr"`
				Y    float32 `xml:"loc_y,attr"`
				Z    float32 `xml:"loc_z,attr"`
				Rest int32   `xml:"rest_time,attr"`
			} `xml:"routes>routestep"`
		} `xml:"walker_template"`
	}
	if err := loadXML(filepath.Join(dir, "npc_walker.xml"), &walkers); err != nil {
		return err
	}
	d.Walkers = map[int32]Route{}
	for _, t := range walkers.Templates {
		route := make(Route, len(t.Steps))
		for i, st := range t.Steps {
			route[i] = RouteStep{X: st.X, Y: st.Y, Z: st.Z, RestTime: st.Rest}
		}
		d.Walkers[t.ID] = route
	}
	d.Spawns = map[int32][]*SpawnGroup{}
	var spawns struct {
		Groups []*SpawnGroup `xml:"spawn"`
	}
	for _, file := range xmlFiles(filepath.Join(dir, "spawns")) {
		spawns.Groups = nil
		if err := loadXML(file, &spawns); err != nil {
			return err
		}
		for _, g := range spawns.Groups {
			g.Pool = min(g.Pool, int32(len(g.Spots)))
			d.Spawns[g.Map] = append(d.Spawns[g.Map], g)
		}
	}
	return nil
}

// xmlFiles lists the xml files under dir, in path order: what an <import> of a folder reads.
func xmlFiles(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err == nil && !e.IsDir() && strings.HasSuffix(path, ".xml") {
			files = append(files, path)
		}
		return nil
	})
	return files
}
