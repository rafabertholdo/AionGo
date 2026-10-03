package game

import (
	"aionlightning/game/data"
	"aionlightning/wire"
)

// equipmentBytes is the room PlayerInfo gives the equipment shown on the select screen.
const equipmentBytes = 208

// writeCharacterInfo is PlayerInfo.writePlayerInfo: a character on the select
// screen, with its appearance and visible equipment.
func (s *Server) writeCharacterInfo(w *wire.Writer, ch *character) {
	race := int32(0)
	if ch.Race == "ASMODIANS" {
		race = 1
	}
	gender := int32(0)
	if ch.Gender == "FEMALE" {
		gender = 1
	}
	a := ch.appearance
	w.D(ch.ID)
	w.S(ch.Name)
	w.B(make([]byte, 44-(len([]rune(ch.Name))*2+2)))
	w.D(gender)
	w.D(race)
	w.D(classIDs[ch.Class])
	w.D(a.Voice)
	w.D(a.SkinRGB)
	w.D(a.HairRGB)
	w.D(a.EyeRGB)
	w.D(a.LipRGB)
	for _, v := range []int32{a.Face, a.Hair, a.Deco, a.Tattoo} {
		w.C(byte(v))
	}
	w.C(4)
	for _, v := range []int32{a.FaceShape, a.Forehead, a.EyeHeight, a.EyeSpace, a.EyeWidth, a.EyeSize, a.EyeShape,
		a.EyeAngle, a.BrowHeight, a.BrowAngle, a.BrowShape, a.Nose, a.NoseBridge, a.NoseWidth, a.NoseTip, a.Cheek,
		a.LipHeight, a.MouthSize, a.LipSize, a.Smile, a.LipShape, a.JawHeight, a.ChinJut, a.EarShape, a.HeadSize,
		a.Neck, a.NeckLength, a.ShoulderSize, a.Torso, a.Chest, a.Waist, a.Hips, a.ArmThickness, a.HandSize,
		a.LegThickness, a.FootSize, a.FacialRate} {
		w.C(byte(v))
	}
	w.C(0)
	w.C(byte(a.ArmLength))
	w.C(byte(a.LegLength))
	w.C(byte(a.Shoulders))
	w.C(0)
	w.C(0)
	w.F(a.Height)
	w.D(100000 + race*2 + gender)
	w.D(ch.WorldID)
	w.F(ch.X)
	w.F(ch.Y)
	w.F(ch.Z)
	w.D(ch.Heading)
	w.D(int32(s.data.Level(ch.Exp)))
	w.D(ch.TitleID)
	w.D(ch.legionID)
	w.B(make([]byte, 4*18)) // legion name slot and unknowns
	used := 0
	for _, item := range ch.equipment {
		template := s.data.Items[item.ItemID]
		if template == nil || used >= equipmentBytes || template.Slot > data.SlotPants {
			continue
		}
		w.C(1)
		w.D(item.SkinID())
		w.D(item.Godstone)
		w.D(item.Color)
		used += 13
	}
	w.B(make([]byte, equipmentBytes-used))
	w.D(ch.deletionSeconds())
	w.D(0)
}
