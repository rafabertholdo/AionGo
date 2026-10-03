// Package data is the game server's static data: AL-Game/data/static_data,
// read from the same XML files the Java server uses.
package data

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Data is the static data the game server has loaded.
type Data struct {
	Items            map[int32]*ItemTemplate
	Experience       []int64 // total experience at the start of each level, from level 0
	Initial          Initial
	SkillTree        []SkillLearn
	Titles           map[int32]*Title
	ItemSets         map[int32]*ItemSet  // by part item id
	BindPoints       map[int32]Location  // by bind id
	BindStones       map[int32]BindStone // by the npc of the bind stone
	Welcome          string              // HTML/welcome.xhtml, or ""
	WorldMaps        map[int32]*WorldMap
	Zones            map[int32][]*Zone // by map id, by priority
	Skills           map[int32]*SkillTemplate
	NpcSkills        map[int32][]NpcSkill // by npc template id
	Recipes          map[int32]*Recipe
	autolearn        map[recipeKey][]*Recipe // the recipes that come with a skill level
	TradeLists       map[int32]*TradeList    // by npc template id
	GoodsLists       map[int32][]int32       // the item ids of each goods list
	Portals          map[int32]*Portal       // by npc template id
	summonStats      map[summonKey]*SummonStats
	PetSkills        map[[2]int32]int32        // the skill a summon uses for an order skill, by order skill and summon npc
	PortalList       []*Portal                 // in the file's order
	Teleporters      map[int32]*Teleporter     // by npc template id
	TeleLocations    map[int32]*TeleLocation   // by location id
	CubeExpand       map[int32]map[int32]int32 // the price an npc asks to expand the cube to a level
	WarehouseExpand  map[int32]map[int32]int32
	Quests           map[int32]*QuestTemplate
	QuestScripts     map[int32]*QuestScript
	QuestStarts      map[int32][]*QuestScript // supported scripts offered by npc template id
	QuestEnds        map[int32][]*QuestScript
	QuestActions     map[int32][]*QuestScript
	QuestKills       map[int32][]*QuestScript
	QuestDropsByNPC  map[int32][]*QuestScript
	QuestXMLTalks    map[int32][]*QuestScript // specialized XML talk events by NPC
	QuestCustomTalks map[int32][]*QuestScript // custom Java handlers with additional talk objects
	QuestItemUses    map[int32][]*QuestScript // custom Java handlers started through item use
	Sieges           []SiegeLocation          // in siege_locations.xml's order

	Npcs        map[int32]*NpcTemplate
	Gatherables map[int32]*GatherableTemplate
	Tribes      Tribes
	Walkers     map[int32]Route
	Spawns      map[int32][]*SpawnGroup // by map id, in file order
	SpawnsByNPC map[int32][]*SpawnGroup // by template id, in file order across maps

	playerStats map[classLevel]*PlayerStats
}

