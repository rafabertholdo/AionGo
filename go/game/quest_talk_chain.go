package game

import (
	"slices"

	"aionlightning/game/data"
)

// talkStep is one NPC conversation in a Java "talk to A, then B, ..." handler
// whose only state is quest variable 0. Zero-valued fields are unused.
type talkStep struct {
	npc     int32
	vars    int32    // required value of variable 0
	page    uint16   // reply to dialog 25
	advance []uint16 // dialogs that raise variable 0 by one (reply: close window)
	give    int32    // quest item handed out on advance if the player has none
	turnIn  int32    // dialog 33 takes turnInCount of this item: window 10000, else 10001
	// turnInCount is the amount dialog 33 requires.
	turnInCount int64
	kinahDialog uint16 // dialog that charges kinahCost, gives `give`, advances; page kinahShort if short
	kinahCost   int64
	kinahShort  uint16
	toReward    bool // advance moves the quest to REWARD as well
	// A purification step: dialog 25 shows page if the player holds holyItem, else pageMissing;
	// 10255 shows 10002 and 1009 takes the item and moves the quest to REWARD.
	holyItem    int32
	pageMissing uint16
	// reward1009: dialog 1009 moves the quest to REWARD (taking holyItem and take, if set) and shows window 5.
	reward1009 bool
	// take is removed from the player when an advance dialog fires.
	take int32
	// A collection step: dialog 33 needs the quest's collect_items. If they are
	// held they are taken, give is handed out, the quest moves on (toReward keeps
	// the variable and moves to REWARD) and collectDone is shown; else collectMissing.
	collectMissing uint16
	collectDone    uint16
}

// talkChain describes a quest run by talkChainDialog. startPage replaces the
// default dialog-25 page at the start NPC; rewardPage, when set, is shown on
// any chain NPC once the quest is in REWARD and lets any of them hand out
// the reward (otherwise only script.EndNPC does).
type talkChain struct {
	startPage  uint16
	rewardPage uint16
	// endPage is shown when the player clicks script.EndNPC in REWARD.
	endPage uint16
	// special, when set, sees every dialog of a quest in START first and returns true if it consumed it.
	special func(c *conn, o *object, script *data.QuestScript, dialogID uint16) bool
	// startItem is handed out when the quest is accepted.
	startItem int32
	// startCount is how many startItem are handed out (default 1).
	startCount int64
	// startVar is variable 0 right after acceptance when Java's start dialog bumps it (default 0).
	startVar int32
	// rewardTake items are removed from the player when the reward is picked at the end NPC.
	rewardTake []int32
	// clickPages: a plain NPC click (dialog -1) shows the same page dialog 25 would.
	// Together with special, the NPCs must be listed in QuestCustomTalks (see talkChainClick).
	clickPages bool
	steps      []talkStep
}

// talkChains holds the ported handlers built on talkChainDialog.
var talkChains = map[int32]*talkChain{
	3200:  &suspiciousCallChain,
	3093:  &secretDumplingRecipeChain,
	3319:  &orderForGojirunerkChain,
	3326:  &shugoMenaceChain,
	3914:  &balaurReportChain,
	3930:  &shatteredStigmaChain,
	3931:  &howToUseStigmaChain,
	3932:  &stopTheShulacksChain,
	3933:  &classPreceptorConsentChain,
	3934:  &questForTemplarsChain,
	3935:  &shoulderTheBurdenChain,
	3936:  &decorationsOfSanctumChain,
	3938:  &wellRoundedChain,
	3939:  &persistenceAndLuckChain,
	3965:  &galleriaOfGrandeurChain,
	3966:  &saluteANewUniformChain,
	3967:  &andusDyeBoxChain,
	3968:  &palentinesRequestChain,
	3969:  &sexiestManAliveChain,
	4015:  &missingLaborersChain,
	4060:  &zombiesDescendantChain,
	4200:  &suspiciousCallChain,
	4934:  &shulacksStigmaChain,
	4935:  &bookletOnStigmaChain,
	4936:  &secretOfTheGreaterStigmaChain,
	4937:  &recognitionOfThePreceptorsChain,
	4938:  &workOfTheFenrisFangsChain,
	4939:  &provingGroundChain,
	4942:  &provingProficiencyChain,
	4943:  &luckAndPersistenceChain,
	19004: &periklessInsightChain,
}

