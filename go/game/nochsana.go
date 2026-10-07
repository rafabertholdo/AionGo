package game

import (
	"strings"
	"time"
)

const nochsanaWorld = 300030000

// isNochsanaGate is the fortress gate: a USEITEM template the 1.9 client
// must still be told is attackable.
func isNochsanaGate(o *object) bool {
	return o.worldID == nochsanaWorld && o.npc != nil && o.npc.ID == 256694
}

// siegeDoorBonus is the damage the siege pets' blows add against castle doors.
// The 1.9 XML omits it; the 4.6 skill_base gives 18008/17814 +19900 for the
// PC_LIGHT, PC_DARK and DRAGON castle door races (about a quarter of the NTC gate).
var siegeDoorBonus = map[int32]int32{17814: 19900, 18008: 19900}

func (e *effect) siegeDoorBonus() int32 {
	o, ok := e.effected.(*object)
	if !ok || o.npc == nil || !strings.HasSuffix(o.npc.Race, "_CASTLE_DOOR") {
		return 0
	}
	return siegeDoorBonus[e.tmpl.ID]
}

// nochsanaArtifact activates the shield declared for the camp's artifact.
func (c *conn) nochsanaArtifact(p *player, o *object) bool {
	if distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 5 || o.useTask != nil {
		return true
	}
	s := c.s
	tmpl := s.data.Skills[1872]
	if tmpl == nil {
		return true
	}
	c.send(useObject(p.ID, o.id, 1))
	p.broadcast(s.playerEmotionTo(p, emoteStartQuestLoot, 0, o.id, 0, 0, 0, 0), true)
	o.useTask = s.later(3*time.Second, func() {
		o.useTask = nil
		if p.conn != c || !p.spawned || p.dead || o.dead || p.seen[o.id] != o ||
			s.byID[o.id] != o || p.WorldID != nochsanaWorld || p.instance != o.instance ||
			distance3D(p.X, p.Y, p.Z, o.x, o.y, o.z) > 5 {
			return
		}
		c.send(useObject(p.ID, o.id, 0))
		p.broadcast(s.playerEmotionTo(p, emoteEndQuestLoot, 0, o.id, 0, 0, 0, 0), true)
		// The skill catalog describes an area shield but the generic area
		// selector also counts monsters. Keep its six nearby PC targets and
		// use the original spell effects and duration.
		targets := []creature{p}
		for _, other := range s.creaturesNear(o, 25) {
			ally, ok := other.(*player)
			if !ok || ally == p || ally.dead || len(targets) > 6 {
				continue
			}
			targets = append(targets, ally)
		}
		areaSkill := *tmpl
		areaSkill.SetProperties = nil
		(&skill{s: s, tmpl: &areaSkill, effector: o, first: p, targets: targets, level: 1}).use()
	})
	return true
}
