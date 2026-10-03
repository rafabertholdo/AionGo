package game

import (
	"slices"
	"sort"
	"strconv"

	"aionlightning/game/data"
	"aionlightning/game/store"
	"aionlightning/wire"
)

func (s *Server) canStartQuest(p *player, script *data.QuestScript) bool {
	template := s.data.Quests[script.ID]
	if template == nil || template.Race != "" && template.Race != p.Race || p.level < template.MinLevel-2 {
		return false
	}
	if len(template.ClassPermitted) != 0 && !slices.Contains(template.ClassPermitted, p.Class) {
		return false
	}
	if template.GenderPermitted != "" && template.GenderPermitted != p.Gender {
		return false
	}
	if template.CombineSkill != 0 {
		matched := false
		for _, skill := range p.skills {
			if skill.ID == template.CombineSkill && skill.Level >= template.CombineSkillPoint && skill.Level <= template.CombineSkillPoint+40 {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	for _, prerequisite := range template.FinishedQuestConditions {
		q := p.quest(prerequisite)
		if q == nil || q.Status != "COMPLETE" {
			return false
		}
	}
	if q := p.quest(script.ID); q != nil {
		return q.Status == "NONE" || q.Status == "COMPLETE" && q.CompleteCount <= int32(template.MaxRepeatCount)
	}
	return true
}

func (s *Server) nearbyQuests(p *player) *wire.Writer {
	ids := make([]int32, 0)
	if p != nil {
		seen := map[int32]bool{}
		for _, o := range p.seen {
			if o.npc == nil {
				continue
			}
			for _, script := range s.data.QuestStarts[o.npc.ID] {
				if !seen[script.ID] && s.canStartQuest(p, script) {
					ids = append(ids, script.ID)
					seen[script.ID] = true
				}
			}
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	w := wire.Packet(smNearbyQuests)
	w.D(int32(len(ids)))
	for _, id := range ids {
		w.H(uint16(id))
		if p.level < s.data.Quests[id].MinLevel {
			w.H(2)
		} else {
			w.H(0)
		}
	}
	return w
}

func dialogWindow(objectID int32, dialogID uint16, questID int32) *wire.Writer {
	w := wire.Packet(smDialogWindow)
	w.D(objectID)
	w.H(dialogID)
	w.D(questID)
	w.H(0)
	return w
}

func (c *conn) dialogNpc(id int32) *object {
	p := c.player
	if p == nil {
		return nil
	}
	o := p.seen[id]
	if o == nil || o.npc == nil || o.dead || distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 10 {
		return nil
	}
	return o
}

// closeDialog is CM_CLOSE_DIALOG: the npc stops looking at the player, and everyone is told.
func (c *conn) closeDialog(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if c.player == nil {
		return
	}
	o := c.player.seen[id]
	if o == nil || o.npc == nil {
		return
	}
	if o.targetID == c.player.ID {
		o.targetID = 0
	}
	o.broadcast(c.s.lookAt(o), true)
}

func (c *conn) showDialog(r *wire.Reader) {
	id := r.D()
	if r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if c.player == nil {
		return
	}
	o := c.player.seen[id]
	if o == nil || o.npc == nil || o.dead {
		return
	}
	// CM_SHOW_DIALOG: the npc turns to the player, then its controller's onDialogRequest asks the quest engine with
	// dialog id -1 (every handler on the npc in turn). NpcController answers an unanswered click with the main menu
	// (window 10), from which the client picks a quest and sends CM_DIALOG_SELECT 25; ActionitemController plays the
	// use animation only when a handler answered, and otherwise does nothing.
	o.targetID = c.player.ID
	o.broadcast(c.s.lookAt(o), true)
	answered := c.questTalk(o, -1, 0)
	if o.npc.Type == "USEITEM" {
		if answered {
			c.useActionObject(o)
		}
		return
	}
	if !answered {
		c.send(dialogWindow(id, 10, 0))
	}
}

func (c *conn) dialogSelect(r *wire.Reader) {
	id := r.D()
	dialogID := r.H()
	r.H() // unknown
	r.H() // last page
	questID := r.D()
	if r.Err != nil {
		return
	}
	c.s.visMu.Lock()
	defer c.s.visMu.Unlock()
	if id == 0 {
		script := c.s.data.QuestScripts[questID]
		if script == nil {
			return
		}
		if script.ID == 3060 {
			c.dialogResult(c.redJournalDialog(nil, script, int32(dialogID)))
		} else if script.ID == 1114 {
			c.nymphsGownDialog(nil, script, int32(dialogID))
		} else if script.ID == 2122 {
			c.ashesToAshesEvent(nil, nil, script, int32(dialogID))
		} else if script.ID == 1107 || script.ID == 2107 || script.ID == lostAxeQuestID || script.ID == 3914 || script.ItemUseDelay > 0 {
			c.itemStartedQuestDialog(script, dialogID)
		}
		return
	}
	o := c.dialogNpc(id)
	if o == nil {
		return
	}
	// NpcController.onDialogSelect: the quest engine first, then the npc's services, and any other dialog is answered
	// by the window of the same number, which is how the client pages through a quest's text.
	if !c.questTalk(o, int32(dialogID), questID) && !javaServiceDialog[dialogID] {
		c.send(dialogWindow(o.id, dialogID, questID))
	}
}

// javaServiceDialog lists the dialog ids NpcController.onDialogSelect answers itself, ahead of its default.
var javaServiceDialog = func() map[uint16]bool {
	m := map[uint16]bool{}
	for _, id := range []uint16{2, 3, 4, 5, 6, 7, 20, 27, 29, 30, 31, 35, 36, 37, 38, 39, 40, 41, 42, 47, 50, 52, 53, 60, 61} {
		m[id] = true
	}
	return m
}()

// questTalk is QuestEngine.onDialog for a talk with npc o: the quest's handler when the client names one, else every
// handler registered on the npc in turn. It says whether one answered (Java's true).
func (c *conn) questTalk(o *object, d, questID int32) bool {
	var scripts []*data.QuestScript
	if questID != 0 {
		if script := c.s.data.QuestScripts[questID]; script != nil {
			scripts = append(scripts, script)
		}
	} else {
		scripts = c.npcTalkScripts(o.npc.ID)
	}
	for _, script := range scripts {
		if c.answered(func() { c.questDialog(o, script, d) }) {
			return true
		}
	}
	return false
}

// answered runs a quest handler and says whether it replied, which is Java's onDialogEvent returning true.
func (c *conn) answered(handler func()) bool {
	before := c.dialogReplies.Load()
	c.dialogPassed.Store(false)
	handler()
	return c.dialogReplies.Load() != before && !c.dialogPassed.Load()
}

// npcTalkScripts are the handlers registered for an npc's talk event.
func (c *conn) npcTalkScripts(npcID int32) []*data.QuestScript {
	var scripts []*data.QuestScript
	seen := map[int32]bool{}
	var translated []*data.QuestScript
	for _, id := range javaTalkNPCs[npcID] {
		if script := c.s.data.QuestScripts[id]; script != nil {
			translated = append(translated, script)
		}
	}
	for _, list := range [][]*data.QuestScript{c.s.data.QuestCustomTalks[npcID], c.s.data.QuestXMLTalks[npcID], c.s.data.QuestStarts[npcID], c.s.data.QuestEnds[npcID], c.s.data.QuestActions[npcID], translated} {
		for _, script := range list {
			if !seen[script.ID] {
				seen[script.ID] = true
				scripts = append(scripts, script)
			}
		}
	}
	return scripts
}

// dialogNotHandled is a handler saying it returned false after all, though it already sent something (Java's
// 1001 plays a movie and returns false so that the dialog window is sent too).
func (c *conn) dialogNotHandled() { c.dialogPassed.Store(true) }

// dialogResult is a handler's boolean, Java's return value, for the ones ported as functions returning it: true is
// answered even when nothing was sent, false is not answered even when something was (a movie).
func (c *conn) dialogResult(handled bool) {
	if handled {
		c.dialogSilent()
	} else {
		c.dialogNotHandled()
	}
}

// dialogSilent is a handler returning true without sending anything.
func (c *conn) dialogSilent() { c.dialogReplies.Add(1) }

// questDialog is one quest handler's onDialogEvent; d is the dialog id, -1 for CM_SHOW_DIALOG.
func (c *conn) questDialog(o *object, script *data.QuestScript, d int32) {
	dialogID := uint16(d)
	if script.Kind == data.QuestWorkOrder {
		c.dialogResult(c.workOrderDialog(o, script, dialogID))
		return
	}
	if script.Kind == data.QuestCustom {
		if port := javaDialogs[script.ID]; port != nil {
			c.dialogResult(c.javaPort(func() bool { return port(c, o, script, d) }))
			return
		}
		if d == -1 {
			if script.ID == lostAxeQuestID && o.npc.ID == lostAxeActionNPCID {
				c.lostAxeAction(o, script, -1)
				return
			}
			// The click-specific ports first; any other handler sees dialog -1 like Java's onDialogEvent does.
			if c.answered(func() { c.dialogResult(c.customQuestShowDialog(o, script)) }) {
				c.dialogSilent()
				return
			}
			c.dialogPassed.Store(false)
		}
		if script.ID == 2122 {
			c.ashesToAshesEvent(o, nil, script, d)
			return
		}
		c.customQuestDialogID(o, script, d)
		return
	}
	switch script.Kind {
	case data.QuestXML:
		if c.xmlQuestDialog(o, script, d) {
			return
		}
		c.dialogResult(c.xmlQuestFallback(o, script, d))
	case data.QuestReportTo:
		c.dialogResult(c.reportToDialog(o, script, d))
	case data.QuestMonsterHunt:
		c.dialogResult(c.monsterHuntDialog(o, script, d))
	case data.QuestItemCollecting:
		c.dialogResult(c.itemCollectingDialog(o, script, d))
	}
}

func questVar(vars int32, id int) int32 {
	return (vars >> (id * 6)) & 0x3f
}

func setQuestVar(vars int32, id int, value int32) int32 {
	shift := id * 6
	return (vars &^ (0x3f << shift)) | ((value & 0x3f) << shift)
}

func monsterHuntComplete(q *store.Quest, script *data.QuestScript) bool {
	for _, monster := range script.MonsterInfos {
		if monster.VarID < 0 || monster.VarID > 4 || questVar(q.Vars, monster.VarID) < monster.MaxKill {
			return false
		}
	}
	return true
}

// startQuest is QuestService.startQuest followed by defaultQuestStartDialog's window 1003; false when the player may
// not start the quest (nothing is sent then).
func (c *conn) startQuest(script *data.QuestScript, objectID int32) bool {
	if !c.beginQuest(script) {
		return false
	}
	c.send(dialogWindow(objectID, 1003, script.ID))
	return true
}

// beginQuest is QuestService.startQuest(env, START): the start checks, then the quest update and the nearby quests.
func (c *conn) beginQuest(script *data.QuestScript) bool {
	p := c.player
	if !c.s.canStartQuest(p, script) || p.level < c.s.data.Quests[script.ID].MinLevel {
		return false
	}
	started := store.Quest{ID: script.ID, Status: "START"}
	q := p.quest(script.ID)
	if q != nil {
		started.CompleteCount = q.CompleteCount
	}
	if err := c.s.quests.SaveQuest(p.ID, started); err != nil {
		c.s.log.Error("starting quest", "quest", script.ID, "err", err)
		return false
	}
	if q == nil {
		p.quests = append(p.quests, started)
	} else {
		*q = started
	}
	c.send(questAccepted(1, started))
	c.send(c.s.nearbyQuests(p))
	return true
}

func (c *conn) readyQuestReward(q *store.Quest, objectID int32) bool {
	if q.Status != "START" {
		return false
	}
	reward := *q
	reward.Status = "REWARD"
	reward.Vars = (reward.Vars &^ 0x3f) | ((reward.Vars + 1) & 0x3f)
	if err := c.s.quests.SaveQuest(c.player.ID, reward); err != nil {
		c.s.log.Error("rewarding quest", "quest", q.ID, "err", err)
		return false
	}
	*q = reward
	c.send(questAccepted(2, reward))
	c.send(dialogWindow(objectID, 5, q.ID))
	return true
}

func (s *Server) hasQuestItems(p *player, template *data.QuestTemplate) bool {
	if template == nil {
		return false
	}
	for _, required := range template.CollectItems {
		var count int64
		for _, item := range p.cube {
			if item.ItemID == required.ID {
				count += item.Count
			}
		}
		if count < required.Count {
			return false
		}
	}
	return true
}

func (c *conn) finishQuest(script *data.QuestScript, objectID int32, dialogID uint16) {
	c.finishQuestReward(script, objectID, dialogID, 0)
}

func (c *conn) finishQuestReward(script *data.QuestScript, objectID int32, dialogID uint16, rewardIndex int) {
	if !c.questFinish(script, dialogID, rewardIndex) {
		return
	}
	c.send(dialogWindow(objectID, 10, 0))
	if script.ID == 1006 {
		c.ascensionQuestFinish()
	}
}

// questFinish is QuestService.questFinish(env, reward): false when the quest is not waiting for its reward (or the
// reward does not exist, where Java throws); with no room for the items only the full-inventory message is sent and
// it still returns true, as Java does.
func (c *conn) questFinish(script *data.QuestScript, dialogID uint16, rewardIndex int) bool {
	p := c.player
	q := p.quest(script.ID)
	template := c.s.data.Quests[script.ID]
	if q == nil || q.Status != "REWARD" || template == nil || rewardIndex < 0 || rewardIndex >= len(template.Rewards) {
		return false
	}
	reward := template.Rewards[rewardIndex]
	items := slices.Clone(reward.Items)
	if dialogID != 17 {
		var choices []data.QuestItem
		if template.UseClassReward {
			choices = template.ClassRewards(p.Class)
		} else {
			choices = reward.SelectableItems
		}
		index := int(dialogID) - 8
		if index < 0 || index >= len(choices) {
			return false
		}
		items = append(items, choices[index])
	}
	if !c.s.questRewardsFit(p, items) {
		c.send(systemMessage(msgInventoryFull))
		return true
	}
	complete := *q
	complete.Status = "COMPLETE"
	complete.CompleteCount++
	completion := store.QuestCompletion{CubeSize: p.CubeSize, WarehouseSize: p.WarehouseSize}
	if reward.TitleID != 0 {
		if title := c.s.data.Titles[reward.TitleID]; title != nil && title.Race == raceID(p.Race) {
			completion.TitleID = reward.TitleID
		}
	}
	if reward.AbyssPoints > 0 {
		abyss := *p.abyss
		abyss.AP += reward.AbyssPoints
		abyss.DailyAP += reward.AbyssPoints
		abyss.WeeklyAP += reward.AbyssPoints
		for rank, required := range abyssRankAP {
			if abyss.AP >= required {
				abyss.Rank = int32(rank + 1)
			}
		}
		abyss.MaxRank = max(abyss.MaxRank, abyss.Rank)
		completion.Abyss = &abyss
	}
	if reward.ExtendInventory == 1 && completion.CubeSize < 9 {
		completion.CubeSize++
	} else if reward.ExtendInventory == 2 {
		completion.WarehouseSize++
	}
	if err := c.s.quests.CompleteQuest(p.ID, complete, completion); err != nil {
		c.s.log.Error("finishing quest", "quest", script.ID, "err", err)
		return false
	}
	c.javaQuestFinish() // QuestEngine.onQuestFinish, while the quest is still REWARD in memory
	*q = complete
	// QuestService.questFinish sends in this order: the items, kinah and experience, then the title, abyss points and
	// cube, the finished quest, the nearby quests, the quests the level unlocks, and last the main menu.
	var afterRewards []*wire.Writer
	if completion.TitleID != 0 && !slices.Contains(p.titles, completion.TitleID) {
		p.titles = append(p.titles, completion.TitleID)
		afterRewards = append(afterRewards, titleList(p))
	}
	if completion.Abyss != nil {
		*p.abyss = *completion.Abyss
		// PlayerCommonData.addAp: the "earned abyss points" message, then the rank.
		afterRewards = append(afterRewards, systemMessage(msgEarnedAP, strconv.Itoa(int(reward.AbyssPoints))), abyssRank(p.abyss))
	}
	// CubeExpandService.expand / WarehouseService.expand: the "slots added" message, then the new size.
	if p.CubeSize != completion.CubeSize {
		p.CubeSize = completion.CubeSize
		afterRewards = append(afterRewards, systemMessage(msgCubeExpanded, "9"), cubeSizeUpdate(p))
	}
	warehouseExpanded := p.WarehouseSize != completion.WarehouseSize
	p.WarehouseSize = completion.WarehouseSize
	for _, item := range items {
		c.s.addItem(p, item.ID, c.s.javaItemCount(item))
	}
	if reward.Kinah > 0 {
		c.s.increaseKinah(p, reward.Kinah)
	}
	if reward.Experience > 0 {
		c.s.giveExp(p, reward.Experience)
	}
	for _, w := range afterRewards {
		c.send(w)
	}
	if warehouseExpanded {
		c.send(systemMessage(msgWarehouseExpanded, "8"))
		c.s.sendWarehouseInfo(p, false)
	}
	c.send(questAccepted(2, complete))
	c.send(c.s.nearbyQuests(p))
	c.questLevelUp()
	return true
}

func raceID(race string) int32 {
	if race == "ASMODIANS" {
		return 1
	}
	return 0
}

func (s *Server) questRewardsFit(p *player, items []data.QuestItem) bool {
	needed := map[int32]int64{}
	for _, item := range items {
		if item.ID != data.Kinah {
			needed[item.ID] += item.Count
		}
	}
	for _, existing := range p.cube {
		if existing == nil || needed[existing.ItemID] <= 0 {
			continue
		}
		template := s.data.Items[existing.ItemID]
		if template != nil {
			needed[existing.ItemID] -= max(0, int64(template.MaxStack)-existing.Count)
		}
	}
	free := p.cubeLimit() - len(p.cube)
	for id, count := range needed {
		if count <= 0 {
			continue
		}
		template := s.data.Items[id]
		if template == nil || template.MaxStack <= 0 {
			return false
		}
		free -= int((count + int64(template.MaxStack) - 1) / int64(template.MaxStack))
	}
	return free >= 0
}