// Load reads the static data under dir (AL-Game's data/static_data).
func Load(dir string) (*Data, error) {
	d := &Data{}
	var err error
	if d.Items, err = loadItems(filepath.Join(dir, "items/item_templates.xml")); err != nil {
		return nil, err
	}
	if d.Experience, err = loadExperience(filepath.Join(dir, "player_experience_table.xml")); err != nil {
		return nil, err
	}
	if err = loadXML(filepath.Join(dir, "player_initial_data.xml"), &d.Initial); err != nil {
		return nil, err
	}
	var tree struct {
		Skills []SkillLearn `xml:"skill"`
	}
	if err = loadXML(filepath.Join(dir, "skill_tree/skill_tree.xml"), &tree); err != nil {
		return nil, err
	}
	// The gathering skills every character starts with (or learns at level 10) are in the second file of the folder.
	var crafts struct {
		Skills []SkillLearn `xml:"skill"`
	}
	if err = loadXML(filepath.Join(dir, "skill_tree/craft_skill_tree.xml"), &crafts); err != nil {
		return nil, err
	}
	d.SkillTree = append(crafts.Skills, tree.Skills...)
	for i := range d.SkillTree {
		d.SkillTree[i].Class = skillTreeClass(d.SkillTree[i].Class)
	}
	if d.playerStats, err = loadPlayerStats(filepath.Join(dir, "stats/player")); err != nil {
		return nil, err
	}
	if d.Titles, err = loadTitles(filepath.Join(dir, "player_titles.xml")); err != nil {
		return nil, err
	}
	if d.ItemSets, err = loadItemSets(filepath.Join(dir, "item_sets/item_sets.xml")); err != nil {
		return nil, err
	}
	var binds struct {
		Points []struct {
			ID    int32   `xml:"bindid,attr"`
			NpcID int32   `xml:"npcid,attr"`
			Price int32   `xml:"price,attr"`
			MapID int32   `xml:"mapid,attr"`
			X     float32 `xml:"posX,attr"`
			Y     float32 `xml:"posY,attr"`
			Z     float32 `xml:"posZ,attr"`
		} `xml:"bind_point"`
	}
	if err = loadXML(filepath.Join(dir, "bind_points/bind_points.xml"), &binds); err != nil {
		return nil, err
	}
	d.BindPoints = map[int32]Location{}
	d.BindStones = map[int32]BindStone{}
	for _, b := range binds.Points {
		d.BindPoints[b.ID] = Location{MapID: b.MapID, X: b.X, Y: b.Y, Z: b.Z}
		d.BindStones[b.NpcID] = BindStone{ID: b.ID, Price: b.Price}
	}
	if welcome, err := os.ReadFile(filepath.Join(dir, "HTML/welcome.xhtml")); err == nil {
		d.Welcome = string(welcome)
	}
	var maps struct {
		Maps []*WorldMap `xml:"map"`
	}
	if err = loadXML(filepath.Join(dir, "world_maps.xml"), &maps); err != nil {
		return nil, err
	}
	if d.Zones, err = loadZones(dir); err != nil {
		return nil, err
	}
	if d.Skills, err = loadSkills(filepath.Join(dir, "skills")); err != nil {
		return nil, err
	}
	if d.NpcSkills, err = loadNpcSkills(dir); err != nil {
		return nil, err
	}
	if d.Recipes, err = loadRecipes(dir); err != nil {
		return nil, err
	}
	d.indexRecipes()
	if d.TradeLists, d.GoodsLists, err = loadShops(dir); err != nil {
		return nil, err
	}
	if d.summonStats, err = loadSummonStats(dir); err != nil {
		return nil, err
	}
	var pets struct {
		Skills []struct {
			Skill int32 `xml:"skill_id,attr"`
			Pet   int32 `xml:"pet_id,attr"`
			Order int32 `xml:"order_skill,attr"`
		} `xml:"pet_skill"`
	}
	if err = loadXML(filepath.Join(dir, "pet_skills/pet_skills.xml"), &pets); err != nil {
		return nil, err
	}
	d.PetSkills = map[[2]int32]int32{}
	for _, k := range pets.Skills {
		d.PetSkills[[2]int32{k.Order, k.Pet}] = k.Skill
	}
	if d.Portals, d.PortalList, err = loadPortals(dir); err != nil {
		return nil, err
	}
	if d.Teleporters, d.TeleLocations, err = loadTeleporters(dir); err != nil {
		return nil, err
	}
	if d.CubeExpand, err = loadExpanders(dir, "cube_expander/cube_expander.xml", "cube"); err != nil {
		return nil, err
	}
	if d.WarehouseExpand, err = loadExpanders(dir, "warehouse_expander/warehouse_expander.xml", "warehouse"); err != nil {
		return nil, err
	}
	if d.Quests, err = loadQuests(filepath.Join(dir, "quest_data/quest_data.xml")); err != nil {
		return nil, err
	}
	if err = d.loadQuestScripts(dir); err != nil {
		return nil, err
	}
	var sieges struct {
		Locations []SiegeLocation `xml:"siege_location"`
	}
	if err = loadXML(filepath.Join(dir, "siege/siege_locations.xml"), &sieges); err != nil {
		return nil, err
	}
	// As SiegeLocationData: only fortresses, artifacts and boss raids are locations, the first of an id.
	seen := map[int32]bool{}
	for _, l := range sieges.Locations {
		switch l.Type {
		case "FORTRESS", "ARTIFACT", "BOSSRAID_LIGHT", "BOSSRAID_DARK":
			if !seen[l.ID] {
				seen[l.ID] = true
				d.Sieges = append(d.Sieges, l)
			}
		}
	}
	d.WorldMaps = map[int32]*WorldMap{}
	for _, m := range maps.Maps {
		d.WorldMaps[m.ID] = m
	}
	if err = d.loadWorld(dir); err != nil {
		return nil, err
	}
	return d, nil
}

