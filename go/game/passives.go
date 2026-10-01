package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// armorTypes is AL-Game's ArmorType, in its order.
var armorTypes = []string{"CHAIN", "CLOTHES", "LEATHER", "PLATE", "ROBE", "SHARD", "SHIELD", "ARROW"}

// passives applies a player's passive skills' stat effects, as its
// EffectController does: weapon and armor masteries as their equipment is worn,
// then every passive skill.
type passives struct {
	s      *Server
	p      *player
	g      *gameStats
	skills []store.Skill // in the order the player's skill list iterates
	levels map[int32]int32
	// The best mastery skill for each weapon and armor type: SkillList's
	// weaponMasterySkills and armorMasterySkills.
	weapon, armor map[string]int32
	// The mastery skills in effect: EffectController holds one of each kind.
	weaponSet, armorSet int32
	applied             map[int32]bool
}

func (s *Server) newPassives(p *player, g *gameStats) *passives {
	ps := &passives{s: s, p: p, g: g, levels: map[int32]int32{}, weapon: map[string]int32{}, armor: map[string]int32{},
		applied: map[int32]bool{}}
	ps.skills = javaHashOrder(p.skills, func(k store.Skill) int32 { return k.ID })
	weaponLevels, armorLevels := map[string]int32{}, map[string]int32{}
	for _, k := range ps.skills {
		ps.levels[k.ID] = k.Level
		t := s.data.Skills[k.ID]
		if t == nil || !t.Passive() || len(t.Effects) == 0 {
			continue
		}
		switch e := t.Effects[0]; e.Kind {
		case "wpnmastery":
			if level, ok := weaponLevels[e.Weapon]; !ok || level < e.BasicLevel {
				weaponLevels[e.Weapon], ps.weapon[e.Weapon] = e.BasicLevel, t.ID
			}
		case "armormastery":
			if level, ok := armorLevels[e.Armor]; !ok || level < e.BasicLevel {
				armorLevels[e.Armor], ps.armor[e.Armor] = e.BasicLevel, t.ID
			}
		}
	}
	return ps
}

// weaponMastery is ItemEquipmentListener.recalculateWeaponMastery on loading: the main hand's mastery.
func (ps *passives) weaponMastery() {
	if main := ps.itemIn(data.SlotMainHand); main != nil {
		ps.useMastery(ps.weapon[main.WeaponType])
	}
}

// useMastery uses a mastery skill, if the player has it.
func (ps *passives) useMastery(skillID int32) {
	if skillID != 0 {
		ps.use(skillID, true)
	}
}

// useAll is PlayerController.updatePassiveStats: every passive skill, used.
func (ps *passives) useAll() {
	for _, k := range ps.skills {
		if t := ps.s.data.Skills[k.ID]; t != nil && t.Passive() {
			ps.use(k.ID, false)
		}
	}
}

// use applies the stat effects of a passive skill whose conditions hold.
func (ps *passives) use(skillID int32, mastery bool) {
	t := ps.s.data.Skills[skillID]
	if t == nil {
		return
	}
	for _, e := range t.Effects {
		switch e.Kind {
		case "wpnmastery":
			// WeaponMasteryEffect.calculate: the best for its weapon, not yet set, and the weapon worn.
			if ps.weapon[e.Weapon] != skillID || ps.weaponSet == skillID || !ps.weaponWorn(e.Weapon) {
				continue
			}
			ps.weaponSet = skillID
		case "armormastery":
			if ps.armor[e.Armor] != skillID || ps.armorSet == skillID || !ps.armorWorn(e.Armor) {
				continue
			}
			ps.armorSet = skillID
		case "weaponstatup", "weaponstatboost":
			if mastery || !ps.weaponWorn(e.Weapon) {
				continue
			}
		case "statup", "statdown", "statboost", "boosthate", "boostheal", "boostskillcastingtime", "changemp":
			if mastery {
				continue
			}
		default:
			continue
		}
		if ps.applied[skillID] && (e.Kind == "wpnmastery" || e.Kind == "armormastery") {
			continue
		}
		ps.applied[skillID] = true
		ps.g.add(statEffect{modifiers: e.Modifiers(ps.levels[skillID])})
	}
}

func (ps *passives) itemIn(slot int32) *data.ItemTemplate {
	for _, item := range ps.p.equipment {
		if item.Slot == slot {
			return ps.s.data.Items[item.ItemID]
		}
	}
	return nil
}

// weaponWorn is Equipment.isWeaponEquipped: in either hand.
func (ps *passives) weaponWorn(weapon string) bool {
	for _, slot := range []int32{data.SlotMainHand, data.SlotSubHand} {
		if t := ps.itemIn(slot); t != nil && t.WeaponType == weapon {
			return true
		}
	}
	return false
}

// armorWorn is Equipment.isArmorEquipped: every piece of body armor worn is of the type,
// which holds as well when none is.
func (ps *passives) armorWorn(armor string) bool {
	const boots, gloves, helmet, pants, shoulder, torso = 1 << 5, 1 << 4, 1 << 2, 1 << 12, 1 << 11, 1 << 3
	for _, slot := range []int32{boots, gloves, helmet, pants, shoulder, torso} {
		if t := ps.itemIn(slot); t != nil && t.ArmorType != armor {
			return false
		}
	}
	return true
}
