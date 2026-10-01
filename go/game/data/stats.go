package data

import (
	"encoding/xml"
	"fmt"
	"math"
	"path/filepath"
)

// ModifierKind is how a modifier changes its stat: AL-Game's StatModifier subclasses.
type ModifierKind uint8

const (
	ModSet ModifierKind = iota
	ModAdd
	ModRate
	ModMean
	ModSub
)

var modifierKinds = map[string]ModifierKind{"set": ModSet, "add": ModAdd, "rate": ModRate, "mean": ModMean, "sub": ModSub}

// Modifier is a <modifiers> entry of an item, title or item set: a change to one stat.
type Modifier struct {
	Kind     ModifierKind
	Stat     Stat
	Value    int32 // set, add, rate, sub
	Min, Max int32 // mean
	Bonus    bool
}

// Priority is when the modifier applies among a stat's modifiers: 0 first.
func (m Modifier) Priority() int {
	switch m.Kind {
	case ModSet, ModMean:
		return 0
	case ModAdd:
		return 1
	}
	return 2
}

// Apply is the modifier's new base, or its bonus increment, given the stat's base and current values.
func (m Modifier) Apply(base, current int32) int32 {
	switch m.Kind {
	case ModSet:
		return m.Value
	case ModMean:
		return base + Round(float32(m.Min+m.Max)/2)
	case ModSub:
		if m.Bonus {
			return -m.Value
		}
		return base - m.Value
	}
	var value int32
	switch {
	case m.Kind == ModAdd && m.Bonus:
		value = m.Value
	case m.Kind == ModAdd:
		value = base + m.Value
	case m.Bonus: // rate
		value = Round(float32(m.Value*base) / 100)
	default:
		value = Round(float32(base) * float32(1+float32(m.Value)/100))
	}
	var low, high int32
	switch m.Stat {
	case AttackSpeed, Speed:
		low, high = 600, 12000
	case FlySpeed:
		low, high = 600, 16000
	}
	if low == 0 && high == 0 {
		if m.Bonus && value+current < 0 {
			return -current
		}
		if !m.Bonus && value < 0 {
			return 0
		}
		return value
	}
	// ponytail: AL-Game clamps base and bonus alike against current, a
	// slip for base values that the port keeps so the numbers match.
	if value+current < low {
		return -(current - low)
	}
	if value+current > high {
		return high - current
	}
	return value
}

// Round is Java's Math.round(float).
func Round(x float32) int32 {
	return int32(math.Floor(float64(x) + 0.5))
}

// Modifiers is a <modifiers> element, in document order.
type Modifiers []Modifier

func (ms *Modifiers) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			m, err := parseModifier(t)
			if err != nil {
				return err
			}
			*ms = append(*ms, m)
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func parseModifier(t xml.StartElement) (Modifier, error) {
	kind, ok := modifierKinds[t.Name.Local]
	if !ok {
		return Modifier{}, fmt.Errorf("unknown modifier <%s>", t.Name.Local)
	}
	m := Modifier{Kind: kind}
	for _, a := range t.Attr {
		switch a.Name.Local {
		case "name":
			stat, ok := StatByName(a.Value)
			if !ok {
				return Modifier{}, fmt.Errorf("unknown stat %q", a.Value)
			}
			m.Stat = stat
		case "value":
			m.Value = int32(atoi(a.Value))
		case "min":
			m.Min = int32(atoi(a.Value))
		case "max":
			m.Max = int32(atoi(a.Value))
		case "bonus":
			m.Bonus = a.Value == "true"
		}
	}
	return m, nil
}

// StatByName is the stat with a StatEnum name.
func StatByName(name string) (Stat, bool) {
	for i, n := range statNames {
		if n == name {
			return Stat(i), true
		}
	}
	return 0, false
}

func (s Stat) String() string {
	if int(s) < len(statNames) {
		return statNames[s]
	}
	return fmt.Sprintf("Stat(%d)", s)
}

// StoneMask is the id SM_INVENTORY_INFO gives a manastone's stat by.
func (s Stat) StoneMask() byte {
	return stoneMasks[s]
}

// PlayerStats is a class's stats at a level: a stats_template of stats/player.
type PlayerStats struct {
	MaxHP            int32   `xml:"maxHp,attr"`
	MaxMP            int32   `xml:"maxMp,attr"`
	Power            int32   `xml:"power,attr"`
	Health           int32   `xml:"health,attr"`
	Agility          int32   `xml:"agility,attr"`
	Accuracy         int32   `xml:"accuracy,attr"`
	Knowledge        int32   `xml:"knowledge,attr"`
	Will             int32   `xml:"will,attr"`
	RunSpeed         float32 `xml:"run_speed,attr"`
	FlySpeed         float32 `xml:"fly_speed,attr"`
	AttackSpeed      float32 `xml:"attack_speed,attr"`
	Evasion          int32   `xml:"evasion,attr"`
	Block            int32   `xml:"block,attr"`
	Parry            int32   `xml:"parry,attr"`
	MainHandAttack   int32   `xml:"main_hand_attack,attr"`
	MainHandAccuracy int32   `xml:"main_hand_accuracy,attr"`
	MainHandCritRate int32   `xml:"main_hand_crit_rate,attr"`
	MagicAccuracy    int32   `xml:"magic_accuracy,attr"`
}

type classLevel struct {
	class string
	level int
}

func loadPlayerStats(dir string) (map[classLevel]*PlayerStats, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.xml"))
	if err != nil {
		return nil, err
	}
	templates := map[classLevel]*PlayerStats{}
	for _, path := range files {
		var doc struct {
			Stats []struct {
				Class    string      `xml:"class,attr"`
				Level    int         `xml:"level,attr"`
				Template PlayerStats `xml:"stats_template"`
			} `xml:"player_stats"`
		}
		if err := loadXML(path, &doc); err != nil {
			return nil, err
		}
		for _, s := range doc.Stats {
			templates[classLevel{s.Class, s.Level}] = &s.Template
		}
	}
	return templates, nil
}

