package game

import (
	"time"

	"aionlightning/game/data"
	"aionlightning/wire"
)

// recordQuestKill is MonsterHunt.onKillEvent for the player who earned the
// monster reward. QuestVars packs five six-bit counters.
func (s *Server) recordQuestKill(o *object, p *player) {
	for _, script := range s.data.QuestKills[o.npc.ID] {
		if script.ID == fungusAmongUsQuestID {
			p.conn.fungusAmongUsKill(o)
			continue
		}
		if script.ID == encroachersQuestID {
			p.conn.encroachersKill(o.npc.ID)
			continue
		}
		if script.ID == scoutItOutQuestID {
			p.conn.scoutItOutKill(o.npc.ID)
			continue
		}
		if script.ID == takeTheInitiativeQuestID {
			p.conn.takeTheInitiativeKill(o.npc.ID)
			continue
		}
		if script.ID == fearThisQuestID {
			p.conn.fearThisKill(o.npc.ID)
			continue
		}
		if script.ID == observatoryQuestID {
			p.conn.observatoryKill(o.npc.ID)
			continue
		}
		if script.ID == impetusiumQuestID {
			p.conn.impetusiumKill(o.npc.ID)
			continue
		}
		if script.ID == reducingTursinStrengthQuestID {
			p.conn.reducingTursinStrengthKill(o.npc.ID)
			continue
		}
		if script.ID == mandurisSecretQuestID {
			p.conn.mandurisSecretKill(o.npc.ID)
			continue
		}
		if script.ID == 1006 {
			p.conn.ascensionKill(o)
			continue
		}
		if script.ID == satalocasHeartQuestID {
			p.conn.satalocasHeartKill(o.npc.ID)
			continue
		}
		if script.ID == shadowsCommandQuestID {
			p.conn.shadowsCommandKill(o.npc.ID)
			continue
		}
		if script.ID == somethingInTheWaterQuestID {
			p.conn.somethingInTheWaterKill(o.npc.ID)
			continue
		}
		if script.ID == scoutingTheScoutsQuestID {
			p.conn.scoutingTheScoutsKill(o.npc.ID)
			continue
		}
		if script.ID == keeperKaidanKeyQuestID {
			p.conn.keeperKaidanKeyKill(o.npc.ID)
			continue
		}
		if script.ID == balaurConspiracyQuestID {
			p.conn.balaurConspiracyKill(o.npc.ID)
			continue
		}
		if script.ID == klawThreatQuestID {
			p.conn.klawThreatKill(o)
			continue
		}
		if script.ID == lepharistPoisonResearchQuestID {
			p.conn.lepharistPoisonResearchKill(o.npc.ID)
			continue
		}
		if script.ID == creatingMonsterQuestID {
			p.conn.creatingMonsterKill(o.npc.ID)
			continue
		}
		if script.ID == indratuLegionQuestID {
			p.conn.indratuLegionKill(o)
			continue
		}
		if script.ID == forestOutlawQuestID {
			p.conn.forestOutlawKill(o.npc.ID)
			continue
		}
		if script.ID == 1011 {
			p.conn.dangerFromAboveKill(o)
			continue
		}
		if script.ID == 1013 {
			p.conn.huntingLepharistRevolutionariesKill(o.npc.ID)
			continue
		}
		if script.ID == 1016 {
			p.conn.sourcePollutionKill(o)
			continue
		}
		if script.ID == 1019 {
			p.conn.flyingReconnaissanceKill(o.npc.ID)
			continue
		}
		if script.ID == 1021 {
			p.conn.trandilasEggsKill(o)
			continue
		}
		if script.ID == 1022 {
			p.conn.krallDesecrationKill(o.npc.ID)
			continue
		}
		if script.ID == frillneckHuntQuestID {
			p.conn.frillneckHuntKill(o.npc.ID)
			continue
		}
		if script.ID == 2114 {
			p.conn.insectProblemKill(o.npc.ID)
			continue
		}
		if script.ID == 1001 {
			p.conn.kerubThreatKill(o.npc.ID)
			continue
		}
		if script.ID == 2001 {
			p.conn.thinkingAheadEvent(o, script, 0, true)
			continue
		}
		if script.ID == 1003 {
			p.conn.illegalLoggingKill(o.npc.ID)
			continue
		}
		if script.ID == 2002 {
			p.conn.wheresRaeEvent(o, script, 0, true)
			continue
		}
		if script.ID == 2004 {
			p.conn.aCharmedCubeEvent(o, script, 0, true)
			continue
		}
		q := p.quest(script.ID)
		if q == nil || q.Status != "START" {
			continue
		}
		for _, monster := range script.MonsterInfos {
			if monster.NPCID != o.npc.ID || monster.VarID < 0 || monster.VarID > 4 {
				continue
			}
			current := questVar(q.Vars, monster.VarID)
			if current >= monster.MaxKill {
				continue
			}
			progress := *q
			progress.Vars = setQuestVar(progress.Vars, monster.VarID, current+1)
			if err := s.quests.SaveQuest(p.ID, progress); err != nil {
				s.log.Error("updating quest kill", "quest", script.ID, "err", err)
				continue
			}
			*q = progress
			p.conn.send(questAccepted(2, progress))
			break
		}
	}
}

func useObject(playerID, objectID int32, action byte) *wire.Writer {
	w := wire.Packet(smUseObject)
	w.D(playerID)
	w.D(objectID)
	w.D(3000)
	w.C(action)
	return w
}

// useQuestObject is ActionitemController.onDialogRequest for starter quest
// objects such as the Kerub grain sack. Its loot becomes available after the
// three-second use animation, and only to the player who used it.
func (c *conn) useQuestObject(o *object, script *data.QuestScript) {
	p := c.player
	if p == nil || o.useTask != nil || o.dead || p.quest(script.ID) == nil || p.quest(script.ID).Status != "START" {
		return
	}
	s := c.s
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.seen[o.id] != o || o.dead {
			return
		}
		q := p.quest(script.ID)
		if q == nil || q.Status != "START" {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		o.dead, o.hp = true, 0
		s.npcDied(o, p)
		s.registerDrop(o, p)
		s.openLoot(p, o.id)
	})
}