func loadXML(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := xml.NewDecoder(f).Decode(v); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func atoi(s string) int64 {
	v, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return v
}

// ExpStart is the total experience a level starts at.
func (d *Data) ExpStart(level int) int64 {
	if level < 1 || level > len(d.Experience) {
		return 0
	}
	return d.Experience[level-1]
}

// MaxLevel is the highest level the experience table has.
func (d *Data) MaxLevel() int {
	return len(d.Experience)
}

// Level is the player level for exp total experience.
func (d *Data) Level(exp int64) int {
	level := 1
	for l := 1; l < len(d.Experience); l++ {
		if exp >= d.Experience[l] {
			level = l + 1
		}
	}
	return min(level, len(d.Experience)-1)
}

func loadExperience(path string) ([]int64, error) {
	var table struct {
		Exp []string `xml:"exp"`
	}
	if err := loadXML(path, &table); err != nil {
		return nil, err
	}
	exp := make([]int64, len(table.Exp))
	for i, e := range table.Exp {
		exp[i] = atoi(e)
	}
	return exp, nil
}

// Location is where a player appears in the world.
type Location struct {
	MapID   int32   `xml:"map_id,attr"`
	X       float32 `xml:"x,attr"`
	Y       float32 `xml:"y,attr"`
	Z       float32 `xml:"z,attr"`
	Heading int32   `xml:"heading,attr"`
}

// SiegeLocation is a siege_locations.xml entry.
type SiegeLocation struct {
	ID    int32  `xml:"id,attr"`
	Type  string `xml:"type,attr"`
	World int32  `xml:"world,attr"`
}

// WorldMap is a world_maps.xml map.
type WorldMap struct {
	ID         int32   `xml:"id,attr"`
	Name       string  `xml:"name,attr"`
	TwinCount  int32   `xml:"twin_count,attr"`  // channels
	DeathLevel float32 `xml:"death_level,attr"` // below it a player dies
	WaterLevel float32 `xml:"water_level,attr"` // below it (less the player's height) a player drowns
	Instance   bool    `xml:"instance,attr"`
	WorldType  string  `xml:"world_type,attr"` // ELYSEA, ASMODAE or ABYSS, or "" for the rest
}

// BindStone is a bind_points.xml entry as the npc that offers it: the bind point it sets and what it costs.
type BindStone struct {
	ID    int32
	Price int32
}

// Initial is player_initial_data.xml: where each race starts and each class's starting items.
type Initial struct {
	Asmodians Location     `xml:"asmodian_spawn_location"`
	Elyos     Location     `xml:"elyos_spawn_location"`
	Classes   []ClassStart `xml:"player_data"`
}

// ClassStart is a starting class's items.
type ClassStart struct {
	Class string `xml:"class,attr"`
	Items []struct {
		ID    int32 `xml:"id,attr"`
		Count int64 `xml:"count,attr"`
	} `xml:"items>item"`
}

// SkillLearn is an entry of skill_tree.xml: a skill a class learns at a level.
type SkillLearn struct {
	SkillID    int32  `xml:"skillId,attr"`
	SkillLevel int32  `xml:"skillLevel,attr"`
	MinLevel   int    `xml:"minLevel,attr"`
	Class      string `xml:"classId,attr"`
	Race       string `xml:"race,attr"`
	Autolearn  bool   `xml:"autolearn,attr"`
	Stigma     bool   `xml:"stigma,attr"`
}

// skillTreeClass matches Java's SkillClass and PlayerClass by enum position.
// The XML retains legacy names, including the reversed Priest/Cleric names.
func skillTreeClass(class string) string {
	switch class {
	case "FIGHTER":
		return "GLADIATOR"
	case "KNIGHT":
		return "TEMPLAR"
	case "WIZARD":
		return "SORCERER"
	case "ELEMENTALLIST":
		return "SPIRIT_MASTER"
	case "CLERIC":
		return "PRIEST"
	case "PRIEST":
		return "CLERIC"
	default:
		return class
	}
}

// SkillsAt is what a class of race learns on reaching level: race-specific, class-wide and general entries.
func (d *Data) SkillsAt(class, race string, level int) []SkillLearn {
	var skills []SkillLearn
	for _, s := range d.SkillTree {
		if s.MinLevel != level || (s.Class != class && s.Class != "ALL") {
			continue
		}
		if s.Race == "" || s.Race == "ALL" || s.Race == race {
			skills = append(skills, s)
		}
	}
	return skills
}
