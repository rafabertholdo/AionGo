package game

import (
	"cmp"
	"slices"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

// Server packets for entering the world, as AL-Game writes them.

func skillList(p *player) *wire.Writer {
	w := wire.Packet(smSkillList)
	skills := javaHashOrder(p.skills, func(s store.Skill) int32 { return s.ID })
	w.H(uint16(len(skills)))
	for _, s := range skills {
		w.H(uint16(s.ID))
		w.H(uint16(s.Level))
		w.C(0)
		w.C(0) // extra level
		w.D(0)
		w.Bool(p.stigma[s.ID])
	}
	// AL-Game writes the "you learned" message part even with no message.
	w.D(0)
	w.H(0x24)
	w.D(0)
	w.H(0)
	w.H(0)
	return w
}

// questList is SM_QUEST_LIST: completed quests, then those in progress.
func questList(p *player) *wire.Writer {
	quests := slices.Clone(p.quests)
	slices.SortFunc(quests, func(a, b store.Quest) int { return cmp.Compare(a.ID, b.ID) })
	var complete, started []store.Quest
	for _, q := range quests {
		switch q.Status {
		case "COMPLETE":
			complete = append(complete, q)
		case "NONE":
		default:
			started = append(started, q)
		}
	}
	w := wire.Packet(smQuestList)
	w.H(uint16(len(complete)))
	for _, q := range complete {
		w.H(uint16(q.ID))
		w.H(0)
		w.C(byte(q.CompleteCount))
	}
	w.C(byte(len(started)))
	for _, q := range started {
		w.H(uint16(q.ID))
		w.H(0)
	}
	for _, q := range started {
		w.C(questStatus[q.Status])
		w.D(q.Vars)
		w.C(0)
	}
	return w
}

// questStatus is QuestStatus.value.
var questStatus = map[string]byte{"NONE": 0, "START": 3, "REWARD": 4, "COMPLETE": 5, "LOCKED": 6}

func recipeList(p *player) *wire.Writer {
	w := wire.Packet(smRecipeList)
	w.H(uint16(len(p.recipes)))
	for _, id := range javaHashOrder(p.recipes, func(id int32) int32 { return id }) {
		w.D(id)
		w.C(0)
	}
	return w
}

func enterWorldCheck() *wire.Writer {
	w := wire.Packet(smEnterWorldCheck)
	w.B([]byte{0, 0, 0})
	return w
}

func uiSettings(kind uint16, blob []byte) *wire.Writer {
	w := wire.Packet(smUiSettings)
	w.H(kind)
	w.C(0x1c)
	w.B(blob)
	return w
}

// inventoryInfo is SM_INVENTORY_INFO with items; nil items is the empty packet that ends the list.
func (s *Server) inventoryInfo(p *player, items []*store.Item) *wire.Writer {
	w := wire.Packet(smInventoryInfo)
	if items == nil {
		w.D(0)
		w.H(0)
		return w
	}
	w.C(1)
	w.C(byte(p.CubeSize))
	w.C(0)
	w.C(0)
	w.H(uint16(len(items)))
	for _, item := range items {
		s.writeItem(w, p, item)
	}
	return w
}

// writeItem is InventoryPacket's general info and the details for the item's kind.
func (s *Server) writeItem(w *wire.Writer, p *player, item *store.Item) {
	t := s.data.Items[item.ItemID]
	if t == nil {
		t = &data.ItemTemplate{ID: item.ItemID}
	}
	w.D(item.UniqueID)
	w.D(t.ID)
	w.H(0x24)
	w.D(t.NameID)
	w.H(0)
	s.writeItemDetails(w, p, item, t)
}

// writeItemDetails is what follows an item's general info in the inventory, and in SM_ADD_ITEMS.
func (s *Server) writeItemDetails(w *wire.Writer, p *player, item *store.Item, t *data.ItemTemplate) {
	equippedSlot := int32(0)
	if item.Equipped {
		equippedSlot = item.Slot
	}
	shownSlot := uint16(item.Slot)
	if item.Equipped {
		shownSlot = 255
	}
	switch {
	case t.ID == data.Kinah:
		w.H(0x16)
		w.C(0)
		w.H(uint16(t.Mask))
		w.Q(item.Count)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.H(255)
		w.C(0)
	case t.IsWeapon():
		w.H(0x4b)
		w.C(0x06)
		w.D(equippedSlot)
		w.C(0x01)
		w.D(data.FirstSlot(t.Slot))
		w.D(0x02)
		w.C(0x0b)
		w.Bool(item.SoulBound)
		w.C(byte(item.Enchant))
		w.D(item.SkinID())
		w.C(0)
		s.writeStones(w, p.stones[item.UniqueID])
		w.D(item.Godstone)
		w.C(0)
		w.D(0)
		w.D(0)
		w.C(0)
		w.H(uint16(t.Mask))
		w.Q(item.Count)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.H(shownSlot)
		w.C(0)
	case t.IsArmor():
		w.H(0x4f)
		w.C(0x06)
		w.D(equippedSlot)
		w.C(0x02)
		w.D(data.FirstSlot(t.Slot))
		w.D(0)
		w.D(0)
		w.C(0x0b)
		w.Bool(item.SoulBound)
		w.C(byte(item.Enchant))
		w.D(item.SkinID())
		w.C(0)
		s.writeStones(w, p.stones[item.UniqueID])
		w.C(0)
		w.D(item.Color)
		w.D(0)
		w.D(0)
		w.C(0)
		w.H(uint16(t.Mask))
		w.Q(item.Count)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.H(shownSlot)
		w.C(1)
	case t.IsStigma():
		writeStigma(w, item, equippedSlot)
	default:
		w.H(0x16)
		w.C(0)
		w.H(uint16(t.Mask))
		w.Q(item.Count)
		w.D(0)
		w.D(0)
		w.H(0)
		w.C(0)
		w.H(uint16(item.Slot))
		w.C(0)
	}
}

// writeStones is up to six manastones: their stats' masks, then their values.
func (s *Server) writeStones(w *wire.Writer, stones []store.Stone) {
	var masks [6]byte
	var values [6]uint16
	count := 0
	for _, stone := range stones {
		t := s.data.Items[stone.ItemID]
		if count == 6 || t == nil || len(t.Modifiers) == 0 {
			continue
		}
		masks[count] = t.Modifiers[0].Stat.StoneMask()
		values[count] = uint16(t.Modifiers[0].Value)
		count++
	}
	w.B(masks[:])
	for _, v := range values {
		w.H(v)
	}
}

// writeStigma is InventoryPacket.writeStigmaInfo, a fixed block around the stone's id.
func writeStigma(w *wire.Writer, item *store.Item, equippedSlot int32) {
	w.H(325)
	w.C(0x6)
	w.D(equippedSlot)
	w.C(0x7)
	w.H(702)
	w.D(0)
	w.H(0)
	w.D(0x3c)
	w.B(make([]byte, 4*40))
	w.D(1)
	w.B(make([]byte, 4*20))
	w.H(0)
	w.H(0x0b)
	w.C(0)
	w.D(item.ItemID)
	w.B(make([]byte, 4*8))
	w.C(0)
	w.D(82750)
	w.B(make([]byte, 4*4))
	w.C(0)
	w.C(0x22)
	w.H(0)
}

// statsInfo is SM_STATS_INFO: the character window.
func (s *Server) statsInfo(p *player) *wire.Writer {
	g := p.stats
	cur := func(st data.Stat) uint16 { return uint16(g.current(st)) }
	base := func(st data.Stat) uint16 { return uint16(g.base(st)) }
	w := wire.Packet(smStatsInfo)
	w.D(p.ID)
	w.D(s.gameTime())
	for _, st := range []data.Stat{data.Power, data.Health, data.Accuracy, data.Agility, data.Knowledge, data.Will,
		data.WaterResistance, data.WindResistance, data.EarthResistance, data.FireResistance} {
		w.H(cur(st))
	}
	w.H(0)
	w.H(0)
	w.H(uint16(p.level))
	w.H(0)
	w.H(0)
	w.H(0)
	w.Q(s.expNeed(p.level))
	w.Q(p.RecoverExp)
	w.Q(p.Exp - s.data.ExpStart(p.level))
	w.D(0)
	w.D(g.current(data.MaxHP))
	w.D(p.life.HP)
	w.D(g.current(data.MaxMP))
	w.D(p.life.MP)
	w.H(cur(data.MaxDP))
	w.H(0) // dp
	w.D(g.current(data.FlyTime))
	w.D(p.life.FP)
	w.C(p.flyState)
	w.C(0)
	w.H(cur(data.MainHandPower))
	w.H(cur(data.OffHandPower))
	w.H(cur(data.PhysicalDefense))
	w.H(cur(data.MainHandPower))
	w.H(cur(data.MagicalResist))
	w.F(float32(g.current(data.AttackRange)) / 1000)
	for _, st := range []data.Stat{data.AttackSpeed, data.Evasion, data.Parry, data.Block, data.MainHandCritical,
		data.OffHandCritical, data.MainHandAccuracy, data.OffHandAccuracy} {
		w.H(cur(st))
	}
	w.H(0)
	w.H(cur(data.MagicalAccuracy))
	w.H(0)
	w.H(0)
	w.H(0)
	w.H(16256)
	w.H(40)
	w.H(uint16(g.current(data.MagicalAttack) + g.current(data.BoostMagicalSkill)))
	w.H(uint16(g.current(data.BoostHeal) - 100))
	w.H(cur(data.CriticalResist))
	w.H(0)
	w.H(0)
	w.H(0)
	w.H(20511)
	w.D(int32(27 + p.CubeSize*9))
	w.D(int32(len(p.cube)))
	w.D(0)
	w.D(0)
	w.D(classIDs[p.Class])
	w.Q(0)
	w.Q(0)
	w.Q(251141)
	w.Q(0)
	for _, st := range []data.Stat{data.Power, data.Health, data.Accuracy, data.Agility, data.Knowledge, data.Will,
		data.WaterResistance, data.WindResistance, data.EarthResistance, data.FireResistance} {
		w.H(base(st))
	}
	w.D(0)
	w.D(g.base(data.MaxHP))
	w.D(g.base(data.MaxMP))
	w.D(g.base(data.MaxDP))
	w.D(g.base(data.FlyTime))
	w.H(base(data.MainHandPower))
	w.H(base(data.OffHandPower))
	w.H(base(data.MainHandPower))
	w.H(base(data.PhysicalDefense))
	w.H(base(data.MagicalResist))
	w.H(0)
	w.F(float32(g.current(data.AttackRange)) / 1000)
	w.H(base(data.Evasion))
	w.H(base(data.Parry))
	w.H(base(data.Block))
	w.H(base(data.MainHandCritical))
	w.H(base(data.OffHandCritical))
	w.H(cur(data.MagicalCritical))
	w.H(0)
	w.H(base(data.MainHandAccuracy))
	w.H(base(data.OffHandAccuracy))
	w.H(0)
	w.H(base(data.MagicalAccuracy))
	w.H(0)
	w.H(uint16(g.base(data.MagicalAttack) + g.base(data.BoostMagicalSkill)))
	w.H(uint16(g.base(data.BoostHeal) - 100))
	w.H(base(data.CriticalResist))
	w.H(0)
	w.H(0)
	w.H(0)
	return w
}

func (s *Server) expNeed(level int) int64 {
	if level >= s.data.MaxLevel() {
		return 0
	}
	return s.data.ExpStart(level+1) - s.data.ExpStart(level)
}

func cubeUpdate(p *player) *wire.Writer {
	w := wire.Packet(smCubeUpdate)
	w.C(6)
	w.C(byte(p.StigmaSlots))
	return w
}

func cubeSizeUpdate(p *player) *wire.Writer {
	w := wire.Packet(smCubeUpdate)
	w.C(0)
	w.C(0)
	w.D(int32(p.cubeLimit()))
	w.C(byte(p.CubeSize))
	w.C(0)
	w.C(0)
	return w
}

// bindPoint is SM_SET_BIND_POINT: where the player returns on death, its bind
// point or else its race's starting place.
func (s *Server) bindPoint(p *player) *wire.Writer {
	at, ok := s.data.BindPoints[p.BindPoint]
	if p.BindPoint == 0 || !ok {
		at = s.startLocation(p.Race)
	}
	w := wire.Packet(smSetBindPoint)
	if p.kisk != nil {
		w.C(4)
	} else {
		w.C(0)
	}
	w.C(1)
	w.D(at.MapID)
	w.F(at.X)
	w.F(at.Y)
	w.F(at.Z)
	if p.kisk != nil {
		w.D(p.kisk.id)
	} else {
		w.D(0)
	}
	return w
}

func (s *Server) startLocation(race string) data.Location {
	if race == "ASMODIANS" {
		return s.data.Initial.Asmodians
	}
	return s.data.Initial.Elyos
}

func playerID(p *player) *wire.Writer {
	w := wire.Packet(smPlayerId)
	w.H(2)
	w.D(0)
	w.H(1)
	w.D(p.ID)
	w.H(0)
	w.S(p.Name)
	return w
}

func macroList(p *player) *wire.Writer {
	w := wire.Packet(smMacroList)
	w.D(p.ID)
	w.C(1)
	w.H(uint16(-len(p.macros)))
	for _, m := range javaHashOrder(p.macros, func(m store.Macro) int32 { return m.Order }) {
		w.C(byte(m.Order))
		w.S(m.Text)
	}
	return w
}

func (s *Server) gameTimePacket() *wire.Writer {
	w := wire.Packet(smGameTime)
	w.D(s.gameTime())
	return w
}

func titleList(p *player) *wire.Writer {
	w := wire.Packet(smTitleList)
	w.C(0)
	w.H(uint16(len(p.titles)))
	for _, id := range p.titles {
		w.D(id)
		w.D(0)
	}
	return w
}

// channelInfo is SM_CHANNEL_INFO: the player's channel and how many the map has.
func (s *Server) channelInfo(p *player) *wire.Writer {
	w := wire.Packet(smChannelInfo)
	w.D(p.instance)
	channels := int32(1)
	if m := s.data.WorldMaps[p.WorldID]; m != nil && m.TwinCount > 0 {
		channels = m.TwinCount
	}
	if m := s.data.WorldMaps[p.WorldID]; m != nil && m.Instance {
		channels = max(s.instanceCount(p.WorldID), p.instance+1)
	}
	w.D(channels)
	return w
}

func playerSpawn(p *player) *wire.Writer {
	w := wire.Packet(smPlayerSpawn)
	w.D(p.WorldID)
	w.D(p.WorldID)
	w.D(0)
	w.C(0)
	w.F(p.X)
	w.F(p.Y)
	w.F(p.Z)
	w.C(byte(p.Heading))
	return w
}

func emotionList() *wire.Writer {
	w := wire.Packet(smEmotionList)
	w.C(0)
	w.H(13)
	for i := range int32(13) {
		w.D(64 + i)
		w.H(0)
	}
	return w
}

// prices is SM_PRICES: prices, their modifier and taxes, all 100%.
func prices() *wire.Writer {
	w := wire.Packet(smPrices)
	w.C(100)
	w.C(100)
	w.C(100)
	return w
}

// abyssRankAP is the AP each abyss rank needs, AbyssRankEnum's required, from rank 1.
var abyssRankAP = []int32{0, 1200, 4220, 10990, 23500, 42780, 69700, 105600, 150800, 214100, 278700,
	344500, 411700, 488200, 565400, 643200, 721600, 800700}

func abyssRank(a *store.AbyssRank) *wire.Writer {
	next := a.Rank
	if next < int32(len(abyssRankAP)) {
		next++
	}
	progress := int32(0)
	if next >= 1 && next <= int32(len(abyssRankAP)) && abyssRankAP[next-1] > 0 {
		progress = 100 * a.AP / abyssRankAP[next-1]
	}
	w := wire.Packet(smAbyssRank)
	w.Q(int64(a.AP))
	w.D(a.Rank)
	w.D(a.TopRanking)
	w.D(progress)
	w.D(a.AllKill)
	w.D(a.MaxRank)
	w.D(a.DailyKill)
	w.Q(int64(a.DailyAP))
	w.D(a.WeeklyKill)
	w.Q(int64(a.WeeklyAP))
	w.D(a.LastKill)
	w.Q(int64(a.LastAP))
	w.C(0)
	return w
}

// Chat types, ChatType.toInteger.
const chatAnnouncement = 0x19

// message is SM_MESSAGE from nobody: a notice in the chat window.
func message(chatType byte, text string) *wire.Writer {
	w := wire.Packet(smMessage)
	w.C(chatType)
	w.C(0)
	w.D(0)
	w.H(0)
	w.S(text)
	return w
}

func statUpdate(opcode byte, current, maximum int32) *wire.Writer {
	w := wire.Packet(opcode)
	w.D(current)
	w.D(maximum)
	return w
}

// friendList and blockList are empty until social is ported (PORTING.md 10).
func friendList() *wire.Writer {
	w := wire.Packet(smFriendList)
	w.H(0)
	w.C(0)
	return w
}

func blockList() *wire.Writer {
	w := wire.Packet(smBlockList)
	w.H(0)
	w.C(0)
	return w
}

// Creature states and visual states.
const (
	visualBlinking  = 64 // protected, just after entering the world
	speedScale      = 1000
	playerInfoScale = 0.25
)

// playerInfo is SM_PLAYER_INFO: how a player looks and where it is.
func (s *Server) playerInfo(p *player, enemy bool) *wire.Writer {
	race, gender := raceGender(p.Character)
	template := 100000 + race*2 + gender
	if p.transformed != 0 {
		template = p.transformed
	}
	a := p.appearance
	w := wire.Packet(smPlayerInfo)
	w.F(p.X)
	w.F(p.Y)
	w.F(p.Z)
	w.D(p.ID)
	w.D(template)
	w.D(template)
	if enemy {
		w.C(0)
	} else {
		w.C(0x26)
	}
	w.C(byte(race))
	w.C(byte(classIDs[p.Class]))
	w.C(byte(gender))
	w.H(p.state)
	w.B(make([]byte, 8))
	w.C(byte(p.Heading))
	w.S(p.Name)
	w.D(p.TitleID)
	w.C(0)
	w.H(0) // casting skill
	w.H(uint16(p.legionID))
	w.H(0)
	w.H(0) // emblem
	w.C(0xff)
	w.C(0)
	w.C(0)
	w.C(0)
	if p.legion != nil {
		w.S(p.legion.Name)
	} else {
		w.S("")
	}
	w.C(byte(100 * p.life.HP / max(p.stats.current(data.MaxHP), 1)))
	w.H(0) // dp
	w.C(0)
	var mask uint16
	var worn []*store.Item
	for _, item := range p.equipment {
		if item.Slot >= 1<<19 && item.Slot != data.SlotNone {
			continue // stigma slots
		}
		mask |= uint16(item.Slot)
		worn = append(worn, item)
	}
	w.H(mask)
	for _, item := range worn {
		if item.Slot < 0x7fff*2 {
			w.D(item.SkinID())
			w.D(item.Godstone)
			w.D(item.Color)
			w.H(0)
		}
	}
	w.D(a.SkinRGB)
	w.D(a.HairRGB)
	w.D(a.EyeRGB)
	w.D(a.LipRGB)
	for _, v := range []int32{a.Face, a.Hair, a.Deco, a.Tattoo} {
		w.C(byte(v))
	}
	w.C(5)
	for _, v := range []int32{a.FaceShape, a.Forehead, a.EyeHeight, a.EyeSpace, a.EyeWidth, a.EyeSize, a.EyeShape,
		a.EyeAngle, a.BrowHeight, a.BrowAngle, a.BrowShape, a.Nose, a.NoseBridge, a.NoseWidth, a.NoseTip, a.Cheek,
		a.LipHeight, a.MouthSize, a.LipSize, a.Smile, a.LipShape, a.JawHeight, a.ChinJut, a.EarShape, a.HeadSize,
		a.Neck, a.NeckLength, a.ShoulderSize, a.Torso, a.Chest, a.Waist, a.Hips, a.ArmThickness, a.HandSize,
		a.LegThickness, a.FootSize, a.FacialRate} {
		w.C(byte(v))
	}
	w.C(0)
	w.C(byte(a.ArmLength))
	w.C(byte(a.LegLength))
	w.C(byte(a.Shoulders))
	w.C(0)
	w.C(0)
	w.C(byte(a.Voice))
	w.F(a.Height)
	w.F(playerInfoScale)
	w.F(2)
	w.F(float32(p.stats.current(data.Speed)) / speedScale)
	w.H(uint16(p.stats.base(data.AttackSpeed)))
	w.H(uint16(p.stats.current(data.AttackSpeed)))
	w.C(2)
	w.S(p.storeMessage())
	w.F(0)
	w.F(0)
	w.F(0)
	w.F(p.X)
	w.F(p.Y)
	w.F(p.Z)
	w.C(0) // move type
	if p.usingFlyTeleport() {
		w.D(p.flightTeleportID)
		w.D(p.flightDistance)
	}
	w.C(p.visualState)
	w.S(p.Note)
	w.H(uint16(p.level))
	w.H(uint16(p.settings.Display))
	w.H(uint16(p.settings.Deny))
	w.H(uint16(p.abyss.Rank))
	w.H(0)
	w.D(0) // target
	w.C(0)
	return w
}

func raceGender(c *store.Character) (race, gender int32) {
	if c.Race == "ASMODIANS" {
		race = 1
	}
	if c.Gender == "FEMALE" {
		gender = 1
	}
	return race, gender
}

func playerState(p *player) *wire.Writer {
	w := wire.Packet(smPlayerState)
	w.D(p.ID)
	w.C(p.visualState)
	w.C(p.seeState)
	w.Bool(p.visualState == visualBlinking)
	return w
}

func weather(code byte) *wire.Writer {
	w := wire.Packet(smWeather)
	w.H(uint16(code))
	w.C(0)
	return w
}

// emptyMailbox is SM_MAIL_SERVICE's empty letter list.
// ponytail: mail comes with social (PORTING.md 10).
func emptyMailbox(p *player) *wire.Writer {
	w := wire.Packet(smMailService)
	w.C(2)
	w.D(p.ID)
	w.H(0)
	w.C(0)
	return w
}

func chatInit(token []byte) *wire.Writer {
	w := wire.Packet(smChatInit)
	w.D(int32(len(token)))
	w.B(token)
	return w
}

// javaHashOrder orders items as a java.util.HashMap keyed by their int keys
// iterates, when they were put in the given order: by bucket, the key's hash
// masked by a table that doubles whenever it is more than 3/4 full.
func javaHashOrder[T any](items []T, key func(T) int32) []T {
	size := int32(16)
	for int32(len(items)) > size*3/4 {
		size *= 2
	}
	bucket := func(k int32) int32 { return (k ^ int32(uint32(k)>>16)) & (size - 1) }
	sorted := slices.Clone(items)
	slices.SortStableFunc(sorted, func(a, b T) int { return cmp.Compare(bucket(key(a)), bucket(key(b))) })
	return sorted
}

// storeMessage is what the sign of the player's private store says, or nothing.
func (p *player) storeMessage() string {
	if p.store == nil {
		return ""
	}
	return p.store.message
}
