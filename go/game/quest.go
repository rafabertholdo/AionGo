package game

import (
	"aionlightning/game/store"
	"aionlightning/wire"
)

func init() {
	handlers[cmPlayMovieEnd] = (*conn).playMovieEnd
	handlers[cmDeleteQuest] = (*conn).deleteQuest
	handlers[cmShowDialog] = (*conn).showDialog
	handlers[cmDialogSelect] = (*conn).dialogSelect
	handlers[cmCloseDialog] = (*conn).closeDialog
}

// questSaver writes player quest transitions; store.Store implements it.
type questSaver interface {
	SaveQuest(playerID int32, quest store.Quest) error
	CompleteQuest(playerID int32, quest store.Quest, reward store.QuestCompletion) error
}

// prologueForRace is the opening quest and movie registered by the 1.9 quest scripts.
func prologueForRace(race string) (questID int32, movieID uint16) {
	switch race {
	case "ELYOS":
		return 1000, 1
	case "ASMODIANS":
		return 2000, 2
	default:
		return 0, 0
	}
}

func (p *player) quest(id int32) *store.Quest {
	for index := range p.quests {
		if p.quests[index].ID == id {
			return &p.quests[index]
		}
	}
	return nil
}

// startPrologue ports the race's onEnterWorldEvent: start once, then replay the
// opening movie after each login until the client reports that it ended.
func (c *conn) startPrologue() {
	p := c.player
	if p == nil {
		return
	}
	questID, movieID := prologueForRace(p.Race)
	if questID == 0 {
		return
	}
	template := c.s.data.Quests[questID]
	if template == nil || template.Race != p.Race || p.level < template.MinLevel {
		return
	}
	q := p.quest(questID)
	if q == nil {
		started := store.Quest{ID: questID, Status: "START"}
		if err := c.s.quests.SaveQuest(p.ID, started); err != nil {
			c.s.log.Error("starting prologue", "character", p.Name, "quest", questID, "err", err)
			return
		}
		p.quests = append(p.quests, started)
		c.send(questAccepted(1, started))
		c.send(c.s.nearbyQuests(p))
		q = p.quest(questID)
	}
	if q.Status == "START" {
		c.send(playMovie(movieID))
	}
}

// playMovieEnd is CM_PLAY_MOVIE_END. The Java client includes the full movie
// descriptor in its reply; the handler uses the id to finish the matching quest.
func (c *conn) playMovieEnd(r *wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	r.C() // movie type
	r.D()
	r.D()
	movieID := r.H()
	r.D()
	if r.Err != nil {
		return
	}
	if c.flyingReconnaissanceMovieEnd(movieID) {
		return
	}
	if c.gaphyrksLoveMovieEnd(movieID) {
		return
	}
	if c.ascensionMovieEnd(movieID) {
		return
	}
	if c.archonOfStormsMovieEnd(movieID) {
		return
	}
	if c.headlessStoneStatueMovieEnd(movieID) {
		return
	}
	questID, expectedMovie := prologueForRace(p.Race)
	if movieID != expectedMovie || questID == 0 {
		return
	}
	template := c.s.data.Quests[questID]
	if template == nil || len(template.Rewards) == 0 {
		return
	}
	q := p.quest(questID)
	if q == nil || q.Status != "START" {
		return
	}
	complete := *q
	complete.Status = "COMPLETE"
	complete.CompleteCount++
	if err := c.s.quests.SaveQuest(p.ID, complete); err != nil {
		c.s.log.Error("finishing prologue", "character", p.Name, "quest", questID, "err", err)
		return
	}
	*q = complete
	c.s.giveExp(p, template.Rewards[0].Experience)
	c.send(questAccepted(2, complete))
	c.send(c.s.nearbyQuests(p))
}

// deleteQuest is CM_DELETE_QUEST. A quest marked cannot_giveup in the static
// data stays in progress; other quests become NONE, as in QuestEngine.deleteQuest.
func (c *conn) deleteQuest(r *wire.Reader) {
	p := c.player
	if p == nil {
		return
	}
	questID := int32(r.H())
	if r.Err != nil {
		return
	}
	template := c.s.data.Quests[questID]
	q := p.quest(questID)
	if template == nil || template.CannotGiveUp || q == nil || q.Status == "NONE" {
		return
	}
	abandoned := *q
	abandoned.Status = "NONE"
	if err := c.s.quests.SaveQuest(p.ID, abandoned); err != nil {
		c.s.log.Error("abandoning quest", "character", p.Name, "quest", questID, "err", err)
		return
	}
	*q = abandoned
	c.abandonWorkOrder(questID)
	c.send(questAccepted(3, store.Quest{ID: questID}))
	c.send(c.s.nearbyQuests(p))
}

// questAccepted is SM_QUEST_ACCEPTED for starting or completing a quest.
func questAccepted(action byte, q store.Quest) *wire.Writer {
	w := wire.Packet(smQuestAccepted)
	w.C(action)
	w.D(q.ID)
	w.C(questStatus[q.Status])
	w.C(0)
	w.D(q.Vars)
	w.H(0)
	return w
}

// playMovie is SM_PLAY_MOVIE: a cutscene movie in the 1.9 client.
func playMovie(id uint16) *wire.Writer {
	return movie(1, id)
}

// movie is SM_PLAY_MOVIE(type, movieId) as the Java handlers send it (type 1: cutscene movie, 0: cutscene).
func movie(kind byte, id uint16) *wire.Writer {
	w := wire.Packet(smPlayMovie)
	w.C(kind)
	w.D(0)
	w.D(0)
	w.H(id)
	w.D(0)
	return w
}