// calculatedStats is AL-Game's ClassStats, for a class at a level with no
// stats_template: power, health, agility, accuracy, knowledge, will, main hand
// attack, crit rate and accuracy, magic accuracy, evasion, block, parry, attack
// speed (ms), fly and run speed, and the max HP curve a·(l-1)² + b·(l-1) + c.
var calculatedStats = map[string]struct {
	power, health, agility, accuracy, knowledge, will            int32
	mainHandAttack, mainHandCritRate, mainHandAccuracy, magicAcc int32
	evasion, block, parry, attackSpeed, flySpeed, runSpeed       int32
	hpA, hpB, hpC                                                float32
}{
	"WARRIOR":       {110, 110, 100, 100, 90, 90, 19, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 1.1688, 1.1688, 284},
	"GLADIATOR":     {115, 115, 100, 100, 90, 90, 19, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 1.3393, 48.246, 342},
	"TEMPLAR":       {115, 100, 110, 100, 90, 105, 19, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 1.3288, 51.878, 281},
	"SCOUT":         {100, 100, 100, 110, 90, 90, 18, 3, 0, 0, 0, 0, 0, 1500, 9, 6, 1.0297, 40.823, 219},
	"ASSASSIN":      {110, 100, 100, 110, 90, 90, 19, 3, 0, 0, 0, 0, 0, 1500, 9, 6, 1.0488, 40.38, 222},
	"RANGER":        {90, 90, 100, 100, 120, 110, 18, 3, 0, 0, 0, 0, 0, 1500, 9, 6, 0.5, 38.5, 133},
	"MAGE":          {90, 90, 95, 95, 115, 115, 16, 1, 0, 0, 0, 0, 0, 1500, 9, 6, 0.7554, 29.457, 132},
	"SORCERER":      {90, 90, 100, 100, 120, 110, 16, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 0.6352, 24.852, 112},
	"SPIRIT_MASTER": {90, 90, 100, 100, 115, 115, 16, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 1, 20.6, 157},
	"PRIEST":        {95, 95, 100, 100, 100, 110, 17, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 1.0303, 40.824, 201},
	"CLERIC":        {105, 110, 90, 100, 105, 110, 19, 2, 0, 0, 0, 0, 0, 1500, 9, 6, 0.9277, 35.988, 229},
	"CHANTER":       {110, 105, 90, 90, 105, 110, 19, 1, 0, 0, 0, 0, 0, 1500, 9, 6, 0.9277, 35.988, 229},
}

// PlayerStatsFor is a class's stats at a level: its stats_template, or else
// AL-Game's CalculatedPlayerStatsTemplate, which uses level 10's max HP and 1000 MP.
func (d *Data) PlayerStatsFor(class string, level int) *PlayerStats {
	if t := d.playerStats[classLevel{class, level}]; t != nil {
		return t
	}
	c := calculatedStats[class]
	const hpLevel = 10 - 1
	return &PlayerStats{
		MaxHP:            Round(float32(float32(c.hpA*hpLevel)*hpLevel) + float32(c.hpB*hpLevel) + c.hpC),
		MaxMP:            1000,
		Power:            c.power,
		Health:           c.health,
		Agility:          c.agility,
		Accuracy:         c.accuracy,
		Knowledge:        c.knowledge,
		Will:             c.will,
		RunSpeed:         float32(c.runSpeed),
		FlySpeed:         float32(c.flySpeed),
		AttackSpeed:      float32(c.attackSpeed) / 1000,
		Evasion:          c.evasion,
		Block:            c.block,
		Parry:            c.parry,
		MainHandAttack:   c.mainHandAttack,
		MainHandAccuracy: c.mainHandAccuracy,
		MainHandCritRate: c.mainHandCritRate,
		MagicAccuracy:    c.magicAcc,
	}
}

// Title is a player_titles.xml title: the stats it gives while worn.
type Title struct {
	ID        int32     `xml:"id,attr"`
	Race      int32     `xml:"race,attr"`
	Modifiers Modifiers `xml:"modifiers"`
}

// ItemSet is an item_sets.xml set: its parts and the bonuses for wearing some or all of them.
type ItemSet struct {
	ID    int32 `xml:"id,attr"`
	Parts []struct {
		ItemID int32 `xml:"itemid,attr"`
	} `xml:"itempart"`
	PartBonuses []SetBonus `xml:"partbonus"`
	FullBonus   *SetBonus  `xml:"fullbonus"`
}

// SetBonus is the stats a set gives for Count parts worn.
type SetBonus struct {
	Count     int       `xml:"count,attr"`
	Modifiers Modifiers `xml:"modifiers"`
}

func loadTitles(path string) (map[int32]*Title, error) {
	var doc struct {
		Titles []*Title `xml:"title"`
	}
	if err := loadXML(path, &doc); err != nil {
		return nil, err
	}
	titles := map[int32]*Title{}
	for _, t := range doc.Titles {
		titles[t.ID] = t
	}
	return titles, nil
}

// loadItemSets returns each set by the items that are part of it.
func loadItemSets(path string) (map[int32]*ItemSet, error) {
	var doc struct {
		Sets []*ItemSet `xml:"itemset"`
	}
	if err := loadXML(path, &doc); err != nil {
		return nil, err
	}
	byItem := map[int32]*ItemSet{}
	for _, set := range doc.Sets {
		for _, part := range set.Parts {
			byItem[part.ItemID] = set
		}
	}
	return byItem, nil
}
