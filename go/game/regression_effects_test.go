package game

import (
	"encoding/binary"
	"testing"
	"testing/synctest"
	"time"

	"aionlightning/game/store"
)

func TestReturnSkillMovesToBindAfterCast(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, tap := fighter(t, s, 1000)
		p.appearance = &store.Appearance{}
		p.skills = append(p.skills, store.Skill{ID: 1801, Level: 1})
		s.spawn(p)
		at := s.bindLocation(p)
		s.playerUseSkill(p, d.Skills[1801], 0, 0, 0, 0)
		if p.cast == nil {
			t.Fatal("Return cast did not start")
		}
		time.Sleep(13 * time.Second)
		synctest.Wait()
		if p.WorldID != at.MapID || p.X != at.X || p.Y != at.Y || p.Z != at.Z {
			t.Fatalf("Return did not move to bind: %d %v %v %v", p.WorldID, p.X, p.Y, p.Z)
		}
		if tap.count(smCastspellEnd) != 1 {
			t.Fatal("Return did not finish casting")
		}
	})
}

func TestGreaterRunningScrollOwnerIconAndExpiry(t *testing.T) {
	d := staticDataOrSkip(t)
	synctest.Test(t, func(t *testing.T) {
		s := testServer(d)
		p, _ := fighter(t, s, 1000)
		packets := &questPackets{}
		p.conn.tap = packets.tap
		s.spawn(p)
		item := &store.Item{UniqueID: 0x40000, ItemID: 164000076, Owner: p.ID, Count: 2}
		p.cube = []*store.Item{item}
		s.useItem(p, item, nil)
		s.flushEffects()
		got := packets.last(smAbnormalState)
		if len(got) != 19 || binary.LittleEndian.Uint16(got[5:7]) != 1 || binary.LittleEndian.Uint16(got[11:13]) != 8845 || got[13] != 3 || binary.LittleEndian.Uint32(got[15:19]) != 300000 {
			t.Fatalf("missing scroll icon and five minute countdown: %x", got)
		}
		if item.Count != 1 {
			t.Fatal("scroll was not consumed")
		}
		time.Sleep(301 * time.Second)
		synctest.Wait()
		s.flushEffects()
		got = packets.last(smAbnormalState)
		if len(got) != 7 || binary.LittleEndian.Uint16(got[5:7]) != 0 {
			t.Fatalf("expired scroll icon remained: %x", got)
		}
	})
}
