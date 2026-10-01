package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// workOrderDialog ports the WorkOrders template. These quests grant a temporary
// recipe and components, then consume the crafted goods at the same NPC.
func (c *conn) workOrderDialog(o *object, script *data.QuestScript, dialogID uint16) {
	if o.npc.ID != script.StartNPC {
		return
	}
	p := c.player
	q := p.quest(script.ID)
	if q == nil || q.Status == "NONE" || q.Status == "COMPLETE" {
		if !c.s.canStartQuest(p, script) {
			return
		}
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 4, script.ID))
		case 1002:
			c.startWorkOrder(script, o.id)
		}
		return
	}
	if q.Status != "START" {
		return
	}
	switch dialogID {
	case 25:
		c.send(dialogWindow(o.id, 5, script.ID))
	case 17:
		template := c.s.data.Quests[script.ID]
		if !c.s.hasQuestItems(p, template) {
			return
		}
		finished := *q
		finished.Status = "COMPLETE"
		finished.CompleteCount++
		if err := c.s.quests.SaveQuest(p.ID, finished); err != nil {
			c.s.log.Error("finishing work order", "quest", script.ID, "err", err)
			return
		}
		*q = finished
		c.s.cleanupWorkOrder(p, script)
		c.send(questAccepted(2, finished))
		c.send(dialogWindow(o.id, 0, 0))
		c.send(c.s.nearbyQuests(p))
	}
}

func (c *conn) startWorkOrder(script *data.QuestScript, objectID int32) {
	p := c.player
	template := c.s.data.Quests[script.ID]
	recipe := c.s.data.Recipes[script.RecipeID]
	if recipe == nil || template == nil || p.level < template.MinLevel || !c.s.canStartQuest(p, script) {
		return
	}
	if !c.s.questRewardsFit(p, script.Components) {
		c.send(systemMessage(msgInventoryFull))
		return
	}
	started := store.Quest{ID: script.ID, Status: "START"}
	q := p.quest(script.ID)
	if q != nil {
		started.CompleteCount = q.CompleteCount
	}
	if err := c.s.quests.SaveQuest(p.ID, started); err != nil {
		c.s.log.Error("starting work order", "quest", script.ID, "err", err)
		return
	}
	if q == nil {
		p.quests = append(p.quests, started)
	} else {
		*q = started
	}
	c.send(questAccepted(1, started))
	for _, component := range script.Components {
		c.s.addItem(p, component.ID, component.Count)
	}
	c.s.learnRecipe(p, recipe)
	c.send(dialogWindow(objectID, 0, 0))
	c.send(c.s.nearbyQuests(p))
}

func (s *Server) cleanupWorkOrder(p *player, script *data.QuestScript) {
	s.deleteRecipe(p, script.RecipeID)
	ids := map[int32]bool{}
	for _, component := range script.Components {
		ids[component.ID] = true
	}
	if template := s.data.Quests[script.ID]; template != nil {
		for _, item := range template.CollectItems {
			ids[item.ID] = true
		}
		for _, item := range template.QuestWorkItems {
			ids[item.ID] = true
		}
	}
	for id := range ids {
		count := s.countItems(p, id)
		if count > 0 {
			s.removeItemsByID(p, id, count)
		}
	}
}

// A work-order recipe is temporary, so deletion updates both persistence and
// the client's recipe list. The same cleanup runs on finish and abandonment.
func (c *conn) abandonWorkOrder(id int32) {
	script := c.s.data.QuestScripts[id]
	if script != nil && script.Kind == data.QuestWorkOrder {
		c.s.cleanupWorkOrder(c.player, script)
	}
}
