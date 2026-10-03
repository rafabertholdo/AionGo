package data

import (
	"encoding/xml"
	"fmt"
	"path/filepath"
	"sort"
)

// QuestScript is a handler declaration from quest_script_data/*.xml or one
// explicitly ported Java quest handler.
type QuestScript struct {
	ID             int32          `xml:"id,attr"`
	Kind           string         `xml:"-"`
	NPCStart       bool           `xml:"-"` // custom handlers that offer a quest from an NPC
	LevelUpStart   bool           `xml:"-"` // Java addQuestLvlUp handlers that create the quest on level-up
	LevelUpNPC     int32          `xml:"-"` // NPC that runs the level-up quest's first dialog stage
	LevelUpWorld   int32          `xml:"-"` // destination world after the first dialog stage
	LevelUpX       float32        `xml:"-"`
	LevelUpY       float32        `xml:"-"`
	LevelUpZ       float32        `xml:"-"`
	LevelUpDelayMS int32          `xml:"-"`
	ItemUseDelay   int32          `xml:"-"` // delayed item-start animation in milliseconds
	MiddleNPC      int32          `xml:"-"` // first report NPC in three-NPC custom chains
	MiddleNPC2     int32          `xml:"-"` // optional second report NPC
	MiddleNPC3     int32          `xml:"-"` // optional third report NPC
	TalkNPCs       []int32        `xml:"-"` // extra NPCs of a talk-chain handler (see game/quest_talk_chain.go)
	FinalNPC       int32          `xml:"-"` // optional final NPC; zero means StartNPC
	StartNPC       int32          `xml:"start_npc_id,attr"`
	EndNPC         int32          `xml:"end_npc_id,attr"`
	ActionNPC      int32          `xml:"action_item_id,attr"`
	ItemID         int32          `xml:"item_id,attr"`
	RecipeID       int32          `xml:"recipe_id,attr"`
	Components     []QuestItem    `xml:"give_component"`
	TalkEvents     []QuestXMLTalk `xml:"on_talk_event"`
	MonsterInfos   []QuestMonster `xml:"monster_infos"`
}

type QuestMonster struct {
	NPCID   int32 `xml:"npc_id,attr"`
	VarID   int   `xml:"var_id,attr"`
	MaxKill int32 `xml:"max_kill,attr"`
}

// QuestXMLTalk is the small XML event language used by the five xml_quest
// declarations. The operations remain ordered, as Java executes them in order.
type QuestXMLTalk struct {
	Conditions struct {
		QuestStatus struct {
			Value string `xml:"value,attr"`
		} `xml:"quest_status"`
	} `xml:"conditions"`
	Vars []QuestXMLVar `xml:"var"`
}

type QuestXMLVar struct {
	Value int32         `xml:"value,attr"`
	NPCs  []QuestXMLNPC `xml:"npc"`
}

type QuestXMLNPC struct {
	ID      int32            `xml:"id,attr"`
	Dialogs []QuestXMLDialog `xml:"dialog"`
}

type QuestXMLDialog struct {
	ID         int32              `xml:"id,attr"`
	Operations QuestXMLOperations `xml:"operations"`
}

type QuestXMLOperations struct {
	Override *bool            `xml:"override,attr"`
	Actions  []QuestXMLAction `xml:",any"`
}

type QuestXMLAction struct {
	XMLName xml.Name
	ID      int32            `xml:"id,attr"`
	QuestID *int32           `xml:"quest_id,attr"`
	VarID   int              `xml:"var_id,attr"`
	Value   int32            `xml:"value,attr"`
	Status  string           `xml:"status,attr"`
	ItemID  int32            `xml:"item_id,attr"`
	Count   int64            `xml:"count,attr"`
	Actions []QuestXMLAction `xml:",any"`
}

const (
	QuestReportTo       = "report_to"
	QuestMonsterHunt    = "monster_hunt"
	QuestItemCollecting = "item_collecting"
	QuestWorkOrder      = "work_order"
	QuestXML            = "xml_quest"
	QuestCustom         = "custom"
)

func supportedQuestScript(kind string) bool {
	return kind == QuestReportTo || kind == QuestMonsterHunt || kind == QuestItemCollecting || kind == QuestWorkOrder || kind == QuestXML || kind == QuestCustom
}

