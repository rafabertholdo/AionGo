package game

import (
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// The stats an enchanted weapon or armor gives (EnchantService.EnchantWeapon): a bonus for each level.

type enchantBonus struct {
	stat data.Stat
	per  int32
}

var (
	enchantDagger = []enchantBonus{{data.PhysicalAttack, 2}}
	enchantHeavy  = []enchantBonus{{data.PhysicalAttack, 4}}
	enchantCaster = []enchantBonus{{data.PhysicalAttack, 3}, {data.BoostMagicalSkill, 20}}
)

// enchantWeapons is EnchantService.getWeaponModifiers, by weapon type.
var enchantWeapons = map[string][]enchantBonus{
	"BOOK_2H": enchantCaster, "DAGGER_1H": enchantDagger, "BOW": enchantHeavy, "ORB_2H": enchantCaster, "STAFF_2H": enchantCaster,
	"SWORD_1H": enchantDagger, "SWORD_2H": enchantHeavy, "MACE_1H": enchantCaster, "POLEARM_2H": enchantHeavy,
}

// defense is DEF1…DEF12: physical defense, life and critical resistance per level.
func defense(pdef, hp, crit int32) []enchantBonus {
	return []enchantBonus{{data.PhysicalDefense, pdef}, {data.MaxHP, hp}, {data.CriticalResist, crit}}
}

// enchantArmors is EnchantService.getArmorModifiers: by armor type, for the torso, the boots, shoulders, gloves and pants.
var enchantArmors = map[string]map[int32][]enchantBonus{
	"ROBE":    {1 << 3: defense(3, 14, 4), 1 << 5: defense(1, 10, 2), 1 << 11: defense(1, 10, 2), 1 << 4: defense(1, 10, 2), 1 << 12: defense(2, 12, 3)},
	"LEATHER": {1 << 3: defense(4, 12, 4), 1 << 5: defense(3, 8, 2), 1 << 11: defense(3, 8, 2), 1 << 4: defense(3, 8, 2), 1 << 12: defense(5, 10, 3)},
	"CHAIN":   {1 << 3: defense(5, 10, 4), 1 << 5: defense(3, 6, 2), 1 << 11: defense(3, 6, 2), 1 << 4: defense(3, 6, 2), 1 << 12: defense(4, 8, 3)},
	"PLATE":   {1 << 3: defense(6, 8, 4), 1 << 5: defense(4, 4, 2), 1 << 11: defense(4, 4, 2), 1 << 4: defense(4, 4, 2), 1 << 12: defense(5, 6, 3)},
}

// enchantModifiers is what an item's enchant level adds to the stats of its wearer.
func enchantModifiers(t *data.ItemTemplate, slot int32, level int32) data.Modifiers {
	if level == 0 {
		return nil
	}
	var bonuses []enchantBonus
	switch {
	case t.IsWeapon():
		bonuses = enchantWeapons[t.WeaponType]
	case t.IsArmor():
		bonuses = enchantArmors[t.ArmorType][slot]
	}
	mods := make(data.Modifiers, 0, len(bonuses))
	for _, b := range bonuses {
		mods = append(mods, data.Modifier{Kind: data.ModAdd, Stat: b.stat, Value: b.per * level, Bonus: true})
	}
	return mods
}

const (
	msgEnchantSucceed = 1300455
	msgEnchantFailed  = 1300456
	msgOptionSucceed  = 1300462
	msgOptionFailed   = 1300463
	enchantDelay      = 5 * time.Second
)

// startEnchanting is EnchantItemAction.act: five seconds of animation, then the enchant stone or manastone works.
func (s *Server) startEnchanting(p *player, stone, target, supplement *store.Item) {
	p.conn.send(itemUsageAnimation(p.ID, stone.UniqueID, stone.ItemID, int32(enchantDelay/time.Millisecond), 0, 0))
	p.itemUse.cancel()
	p.itemUse = s.later(enchantDelay, func() {
		if p.dead || itemIn(p.cube, stone.UniqueID) == nil {
			return
		}
		var ok bool
		if stone.ItemID > 166000000 && stone.ItemID < 167000000 {
			ok = s.enchantItem(p, stone, target, supplement)
		} else {
			ok = s.socketManastone(p, stone, target, supplement)
		}
		end := int32(2)
		if ok {
			end = 1
		}
		p.conn.send(itemUsageAnimation(p.ID, stone.UniqueID, stone.ItemID, 0, byte(end), 0))
	})
}

// qualityCap is how many levels above an item an enchant stone must be before it helps.
func qualityCap(quality string) int32 {
	switch quality {
	case "RARE":
		return 5
	case "LEGEND", "MYTHIC":
		return 10
	case "EPIC", "UNIQUE":
		return 15
	}
	return 0
}

// enchantItem is EnchantService.enchantItem: a stone that is too low fails, a supplement adds to the chance, and a
// failure takes an enchanted item back a level (or to 10).
func (s *Server) enchantItem(p *player, stone, target, supplement *store.Item) bool {
	st, t := s.template(stone), s.template(target)
	if st == nil || t == nil || t.Level > st.Level {
		return false
	}
	success := int32(50)
	if extra := st.Level - t.Level - qualityCap(t.Quality); extra > 0 {
		success += extra * 5
	}
	if supplement != nil {
		// Like socketManastone, an unknown supplement or too few of them fail instead of Java's default bonus
		// and partial use.
		bonus := manastoneSupplementBonus(s.template(supplement))
		count := enchantSupplementCount(st.Level, target.Enchant)
		if bonus == 0 || s.countItems(p, supplement.ItemID) < count {
			return false
		}
		s.decreaseItemsByID(p, supplement.ItemID, count)
		success += int32(bonus)
	}
	success = min(success, 95)
	won := rnd(0, 100) < success
	level := int32(target.Enchant)
	switch {
	case !won && level > 10:
		level = 10
	case !won && level > 0:
		level--
	case won && (t.Quality == "UNIQUE" || t.Quality == "EPIC") && level < 15, won && level < 10:
		level++
	}
	target.Enchant = int8(level)
	s.itemChanged(p, target)
	if won {
		p.conn.send(systemMessage(msgEnchantSucceed, descriptionID(t.NameID)))
	} else {
		p.conn.send(systemMessage(msgEnchantFailed, descriptionID(t.NameID)))
	}
	s.decreaseItemCount(p, stone, 1)
	return won
}

// manastoneRate is the chance to socket a manastone when the item has that many: this server's enchants.properties.
var manastoneRates = [...]int{76, 57, 43, 33, 25, 19, 2}

func manastoneSuccessRate(socketed int) int {
	if socketed >= len(manastoneRates)-1 {
		return manastoneRates[len(manastoneRates)-1]
	}
	return manastoneRates[socketed]
}

// socketManastone is EnchantService.socketManastone, without a supplement.
func (s *Server) socketManastone(p *player, stone, target, supplement *store.Item) bool {
	t := s.template(target)
	stoneTemplate := s.template(stone)
	if t == nil || stoneTemplate == nil || stoneTemplate.ID < 167000000 || stoneTemplate.ID >= 168000000 ||
		(!t.IsWeapon() && !t.IsArmor()) {
		return false
	}
	stones := p.stones[target.UniqueID]
	rate := manastoneSuccessRate(len(stones))
	supplementCount := int64(0)
	if supplement != nil {
		supplementTemplate := s.template(supplement)
		bonus := manastoneSupplementBonus(supplementTemplate)
		if bonus == 0 {
			return false
		}
		rate += bonus
		supplementCount = manastoneSupplementCount(stoneTemplate.ID, stoneTemplate.Level, len(stones))
		if supplementTemplate == nil || s.countItems(p, supplement.ItemID) < supplementCount {
			return false
		}
		s.decreaseItemCount(p, supplement, supplementCount)
	}
	won := rnd(0, 100) < int32(min(rate, 100))
	if won {
		if len(stones) <= 6 {
			// The first free slot.
			slot := int32(0)
			for slices.ContainsFunc(stones, func(o store.Stone) bool { return o.Slot == slot }) {
				slot++
			}
			added := store.Stone{ItemID: stone.ItemID, Slot: slot}
			p.stones[target.UniqueID] = append(stones, added)
			if err := s.items.AddStone(target.UniqueID, added); err != nil {
				s.log.Error("saving a stone", "item", target.ItemID, "err", err)
			}
		}
		p.conn.send(systemMessage(msgOptionSucceed, descriptionID(t.NameID)))
	} else {
		p.conn.send(systemMessage(msgOptionFailed, descriptionID(t.NameID)))
		delete(p.stones, target.UniqueID)
		if err := s.items.DeleteStones(target.UniqueID); err != nil {
			s.log.Error("deleting stones", "item", target.ItemID, "err", err)
		}
	}
	s.itemChanged(p, target)
	s.decreaseItemCount(p, stone, 1)
	return won
}

func manastoneSupplementBonus(t *data.ItemTemplate) int {
	if t == nil {
		return 0
	}
	switch t.ID {
	case 166100000, 166100003, 166100006:
		return 10
	case 166100001, 166100004, 166100007:
		return 15
	case 166100002, 166100005, 166100008:
		return 20
	default:
		return 0
	}
}

// enchantSupplementCount is how many supplements an enchant stone of the level uses, doubled above +10.
func enchantSupplementCount(stoneLevel int32, enchant int8) int64 {
	var count int64
	switch {
	case stoneLevel > 90:
		count = 145
	case stoneLevel > 80:
		count = 115
	case stoneLevel > 70:
		count = 85
	case stoneLevel > 60:
		count = 55
	case stoneLevel > 50:
		count = 25
	case stoneLevel > 40:
		count = 10
	case stoneLevel > 30:
		count = 5
	default:
		count = 1
	}
	if enchant+1 > 10 {
		count *= 2
	}
	return count
}

func manastoneSupplementCount(stoneID, level int32, socketed int) int64 {
	count := int64(1)
	if level > 30 {
		count++
	}
	if level > 40 {
		count++
	}
	if level > 50 {
		count++
	}
	switch stoneID {
	case 167000230, 167000235, 167000294, 167000267, 167000299:
		count = 5
	case 167000331:
		count = 10
	case 167000358, 167000363:
		count = 15
	case 167000550:
		count = 20
	case 167000454, 167000427, 167000459:
		count = 25
	case 167000491:
		count = 50
	case 167000518, 167000522:
		count = 75
	}
	if socketed > 0 {
		count *= int64(socketed + 1)
	}
	return count
}

// itemChanged saves an item that was enchanted or socketed, and its wearer's stats change if it is worn.
func (s *Server) itemChanged(p *player, item *store.Item) {
	s.saveItem(item)
	if item.Equipped {
		p.stats = s.playerStats(p)
		p.conn.send(s.statsInfo(p))
	}
	p.conn.send(s.updateItemPacket(p, item))
}
