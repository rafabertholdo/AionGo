package game

import (
	"maps"
	"slices"
	"time"

	"aionlightning/game/store"
	"aionlightning/wire"
)

// savedEffectMin is how much an effect or cooldown must have left for PlayerEffectsDAO to keep it.
const savedEffectMin = time.Minute

// savedEffects is PlayerEffectsDAO.storePlayerEffects' rows: the icon effects with a minute or more left,
// with their skill's cooldown, then the other cooldowns with a minute or more left.
//
// Java stores the time an effect has run and restores the template duration minus it, so a restored effect
// lengthens on every relog; Current here is the template duration minus what is left, which restores exactly.
func savedEffects(p *player, now time.Time) []store.SavedEffect {
	cooldowns := maps.Clone(p.cooldowns)
	var list []store.SavedEffect
	for _, e := range p.fx.list() {
		left := e.endTime.Sub(now)
		if e.duration == 0 || left < savedEffectMin {
			continue
		}
		current := max(e.tmpl.EffectsDuration()-int32(left/time.Millisecond), 1)
		list = append(list, store.SavedEffect{SkillID: e.tmpl.ID, Level: e.level, Current: current, Reuse: cooldowns[e.tmpl.ID]})
		delete(cooldowns, e.tmpl.ID)
	}
	for _, id := range slices.Sorted(maps.Keys(cooldowns)) {
		if until := cooldowns[id]; until.Sub(now) >= savedEffectMin {
			list = append(list, store.SavedEffect{SkillID: id, Reuse: until})
		}
	}
	return list
}

// restoreEffects is PlayerEffectsDAO.loadPlayerEffects and PlayerEffectController.addSavedEffect:
// the cooldowns not yet over come back, and the effects resume for what was left of them.
func (s *Server) restoreEffects(p *player, list []store.SavedEffect, now time.Time) {
	for _, saved := range list {
		if saved.Reuse.After(now) {
			p.setCooldown(saved.SkillID, saved.Reuse)
		}
		tmpl := s.data.Skills[saved.SkillID]
		if saved.Current <= 0 || tmpl == nil {
			continue
		}
		left := tmpl.EffectsDuration() - saved.Current
		if left <= 0 {
			continue
		}
		e := s.newEffect(p, p, tmpl, saved.Level, left)
		e.success = slices.Clone(e.templates)
		e.added = true
		if old := p.fx.abnormal[tmpl.Stack]; old != nil {
			old.end()
		}
		p.fx.abnormal[tmpl.Stack] = e
		e.start(true)
	}
	if len(p.fx.abnormal) > 0 {
		s.broadcastEffects(p)
	}
}

// stopEffects ends the effects' timers when the player leaves the world: the player is discarded,
// so its effects end without telling a client that is going away.
func (c *effectController) stopEffects() {
	for _, m := range []map[string]*effect{c.abnormal, c.noshow} {
		for _, e := range m {
			e.stopped = true
			e.task.cancel()
			for _, t := range e.periodic {
				t.cancel()
			}
		}
	}
}

// skillCooldowns is SM_SKILL_COOLDOWN: the seconds left of each skill cooling down.
func skillCooldowns(cooldowns map[int32]time.Time, now time.Time) *wire.Writer {
	w := wire.Packet(smSkillCooldown)
	w.H(uint16(len(cooldowns)))
	for _, id := range slices.Sorted(maps.Keys(cooldowns)) {
		w.H(uint16(id))
		w.D(max(int32(cooldowns[id].Sub(now)/time.Second), 0))
	}
	return w
}
