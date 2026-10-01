package game

import (
	"slices"

	"aionlightning/game/data"
)

// stat is one of a creature's stats, AL-Game's Stat: base, the value its
// modifiers replace, reset to origin on every recompute, and a bonus on top.
type stat struct {
	origin, base, bonus int32
}

// statEffect is a set of modifiers in effect: an item's, a set bonus, a title.
type statEffect struct {
	key       string // set for a skill's effect, so it can be taken off
	item      bool   // an item's own modifiers, which apply to the slot it is worn in
	slot      int32  // that slot
	modifiers data.Modifiers
}

// gameStats is AL-Game's CreatureGameStats: a creature's stats, and the
// effects that modify them, applied in the order they were added.
type gameStats struct {
	stats   [data.NumStats]*stat
	effects []statEffect
}

// flyTime is AL-Game's gameserver.base.flytime, in seconds.
const flyTime = 60

// newPlayerStats is PlayerGameStats' initial stats for a class template at a level.
func newPlayerStats(t *data.PlayerStats, level int) *gameStats {
	g := &gameStats{}
	l := float32(level)
	agilityBased := data.Round(float32(float32(float32(t.Agility)*3.1)-248.5) + float32(12.4*l))
	accuracyBased := (t.Accuracy*2 - 10) + 8*int32(level)
	for _, s := range []struct {
		stat  data.Stat
		value int32
	}{
		{data.MaxHP, t.MaxHP}, {data.MaxMP, t.MaxMP}, {data.Power, t.Power}, {data.Accuracy, t.Accuracy},
		{data.Health, t.Health}, {data.Agility, t.Agility}, {data.Knowledge, t.Knowledge}, {data.Will, t.Will},
		{data.MainHandPower, data.Round(float32(18 * float32(float32(t.Power)*0.01)))},
		{data.MainHandCritical, t.MainHandCritRate}, {data.OffHandPower, 0}, {data.OffHandCritical, 0},
		{data.AttackSpeed, data.Round(t.AttackSpeed * 1000)}, {data.MainHandAttackSpeed, data.Round(t.AttackSpeed * 1000)},
		{data.OffHandAttackSpeed, 0}, {data.AttackRange, 1500}, {data.PhysicalDefense, 0},
		{data.Parry, agilityBased}, {data.Evasion, agilityBased}, {data.Block, agilityBased}, {data.DamageReduce, 0},
		{data.MainHandAccuracy, accuracyBased}, {data.OffHandAccuracy, accuracyBased}, {data.MagicalResist, 0},
		{data.WindResistance, 0}, {data.FireResistance, 0}, {data.WaterResistance, 0}, {data.EarthResistance, 0},
		{data.MagicalAccuracy, data.Round(float32(14.26 * l))}, {data.BoostMagicalSkill, 0},
		{data.Speed, data.Round(t.RunSpeed * 1000)}, {data.FlySpeed, data.Round(t.FlySpeed * 1000)},
		{data.PvpAttackRatio, 0}, {data.PvpDefendRatio, 0}, {data.BoostCastingTime, 100}, {data.BoostHate, 100},
		{data.BoostHeal, 100},
		// Then the template's own values for these: as in AL-Game they only
		// replace the base, so the formulas above return on the first recompute.
		{data.Parry, t.Parry}, {data.Block, t.Block}, {data.Evasion, t.Evasion},
		{data.MagicalAccuracy, t.MagicAccuracy}, {data.MainHandAccuracy, t.MainHandAccuracy},
		{data.FlyTime, flyTime}, {data.RegenHp, int32(level) + 3}, {data.RegenMp, int32(level) + 8},
		{data.MaxDP, 4000},
	} {
		g.initStat(s.stat, s.value)
	}
	return g
}

func (g *gameStats) initStat(s data.Stat, value int32) {
	if st := g.stats[s]; st != nil {
		st.base, st.bonus = value, 0
		return
	}
	g.stats[s] = &stat{origin: value, base: value}
}

func (g *gameStats) set(s data.Stat, value int32, bonus bool) {
	st := g.stats[s]
	if st == nil {
		st = &stat{}
		g.stats[s] = st
	}
	if bonus {
		st.bonus = value
	} else {
		st.base = value
	}
}

func (g *gameStats) current(s data.Stat) int32 {
	if st := g.stats[s]; st != nil {
		return st.base + st.bonus
	}
	return 0
}

func (g *gameStats) base(s data.Stat) int32 {
	if st := g.stats[s]; st != nil {
		return st.base
	}
	return 0
}

