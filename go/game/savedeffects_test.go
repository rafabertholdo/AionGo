package game

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/store"
)

func TestSavedEffectsRestoreWhatWasLeft(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		s.spawn(p)
		item := &store.Item{UniqueID: 0x40000, ItemID: 164000076, Owner: p.ID, Count: 1}
		p.cube = []*store.Item{item}
		s.useItem(p, item, nil) // a greater running scroll: skill 8845 for five minutes
		time.Sleep(100 * time.Second)
		now := time.Now()
		p.setCooldown(1801, now.Add(2*time.Minute))
		p.setCooldown(1802, now.Add(30*time.Second))
		p.setCooldown(8845, now.Add(3*time.Minute))

		list := savedEffects(p, now)
		want := []store.SavedEffect{
			{SkillID: 8845, Level: 3, Current: 100000, Reuse: now.Add(3 * time.Minute)},
			{SkillID: 1801, Reuse: now.Add(2 * time.Minute)},
		}
		if len(list) != len(want) {
			t.Fatalf("saved %+v, want %+v", list, want)
		}
		for i := range want {
			if list[i].SkillID != want[i].SkillID || list[i].Level != want[i].Level || list[i].Current != want[i].Current || !list[i].Reuse.Equal(want[i].Reuse) {
				t.Fatalf("saved %+v, want %+v", list, want)
			}
		}
		p.fx.stopEffects()

		q, _ := fighter(t, s, 1000)
		q.ID = 0x20001
		s.restoreEffects(q, list, now)
		s.flushEffects()
		e := q.fx.abnormal[d.Skills[8845].Stack]
		if e == nil || e.elapsed() != 200000 {
			t.Fatalf("restored effect %v, want 200 s left", e)
		}
		if !q.skillDisabled(1801) || !q.skillDisabled(8845) || q.skillDisabled(1802) {
			t.Fatalf("restored cooldowns %v", q.cooldowns)
		}
		time.Sleep(201 * time.Second)
		synctest.Wait()
		if len(q.fx.abnormal) != 0 {
			t.Fatal("restored effect outlived what was left of it")
		}
		if again := savedEffects(q, time.Now()); len(again) != 0 {
			t.Fatalf("expired effect and cooldowns saved again: %+v", again)
		}
	})
}

func TestSavedEffectsSkipShortAndUnknown(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		now := time.Now()
		full := d.Skills[8845].EffectsDuration()
		s.restoreEffects(p, []store.SavedEffect{
			{SkillID: 8845, Level: 1, Current: full},      // already over
			{SkillID: 8845, Level: 1, Current: 0},         // a cooldown-only row
			{SkillID: 0x7fff, Level: 1, Current: 1},       // no such skill
			{SkillID: 1801, Reuse: now.Add(-time.Second)}, // cooldown over
		}, now)
		if len(p.fx.abnormal) != 0 || p.skillDisabled(1801) {
			t.Fatal("restored an effect or cooldown that was over")
		}
		s.restoreEffects(p, []store.SavedEffect{{SkillID: 8845, Level: 1, Current: full - 50000}}, now)
		if list := savedEffects(p, now); len(list) != 0 {
			t.Fatalf("saved an effect with under a minute left: %+v", list)
		}
	})
}

func TestSkillCooldownPacket(t *testing.T) {
	now := time.Unix(1000, 0)
	got := skillCooldowns(map[int32]time.Time{1802: now.Add(-time.Second), 1801: now.Add(90*time.Second + 900*time.Millisecond)}, now)
	want := []byte{smSkillCooldown, 2, 0, 0x09, 0x07, 90, 0, 0, 0, 0x0a, 0x07, 0, 0, 0, 0}
	if !bytes.Equal(got.Data, want) {
		t.Fatalf("SM_SKILL_COOLDOWN %x, want %x", got.Data, want)
	}
}

func TestItemCooldownsSaveAndPacket(t *testing.T) {
	now := time.Unix(1000, 0)
	m := map[int32]store.ItemCooldown{
		7: {UseDelay: 60000, Reuse: now.Add(30 * time.Second)},
		3: {UseDelay: 600000, Reuse: now.Add(90*time.Second + 900*time.Millisecond)},
		9: {UseDelay: 10000, Reuse: now.Add(29 * time.Second)},
	}
	saved := savedItemCooldowns(m, now)
	if len(saved) != 2 || saved[9] != (store.ItemCooldown{}) || len(m) != 3 {
		t.Fatalf("saved %+v from %+v, want groups 3 and 7", saved, m)
	}
	got := itemCooldowns(saved, now)
	want := []byte{smItemCooldown, 2, 0, 3, 0, 90, 0, 0, 0, 0xc0, 0x27, 0x09, 0, 7, 0, 30, 0, 0, 0, 0x60, 0xea, 0, 0}
	if !bytes.Equal(got.Data, want) {
		t.Fatalf("SM_ITEM_COOLDOWN %x, want %x", got.Data, want)
	}
}
