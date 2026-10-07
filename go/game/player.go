package game

import (
	"cmp"
	"maps"
	"slices"
	"time"

	"aionlightning/game/data"
	"aionlightning/game/store"
)

// player is a character in the world.
type player struct {
	*character
	adminInvulnerable bool
	adminDPS          *task // //dps hitting the current target each second
	adminAppearance   *store.Appearance
	level             int
	stats             *gameStats
	life              store.LifeStats // current HP, MP and FP
	skills            []store.Skill
	quests            []store.Quest
	recipes           []int32
	settings          *store.Settings
	macros            []store.Macro
	titles            []int32
	abyss             *store.AbyssRank
	equipment         []*store.Item // worn, by slot
	cube              []*store.Item // carried, kinah aside
	kinah             *store.Item
	stones            map[int32][]store.Stone // manastones, by the item they are in

	punish           punishment // gag and prison
	visualState      byte
	seeState         byte // CreatureSeeState bits: the hidden it sees through
	friends          []*store.Character
	group            *group
	alliance         *alliance
	kisk             *object // the kisk it is bound to
	legion           *legion
	member           *legionMember  // its place in the legion
	stigma           map[int32]bool // the skills its stigma stones teach
	summon           *object        // the summon it has called
	instance         int32          // the instance of its map it is in, from 0
	mailbox          []*letter
	hurtBy           map[int32]int32 // the damage players have done to it, by player id
	store            *privateStore   // what it sells, if it has a store open
	trading          bool
	itemUse          *task           // an enchant that is being done
	skillXP          map[int32]int32 // the experience of the crafting and gathering skills, kept only while playing
	interaction      *interaction    // gathering or crafting
	flyState         byte            // 0 on the ground, 1 flying, 2 gliding
	flightTeleportID int32           // the client-side flight path of a flight teleport in progress, else 0
	flightDistance   int32
	fpTask           *task         // reduces or restores the fly time
	zone             *data.Zone    // the zone it is in, if any
	drowning         *task         // takes life while it is in water
	warehouse        []*store.Item // the regular warehouse, and its money
	warehouseKinah   *store.Item
	acctWH           *accountWarehouse // shared by the characters of its account
	friendStatus     byte              // FriendList.Status
	blocks           []store.Block
	rebirthPercent   int32  // how much life a rebirth effect gives back, remembered at death
	transformed      int32  // the npc model it looks like under a transform effect, or 0
	state            uint16 // CreatureState: active, resting, weapon out, walking …
	targetID         int32
	dead             bool
	lastAttack       time.Time
	attackCounter    int32                        // the next attack's number
	restore          *task                        // regenerates HP and MP
	protection       *task                        // ends the protection after entering a map
	requests         map[int32]request            // questions it was asked and hasn't answered
	itemCooldowns    map[int32]store.ItemCooldown // when each use delay group of items is ready again
	cast             *skill                       // what it is casting
	cooldowns        map[int32]time.Time          // when each skill can be used again
	savedEffects     []store.SavedEffect          // player_effects rows, restored once it enters the world
	moves            int32                        // how many times it has set out to move
	dp               int32                        // divine power
	lookingForGroup  bool                         // session-only LFG toggle (CM_PLAYER_STATUS_INFO 9)
	fx               effectController             // the effects it is under
	fxMods           []keyedMods                  // the stat changes of those
	dirtyHP, dirtyMP bool                         // to tell the client of
	cell             cell

	// In the world, guarded by Server.visMu.
	conn    *conn
	spawned bool
	known   map[int32]*player // the players it sees: KnownList
	seen    map[int32]*object // the npcs and gatherables it sees
	target  [3]float32        // where it last set out for: MoveController
}

