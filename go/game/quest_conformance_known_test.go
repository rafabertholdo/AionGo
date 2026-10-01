package game

// conformanceException is a documented Java deviation: the Java handler really does this, so the rule does not apply.
type conformanceException struct {
	Reason string
	Java   string // file:line in AL-Game/data/scripts/system/handlers/quest
}

// conformanceExceptions is "<quest id>/<rule>" -> why Java itself breaks the rule. Cite the Java line.
var conformanceExceptions = map[string]conformanceException{
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
	"1309/clickpage":  true,
	"1323/clickpage":  true,
	"1913/clickpage":  true,
	"1914/clickpage":  true,
	"1915/clickpage":  true,
	"1916/clickpage":  true,
	"2001/clickpage":  true,
	"2002/clickpage":  true,
	"2002/echo":       true,
	"2004/clickpage":  true,
	"2006/clickpage":  true,
	"2007/clickpage":  true,
	"2122/clickpage":  true,
	"2274/clickpage":  true,
	"2901/clickpage":  true,
	"2902/clickpage":  true,
	"2903/clickpage":  true,
	"2904/clickpage":  true,
	"3093/clickpage":  true,
	"3200/clickpage":  true,
	"3914/clickpage":  true,
	"3930/clickpage":  true,
	"3931/clickpage":  true,
	"3965/clickpage":  true,
	"3966/clickpage":  true,
	"3967/clickpage":  true,
	"3968/clickpage":  true,
	"3969/click":      true,
	"3969/clickpage":  true,
	"4200/clickpage":  true,
	"4934/clickpage":  true,
	"4935/clickpage":  true,
	"19004/clickpage": true,
}