func (c *conn) talkChainDialog(o *object, script *data.QuestScript, dialogID uint16, chain *talkChain) {
	if c.player == nil || o == nil || o.npc == nil {
		return
	}
	p := c.player
	q := p.quest(script.ID)
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		if dialogID == 25 && chain.startPage != 0 {
			if c.s.canStartQuest(p, script) {
				c.send(dialogWindow(o.id, chain.startPage, script.ID))
			}
			return
		}
		c.customQuestStartN(o, script, dialogID, chain.startItem, max(chain.startCount, 1))
		if q := p.quest(script.ID); dialogID == 1002 && chain.startVar != 0 && q != nil && q.Status == "START" && questVar(q.Vars, 0) == 0 {
			c.customQuestProgress(script.ID, setQuestVar(q.Vars, 0, chain.startVar), "")
		}
		return
	}
	if q == nil {
		return
	}
	if q.Status == "REWARD" {
		if o.npc.ID != script.EndNPC && chain.rewardPage == 0 {
			return
		}
		if o.npc.ID == script.EndNPC && dialogID == 1009 {
			for _, id := range chain.rewardTake {
				c.s.removeItemsByID(p, id, c.s.countItems(p, id))
			}
		}
		switch {
		case dialogID == 1009:
			c.send(dialogWindow(o.id, 5, script.ID))
		case dialogID >= 8 && dialogID <= 17:
			c.finishQuest(script, o.id, dialogID)
		}
		return
	}
	if q.Status != "START" {
		return
	}
	if chain.special != nil && chain.special(c, o, script, dialogID) {
		return
	}
	vars := questVar(q.Vars, 0)
	for i := range chain.steps {
		if step := &chain.steps[i]; step.npc == o.npc.ID && step.vars == vars {
			c.talkStep(o, script, step, vars, dialogID)
			return
		}
	}
}