func (d *Data) loadQuestScripts(dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "quest_script_data/*.xml"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no quest script files in %s", dir)
	}
	d.QuestScripts = map[int32]*QuestScript{}
	d.QuestStarts = map[int32][]*QuestScript{}
	d.QuestEnds = map[int32][]*QuestScript{}
	d.QuestActions = map[int32][]*QuestScript{}
	d.QuestKills = map[int32][]*QuestScript{}
	d.QuestDropsByNPC = map[int32][]*QuestScript{}
	d.QuestXMLTalks = map[int32][]*QuestScript{}
	d.QuestCustomTalks = map[int32][]*QuestScript{}
	d.QuestItemUses = map[int32][]*QuestScript{}
	for _, path := range paths {
		var file struct {
			Reports    []*QuestScript `xml:"report_to"`
			Hunts      []*QuestScript `xml:"monster_hunt"`
			Collecting []*QuestScript `xml:"item_collecting"`
			WorkOrders []*QuestScript `xml:"work_order"`
			XML        []*QuestScript `xml:"xml_quest"`
		}
		if err := loadXML(path, &file); err != nil {
			return err
		}
		for _, group := range []struct {
			kind    string
			scripts []*QuestScript
		}{
			{QuestReportTo, file.Reports},
			{QuestMonsterHunt, file.Hunts},
			{QuestItemCollecting, file.Collecting},
			{QuestWorkOrder, file.WorkOrders},
			{QuestXML, file.XML},
		} {
			for _, script := range group.scripts {
				if previous := d.QuestScripts[script.ID]; previous != nil {
					return fmt.Errorf("quest script %d appears as both %s and %s", script.ID, previous.Kind, group.kind)
				}
				script.Kind = group.kind
				if group.kind == QuestWorkOrder {
					script.EndNPC = script.StartNPC
				}
				// Java stores monster_infos in an NPC-keyed map: a later
				// declaration for the same NPC replaces the earlier one.
				if group.kind == QuestMonsterHunt {
					byNPC := map[int32]int{}
					unique := make([]QuestMonster, 0, len(script.MonsterInfos))
					for _, monster := range script.MonsterInfos {
						if at, ok := byNPC[monster.NPCID]; ok {
							unique[at] = monster
						} else {
							byNPC[monster.NPCID] = len(unique)
							unique = append(unique, monster)
						}
					}
					script.MonsterInfos = unique
				}
				if script.EndNPC == 0 && (group.kind == QuestMonsterHunt || group.kind == QuestItemCollecting) {
					script.EndNPC = script.StartNPC
				}
				d.QuestScripts[script.ID] = script
				if group.kind == QuestXML {
					for _, event := range script.TalkEvents {
						for _, variable := range event.Vars {
							for _, npc := range variable.NPCs {
								d.QuestXMLTalks[npc.ID] = append(d.QuestXMLTalks[npc.ID], script)
							}
						}
					}
				}
				if !supportedQuestScript(group.kind) {
					continue
				}
				d.QuestStarts[script.StartNPC] = append(d.QuestStarts[script.StartNPC], script)
				d.QuestEnds[script.EndNPC] = append(d.QuestEnds[script.EndNPC], script)
				if script.ActionNPC != 0 {
					d.QuestActions[script.ActionNPC] = append(d.QuestActions[script.ActionNPC], script)
				}
				for _, monster := range script.MonsterInfos {
					d.QuestKills[monster.NPCID] = append(d.QuestKills[monster.NPCID], script)
				}
				if template := d.Quests[script.ID]; template != nil {
					seenDrops := map[int32]bool{}
					for _, drop := range template.QuestDrops {
						if !seenDrops[drop.NPCID] {
							d.QuestDropsByNPC[drop.NPCID] = append(d.QuestDropsByNPC[drop.NPCID], script)
							seenDrops[drop.NPCID] = true
						}
					}
				}
			}
		}
	}
	// Individually coded Java handlers enter the indexes only after their Go
	// behavior exists. An unsupported custom quest must not advertise a marker.
	for _, script := range []*QuestScript{
		{ID: 1111, Kind: QuestCustom, StartNPC: 203075, EndNPC: 203075, NPCStart: true},                                                                                                                                                                 // Insomnia Medicine
		{ID: 1114, Kind: QuestCustom, EndNPC: 203075, ItemID: 182200214, TalkNPCs: []int32{203058, 700008}},                                                                                                                                             // The Nymph's Gown
		{ID: 1011, Kind: QuestCustom, StartNPC: 203109, EndNPC: 203109, LevelUpStart: true, LevelUpNPC: 203109, TalkNPCs: []int32{203122}},                                                                                                              // Danger From Above
		{ID: 1012, Kind: QuestCustom, StartNPC: 203111, EndNPC: 203111, LevelUpStart: true, LevelUpNPC: 203111},                                                                                                                                         // Masked Loiterers
		{ID: 1013, Kind: QuestCustom, StartNPC: 203126, EndNPC: 203126, LevelUpStart: true, LevelUpNPC: 203126, MonsterInfos: []QuestMonster{{NPCID: 210688, VarID: 0, MaxKill: 11}, {NPCID: 210316, VarID: 0, MaxKill: 1}}},                            // Hunting Lepharist Revolutionaries
		{ID: 1014, Kind: QuestCustom, StartNPC: 203129, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203129, ItemID: 182200012, MonsterInfos: []QuestMonster{{NPCID: 210145, VarID: 0, MaxKill: 10}}, TalkNPCs: []int32{730020, 700090}},             // Odium in the Dukaki Settlement
		{ID: 1015, Kind: QuestCustom, StartNPC: 203129, EndNPC: 203129, LevelUpStart: true, LevelUpNPC: 203129},                                                                                                                                         // Frillneck Hunt
		{ID: 1016, Kind: QuestCustom, StartNPC: 203149, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203149, MonsterInfos: []QuestMonster{{NPCID: 210318, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{203148, 203832, 203705, 203822, 203761, 203195}}, // Source of the Pollution
		{ID: 1017, Kind: QuestCustom, StartNPC: 203178, EndNPC: 203178, LevelUpStart: true, LevelUpNPC: 203178},                                                                                                                                         // Held Sacred
		{ID: 1018, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203098},                                                                                                                                         // Mark of Vengeance
		{ID: 1019, Kind: QuestCustom, StartNPC: 203146, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203146, ItemID: 182200023, MonsterInfos: []QuestMonster{{NPCID: 210697, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{203147, 700037}},              // Flying Reconnaissance
		{ID: 1020, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203098, ItemID: 182200024, TalkNPCs: []int32{700141, 700142, 700551}},                                                                           // Sealing the Abyss Gate
		{ID: 1021, Kind: QuestCustom, StartNPC: 203129, EndNPC: 203129, LevelUpStart: true, LevelUpNPC: 203129, MonsterInfos: []QuestMonster{{NPCID: 210202, VarID: 0, MaxKill: 1}}},                                                                    // Trandila's Eggs
		{ID: 1022, Kind: QuestCustom, StartNPC: 203178, EndNPC: 203178, LevelUpStart: true, LevelUpNPC: 203178, MonsterInfos: []QuestMonster{{NPCID: 210178, VarID: 0, MaxKill: 5}}},                                                                    // Krall Desecration
		{ID: 1023, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098, LevelUpStart: true, LevelUpNPC: 203098, TalkNPCs: []int32{203183}},                                                                                                              // A Nest of Lepharists
		{ID: 1091, Kind: QuestCustom, EndNPC: 798155}, // A Request from Atropos
		{ID: 1092, Kind: QuestCustom, EndNPC: 798155, LevelUpStart: true, TalkNPCs: []int32{798206, 700388, 700389, 700390}},                                                                                                                                       // Josnack's Dilemma
		{ID: 1098, Kind: QuestCustom, StartNPC: 790001, EndNPC: 790001, NPCStart: true, LevelUpStart: true, LevelUpNPC: 790001, TalkNPCs: []int32{730008, 730019, 730133, 203183, 203989, 798155, 204549, 203752, 203164, 203917, 203996, 798176, 798212, 204535}}, // Pearl of Protection
		{ID: 1163, Kind: QuestCustom, StartNPC: 203096, EndNPC: 203155, NPCStart: true, TalkNPCs: []int32{203151}},                                                                                                                                                 // Arachna Antidote
		{ID: 1170, Kind: QuestCustom, StartNPC: 730000, EndNPC: 730000, NPCStart: true},                                                                                                                                                                            // Headless Stone Statue
		{ID: 1183, Kind: QuestCustom, StartNPC: 730012, EndNPC: 730012, NPCStart: true, TalkNPCs: []int32{730013, 730014}},                                                                                                                                         // Spirit of Nature
		{ID: 1192, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098, NPCStart: true, TalkNPCs: []int32{203701, 203833}},                                                                                                                                         // Verteron Reinforcements
		{ID: 1194, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098, NPCStart: true, MonsterInfos: []QuestMonster{{NPCID: 210185, VarID: 0, MaxKill: 10}, {NPCID: 210186, VarID: 0, MaxKill: 10}}},                                                              // Reducing Tursin Strength
		{ID: 1197, Kind: QuestCustom, StartNPC: 700004, EndNPC: 203129, ItemID: 182200558, ItemUseDelay: 3000},                                                                                                                                                     // Krall Book
		{ID: 1220, Kind: QuestCustom, StartNPC: 203172, MiddleNPC: 798004, EndNPC: 798046, NPCStart: true},                                                                                                                                                         // A Secret Delivery
		{ID: 1300, Kind: QuestCustom, StartNPC: 203901, EndNPC: 203901},                                                                                                                                                                                            // Orders from Telemachus
		{ID: 1097, Kind: QuestCustom, StartNPC: 790001, EndNPC: 790001, NPCStart: true, LevelUpStart: true, LevelUpNPC: 790001, TalkNPCs: []int32{798316, 279034}},                                                                                                 // Sword of Transcendence
		{ID: 1122, Kind: QuestCustom, StartNPC: 203060, EndNPC: 790001, ItemID: 182200216, NPCStart: true},                                                                                                                                                         // Delivering Pernos's Robe
		{ID: 1130, Kind: QuestCustom, StartNPC: 203098, EndNPC: 203098},                                                                                                                                                                                            // Summons to the Citadel
		{ID: 1149, Kind: QuestCustom, StartNPC: 203145, EndNPC: 203145, NPCStart: true, TalkNPCs: []int32{203191}},                                                                                                                                                 // Missing Poppy
		{ID: 1156, Kind: QuestCustom, StartNPC: 203128, EndNPC: 798003, NPCStart: true, TalkNPCs: []int32{700003}},                                                                                                                                                 // Stolen Village Seal
		{ID: 1157, Kind: QuestCustom, StartNPC: 798003, EndNPC: 798003, NPCStart: true},                                                                                                                                                                            // Gaphyrk's Love
		{ID: 1162, Kind: QuestCustom, StartNPC: 203095, EndNPC: 203095, NPCStart: true, TalkNPCs: []int32{203093, 700005}},                                                                                                                                         // Alteno's Wedding Ring
		{ID: 1158, Kind: QuestCustom, StartNPC: 798003, EndNPC: 203128, NPCStart: true, TalkNPCs: []int32{700003}},                                                                                                                                                 // Village Seal Found
		{ID: 1141, Kind: QuestCustom, StartNPC: 730001, EndNPC: 700122, NPCStart: true},                                                                                                                                                                            // Belbua's Treasure
		{ID: 1146, Kind: QuestCustom, StartNPC: 203123, EndNPC: 203139, NPCStart: true},                                                                                                                                                                            // Delicate Mandrake
		{ID: 3060, Kind: QuestCustom, StartNPC: 798190, EndNPC: 798193, MiddleNPC: 798191, MiddleNPC2: 798192, ItemID: 182208043, ItemUseDelay: 3000},                                                                                                              // The Red Journal
		{ID: 1006, Kind: QuestCustom, StartNPC: 790001, EndNPC: 790001, LevelUpStart: true, LevelUpNPC: 790001, ItemID: 182200007, MonsterInfos: []QuestMonster{{NPCID: 211042, VarID: 0, MaxKill: 54}}, TalkNPCs: []int32{730008, 205000}},                        // Ascension
		{ID: 1007, Kind: QuestCustom, StartNPC: 790001, EndNPC: 203758, LevelUpStart: true, LevelUpNPC: 790001, TalkNPCs: []int32{203725, 203752, 203758, 203759, 203760, 203761}},                                                                                 // A Ceremony in Sanctum
		{ID: 1031, Kind: QuestCustom, StartNPC: 203902, EndNPC: 203902, MonsterInfos: []QuestMonster{{NPCID: 210770, VarID: 0, MaxKill: 6}, {NPCID: 210771, VarID: 0, MaxKill: 6}, {NPCID: 210759, VarID: 0, MaxKill: 6}, {NPCID: 210758, VarID: 0, MaxKill: 6}}, TalkNPCs: []int32{203936, 700179, 204043, 204030}}, // The Manduri's Secret
		{ID: 1032, Kind: QuestCustom, StartNPC: 203932, EndNPC: 203932, ItemID: 182201001, TalkNPCs: []int32{730020, 730019, 700157}},                                                                                                                                                                                // A Ruler's Duty
		{ID: 1033, Kind: QuestCustom, StartNPC: 203900, EndNPC: 203900, LevelUpStart: true, LevelUpNPC: 203900, MonsterInfos: []QuestMonster{{NPCID: 210799, VarID: 0, MaxKill: 11}}, TalkNPCs: []int32{203996}},                                                                                                     // Sataloca's Heart
		{ID: 1034, Kind: QuestCustom, StartNPC: 203903, EndNPC: 203903, LevelUpStart: true, LevelUpNPC: 203903, TalkNPCs: []int32{204032, 204501, 700149}},                                                                                                                                                           // Disappearing Aether
		{ID: 1035, Kind: QuestCustom, StartNPC: 203917, EndNPC: 203917, LevelUpStart: true, LevelUpNPC: 203917, TalkNPCs: []int32{203992, 700158, 203965, 203968, 203987, 700160, 203934, 700159}},                                                                                                                   // Refreshing the Springs
		{ID: 2123, Kind: QuestCustom, StartNPC: 203550, EndNPC: 203550, NPCStart: true, TalkNPCs: []int32{700128}},                                                                                                                                                                                                   // The Imprisoned Gourmet
		{ID: 2125, Kind: QuestCustom, StartNPC: 203540, EndNPC: 203543, NPCStart: true},                                                                                                                                                                                                                              // The Robbery Plot
		{ID: 1036, Kind: QuestCustom, StartNPC: 203904, EndNPC: 203901, LevelUpStart: true, LevelUpNPC: 203904, TalkNPCs: []int32{204045, 204003, 204004, 204020}},                                                                                                                                                   // Kaidan Prisoner
		{ID: 2135, Kind: QuestCustom, StartNPC: 203532, EndNPC: 203532, ItemID: 182203131, NPCStart: true},                                                                                                                                                                                                           // For Love of Negi
		{ID: 2114, Kind: QuestCustom, StartNPC: 203533, EndNPC: 203533, NPCStart: true},                                                                                                                                                                                                                              // The Insect Problem
		{ID: 2122, Kind: QuestCustom, StartNPC: 203551, EndNPC: 203551, ItemID: 182203120},                                                                                                                                                                                                                           // Ashes to Ashes
		{ID: 1037, Kind: QuestCustom, StartNPC: 203965, EndNPC: 203965, LevelUpStart: true, LevelUpNPC: 203965, TalkNPCs: []int32{203967, 700151, 700154, 700150, 700153, 700152}},                                                                                                                                   // Secrets of the Temple
		{ID: 1038, Kind: QuestCustom, StartNPC: 203933, EndNPC: 203991, LevelUpStart: true, LevelUpNPC: 203933, MonsterInfos: []QuestMonster{{NPCID: 204005, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{700162, 700172}},                                                                                              // The Shadow's Command
		{ID: 1039, Kind: QuestCustom, StartNPC: 203946, EndNPC: 203946, LevelUpStart: true, LevelUpNPC: 203946, ItemID: 182201009, MonsterInfos: []QuestMonster{{NPCID: 210946, VarID: 1, MaxKill: 3}, {NPCID: 210947, VarID: 2, MaxKill: 3}}, TalkNPCs: []int32{203705}},                                            // Something in the Water
		{ID: 1040, Kind: QuestCustom, StartNPC: 203989, EndNPC: 203989, LevelUpStart: true, LevelUpNPC: 203989, MonsterInfos: []QuestMonster{{NPCID: 212010, VarID: 0, MaxKill: 3}, {NPCID: 204046, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{203901, 204020, 204024}},                                               // Scouting the Scouts
		{ID: 1041, Kind: QuestCustom, StartNPC: 203901, EndNPC: 204042, LevelUpStart: true, LevelUpNPC: 203901, TalkNPCs: []int32{204015, 700267, 203833, 278500, 700181}},                                                                                                                                           // A Dangerous Artifact
		{ID: 1042, Kind: QuestCustom, StartNPC: 203989, EndNPC: 203901, LevelUpStart: true, LevelUpNPC: 203989, ItemID: 182201018, MonsterInfos: []QuestMonster{{NPCID: 212029, VarID: 0, MaxKill: 2}, {NPCID: 212033, VarID: 0, MaxKill: 2}}},                                                                       // Keeper of the Kaidan Key
		{ID: 1043, Kind: QuestCustom, StartNPC: 203901, EndNPC: 203901, LevelUpStart: true, LevelUpNPC: 203901, MonsterInfos: []QuestMonster{{NPCID: 211629, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{204020, 204044}},                                                                                              // Balaur Conspiracy
		{ID: 1051, Kind: QuestCustom, StartNPC: 204501, EndNPC: 204501, LevelUpStart: true, LevelUpNPC: 204501, TalkNPCs: []int32{204582, 203882, 278503, 700217, 700303}},                                                                                                                                           // The Ruins of Roah
		{ID: 1052, Kind: QuestCustom, StartNPC: 204549, EndNPC: 730024, LevelUpStart: true, LevelUpNPC: 204549, ItemID: 182201603, TalkNPCs: []int32{730026}},                                                                                                                                                        // Root of the Rot
		{ID: 1053, Kind: QuestCustom, StartNPC: 204583, EndNPC: 204502, LevelUpStart: true, LevelUpNPC: 204583, MonsterInfos: []QuestMonster{{NPCID: 700169, VarID: 0, MaxKill: 1}, {NPCID: 212120, VarID: 0, MaxKill: 1}}},                                                                                          // The Klaw Threat
		{ID: 1054, Kind: QuestCustom, StartNPC: 730024, EndNPC: 204647, LevelUpStart: true, LevelUpNPC: 730024, TalkNPCs: []int32{730008, 730019}},                                                                                                                                                                   // The Power of Elim
		{ID: 1055, Kind: QuestCustom, StartNPC: 204629, EndNPC: 204629, LevelUpStart: true, LevelUpNPC: 204629, TalkNPCs: []int32{204625, 204628, 204627, 204626, 204622, 700270}},                                                                                                                                   // Eternal Rest
		{ID: 1056, Kind: QuestCustom, StartNPC: 204504, EndNPC: 203707, LevelUpStart: true, LevelUpNPC: 204504, MonsterInfos: []QuestMonster{{NPCID: 212151, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{204574, 203705}},                                                                                              // Lepharist Poison Research
		{ID: 1057, Kind: QuestCustom, StartNPC: 204502, EndNPC: 204500, LevelUpStart: true, LevelUpNPC: 204502, MonsterInfos: []QuestMonster{{NPCID: 700219, VarID: 0, MaxKill: 3}, {NPCID: 212211, VarID: 0, MaxKill: 1}}, TalkNPCs: []int32{204619, 700218, 700279}},                                               // Creating a Monster
		{ID: 1058, Kind: QuestCustom, StartNPC: 204020, EndNPC: 204501, LevelUpStart: true, LevelUpNPC: 204020, TalkNPCs: []int32{204501}},                                                                                                                                                                           // Aether Insanity
		{ID: 1059, Kind: QuestCustom, StartNPC: 204505, EndNPC: 204505, ItemID: 182201619, LevelUpStart: true, LevelUpNPC: 204505, TalkNPCs: []int32{204533, 700282, 204535}},                                                                                                                                        // The Archon of Storms
		{ID: 1062, Kind: QuestCustom, StartNPC: 204500, EndNPC: 204500, LevelUpStart: true, LevelUpNPC: 204500, TalkNPCs: []int32{204600, 204610}},                                                                                                                                                                   // Indratu Legion
		{ID: 1071, Kind: QuestCustom, StartNPC: 278532, EndNPC: 278532, ItemID: 182202001, LevelUpStart: true, LevelUpNPC: 278532, TalkNPCs: []int32{798026, 798025, 279019}},                                                                                                                                        // Speaking Balaur
		{ID: 1075, Kind: QuestCustom, StartNPC: 278506, EndNPC: 279023, LevelUpStart: true, LevelUpNPC: 278506, TalkNPCs: []int32{278643}},                                                                                                                                                                           // New Wings
		{ID: 1076, Kind: QuestCustom, StartNPC: 278500, EndNPC: 203704, ItemID: 182202006, LevelUpStart: true, LevelUpNPC: 278500, TalkNPCs: []int32{203834, 203786, 203754}},                                                                                                                                        // Fragment of Memory II
		{ID: 1072, Kind: QuestCustom, StartNPC: 278627, EndNPC: 278554, LevelUpStart: true, LevelUpNPC: 278627, TalkNPCs: []int32{278628, 278629, 278630, 278631, 278632, 278633}},                                                                                                                                   // Abyss Training
		{ID: 1123, Kind: QuestCustom, StartNPC: 790001, EndNPC: 790001, NPCStart: true},                                                                                                                                                                                                                              // Where's Tutty?
		{ID: 1100, Kind: QuestCustom, StartNPC: 203067, EndNPC: 203067},                                                                                                                                                                                                                                              // Kalio's Call
		{ID: 2100, Kind: QuestCustom, StartNPC: 203516, EndNPC: 203516},                                                                                                                                                                                                                                              // Order of the Captain
		{ID: 2011, Kind: QuestCustom, StartNPC: 203558, EndNPC: 203558, MonsterInfos: []QuestMonster{{NPCID: 700092, VarID: 0, MaxKill: 7}}, TalkNPCs: []int32{203572}},                                                                                                                                              // Fungus Among Us
		{ID: 2012, Kind: QuestCustom, StartNPC: 203559, EndNPC: 203559, MonsterInfos: []QuestMonster{{NPCID: 210715, VarID: 0, MaxKill: 4}}},                                                                                                                                                                         // Encroachers
		{ID: 2013, Kind: QuestCustom, StartNPC: 203605, EndNPC: 203605, TalkNPCs: []int32{700096}},                                                                                                                                                                                                                   // A Dangerous Crop
		{ID: 2014, Kind: QuestCustom, StartNPC: 203606, EndNPC: 203631, TalkNPCs: []int32{700009, 203633}, MonsterInfos: []QuestMonster{{NPCID: 700135, VarID: 0, MaxKill: 1}}},                                                                                                                                      // Scout it Out
		{ID: 2015, Kind: QuestCustom, EndNPC: 203631, MonsterInfos: []QuestMonster{{NPCID: 210510, VarID: 1, MaxKill: 1}, {NPCID: 210504, VarID: 2, MaxKill: 5}, {NPCID: 210506, VarID: 3, MaxKill: 5}}},                                                                                                             // Take the Initiative
		{ID: 2016, Kind: QuestCustom, EndNPC: 203631, ItemID: 182203019, TalkNPCs: []int32{203621}, MonsterInfos: []QuestMonster{{NPCID: 210455, VarID: 0, MaxKill: 6}, {NPCID: 210458, VarID: 0, MaxKill: 6}, {NPCID: 214032, VarID: 0, MaxKill: 6}}},                                                               // Fear This
		{ID: 2017, Kind: QuestCustom, EndNPC: 203558, TalkNPCs: []int32{203654}, MonsterInfos: []QuestMonster{{NPCID: 210528, VarID: 0, MaxKill: 6}, {NPCID: 210721, VarID: 0, MaxKill: 6}}},                                                                                                                         // Trespassers at the Observatory
		{ID: 2018, Kind: QuestCustom, EndNPC: 203649, TalkNPCs: []int32{700097, 700098}, MonsterInfos: []QuestMonster{{NPCID: 210588, VarID: 0, MaxKill: 4}, {NPCID: 210752, VarID: 0, MaxKill: 1}}},                                                                                                                 // Reconstructing Impetusium
		{ID: 2200, Kind: QuestCustom, EndNPC: 203557},                                       // Altgard Duties
		{ID: 2300, Kind: QuestCustom, EndNPC: 204301},                                       // Morheim Commander's Call
		{ID: 1001, Kind: QuestCustom, StartNPC: 203071, EndNPC: 203067},                     // The Kerub Threat
		{ID: 2001, Kind: QuestCustom, StartNPC: 203518, EndNPC: 203518},                     // Thinking Ahead
		{ID: 1002, Kind: QuestCustom, StartNPC: 203076, EndNPC: 203067},                     // Request of the Elim
		{ID: 1003, Kind: QuestCustom, StartNPC: 203081, EndNPC: 203081},                     // Illegal Logging
		{ID: 2002, Kind: QuestCustom, StartNPC: 203519, EndNPC: 203516},                     // Where's Rae?
		{ID: 2003, Kind: QuestCustom, StartNPC: 203539, EndNPC: 203539},                     // Treasure of the Deceased
		{ID: 1004, Kind: QuestCustom, StartNPC: 203082, EndNPC: 203067},                     // Neutralizing Odium
		{ID: 1005, Kind: QuestCustom, StartNPC: 203067, EndNPC: 203067},                     // Barring the Gate
		{ID: 2004, Kind: QuestCustom, StartNPC: 203539, EndNPC: 203539},                     // A Charmed Cube
		{ID: 2005, Kind: QuestCustom, StartNPC: 203540, EndNPC: 203540},                     // Teaching a Lesson
		{ID: 2006, Kind: QuestCustom, StartNPC: 203540, EndNPC: 203516},                     // Hit Them Where it Hurts
		{ID: 2007, Kind: QuestCustom, StartNPC: 203516, EndNPC: 203516},                     // Where's Rae This Time?
		{ID: 1107, Kind: QuestCustom, StartNPC: 203075, EndNPC: 203075, ItemID: 182200501},  // The Lost Axe
		{ID: 2136, Kind: QuestCustom, EndNPC: 790009, ActionNPC: 700146, ItemID: 182203130}, // The Lost Axe (Asmodian)
		{ID: 2107, Kind: QuestCustom, StartNPC: 203516, EndNPC: 203512, ItemID: 182203107},  // Return to Sender
		{ID: 1205, Kind: QuestCustom, EndNPC: 203087},                                       // A New Skill (Elyos)
		{ID: 2132, Kind: QuestCustom, EndNPC: 203527},                                       // A New Skill (Asmodians)
		{ID: 1966, Kind: QuestCustom, EndNPC: 278555, ItemID: 182206035, ItemUseDelay: 3000},
		{ID: 1967, Kind: QuestCustom, EndNPC: 278555, ItemID: 182206036, ItemUseDelay: 3000},
		{ID: 1968, Kind: QuestCustom, EndNPC: 278556, ItemID: 182206037, ItemUseDelay: 3000},
		{ID: 1969, Kind: QuestCustom, EndNPC: 278556, ItemID: 182206038, ItemUseDelay: 3000},
		{ID: 1970, Kind: QuestCustom, EndNPC: 278556, ItemID: 182206039, ItemUseDelay: 3000},
		{ID: 2966, Kind: QuestCustom, EndNPC: 278055, ItemID: 182207044, ItemUseDelay: 3000},
		{ID: 2967, Kind: QuestCustom, EndNPC: 278055, ItemID: 182207045, ItemUseDelay: 3000},
		{ID: 2968, Kind: QuestCustom, EndNPC: 278055, ItemID: 182207046, ItemUseDelay: 3000},
		{ID: 2969, Kind: QuestCustom, EndNPC: 278056, ItemID: 182207047, ItemUseDelay: 3000},
		{ID: 2970, Kind: QuestCustom, EndNPC: 278056, ItemID: 182207048, ItemUseDelay: 3000},
		{ID: 2971, Kind: QuestCustom, EndNPC: 278056, ItemID: 182207049, ItemUseDelay: 3000},
		{ID: 1355, Kind: QuestCustom, EndNPC: 203933, ItemID: 182201400, ItemUseDelay: 3000},
		{ID: 2316, Kind: QuestCustom, EndNPC: 204386, ItemID: 182204115, ItemUseDelay: 3000},
		{ID: 2216, Kind: QuestCustom, EndNPC: 203606, ItemID: 182203210, ItemUseDelay: 3000},
		{ID: 2228, Kind: QuestCustom, EndNPC: 203619, ItemID: 182203221, ItemUseDelay: 3000},
		{ID: 1182, Kind: QuestCustom, EndNPC: 203099, ItemID: 182200549, ItemUseDelay: 3000},
		{ID: 3049, Kind: QuestCustom, EndNPC: 798211, ItemID: 182208034, ItemUseDelay: 3000},
		{ID: 2569, Kind: QuestCustom, StartNPC: 204754, EndNPC: 204754, MiddleNPC: 204701, FinalNPC: 204768, NPCStart: true},
		{ID: 2553, Kind: QuestCustom, StartNPC: 798119, EndNPC: 798119, MiddleNPC: 204702, NPCStart: true},
		{ID: 2512, Kind: QuestCustom, StartNPC: 204703, EndNPC: 204703, MiddleNPC: 204801, FinalNPC: 204753, NPCStart: true},
		{ID: 2953, Kind: QuestCustom, StartNPC: 204191, EndNPC: 204191, MiddleNPC: 204071, NPCStart: true},
		{ID: 2928, Kind: QuestCustom, StartNPC: 204261, EndNPC: 204261, MiddleNPC: 204235, NPCStart: true},
		{ID: 2954, Kind: QuestCustom, StartNPC: 204191, EndNPC: 204191, MiddleNPC: 204221, NPCStart: true},
		{ID: 2914, Kind: QuestCustom, StartNPC: 204147, EndNPC: 204147, MiddleNPC: 204236, NPCStart: true},
		{ID: 2773, Kind: QuestCustom, StartNPC: 279019, EndNPC: 279019, MiddleNPC: 798110, FinalNPC: 204734, NPCStart: true},
		{ID: 4036, Kind: QuestCustom, StartNPC: 205187, EndNPC: 205187, MiddleNPC: 205144, FinalNPC: 205190, NPCStart: true},
		{ID: 4020, Kind: QuestCustom, StartNPC: 205120, EndNPC: 205120, MiddleNPC: 205141, NPCStart: true},
		{ID: 3091, Kind: QuestCustom, StartNPC: 798191, EndNPC: 798191, MiddleNPC: 798176, NPCStart: true},
		{ID: 3020, Kind: QuestCustom, StartNPC: 798143, EndNPC: 798143, MiddleNPC: 798149, NPCStart: true},
		{ID: 3037, Kind: QuestCustom, StartNPC: 798166, EndNPC: 798166, MiddleNPC: 798199, NPCStart: true},
		{ID: 3081, Kind: QuestCustom, StartNPC: 798155, EndNPC: 798155, MiddleNPC: 203830, FinalNPC: 798116, NPCStart: true},
		{ID: 2514, Kind: QuestCustom, StartNPC: 204702, EndNPC: 204702, MiddleNPC: 204700, MiddleNPC2: 204763, NPCStart: true},
		{ID: 2583, Kind: QuestCustom, StartNPC: 204715, EndNPC: 204715, MiddleNPC: 204805, MiddleNPC2: 204361, NPCStart: true},
		{ID: 2965, Kind: QuestCustom, StartNPC: 204182, EndNPC: 204182, MiddleNPC: 204055, MiddleNPC2: 278002, FinalNPC: 278109, NPCStart: true},
		{ID: 2913, Kind: QuestCustom, StartNPC: 204193, EndNPC: 204193, MiddleNPC: 204170, MiddleNPC2: 798065, FinalNPC: 204173, NPCStart: true},
		{ID: 2917, Kind: QuestCustom, StartNPC: 203574, EndNPC: 203574, MiddleNPC: 798029, MiddleNPC2: 204108, FinalNPC: 204241, NPCStart: true},
		{ID: 2767, Kind: QuestCustom, StartNPC: 279004, EndNPC: 279004, MiddleNPC: 279024, MiddleNPC2: 279022, NPCStart: true},
		{ID: 4001, Kind: QuestCustom, StartNPC: 205128, EndNPC: 205128, MiddleNPC: 205131, MiddleNPC2: 205133, FinalNPC: 205121, NPCStart: true},
		{ID: 3083, Kind: QuestCustom, StartNPC: 798156, EndNPC: 798156, MiddleNPC: 730024, MiddleNPC2: 798215, NPCStart: true},
		{ID: 3076, Kind: QuestCustom, StartNPC: 798155, EndNPC: 798155, MiddleNPC: 278503, MiddleNPC2: 278556, NPCStart: true},
		{ID: 3035, Kind: QuestCustom, StartNPC: 798155, EndNPC: 798155, MiddleNPC: 203830, MiddleNPC2: 279029, NPCStart: true},
		{ID: 3023, Kind: QuestCustom, StartNPC: 798138, EndNPC: 798138, MiddleNPC: 203785, MiddleNPC2: 798222, NPCStart: true},
		{ID: 2611, Kind: QuestCustom, StartNPC: 204763, EndNPC: 204763, MiddleNPC: 204773, MiddleNPC2: 204772, MiddleNPC3: 204700, NPCStart: true},
		{ID: 2501, Kind: QuestCustom, StartNPC: 204701, EndNPC: 204701, MiddleNPC: 204713, MiddleNPC2: 204719, MiddleNPC3: 204733, NPCStart: true},
		{ID: 2515, Kind: QuestCustom, StartNPC: 790015, EndNPC: 790015, MiddleNPC: 204192, MiddleNPC2: 204205, MiddleNPC3: 798081, NPCStart: true},
		{ID: 2523, Kind: QuestCustom, StartNPC: 204802, EndNPC: 204802, MiddleNPC: 798117, MiddleNPC2: 798118, MiddleNPC3: 798119, FinalNPC: 204734, NPCStart: true},
		{ID: 2539, Kind: QuestCustom, StartNPC: 790022, EndNPC: 790022, MiddleNPC: 204433, MiddleNPC2: 204112, MiddleNPC3: 204056, NPCStart: true},
		{ID: 2692, Kind: QuestCustom, StartNPC: 212164, EndNPC: 212164, MiddleNPC: 204108, MiddleNPC2: 279027, MiddleNPC3: 279029, NPCStart: true},
		{ID: 2912, Kind: QuestCustom, StartNPC: 204193, EndNPC: 204193, MiddleNPC: 204089, MiddleNPC2: 204088, MiddleNPC3: 204240, FinalNPC: 204236, NPCStart: true},
		{ID: 4052, Kind: QuestCustom, StartNPC: 730152, EndNPC: 730152, MiddleNPC: 205179, MiddleNPC2: 205166, MiddleNPC3: 205197, NPCStart: true},
		{ID: 4101, Kind: QuestCustom, StartNPC: 205159, EndNPC: 205159, MiddleNPC: 205194, MiddleNPC2: 205195, MiddleNPC3: 205196, FinalNPC: 205193, NPCStart: true},
		{ID: 1528, Kind: QuestCustom, StartNPC: 204553, MiddleNPC: 204549, EndNPC: 204583, NPCStart: true},
		{ID: 1527, Kind: QuestCustom, StartNPC: 204555, MiddleNPC: 204562, EndNPC: 730024, NPCStart: true},
		{ID: 1609, Kind: QuestCustom, StartNPC: 204574, MiddleNPC: 204557, EndNPC: 730024, NPCStart: true},
		{ID: 1628, Kind: QuestCustom, StartNPC: 204596, MiddleNPC: 204590, EndNPC: 204630, NPCStart: true},
		{ID: 1909, Kind: QuestCustom, StartNPC: 203739, MiddleNPC: 203726, EndNPC: 203099, NPCStart: true},
		{ID: 1452, Kind: QuestCustom, StartNPC: 203934, MiddleNPC: 203834, EndNPC: 203704, NPCStart: true},
		{ID: 1324, Kind: QuestCustom, StartNPC: 203904, MiddleNPC: 204031, EndNPC: 203940, NPCStart: true},
		{ID: 2693, Kind: QuestCustom, StartNPC: 204763, MiddleNPC: 204702, EndNPC: 204803, NPCStart: true},
		{ID: 2651, Kind: QuestCustom, StartNPC: 204775, MiddleNPC: 204764, EndNPC: 204650, NPCStart: true},
		{ID: 1314, Kind: QuestCustom, StartNPC: 730023, MiddleNPC: 730021, EndNPC: 730014, NPCStart: true},
		{ID: 2222, Kind: QuestCustom, StartNPC: 203607, MiddleNPC: 203609, EndNPC: 203631, NPCStart: true},
		{ID: 1218, Kind: QuestCustom, StartNPC: 203121, MiddleNPC: 798004, EndNPC: 203172, NPCStart: true},
		{ID: 1131, Kind: QuestCustom, StartNPC: 203097, MiddleNPC: 798001, EndNPC: 203101, NPCStart: true},
		{ID: 1553, Kind: QuestCustom, StartNPC: 203786, MiddleNPC: 730051, MiddleNPC2: 204500, EndNPC: 204584, NPCStart: true},
		{ID: 1620, Kind: QuestCustom, StartNPC: 204519, MiddleNPC: 790000, MiddleNPC2: 730001, EndNPC: 203125, NPCStart: true},
		{ID: 1578, Kind: QuestCustom, StartNPC: 730025, MiddleNPC: 730024, MiddleNPC2: 204560, EndNPC: 204579, NPCStart: true},
		{ID: 1605, Kind: QuestCustom, StartNPC: 204576, MiddleNPC: 204530, MiddleNPC2: 204501, EndNPC: 204577, NPCStart: true},
		{ID: 1483, Kind: QuestCustom, StartNPC: 798126, MiddleNPC: 203940, MiddleNPC2: 203944, EndNPC: 798127, NPCStart: true},
		{ID: 2578, Kind: QuestCustom, ItemID: 182204453, ItemUseDelay: 3000, MiddleNPC: 204741, MiddleNPC2: 790017, EndNPC: 204746},
		{ID: 2846, Kind: QuestCustom, ItemID: 182205677, ItemUseDelay: 3000, MiddleNPC: 278039, MiddleNPC2: 279027, EndNPC: 798317},
		{ID: 2847, Kind: QuestCustom, ItemID: 182205678, ItemUseDelay: 3000, MiddleNPC: 278020, MiddleNPC2: 279005, EndNPC: 798063},
		{ID: 2848, Kind: QuestCustom, ItemID: 182205679, ItemUseDelay: 3000, MiddleNPC: 278137, MiddleNPC2: 278089, EndNPC: 204799},
		{ID: 4053, Kind: QuestCustom, ItemID: 182209031, ItemUseDelay: 3000, MiddleNPC: 205165, MiddleNPC2: 205167, EndNPC: 205178},
		{ID: 3934, Kind: QuestCustom, StartNPC: 203701, EndNPC: 203701, NPCStart: true, TalkNPCs: []int32{798359, 798360, 798361, 798362, 798363, 798364, 798365, 798366, 203752}},
		{ID: 3935, Kind: QuestCustom, StartNPC: 203701, EndNPC: 203701, NPCStart: true, TalkNPCs: []int32{203316, 203702, 203329, 203752}},
		{ID: 3936, Kind: QuestCustom, StartNPC: 203710, EndNPC: 203710, NPCStart: true},
		{ID: 3938, Kind: QuestCustom, StartNPC: 203701, EndNPC: 203701, NPCStart: true, TalkNPCs: []int32{203788, 203792, 203790, 203793, 203784, 203786, 798316, 203752}},
		{ID: 3939, Kind: QuestCustom, StartNPC: 203701, EndNPC: 203701, NPCStart: true, TalkNPCs: []int32{203780, 203781, 203752}},
		{ID: 3965, Kind: QuestCustom, StartNPC: 798311, EndNPC: 798390, NPCStart: true, TalkNPCs: []int32{798391}},
		{ID: 3966, Kind: QuestCustom, StartNPC: 798391, EndNPC: 798391, NPCStart: true, TalkNPCs: []int32{203994, 204030, 204568}},
		{ID: 3967, Kind: QuestCustom, StartNPC: 798391, EndNPC: 798391, NPCStart: true, TalkNPCs: []int32{798309}},
		{ID: 3968, Kind: QuestCustom, StartNPC: 798390, EndNPC: 798390, NPCStart: true, TalkNPCs: []int32{798176, 204528, 203927}},
		{ID: 3969, Kind: QuestCustom, StartNPC: 798390, EndNPC: 798390, NPCStart: true, TalkNPCs: []int32{798391}},
		{ID: 4015, Kind: QuestCustom, StartNPC: 205130, EndNPC: 205130, NPCStart: true},
		{ID: 4060, Kind: QuestCustom, EndNPC: 205204, ItemID: 182209037, ItemUseDelay: 3000, TalkNPCs: []int32{205156, 204143, 204731}},
		{ID: 3093, Kind: QuestCustom, StartNPC: 798185, EndNPC: 798185, NPCStart: true, TalkNPCs: []int32{798177, 798179, 203784}},
		{ID: 3200, Kind: QuestCustom, StartNPC: 204658, EndNPC: 798322, NPCStart: true, ItemID: 182209082, ActionNPC: 700522, TalkNPCs: []int32{798332, 279006}},
		{ID: 3319, Kind: QuestCustom, StartNPC: 798050, EndNPC: 798050, NPCStart: true, TalkNPCs: []int32{798138}},
		{ID: 3326, Kind: QuestCustom, StartNPC: 798053, EndNPC: 798053, NPCStart: true, MonsterInfos: []QuestMonster{{NPCID: 210897, VarID: 0, MaxKill: 20}, {NPCID: 210939, VarID: 0, MaxKill: 20}, {NPCID: 210873, VarID: 0, MaxKill: 20}, {NPCID: 210919, VarID: 0, MaxKill: 20}, {NPCID: 211754, VarID: 0, MaxKill: 20}}},
		{ID: 3914, Kind: QuestCustom, EndNPC: 203384, ItemID: 182206084, TalkNPCs: []int32{203752}},
		{ID: 3930, Kind: QuestCustom, StartNPC: 203711, EndNPC: 203711, NPCStart: true, ActionNPC: 700562, TalkNPCs: []int32{203833, 798321}},
		{ID: 3931, Kind: QuestCustom, StartNPC: 203711, EndNPC: 203711, NPCStart: true, TalkNPCs: []int32{798321, 279005}},
		{ID: 3932, Kind: QuestCustom, StartNPC: 203711, EndNPC: 203711, NPCStart: true, TalkNPCs: []int32{204656}},
		{ID: 3933, Kind: QuestCustom, StartNPC: 203701, EndNPC: 203701, NPCStart: true, TalkNPCs: []int32{203704, 203705, 203706, 203707, 203752}},
		{ID: 4200, Kind: QuestCustom, StartNPC: 204839, EndNPC: 204286, NPCStart: true, ItemID: 182209097, ActionNPC: 700522, TalkNPCs: []int32{798332, 279006}},
		{ID: 4934, Kind: QuestCustom, StartNPC: 204051, EndNPC: 204051, NPCStart: true, ActionNPC: 700562, TalkNPCs: []int32{204211, 204285}},
		{ID: 4935, Kind: QuestCustom, StartNPC: 204051, EndNPC: 204051, NPCStart: true, TalkNPCs: []int32{204285, 279005}},
		{ID: 4936, Kind: QuestCustom, StartNPC: 204051, EndNPC: 204051, NPCStart: true, TalkNPCs: []int32{204837}},
		{ID: 4937, Kind: QuestCustom, StartNPC: 204053, EndNPC: 204053, NPCStart: true, TalkNPCs: []int32{204059, 204058, 204057, 204056, 204075}},
		{ID: 4938, Kind: QuestCustom, StartNPC: 204053, EndNPC: 204053, NPCStart: true, TalkNPCs: []int32{798367, 798368, 798369, 798370, 798371, 798372, 798373, 798374, 204075}},
		{ID: 4939, Kind: QuestCustom, StartNPC: 204053, EndNPC: 204053, NPCStart: true, TalkNPCs: []int32{204055, 204273, 204054, 204075}},
		{ID: 4942, Kind: QuestCustom, StartNPC: 204053, EndNPC: 204053, NPCStart: true, TalkNPCs: []int32{204104, 204108, 204106, 204110, 204100, 204102, 798317, 204075}},
		{ID: 4943, Kind: QuestCustom, StartNPC: 204053, EndNPC: 204053, NPCStart: true, TalkNPCs: []int32{204096, 204097, 204075}},
		{ID: 19004, Kind: QuestCustom, StartNPC: 203757, EndNPC: 798500, NPCStart: true, TalkNPCs: []int32{203752, 203701}},
		{ID: 2901, Kind: QuestCustom, StartNPC: 204191, EndNPC: 203559, LevelUpStart: true, LevelUpNPC: 204191, LevelUpWorld: 220030000, LevelUpX: 1748, LevelUpY: 1807, LevelUpZ: 255, LevelUpDelayMS: 1000},
		{ID: 2902, Kind: QuestCustom, StartNPC: 204191, EndNPC: 203559, LevelUpStart: true, LevelUpNPC: 204191, LevelUpWorld: 220030000, LevelUpX: 1748, LevelUpY: 1807, LevelUpZ: 255, LevelUpDelayMS: 1000},
		{ID: 2903, Kind: QuestCustom, StartNPC: 204191, EndNPC: 203559, LevelUpStart: true, LevelUpNPC: 204191, LevelUpWorld: 220030000, LevelUpX: 1748, LevelUpY: 1807, LevelUpZ: 255, LevelUpDelayMS: 1000},
		{ID: 2904, Kind: QuestCustom, StartNPC: 204191, EndNPC: 203559, LevelUpStart: true, LevelUpNPC: 204191, LevelUpWorld: 220030000, LevelUpX: 1748, LevelUpY: 1807, LevelUpZ: 255, LevelUpDelayMS: 1000},
		{ID: 1913, Kind: QuestCustom, EndNPC: 203097, LevelUpStart: true, LevelUpNPC: 203726, LevelUpWorld: 210030000, LevelUpX: 1643, LevelUpY: 1500, LevelUpZ: 120, LevelUpDelayMS: 1000},
		{ID: 1914, Kind: QuestCustom, EndNPC: 203097, LevelUpStart: true, LevelUpNPC: 203726, LevelUpWorld: 210030000, LevelUpX: 1643, LevelUpY: 1500, LevelUpZ: 120, LevelUpDelayMS: 1000},
		{ID: 1915, Kind: QuestCustom, EndNPC: 203097, LevelUpStart: true, LevelUpNPC: 203726, LevelUpWorld: 210030000, LevelUpX: 1643, LevelUpY: 1500, LevelUpZ: 120, LevelUpDelayMS: 1000},
		{ID: 1916, Kind: QuestCustom, EndNPC: 203097, LevelUpStart: true, LevelUpNPC: 203726, LevelUpWorld: 210030000, LevelUpX: 1643, LevelUpY: 1500, LevelUpZ: 120, LevelUpDelayMS: 1000},
		{ID: 1309, Kind: QuestCustom, StartNPC: 203932, EndNPC: 203830, ItemID: 182201304, ItemUseDelay: 3000},
		{ID: 1323, Kind: QuestCustom, StartNPC: 730019, EndNPC: 203939, ItemID: 182201309, ItemUseDelay: 3000},
		{ID: 2274, Kind: QuestCustom, StartNPC: 203668, EndNPC: 203560, ItemID: 182203249, ItemUseDelay: 3000},
		{ID: 2019, Kind: QuestCustom},                                   // Securing the Supply Route (translated Java handler)
		{ID: 2020, Kind: QuestCustom},                                   // Keeping the Black Claw Tribe in Check (translated Java handler)
		{ID: 2021, Kind: QuestCustom},                                   // Know Your Enemy (translated Java handler)
		{ID: 2022, Kind: QuestCustom},                                   // Crushing the Conspiracy (translated Java handler)
		{ID: 2031, Kind: QuestCustom},                                   // Petrifying Elim (translated Java handler)
		{ID: 2032, Kind: QuestCustom},                                   // Guardian Spirit (translated Java handler)
		{ID: 2033, Kind: QuestCustom},                                   // Destroying the Curse (translated Java handler)
		{ID: 2034, Kind: QuestCustom},                                   // The Hand Behind the Ice Claw (translated Java handler)
		{ID: 2035, Kind: QuestCustom},                                   // [Group] The Three Keys (translated Java handler)
		{ID: 2036, Kind: QuestCustom},                                   // A Captive Flame (translated Java handler)
		{ID: 2037, Kind: QuestCustom},                                   // The Protector of Nepra (translated Java handler)
		{ID: 2038, Kind: QuestCustom},                                   // A Lost Daeva (translated Java handler)
		{ID: 2039, Kind: QuestCustom},                                   // Allies Among Enemies (translated Java handler)
		{ID: 2040, Kind: QuestCustom},                                   // Kikananta's Loyalty (translated Java handler)
		{ID: 2051, Kind: QuestCustom},                                   // Saving Beluslan Fortress (translated Java handler)
		{ID: 2052, Kind: QuestCustom},                                   // An Undead Occupation (translated Java handler)
		{ID: 2053, Kind: QuestCustom},                                   // A Missing Father (translated Java handler)
		{ID: 2054, Kind: QuestCustom},                                   // Light up the Lighthouse (translated Java handler)
		{ID: 2055, Kind: QuestCustom},                                   // The Seiren's Treasure (translated Java handler)
		{ID: 2056, Kind: QuestCustom},                                   // Thawing Kurngalfberg (translated Java handler)
		{ID: 2060, Kind: QuestCustom},                                   // Restoring Beluslan Observatory (translated Java handler)
		{ID: 2500, Kind: QuestCustom},                                   // Orders From Nerita (translated Java handler)
		{ID: 2701, Kind: QuestCustom},                                   // The Governor's Summons (translated Java handler)
		{ID: 2091, Kind: QuestCustom},                                   // Meet the Reapers (translated Java handler)
		{ID: 1466, Kind: QuestCustom, StartNPC: 212649, NPCStart: true}, // [Spy] Respect For Deltras (translated Java handler)
		{ID: 1467, Kind: QuestCustom, StartNPC: 204045, NPCStart: true}, // [Group] The Four Leaders (translated Java handler)
		{ID: 1468, Kind: QuestCustom, StartNPC: 790004, NPCStart: true}, // Hannet's Lost Love (translated Java handler)
		{ID: 1469, Kind: QuestCustom, StartNPC: 790004, NPCStart: true}, // [Spy/Group] Finding Denlavis (translated Java handler)
		{ID: 1470, Kind: QuestCustom, StartNPC: 790004, NPCStart: true}, // [Spy/Group] Hannet's Vengeance (translated Java handler)
		{ID: 1471, Kind: QuestCustom, StartNPC: 203991, NPCStart: true}, // Fake Stigma (translated Java handler)
		{ID: 1472, Kind: QuestCustom, StartNPC: 203903, NPCStart: true}, // [Spy] Ganimerk's Espionage (translated Java handler)
		{ID: 1500, Kind: QuestCustom},                                   // Orders From Perento (translated Java handler)
		{ID: 1701, Kind: QuestCustom},                                   // Governor's Directive (translated Java handler)
		{ID: 2008, Kind: QuestCustom},                                   // Ascension (translated Java handler)
		{ID: 2009, Kind: QuestCustom},                                   // A Ceremony in Pandaemonium (translated Java handler)
		{ID: 1929, Kind: QuestCustom},                                   // A Sliver of Darkness (translated Java handler)
		{ID: 2098, Kind: QuestCustom},                                   // But What we Make (translated Java handler)
		{ID: 2900, Kind: QuestCustom},                                   // No Escaping Destiny (translated Java handler)
	} {
		if d.Quests[script.ID] == nil || d.QuestScripts[script.ID] != nil {
			return fmt.Errorf("custom quest %d has no template or conflicts with XML", script.ID)
		}
		d.QuestScripts[script.ID] = script
		if script.ID == 1098 {
			for _, npcID := range []int32{790001, 730008, 730019, 730133, 203183, 203989, 798155, 204549, 203752, 203164, 203917, 203996, 798176, 798212, 204535} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1092 {
			for _, npcID := range []int32{798155, 798206, 700388, 700389, 700390} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1162 {
			for _, npcID := range []int32{203095, 203093, 700005} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1163 {
			for _, npcID := range []int32{203096, 203151, 203155} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1170 {
			for _, npcID := range []int32{730000, 700033} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1183 {
			for _, npcID := range []int32{730012, 730013, 730014} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1192 {
			for _, npcID := range []int32{203098, 203701, 203833} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1031 {
			for _, npcID := range []int32{203902, 203936, 700179, 204043, 204030} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1032 {
			for _, npcID := range []int32{203932, 730020, 730019, 700157} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1033 {
			d.QuestCustomTalks[203996] = append(d.QuestCustomTalks[203996], script)
		}
		if script.ID == 1034 {
			for _, npcID := range []int32{204032, 204501, 700149} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1035 {
			for _, npcID := range []int32{203992, 700158, 203965, 203968, 203987, 700160, 203934, 700159} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1036 {
			for _, npcID := range []int32{204045, 204003, 204004, 204020} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1037 {
			for _, npcID := range []int32{203967, 700151, 700154, 700150, 700153, 700152} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1038 {
			for _, npcID := range []int32{700162, 700172} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1039 {
			d.QuestCustomTalks[script.StartNPC] = append(d.QuestCustomTalks[script.StartNPC], script)
			d.QuestCustomTalks[203705] = append(d.QuestCustomTalks[203705], script)
		}
		if script.NPCStart {
			d.QuestStarts[script.StartNPC] = append(d.QuestStarts[script.StartNPC], script)
		}
		d.QuestEnds[script.EndNPC] = append(d.QuestEnds[script.EndNPC], script)
		if script.ActionNPC != 0 {
			d.QuestActions[script.ActionNPC] = append(d.QuestActions[script.ActionNPC], script)
		}
		if script.LevelUpStart && script.LevelUpNPC != 0 {
			d.QuestCustomTalks[script.LevelUpNPC] = append(d.QuestCustomTalks[script.LevelUpNPC], script)
		}
		for _, monster := range script.MonsterInfos {
			d.QuestKills[monster.NPCID] = append(d.QuestKills[monster.NPCID], script)
		}
		if script.MiddleNPC != 0 && script.MiddleNPC != script.EndNPC {
			d.QuestEnds[script.MiddleNPC] = append(d.QuestEnds[script.MiddleNPC], script)
		}
		if script.MiddleNPC2 != 0 && script.MiddleNPC2 != script.EndNPC && script.MiddleNPC2 != script.MiddleNPC {
			d.QuestEnds[script.MiddleNPC2] = append(d.QuestEnds[script.MiddleNPC2], script)
		}
		if script.MiddleNPC3 != 0 && script.MiddleNPC3 != script.EndNPC && script.MiddleNPC3 != script.MiddleNPC && script.MiddleNPC3 != script.MiddleNPC2 {
			d.QuestEnds[script.MiddleNPC3] = append(d.QuestEnds[script.MiddleNPC3], script)
		}
		for _, npcID := range script.TalkNPCs {
			if npcID != script.EndNPC {
				d.QuestEnds[npcID] = append(d.QuestEnds[npcID], script)
			}
		}
		if script.MiddleNPC != 0 && script.FinalNPC != 0 && script.FinalNPC != script.EndNPC {
			d.QuestEnds[script.FinalNPC] = append(d.QuestEnds[script.FinalNPC], script)
		}
		if script.ID == 1205 || script.ID == 2132 {
			startNPC := script.EndNPC
			for npcID := startNPC + 1; npcID <= startNPC+3; npcID++ {
				d.QuestEnds[npcID] = append(d.QuestEnds[npcID], script)
			}
		}
		if script.ID == 2122 {
			for _, npcID := range []int32{203551, 700148, 730029} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
			d.QuestItemUses[script.ItemID] = append(d.QuestItemUses[script.ItemID], script)
		}
		if script.ID == 2123 {
			d.QuestCustomTalks[700128] = append(d.QuestCustomTalks[700128], script)
		}
		switch script.ID {
		case 2011:
			for _, npcID := range []int32{203558, 203572} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		case 2012:
			d.QuestCustomTalks[203559] = append(d.QuestCustomTalks[203559], script)
		case 2013:
			for _, npcID := range []int32{203605, 700096} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		case 2014:
			for _, npcID := range []int32{203606, 700009, 203633, 203631} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		case 2015:
			d.QuestCustomTalks[203631] = append(d.QuestCustomTalks[203631], script)
		case 2016:
			for _, npcID := range []int32{203631, 203621} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
			d.QuestItemUses[script.ItemID] = append(d.QuestItemUses[script.ItemID], script)
		case 2017:
			for _, npcID := range []int32{203654, 203558} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		case 2018:
			for _, npcID := range []int32{203649, 700097, 700098} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 3060 {
			for _, npcID := range []int32{798190, 798191, 798192, 798193} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1114 {
			for _, npcID := range []int32{203075, 203058, 700008} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1011 {
			d.QuestCustomTalks[203122] = append(d.QuestCustomTalks[203122], script)
			d.QuestKills[700091] = append(d.QuestKills[700091], script)
		}
		if script.ID == 1012 {
			d.QuestCustomTalks[203111] = append(d.QuestCustomTalks[203111], script)
		}
		if script.ID == 1014 {
			for _, npcID := range []int32{730020, 700090} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
			d.QuestItemUses[script.ItemID] = append(d.QuestItemUses[script.ItemID], script)
		}
		if script.ID == 1015 {
			for _, npcID := range []int32{210126, 210200, 210201} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 1016 {
			for _, npcID := range []int32{203148, 203832, 203705, 203822, 203761, 203098, 203195} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1019 {
			for _, npcID := range []int32{203098, 203147, 700037} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
			d.QuestItemUses[script.ItemID] = append(d.QuestItemUses[script.ItemID], script)
		}
		if script.ID == 1130 {
			d.QuestCustomTalks[203098] = append(d.QuestCustomTalks[203098], script)
		}
		if script.ID == 1194 {
			d.QuestCustomTalks[script.StartNPC] = append(d.QuestCustomTalks[script.StartNPC], script)
		}
		if script.ID == 1156 {
			for _, npcID := range []int32{203128, 700003, 798003} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1157 {
			d.QuestCustomTalks[798003] = append(d.QuestCustomTalks[798003], script)
		}
		if script.ID == 1158 {
			for _, npcID := range []int32{798003, 700003, 203128} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 1141 {
			d.QuestCustomTalks[script.StartNPC] = append(d.QuestCustomTalks[script.StartNPC], script)
			d.QuestCustomTalks[script.EndNPC] = append(d.QuestCustomTalks[script.EndNPC], script)
		}
		if script.ID == 1146 {
			d.QuestCustomTalks[script.StartNPC] = append(d.QuestCustomTalks[script.StartNPC], script)
			d.QuestCustomTalks[script.EndNPC] = append(d.QuestCustomTalks[script.EndNPC], script)
		}
		if script.ID == 1023 {
			d.QuestCustomTalks[203183] = append(d.QuestCustomTalks[203183], script)
		}
		if script.ID == 1006 || script.ID == 1032 || script.ID == 1039 || script.ID == 1042 || script.ID == 1052 || script.ID == 1059 || script.ID == 1071 || script.ID == 1076 || script.ID == 1107 || script.ID == 1114 || script.ID == 2107 || script.ID == 2136 || script.ID == 4200 || script.ID == 3200 || script.ID == 3914 || script.ItemUseDelay > 0 {
			d.QuestItemUses[script.ItemID] = append(d.QuestItemUses[script.ItemID], script)
		}
		if script.ID == 1052 {
			d.QuestItemUses[182201604] = append(d.QuestItemUses[182201604], script)
		}
		if script.ID == 1062 {
			for _, npcID := range []int32{700220, 212588} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ItemUseDelay > 0 && script.StartNPC != 0 {
			d.QuestCustomTalks[script.StartNPC] = append(d.QuestCustomTalks[script.StartNPC], script)
			if script.EndNPC != script.StartNPC {
				d.QuestCustomTalks[script.EndNPC] = append(d.QuestCustomTalks[script.EndNPC], script)
			}
		}
		if script.ID == 2114 {
			for _, npcID := range []int32{210734, 210380, 210381} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 1001 {
			d.QuestCustomTalks[203071] = append(d.QuestCustomTalks[203071], script)
			d.QuestKills[210670] = append(d.QuestKills[210670], script)
		}
		if script.ID == 3969 {
			for _, npcID := range []int32{798390, 798391} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 4015 {
			d.QuestCustomTalks[730107] = append(d.QuestCustomTalks[730107], script)
		}
		if script.ID == 2001 {
			d.QuestCustomTalks[700093] = append(d.QuestCustomTalks[700093], script)
			for _, npcID := range []int32{210368, 210369} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 1002 {
			d.QuestCustomTalks[730010] = append(d.QuestCustomTalks[730010], script)
		}
		if script.ID == 1003 {
			for _, npcID := range []int32{210096, 210149, 210145, 210146, 210150, 210151, 210092, 210160, 210154} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 2002 {
			for _, npcID := range []int32{700045, 203537} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
			for _, npcID := range []int32{210377, 210378} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 1004 {
			d.QuestCustomTalks[700030] = append(d.QuestCustomTalks[700030], script)
		}
		if script.ID == 1005 {
			for _, npcID := range []int32{700081, 700082, 700083, 700080} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		if script.ID == 2004 {
			d.QuestCustomTalks[700047] = append(d.QuestCustomTalks[700047], script)
			for _, npcID := range []int32{210402, 210403} {
				d.QuestKills[npcID] = append(d.QuestKills[npcID], script)
			}
		}
		if script.ID == 2006 {
			d.QuestCustomTalks[700095] = append(d.QuestCustomTalks[700095], script)
		}
		if script.ID == 2007 {
			for _, npcID := range []int32{700081, 700082, 700083} {
				d.QuestCustomTalks[npcID] = append(d.QuestCustomTalks[npcID], script)
			}
		}
		seenDrops := map[int32]bool{}
		for _, drop := range d.Quests[script.ID].QuestDrops {
			if !seenDrops[drop.NPCID] {
				d.QuestDropsByNPC[drop.NPCID] = append(d.QuestDropsByNPC[drop.NPCID], script)
				seenDrops[drop.NPCID] = true
			}
		}
	}
	for _, index := range []map[int32][]*QuestScript{d.QuestStarts, d.QuestEnds, d.QuestActions, d.QuestKills, d.QuestDropsByNPC} {
		for _, scripts := range index {
			sort.Slice(scripts, func(i, j int) bool { return scripts[i].ID < scripts[j].ID })
		}
	}
	return nil
}
