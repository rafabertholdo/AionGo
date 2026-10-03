package game

import (
	"bytes"
	"encoding/binary"
	"slices"
	"testing"

	"aionlightning/wire"
)

func allianceGroupFixture(t *testing.T, count int) (*Server, *alliance, [][][]byte) {
	t.Helper()
	s := testServer(staticDataOrSkip(t))
	a := &alliance{id: 0x10001}
	packets := make([][][]byte, count)
	for i := range count {
		p, _ := fighter(t, s, 1000)
		p.ID = int32(0x20000 + i)
		p.Name = "Member"
		p.alliance = a
		p.conn.tap = func(w *wire.Writer) { packets[i] = append(packets[i], bytes.Clone(w.Data)) }
		a.members = append(a.members, p)
	}
	a.captain = a.members[0]
	return s, a, packets
}

func allianceGroupRequest(first, group, second int32) *wire.Reader {
	return packet(cmAllianceGroupChange, func(w *wire.Writer) {
		w.D(first)
		w.D(group)
		w.D(second)
	})
}

func TestAllianceGroupChangeMoveAndSwap(t *testing.T) {
	for _, mode := range []string{"captain_move", "vice_move", "swap", "same_group", "self_swap", "offline_recipient"} {
		t.Run(mode, func(t *testing.T) {
			s, a, packets := allianceGroupFixture(t, 8)
			before := slices.Clone(a.members)
			first, second := a.members[1], a.members[7]
			actor := a.captain
			group := int32(1003)
			wantSlots := []int32{1003}
			subjects := []*player{first}
			switch mode {
			case "vice_move":
				actor = a.members[2]
				a.vice = []*player{actor}
			case "swap":
				group = 0
				wantSlots = []int32{1001, 1000}
				subjects = append(subjects, second)
			case "same_group":
				group = 1000
				wantSlots = []int32{1000}
			case "self_swap":
				group, second = 0, first
				wantSlots = []int32{1000, 1000}
				subjects = append(subjects, first)
			case "offline_recipient":
				a.members[6].conn = nil
			}
			h := handlers[cmAllianceGroupChange]
			if h == nil || clientPacketStates[cmAllianceGroupChange] != inGame {
				t.Fatal("request is not registered for in-game clients")
			}
			h(actor.conn, allianceGroupRequest(first.ID, group, second.ID))
			if !slices.Equal(before, a.members) || a.captain != before[0] {
				t.Fatal("subgroup change altered roster order or captain")
			}
			for i, received := range packets {
				if a.members[i].conn == nil {
					if len(received) != 0 {
						t.Fatal("offline recipient received packets")
					}
					continue
				}
				if len(received) != 2*len(subjects) {
					t.Fatalf("member %d received %d packets", i, len(received))
				}
				for j, subject := range subjects {
					info := received[2*j]
					if info[0] != smAllianceMemberInfo || int32(binary.LittleEndian.Uint32(info[1:5])) != wantSlots[j] {
						t.Fatalf("member %d got wrong subgroup packet %x", i, info)
					}
					if !bytes.Equal(info, s.allianceMemberInfo(a, subject, 13).Data) || info[56] != 13 {
						t.Fatalf("member %d got wrong member info %x", i, info)
					}
					// Java SM_PLAYER_ID: H(2), D(0), H(1), D(id), H(0), S(name).
					wantID := []byte{0xab, 2, 0, 0, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0,
						'M', 0, 'e', 0, 'm', 0, 'b', 0, 'e', 0, 'r', 0, 0, 0}
					binary.LittleEndian.PutUint32(wantID[9:13], uint32(subject.ID))
					if !bytes.Equal(received[2*j+1], wantID) {
						t.Fatalf("member %d got wrong ID packet %x", i, received[2*j+1])
					}
				}
			}
			for _, subject := range subjects {
				// Later movement updates must retain the new assignment.
				info := s.allianceMemberInfo(a, subject, 1).Data
				if int32(binary.LittleEndian.Uint32(info[1:5])) != a.slot(subject) {
					t.Fatal("movement update lost subgroup assignment")
				}
			}
		})
	}
}

