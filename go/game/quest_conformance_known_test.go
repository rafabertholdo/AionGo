package game

// conformanceException is a documented Java deviation: the Java handler really does this, so the rule does not apply.
type conformanceException struct {
	Reason string
	Java   string // file:line in AL-Game/data/scripts/system/handlers/quest
}

// conformanceExceptions is "<quest id>/<rule>" -> why Java itself breaks the rule. Cite the Java line.
var conformanceExceptions = map[string]conformanceException{
	"1020/echo":      {Java: "verteron/_1020SealingTheAbyssGate.java:85", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"1122/clickpage": {Java: "poeta/_1122DeliveringPernossRobe.java:55", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"1123/clickpage": {Java: "poeta/_1123WheresTutty.java:51", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"1149/clickpage": {Java: "verteron/_1149MissingPoppy.java:53", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"2006/echo":      {Java: "ishalgen/_2006HitThemWhereitHurts.java:53", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"2122/echo":      {Java: "ishalgen/_2122AshestoAshes.java:59", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"2136/click":     {Java: "ishalgen/_2136TheLostAxe.java:60", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"2136/echo":      {Java: "ishalgen/_2136TheLostAxe.java:60", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"3967/click":     {Java: "sanctum/_3967AndusDyeBox.java:54", Reason: "Java's onDialogEvent does this (fall-through or a handler returning true without a window); verified probe by probe by TestQuestJavaParity"},
	"1197/click":     {Java: "verteron/_1197KrallBook.java:76-85", Reason: "The Krall Book NPC answers a plain click with no dialog packet while offering its item"},
	"1197/echo":      {Java: "verteron/_1197KrallBook.java:76-85", Reason: "The Krall Book NPC answers every dialog selection with no packet while the quest is absent or NONE"},
	"2123/echo":      {Java: "ishalgen/_2123TheImprisonedGourmet.java:154-173", Reason: "NPC 700128 handles every dialog selection during the initial quest step, so unknown selections produce no window echo"},
	"1194/clickpage": {Java: "verteron/_1194ReducingTursinStrength.java:72-75", Reason: "Starter explicitly shows reward page 1352 on a plain click while the quest is REWARD"},
	"2016/clickpage": {Java: "altgard/_2016FearThis.java:145-150", Reason: "The reward giver explicitly shows page 2375 on a plain click while the quest is REWARD"},
	"2017/clickpage": {Java: "altgard/_2017TrespassersattheObservatory.java:106-114", Reason: "The reward giver explicitly shows page 2034 on a plain click while the quest is REWARD"},
	"2018/echo":      {Java: "altgard/_2018ReconstructingImpetusium.java:126-129", Reason: "The jewel-box handler returns true so ActionitemController runs its timed use and drop registration instead of echoing the click"},
	"1183/clickpage": {Java: "verteron/_1183SpiritOfNature.java:87", Reason: "First helper explicitly handles click by sending page 1352"},
	"1170/accept":    {Java: "verteron/_1170HeadlessStoneStatue.java:69", Reason: "Body starts the quest on any initial dialog"},
	"1170/echo":      {Java: "verteron/_1170HeadlessStoneStatue.java:69", Reason: "Body starts and sends page 1011 before returning false for the framework echo"},
	"1170/click":     {Java: "verteron/_1170HeadlessStoneStatue.java:69", Reason: "Body starts the quest on any initial dialog and sends page 1011"},
	"1162/echo":      {Java: "verteron/_1162_AltenosWeddingRing.java:128", Reason: "Reporter advances to REWARD for any dialog when var is 1"},
	"1076/clickpage": {Java: "reshanta/_1076FragmentofMemory2.java:95", Reason: "REWARD click explicitly returns page 10002"},
	"1075/clickpage": {Java: "reshanta/_1075NewWings.java:88", Reason: "REWARD click explicitly returns page 10002"},
	"1072/clickpage": {Java: "reshanta/_1072AbyssTraining.java:84", Reason: "REWARD click explicitly returns page 10002"},
	"1071/clickpage": {Java: "reshanta/_1071SpeakingBalaur.java:90", Reason: "REWARD click explicitly returns page 10002"},
	"1062/clickpage": {Java: "heiron/_1062IndratuLegion.java:91", Reason: "REWARD click explicitly returns page 10002"},
	"1034/clickpage": {Java: "eltnen/_1034DisappearingAether.java:90", Reason: "REWARD click explicitly returns page 2375"},
	"1032/clickpage": {Java: "eltnen/_1032ARulersDuty.java:137", Reason: "REWARD click explicitly returns page 2716"},
	"1031/clickpage": {Java: "eltnen/_1031TheMandurisSecret.java:145", Reason: "REWARD click explicitly returns page 3398"},
	"1002/clickpage": {Java: "poeta/_1002RequestoftheElim.java:101", Reason: "REWARD: if(getDialogId() == -1) return sendQuestDialog(.., 2716) for npc 203067 (the case -1 at l.207 is the loot barrel)"},
	"1005/clickpage": {Java: "poeta/_1005BarringtheGate.java:195", Reason: "REWARD: if(getDialogId() == -1) return sendQuestDialog(.., 2716) for npc 203067"},
	"1011/clickpage": {Java: "verteron/_1011DangerFromAbove.java:148", Reason: "REWARD: if (getDialogId() == -1) return sendQuestDialog(.., 1693) for npc 203109"},
	"1015/clickpage": {Java: "verteron/_1015FrillneckHunt.java:111", Reason: "REWARD: if(getDialogId() == -1) return sendQuestDialog(.., 1693) for npc 203129"},
	"1016/clickpage": {Java: "verteron/_1016SourceOfThePollution.java:89", Reason: "REWARD: if(getDialogId() == -1) return sendQuestDialog(.., 4080) for npc 203098"},
	"1019/clickpage": {Java: "verteron/_1019FlyingReconnaissance.java:178", Reason: "REWARD: targetId == 203098 && getDialogId() == -1 -> sendQuestDialog(.., 2034)"},
	"1020/clickpage": {Java: "verteron/_1020SealingTheAbyssGate.java:101", Reason: "REWARD: if(getDialogId() == -1) return sendQuestDialog(.., 1352) for npc 203098"},
	"1042/clickpage": {Java: "eltnen/_1042KeeperoftheKaidanKey.java:90-91", Reason: "REWARD: Telemachus handles dialog -1 by sending the specific reward page 10002"},
	"1057/clickpage": {Java: "heiron/_1057CreatingAMonster.java:98-99", Reason: "REWARD: end NPC 204500 handles dialog -1 by sending the specific reward page 10002"},
	"1141/clickpage": {Java: "verteron/_1141BelbuasTreasure.java:71", Reason: "npc 700122 with var 0 and dialog -1 sets REWARD and returns defaultQuestEndDialog: window 5 on the click"},
	"1141/finish":    {Java: "verteron/_1141BelbuasTreasure.java:78", Reason: "npc 700122: the reward select first gets the brace-less else's window 10, then defaultQuestEndDialog finishes the quest and sends window 10 again"},
	"1141/echo":      {Java: "verteron/_1141BelbuasTreasure.java:78", Reason: "npc 700122, any dialog but the var-0 click: the brace-less else sends window 10 before defaultQuestEndDialog (false for an unknown id), so the echo follows window 10"},
}

// conformanceKnownFailing is "<quest id>/<rule>" for rules a ported handler still breaks. Every entry has a row in
// QUEST_AUDIT.md; delete the entry when the handler is fixed (TestQuestConformanceKnownFailingStillFail fails if
// you forget). Do not add entries for new ports: fix the port.
var conformanceKnownFailing = map[string]bool{
	"1156/clickpage":  true,
	"1157/clickpage":  true,
	"1158/clickpage":  true,
	"2001/clickpage":  true,
	"2002/clickpage":  true,
	"2002/echo":       true,
	"2004/clickpage":  true,
	"2006/clickpage":  true,
	"2007/clickpage":  true,
	"2122/clickpage":  true,
	"3200/clickpage":  true,
	"3914/clickpage":  true,
	"3930/clickpage":  true,
	"3965/clickpage":  true,
	"3966/clickpage":  true,
	"3967/clickpage":  true,
	"3968/clickpage":  true,
	"3969/click":      true,
	"3969/clickpage":  true,
	"4200/clickpage":  true,
	"4934/clickpage":  true,
	"19004/clickpage": true,
}
