// Code generated from AL-Game's StatEnum; DO NOT EDIT.

package data

// Stat is a creature stat, AL-Game's StatEnum, in its order.
type Stat uint8

// Stats, by their StatEnum name.
const (
	MaxDP                        Stat = iota // MAXDP
	MaxHP                                    // MAXHP
	MaxMP                                    // MAXMP
	Agility                                  // AGILITY
	Block                                    // BLOCK
	Evasion                                  // EVASION
	Concentration                            // CONCENTRATION
	Will                                     // WILL
	Health                                   // HEALTH
	Accuracy                                 // ACCURACY
	Knowledge                                // KNOWLEDGE
	Parry                                    // PARRY
	Power                                    // POWER
	Speed                                    // SPEED
	HitCount                                 // HIT_COUNT
	AttackRange                              // ATTACK_RANGE
	AttackSpeed                              // ATTACK_SPEED
	PhysicalAttack                           // PHYSICAL_ATTACK
	PhysicalAccuracy                         // PHYSICAL_ACCURACY
	PhysicalCritical                         // PHYSICAL_CRITICAL
	PhysicalDefense                          // PHYSICAL_DEFENSE
	MainHandHits                             // MAIN_HAND_HITS
	MainHandAccuracy                         // MAIN_HAND_ACCURACY
	MainHandCritical                         // MAIN_HAND_CRITICAL
	MainHandPower                            // MAIN_HAND_POWER
	MainHandAttackSpeed                      // MAIN_HAND_ATTACK_SPEED
	OffHandHits                              // OFF_HAND_HITS
	OffHandAccuracy                          // OFF_HAND_ACCURACY
	OffHandCritical                          // OFF_HAND_CRITICAL
	OffHandPower                             // OFF_HAND_POWER
	OffHandAttackSpeed                       // OFF_HAND_ATTACK_SPEED
	CriticalResist                           // CRITICAL_RESIST
	MagicalAttack                            // MAGICAL_ATTACK
	MagicalAccuracy                          // MAGICAL_ACCURACY
	MagicalCritical                          // MAGICAL_CRITICAL
	MagicalResist                            // MAGICAL_RESIST
	MaxDamages                               // MAX_DAMAGES
	MinDamages                               // MIN_DAMAGES
	IsMagicalAttack                          // IS_MAGICAL_ATTACK
	EarthResistance                          // EARTH_RESISTANCE
	FireResistance                           // FIRE_RESISTANCE
	WindResistance                           // WIND_RESISTANCE
	WaterResistance                          // WATER_RESISTANCE
	BoostMagicalSkill                        // BOOST_MAGICAL_SKILL
	BoostCastingTime                         // BOOST_CASTING_TIME
	BoostHate                                // BOOST_HATE
	BoostHeal                                // BOOST_HEAL
	FlyTime                                  // FLY_TIME
	FlySpeed                                 // FLY_SPEED
	PvpAttackRatio                           // PVP_ATTACK_RATIO
	PvpDefendRatio                           // PVP_DEFEND_RATIO
	DamageReduce                             // DAMAGE_REDUCE
	BleedResistance                          // BLEED_RESISTANCE
	BlindResistance                          // BLIND_RESISTANCE
	CharmResistance                          // CHARM_RESISTANCE
	ConfuseResistance                        // CONFUSE_RESISTANCE
	CurseResistance                          // CURSE_RESISTANCE
	DiseaseResistance                        // DISEASE_RESISTANCE
	FearResistance                           // FEAR_RESISTANCE
	OpenareialResistance                     // OPENAREIAL_RESISTANCE
	ParalyzeResistance                       // PARALYZE_RESISTANCE
	PerificationResistance                   // PERIFICATION_RESISTANCE
	PoisonResistance                         // POISON_RESISTANCE
	RootResistance                           // ROOT_RESISTANCE
	SilenceResistance                        // SILENCE_RESISTANCE
	SleepResistance                          // SLEEP_RESISTANCE
	SlowResistance                           // SLOW_RESISTANCE
	SnareResistance                          // SNARE_RESISTANCE
	SpinResistance                           // SPIN_RESISTANCE
	StaggerResistance                        // STAGGER_RESISTANCE
	StumbleResistance                        // STUMBLE_RESISTANCE
	StunResistance                           // STUN_RESISTANCE
	RegenMp                                  // REGEN_MP
	RegenHp                                  // REGEN_HP
	RegenFp                                  // REGEN_FP
	StaggerBoost                             // STAGGER_BOOST
	StumbleBoost                             // STUMBLE_BOOST
	StunBoost                                // STUN_BOOST
	HealBoost                                // HEAL_BOOST
	Allresist                                // ALLRESIST
	StunlikeResistance                       // STUNLIKE_RESISTANCE
	ElementalResistanceDark                  // ELEMENTAL_RESISTANCE_DARK
	ElementalResistanceLight                 // ELEMENTAL_RESISTANCE_LIGHT
	MagicalCriticalResist                    // MAGICAL_CRITICAL_RESIST
	MagicalCriticalDamageReduce              // MAGICAL_CRITICAL_DAMAGE_REDUCE
	PhysicalCriticalResist                   // PHYSICAL_CRITICAL_RESIST
	PhysicalCriticalDamageReduce             // PHYSICAL_CRITICAL_DAMAGE_REDUCE
	Erfire                                   // ERFIRE
	Erair                                    // ERAIR
	Erearth                                  // EREARTH
	Erwater                                  // ERWATER
	AbnormalResistanceAll                    // ABNORMAL_RESISTANCE_ALL
	MagicalDefend                            // MAGICAL_DEFEND
	Allpara                                  // ALLPARA
	Knowil                                   // KNOWIL
	Agidex                                   // AGIDEX
	Strvit                                   // STRVIT
)