func TestAllianceGroupChangeRejectsInvalidRequests(t *testing.T) {
	for _, mode := range []string{"ordinary_member", "solo", "missing_first", "missing_second", "foreign_member",
		"negative_group", "fifth_group", "full_group", "truncated", "absent_player"} {
		t.Run(mode, func(t *testing.T) {
			s, a, packets := allianceGroupFixture(t, 8)
			actor := a.captain
			first, group, second := a.members[7].ID, int32(1003), a.members[1].ID
			wantMessages := 0
			switch mode {
			case "ordinary_member":
				actor = a.members[1]
				wantMessages = 1
			case "solo":
				actor.alliance = nil
				wantMessages = 1
			case "missing_first":
				first = -1
			case "missing_second":
				group, second = 0, -1
			case "foreign_member":
				foreign, _ := fighter(t, s, 1000)
				foreign.ID = 0x30000
				s.spawned[foreign.ID] = foreign
				first = foreign.ID
			case "negative_group":
				group = -1
			case "fifth_group":
				group = 1004
			case "full_group":
				group = 1000
			case "absent_player":
				actor.conn.player = nil
			}
			if mode == "truncated" {
				for size := range 12 {
					actor.conn.allianceGroupChange(wire.NewReader(make([]byte, size)))
				}
			} else {
				actor.conn.allianceGroupChange(allianceGroupRequest(first, group, second))
			}
			if a.slots != nil {
				t.Fatal("invalid request initialized or mutated subgroup state")
			}
			for i, received := range packets {
				want := 0
				if a.members[i] == actor {
					want = wantMessages
				}
				if len(received) != want {
					t.Fatalf("member %d received %d packets, want %d", i, len(received), want)
				}
				if want == 1 {
					text := "You do not have the authority for that."
					if mode == "solo" {
						text = "You are not in an alliance."
					}
					if !bytes.Equal(received[0], message(chatAnnouncement, text).Data) {
						t.Fatal("wrong authority rejection message")
					}
				}
			}
		})
	}
}

func TestAllianceGroupFullSwapAndCapacity(t *testing.T) {
	s, a, packets := allianceGroupFixture(t, maxAllianceSize)
	first, second := a.members[0], a.members[23]
	s.changeAllianceGroup(a, first.ID, 0, second.ID)
	if a.slot(first) != 1003 || a.slot(second) != 1000 {
		t.Fatal("swap between full subgroups failed")
	}
	for id := int32(1000); id <= 1003; id++ {
		if a.groupSize(id) != 6 {
			t.Fatalf("group %d has %d members", id, a.groupSize(id))
		}
	}
	joined, tap := fighter(t, s, 1000)
	joined.ID = 0x30000
	s.addToAlliance(a, joined)
	if len(a.members) != maxAllianceSize || joined.alliance != nil || tap.count(smAllianceInfo) != 0 {
		t.Fatal("full alliance accepted another member")
	}
	for i, received := range packets {
		if len(received) != 4 {
			t.Fatalf("member %d received %d packets, want four swap packets", i, len(received))
		}
	}
}

func TestAllianceGroupSlotsSurviveLeaveAndJoin(t *testing.T) {
	s, a, _ := allianceGroupFixture(t, 8)
	moved, leaving := a.members[7], a.members[1]
	s.changeAllianceGroup(a, moved.ID, 1003, 0)
	s.leaveAlliance(leaving, allianceLeaving)
	if a.slot(moved) != 1003 || a.slot(a.members[5]) != 1001 || a.slot(leaving) != 0 {
		t.Fatal("leaving shifted another member's subgroup or retained the departed member")
	}
	joined, _ := fighter(t, s, 1000)
	joined.ID = 0x30000
	s.addToAlliance(a, joined)
	if a.slot(joined) != 1000 || a.slot(moved) != 1003 {
		t.Fatal("joining did not fill the first available subgroup")
	}
}
