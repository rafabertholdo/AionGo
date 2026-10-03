# Starting skills and Java class mapping

The Go skill-tree loader previously matched XML `classId` strings directly to
player class names. Java instead matches `SkillClass.ordinal()` to
`PlayerClass.ordinal()`. These names differ in six positions, including reversed
Priest/Cleric names. A new Priest therefore received common skills but no
class-specific active or passive skills.

The loader now normalizes XML class names once, before character creation,
level-up learning and admin skill restoration use the tree.

| XML SkillClass | PlayerClass |
|---|---|
| WARRIOR | WARRIOR |
| FIGHTER | GLADIATOR |
| KNIGHT | TEMPLAR |
| SCOUT | SCOUT |
| ASSASSIN | ASSASSIN |
| RANGER | RANGER |
| MAGE | MAGE |
| WIZARD | SORCERER |
| ELEMENTALLIST | SPIRIT_MASTER |
| CLERIC | PRIEST |
| PRIEST | CLERIC |
| CHANTER | CHANTER |

Both races have the same level-one class-specific active skills:

| Starting class | Active skill IDs and names | Class passive count | Total starting skill count |
|---|---|---:|---:|
| Warrior | 169 Ferocious Strike I | 8 | 12 |
| Scout | 564 Swift Edge I; 572 Focused Evasion I | 5 | 10 |
| Mage | 1351 Flame Bolt I; 1373 Root I | 3 | 8 |
| Priest | 965 Healing Light I; 975 Smite I | 4 | 9 |

Totals include Return (1801), Bandage Heal (1803), and Collection (30001).
These follow Java's configured policy: autolearn entries are granted, while
other skills require learning normally. Advanced classes are obtained through
ascension rather than character creation.

Existing characters regain eligible missing autolearn skills when loaded for
login, before stats are calculated and the skill list is sent. Advanced classes
also receive eligible base-class entries below level 10, following Java's
`addMissingSkills` traversal. Race, character level and stigma restrictions are
checked. Existing higher skill levels and unrelated learned skills are retained;
the repair does not remove skills that may have been granted previously.
Collection grants use Essencetapping from level 10 onward. Failed saves stop
player loading and preserve the failed skill's in-memory state; successful saves
remain valid and can be retried on the next login without duplication.

Sources:

- [Java SkillClass enum](../java/AL-Game/src/main/java/com/aionemu/gameserver/skillengine/model/learn/SkillClass.java)
- [Java PlayerClass enum](../java/AL-Game/src/main/java/com/aionemu/gameserver/model/PlayerClass.java)
- [Java SkillTreeData ordinal lookup](../java/AL-Game/src/main/java/com/aionemu/gameserver/dataholders/SkillTreeData.java)
- [Java SkillLearnService](../java/AL-Game/src/main/java/com/aionemu/gameserver/services/SkillLearnService.java)
- [Java skill tree](../java/AL-Game/data/static_data/skill_tree/skill_tree.xml)
- [Java craft skill tree](../java/AL-Game/data/static_data/skill_tree/craft_skill_tree.xml)
- [User's Aion Codex reference](https://aioncodex.com/us/skills/priest/active/)

Regression checks compare all twelve classes against the raw XML using Java's
ordinal mapping, for both races and every level present in the files. Separate
checks assert exact level-one grants, restoration for all classes, repeated-login
idempotence, learning restrictions, existing levels and database failure handling.
Live database integration tests require a configured test database. Deployment
of the corrected game server is required before live characters benefit.