// NumStats is how many stats there are.
const NumStats = 97

// statNames are the StatEnum names static data refers to stats by.
var statNames = [NumStats]string{
	"MAXDP",
	"MAXHP",
	"MAXMP",
	"AGILITY",
	"BLOCK",
	"EVASION",
	"CONCENTRATION",
	"WILL",
	"HEALTH",
	"ACCURACY",
	"KNOWLEDGE",
	"PARRY",
	"POWER",
	"SPEED",
	"HIT_COUNT",
	"ATTACK_RANGE",
	"ATTACK_SPEED",
	"PHYSICAL_ATTACK",
	"PHYSICAL_ACCURACY",
	"PHYSICAL_CRITICAL",
	"PHYSICAL_DEFENSE",
	"MAIN_HAND_HITS",
	"MAIN_HAND_ACCURACY",
	"MAIN_HAND_CRITICAL",
	"MAIN_HAND_POWER",
	"MAIN_HAND_ATTACK_SPEED",
	"OFF_HAND_HITS",
	"OFF_HAND_ACCURACY",
	"OFF_HAND_CRITICAL",
	"OFF_HAND_POWER",
	"OFF_HAND_ATTACK_SPEED",
	"CRITICAL_RESIST",
	"MAGICAL_ATTACK",
	"MAGICAL_ACCURACY",
	"MAGICAL_CRITICAL",
	"MAGICAL_RESIST",
	"MAX_DAMAGES",
	"MIN_DAMAGES",
	"IS_MAGICAL_ATTACK",
	"EARTH_RESISTANCE",
	"FIRE_RESISTANCE",
	"WIND_RESISTANCE",
	"WATER_RESISTANCE",
	"BOOST_MAGICAL_SKILL",
	"BOOST_CASTING_TIME",
	"BOOST_HATE",
	"BOOST_HEAL",
	"FLY_TIME",
	"FLY_SPEED",
	"PVP_ATTACK_RATIO",
	"PVP_DEFEND_RATIO",
	"DAMAGE_REDUCE",
	"BLEED_RESISTANCE",
	"BLIND_RESISTANCE",
	"CHARM_RESISTANCE",
	"CONFUSE_RESISTANCE",
	"CURSE_RESISTANCE",
	"DISEASE_RESISTANCE",
	"FEAR_RESISTANCE",
	"OPENAREIAL_RESISTANCE",
	"PARALYZE_RESISTANCE",
	"PERIFICATION_RESISTANCE",
	"POISON_RESISTANCE",
	"ROOT_RESISTANCE",
	"SILENCE_RESISTANCE",
	"SLEEP_RESISTANCE",
	"SLOW_RESISTANCE",
	"SNARE_RESISTANCE",
	"SPIN_RESISTANCE",
	"STAGGER_RESISTANCE",
	"STUMBLE_RESISTANCE",
	"STUN_RESISTANCE",
	"REGEN_MP",
	"REGEN_HP",
	"REGEN_FP",
	"STAGGER_BOOST",
	"STUMBLE_BOOST",
	"STUN_BOOST",
	"HEAL_BOOST",
	"ALLRESIST",
	"STUNLIKE_RESISTANCE",
	"ELEMENTAL_RESISTANCE_DARK",
	"ELEMENTAL_RESISTANCE_LIGHT",
	"MAGICAL_CRITICAL_RESIST",
	"MAGICAL_CRITICAL_DAMAGE_REDUCE",
	"PHYSICAL_CRITICAL_RESIST",
	"PHYSICAL_CRITICAL_DAMAGE_REDUCE",
	"ERFIRE",
	"ERAIR",
	"EREARTH",
	"ERWATER",
	"ABNORMAL_RESISTANCE_ALL",
	"MAGICAL_DEFEND",
	"ALLPARA",
	"KNOWIL",
	"AGIDEX",
	"STRVIT",
}

// stoneMasks are the ids SM_INVENTORY_INFO gives a manastone's stat by.
var stoneMasks = [NumStats]byte{
	MaxHP:             18,
	MaxMP:             20,
	Agility:           107,
	Block:             33,
	Evasion:           31,
	Concentration:     41,
	Knowledge:         106,
	Parry:             32,
	Speed:             36,
	AttackSpeed:       29,
	PhysicalAttack:    25,
	PhysicalAccuracy:  30,
	PhysicalCritical:  34,
	PhysicalDefense:   26,
	MagicalAttack:     27,
	MagicalAccuracy:   105,
	MagicalCritical:   40,
	MagicalResist:     28,
	FireResistance:    15,
	BoostMagicalSkill: 104,
	BoostHate:         109,
	FlyTime:           23,
	FlySpeed:          37,
}