func (g *gameStats) bonus(s data.Stat) int32 {
	if st := g.stats[s]; st != nil {
		return st.bonus
	}
	return 0
}

// addKeyed puts the modifiers of a skill's effect on, in place of any it put on before.
func (g *gameStats) addKeyed(key string, mods data.Modifiers) {
	g.removeKeyed(key)
	g.effects = append(g.effects, statEffect{key: key, modifiers: mods})
}

// removeKeyed takes a skill's effect off.
func (g *gameStats) removeKeyed(key string) {
	g.effects = slices.DeleteFunc(g.effects, func(e statEffect) bool { return e.key != "" && e.key == key })
}

func (g *gameStats) add(e statEffect) {
	if len(e.modifiers) > 0 {
		g.effects = append(g.effects, e)
	}
}

// recompute is CreatureGameStats.recomputeStats: every stat back to its
// origin, then every effect's modifiers, a stat at a time by priority.
// offHandWeapon is whether a weapon is worn in the off hand.
func (g *gameStats) recompute(offHandWeapon bool) {
	for _, st := range g.stats {
		if st != nil {
			st.base, st.bonus = st.origin, 0
		}
	}
	var ordered [data.NumStats][3]data.Modifiers
	for _, e := range g.effects {
		// Java keeps slots across an effect's modifiers, and only an item's resets it.
		var slots int32
		for _, m := range e.modifiers {
			if e.item {
				slots = e.slot
			}
			if slots == 0 {
				slots = data.SlotNone
			}
			if isHandStat(m.Stat) {
				switch {
				case slots != data.SlotMainHand && slots != data.SlotSubHand && offHandWeapon:
					slots = data.SlotMainHand | data.SlotSubHand
				case slots != data.SlotMainHand && slots != data.SlotSubHand:
					slots = data.SlotMainHand
					g.set(data.OffHandAccuracy, 0, false)
				case slots == data.SlotMainHand:
					g.set(data.MainHandPower, 0, false)
				}
			}
			for bit := int32(1); bit > 0 && bit <= slots; bit <<= 1 {
				if slots&bit != 0 {
					s := handStat(m.Stat, bit)
					ordered[s][m.Priority()] = append(ordered[s][m.Priority()], m)
				}
			}
		}
	}
	for s := range ordered {
		byPriority := ordered[s]
		if len(byPriority[0])+len(byPriority[1])+len(byPriority[2]) == 0 {
			continue
		}
		st := g.stats[s]
		if st == nil {
			st = &stat{}
			g.stats[s] = st
		}
		for _, modifiers := range byPriority {
			for _, m := range modifiers {
				value := m.Apply(st.base, st.base+st.bonus)
				if m.Bonus {
					st.bonus += value
				} else {
					st.base = value
				}
			}
		}
	}
	g.set(data.AttackSpeed, data.Round(float32(g.base(data.MainHandAttackSpeed))+float32(float32(g.base(data.OffHandAttackSpeed))*0.25)), false)
	g.set(data.AttackSpeed, g.bonus(data.MainHandAttackSpeed)+g.bonus(data.OffHandAttackSpeed), true)
}

// isHandStat is StatEnum.isMainOrSubHandStat: a stat each hand has its own of.
func isHandStat(s data.Stat) bool {
	return s == data.PhysicalAttack || s == data.Power || s == data.PhysicalAccuracy || s == data.PhysicalCritical
}

// handStat is StatEnum.getMainOrSubHandStat: the hand's own stat for a slot.
// Its cases fall through as in Java, so a slot that is neither hand ends at
// the main hand's attack speed.
func handStat(s data.Stat, slot int32) data.Stat {
	switch s {
	case data.PhysicalAttack, data.Power:
		if slot == data.SlotSubHand {
			return data.OffHandPower
		}
		if slot == data.SlotMainHand {
			return data.MainHandPower
		}
		fallthrough
	case data.PhysicalAccuracy:
		if slot == data.SlotSubHand {
			return data.OffHandAccuracy
		}
		if slot == data.SlotMainHand {
			return data.MainHandAccuracy
		}
		fallthrough
	case data.PhysicalCritical:
		if slot == data.SlotSubHand {
			return data.OffHandCritical
		}
		if slot == data.SlotMainHand {
			return data.MainHandCritical
		}
		fallthrough
	case data.HitCount:
		if slot == data.SlotSubHand {
			return data.OffHandHits
		}
		if slot == data.SlotMainHand {
			return data.MainHandHits
		}
		fallthrough
	case data.AttackSpeed:
		if slot == data.SlotSubHand {
			return data.OffHandAttackSpeed
		}
		return data.MainHandAttackSpeed
	}
	return s
}
