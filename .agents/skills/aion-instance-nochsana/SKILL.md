---
name: aion-instance-nochsana
description: Investigate, implement, review, or play-test Nochsana Training Camp (NTC) for Aion 1.5/1.9. Covers its original Lower Abyss entrances, route, gate, siege weapon, artifact, mobs, General, quests, drops, and version traps. Do not use for later Classic or revamped NTC without an explicit version comparison.
---

# Nochsana Training Camp expert

Use this skill for map **300030000**, the original 1.5/1.9 six-player Nochsana Training Camp. Read [the gameplay and version evidence](references/gameplay.md) for route, encounters, quests, rewards, and source disagreements. Read [the server catalog](references/server-catalog.md) when changing or reviewing code, spawns, skills, quest credit, or packets. Consult `aion-server-expert` as well when starting the stack or comparing a real 1.9 client.

## Evidence rules

- Treat this repository's 1.9 client identity and static data as evidence of **identity, placement, and declared effects**, not proof that a scripted mechanic executes. A passing Go test proves its tested behavior, not retail parity.
- Use dated 2009–2010 firsthand guides for original gameplay. Their translations, cooldown reports, damage estimates, and personal tactics can disagree; keep the disagreement visible.
- The local 4.6 `MiDoor` and `MiBGuard_ChiefC` AI patterns are leads only. Confirm timing, thresholds, targets, reinforcements, and reset against 1.9 before porting them.
- Exclude the Eltnen/Morheim entrance added in 3.5, later Classic weekly entry rules, and the 7.0 solo remake from 1.9 implementation decisions. Check version before using any web page about NTC.
- When a player reports a failure, identify the exact run, faction, object ID, quest state, and client action. Compare client packets or a 1.9 play-through when collision, cursor behavior, movement, or phase ordering matters.

## Review and play-test checkpoints

1. Confirm faction portal **700413 Elyos** or **700414 Asmodian**, level **25–28**, group admission, and arrival. Observe whether this build enforces the period entrance quest and lockout; the local portal XML does not declare either.
2. Follow the outer path to artifact **700437** and the four named elites, then fortress gate **256694**. Use the faction's quest siege item, command its summoned weapon to attack, and check gate damage and the reported two-elite response.
3. Clear the fort, upper ramp, Docs, and Teleporter before engaging General **256693**. Observe linked aggro, damage, debuff, knockdown, fear, shield or add behavior, leash, wipe, and repeat entry.
4. Verify quest stages and party credit separately for each faction, chest object identity, seed-drop eligibility, loot, and exit **700438**. Record the first packet or state divergence and update the reference with its version and evidence.

Do not mark the instance complete from static spawn or skill counts alone. Current known gaps and source locations are in the linked references.
