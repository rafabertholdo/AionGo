package game

import (
	"aionlightning/game/data"
	"aionlightning/game/store"
)

// workOrderDialog ports the WorkOrders template. These quests grant a temporary
// recipe and components, then consume the crafted goods at the same NPC.
// workOrderDialog is template/WorkOrders.onDialogEvent.
func (c *conn) workOrderDialog(o *object, script *data.QuestScript, dialogID uint16) bool {
	if o.npc.ID != script.StartNPC {
		return false
	}
	p := c.player
	q := p.quest(script.ID)
	switch {
	case q == nil || q.Status == "NONE" || q.Status == "COMPLETE":
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 4, script.ID))
			return true
		case 1002:
			if c.beginQuest(script) {
				if c.addQuestItems(script.Components) {
					if recipe := c.s.data.Recipes[script.RecipeID]; recipe != nil {
						c.s.learnRecipe(p, recipe)
					}
					c.send(dialogWindow(o.id, 0, 0))
				}
				return true
			}
		}
	case q.Status == "START":
		switch dialogID {
		case 25:
			c.send(dialogWindow(o.id, 5, script.ID))
			return true
		case 17:
			if c.collectQuestItems(script.ID) {
				c.s.cleanupWorkOrder(p, script)
				c.updateQuest(script.ID, func(q *store.Quest) { q.Status, q.CompleteCount = "COMPLETE", q.CompleteCount+1 })
				c.send(dialogWindow(o.id, 0, 0))
				return true
			}
		}
	}
	return false
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
