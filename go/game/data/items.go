package data

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ItemTemplate is an item_template of items/item_templates.xml, with the
// attributes the game server uses so far.
type ItemTemplate struct {
	ID            int32
	NameID        int32 // desc, the client's string for the name
	Mask          int32
	Slot          int32 // equipment slot mask (ItemSlot)
	EquipmentType string
	WeaponType    string
	ArmorType     string
	Modifiers     Modifiers
	MaxStack      int32 // 0 for no limit, 1 for items that don't stack
	Quality       string
	Level         int32
	Price         int32
	ItemType      string
	Race          string
	UseDelay      int32 // milliseconds
	UseDelayID    int32
	Dye           bool      // whether it can be dyed
	Restrict      [12]int32 // the level each class may use it from, or 0 if it can't
	Actions       []*Node   // what using the item does: skilluse, skilllearn, dye, …
	Stigma        *Stigma   // what a stigma stone teaches, or nil
}

// IsEquipment reports whether the item is a weapon or armor.
func (t *ItemTemplate) IsEquipment() bool {
	return t.IsWeapon() || t.IsArmor()
}

func (t *ItemTemplate) IsWeapon() bool { return t.EquipmentType == "WEAPON" }

func (t *ItemTemplate) IsArmor() bool { return t.EquipmentType == "ARMOR" }

// IsStigma is AL-Game's test for a stigma stone, by id.
func (t *ItemTemplate) IsStigma() bool { return t.ID > 140000000 && t.ID < 140001000 }

// Kinah is the money item.
const Kinah = 182400001

// loadItems streams the item templates: the file is 23 MB.
func loadItems(path string) (map[int32]*ItemTemplate, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	items := map[int32]*ItemTemplate{}
	decoder := xml.NewDecoder(f)
	var current *ItemTemplate
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return items, nil
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "item_template":
			current = &ItemTemplate{}
			for _, a := range start.Attr {
				switch a.Name.Local {
				case "id":
					current.ID = int32(atoi(a.Value))
				case "desc":
					current.NameID = int32(atoi(a.Value))
				case "mask":
					current.Mask = int32(atoi(a.Value))
				case "slot":
					current.Slot = int32(atoi(a.Value))
				case "equipment_type":
					current.EquipmentType = a.Value
				case "weapon_type":
					current.WeaponType = a.Value
				case "armor_type":
					current.ArmorType = a.Value
				case "max_stack_count":
					current.MaxStack = int32(atoi(a.Value))
				case "quality":
					current.Quality = a.Value
				case "level":
					current.Level = int32(atoi(a.Value))
				case "price":
					current.Price = int32(atoi(a.Value))
				case "item_type":
					current.ItemType = a.Value
				case "race":
					current.Race = a.Value
				case "usedelay":
					current.UseDelay = int32(atoi(a.Value))
				case "restrict":
					for i, v := range strings.Split(a.Value, ",") {
						if i < len(current.Restrict) {
							current.Restrict[i] = int32(atoi(v))
						}
					}
				case "usedelayid":
					current.UseDelayID = int32(atoi(a.Value))
				case "dye":
					current.Dye = a.Value == "true"
				}
			}
			items[current.ID] = current
		case "stigma":
			if current == nil {
				continue
			}
			var st Stigma
			if err := decoder.DecodeElement(&st, &start); err != nil {
				return nil, fmt.Errorf("%s: item %d: %w", filepath.Base(path), current.ID, err)
			}
			current.Stigma = &st
		case "actions":
			if current == nil {
				continue
			}
			node, err := readNode(decoder, start)
			if err != nil {
				return nil, fmt.Errorf("%s: item %d: %w", filepath.Base(path), current.ID, err)
			}
			current.Actions = node.Children
			for _, action := range current.Actions {
				if action.Name != "skilllearn" {
					continue
				}
				// Skill books use the same legacy SkillClass enum as the skill tree.
				action.Attr["class"] = skillTreeClass(action.Str("class"))
				switch action.Str("race") {
				case "PC_LIGHT":
					action.Attr["race"] = "ELYOS"
				case "PC_DARK":
					action.Attr["race"] = "ASMODIANS"
				}
			}
		case "modifiers":
			if current == nil {
				continue
			}
			if err := decoder.DecodeElement(&current.Modifiers, &start); err != nil {
				return nil, fmt.Errorf("%s: item %d: %w", filepath.Base(path), current.ID, err)
			}
		}
	}
}

// Equipment slot masks (ItemSlot).
const (
	SlotMainHand = 1
	SlotSubHand  = 1 << 1
	SlotPants    = 1 << 12
	SlotMainOff  = 1 << 17
	SlotSubOff   = 1 << 18
	SlotNone     = 1 << 25
)

// FirstSlot is the lowest single slot in mask, where a new character's starting equipment goes.
func FirstSlot(mask int32) int32 {
	return mask & -mask
}

// SoulBound is whether the item binds to the character that equips it.
func (t *ItemTemplate) SoulBound() bool { return t.Mask&(1<<6) != 0 }

// AllowedFor is ItemTemplate.isAllowedFor: whether a class of that number (PlayerClass' ordinal) may use it at the level.
func (t *ItemTemplate) AllowedFor(class int, level int) bool {
	return t.Restrict[class] != 0 && int(t.Restrict[class]) <= level
}

// Stigma is what an item that is a stigma stone teaches: a skill, for shards, if the skills it builds on are known.
type Stigma struct {
	SkillID  int32 `xml:"skillid,attr"`
	SkillLvl int32 `xml:"skilllvl,attr"`
	Shard    int32 `xml:"shard,attr"`
	Require  []struct {
		SkillIDs []int32 `xml:"skillId"`
	} `xml:"require_skill"`
}

// ClassSpecific is ItemTemplate.isClassSpecific: the item is for the class (by its number), or for the class it grew from.
func (t *ItemTemplate) ClassSpecific(class int) bool {
	if t.Restrict[class] > 0 {
		return true
	}
	starts := map[int]int{1: 0, 2: 0, 4: 3, 5: 3, 7: 6, 8: 6, 10: 9, 11: 9}
	if start, ok := starts[class]; ok {
		return t.Restrict[start] > 0
	}
	return false
}