// loadPlayer is PlayerService.getPlayer: everything about a character that
// entering the world needs, from the database.
func (s *Server) loadPlayer(ch *character) (*player, error) {
	p := &player{character: ch, state: stateActive, attackCounter: 1, level: s.data.Level(ch.Exp), stones: map[int32][]store.Stone{}, known: map[int32]*player{}, seen: map[int32]*object{}}
	p.fx = newEffectController(p)
	id := ch.ID
	var err error
	if err = s.loadSocial(p); err != nil {
		return nil, err
	}
	if p.macros, err = s.store.Macros(id); err != nil {
		return nil, err
	}
	if p.skills, err = s.store.Skills(id); err != nil {
		return nil, err
	}
	if err = s.restoreAutolearnSkills(p); err != nil {
		return nil, err
	}
	if p.titles, err = s.store.Titles(id); err != nil {
		return nil, err
	}
	if p.settings, err = s.store.Settings(id); err != nil {
		return nil, err
	}
	if p.abyss, err = s.store.AbyssRank(id); err != nil {
		return nil, err
	}
	if p.abyss == nil {
		p.abyss = &store.AbyssRank{Rank: 1, MaxRank: 1}
		if err := s.store.InsertAbyssRank(id, p.abyss); err != nil {
			return nil, err
		}
	}
	if p.equipment, err = s.store.Items(id, 0, true); err != nil {
		return nil, err
	}
	slices.SortStableFunc(p.equipment, func(a, b *store.Item) int { return cmp.Compare(a.Slot, b.Slot) })
	if p.quests, err = s.store.Quests(id); err != nil {
		return nil, err
	}
	if p.recipes, err = s.store.Recipes(id); err != nil {
		return nil, err
	}
	carried, err := s.store.Items(id, 0, false)
	if err != nil {
		return nil, err
	}
	stored, err := s.store.Items(id, storageWarehouse, false)
	if err != nil {
		return nil, err
	}
	for _, item := range stored {
		if item.ItemID == data.Kinah && p.warehouseKinah == nil {
			p.warehouseKinah = item
		} else {
			p.warehouse = append(p.warehouse, item)
		}
	}
	if p.acctWH, err = s.accountWarehouseOf(ch.AccountID); err != nil {
		return nil, err
	}
	for _, item := range carried {
		if item.ItemID == data.Kinah && p.kinah == nil {
			p.kinah = item
		} else {
			p.cube = append(p.cube, item)
		}
	}
	for _, item := range append(slices.Clone(p.equipment), p.cube...) {
		stones, err := s.store.Stones(item.UniqueID)
		if err != nil {
			return nil, err
		}
		for _, stone := range stones {
			if stone.Category == 0 {
				p.stones[item.UniqueID] = append(p.stones[item.UniqueID], stone)
			}
		}
		slices.SortFunc(p.stones[item.UniqueID], func(a, b store.Stone) int { return cmp.Compare(a.Slot, b.Slot) })
	}
	// ItemService.restoreKinah: a character always carries kinah, if only 0.
	if p.kinah == nil {
		p.kinah = &store.Item{UniqueID: s.ids.nextID(), ItemID: data.Kinah, Owner: id}
		if err := s.store.InsertItem(p.kinah); err != nil {
			return nil, err
		}
	}
	s.stigmaLogin(p)
	s.visMu.Lock()
	err = s.loadLegion(p)
	s.visMu.Unlock()
	if err != nil {
		return nil, err
	}
	if err := s.loadPunishment(p); err != nil {
		return nil, err
	}
	p.stats = s.playerStats(p)
	life, err := s.store.LifeStats(id)
	if err != nil {
		return nil, err
	}
	if life == nil {
		life = &store.LifeStats{HP: p.stats.current(data.MaxHP), MP: p.stats.current(data.MaxMP), FP: p.stats.current(data.FlyTime)}
		if err := s.store.SaveLifeStats(id, *life); err != nil {
			return nil, err
		}
	}
	p.life = *life
	if p.savedEffects, err = s.store.Effects(id); err != nil {
		return nil, err
	}
	if p.itemCooldowns, err = s.store.ItemCooldowns(id); err != nil {
		return nil, err
	}
	maps.DeleteFunc(p.itemCooldowns, func(_ int32, c store.ItemCooldown) bool { return !c.Reuse.After(time.Now()) })
	return p, nil
}

// playerStats is a player's stats from its class template, worn items, passive
// skills and title, added in the order PlayerService.getPlayer adds them.
// Godstones add no stats: they proc in godstoneProcs.
func (s *Server) playerStats(p *player) *gameStats {
	g := newPlayerStats(s.data.PlayerStatsFor(p.Class, p.level), p.level)
	passives := s.newPassives(p, g)
	setsApplied := map[[2]int32]bool{}
	for _, item := range p.equipment {
		if item.Slot == data.SlotMainOff || item.Slot == data.SlotSubOff {
			continue
		}
		template := s.data.Items[item.ItemID]
		if template == nil {
			continue
		}
		g.add(statEffect{item: true, slot: item.Slot, modifiers: template.Modifiers})
		g.add(statEffect{modifiers: enchantModifiers(template, item.Slot, int32(item.Enchant))})
		if set := s.data.ItemSets[item.ItemID]; set != nil {
			worn := p.setPartsWorn(s.data, set.ID)
			for _, bonus := range set.PartBonuses {
				key := [2]int32{set.ID, int32(bonus.Count)}
				if bonus.Count <= worn && !setsApplied[key] {
					setsApplied[key] = true
					g.add(statEffect{modifiers: bonus.Modifiers})
				}
			}
			if full := set.FullBonus; full != nil && worn == full.Count {
				key := [2]int32{set.ID, int32(worn + 1)}
				if !setsApplied[key] {
					setsApplied[key] = true
					g.add(statEffect{modifiers: full.Modifiers})
				}
			}
		}
		for _, stone := range p.stones[item.UniqueID] {
			if template := s.data.Items[stone.ItemID]; template != nil {
				g.add(statEffect{modifiers: template.Modifiers})
			}
		}
		if template.IsWeapon() {
			passives.weaponMastery()
		}
		if template.IsArmor() {
			for _, armor := range armorTypes {
				passives.useMastery(passives.armor[armor])
			}
		}
	}
	passives.useAll()
	if title := s.data.Titles[p.TitleID]; title != nil {
		g.add(statEffect{modifiers: title.Modifiers})
	}
	for _, k := range p.fxMods {
		g.addKeyed(k.key, k.mods)
	}
	g.recompute(p.offHandWeapon(s.data))
	return g
}

func (p *player) setPartsWorn(d *data.Data, setID int32) int {
	worn := 0
	for _, item := range p.equipment {
		if set := d.ItemSets[item.ItemID]; set != nil && set.ID == setID {
			worn++
		}
	}
	return worn
}

// offHandWeapon reports whether a weapon is worn in the off hand: Equipment.getOffHandWeaponType.
func (p *player) offHandWeapon(d *data.Data) bool {
	for _, item := range p.equipment {
		if t := d.Items[item.ItemID]; item.Slot == data.SlotSubHand && t != nil && t.IsWeapon() {
			return true
		}
	}
	return false
}