func (c *conn) talkStep(o *object, script *data.QuestScript, step *talkStep, vars int32, dialogID uint16) {
	p := c.player
	next := setQuestVar(p.quest(script.ID).Vars, 0, vars+1)
	advance := func() {
		status := ""
		if step.toReward {
			status = "REWARD"
		}
		if c.customQuestProgress(script.ID, next, status) {
			c.send(dialogWindow(o.id, 10, 0))
		}
	}
	switch {
	case dialogID == 25:
		page := step.page
		if step.holyItem != 0 && c.s.countItems(p, step.holyItem) < 1 {
			page = step.pageMissing
		}
		c.send(dialogWindow(o.id, page, script.ID))
	case slices.Contains(step.advance, dialogID):
		if step.take != 0 && c.s.countItems(p, step.take) == 0 {
			return // Java only advances when the item can be taken
		}
		if step.give != 0 && c.s.countItems(p, step.give) == 0 {
			if !c.s.questRewardsFit(p, []data.QuestItem{{ID: step.give, Count: 1}}) {
				return
			}
			c.s.addItem(p, step.give, 1)
		}
		if step.take != 0 {
			c.s.removeItemsByID(p, step.take, c.s.countItems(p, step.take))
		}
		advance()
	case dialogID == 33 && step.collectMissing != 0:
		template := c.s.data.Quests[script.ID]
		if !c.s.hasQuestItems(p, template) {
			c.send(dialogWindow(o.id, step.collectMissing, script.ID))
			return
		}
		if step.give != 0 && !c.s.questRewardsFit(p, []data.QuestItem{{ID: step.give, Count: 1}}) {
			return
		}
		status, vars := "", next
		if step.toReward {
			status, vars = "REWARD", p.quest(script.ID).Vars
		}
		if c.customQuestProgress(script.ID, vars, status) {
			for _, item := range template.CollectItems {
				c.s.removeItemsByID(p, item.ID, item.Count)
			}
			if step.give != 0 {
				c.s.addItem(p, step.give, 1)
			}
			c.send(dialogWindow(o.id, step.collectDone, script.ID))
		}
	case dialogID == 33 && step.turnIn != 0:
		if c.s.countItems(p, step.turnIn) < step.turnInCount {
			c.send(dialogWindow(o.id, 10001, script.ID))
			return
		}
		if c.customQuestProgress(script.ID, next, "") {
			c.s.removeItemsByID(p, step.turnIn, c.s.countItems(p, step.turnIn))
			c.send(dialogWindow(o.id, 10000, script.ID))
		}
	case step.kinahDialog != 0 && dialogID == step.kinahDialog:
		if p.kinah.Count < step.kinahCost {
			c.send(dialogWindow(o.id, step.kinahShort, script.ID))
			return
		}
		if step.give != 0 && c.s.countItems(p, step.give) == 0 && !c.s.questRewardsFit(p, []data.QuestItem{{ID: step.give, Count: 1}}) {
			return // Java would take the kinah first and lose it; refuse instead
		}
		if c.customQuestProgress(script.ID, next, "") {
			c.s.decreaseKinah(p, step.kinahCost)
			if step.give != 0 && c.s.countItems(p, step.give) == 0 {
				c.s.addItem(p, step.give, 1)
			}
			c.send(dialogWindow(o.id, 10, 0))
		}
	case step.holyItem != 0 && dialogID == 10255:
		c.send(dialogWindow(o.id, 10002, script.ID))
	case (step.holyItem != 0 || step.reward1009) && dialogID == 1009:
		if c.customQuestProgress(script.ID, p.quest(script.ID).Vars, "REWARD") {
			for _, id := range []int32{step.holyItem, step.take} {
				if id != 0 {
					c.s.removeItemsByID(p, id, c.s.countItems(p, id))
				}
			}
			c.send(dialogWindow(o.id, 5, script.ID))
		}
	}
}

// talkChainShowDialog answers a plain NPC click in REWARD for chains with a rewardPage.
func (c *conn) talkChainShowDialog(o *object, script *data.QuestScript, chain *talkChain) bool {
	q := c.player.quest(script.ID)
	if q == nil || q.Status != "REWARD" {
		return false
	}
	page := chain.rewardPage
	if page == 0 && o.npc.ID == script.EndNPC {
		page = chain.endPage
	}
	if page == 0 {
		return false
	}
	c.send(dialogWindow(o.id, page, script.ID))
	return true
}

// talkChainClick answers a plain click (dialog -1) on an NPC listed in
// QuestCustomTalks: the chain's special hook first, then, with clickPages, the
// page dialog 25 would show. It reports whether a window was sent.
func (c *conn) talkChainClick(o *object, script *data.QuestScript, chain *talkChain) bool {
	p := c.player
	q := p.quest(script.ID)
	if q != nil && q.Status == "START" && chain.special != nil && chain.special(c, o, script, ^uint16(0)) {
		return true
	}
	if !chain.clickPages {
		return false
	}
	if o.npc.ID == script.StartNPC && (q == nil || q.Status == "NONE" || q.Status == "COMPLETE") {
		if !c.s.canStartQuest(p, script) {
			return false
		}
		c.send(dialogWindow(o.id, max(chain.startPage, 1011), script.ID))
		return true
	}
	if q == nil || q.Status != "START" {
		return false
	}
	vars := questVar(q.Vars, 0)
	for i := range chain.steps {
		if step := &chain.steps[i]; step.npc == o.npc.ID && step.vars == vars {
			c.send(dialogWindow(o.id, step.page, script.ID))
			return true
		}
	}
	return false
}
