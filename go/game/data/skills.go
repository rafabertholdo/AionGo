package data

import (
	"encoding/xml"
	"path/filepath"
	"strconv"
	"strings"
)

// SkillTemplate is a skill_template of skills/skill_templates*.xml, with what
// the game server uses so far.
type SkillTemplate struct {
	ID           int32  `xml:"skill_id,attr"`
	Name         string `xml:"name,attr"`
	NameID       int32  `xml:"nameId,attr"`
	Stack        string `xml:"stack,attr"`
	Level        int32  `xml:"lvl,attr"`
	Type         string `xml:"skilltype,attr"`    // MAGICAL, PHYSICAL, NONE
	SubType      string `xml:"skillsubtype,attr"` // ATTACK, BUFF, DEBUFF, HEAL, …
	TargetSlot   string `xml:"tslot,attr"`
	Activation   string `xml:"activation,attr"` // PASSIVE, ACTIVE, TOGGLE, PROVOKED, …
	Duration     int32  `xml:"duration,attr"`   // cast time, in milliseconds
	Cooldown     int32  `xml:"cooldown,attr"`   // in tenths of a second
	PenaltySkill int32  `xml:"penalty_skill_id,attr"`
	PvpDamage    int32  `xml:"pvp_damage,attr"`
	PvpDuration  int32  `xml:"pvp_duration,attr"`
	ChainProb    int32  `xml:"chain_skill_prob,attr"`
	CancelRate   int32  `xml:"cancel_rate,attr"`
	SetException int32  `xml:"skillset_exception,attr"`

	// What the skill checks and does, as the XML says: each element's name, attributes and children.
	InitProperties  Nodes        `xml:"initproperties"`
	StartConditions Nodes        `xml:"startconditions"`
	SetProperties   Nodes        `xml:"setproperties"`
	UseConditions   Nodes        `xml:"useconditions"`
	Actions         Nodes        `xml:"actions"`
	Effects         SkillEffects `xml:"effects"`
}

func (t *SkillTemplate) IsActive() bool { return t.Activation == "ACTIVE" }
func (t *SkillTemplate) IsToggle() bool { return t.Activation == "TOGGLE" }

// EffectsDuration is the longest duration of the skill's effects.
func (t *SkillTemplate) EffectsDuration() int32 {
	var d int32
	for _, e := range t.Effects {
		d = max(d, e.Node.Int("duration"))
	}
	return d
}

// Passive reports whether the skill works on its own, like a weapon mastery.
func (t *SkillTemplate) Passive() bool { return t.Activation == "PASSIVE" }

// SkillEffect is one of a skill's <effects>: its kind is the element's name.
type SkillEffect struct {
	Kind       string // statup, wpnmastery, armormastery, …
	Weapon     string // for wpnmastery, weaponstatup
	Armor      string // for armormastery
	BasicLevel int32
	Changes    []Change
	Node       *Node // all of the element
}

// Change is a skill effect's <change>: a stat changed by value + delta × skill level.
type Change struct {
	Stat  Stat
	Func  string // ADD, PERCENT, REPLACE
	Value int32
	Delta int32
}

// SkillEffects is an <effects> element, in document order.
type SkillEffects []SkillEffect

func (es *SkillEffects) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			node, err := readNode(d, t)
			if err != nil {
				return err
			}
			e := SkillEffect{Kind: t.Name.Local, Node: node, Weapon: node.Str("weapon"), Armor: node.Str("armor"),
				BasicLevel: node.Int("basiclvl")}
			e.readChanges(node)
			*es = append(*es, e)
		case xml.EndElement:
			return nil
		}
	}
}

// readChanges reads the effect's <change> children.
func (e *SkillEffect) readChanges(node *Node) {
	for _, child := range node.Children {
		if child.Name != "change" {
			continue
		}
		c := Change{Func: child.Str("func"), Value: child.Int("value"), Delta: child.Int("delta")}
		known := true
		if name := child.Str("stat"); name != "" {
			c.Stat, known = StatByName(name)
		}
		// AL-Game skips a change to a stat it doesn't know.
		if known {
			e.Changes = append(e.Changes, c)
		}
	}
}

// Modifiers is BufEffect.getModifiers: the effect's changes for a skill level,
// all bonuses.
func (e *SkillEffect) Modifiers(skillLevel int32) Modifiers {
	var ms Modifiers
	for _, c := range e.Changes {
		m := Modifier{Stat: c.Stat, Value: c.Value + c.Delta*skillLevel, Bonus: true}
		switch c.Func {
		case "ADD":
			m.Kind = ModAdd
		case "PERCENT":
			m.Kind = ModRate
		case "REPLACE":
			m.Kind = ModSet
		default:
			continue
		}
		ms = append(ms, m)
	}
	return ms
}

func loadSkills(dir string) (map[int32]*SkillTemplate, error) {
	skills := map[int32]*SkillTemplate{}
	for _, name := range []string{"skill_templates.xml", "skill_templates2.xml"} {
		var doc struct {
			Skills []*SkillTemplate `xml:"skill_template"`
		}
		if err := loadXML(filepath.Join(dir, name), &doc); err != nil {
			return nil, err
		}
		for _, s := range doc.Skills {
			skills[s.ID] = s
		}
	}
	return skills, nil
}

// Node is an element of a skill template, as generic as the XML: the skill engine reads what it needs by name.
type Node struct {
	Name     string
	Attr     map[string]string
	Children []*Node
}

// Str is an attribute, or "".
func (n *Node) Str(name string) string {
	if n == nil {
		return ""
	}
	return n.Attr[name]
}

// Int is an attribute as a number, or 0.
func (n *Node) Int(name string) int32 { return int32(atoi(n.Str(name))) }

// Float is an attribute as a number, or 0.
func (n *Node) Float(name string) float32 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(n.Str(name)), 32)
	return float32(v)
}

// Bool is an attribute that is "true".
func (n *Node) Bool(name string) bool { return n.Str(name) == "true" }

// Has is whether the attribute is there.
func (n *Node) Has(name string) bool {
	if n == nil {
		return false
	}
	_, ok := n.Attr[name]
	return ok
}

// Child is the first child with the name, or nil.
func (n *Node) Child(name string) *Node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// readNode reads the element that start began, through its end.
func readNode(d *xml.Decoder, start xml.StartElement) (*Node, error) {
	n := &Node{Name: start.Name.Local, Attr: make(map[string]string, len(start.Attr))}
	for _, a := range start.Attr {
		n.Attr[a.Name.Local] = a.Value
	}
	for {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			child, err := readNode(d, t)
			if err != nil {
				return nil, err
			}
			n.Children = append(n.Children, child)
		case xml.EndElement:
			return n, nil
		}
	}
}

// Nodes is the elements of a section of a skill template, in document order.
type Nodes []*Node

func (ns *Nodes) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		token, err := d.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			node, err := readNode(d, t)
			if err != nil {
				return err
			}
			*ns = append(*ns, node)
		case xml.EndElement:
			return nil
		}
	}
}
